package boxloom

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

const phasesPath = "../examples/ir/phases.json"

// phasesExample は例の IR の phases のブロックを、テンプレート tmpl で作る。
func phasesExample(t *testing.T, tmpl string) *PhasesSubsystem {
	t.Helper()
	d := mustReadIR(t, phasesPath)
	for _, sl := range d.Slides {
		for _, b := range sl.Body {
			if b.Type == "subsystem" && b.Subsystem == "phases" {
				return figureFromIR(t, tmpl, "phases", string(b.Data)).(*PhasesSubsystem)
			}
		}
	}
	t.Fatal("no phases block in the example")
	return nil
}

// 例(3段・群2つ・箱7つ)は report・light・dark で警告なしに組める。山形は段の数
// (左端は五角形の矢印)、群の帯は2つ、箱の番号は段をまたいで通しで、report は
// ローマ数字、light は算用数字。
func TestPhasesExampleBuilds(t *testing.T) {
	d := mustReadIR(t, phasesPath)
	nums := map[string][]string{
		"report": {"I. ", "II. ", "III. ", "IV. ", "V. ", "VI. ", "VII. "},
		"light":  {"1. ", "2. ", "3. ", "4. ", "5. ", "6. ", "7. "},
		"dark":   {"1. ", "7. "},
	}
	for name, want := range nums {
		out := filepath.Join(t.TempDir(), name+".pptx")
		warnings, err := renderDeckToFile(mustTemplate(name), d, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warnings)
		}
		x := slideXML(t, out, "ppt/slides/slide2.xml")
		if n := strings.Count(x, `prst="homePlate"`); n != 1 {
			t.Errorf("%s: %d homePlate shapes, want 1", name, n)
		}
		if n := strings.Count(x, `prst="chevron"`); n != 2 {
			t.Errorf("%s: %d chevron shapes, want 2", name, n)
		}
		for _, s := range []string{"① 調べと案づくり", "② 報告書", "Step1 現状を知る", "4月〜6月", "追加の検討"} {
			if !strings.Contains(x, s) {
				t.Errorf("%s: slide lacks %q", name, s)
			}
		}
		for _, s := range want {
			if !strings.Contains(x, ">"+s+"<") {
				t.Errorf("%s: slide lacks the number %q", name, s)
			}
		}
		if !strings.Contains(x, `<a:buChar char="・"/>`) {
			t.Errorf("%s: detail lists should be paragraph bullets", name)
		}
	}
}

// 置き方: 山形は段の列に並び、次の山形の切り欠きに食い込む。群の帯は段の列の
// 端から端まで。箱は枠の中に重ならずに収まり、どの段も箱の列の下の端がそろう。
// 高さが増えると箱が高くなる(余りは箱に配る)。
func TestPhasesPlacement(t *testing.T) {
	sub := phasesExample(t, "report")
	v := sub.vals()
	for _, h := range []float64{340, 420, 900} {
		box := LBox{X: 24, Y: 80, Width: 912, Height: h}
		g, pl := sub.place(box)
		if len(pl.chevrons) != 3 || len(pl.bands) != 2 {
			t.Fatalf("chevrons %d bands %d", len(pl.chevrons), len(pl.bands))
		}
		for i := 0; i+1 < 3; i++ {
			a, b := pl.chevrons[i], pl.chevrons[i+1]
			if math.Abs(a.X+a.Width-(b.X+v.ChevronDepth-v.ChevronGap)) > 1e-6 {
				t.Errorf("chevron %d tip %.2f does not nest in the next notch at %.2f", i, a.X+a.Width, b.X)
			}
		}
		// 群1 は s1〜s2、群2 は s3。
		if b := pl.bands[0]; math.Abs(b.X-pl.items[0][0].X) > 1e-6 || math.Abs(b.X+b.Width-(pl.items[1][0].X+g.colW)) > 1e-6 {
			t.Errorf("band 0 = %+v", b)
		}
		var bottoms []float64
		var all []LBox
		for i, col := range pl.items {
			last := col[len(col)-1]
			bottoms = append(bottoms, last.Y+last.Height)
			for _, b := range col {
				if !within(box, b) {
					t.Errorf("h=%v: step %d box %+v outside %+v", h, i, b, box)
				}
				all = append(all, b)
			}
		}
		for _, y := range bottoms[1:] {
			if math.Abs(y-bottoms[0]) > 1e-6 {
				t.Errorf("h=%v: column bottoms differ %v", h, bottoms)
			}
		}
		for i := range all {
			for j := i + 1; j < len(all); j++ {
				if boxesOverlap(all[i], all[j]) {
					t.Errorf("h=%v: boxes overlap %+v %+v", h, all[i], all[j])
				}
			}
		}
		if ws := sub.Lint(box); h >= 340 && len(ws) != 0 {
			t.Errorf("h=%v: warnings %v", h, ws)
		}
	}
	_, a := sub.place(LBox{Width: 912, Height: 360})
	_, b := sub.place(LBox{Width: 912, Height: 480})
	if !(b.items[2][1].Height > a.items[2][1].Height) {
		t.Errorf("extra height is not given to the boxes: %.1f vs %.1f", a.items[2][1].Height, b.items[2][1].Height)
	}
}

