package boxloom

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// report の段階の図と循環の図の見た目(#110)。文はすべて架空。

// report の phases の群の帯は基調色の塗りに白い字(on_conditional)。light は従来の枠線。
func TestReportPhasesBandFilled(t *testing.T) {
	for _, tc := range []struct {
		tmpl string
		fill bool
	}{{"report", true}, {"light", false}} {
		sub := figureFromIR(t, tc.tmpl, "phases", `{"steps": [
		  {"title": "調べる", "id": "a", "items": [{"title": "声を集める"}]},
		  {"title": "試す", "id": "b", "items": [{"title": "小さく試す"}]}],
		  "groups": [{"title": "架空の一年目", "from": "a", "to": "b"}]}`)
		p := figureSlide(t, tc.tmpl, SubsystemNode(sub, WithGrow(1)), "phases-band-"+tc.tmpl)
		th := mustTemplate(tc.tmpl).Theme
		var band *shape
		for k := range p.Shapes {
			if p.Shapes[k].text == "架空の一年目" {
				band = &p.Shapes[k]
			}
		}
		if band == nil {
			t.Fatalf("%s: band missing", tc.tmpl)
		}
		if filled := band.fill == th.c("accent") && !band.hasOutline; filled != tc.fill {
			t.Errorf("%s: band fill=%q outline=%v, want filled=%v", tc.tmpl, band.fill, band.hasOutline, tc.fill)
		}
		if tc.fill && (len(band.runs) == 0 || band.runs[0].Color != th.c("on_conditional")) {
			t.Errorf("%s: band text must be on_conditional: %+v", tc.tmpl, band.runs)
		}
	}
}

// 列の中の cycle に 20 字前後の題が5つ:題は扇に入らないので輪の外に置き、輪を
// 小さくして置き場を取る。題どうし・題と輪(番号の丸を含む正方形)が重ならず、
// どれも図の矩形の中にあり、警告は出ない。
func TestReportCycleTitlesOutsideInColumn(t *testing.T) {
	src := "---\ntitle: T\ntemplate: report\n---\n\n# 輪と説明\n\n" +
		"架空の町の拠点づくりは、五つの段を一年かけて回し、毎年ふり返って次の担い手へ渡していく。\n\n" +
		"- 住民の声を集める\n- 週末に拠点を開く\n- 決まりを書いて渡す\n\n::::columns\n:::column\n```cycle\nsteps:\n" +
		"  - title: 地域の担い手を集めて話を聞く場をつくる\n" +
		"  - title: 週末の拠点を開いて催しを続けていく\n" +
		"  - {title: 小さな試みを重ねて手応えを確かめる, detail: 三か月ごと}\n" +
		"  - title: 運営の決まりを皆で話し合って書く\n" +
		"  - title: 一年をふり返って次の担い手へ渡す\n" +
		"```\n:::\n:::column 段ごとの要点\n- 住民に話を聞く\n- 担い手を決める\n:::\n::::\n"
	path := filepath.Join(t.TempDir(), "c.bx.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	errs, warns, _ := CheckFileCapacity(path, "report")
	if len(errs) > 0 {
		t.Fatalf("errors %+v", errs)
	}
	for _, w := range warns {
		if w.Code == "figure-text-overflow" || w.Code == "subsystem-min" || w.Code == "layout-overflow" {
			t.Errorf("%+v", w)
		}
	}
	sub := figureFromIR(t, "report", "cycle", `{"steps": [
	  {"title": "地域の担い手を集めて話を聞く場をつくる"}, {"title": "週末の拠点を開いて催しを続けていく"},
	  {"title": "小さな試みを重ねて手応えを確かめる", "detail": "三か月ごと"},
	  {"title": "運営の決まりを皆で話し合って書く"}, {"title": "一年をふり返って次の担い手へ渡す"}]}`).(*CycleSubsystem)
	defer useFonts(mustTemplate("report").Fonts)()
	// 輪が小さくなる列:本文の3分の1ほどの幅の列と、半分の幅で低い列(上に表などがある)。
	for _, box := range []LBox{{X: 50, Y: 82.8, Width: 300, Height: 391.2}, {X: 50, Y: 82.8, Width: 340, Height: 391.2},
		{X: 50, Y: 200, Width: 418, Height: 260}} {
		inside := sub.layoutFor(box.Width, box.Height, 0)
		if inside.outside != true {
			t.Fatalf("%v: titles must go outside the ring", box)
		}
		pl := sub.place(box)
		if !pl.outside {
			t.Fatalf("%v: place did not put titles outside", box)
		}
		v := sub.vals()
		e := pl.r + v.NumberDiameter/2
		ring := LBox{X: pl.cx - e, Y: pl.cy - e, Width: 2 * e, Height: 2 * e}
		if !within(box, ring) {
			t.Errorf("%v: ring %+v outside the figure", box, ring)
		}
		for i, l := range pl.labels {
			if l.box.Width <= 0 {
				t.Fatalf("%v: steps[%d] has no outside title box", box, i)
			}
			if !within(box, l.box) {
				t.Errorf("%v: steps[%d] title %+v outside the figure", box, i, l.box)
			}
			if boxesOverlap(l.box, ring) {
				t.Errorf("%v: steps[%d] title %+v overlaps the ring %+v", box, i, l.box, ring)
			}
			for j := i + 1; j < len(pl.labels); j++ {
				if boxesOverlap(l.box, pl.labels[j].box) {
					t.Errorf("%v: titles %d and %d overlap: %+v %+v", box, i, j, l.box, pl.labels[j].box)
				}
			}
		}
		if ws := sub.Lint(box); len(ws) != 0 {
			t.Errorf("%v: warns %v", box, ws)
		}
		// 輪は扇に題を入れるときより小さい。
		if in := sub.radiusFor(box.Width, 0, sub.labelSizes(sub.labelCap(box.Width))); !(pl.r < math.Min(in, sub.radiusFor(box.Height, 1, sub.labelSizes(sub.labelCap(box.Width))))) {
			t.Errorf("%v: ring r=%.1f did not shrink (inside r=%.1f)", box, pl.r, in)
		}
	}
	// 外に置いても枠に入らないほど狭ければ扇の中のまま、警告になる。
	if ws := sub.Lint(LBox{Width: 220, Height: 220}); !strings.Contains(strings.Join(ws, "\n"), "text does not fit") {
		t.Errorf("a too small box must warn: %v", ws)
	}
}
