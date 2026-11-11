package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const shapesPath = "../examples/ir/shapes.json"

// shapesData は IR の data を読む(誤りなら試験を止める)。
func shapesData(t *testing.T, data string) *ShapesData {
	t.Helper()
	d, err := shapesFromIR(json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func slideXML(t *testing.T, pptx, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(pptx)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			return string(b)
		}
	}
	t.Fatalf("%s not found", name)
	return ""
}

// 例の IR を dark・light で組める。図形は本物の図形、コネクタは図形に繋がった
// p:cxnSp(a:stCxn・a:endCxn)で、矢印は a:tailEnd。誤りも警告も出ない。
func TestShapesExampleBuilds(t *testing.T) {
	d := mustReadIR(t, shapesPath)
	for _, name := range []string{"dark", "light"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), name+".pptx")
		warnings, err := renderDeckToFile(tm, d, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warnings)
		}
		x := slideXML(t, out, "ppt/slides/slide2.xml")
		for _, want := range []string{`prst="roundRect"`, `prst="ellipse"`, `prst="straightConnector1"`, `<a:tailEnd type="triangle"`, "<a:t>利用者の端末</a:t>"} {
			if !strings.Contains(x, want) {
				t.Errorf("%s: slide lacks %s", name, want)
			}
		}
		// コネクタ5本はどれも、実在する図形の id に両端を繋ぐ。
		ids := map[string]bool{}
		for _, m := range regexp.MustCompile(`<p:cNvPr id="(\d+)" name="(?:Shape|TextBox) `).FindAllStringSubmatch(x, -1) {
			ids[m[1]] = true
		}
		cx := regexp.MustCompile(`<a:stCxn id="(\d+)" idx="\d"/><a:endCxn id="(\d+)" idx="\d"/>`).FindAllStringSubmatch(x, -1)
		if len(cx) != 5 {
			t.Fatalf("%s: %d connected connectors, want 5", name, len(cx))
		}
		for _, m := range cx {
			if !ids[m[1]] || !ids[m[2]] {
				t.Errorf("%s: connector joins unknown shape ids %v", name, m[1:])
			}
		}
		// 色はテーマから(16進は IR に無い)。
		if !strings.Contains(x, tm.Theme["accent"]) {
			t.Errorf("%s: accent color not used", name)
		}
	}
	errs, warns := CheckFile(shapesPath, "")
	if len(errs) != 0 || len(warns) != 0 {
		t.Fatalf("check: errs %+v warns %+v", errs, warns)
	}
}

// 楕円の接続点は 0・2・4・6、矩形は 0・1・2・3。横に離れていれば左右の辺を使う。
func TestConnectorSides(t *testing.T) {
	a, b := LBox{X: 0, Y: 0, Width: 100, Height: 50}, LBox{X: 200, Y: 20, Width: 100, Height: 50}
	if sa, sb, _ := connectorSides("rect", "rect", a, b); sa != 3 || sb != 1 {
		t.Fatalf("horizontal: %d %d", sa, sb)
	}
	below := LBox{X: 10, Y: 200, Width: 100, Height: 50}
	if sa, sb, _ := connectorSides("rect", "rect", a, below); sa != 2 || sb != 0 {
		t.Fatalf("vertical: %d %d", sa, sb)
	}
	if idx, x, y := connectorSite("ellipse", b, 1); idx != 2 || x != 200 || y != 45 {
		t.Fatalf("ellipse left site: %d %v %v", idx, x, y)
	}
	if idx, _, _ := connectorSite("rect", b, 3); idx != 3 {
		t.Fatalf("rect right site: %d", idx)
	}
}

// #41:向かい合う辺の組だと辺をかすめる(浅い角度で入る)位置関係では、隣り合う
// 辺の組(右→上など)に替える。
func TestConnectorSidesShallow(t *testing.T) {
	// 右下にあり、縦のすき間のほうが少し大きい:以前は下→上で、ほぼ水平に上の辺へ入った。
	a := LBox{X: 0, Y: 0, Width: 100, Height: 120}
	b := LBox{X: 130, Y: 160, Width: 120, Height: 120}
	sa, sb, ang := connectorSides("rect", "ellipse", a, b)
	if sa != 3 || sb != 0 {
		t.Fatalf("got sides %d->%d (%.1f deg), want right->top", sa, sb, ang)
	}
	if ang < connectorAngleMin {
		t.Fatalf("angle %.1f < %v", ang, connectorAngleMin)
	}
	// 向かい合う組で十分な角度があれば、それを使う(真横・斜め上の遠い箱)。
	far := LBox{X: 400, Y: -150, Width: 100, Height: 50}
	if sa, sb, _ := connectorSides("rect", "rect", a, far); sa != 3 || sb != 1 {
		t.Fatalf("far: %d %d", sa, sb)
	}
}

