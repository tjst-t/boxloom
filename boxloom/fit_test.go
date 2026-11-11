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
