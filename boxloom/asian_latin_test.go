package boxloom

// 和文と欧文の境目の空き(#183、docs/eval.md の F4)。LibreOffice は同じ行で和文の字と
// 欧文の字(数字・英字・半角の括弧)が隣り合うところに 0.25em を足す。字の幅ちょうどの
// 枠では最後の字が次の行に折れ、report の pairs では下の罫線と重なった。測るときに
// 境目ごとの空き(fonts.asianLatinSpacingEm)を数えることを確かめる。

import (
	"math"
	"strings"
	"testing"
)

// TestAsianLatinGap は空きの数え方:境目ごとに数え、行の頭・終わりの境目は数えない。
// 0.25em と行の頭・終わりの扱いは LibreOffice で確かめた値(BIZ UDPGothic 14pt。
// 「ああ5ああ」は字の幅+2つの空きより 0.5pt 狭い枠で折れ、0.5pt 広い枠で1行)。
func TestAsianLatinGap(t *testing.T) {
	report := mustTemplate("report")
	dark := mustTemplate("dark")
	if report.Fonts.AsianLatinSpacing != 0.25 || dark.Fonts.AsianLatinSpacing != 0 {
		t.Fatalf("asianLatinSpacingEm: report %v (want 0.25), dark %v (want 0)", report.Fonts.AsianLatinSpacing, dark.Fonts.AsianLatinSpacing)
	}
	defer useFonts(activeFonts)()
	const size = 14.0
	glyphs := func(s string) float64 {
		w := 0.0
		for _, ch := range s {
			w += charEmWidth(ch, false) * size
		}
		return w
	}
	lines := func(s string, width float64) []string {
		var out []string
		for _, ln := range wrapRuns([]TextRun{{Text: s}}, size, width) {
			out = append(out, runsText(ln))
		}
		return out
	}

	activeFonts = dark.Fonts // dark・light は数えない(出力を変えない)
	if got, want := naturalRunsWidth([]TextRun{{Text: "ああ5ああ"}}, size), glyphs("ああ5ああ"); math.Abs(got-want) > 1e-9 {
		t.Errorf("dark: width %.3f, want %.3f (no gap)", got, want)
	}

	activeFonts = report.Fonts
	gap := 0.25 * size
	s := "ああ5ああ"
	want := glyphs(s) + 2*gap
	for name, got := range map[string]float64{
		"naturalRunsWidth": naturalRunsWidth([]TextRun{{Text: s}}, size),
		"naturalTextWidth": naturalTextWidth(&Spec{Text: s, FontSize: size}),
		"runWidthPt":       runWidthPt(s, size, false),
	} {
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s(%q) = %.3f, want %.3f (glyphs + 2 gaps)", name, s, got, want)
		}
	}
	// 字の幅ちょうど(空きを数えない幅)の枠では折れる。空きを足した幅なら1行。
	if got := lines(s, glyphs(s)+0.5); len(got) != 2 {
		t.Errorf("width = glyphs+0.5: lines %q, want 2 (LibreOffice wraps the last character)", got)
	}
	if got := lines(s, want); len(got) != 1 {
		t.Errorf("width = glyphs+gaps: lines %q, want 1", got)
	}
	if got := wrapText(s, size, glyphs(s)+0.5, false); len(got) != 2 {
		t.Errorf("wrapText at glyphs+0.5: %q, want 2 lines", got)
	}
	// 行の終わりの境目(「5」の後ろ)は数えない:「ああああ5」は1つの空きで1行目に入る。
	long := "ああああ5あああ"
	if got := lines(long, glyphs("ああああ5")+gap+0.5); got[0] != "ああああ5" {
		t.Errorf("trailing gap counted: first line %q, want %q", got[0], "ああああ5")
	}
	// 行の頭の境目(「5」の前で折れたとき)は数えない。
	if got := lines("あああ5あ", glyphs("あああ")+0.5); len(got) != 2 || got[1] != "5あ" {
		t.Errorf("leading gap: lines %q, want [あああ 5あ]", got)
	}
	if got := wrapText("あああ5あ", size, glyphs("あああ")+0.5, false); len(got) != 2 || got[1] != "5あ" {
		t.Errorf("wrapText leading gap: lines %q, want [あああ 5あ]", got)
	}
	// 箇条の印(段落の印として書く)と本文の間には数えない。
	if got, want := naturalRunsWidth([]TextRun{{Text: "・", Mark: true}, {Text: "5あ"}}, size), glyphs("・5あ")+gap; math.Abs(got-want) > 1e-9 {
		t.Errorf("mark: width %.3f, want %.3f (one gap, none after the mark)", got, want)
	}
}

// TestPairsSideFitsLibreOffice は eval F4 の形(noimage の t05 の2枚目):report の pairs の
// 側の字が、空きを数えなければ枠の幅にちょうど1行に入るとき。1行で組んだ側の字は
// どれも、LibreOffice の折り方(選んだ字体の字の幅+境目ごとに 0.25em)で枠に入る。
func TestPairsSideFitsLibreOffice(t *testing.T) {
	src := `---
title: 例
template: report
fictional: true
---

# 変わること

:::::pairs 今 | これから
:::subject 受け持つところ
休日も学校の活動として先生が見ている
:::
:::response 受け持つところ
地域のクラブ(スポーツ協会・文化団体)が休日の活動を受け持つ
:::
:::subject 練習の場所
学校の中
:::
:::response 練習の場所
13部のうち5部は学校の外(学校から1.2〜4.5km)
:::
:::subject 費用
会費は無い
:::
:::response 費用
会費は月1,500〜3,000円(部による)、保険は年800円
:::
:::::
`
	dump, err := Layout("f4.bx.md", []byte(src), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tp := mustTemplate("report")
	face := tp.Fonts.EastAsian // report は欧文も和文も同じ字体
	lo := func(text string, bold bool, size float64) float64 {
		w := 0.0
		var prev rune
		for _, ch := range text {
			w += face.width(ch, bold) * size
			if prev != 0 && ((isEastAsianRune(prev) && isLatinSpaced(ch)) || (isLatinSpaced(prev) && isEastAsianRune(ch))) {
				w += 0.25 * size
			}
			prev = ch
		}
		return w
	}
	checked := 0
	var walk func(n *LayoutNode)
	walk = func(n *LayoutNode) {
		for _, c := range n.Children {
			walk(c)
		}
		if n.Kind != "text" || !strings.Contains(n.Path, "/pairs/") || !strings.Contains(n.Path, "/response/body") {
			return
		}
		lh := n.LineHeight
		if lh == 0 {
			lh = tp.Fonts.lineHeight()
		}
		if n.Box.Height > n.FontSize*lh+0.01 {
			return // 2行以上で組んだ
		}
		var text strings.Builder
		bold := n.Bold
		for _, r := range n.Runs {
			text.WriteString(r.Text)
			bold = bold || r.Bold
		}
		if text.Len() == 0 && n.Text != nil {
			text.WriteString(*n.Text)
		}
		checked++
		if w := lo(text.String(), bold, n.FontSize); w > n.Box.Width {
			t.Errorf("%s %q: laid out as 1 line in %.2fpt, but LibreOffice needs %.2fpt (wraps onto the rule)", n.Path, text.String(), n.Box.Width, w)
		}
	}
	for _, s := range dump.Slides {
		walk(s.Root)
	}
	if checked == 0 {
		t.Fatal("no one-line response text in the pairs")
	}
}
