package boxloom

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// Gantt の月の格子(axisStyle: monthGrid)・山形の帯(barShape: chevron)・高さを使い切る
// (fillHeight)の試験(Backlog #89)。例の文は架空。

// ganttMonthGridDeck は架空の8か月(2026年9月〜2027年4月)の Gantt を1枚だけ持つ deck。
const ganttMonthGridDeck = "---\ntitle: 架空の計画\ncover: false\n---\n\n# 導入の計画\n\n```gantt\n" + `span: 2026-09 .. 2027-05
ticks:
  - {at: 2026-11, label: 中間報告, status: conditional}
legend:
  support: 確定
  conditional: 調整中
groups:
  - id: g1
    label: [準備]
    rows:
      - label: 要件の整理
        segments:
          - {from: 2026-09, to: 2026-11, status: support, label: 聞き取り}
      - label: 設計
        segments:
          - {from: 2026-10, to: 2027-01, status: conditional, label: 基本設計}
  - id: g2
    label: [実施]
    rows:
      - label: 構築
        segments:
          - {from: 2026-12, to: 2027-03, status: support, label: 構築と試験}
      - label: 移行
        note: {from: 2027-02, to: 2027-05, text: 日程は次の会議で決める}
      - label: 定着
        segments:
          - {from: 2027-03, to: 2027-05, status: support, label: 教育, tentative: true}
` + "```\n"

