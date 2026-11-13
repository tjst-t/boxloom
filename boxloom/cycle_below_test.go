package boxloom

import (
	"fmt"
	"testing"
)

// 縦長の狭い枠の cycle(#154)。文はすべて架空。

const cycleLongSteps = `[
  {"title": "地域の担い手を広く集めて最初の話し合いの場を町内で開く"},
  {"title": "週末だけ開く小さな拠点を借りて季節の催しを続けて回していく"},
  {"title": "小さな試みを何度も重ねて手応えと課題を皆で確かめる"},
  {"title": "運営の決まりと役割の分け方を全員で話し合って書き残しておく"},
  {"title": "一年をふり返って学んだことを次の担い手へ確かに渡す"}]`

// cycleShapesOverlap は below の図形(輪と番号の丸を囲む正方形・並びの番号の丸・題の枠・
// 凡例)どうしの重なりと、枠 box からのはみ出しを書く(無ければ "")。
func cycleShapesOverlap(pl *cyclePlaced, box LBox) string {
	shapes := map[string]LBox{"ring": {X: pl.cx - pl.r, Y: pl.cy - pl.r, Width: 2 * pl.r, Height: 2 * pl.r}}
	for i := range pl.labels {
		shapes[fmt.Sprintf("title[%d]", i)] = pl.labels[i].box
		shapes[fmt.Sprintf("num[%d]", i)] = pl.listNums[i]
	}
	if pl.legend.Height > 0 {
		shapes["legend"] = pl.legend
	}
	const eps = 0.01
	for k, b := range shapes {
		if b.X < box.X-eps || b.Y < box.Y-eps || b.X+b.Width > box.X+box.Width+eps || b.Y+b.Height > box.Y+box.Height+eps {
			return fmt.Sprintf("%s %+v is outside the box %+v", k, b, box)
		}
		for k2, b2 := range shapes {
			if k < k2 && boxesOverlap(b, b2) {
				return fmt.Sprintf("%s %+v overlaps %s %+v", k, b, k2, b2)
			}
		}
	}
	// 輪の番号の丸は輪の中に。
	for i, n := range pl.numbers {
		ring := shapes["ring"]
		if n.X < ring.X || n.Y < ring.Y || n.X+n.Width > ring.X+ring.Width || n.Y+n.Height > ring.Y+ring.Height {
			return fmt.Sprintf("ring number[%d] %+v is outside the ring", i, n)
		}
	}
	return ""
}

// 枠 279×326 に長い題(25〜35字)の5段:題を輪の外に置くと輪が最小(96pt)・題の枠が
// 5字幅に潰れるので、輪を上に、番号の丸と題の並びをその下に置く。輪の直径は 150pt
// 以上、題の枠の幅は 180pt 以上で、図形は重ならず枠からはみ出さず、警告も無い。
func TestCycleBelowInTallNarrowBox(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		sub := figureFromIR(t, tmpl, "cycle", `{"steps": `+cycleLongSteps+`}`).(*CycleSubsystem)
		for _, st := range sub.Data.Steps {
			if n := len([]rune(runsText(itemRuns(st.Title, "", nil)))); n < 25 || n > 35 {
				t.Fatalf("title %q has %d runes, want 25-35", runsText(itemRuns(st.Title, "", nil)), n)
			}
		}
		restore := useFonts(mustTemplate(tmpl).Fonts)
		box := LBox{X: 40, Y: 80, Width: 279, Height: 326}
		pl := sub.place(box)
		lint := sub.Lint(box)
		restore()
		if !pl.below {
			t.Fatalf("%s: titles must go below the ring (outside=%v r=%.1f)", tmpl, pl.outside, pl.r)
		}
		// report(行送り 1.2)は 150pt 以上。dark・light は行送りが 1.35 で並びが高いので、
		// 並びの字を小さい字(small)まで下げても 150pt に届かないが、外に置くときの最小の輪
		// (outsideMinDiameter)よりは大きい。
		want := 150.0
		if tmpl != "report" {
			want = sub.vals().OutsideMinDiameter + 30
		}
		if d := 2 * pl.r; d < want-0.01 {
			t.Errorf("%s: ring diameter %.2f, want >= %.0f", tmpl, d, want)
		}
		for i, l := range pl.labels {
			if l.box.Width < 180 {
				t.Errorf("%s: title[%d] box width %.1f, want >= 180", tmpl, i, l.box.Width)
			}
			if i > 0 && l.box.Y <= pl.labels[i-1].box.Y {
				t.Errorf("%s: titles must be listed in step order", tmpl)
			}
		}
		if o := cycleShapesOverlap(pl, box); o != "" {
			t.Errorf("%s: %s", tmpl, o)
		}
		if len(lint) > 0 {
			t.Errorf("%s: lint %v", tmpl, lint)
		}
	}
}

// 補足のある段は並びの中で題の下に補足を置き(枠が補足のぶん高い)、凡例は並びの下。
func TestCycleBelowDetailAndLegend(t *testing.T) {
	data := `{"steps": [
  {"title": "地域の担い手を広く集めて最初の話し合いの場を町内で開く", "detail": "毎月の第一土曜に開く"},
  {"title": "週末だけ開く小さな拠点を借りて季節の催しを続けて回していく", "status": "conditional"},
  {"title": "小さな試みを何度も重ねて手応えと課題を皆で確かめる"},
  {"title": "運営の決まりと役割の分け方を全員で話し合って書き残しておく"},
  {"title": "一年をふり返って学んだことを次の担い手へ確かに渡す"}], "legend": {"conditional": "予算が付くときだけ"}}`
	sub := figureFromIR(t, "report", "cycle", data).(*CycleSubsystem)
	restore := useFonts(mustTemplate("report").Fonts)
	defer restore()
	box := LBox{X: 40, Y: 80, Width: 279, Height: 360}
	pl := sub.place(box)
	if !pl.below {
		t.Fatalf("titles must go below the ring (outside=%v r=%.1f)", pl.outside, pl.r)
	}
	if pl.labels[0].box.Height <= pl.labels[2].box.Height {
		t.Errorf("the step with a detail must be taller: %+v vs %+v", pl.labels[0].box, pl.labels[2].box)
	}
	last := pl.labels[len(pl.labels)-1].box
	if pl.legend.Height == 0 || pl.legend.Y < last.Y+last.Height {
		t.Errorf("legend %+v must be below the list (last %+v)", pl.legend, last)
	}
	if o := cycleShapesOverlap(pl, box); o != "" {
		t.Error(o)
	}
	if lint := sub.Lint(box); len(lint) > 0 {
		t.Errorf("lint %v", lint)
	}
	// 描くと、並びの番号の丸と題の枠が段ごとに出る(扇の中に題は書かない)。
	p := NewPPTXBuilder(960, 540)
	sub.Render(RenderContext{Out: p, Roles: mustTemplate("report").Theme}, box)
	nums, titles := 0, 0
	for _, sh := range p.Shapes {
		if !within(box, sh.box) {
			t.Errorf("shape %d %+v outside %+v", sh.id, sh.box, box)
		}
		for i, l := range pl.labels {
			if sh.box == l.box {
				titles++
			}
			if sh.box == pl.listNums[i] {
				nums++
			}
		}
	}
	if nums != 5 || titles != 5 {
		t.Errorf("drew %d list numbers and %d titles, want 5 and 5", nums, titles)
	}
}
