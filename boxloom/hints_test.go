package boxloom

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// aspectBox は縦横比 w:h を保つ図(試験用)。
type aspectBox struct{ w, h float64 }

func (a aspectBox) Name() string { return "aspect" }
func (a aspectBox) Measure(c Constraint) Measurement {
	m := Measurement{FixedAspect: true}
	switch {
	case c.Width > 0:
		m.Natural = Size{Width: c.Width, Height: c.Width * a.h / a.w}
	case c.Height > 0:
		m.Natural = Size{Width: c.Height * a.w / a.h, Height: c.Height}
	}
	m.Min, m.Max = m.Natural, m.Natural
	return m
}
func (a aspectBox) Render(RenderContext, LBox) {}

// 縦横比を保つ図は、高さで決まった幅より広い割り当ての横を残す。余りは
// ブロックの幅 − 高さ×比 で、図の左右どちらに置いても合計で数える。
func TestBlockSpareWidth(t *testing.T) {
	for _, place := range []PlaceAlign{PlaceStart, PlaceCenter, PlaceEnd} {
		fig := SubsystemNode(aspectBox{w: 2, h: 1}, WithGrow(1), WithPlace(place, PlaceStart))
		body := Col([]*Spec{fig})
		computeLayout(body, 0, 0, 800, 300) // 高さ 300 → 幅 600、余り 200
		got := blockSpareWidth(fig)
		if math.Abs(got-200) > 0.01 {
			t.Errorf("place %d: spare = %.2f, want 200", place, got)
		}
		w := spareWidthWarnings(body, map[int]float64{0: got}, []string{"image"})
		if len(w) != 1 || !strings.Contains(w[0], "200.0pt (25%)") {
			t.Errorf("place %d: warnings = %q", place, w)
		}
	}
	// 幅で決まる(余らない)図・図を持たないブロックは 0。
	fig := SubsystemNode(aspectBox{w: 2, h: 1}, WithGrow(1))
	computeLayout(Col([]*Spec{fig}), 0, 0, 400, 300)
	if got := blockSpareWidth(fig); got != 0 {
		t.Errorf("width-bound figure: spare = %.2f", got)
	}
	txt := TextNode("文")
	computeLayout(Col([]*Spec{txt}), 0, 0, 400, 300)
	if got := blockSpareWidth(txt); got != 0 {
		t.Errorf("text block: spare = %.2f", got)
	}
}

// 例の文書の横長の画像(blocks.bx.md の /slides/4)は横が空き、check は
// spare-width の警告と容量の remainingWidth を同じ数字で出す。
func TestSpareWidthWarningAndCapacity(t *testing.T) {
	for _, tpl := range []string{"light", "dark"} {
		_, warns, caps := CheckFileCapacity("../examples/md/blocks.bx.md", tpl)
		var rw float64
		for _, c := range caps {
			for _, b := range c.Blocks {
				if b.Path == "/slides/4/body/0" {
					rw = b.RemainingWidth
				} else if b.RemainingWidth != 0 && b.Type != "gallery" {
					t.Errorf("%s: %s %s: remainingWidth %.1f", tpl, b.Path, b.Type, b.RemainingWidth)
				}
			}
		}
		if rw <= 0 {
			t.Fatalf("%s: no remainingWidth on the landscape image", tpl)
		}
		found := false
		for _, w := range warns {
			if w.Code != "spare-width" {
				continue
			}
			found = true
			if w.Path != "/slides/4/body/0" || !strings.Contains(w.Message, fmt.Sprintf("%.1fpt", rw)) || w.Hint == "" || w.Line == 0 {
				t.Errorf("%s: %+v (capacity %.1f)", tpl, w, rw)
			}
		}
		if !found {
			t.Errorf("%s: no spare-width warning", tpl)
		}
	}
}

// fill-ratio の hint:隣も薄く、まとめると収まるときは、隣のスライドと見込みの
// 割合(容量の need・blockGap・bodyArea から)を添える。そうでなければ直し方だけ。
func TestFillRatioMergeHint(t *testing.T) {
	_, warns, caps := CheckFileCapacity("../examples/md/subsystems-showcase.bx.md", "light")
	capOf := map[string]SlideCapacity{}
	for _, c := range caps {
		capOf[c.Path] = c
	}
	byPath := map[string]Issue{}
	for _, w := range warns {
		if w.Code == "fill-ratio" {
			byPath[w.Path] = w
		}
	}
	// /slides/4 と /slides/5 はどちらも薄く、隣り合う。
	a, b := capOf["/slides/4"], capOf["/slides/5"]
	want := fmt.Sprintf("約 %.0f%%", (a.Need+a.BlockGap+b.Need)/math.Min(a.BodyArea, b.BodyArea)*100)
	for p, other := range map[string]string{"/slides/4": "/slides/5", "/slides/5": "/slides/4"} {
		h := byPath[p].Hint
		if !strings.Contains(h, "隣の "+other) || !strings.Contains(h, want) || !strings.Contains(h, fillRatioHint) {
			t.Errorf("%s: hint = %q (want %s, %s)", p, h, other, want)
		}
	}
	// /slides/1 の隣(0 は表紙、2 は区切り)は本文を持たないので、まとめる相手は無い。
	if h := byPath["/slides/1"].Hint; h != fillRatioHint {
		t.Errorf("/slides/1: hint = %q", h)
	}
}

// まとめると本文の領域を越える組は勧めない。
func TestMergeHintNeedsFit(t *testing.T) {
	capOf := map[string]SlideCapacity{
		"/slides/1": {Path: "/slides/1", BodyArea: 400, Need: 250, BlockGap: 16, Blocks: []BlockCapacity{{}}},
		"/slides/2": {Path: "/slides/2", BodyArea: 400, Need: 200, BlockGap: 16, Blocks: []BlockCapacity{{}}},
		"/slides/3": {Path: "/slides/3", BodyArea: 400, Need: 100, BlockGap: 16, Blocks: []BlockCapacity{{}}},
	}
	thin := map[int]float64{1: 62, 2: 50, 3: 25}
	// 2 の隣は 1(250+16+200 > 400 で収まらない)と 3(200+16+100 = 316、収まる)。
	if h := mergeHint(2, thin, capOf); !strings.Contains(h, "/slides/3") || !strings.Contains(h, "約 79%") {
		t.Errorf("slide 2: %q", h)
	}
	if h := mergeHint(1, thin, capOf); h != "" {
		t.Errorf("slide 1: %q", h)
	}
}
