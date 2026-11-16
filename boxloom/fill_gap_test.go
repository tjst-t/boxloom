package boxloom

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// fillGapDeck は箇条を n 個だけ持つ本文のスライド1枚の文書(題とリードつき)。
func fillGapDeck(tmpl string, n int) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: 埋まりの例\ntemplate: %s\nfictional: true\n---\n\n# 進め方\n\n:::lead\n手順を順に進める。\n:::\n\n", tmpl)
	for k := 0; k < n; k++ {
		fmt.Fprintf(&b, "- 手順%dを決めた日までに終える\n", k+1)
	}
	return []byte(b.String())
}

// 本文の下の空きが fillGapMax(80pt)を越えれば、割合が fillRatioMin(65%)以上でも
// fill-ratio を出し、空きが上限以下なら出さない(#181)。report・dark・light の全てで、
// 箇条の数を1つずつ増やして確かめる。割合だけでは見逃していた枚(65% 以上で空きが 80pt
// 超)と、警告の出ない枚がどちらも現れること。
func TestFillGapWarnsLargeBlankBelowBody(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		gapMax := mustTemplate(tmpl).Values.FillGapMax
		if gapMax <= 0 {
			t.Fatalf("%s: fillGapMax = %v", tmpl, gapMax)
		}
		gapOnly, clean := 0, 0
		for n := 1; n <= 30; n++ {
			res := Check("x.bx.md", fillGapDeck(tmpl, n), CheckOptions{})
			if len(res.Errors) > 0 {
				t.Fatalf("%s n=%d: %s", tmpl, n, jsonOf(res.Errors))
			}
			if len(fitIssues(res.Warnings, codeLayoutOverflow)) > 0 {
				break
			}
			c := res.Capacity[0]
			blank := c.Remaining // 箇条だけの本文は伸びないので、下の空きは残りと同じ
			ratio := (c.BodyArea - blank) / c.BodyArea
			warned := len(fitIssues(res.Warnings, "fill-ratio")) > 0
			want := ratio < 0.65 || blank > gapMax+0.05
			if warned != want {
				t.Errorf("%s n=%d: fill-ratio warned=%v, want %v (fill %.1f%%, blank %.1fpt)", tmpl, n, warned, want, ratio*100, blank)
			}
			if warned && ratio >= 0.65 {
				gapOnly++
				w := fitIssues(res.Warnings, "fill-ratio")[0]
				if !strings.Contains(w.Message, "left blank below the body") || w.Hint == "" {
					t.Errorf("%s n=%d: message %q hint %q", tmpl, n, w.Message, w.Hint)
				}
			}
			if !warned {
				clean++
			}
		}
		if gapOnly == 0 || clean == 0 {
			t.Errorf("%s: slides warned only by the blank = %d, slides without fill-ratio = %d; want both", tmpl, gapOnly, clean)
		}
	}
}

// AI が作った report の資料(examples/density-eval、架空の題材)で、本文の下に大きな空き
// (92〜124pt)のある枚に fill-ratio が出て、図が伸びて埋まった枚・空きの小さい枚には出ない
// (docs/eval.md の F2)。
func TestFillGapDensityEvalReport(t *testing.T) {
	for _, c := range []struct {
		file string
		thin []bool // 本文のスライド(/slides/1〜3)ごとに fill-ratio が出るか
	}{
		{"t01", []bool{true, true, true}},  // 空き 92・124・119pt(埋まり 77・66・67%)
		{"t03", []bool{true, true, false}}, // 3枚目はグラフが伸びて 97%
		{"t07", []bool{false, false, false}},
		{"t10", []bool{false, true, false}}, // 3枚目は空き 78pt
	} {
		path := "../examples/density-eval/boxloom/" + c.file + ".bx.md"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skip(err)
		}
		res := Check(path, data, CheckOptions{})
		got := map[string]bool{}
		for _, w := range fitIssues(res.Warnings, "fill-ratio") {
			got[w.Path] = true
		}
		for k, want := range c.thin {
			p := fmt.Sprintf("/slides/%d", k+1)
			if got[p] != want {
				t.Errorf("%s %s: fill-ratio = %v, want %v", c.file, p, got[p], want)
			}
		}
	}
}
