package boxloom

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 屋根・柱・土台の図(pillars、#121)。文はすべて架空。

var pillarsTestPillars = []string{
	`{title: 場をひらく, items: [週末に拠点を開く, 催しを月に一度続ける], label: "1"}`,
	`{title: 担い手を育てる, items: [運営を三人で回す, 次の担い手へ渡す], label: "2"}`,
	`{title: 声を集める, items: [住民に話を聞く, 数と声をまとめる], label: "3"}`,
	`{title: 決まりを書く, items: [運営の決まりを作る], label: "4"}`,
	`{title: ふり返る, items: [年に一度見直す], label: "5"}`,
}

func pillarsYAML(n int) string {
	var b strings.Builder
	b.WriteString("```pillars\nroof: 架空の町 Kirinoha の拠点を、だれもが立ち寄る居場所にする\npillars:\n")
	for _, p := range pillarsTestPillars[:n] {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	b.WriteString("foundation: 住民の参加と、町の小さな予算\n```\n")
	return b.String()
}

// pillarsMD は pillars を1枚に置く(inColumns なら columns の左のまとまりに、右に箇条)。
func pillarsMD(tmpl string, n int, inColumns bool) string {
	head := fmt.Sprintf("---\ntitle: T\ntemplate: %s\n---\n\n# 柱が拠点を支える\n\n", tmpl)
	if !inColumns {
		return head + pillarsYAML(n)
	}
	return head + "::::columns\n:::column\n" + pillarsYAML(n) + ":::\n:::column 柱ごとの要点 {role=main}\n" +
		"- 週末に拠点を開き、催しを続ける\n- 運営を三人で回す\n- 住民に話を聞く\n:::\n::::\n"
}

// pillarsData は pillarsMD と同じ図の IR の data。
func pillarsData(t *testing.T, n int) string {
	t.Helper()
	v, errs := mdToAny(t, "# A\n\n"+pillarsYAML(n))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	b, _ := json.Marshal(v)
	var d struct {
		Slides []struct {
			Body []struct {
				Data json.RawMessage `json:"data"`
			} `json:"body"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return string(d.Slides[0].Body[0].Data)
}

// 3本・5本の柱を、1枚に置いても columns の中に置いても(3つのテンプレート):誤り・
// 警告が無く、屋根・名前の丸・柱・土台は図の矩形の中で互いに重ならない。描いた図形の
// 文字の枠も重ならず、図の矩形の中。
func TestPillarsShapesApartInsideFigure(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		for _, n := range []int{3, 5} {
			for _, inCols := range []bool{false, true} {
				name := fmt.Sprintf("%s/%d/columns=%v", tmpl, n, inCols)
				src := []byte(pillarsMD(tmpl, n, inCols))
				path := filepath.Join(t.TempDir(), "p.bx.md")
				if err := os.WriteFile(path, src, 0o644); err != nil {
					t.Fatal(err)
				}
				errs, warns, _ := CheckFileCapacity(path, tmpl)
				if len(errs) > 0 || len(warns) > 0 {
					t.Errorf("%s: errors %+v warnings %+v", name, errs, warns)
				}
				dump, err := Layout(path, src, BuildOptions{})
				if err != nil {
					t.Fatal(err)
				}
				leaf := findSubsystem(dump.Slides[1].Root, "pillars")
				if leaf == nil {
					t.Fatalf("%s: no pillars in the layout", name)
				}
				d := leaf.Subsystem.Draw
				box := LBox{X: d.X, Y: d.Y, Width: d.Width, Height: d.Height}
				sub := figureFromIR(t, tmpl, "pillars", pillarsData(t, n)).(*PillarsSubsystem)
				restore := useFonts(mustTemplate(tmpl).Fonts)
				_, pl := sub.place(box)
				lint := sub.Lint(box)
				p := NewPPTXBuilder(960, 540)
				sub.Render(RenderContext{Out: p, Roles: mustTemplate(tmpl).Theme}, box)
				restore()
				if len(lint) > 0 {
					t.Errorf("%s: lint %v", name, lint)
				}
				parts := map[string]LBox{"roof": pl.roof, "foundation": pl.foundation}
				for i := range pl.pillars {
					parts[fmt.Sprintf("pillars[%d]", i)] = pl.pillars[i]
					parts[fmt.Sprintf("labels[%d]", i)] = pl.labels[i]
				}
				checkApartInside(t, name+" parts", box, parts)
				texts := map[string]LBox{}
				for k, s := range p.Shapes {
					if s.kind != skShape || s.text == "" {
						continue
					}
					tr := shapeTextRect(s.prst, s.box, s.depth)
					texts[fmt.Sprintf("shape %d %q", k, s.text)] = LBox{X: tr.X + s.insetX, Y: tr.Y + s.insetY,
						Width: tr.Width - 2*s.insetX, Height: tr.Height - 2*s.insetY}
				}
				if len(texts) != 2*n+2 {
					t.Errorf("%s: %d shapes with text, want %d", name, len(texts), 2*n+2)
				}
				checkApartInside(t, name+" text", box, texts)
			}
		}
	}
}

func checkApartInside(t *testing.T, name string, box LBox, parts map[string]LBox) {
	t.Helper()
	var keys []string
	for k, b := range parts {
		if b.Width <= 0 {
			continue
		}
		keys = append(keys, k)
		if !within(box, b) {
			t.Errorf("%s: %s %+v is outside the figure %+v", name, k, b, box)
		}
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if boxesOverlap(parts[keys[i]], parts[keys[j]]) {
				t.Errorf("%s: %s %+v overlaps %s %+v", name, keys[i], parts[keys[i]], keys[j], parts[keys[j]])
			}
		}
	}
}

// 屋根は文を持つ台形(adj で斜めの辺)、柱は箇条の印つきの箱、名前は丸。色は役割名から
// (基調色の系統):屋根は accent、柱・土台は accent を地の色に寄せた色。
func TestPillarsRenderShapes(t *testing.T) {
	tm := mustTemplate("report")
	sub := figureFromIR(t, "report", "pillars", pillarsData(t, 3)).(*PillarsSubsystem)
	defer useFonts(tm.Fonts)()
	m := sub.Measure(Constraint{Width: 860})
	p := NewPPTXBuilder(960, 540)
	sub.Render(RenderContext{Out: p, Roles: tm.Theme}, LBox{X: 50, Y: 100, Width: 860, Height: m.Natural.Height})
	var roof, ellipses, rects int
	accent := tm.Theme.Color("accent")
	for _, s := range p.Shapes {
		switch s.prst {
		case "trapezoid":
			roof++
			if s.fill != accent || len(s.adj) != 1 || s.adj[0].Name != "adj" || s.adj[0].Val <= 0 {
				t.Errorf("roof: fill %s adj %+v", s.fill, s.adj)
			}
		case "ellipse":
			ellipses++
		case "rect":
			rects++
			if s.fill == accent || s.fill != mixHex(accent, tm.Theme.Color("bg"), sub.vals().PillarTint) && s.fill != mixHex(accent, tm.Theme.Color("bg"), sub.vals().FoundationTint) {
				t.Errorf("box fill %s is not a tint of the accent", s.fill)
			}
		}
	}
	if roof != 1 || ellipses != 3 || rects != 4 {
		t.Errorf("roof %d ellipses %d rects %d", roof, ellipses, rects)
	}
	var xml strings.Builder
	for i := range p.Shapes {
		xml.WriteString(p.shapeXML(&p.Shapes[i]))
	}
	for _, want := range []string{`prst="trapezoid"`, "<a:buChar", "週末に拠点を開く", "住民の参加と、町の小さな予算"} {
		if !strings.Contains(xml.String(), want) {
			t.Errorf("slide XML lacks %s", want)
		}
	}
}

// 図の矩形が狭すぎ・低すぎて文が入らなければ figure-text-overflow の警告になる。
func TestPillarsOverflowWarns(t *testing.T) {
	sub := figureFromIR(t, "light", "pillars", pillarsData(t, 5)).(*PillarsSubsystem)
	defer useFonts(mustTemplate("light").Fonts)()
	ws := sub.Lint(LBox{Width: 300, Height: 120})
	if len(ws) == 0 || !strings.Contains(strings.Join(ws, "\n"), "pillars: pillars[") {
		t.Errorf("no overflow warning: %v", ws)
	}
	m := sub.Measure(Constraint{})
	if ws := sub.Lint(LBox{Width: m.Min.Width, Height: sub.Measure(Constraint{Width: m.Min.Width}).Natural.Height}); len(ws) > 0 {
		t.Errorf("min width warns %v", ws)
	}
}

// 柱の数(2〜6)・名前(2字まで)・知らないキーは IR でも Markdown でも誤り。
func TestPillarsSchemaErrors(t *testing.T) {
	ir := func(data string) string {
		return `{"boxloom":"0.1","slides":[{"type":"content","title":"A","body":[{"type":"subsystem","subsystem":"pillars","data":` + data + `}]}]}`
	}
	two := `{"title":"人"},{"title":"場"}`
	for _, c := range []struct{ name, data, want string }{
		{"one pillar", `{"roof":"r","pillars":[{"title":"人"}]}`, "2 to 6 pillars"},
		{"seven pillars", `{"roof":"r","pillars":[` + strings.Repeat(`{"title":"人"},`, 6) + `{"title":"人"}]}`, "2 to 6 pillars"},
		{"long label", `{"roof":"r","pillars":[{"title":"人","label":"123"},{"title":"場"}]}`, "longer than 2"},
		{"blank label", `{"roof":"r","pillars":[{"title":"人","label":" "},{"title":"場"}]}`, "blank"},
		{"no roof", `{"pillars":[` + two + `]}`, "roof"},
		{"color", `{"roof":"r","color":"accent","pillars":[` + two + `]}`, "color"},
		{"empty items", `{"roof":"r","pillars":[{"title":"人","items":[]},{"title":"場"}]}`, "items"},
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		os.WriteFile(path, []byte(ir(c.data)), 0o644)
		errs, _ := CheckFile(path, "")
		if len(errs) == 0 || !strings.Contains(fmt.Sprint(errs), c.want) {
			t.Errorf("%s: errors %+v (want %q)", c.name, errs, c.want)
		}
	}
	for _, c := range []struct{ name, body string }{
		{"long label", "roof: r\npillars:\n  - {title: 人, label: 三文字}\n  - {title: 場}\n"},
		{"one pillar", "roof: r\npillars:\n  - {title: 人}\n"},
		{"seven pillars", "roof: r\npillars:\n" + strings.Repeat("  - {title: 人}\n", 7)},
		{"unknown key", "roof: r\nwidth: 3\npillars:\n  - {title: 人}\n  - {title: 場}\n"},
		{"no roof", "pillars:\n  - {title: 人}\n  - {title: 場}\n"},
	} {
		_, errs := mdToAny(t, "# A\n\n```pillars\n"+c.body+"```\n")
		if len(errs) == 0 {
			t.Errorf("markdown %s: no error", c.name)
		}
	}
	// 数字だけの名前は引用しても・しなくても文字列。
	if _, errs := mdToAny(t, "# A\n\n```pillars\nroof: r\npillars:\n  - {title: 人, label: 1}\n  - {title: 場, label: \"2\"}\n```\n"); len(errs) > 0 {
		t.Errorf("numeric labels: %+v", errs)
	}
}

// 値の表:0 以下・割合 1 以上・高さの順の誤りは読み込みで誤り。
func TestPillarsValuesChecked(t *testing.T) {
	for _, c := range []struct {
		key  string
		val  float64
		want string
	}{
		{"roofSlant", 0, "subsystems.pillars.roofSlant: must be greater than 0"},
		{"pillarMinWidth", -4, "subsystems.pillars.pillarMinWidth: must be greater than 0"},
		{"pillarTint", 1, "subsystems.pillars.pillarTint: must be less than 1"},
		{"pillarMinHeight", 400, "pillarMinHeight ≤ pillarNaturalHeight"},
	} {
		raw, _ := valueFiles.ReadFile("subsystems/pillars.json")
		var v map[string]any
		json.Unmarshal(raw, &v)
		v[c.key] = c.val
		b := mutate(t, func(m map[string]any) { m["subsystems"] = map[string]any{"pillars": v} })
		if _, err := parseTemplateFile("light", b); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s=%v: err = %v (want %q)", c.key, c.val, err, c.want)
		}
	}
}
