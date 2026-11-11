package boxloom

import (
	"math"
	"strings"
	"testing"
)

// 全角の文字(どの和文の字体でも 1em)には幅の余裕を掛けない。欧文には掛ける。
func TestWidthMarginOnlyOnNarrowRunes(t *testing.T) {
	tm := mustTemplate("light")
	restore := useFonts(tm.Fonts)
	defer restore()
	if w := charEmWidth('漢', false); math.Abs(w-1) > 0.01 {
		t.Errorf("a full-width rune must be measured at 1em without the margin, got %.3f", w)
	}
	raw := activeFonts.emWidth('a', false)
	if w := charEmWidth('a', false); math.Abs(w-raw*tm.Fonts.WidthMargin) > 1e-9 {
		t.Errorf("a Latin rune must carry widthMargin: %.4f, want %.4f", w, raw*tm.Fonts.WidthMargin)
	}
}

func phraseLines(t *testing.T, text string, size, width float64) []string {
	t.Helper()
	var out []string
	for _, l := range wrapRuns([]TextRun{{Text: text, Phrase: true}}, size, width) {
		out = append(out, runsText(l))
	}
	return out
}

// 2行になるリード文は、文の切れ目で折る(行の数は増やさない)。
func TestPhraseBreaksAtSentenceEnd(t *testing.T) {
	tm := mustTemplate("light")
	restore := useFonts(tm.Fonts)
	defer restore()
	text := "経済全体の生産性への効果は、推計で年 0.1〜1.5 ポイントと幅が大きい。分かれ目は業種ごとの導入の速さである。"
	got := phraseLines(t, text, 18, 880)
	if len(got) != 2 || !strings.HasSuffix(got[0], "大きい。") {
		t.Errorf("want the break after the first sentence, got %q", got)
	}
	// 文の切れ目が無ければ読点で折る。
	text = "国・企業・人の差を開いているのは技術そのものではなく、備えの差から生まれている格差である"
	got = phraseLines(t, text, 18, 560)
	if len(got) < 2 || !strings.HasSuffix(got[0], "、") {
		t.Errorf("want the break after a comma, got %q", got)
	}
}

// 寄せても行の数は詰めて折ったときと同じ。寄せられなければ詰めて折る。
func TestPhraseKeepsLineCount(t *testing.T) {
	tm := mustTemplate("light")
	restore := useFonts(tm.Fonts)
	defer restore()
	texts := []string{
		"短い。とても長い文がこのあとに続いていて、一行には入りきらないので次の行へ送られることになるがそれで行が増えてはいけない",
		"句読点のない長い文を一行に入りきらない長さまで書き続けてみることで詰めて折ったときと同じ結果になるかを確かめる",
		"AI の影響は、4 つの入口(雇用・生産性・基盤・情報)から、働く場・学ぶ場・決める場の 3 つを通って社会に広がる。",
	}
	for _, text := range texts {
		for _, w := range []float64{200, 360, 560, 880} {
			greedy := wrapRuns([]TextRun{{Text: text}}, 18, w)
			phrase := phraseLines(t, text, 18, w)
			if len(phrase) != len(greedy) {
				t.Errorf("%q at %.0f: %d lines with phrase breaks, %d without", text, w, len(phrase), len(greedy))
			}
			if strings.Join(phrase, "") != text {
				t.Errorf("%q at %.0f: text changed: %q", text, w, phrase)
			}
		}
	}
	// 句読点が無ければ詰めて折ったのと同じ。
	text := texts[1]
	g := wrapRuns([]TextRun{{Text: text}}, 18, 360)
	p := phraseLines(t, text, 18, 360)
	for i := range g {
		if runsText(g[i]) != p[i] {
			t.Errorf("without punctuation the lines must match the greedy wrap: %q vs %q", runsText(g[i]), p[i])
		}
	}
}
