package boxloom

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const cycleData = `{"start": 0, "steps": [
  {"title": "場を知る", "detail": "住民の声を集める"},
  {"title": "仲間を集める"},
  {"title": "小さく試す"},
  {"title": "形にする", "status": "conditional"},
  {"title": "ふり返る"}], "legend": {"conditional": "予算が付くときだけ"}}`

// cycle は IR と Markdown(```cycle)で同じ data になり、段の数(3〜6)・start(0 か 1)の
// 外は誤り。
func TestCycleReads(t *testing.T) {
	md := "---\ntitle: T\n---\n\n# 輪\n\n```cycle\nstart: 0\nsteps:\n  - {title: 場を知る, detail: 住民の声を集める}\n  - title: 仲間を集める\n" +
		"  - title: 小さく試す\n  - {title: 形にする, status: conditional}\n  - title: ふり返る\nlegend:\n  conditional: 予算が付くときだけ\n```\n"
	v, errs := mdToAny(t, md)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	got := v.(map[string]any)["slides"].([]any)[1].(map[string]any)["body"].([]any)[0].(map[string]any)
	var want any
	json.Unmarshal([]byte(cycleData), &want)
	if got["subsystem"] != "cycle" || !reflect.DeepEqual(got["data"], want) {
		t.Fatalf("markdown data = %#v", got)
	}
	sub := figureFromIR(t, "light", "cycle", cycleData).(*CycleSubsystem)
	if sub.Data.Start != 0 || len(sub.Data.Steps) != 5 || sub.Data.Steps[3].Status != "conditional" {
		t.Fatalf("data = %+v", sub.Data)
	}
	if d := figureFromIR(t, "light", "cycle", `{"steps": [{"title": "a"}, {"title": "b"}, {"title": "c"}]}`).(*CycleSubsystem); d.Data.Start != 1 {
		t.Errorf("start defaults to 1, got %d", d.Data.Start)
	}
	for data, msg := range map[string]string{
		`{"steps": [{"title": "a"}, {"title": "b"}]}`: "steps needs 3 to 6 steps",
		`{"steps": [{"title": "a"}, {"title": "b"}, {"title": "c"}, {"title": "d"}, {"title": "e"}, {"title": "f"}, {"title": "g"}]}`: "steps needs 3 to 6 steps",
		`{"start": 2, "steps": [{"title": "a"}, {"title": "b"}, {"title": "c"}]}`:                                                     "start: must be 0 or 1",
		`{"steps": [{"title": "a"}, {"title": "b"}, {"title": "c", "x": 1}]}`:                                                         "unknown field",
		`{"steps": [{"title": "a"}, {"title": "b"}, {"title": "c", "status": "risk"}]}`:                                               "unknown status",
	} {
		if _, err := mustTemplate("light").Subsystems.Build("cycle", json.RawMessage(data)); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%s: err = %v, want %q", data, err, msg)
		}
	}
	head := "---\ntitle: T\n---\n\n# 輪\n\n```cycle\n"
	for src, msg := range map[string]string{
		"steps:\n  - title: a\n  - title: b\n": "3 つ以上要ります",
		"steps:\n  - title: a\n  - title: b\n  - title: c\n  - title: d\n  - title: e\n  - title: f\n  - title: g\n": "6 つまで",
		"start: 2\nsteps:\n  - title: a\n  - title: b\n  - title: c\n":                                               "0 か 1",
		"steps:\n  - title: a\n  - title: b\n  - title: c\nangle: 30\n":                                              "angle",
	} {
		_, errs := mdToAny(t, head+src+"```\n")
		if len(errs) == 0 || !strings.Contains(errs[0].Message, msg) {
			t.Errorf("%q: errs = %+v, want %q", src, errs, msg)
		}
	}
}

