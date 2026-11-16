package boxloom

import (
	"math"
	"strings"
	"testing"
)

// process の段の補足が1字だけ次の行に折れないこと(#184、eval F5)。文はすべて架空。

const processOrphanData = `{"steps": [
  {"title": "説明会", "detail": "5月 沿線の住民"},
  {"title": "試行", "detail": "%s", "emphasis": true},
  {"title": "本格の運行", "detail": "翌4月から"}]}`

// placeProcess は幅 width の枠に置いた process とその置き場所。
func placeProcess(t *testing.T, tmpl, detail string, width float64) (*ProcessSubsystem, *processPlaced, LBox) {
	t.Helper()
	sub := figureFromIR(t, tmpl, "process", strings.Replace(processOrphanData, "%s", detail, 1)).(*ProcessSubsystem)
	m := sub.Measure(Constraint{Width: width})
	box := LBox{X: 50, Y: 300, Width: width, Height: m.Natural.Height}
	_, pl := sub.place(box)
	return sub, pl, box
}

// eval F5 の形(report、本文の幅 860pt に3段)。補足は和文と欧文の間の空き(#183)を
// 数えると箱の幅にわずかに入らず、箱の幅で折ると最後の1字「ド」だけが次の行に残る。
// 補足の枠は箱の幅より detailSlack だけ広く(行の終わりの箱は padding の半分まで)、
// 折り返しもその幅で測るので1行に入り、高さは1行・警告は無い。
func TestProcessDetailSlack(t *testing.T) {
	tm := mustTemplate("report")
	defer useFonts(tm.Fonts)()
	const detail = "10月〜翌3月 西線の減便とデマンド"
	sub, pl, _ := placeProcess(t, "report", detail, 860)
	v, tx := sub.vals(), sub.text()
	if v.DetailSlack <= 0 {
		t.Fatalf("detailSlack = %v", v.DetailSlack)
	}
	b, d := pl.boxes[1], pl.details[1]
	runs := itemRuns(Inline{{Text: detail}}, "", nil)
	if n := len(wrapRuns(runs, tx.Detail, b.Width)); n != 2 {
		t.Fatalf("detail wraps to %d lines at the box width %.1fpt, want 2 (the case F5 is about)", n, b.Width)
	}
	if n := len(wrapRuns(runs, tx.Detail, d.Width)); n != 1 {
		t.Errorf("detail wraps to %d lines at the frame width %.1fpt, want 1", n, d.Width)
	}
	if want := b.Width + v.DetailSlack; math.Abs(d.Width-want) > 0.01 {
		t.Errorf("middle detail frame %.2fpt, want %.2fpt", d.Width, want)
	}
	if one := tx.Detail * lineHeightMult(); math.Abs(d.Height-one) > 0.01 {
		t.Errorf("detail height %.2fpt, want one line %.2fpt", d.Height, one)
	}
	// 行の終わりの箱は図の内側の余白を半分残す。
	last, lb := pl.details[2], pl.boxes[2]
	if want := lb.Width + math.Min(v.DetailSlack, v.Padding/2); math.Abs(last.Width-want) > 0.01 {
		t.Errorf("last detail frame %.2fpt, want %.2fpt", last.Width, want)
	}
	// 広げた枠は次の段の補足に届かない。
	if d.X+d.Width >= pl.details[2].X {
		t.Errorf("detail frame %.1f..%.1f reaches the next detail at %.1f", d.X, d.X+d.Width, pl.details[2].X)
	}
	for _, w := range sub.Lint(LBox{X: 50, Y: 300, Width: 860, Height: sub.Measure(Constraint{Width: 860}).Natural.Height}) {
		t.Errorf("unexpected warning: %s", w)
	}
}

// 補足が箱の幅で折れて最後の行に1字だけ残る → process-detail-orphan が1件(path はその補足、
// hint は短くする・改行で分ける)。1行に入る長さ・作者の改行で短い行になるときは出ない。
func TestProcessDetailOrphanWarning(t *testing.T) {
	for _, tmpl := range []string{"light", "dark", "report"} {
		func() {
			tm := mustTemplate(tmpl)
			defer useFonts(tm.Fonts)()
			sub, pl, _ := placeProcess(t, tmpl, "あ", 860)
			size, w := sub.text().Detail, pl.details[1].Width
			// 1行に入る一番長い「あ」の並び。
			fit := ""
			for len(wrapRuns(itemRuns(Inline{{Text: fit + "あ"}}, "", nil), size, w)) == 1 {
				fit += "あ"
			}
			orphans := func(detail string) []Issue {
				sub, _, box := placeProcess(t, tmpl, detail, 860)
				var out []Issue
				for _, s := range sub.Lint(box) {
					is := warningIssue("d.json", "slide 1: [SUBSYSTEM WARNING] body[0]: "+strings.TrimPrefix(s, "[SUBSYSTEM WARNING] "))
					if is.Code == "process-detail-orphan" {
						out = append(out, is)
					}
				}
				return out
			}
			got := orphans(fit + "い")
			if len(got) != 1 {
				t.Fatalf("%s: process-detail-orphan %d times, want 1 (%v)", tmpl, len(got), got)
			}
			if got[0].Path != "/slides/0/body/0/data/steps/1/detail" {
				t.Errorf("%s: path %q", tmpl, got[0].Path)
			}
			if !strings.Contains(got[0].Message, `only 1 character(s) ("い")`) {
				t.Errorf("%s: message %q", tmpl, got[0].Message)
			}
			hinted := []Issue{got[0]}
			addLayoutHints(hinted, nil, TemplateValues{})
			if !strings.Contains(hinted[0].Hint, "改行") {
				t.Errorf("%s: hint %q", tmpl, hinted[0].Hint)
			}
			for _, ok := range []string{fit, fit + "いうえ", "前の行\\nい"} {
				if got := orphans(ok); len(got) != 0 {
					t.Errorf("%s: %q: unexpected %v", tmpl, ok, got)
				}
			}
		}()
	}
}