// 書いた PPTX で、コネクタの両端の座標が、繋いだ図形の接続点(a:stCxn・a:endCxn の
// idx が指す点。presetShapeDefinitions の cxnLst の順)と一致するか。
func TestConnectorEndsMatchSites(t *testing.T) {
	src := "---\ntitle: t\ntemplate: light\ncover: false\n---\n\n# t\n\n```shapes\naspect: 2.6\nitems:\n" +
		"  - {id: s1, kind: rect, x: 0.0, y: 0.05, w: 0.25, h: 0.2}\n" +
		"  - {id: t1, kind: rect, x: 0.3, y: 0.32, w: 0.25, h: 0.2}\n" +
		"  - {id: s2, kind: rect, x: 0.0, y: 0.6, w: 0.2, h: 0.18}\n" +
		"  - {id: t2, kind: roundRect, x: 0.28, y: 0.8, w: 0.2, h: 0.18}\n" +
		"  - {id: s3, kind: rect, x: 0.62, y: 0.1, w: 0.14, h: 0.3}\n" +
		"  - {id: t3, kind: ellipse, x: 0.8, y: 0.55, w: 0.18, h: 0.3}\n" +
		"  - {id: c, kind: ellipse, x: 0.4, y: 0.6, w: 0.12, h: 0.2}\n" +
		"  - {kind: connector, from: s1, to: t1, arrow: true}\n" +
		"  - {kind: connector, from: s2, to: t2, arrow: true}\n" +
		"  - {kind: connector, from: s3, to: t3, arrow: true}\n" +
		"  - {kind: connector, from: c, to: s1, arrow: true}\n" +
		"  - {kind: connector, from: t3, to: c}\n```\n"
	deck, err := ReadDeck("t.bx.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	pp := filepath.Join(t.TempDir(), "c.pptx")
	if err := os.WriteFile(pp, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	x := slideXML(t, pp, "ppt/slides/slide1.xml")
	type geo struct {
		prst         string
		x, y, w, h   float64
		flipH, flipV bool
	}
	num := func(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
	shapes := map[string]geo{}
	spRe := regexp.MustCompile(`(?s)<p:sp>.*?<p:cNvPr id="(\d+)".*?<a:off x="(\d+)" y="(\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>.*?prst="(\w+)".*?</p:sp>`)
	for _, m := range spRe.FindAllStringSubmatch(x, -1) {
		shapes[m[1]] = geo{prst: m[6], x: num(m[2]), y: num(m[3]), w: num(m[4]), h: num(m[5])}
	}
	site := func(g geo, idx int) (float64, float64) {
		if g.prst == "ellipse" { // cxnLst: t, (il,it), l, (il,ib), b, (ir,ib), r, (ir,it)
			pts := [][2]float64{{.5, 0}, {0, .5}, {.5, 1}, {1, .5}}
			if idx%2 != 0 {
				t.Fatalf("ellipse diagonal site %d not expected", idx)
			}
			p := pts[idx/2]
			return g.x + p[0]*g.w, g.y + p[1]*g.h
		}
		pts := [][2]float64{{.5, 0}, {0, .5}, {.5, 1}, {1, .5}} // rect・roundRect: t, l, b, r
		p := pts[idx]
		return g.x + p[0]*g.w, g.y + p[1]*g.h
	}
	cxRe := regexp.MustCompile(`(?s)<p:cxnSp>.*?<a:stCxn id="(\d+)" idx="(\d+)"/><a:endCxn id="(\d+)" idx="(\d+)"/>.*?<a:xfrm([^>]*)><a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>.*?prst="straightConnector1".*?</p:cxnSp>`)
	ms := cxRe.FindAllStringSubmatch(x, -1)
	if len(ms) != 5 {
		t.Fatalf("connectors: %d", len(ms))
	}
	for _, m := range ms {
		ox, oy, cx, cy := num(m[6]), num(m[7]), num(m[8]), num(m[9])
		x1, y1, x2, y2 := ox, oy, ox+cx, oy+cy
		if strings.Contains(m[5], `flipH="1"`) {
			x1, x2 = x2, x1
		}
		if strings.Contains(m[5], `flipV="1"`) {
			y1, y2 = y2, y1
		}
		si, _ := strconv.Atoi(m[2])
		ei, _ := strconv.Atoi(m[4])
		sx, sy := site(shapes[m[1]], si)
		ex, ey := site(shapes[m[3]], ei)
		const tol = 2 // EMU の丸め
		if math.Abs(sx-x1) > tol || math.Abs(sy-y1) > tol || math.Abs(ex-x2) > tol || math.Abs(ey-y2) > tol {
			t.Errorf("connector %s(%d)->%s(%d): line (%v,%v)-(%v,%v), sites (%v,%v)-(%v,%v)", m[1], si, m[3], ei, x1, y1, x2, y2, sx, sy, ex, ey)
		}
	}
}

// 浅くしか繋げない位置関係は警告 shapes-connector-angle(path はそのコネクタ)。
func TestConnectorAngleWarning(t *testing.T) {
	src := "---\ntitle: t\ntemplate: light\ncover: false\n---\n\n# t\n\n```shapes\naspect: 2.6\nitems:\n" +
		"  - {id: a, kind: rect, x: 0.0, y: 0.0, w: 0.3, h: 0.3}\n" +
		"  - {id: b, kind: rect, x: 0.32, y: 0.2, w: 0.3, h: 0.3}\n" +
		"  - {id: c, kind: rect, x: 0.0, y: 0.7, w: 0.2, h: 0.3}\n" +
		"  - {kind: connector, from: a, to: b, arrow: true}\n" +
		"  - {kind: connector, from: a, to: c, arrow: true}\n```\n"
	res := Check("t.bx.md", []byte(src), CheckOptions{})
	var got []Issue
	for _, w := range res.Warnings {
		if w.Code == "shapes-connector-angle" {
			got = append(got, w)
		}
	}
	if len(got) != 1 || got[0].Path != "/slides/0/body/0/data/items/3" {
		t.Fatalf("warnings: %+v", res.Warnings)
	}
}

// 自己申告: 自然な高さは縦横比から、aspect を書けばそれが上限、書かなければ上限なし。
// 最小は図形の文が収まる高さ。
func TestShapesMeasure(t *testing.T) {
	v := defaultShapesValues()
	s := &ShapesSubsystem{Data: shapesData(t, `{"aspect": 2, "items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.5, "h": 0.5}]}`)}
	m := s.Measure(Constraint{Width: 432})
	inner := 432 - 2*v.Padding
	if !near(m.Natural.Height, inner/2+2*v.Padding) || !near(m.Max.Height, m.Natural.Height) {
		t.Fatalf("natural %v max %v", m.Natural, m.Max)
	}
	if !near(m.Min.Height, v.MinHeight) {
		t.Fatalf("min = %v, want %v", m.Min.Height, v.MinHeight)
	}
	free := &ShapesSubsystem{Data: shapesData(t, `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.5, "h": 0.5}]}`)}
	if m := free.Measure(Constraint{Width: 432}); m.Max.Height != 0 || !near(m.Natural.Height, inner/v.DefaultAspect+2*v.Padding) {
		t.Fatalf("no aspect: %+v", m)
	}
	// 低い図形に長い文を入れると、最小の高さが上がる。
	long := &ShapesSubsystem{Data: shapesData(t, `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.2, "h": 0.1, "text": "`+strings.Repeat("長い文", 12)+`"}]}`)}
	if m := long.Measure(Constraint{Width: 432}); m.Min.Height <= v.MinHeight || m.Natural.Height < m.Min.Height {
		t.Fatalf("text should raise min: %+v", m)
	}
	// 割り当てがその最小なら、文は収まる(Lint が文のはみ出しを言わない)。
	m = long.Measure(Constraint{Width: 432})
	for _, w := range long.Lint(LBox{Width: 432, Height: m.Min.Height}) {
		if strings.Contains(w, "text does not fit") {
			t.Fatalf("text should fit at min height: %s", w)
		}
	}
}

// 読み込みが断るもの。
func TestShapesFromIRRejects(t *testing.T) {
	cases := map[string]string{
		"x outside":         `{"items": [{"kind": "rect", "x": 1.2, "y": 0, "w": 0.1, "h": 0.1}]}`,
		"x+w":               `{"items": [{"kind": "rect", "x": 0.8, "y": 0, "w": 0.3, "h": 0.1}]}`,
		"y+h":               `{"items": [{"kind": "rect", "x": 0, "y": 0.95, "w": 0.1, "h": 0.1}]}`,
		"zero size":         `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0, "h": 0.1}]}`,
		"unknown kind":      `{"items": [{"kind": "star", "x": 0, "y": 0, "w": 0.1, "h": 0.1}]}`,
		"key of other kind": `{"items": [{"kind": "line", "from": [0, 0], "to": [1, 1], "fill": "card"}]}`,
		"missing key":       `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1}]}`,
		"text needs text":   `{"items": [{"kind": "text", "x": 0, "y": 0, "w": 0.1, "h": 0.1}]}`,
		"hex color":         `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1, "fill": "#34C08F"}]}`,
		"lower hex color":   `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1, "fill": "deadbe"}]}`,
		"font size number":  `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1, "textStyle": "18pt"}]}`,
		"title text style":  `{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1, "textStyle": "slideTitle"}]}`,
		"point outside":     `{"items": [{"kind": "arrow", "from": [0, 0], "to": [1.5, 1]}]}`,
		"point shape":       `{"items": [{"kind": "line", "from": [0, 0, 0], "to": [1, 1]}]}`,
		"same points":       `{"items": [{"kind": "line", "from": [0.5, 0.5], "to": [0.5, 0.5]}]}`,
		"duplicate id":      `{"items": [{"id": "a", "kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1}, {"id": "a", "kind": "ellipse", "x": 0.5, "y": 0, "w": 0.1, "h": 0.1}]}`,
		"missing target":    `{"items": [{"id": "a", "kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1}, {"kind": "connector", "from": "a", "to": "b"}]}`,
		"self connector":    `{"items": [{"id": "a", "kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1}, {"kind": "connector", "from": "a", "to": "a"}]}`,
		"no items":          `{"items": []}`,
		"aspect":            `{"aspect": 0, "items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1}]}`,
	}
	for name, src := range cases {
		if _, err := shapesFromIR(json.RawMessage(src)); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.HasPrefix(err.Error(), "shapes: ") {
			t.Errorf("%s: error lacks the location: %v", name, err)
		}
	}
	// 辺が接するだけ・図形の中の文字は、組み立ての誤りでも重なりでもない。
	ok := shapesData(t, `{"items": [{"id": "a", "kind": "rect", "x": 0, "y": 0, "w": 0.5, "h": 0.5, "text": "中の文字"}, {"id": "b", "kind": "ellipse", "x": 0.5, "y": 0, "w": 0.5, "h": 0.5}]}`)
	if w := (&ShapesSubsystem{Data: ok}).Lint(LBox{Width: 600, Height: 300}); len(w) != 0 {
		t.Fatalf("touching shapes warned: %v", w)
	}
}

// テーマに無い役割名は、テンプレートの Factory が断る(build の誤り)。
func TestShapesUnknownRoleRejectedByTemplate(t *testing.T) {
	tm, err := LookupTemplate("dark")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tm.Subsystems.Build("shapes", json.RawMessage(`{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1, "fill": "highlight"}]}`))
	if err == nil || !strings.Contains(err.Error(), "items[0]") {
		t.Fatalf("err = %v", err)
	}
	// 字の段はテンプレートの values から引く。
	sub, err := tm.Subsystems.Build("shapes", json.RawMessage(`{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.1, "h": 0.1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := sub.(*ShapesSubsystem).textSize("body"); got != tm.Values.Body {
		t.Fatalf("body size = %v, want %v", got, tm.Values.Body)
	}
}

// 図形どうしの重なりは警告。線・矢印・コネクタは見ない。文が収まらなければ別の警告。
func TestShapesLint(t *testing.T) {
	d := shapesData(t, `{"items": [
	  {"id": "a", "kind": "rect", "x": 0, "y": 0, "w": 0.5, "h": 0.5},
	  {"kind": "text", "x": 0.4, "y": 0.4, "w": 0.3, "h": 0.3, "text": "上に重ねた文字"},
	  {"id": "c", "kind": "ellipse", "x": 0.8, "y": 0.8, "w": 0.2, "h": 0.2, "text": "`+strings.Repeat("収まらない長い文", 10)+`"},
	  {"kind": "line", "from": [0, 0], "to": [1, 1]},
	  {"kind": "connector", "from": "a", "to": "c", "arrow": true}]}`)
	w := (&ShapesSubsystem{Data: d}).Lint(LBox{Width: 600, Height: 300})
	if len(w) != 2 {
		t.Fatalf("warnings = %v", w)
	}
	if !strings.Contains(w[0], `items[1] ("上に重ねた文字") overlaps items[0] ("a")`) {
		t.Errorf("overlap warning = %s", w[0])
	}
	if !strings.Contains(w[1], `items[2] ("c"): text does not fit`) {
		t.Errorf("text warning = %s", w[1])
	}
	for in, want := range map[string][2]string{
		"slide 2: [SUBSYSTEM WARNING] body[1]: " + w[0][len("[SUBSYSTEM WARNING] "):]: {"shapes-overlap", "/slides/1/body/1/data/items/1"},
		"slide 2: [SUBSYSTEM WARNING] body[1]: " + w[1][len("[SUBSYSTEM WARNING] "):]: {"shapes-text-overflow", "/slides/1/body/1/data/items/2"},
	} {
		if is := warningIssue("f", in); is.Code != want[0] || is.Path != want[1] {
			t.Errorf("%q -> %s %s, want %v", in, is.Code, is.Path, want)
		}
	}
}

// 重なりの例は、誤り0件で shapes-overlap の警告が1件。path は後から置いた図形、
// .md なら図形の行が付く。
func TestCheckShapesOverlapExample(t *testing.T) {
	errs, warns := CheckFile("../examples/ir/check-warnings/shapes-overlap.json", "")
	if len(errs) != 0 || len(warns) != 1 || warns[0].Code != "shapes-overlap" || warns[0].Path != "/slides/1/body/1/data/items/7" {
		t.Fatalf("errs %+v warns %+v", errs, warns)
	}
	src, err := os.ReadFile("../examples/md/shapes.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	const dbLine = "  - {id: db, kind: ellipse, x: 0.72, y: 0.4, w: 0.18, h: 0.2, text: データベース, line: grid}"
	s := strings.Replace(string(src), dbLine, strings.Replace(dbLine, "y: 0.4", "y: 0.25", 1), 1)
	path := writeTemp(t, "overlap.bx.md", s)
	_, warns = CheckFile(path, "")
	if len(warns) != 1 || warns[0].Code != "shapes-overlap" || warns[0].Line != lineOf(s, "id: db,") {
		t.Fatalf("md warns = %+v, want line %d", warns, lineOf(s, "id: db,"))
	}
}

// .md の意味の誤り(枠の外・知らない役割名・無い id)は、その図形・キーの行で返る。
func TestCheckShapesMarkdownLines(t *testing.T) {
	src, err := os.ReadFile("../examples/md/shapes.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		from, to, code, path, lineHas string
	}{
		{"x: 0.72, y: 0.12, w: 0.18", "x: 0.72, y: 0.12, w: 0.3", "shape-bounds", "/slides/1/body/1/data/items/6/w", "id: auth,"},
		{"text: データベース, line: grid", "text: データベース, line: grid, fill: highlight", "role-exists", "/slides/1/body/1/data/items/7/fill", "id: db,"},
		{"from: jobs, to: db", "from: jobs, to: cache", "target-exists", "/slides/1/body/1/data/items/13/to", "from: jobs,"},
		{"    y: 0.68\n", "    y: 0.88\n", "shape-bounds", "/slides/1/body/1/data/items/8/h", "    h: 0.2"},
	}
	for _, c := range cases {
		s := strings.Replace(string(src), c.from, c.to, 1)
		if s == string(src) {
			t.Fatalf("%s: pattern not found", c.code)
		}
		errs, _ := CheckFile(writeTemp(t, "bad.bx.md", s), "")
		if len(errs) != 1 || errs[0].Code != c.code || errs[0].Path != c.path || errs[0].Line != lineOf(s, c.lineHas) {
			t.Errorf("%s: errs = %+v, want path %s line %d", c.code, errs, c.path, lineOf(s, c.lineHas))
		}
	}
}

// ```shapes の YAML の形の誤りは、パーサーが行と列つきで返す。
func TestShapesYAMLErrors(t *testing.T) {
	head := "# A\n```shapes\nitems:\n" // 1つ目の item はファイルの4行目
	cases := []struct {
		name, yaml, code string
		line             int
	}{
		{"hex", "  - {kind: rect, x: 0, y: 0, w: 0.1, h: 0.1, fill: \"#34C08F\"}\n", codeInvalidValue, 4},
		{"outside", "  - {kind: rect, x: 0, y: 0, w: 0.1, h: 0.1}\n  - {kind: rect, x: 1.5, y: 0, w: 0.1, h: 0.1}\n", codeInvalidValue, 5},
		{"quoted number", "  - {kind: rect, x: \"0.1\", y: 0, w: 0.1, h: 0.1}\n", codeInvalidValue, 4},
		{"bad point", "  - {kind: line, from: [0.1], to: [0.5, 0.5]}\n", codeInvalidValue, 4},
		{"unknown kind", "  - {kind: star, x: 0, y: 0, w: 0.1, h: 0.1}\n", codeInvalidValue, 4},
		{"no kind", "  - {x: 0, y: 0, w: 0.1, h: 0.1}\n", codeMissingKey, 4},
		{"key of other kind", "  - kind: line\n    from: [0, 0]\n    to: [1, 1]\n    fill: card\n", codeUnknownKey, 7},
		{"font size", "  - {kind: text, x: 0, y: 0, w: 0.1, h: 0.1, text: a, textStyle: 18}\n", codeInvalidValue, 4},
	}
	for _, c := range cases {
		_, errs := MarkdownToIR("t.bx.md", []byte(head+c.yaml+"```\n"))
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.Line == c.line && strings.HasPrefix(e.Path, "/slides/0/body/0/data") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at line %d, got %v", c.name, c.code, c.line, errs)
		}
	}
}

// `boxloom schema` は dark・light の両方で shapes を名乗り、irSchema に同梱する。
func TestSchemaDeclaresShapes(t *testing.T) {
	m := schemaOut(t, "")
	for _, ts := range m["templates"].([]any) {
		tm := ts.(map[string]any)
		found := false
		for _, s := range tm["subsystems"].([]any) {
			sm := s.(map[string]any)
			if sm["name"] == "shapes" {
				found = sm["fence"] == "```shapes" && sm["schema"] == "/irSchema/$defs/shapes"
			}
		}
		if !found {
			t.Errorf("%s: shapes not declared", tm["name"])
		}
	}
	defs := m["irSchema"].(map[string]any)["$defs"].(map[string]any)
	if _, ok := defs["shapes"]; !ok {
		t.Error("irSchema lacks shapes")
	}
	// 字の段の一覧は、Schema とコードで同じ。
	var sch struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(embeddedSchema("subsystems/shapes.schema.json"), &sch); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(sch.Defs["textStyle"].Enum, ","); got != strings.Join(shapeTextStyles, ",") {
		t.Errorf("schema textStyle %s, code %s", got, strings.Join(shapeTextStyles, ","))
	}
}

func writeTemp(t *testing.T, name, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// lineOf は sub を含む最初の行の番号(1から)。
func lineOf(s, sub string) int {
	for i, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return i + 1
		}
	}
	return 0
}

// 例:斜めの3組のうち、浅くしか繋げない1組(受付→振り分け)だけが警告になる。
// .md でも同じ警告にコネクタの行が付く。
func TestConnectorAngleExample(t *testing.T) {
	errs, warns := CheckFile("../examples/ir/check-warnings/shapes-connector-angle.json", "")
	if len(errs) != 0 || len(warns) != 1 || warns[0].Code != "shapes-connector-angle" || warns[0].Path != "/slides/0/body/0/data/items/6" {
		t.Fatalf("errs %+v warns %+v", errs, warns)
	}
	p := "../examples/md/warnings/shapes-connector-angle.bx.md"
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	errs, warns = CheckFile(p, "")
	if len(errs) != 0 || len(warns) != 1 || warns[0].Line != lineOf(string(b), "from: s1, to: t1") {
		t.Fatalf("md: errs %+v warns %+v", errs, warns)
	}
}