// 扇は本物の blockArc で、上から時計回りに等しく分かれ、基調色の濃淡で塗る(最初が
// 基調色)。番号の丸は start から、題は扇の中(輪の外接の正方形の中)に、補足は輪の外に。
// 状態は扇の枠線。
func TestCycleDraws(t *testing.T) {
	tm := mustTemplate("light")
	sub := figureFromIR(t, "light", "cycle", cycleData)
	p := figureSlide(t, "light", SubsystemNode(sub, WithGrow(1)), "cycle")
	var arcs, rects, ovals []shape
	for _, s := range p.Shapes {
		switch {
		case s.prst == "blockArc":
			arcs = append(arcs, s)
		case s.prst == "ellipse":
			ovals = append(ovals, s)
		case s.prst == "rect" && s.text != "":
			rects = append(rects, s)
		}
	}
	if len(arcs) != 5 || len(ovals) != 5 {
		t.Fatalf("want 5 sectors and 5 number circles, got %d and %d", len(arcs), len(ovals))
	}
	if arcs[0].fill != tm.Theme.c("accent") {
		t.Errorf("first sector must be the accent color: %s", arcs[0].fill)
	}
	seen := map[string]bool{}
	for i, a := range arcs {
		if seen[a.fill] {
			t.Errorf("sector %d repeats the color %s; each step is its own shade", i, a.fill)
		}
		seen[a.fill] = true
		if len(a.adj) != 3 {
			t.Fatalf("sector %d: adj = %+v", i, a.adj)
		}
		// 上(270°)から 72° ずつ。境目に gapAngle/2 の隙間。
		start := math.Mod(270+72*float64(i)+1, 360) * 60000
		if math.Abs(float64(a.adj[0].Val)-start) > 1 {
			t.Errorf("sector %d starts at %d, want %.0f", i, a.adj[0].Val, start)
		}
		if a.box != arcs[0].box || math.Abs(a.box.Width-a.box.Height) > 0.01 {
			t.Errorf("all sectors share one square ring box: %+v", a.box)
		}
		if (i == 3) != a.hasOutline {
			t.Errorf("sector %d outline = %v; only the conditional step has one", i, a.hasOutline)
		}
	}
	if arcs[3].outlineColor != tm.Theme.c("conditional") {
		t.Errorf("status outline = %s", arcs[3].outlineColor)
	}
	for i, o := range ovals {
		if want := string(rune('0' + i)); o.text != want {
			t.Errorf("number %d = %q, want %q (start 0)", i, o.text, want)
		}
	}
	ring := arcs[0].box
	titles := map[string]LBox{}
	for _, r := range rects {
		titles[r.text] = r.box
	}
	for _, s := range []string{"場を知る", "仲間を集める", "小さく試す", "形にする", "ふり返る"} {
		b, ok := titles[s]
		if !ok {
			t.Errorf("title %q is not drawn", s)
			continue
		}
		if !within(ring, b) {
			t.Errorf("title %q (%+v) must be inside the ring %+v", s, b, ring)
		}
	}
	if b, ok := titles["住民の声を集める"]; !ok || within(ring, b) {
		t.Errorf("detail must be outside the ring: %+v", b)
	}
	if !strings.Contains(strings.Join(func() []string {
		var o []string
		for _, s := range p.Shapes {
			o = append(o, s.text)
		}
		return o
	}(), "|"), "予算が付くときだけ") {
		t.Error("legend text is missing")
	}
	if w := sub.(*CycleSubsystem).Lint(LBox{X: 0, Y: 0, Width: 600, Height: 400}); len(w) != 0 {
		t.Errorf("unexpected warnings: %v", w)
	}

	// 強調があれば、強調の段だけが基調色で、ほかは同じ淡い色。
	emph := figureFromIR(t, "light", "cycle", `{"steps": [{"title": "a"}, {"title": "b", "emphasis": true}, {"title": "c"}, {"title": "d"}]}`)
	p = figureSlide(t, "light", SubsystemNode(emph, WithGrow(1)), "cycle-emphasis")
	var fills []string
	for _, s := range p.Shapes {
		if s.prst == "blockArc" {
			fills = append(fills, s.fill)
		}
	}
	if len(fills) != 4 || fills[1] != tm.Theme.c("accent") || fills[0] != fills[2] || fills[0] != fills[3] || fills[0] == fills[1] {
		t.Errorf("emphasis fills = %v", fills)
	}
}

