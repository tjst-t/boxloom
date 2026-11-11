package boxloom

import (
	"math"
	"strings"
	"testing"
)

// fixedSub は申告する大きさを決め打ちにした試験用のサブシステム。
type fixedSub struct{ nat, min Size }

func (f fixedSub) Name() string { return "fixed" }
func (f fixedSub) Measure(c Constraint) Measurement {
	m := Measurement{Natural: f.nat, Min: f.min}
	if c.Width > 0 {
		m.Natural.Width = c.Width
	}
	return m
}
func (f fixedSub) Render(RenderContext, LBox) {}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// grow を付けなければ、サブシステムは自分の自然な高さをもらう。
func TestSubsystemGetsNaturalHeight(t *testing.T) {
	sub := SubsystemNode(fixedSub{nat: Size{100, 120}, min: Size{50, 80}})
	root := Col([]*Spec{TextNode("title", WithHeight(30)), sub}, WithGap(10))
	computeLayout(root, 0, 0, 400, 500)
	if !near(sub.Box.Height, 120) {
		t.Fatalf("height = %v, want 120 (natural)", sub.Box.Height)
	}
	if !near(sub.Box.Y, 40) {
		t.Fatalf("y = %v, want 40", sub.Box.Y)
	}
}

// 足りないときは、まずサブシステムが最小の大きさまで縮み、テキストは縮まない。
func TestSubsystemShrinksToMinBeforeText(t *testing.T) {
	title := TextNode("title", WithHeight(30))
	sub := SubsystemNode(fixedSub{nat: Size{100, 200}, min: Size{50, 100}})
	root := Col([]*Spec{title, sub}, WithGap(10))
	computeLayout(root, 0, 0, 400, 180) // 30 + 10 + 200 = 240 > 180
	if !near(title.Box.Height, 30) {
		t.Fatalf("title height = %v, want 30 (unchanged)", title.Box.Height)
	}
	if !near(sub.Box.Height, 140) {
		t.Fatalf("sub height = %v, want 140", sub.Box.Height)
	}
	if w := lintSubsystemMin(root); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
}

// 最小の大きさでも足りなければ、従来どおり全員が比例して縮み、警告が出る。
func TestSubsystemBelowMinWarns(t *testing.T) {
	title := TextNode("title", WithHeight(30))
	sub := SubsystemNode(fixedSub{nat: Size{100, 200}, min: Size{50, 100}})
	root := Col([]*Spec{title, sub}, WithGap(10))
	computeLayout(root, 0, 0, 400, 100)
	if sub.Box.Height >= 100 {
		t.Fatalf("sub height = %v, want < 100", sub.Box.Height)
	}
	w := lintSubsystemMin(root)
	if len(w) != 1 || !strings.Contains(w[0], "fixed: needs at least 100.0pt height") {
		t.Fatalf("warnings = %v", w)
	}
}

// 余るときは grow で受け取る(旧来の canvas と同じ使い方もできる)。
func TestSubsystemGrows(t *testing.T) {
	sub := SubsystemNode(fixedSub{nat: Size{100, 120}, min: Size{50, 80}}, WithGrow(1))
	root := Col([]*Spec{sub})
	computeLayout(root, 0, 0, 400, 500)
	if !near(sub.Box.Height, 500) {
		t.Fatalf("height = %v, want 500", sub.Box.Height)
	}
}

// Gantt の申告: 行が増えると自然な高さが増え、最小は自然以下、幅は渡した値を返す。
func TestGanttMeasure(t *testing.T) {
	chart := buildChart()
	g := &GanttSubsystem{Chart: chart}
	m := g.Measure(Constraint{Width: 896})
	if m.Natural.Width != 896 {
		t.Fatalf("natural width = %v, want 896", m.Natural.Width)
	}
	if !(m.Min.Height > 0 && m.Min.Height <= m.Natural.Height) {
		t.Fatalf("min %v / natural %v", m.Min.Height, m.Natural.Height)
	}
	smaller := *chart
	smaller.Groups = chart.Groups[:1]
	m2 := (&GanttSubsystem{Chart: &smaller}).Measure(Constraint{Width: 896})
	if !(m2.Natural.Height < m.Natural.Height) {
		t.Fatalf("fewer rows should need less height: %v vs %v", m2.Natural.Height, m.Natural.Height)
	}
}

// デモのスライドで、Gantt は最小の大きさ以上をもらっている。
func TestDemoSlideRespectsGanttMin(t *testing.T) {
	spec := buildSlideSpec(themeDark(), "subtitle")
	computeLayout(spec, 0, 0, pageW, pageH)
	if w := lintSubsystemMin(spec); len(w) != 0 {
		t.Fatalf("warnings: %v", w)
	}
}

// 登録簿: テンプレートは既存の種類を差し替えられるが、無い種類・重複は誤り。
func TestRegistryOverride(t *testing.T) {
	base := DefaultSubsystems()
	tpl := base.Clone()
	called := false
	if err := tpl.Override("gantt", func(data any) (Subsystem, error) {
		called = true
		return fixedSub{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.Build("gantt", buildChart()); err != nil || !called {
		t.Fatalf("override not used: %v", err)
	}
	if s, _ := base.Build("gantt", buildChart()); s.Name() != "gantt" {
		t.Fatalf("clone leaked into base")
	}
	if err := tpl.Override("gant", nil); err == nil {
		t.Fatal("override of unknown kind should fail")
	}
	if err := tpl.Register("gantt", nil); err == nil {
		t.Fatal("duplicate register should fail")
	}
	if _, err := tpl.Build("pie", nil); err == nil {
		t.Fatal("unknown kind should fail")
	}
	if _, err := base.Build("gantt", "not a chart"); err == nil {
		t.Fatal("wrong data type should fail")
	}
}

// テーマに無い役割名は覚えておかれる。
func TestCheckedRolesMissing(t *testing.T) {
	r := newCheckedRoles(Theme{"support": "00FF00"})
	r.Color("support")
	r.Color("danger")
	if got := r.Missing(); len(got) != 1 || got[0] != "danger" {
		t.Fatalf("missing = %v", got)
	}
}