// buildGanttSlide は deck をテンプレート name で組み、本文の箱・Gantt の葉・描いた図形を返す。
func buildGanttSlide(t *testing.T, name, src string) (*Template, *Spec, *Spec, []shape) {
	t.Helper()
	deck, issues, err := ParseMarkdown("gantt.bx.md", []byte(src))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	tm, err := LookupTemplate(name)
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	if len(built) != 1 {
		t.Fatalf("%d slides", len(built))
	}
	root := built[0].Root
	computeLayout(root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	p := NewPPTXBuilder(tm.Values.PageW, tm.Values.PageH)
	roles := newCheckedRoles(tm.Theme)
	renderSlide(RenderContext{Out: p, Roles: roles}, root)
	if m := roles.Missing(); len(m) > 0 {
		t.Fatalf("missing roles %v", m)
	}
	var body, leaf *Spec
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.frame == frameBody {
			body = s
		}
		if _, ok := s.Subsystem.(*GanttSubsystem); ok {
			leaf = s
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(root)
	if body == nil || leaf == nil {
		t.Fatalf("body %v, gantt %v", body != nil, leaf != nil)
	}
	return tm, body, leaf, p.Shapes
}

func textsOf(shapes []shape) map[string][]shape {
	out := map[string][]shape{}
	for _, s := range shapes {
		if s.kind == skText && s.text != "" {
			out[s.text] = append(out[s.text], s)
		}
	}
	return out
}

// report:月ごとに列(8列)・年の行(2026年・2027年)と月の行の2段・年の境目の罫線・
// 山形の帯・本文の高さを使い切る。
func TestReportGanttMonthGrid(t *testing.T) {
	tm, body, leaf, shapes := buildGanttSlide(t, "report", ganttMonthGridDeck)
	vals := leaf.Subsystem.(*GanttSubsystem).vals()
	if !vals.monthGrid() || !vals.chevron() || !vals.FillHeight {
		t.Fatalf("report gantt values: axisStyle %q barShape %q fillHeight %v", vals.AxisStyle, vals.BarShape, vals.FillHeight)
	}
	texts := textsOf(shapes)

	// 月の列:9月〜4月の8つ。同じ幅で、左から順に並ぶ。
	months := []string{"9月", "10月", "11月", "12月", "1月", "2月", "3月", "4月"}
	var cols []shape
	for _, m := range months {
		if len(texts[m]) != 1 {
			t.Fatalf("month %q drawn %d times", m, len(texts[m]))
		}
		cols = append(cols, texts[m][0])
	}
	for i := 1; i < len(cols); i++ {
		a, b := cols[i-1].box, cols[i].box
		if !nearPt(a.X+a.Width, b.X) || !nearPt(a.Width, b.Width) {
			t.Errorf("month columns %d,%d: %+v %+v (want adjacent, same width)", i-1, i, a, b)
		}
	}

	// 年の行:2026年は9月の列、2027年は1月の列の左端から。月の行の上に置く。
	y26, y27 := texts["2026年"], texts["2027年"]
	if len(y26) != 1 || len(y27) != 1 {
		t.Fatalf("year labels: 2026 %d, 2027 %d", len(y26), len(y27))
	}
	jan := cols[4].box
	if !(y26[0].box.X >= cols[0].box.X && y26[0].box.X < cols[1].box.X) || !(y27[0].box.X >= jan.X && y27[0].box.X < jan.X+jan.Width) {
		t.Errorf("year labels at %.1f, %.1f; months start %.1f, January %.1f", y26[0].box.X, y27[0].box.X, cols[0].box.X, jan.X)
	}
	if !nearPt(y27[0].box.Y+y27[0].box.Height, jan.Y) || !nearPt(jan.Height, vals.GridRowH) {
		t.Errorf("year row %+v is not just above the month row %+v", y27[0].box, jan)
	}
	// 年の境目(1月の左端)の罫線は年の行から引く。ほかの月の境目は月の行から。
	var yearLine, monthLines int
	for _, s := range shapes {
		if s.kind != skLine || !nearPt(s.x1, s.x2) || s.lineColor != tm.Theme["grid"] {
			continue
		}
		top := math.Min(s.y1, s.y2)
		switch {
		case nearPt(s.x1, jan.X) && nearPt(top, y27[0].box.Y):
			yearLine++
		case nearPt(top, jan.Y):
			monthLines++
		}
	}
	if yearLine != 1 || monthLines != 6 {
		t.Errorf("year boundary rules %d (want 1), month rules %d (want 6)", yearLine, monthLines)
	}
	// 書いた目盛りの名前は格子の上に重ねる(日付の目盛りの文字は描かない)。
	if len(texts["中間報告"]) != 1 || len(texts["2026/9"]) != 0 {
		t.Errorf("tick labels: %d 中間報告, %d 2026/9", len(texts["中間報告"]), len(texts["2026/9"]))
	}

	// 帯は山形(homePlate)。確かな帯は状態の色を薄く塗り、仮の帯は破線の枠。
	var chevrons, filled int
	for _, s := range shapes {
		if s.kind == skShape && s.prst == "homePlate" && s.box.Width > vals.LegendSwatchW {
			chevrons++
			if s.fill != "" {
				filled++
				if s.fillAlpha != vals.ChevronFillOpacity {
					t.Errorf("chevron fill alpha %v", s.fillAlpha)
				}
			}
		}
		if s.kind == skRoundRect && s.box.Width < leaf.Box.Width-1 {
			t.Errorf("a rounded bar is drawn: %+v", s.box)
		}
	}
	if chevrons != 4 || filled != 3 {
		t.Errorf("chevrons %d (filled %d), want 4 (3)", chevrons, filled)
	}
	if bar := texts["聞き取り"]; len(bar) != 1 || bar[0].color != tm.Theme["text_primary"] {
		t.Errorf("segment label not in the text color: %+v", bar)
	}

	// 本文の領域の高さを使い切る(Measure の上限なし)。
	if !nearPt(leaf.Box.Height, body.Box.Height) {
		t.Errorf("gantt height %.1f, body %.1f", leaf.Box.Height, body.Box.Height)
	}
	if m := leaf.Subsystem.Measure(Constraint{Width: leaf.Box.Width}); m.Max.Height != 0 {
		t.Errorf("Max.Height = %v, want 0 (no limit)", m.Max.Height)
	}
	// 余りは注記の行を除く行に等しく配る。注記の行は自然な高さのまま。
	g := leaf.Subsystem.(*GanttSubsystem)
	f := ganttFrameFor(vals, g.chart(leaf.Box.Width), leaf.Box)
	if !(f.rows.base > vals.NaturalRowH) || f.rows.base > vals.MaxRowH+1e-9 {
		t.Errorf("row height %.1f (natural %v, max %v)", f.rows.base, vals.NaturalRowH, vals.MaxRowH)
	}
	for gi, want := range [][]float64{{f.rows.base, f.rows.base}, {f.rows.base, vals.NaturalRowH, f.rows.base}} {
		for ri, w := range want {
			if h := f.rows.h(gi, ri); !nearPt(h, w) {
				t.Errorf("row %d/%d height %.1f, want %.1f", gi, ri, h, w)
			}
		}
	}
}

// 行が多いと、行の高さは上限(maxRowHeight)に届かず、行の本体が Gantt の下端まで届く。
func TestReportGanttFillsWithRows(t *testing.T) {
	var rows strings.Builder
	for i := 0; i < 12; i++ {
		rows.WriteString("      - label: 作業\n        segments:\n          - {from: 2026-09, to: 2027-01, status: support}\n")
	}
	src := "---\ntitle: 架空\ncover: false\n---\n\n# 計画\n\n```gantt\nspan: 2026-09 .. 2027-05\ngroups:\n  - label: [全体]\n    rows:\n" + rows.String() + "```\n"
	_, body, leaf, _ := buildGanttSlide(t, "report", src)
	vals := leaf.Subsystem.(*GanttSubsystem).vals()
	if !nearPt(leaf.Box.Height, body.Box.Height) {
		t.Fatalf("gantt height %.1f, body %.1f", leaf.Box.Height, body.Box.Height)
	}
	f := ganttFrameFor(vals, leaf.Subsystem.(*GanttSubsystem).chart(leaf.Box.Width), leaf.Box)
	if !(f.rows.base > vals.NaturalRowH && f.rows.base < vals.MaxRowH) || !nearPt(f.rowsEnd, f.bodyTop+f.bodyH) {
		t.Errorf("row %.1f; rows end %.1f, body ends %.1f", f.rows.base, f.rowsEnd, f.bodyTop+f.bodyH)
	}
}

// dark・light は従来どおり:目盛りの軸・角の丸い帯・上限のある高さ。
func TestGanttDefaultStyleUnchanged(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		_, body, leaf, shapes := buildGanttSlide(t, name, ganttMonthGridDeck)
		vals := leaf.Subsystem.(*GanttSubsystem).vals()
		if vals.monthGrid() || vals.chevron() || vals.FillHeight {
			t.Fatalf("%s: %+v", name, vals)
		}
		for _, s := range shapes {
			if s.prst == "homePlate" {
				t.Errorf("%s: chevron drawn", name)
			}
		}
		texts := textsOf(shapes)
		if len(texts["2026/9"]) != 1 || len(texts["10月"]) != 0 {
			t.Errorf("%s: tick axis not drawn as before", name)
		}
		if m := leaf.Subsystem.Measure(Constraint{Width: leaf.Box.Width}); m.Max.Height <= 0 || leaf.Box.Height > m.Max.Height+1e-6 || leaf.Box.Height >= body.Box.Height {
			t.Errorf("%s: gantt height %.1f, max %.1f, body %.1f", name, leaf.Box.Height, m.Max.Height, body.Box.Height)
		}
	}
}

// 長い span(等しい幅では月の文字が列に入らない)は、書いたとおりの軸のまま年の行だけ。
func TestGanttMonthGridLongSpan(t *testing.T) {
	data := `{"span":{"from":"2026-01","to":"2034-01"},"groups":[{"label":["G"],"rows":[
	  {"label":"r","segments":[{"from":"2026-01","to":"2030-12","status":"support"}]}]}]}`
	s, err := NewGanttSubsystem(json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	g := s.(*GanttSubsystem)
	gv := *defaultGanttValues()
	gv.AxisStyle = "monthGrid"
	g.V = &gv
	if g.chart(900) != g.Chart {
		t.Fatal("a long span should keep the written axis")
	}
	if gridShowMonths(&gv, g.Chart, ganttChartW(&gv, 900)) {
		t.Fatal("96 months should not show month labels at 900pt")
	}
	if h := ganttAxisH(&gv, g.Chart, 900); !nearPt(h, gv.GridRowH) {
		t.Errorf("axis height %.1f, want one grid row %.1f", h, gv.GridRowH)
	}
}

// 値の表の誤り:知らない描き方・不透明度の範囲・年と月の書き方。
func TestGanttStyleValuesChecked(t *testing.T) {
	base := *defaultGanttValues()
	for _, c := range []struct {
		edit func(*GanttValues)
		want string
	}{
		{func(v *GanttValues) { v.AxisStyle = "grid" }, "axisStyle"},
		{func(v *GanttValues) { v.BarShape = "arrow" }, "barShape"},
		{func(v *GanttValues) { v.ChevronFillOpacity = 1.5 }, "chevronFillOpacity"},
		{func(v *GanttValues) { v.GridYearFormat = "M" }, "gridYearFormat"},
		{func(v *GanttValues) { v.GridMonthFormat = "YYYY" }, "gridMonthFormat"},
	} {
		v := base
		c.edit(&v)
		if err := checkGanttStyle("gantt", v); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.want, err)
		}
	}
	if err := checkGanttStyle("gantt", base); err != nil {
		t.Error(err)
	}
}