// 本物の図形で書く:扇は prstGeom blockArc に3つの調整値。
func TestCycleWritesBlockArc(t *testing.T) {
	sub := figureFromIR(t, "light", "cycle", cycleData)
	p := figureSlide(t, "light", SubsystemNode(sub, WithGrow(1)), "cycle-xml")
	out := filepath.Join(t.TempDir(), "c.pptx")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteDeck(f, []*PPTXBuilder{p}, "c"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	x := slideXML(t, out, "ppt/slides/slide1.xml")
	if n := strings.Count(x, `<a:prstGeom prst="blockArc"><a:avLst><a:gd name="adj1" fmla="val `); n != 5 {
		t.Errorf("want 5 blockArc shapes with adjust values, got %d", n)
	}
}

// 形を保つ(FixedAspect):広い枠では高さで輪が決まり、横の余りは spare-width。
// 狭い枠では幅で決まる。最小より狭いと最小を申告する。高さ 360 は題が扇に入る大きさ
// (題を輪の外に置く高さでは、その幅で測り直すと扇に入る輪になり、往復しない)。
func TestCycleMeasure(t *testing.T) {
	sub := figureFromIR(t, "light", "cycle", cycleData)
	m := sub.Measure(Constraint{Height: 360})
	if !m.FixedAspect || m.Natural.Width <= 0 || m.Natural.Width >= 600 {
		t.Fatalf("height 360: %+v", m)
	}
	if h := sub.Measure(Constraint{Width: m.Natural.Width}).Natural.Height; math.Abs(h-360) > 1 {
		t.Errorf("the width for height 360 must give back height 360, got %.1f", h)
	}
	if m.Min.Width <= 0 || m.Min.Width > 400 || m.Min.Height <= 0 {
		t.Errorf("min = %+v", m.Min)
	}
	w, h := fitBox(sub, LBox{Width: 880, Height: 360})
	if math.Abs(h-360) > 1 || w > 600 {
		t.Errorf("fit in a wide box = %.1f x %.1f; the ring is limited by the height", w, h)
	}
}

// cycle は並列のまとまり(columns)の中に置ける(左に輪・右に段ごとの説明)。本文に
// 1つだけ置くと横が余り、spare-width になる。process はまとまりの中に置けない。
func TestCycleInColumns(t *testing.T) {
	steps := "steps:\n  - title: 場を知る\n  - title: 仲間を集める\n  - title: 小さく試す\n  - title: 形にする\n  - title: ふり返る\n"
	alone := "# 輪だけ\n\n```cycle\n" + steps + "```\n"
	_, warns, _ := checkMD(t, alone)
	if !hasWarn(warns, "spare-width", "/slides/0/body/0") {
		t.Errorf("a lone cycle must warn spare-width: %+v", warns)
	}
	cols := "# 輪と説明\n\n::::columns\n:::column\n```cycle\n" + steps + "```\n:::\n:::column 段ごとの要点\n" +
		"- **場を知る**:住民に話を聞く\n- **仲間を集める**:担い手を決める\n- **小さく試す**:催しを続ける\n- **形にする**:決まりを書く\n- **ふり返る**:次の年へ渡す\n:::\n::::\n"
	for _, tmpl := range []string{"light", "report"} {
		path := filepath.Join(t.TempDir(), "c.bx.md")
		if err := os.WriteFile(path, []byte(cols), 0o644); err != nil {
			t.Fatal(err)
		}
		errs, warns, _ := CheckFileCapacity(path, tmpl)
		if len(errs) > 0 {
			t.Fatalf("%s: errors %+v", tmpl, errs)
		}
		for _, w := range warns {
			if w.Code == "subsystem-min" || w.Code == "figure-text-overflow" || w.Code == "layout-overflow" {
				t.Errorf("%s: %+v", tmpl, w)
			}
		}
	}
	ir := `{"boxloom":"0.1","slides":[{"type":"content","title":"A","body":[{"type":"columns","columns":[` +
		`{"body":[{"type":"subsystem","subsystem":"cycle","data":{"steps":[{"title":"a"},{"title":"b"},{"title":"c"}]}}]},` +
		`{"body":[{"type":"paragraph","content":"b"}]}]}]}]}`
	if _, err := ReadIR("t.json", []byte(ir)); err != nil {
		t.Fatalf("cycle in a column must be read: %v", err)
	}
}

// 題が扇に収まらず、輪の外に置いても枠に入らなければ警告(figure-text-overflow)、状態に
// 凡例の文が無ければ figure-legend。
func TestCycleWarnings(t *testing.T) {
	long := figureFromIR(t, "light", "cycle", `{"steps": [{"title": "とても長い段の名前をここに書いてしまった例"}, {"title": "b"}, {"title": "c", "status": "danger"}]}`)
	w := strings.Join(long.(*CycleSubsystem).Lint(LBox{Width: 220, Height: 200}), "\n")
	if !strings.Contains(w, "cycle: steps[0]") || !strings.Contains(w, "text does not fit") {
		t.Errorf("long title must warn: %s", w)
	}
	if !strings.Contains(w, `legend: status "danger"`) {
		t.Errorf("status without legend must warn: %s", w)
	}
	is := warningIssue("x.json", "slide 2: [SUBSYSTEM WARNING] body[0]: cycle: steps[0] (\"x\"): text does not fit (needs 1x1pt, has 0x0pt)")
	if is.Code != "figure-text-overflow" || is.Path != "/slides/1/body/0/data/steps/0" {
		t.Errorf("issue = %+v", is)
	}
}