// Measure: 最小は一番文の多い段の箱が収まる高さで、文が増えると増える。
func TestPhasesMeasure(t *testing.T) {
	short := figureFromIR(t, "light", "phases", `{"steps": [
	  {"title": "A", "items": [{"title": "a"}]}, {"title": "B", "items": [{"title": "b"}]}]}`)
	long := figureFromIR(t, "light", "phases", `{"steps": [
	  {"title": "A", "items": [{"title": "a", "detail": ["一つ目の文", "二つ目の文", "三つ目の文", "四つ目の文"]}, {"title": "c"}]},
	  {"title": "B", "items": [{"title": "b"}]}]}`)
	for _, w := range []float64{400, 900} {
		ms, ml := short.Measure(Constraint{Width: w}), long.Measure(Constraint{Width: w})
		for _, m := range []Measurement{ms, ml} {
			if !(m.Min.Height <= m.Natural.Height && m.Natural.Height <= m.Max.Height) {
				t.Errorf("w=%v: min %.1f nat %.1f max %.1f", w, m.Min.Height, m.Natural.Height, m.Max.Height)
			}
		}
		if !(ml.Min.Height > ms.Min.Height) {
			t.Errorf("w=%v: min height does not grow with the text (%.1f vs %.1f)", w, ms.Min.Height, ml.Min.Height)
		}
	}
}

// 本文に収まらない phases は layout-overflow(はみ出し)になる。狭い枠に
// 描くと、折り返せない語の収まらない箱が figure-text-overflow(その箱を指す)になる。
func TestPhasesOverflow(t *testing.T) {
	var items []string
	for i := 0; i < 9; i++ {
		items = append(items, "      - title: 長い箱の題\n        detail:\n          - 一つ目の長めの補足の文を書く\n          - 二つ目の長めの補足の文を書く\n          - 三つ目の長めの補足の文を書く")
	}
	md := "---\ntitle: t\ntemplate: light\n---\n\n# 題\n\n```phases\nsteps:\n  - title: A\n    items:\n" + strings.Join(items, "\n") +
		"\n  - title: B\n    items:\n      - title: b\n```\n"
	res := Check("t.bx.md", []byte(md), CheckOptions{})
	if len(res.Errors) != 0 {
		t.Fatal(res.Errors)
	}
	overflow := false
	for _, w := range res.Warnings {
		if w.Code == "layout-overflow" && w.Path == "/slides/1/body/0" {
			overflow = true
		}
	}
	if !overflow {
		t.Errorf("warnings = %+v", res.Warnings)
	}

	// 日本語は文字ごとに折り返せるので、収まらないのは折り返せない長い語。
	sub := figureFromIR(t, "light", "phases", `{"steps": [
	  {"title": "A", "items": [{"title": "a"}, {"title": "Infrastructure-Operations"}]}, {"title": "B", "items": [{"title": "b"}]}]}`).(*PhasesSubsystem)
	ws := sub.Lint(LBox{Width: 260, Height: 400})
	if len(ws) == 0 {
		t.Fatal("no warning for a frame that is too narrow")
	}
	is := warningIssue("x.json", "slide 2: "+strings.Replace(ws[0], "WARNING] ", "WARNING] body[0]: ", 1))
	if is.Code != "figure-text-overflow" || is.Path != "/slides/1/body/0/data/steps/0/items/1" {
		t.Errorf("issue = %+v (%s)", is, ws[0])
	}
}

