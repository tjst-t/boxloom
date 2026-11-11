package boxloom

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func ganttForLabelTest(t *testing.T, label, to string) *GanttSubsystem {
	t.Helper()
	data := `{"span":{"from":"2026-01","to":"2030-12"},"groups":[{"id":"g","label":["G"],"rows":[
	  {"label":"r","segments":[{"from":"2026-01","to":"` + to + `","status":"support","label":"` + label + `"}]}]}]}`
	s, err := NewGanttSubsystem(json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return s.(*GanttSubsystem)
}

// 区間の文は、帯に1行で収まれば従来どおり、収まらなければ帯の中で折り返し
// (3行まで)、それでも置けなければ描かずに警告する。
func TestSegmentLabelModes(t *testing.T) {
	const boxW = 900.0
	cases := []struct {
		label, to string
		want      int
	}{
		{"短い文", "2028-06", segLabelFit},
		{"帯より長い文は帯の中で折り返して行を高くする", "2026-12", segLabelWrap},
		{"帯が狭すぎて置けない文", "2026-03", segLabelHidden},
		{"", "2026-03", segLabelNone},
	}
	for _, c := range cases {
		g := ganttForLabelTest(t, c.label, c.to)
		gv := g.vals()
		chartW := boxW - 2*gv.Padding - (gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW)
		lp := planSegmentLabel(gv, g.Chart, chartW, g.Chart.Groups[0].Rows[0].Segments[0])
		if lp.mode != c.want {
			t.Errorf("%q to %s: mode %d, want %d", c.label, c.to, lp.mode, c.want)
		}
		warns := g.Lint(LBox{Width: boxW, Height: 300})
		if hidden := len(warns) == 1 && strings.Contains(warns[0], "is not drawn"); hidden != (c.want == segLabelHidden) {
			t.Errorf("%q: Lint = %v", c.label, warns)
		}
	}
}

// 折り返す文が行に収まらないなら、その行を高くする(Measure の自然な高さが増える)。
func TestSegmentLabelWrapGrowsRow(t *testing.T) {
	short := ganttForLabelTest(t, "短い", "2026-12")
	long := ganttForLabelTest(t, "とても長い文を三行に折り返すので行を高くする必要があるはずの文", "2026-12")
	c := Constraint{Width: 900}
	hs, hl := short.Measure(c).Natural.Height, long.Measure(c).Natural.Height
	if !(hl > hs) {
		t.Fatalf("natural height: long %v, short %v (want long > short)", hl, hs)
	}
	gv := long.vals()
	chartW := 900 - 2*gv.Padding - (gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW)
	lp := planSegmentLabel(gv, long.Chart, chartW, long.Chart.Groups[0].Rows[0].Segments[0])
	if lp.mode != segLabelWrap || hl-hs < lp.rowH-gv.NaturalRowH-0.01 {
		t.Fatalf("mode %d rowH %v, grew %v", lp.mode, lp.rowH, hl-hs)
	}
}

// 例の IR/Markdown:誤り0件で gantt-segment-label の警告が、置けない区間を指して出る。
func TestCheckGanttSegmentLabelExample(t *testing.T) {
	const want = "/slides/0/body/0/data/groups/0/rows/2/segments/0"
	for _, f := range []string{"../examples/ir/check-warnings/gantt-segment-label.json", "../examples/md/warnings/gantt-segment-label.bx.md"} {
		errs, warns := CheckFile(f, "")
		var got []Issue
		for _, w := range warns {
			if w.Code == "gantt-segment-label" {
				got = append(got, w)
			}
		}
		if len(errs) != 0 || len(got) != 1 || got[0].Path != want {
			t.Fatalf("%s: errs %+v warns %+v", f, errs, warns)
		}
		if strings.HasSuffix(f, ".md") {
			src, _ := os.ReadFile(f)
			if l := lineOf(string(src), "帯が狭すぎて置けない文"); got[0].Line != l {
				t.Fatalf("line %d, want %d", got[0].Line, l)
			}
		}
	}
}

// 目盛りと節目の日付は値の表の dateFormat で書く。月を含まない形・崩れた年は誤り。
func TestGanttDateFormat(t *testing.T) {
	d := time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC)
	for f, want := range map[string]string{"YYYY/M": "2026/12", "YY/M": "26/12", "YYYY年M月": "2026年12月", "YYYY-MM": "2026-12"} {
		if got := formatGanttDate(d, f); got != want {
			t.Errorf("formatGanttDate(%q) = %q, want %q", f, got, want)
		}
		if err := checkDateFormat("gantt", f); err != nil {
			t.Errorf("checkDateFormat(%q) = %v", f, err)
		}
	}
	for _, f := range []string{"", "YYYY", "YYY/M"} {
		if checkDateFormat("gantt", f) == nil {
			t.Errorf("checkDateFormat(%q) accepted a bad format", f)
		}
	}
	m := Milestone{Date: d, Label: "外部監査", AutoDate: true}
	if got := m.text(defaultGanttValues()); got != "2026/12 外部監査" {
		t.Errorf("milestone = %q", got)
	}
}
