package boxloom

import (
	"strings"
	"testing"
)

// 縦長の狭い枠の cycle(#155)。文はすべて架空。

const cycleLongSteps = `[
  {"title": "地域の担い手を広く集めて最初の話し合いの場を町内で開く"},
  {"title": "週末だけ開く小さな拠点を借りて季節の催しを続けて回していく"},
  {"title": "小さな試みを何度も重ねて手応えと課題を皆で確かめる"},
  {"title": "運営の決まりと役割の分け方を全員で話し合って書き残しておく"},
  {"title": "一年をふり返って学んだことを次の担い手へ確かに渡す"}]`

// cycleShortSteps は8字前後の題。dark・light(題 14pt)では幅 279 の扇に入らず、輪の横では
// 窮屈になる(扇の題の figure-text-overflow も出る)ので、短い題(4字)の cycleTinySteps を使う。
const cycleShortSteps = `[
  {"title": "担い手を集める"},
  {"title": "週末に拠点を開く"},
  {"title": "試みを重ねる"},
  {"title": "決まりを書き残す"},
  {"title": "一年をふり返る"}]`

const cycleTinySteps = `[
  {"title": "集める"}, {"title": "開く"}, {"title": "試す"}, {"title": "決める"}, {"title": "ふり返る"}]`

// cycleCrampedBox は幅 279・高さ 326 の枠(#154 で輪の下に題を並べた枠)。
var cycleCrampedBox = LBox{X: 40, Y: 80, Width: 279, Height: 326}

// 長い題(20〜40字)の5段:題は扇に入らず、輪の横(外)に置いても窮屈なので cycle-cramped が
// 1件(path は図のブロック、文に輪の直径と題の枠の幅)。輪の下の題の並びは描かない
// (番号の丸は輪の5つだけで、輪の下に並ぶ題の枠は無い)。
func TestCycleCrampedWarning(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		sub := figureFromIR(t, tmpl, "cycle", `{"steps": `+cycleLongSteps+`}`).(*CycleSubsystem)
		for _, st := range sub.Data.Steps {
			if n := len([]rune(runsText(itemRuns(st.Title, "", nil)))); n < 20 || n > 40 {
				t.Fatalf("title has %d runes, want 20-40", n)
			}
		}
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		box := cycleCrampedBox
		pl := sub.place(box)
		lint := sub.Lint(box)
		p := NewPPTXBuilder(960, 540)
		sub.Render(RenderContext{Out: p, Roles: tm.Theme}, box)
		restore()
		var cramped []Issue
		for _, w := range lint {
			if !strings.HasPrefix(w, "[SUBSYSTEM WARNING] ") {
				t.Fatalf("%s: warning %q", tmpl, w)
			}
			is := warningIssue("d.json", "slide 1: [SUBSYSTEM WARNING] body[0]: "+strings.TrimPrefix(w, "[SUBSYSTEM WARNING] "))
			if is.Code == "cycle-cramped" {
				cramped = append(cramped, is)
			}
		}
		if len(cramped) != 1 {
			t.Fatalf("%s: cycle-cramped %d times, want 1 (warns %v)", tmpl, len(cramped), lint)
		}
		if c := cramped[0]; c.Path != "/slides/0/body/0" || !strings.Contains(c.Message, "ring diameter") || !strings.Contains(c.Message, "title box") {
			t.Errorf("%s: cramped issue %+v", tmpl, c)
		}
		ovals := 0
		ringBottom := pl.cy + pl.r
		below := 0
		for _, s := range p.Shapes {
			if s.prst == "ellipse" {
				ovals++
			}
		}
		for _, l := range pl.labels {
			if l.box.Width > 0 && l.box.Y >= ringBottom {
				below++
			}
		}
		if ovals != 5 {
			t.Errorf("%s: %d number circles, want 5 (no list below the ring)", tmpl, ovals)
		}
		if below > 1 {
			t.Errorf("%s: %d titles are listed below the ring", tmpl, below)
		}
	}
}

// 短い題(report は8字程度、dark・light は4字程度)の5段を同じ枠に置くと cycle-cramped も
// ほかの警告も出ない。
func TestCycleShortTitlesNotCramped(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		steps := cycleTinySteps
		if tmpl == "report" {
			steps = cycleShortSteps
		}
		sub := figureFromIR(t, tmpl, "cycle", `{"steps": `+steps+`}`).(*CycleSubsystem)
		restore := useFonts(mustTemplate(tmpl).Fonts)
		lint := sub.Lint(cycleCrampedBox)
		restore()
		for _, w := range lint {
			t.Errorf("%s: %s", tmpl, w)
		}
	}
}

// withoutCramped は cycle-cramped(#155)を除いた Lint の警告。
func withoutCramped(lint []string) []string {
	var out []string
	for _, w := range lint {
		if !strings.Contains(w, "cycle: titles are cramped outside the ring") {
			out = append(out, w)
		}
	}
	return out
}

// hasCramped は Lint の警告に cycle-cramped があるか。
func hasCramped(lint []string) bool { return len(withoutCramped(lint)) != len(lint) }