// 読み込みが断るもの。
func TestPhasesFromIRRejects(t *testing.T) {
	two := `{"title": "A", "id": "a", "items": [{"title": "x"}]}, {"title": "B", "id": "b", "items": [{"title": "y"}]}`
	for _, c := range []struct{ data, msg string }{
		{`{"steps": [{"title": "A", "items": [{"title": "x"}]}]}`, "steps needs 2 to 5 steps"},
		{`{"steps": [` + strings.Repeat(`{"title": "A", "items": [{"title": "x"}]},`, 5) + `{"title": "A", "items": [{"title": "x"}]}]}`, "steps needs 2 to 5 steps"},
		{`{"steps": [{"title": "A", "items": []}, {"title": "B", "items": [{"title": "y"}]}]}`, "steps[0].items: a step needs at least one item"},
		{`{"steps": [` + two + `], "groups": [{"title": "g", "from": "a", "to": "c"}]}`, `groups[0].to: no step has the id "c"`},
		{`{"steps": [` + two + `], "groups": [{"title": "g", "from": "b", "to": "a"}]}`, "comes before"},
		{`{"steps": [` + two + `], "groups": [{"title": "g", "from": "a", "to": "b"}, {"title": "h", "from": "b", "to": "b"}]}`, "must not overlap"},
		{`{"steps": [{"title": "A", "id": "a", "items": [{"title": "x"}]}, {"title": "B", "id": "a", "items": [{"title": "y"}]}]}`, "is used by another step"},
		{`{"steps": [{"title": "A", "items": [{"title": "x", "status": "accent"}]}, {"title": "B", "items": [{"title": "y"}]}]}`, "unknown status"},
		{`{"steps": [{"title": "A", "items": [{"title": "x", "x": 1}]}, {"title": "B", "items": [{"title": "y"}]}]}`, "unknown field"},
	} {
		_, err := mustTemplate("light").Subsystems.Build("phases", json.RawMessage(c.data))
		if err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: err = %v", c.data, err)
		}
	}
}

// detail は1つの文か文の列。列の項目に文中の要素が直に並ぶなら1つの文。
func TestPhasesDetailForms(t *testing.T) {
	sub := figureFromIR(t, "light", "phases", `{"steps": [
	  {"title": "A", "items": [{"title": "x", "detail": "一つの文"}, {"title": "y", "detail": ["箇条1", "箇条2"]},
	    {"title": "z", "detail": ["強調の", {"type": "em", "content": "ある"}, "文"]}]},
	  {"title": "B", "items": [{"title": "w"}]}]}`).(*PhasesSubsystem)
	its := sub.Data.Steps[0].Items
	if its[0].Points != nil || its[0].Detail.Plain() != "一つの文" {
		t.Errorf("single sentence: %+v", its[0])
	}
	if len(its[1].Points) != 2 {
		t.Errorf("list: %+v", its[1])
	}
	if its[2].Points != nil || len(its[2].Detail) != 3 {
		t.Errorf("inline with an element: %+v", its[2])
	}
	// Markdown: 列は文の列(改行でつながない)、1行は文。
	head := "---\ntitle: t\n---\n\n# 題\n\n```phases\nsteps:\n  - title: A\n    items:\n      - {title: x, detail: [a, b]}\n      - {title: y, detail: c}\n  - title: B\n    items: [{title: z}]\n```\n"
	ir, errs := MarkdownToIR("t.bx.md", []byte(head))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if s := string(compactJSON(t, ir)); !strings.Contains(s, `"detail":["a","b"]`) || !strings.Contains(s, `"detail":"c"`) {
		t.Errorf("ir = %s", s)
	}
	_, errs = MarkdownToIR("t.bx.md", []byte(strings.Replace(head, "{title: z}", "{title: z, color: accent}", 1)))
	if len(errs) == 0 || errs[0].Code != codeUnknownKey {
		t.Errorf("errs = %+v", errs)
	}
}

// 番号の形は値の表で選び、知らない形は誤り。
func TestPhasesNumberStyle(t *testing.T) {
	if phasesNumber("roman", 4) != "IV." || phasesNumber("arabic", 12) != "12." {
		t.Errorf("numbers: %q %q", phasesNumber("roman", 4), phasesNumber("arabic", 12))
	}
	v := *defaultPhasesValues()
	v.NumberStyle = "kanji"
	if err := v.check("subsystems.phases"); err == nil || !strings.Contains(err.Error(), "numberStyle") {
		t.Errorf("err = %v", err)
	}
	if mustTemplate("report").Subsystems == nil {
		t.Fatal("no registry")
	}
	if s := phasesExample(t, "report"); s.vals().NumberStyle != "roman" {
		t.Errorf("report numberStyle = %q", s.vals().NumberStyle)
	}
}
