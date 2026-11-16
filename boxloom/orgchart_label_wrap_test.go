package boxloom

import (
	"strings"
	"testing"
)

// 組織図の細い箱で名前が3行以上に折れるときの警告(orgchart-label-wrap、#176)。文はすべて架空。

// orgLabelWrapIssues は幅 width の枠に置いた図の Lint を check の Issue に直し、
// orgchart-label-wrap だけを返す(ほかの警告は others)。
func orgLabelWrapIssues(t *testing.T, tmpl, data string, width float64) (wraps []Issue, others []string) {
	t.Helper()
	tm := mustTemplate(tmpl)
	defer useFonts(tm.Fonts)()
	sub := figureFromIR(t, tmpl, "orgchart", data).(*OrgChartSubsystem)
	m := sub.Measure(Constraint{Width: width})
	box := LBox{X: 24, Y: 80, Width: width, Height: m.Natural.Height}
	for _, w := range sub.Lint(box) {
		is := warningIssue("d.json", "slide 1: [SUBSYSTEM WARNING] body[0]: "+strings.TrimPrefix(w, "[SUBSYSTEM WARNING] "))
		if is.Code == "orgchart-label-wrap" {
			wraps = append(wraps, is)
		} else {
			others = append(others, w)
		}
	}
	return wraps, others
}

// #161 の R7 と同じ形(4つの部門、1つの部門に2つの係)を本文の幅(912pt)に置く。係の名前が
// 長く細い箱の中で4行に折れる → 警告が1件(path はその係、文に行の数、hint は名前を短くする)。
// 名前が2行以内なら出ない。
func TestOrgChartLabelWrapWarning(t *testing.T) {
	const data = `{"root": {"title": "図書館長", "detail": "職員 60 人"}, "units": [
	  {"title": "中央館", "detail": "22 人", "members": [{"title": "選書と目録"}, {"title": "貸し出しと相談"}, {"title": "電子と広報"}]},
	  {"title": "駅前館", "detail": "8 人", "members": [{"title": "貸し出し"}, {"title": "予約の受け取り"}]},
	  {"title": "地区館", "detail": "28 人", "members": [{"title": "北館と東館", "detail": "15 人"}, {"title": "%s", "detail": "15 人"}]},
	  {"title": "事務局", "detail": "2 人"}]}`
	// 4行に折れる名前(report は係の字が小さいので長めの名前)。
	longName := map[string]string{
		"light": "西館と南館(学習室と子どもの本の多い館)", "dark": "西館と南館(学習室と子どもの本の多い館)",
		"report": "西館と南館(学習室と子どもの本の利用の多い二つの館)"}
	short := strings.Replace(data, "%s", "西館と南館", 1)
	const width = 912.0
	for _, tmpl := range []string{"light", "dark", "report"} {
		long := strings.Replace(data, "%s", longName[tmpl], 1)
		wraps, others := orgLabelWrapIssues(t, tmpl, long, width)
		if len(wraps) != 1 {
			t.Fatalf("%s: orgchart-label-wrap %d times, want 1 (%v, others %v)", tmpl, len(wraps), wraps, others)
		}
		w := wraps[0]
		if w.Path != "/slides/0/body/0/data/units/2/members/1" {
			t.Errorf("%s: path %q", tmpl, w.Path)
		}
		if !strings.Contains(w.Message, "wraps to 4 lines in a narrow box") {
			t.Errorf("%s: message %q", tmpl, w.Message)
		}
		hinted := []Issue{w}
		addLayoutHints(hinted, nil, TemplateValues{})
		if !strings.Contains(hinted[0].Hint, "名前を短く") {
			t.Errorf("%s: hint %q", tmpl, hinted[0].Hint)
		}
		t.Logf("%s: %s", tmpl, w.Message)

		if wraps, others := orgLabelWrapIssues(t, tmpl, short, width); len(wraps) != 0 || len(others) != 0 {
			t.Errorf("%s: short name warned %v %v", tmpl, wraps, others)
		}
	}
}

// 行の数を直接確かめる:4行に折れる名前は出し、2行の名前は出さない。
func TestOrgChartLabelWrapLines(t *testing.T) {
	fv := figureBoxValues{insetX: 4, insetY: 4}
	n := &FigureNode{Title: Inline{{Text: "地域連携と新規事業の共同開発を進める推進室"}}}
	defer useFonts(mustTemplate("light").Fonts)()
	size := 12.0
	w4 := 0.0
	for x := 20.0; x < 400; x++ {
		if l := len(wrapRuns(itemRuns(n.Title, "", nil), size, x-2*fv.insetX)); l == 4 {
			w4 = x
			break
		}
	}
	if w4 == 0 {
		t.Fatal("no width gives 4 lines")
	}
	if got := orgLabelWrapWarning("units[0]", n, size, w4, fv); !strings.Contains(got, "wraps to 4 lines") {
		t.Errorf("4 lines: %q", got)
	}
	two := 0.0
	for x := w4; x < 600; x++ {
		if l := len(wrapRuns(itemRuns(n.Title, "", nil), size, x-2*fv.insetX)); l == 2 {
			two = x
			break
		}
	}
	if got := orgLabelWrapWarning("units[0]", n, size, two, fv); got != "" {
		t.Errorf("2 lines warned: %q", got)
	}
}
