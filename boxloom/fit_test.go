package main

import (
	"math"
	"testing"
)

func nearPt(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// aspectSub は縦横比 2:1(幅÷高さ)を保つ試験用の図。
type aspectSub struct{ drawn LBox }

func (a *aspectSub) Name() string { return "aspect-test" }
func (a *aspectSub) Measure(c Constraint) Measurement {
	w, h := 400.0, 200.0
	switch {
	case c.Width > 0:
		w, h = c.Width, c.Width/2
	case c.Height > 0:
		w, h = c.Height*2, c.Height
	}
	return Measurement{Natural: Size{w, h}, Min: Size{100, 50}, Max: Size{w, h}, FixedAspect: true}
}
func (a *aspectSub) Render(ctx RenderContext, box LBox) { a.drawn = box }

func renderTree(root *Spec, w, h float64) {
	computeLayout(root, 0, 0, w, h)
	renderSpec(RenderContext{Out: NewPPTXBuilder(w, h), Roles: newCheckedRoles(Theme{})}, root)
}

// 列の中では、幅に合わせた高さ(幅÷2)をもらう。
func TestFixedAspectGetsHeightForWidth(t *testing.T) {
	sub := &aspectSub{}
	leaf := SubsystemNode(sub)
	root := Col([]*Spec{leaf, TextNode("下の文")}, WithAlignItems(AlignStretch))
	renderTree(root, 600, 540)
	if !nearPt(leaf.Box.Width, 600) || !nearPt(leaf.Box.Height, 300) {
		t.Fatalf("box = %+v, want 600x300", leaf.Box)
	}
	if sub.drawn != leaf.Box {
		t.Errorf("drawn %+v, want the allocated box %+v", sub.drawn, leaf.Box)
	}
}

// 行の中で高さが伸ばされても、図は形を保つ大きさだけを受け取り、余りは Place の側に残る。
func TestFixedAspectPlacesSpare(t *testing.T) {
	for _, tc := range []struct {
		place Place
		wantY float64
	}{
		{Place{}, 0},
		{Place{X: PlaceStart, Y: PlaceCenter}, 75},
		{Place{X: PlaceStart, Y: PlaceEnd}, 150},
	} {
		sub := &aspectSub{}
		leaf := SubsystemNode(sub, WithPlace(tc.place.X, tc.place.Y))
		tall := Col(nil, WithHeight(400), WithWidth(10))
		root := Row([]*Spec{leaf, tall}, WithAlignItems(AlignStretch))
		renderTree(root, 510, 400)
		if !nearPt(leaf.Box.Height, 400) {
			t.Fatalf("allocated height %.1f, want 400 (stretched)", leaf.Box.Height)
		}
		d := sub.drawn
		if !nearPt(d.Height, d.Width/2) || !nearPt(d.Width, leaf.Box.Width) {
			t.Errorf("place %+v: drawn %+v keeps no 2:1 shape in %+v", tc.place, d, leaf.Box)
		}
		if !nearPt(d.Y-leaf.Box.Y, tc.wantY) {
			t.Errorf("place %+v: drawn y offset %.1f, want %.1f", tc.place, d.Y-leaf.Box.Y, tc.wantY)
		}
	}
}

// 形を保たない図は、申告した上限(Max)で止まる。上限が無ければ割り当てをそのまま使う。
type capSub struct {
	maxH  float64
	drawn LBox
}

func (m *capSub) Name() string { return "max-test" }
func (m *capSub) Measure(c Constraint) Measurement {
	return Measurement{Natural: Size{100, 50}, Max: Size{0, m.maxH}}
}
func (m *capSub) Render(ctx RenderContext, box LBox) { m.drawn = box }

func TestMaxClampsCrossAxis(t *testing.T) {
	for _, maxH := range []float64{120, 0} {
		sub := &capSub{maxH: maxH}
		leaf := SubsystemNode(sub, WithGrow(1))
		root := Row([]*Spec{leaf, Col(nil, WithHeight(300), WithWidth(10))}, WithAlignItems(AlignStretch))
		renderTree(root, 400, 300)
		want := 300.0
		if maxH > 0 {
			want = maxH
		}
		if !nearPt(sub.drawn.Height, want) || !nearPt(sub.drawn.Width, leaf.Box.Width) {
			t.Errorf("maxH %.0f: drawn %+v, want height %.0f in %+v", maxH, sub.drawn, want, leaf.Box)
		}
	}
}

// 行の注記の文が破線の箱に収まらないときは、その行だけが高くなり、本体の
// 高さの合計は変わらない(Backlog #37)。
func TestGanttNoteRowGrowsToFitText(t *testing.T) {
	d, err := readDeck("../examples/ir/gantt-full.json")
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("dark")
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	var leaf *Spec
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindSubsystem {
			leaf = s
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	root := built[0].Root
	computeLayout(root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	walk(root)
	g := leaf.Subsystem.(*GanttSubsystem)
	gv := g.vals()
	box := subsystemDrawBox(leaf)
	needs := ganttNoteNeeds(gv, g.Chart, box.Width)
	if len(needs) == 0 {
		t.Fatal("example has no row note")
	}
	gm := ganttMetricsFor(gv, g.Chart, box.Width)
	bodyH := box.Height - 2*gv.Padding - gm.annBottomH -
		(gv.LegendH + gv.LegendGap + gm.annTopH + gm.milestoneH + gv.AxisLabelH + 10)
	rows := ganttRowLayout(gv, g.Chart, box.Width, bodyH)
	for _, n := range needs {
		if h := rows.h(n.group, n.row); h+0.01 < n.rowH {
			t.Errorf("note %q: row height %.1f < needed %.1f", n.text, h, n.rowH)
		}
		if n.rowH <= rows.base {
			t.Errorf("note %q fits a normal row (need %.1f, base %.1f); the example no longer tests #37", n.text, n.rowH, rows.base)
		}
	}
	gaps := gv.GroupGap * float64(len(g.Chart.Groups)-1)
	if got := rows.bodyHeight(gaps); !nearPt(got, bodyH) {
		t.Errorf("rows sum to %.2f, body is %.2f", got, bodyH)
	}
	if ws, _ := tm.RenderDeck(d, t.TempDir()+"/g.pptx"); len(ws) != 0 {
		t.Errorf("unexpected warnings: %v", ws)
	}
}

// rowsFor は高くした行の分だけほかの行を低くする。
func TestRowsFor(t *testing.T) {
	need := [][]float64{{0, 30, 0}, {0}}
	r := rowsFor(need, 100, 10)
	if !nearPt(r.base, 70.0/3) || !nearPt(r.h(0, 1), 30) || !nearPt(r.bodyHeight(0), 100) {
		t.Errorf("base %.2f h %.2f sum %.2f", r.base, r.h(0, 1), r.bodyHeight(0))
	}
	if r := rowsFor(need, 40, 10); !nearPt(r.base, 10) {
		t.Errorf("base must not go under minRowH: %.2f", r.base)
	}
}
