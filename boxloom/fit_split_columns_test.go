package boxloom

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// columnsWithFigure は2列の並列のまとまり:左は図(phases、段ごとに n 個の箱)、右は m 項目の箇条。
func columnsWithFigure(n, m int) string {
	var b strings.Builder
	b.WriteString("::::columns\n:::column 段の流れ\n")
	b.WriteString(emergencyPhases(n))
	b.WriteString(":::\n:::column 聞き取りの結果 {role=main}\n")
	for i := 0; i < m; i++ {
		fmt.Fprintf(&b, "- %s(%d)\n", splitSentence, i+1)
	}
	b.WriteString(":::\n::::\n")
	return b.String()
}

// 図を含む2列が非常用の段でも入らないとき、-fit は列ごとの2枚に分けて収める:並び順のまま
// (role=main の列を先にしない)、各枚に列の題・n/m の印、fit-split に「列ごとに分けた」、
// 容量は枚ごとに split と段。-fit なしは元のまま(はみ出し)。
func TestFitSplitColumns(t *testing.T) {
	md := splitDeckMD("light", columnsWithFigure(6, 12))
	plain := Check("c.bx.md", []byte(md), CheckOptions{})
	if len(fitIssues(plain.Warnings, codeLayoutOverflow)) == 0 {
		t.Fatalf("without -fit = %+v", plain.Warnings)
	}
	res := Check("c.bx.md", []byte(md), CheckOptions{Fit: true})
	sp := fitIssues(res.Info, codeFitSplit)
	if len(sp) != 1 || sp[0].Path != "/slides/2" || !strings.Contains(sp[0].Message, "split them by column into 2 slides") ||
		!strings.Contains(sp[0].Message, "slide 2 of 2 starts at /slides/2/body/0/columns/1") {
		t.Fatalf("fit-split = %+v", res.Info)
	}
	if len(fitIssues(res.Warnings, codeLayoutOverflow)) != 0 {
		t.Fatalf("check -fit still overflows: %+v", res.Warnings)
	}
	for _, w := range res.Warnings {
		if fitBrokenCodes[w.Code] {
			t.Fatalf("check -fit leaves a broken figure: %+v", w)
		}
	}
	var parts []SlideCapacity
	for _, c := range res.Capacity {
		if c.Path == "/slides/2" {
			parts = append(parts, c)
		}
	}
	if len(parts) != 2 || parts[0].Split == nil || parts[0].Split.Part != 1 || parts[1].Split.Part != 2 || parts[1].Split.Slides != 2 ||
		parts[0].Blocks[0].Path != "/slides/2/body/0" || parts[1].Blocks[0].Columns[0].Path != "/slides/2/body/0/columns/1" {
		t.Fatalf("capacity = %+v", parts)
	}

	slides, b := buildSlidesXML(t, md, BuildOptions{Fit: true})
	base, _ := buildSlidesXML(t, md, BuildOptions{AllowOverflow: true, noOverflowMark: true})
	if len(slides) != len(base)+1 || len(b.Info) != 1 || !strings.Contains(b.Info[0], "[FIT-SPLIT INFO]") {
		t.Fatalf("build -fit = %+v (%d slides, base %d)", b, len(slides), len(base))
	}
	// 表紙・目次・区切りの後の2枚(4・5枚目)が列ごとの枚:並び順のまま、列の題と n/m。
	first, second := slides[4], slides[5]
	if !strings.Contains(first, "段の流れ") || strings.Contains(first, "聞き取りの結果") || !strings.Contains(first, "1/2") {
		t.Errorf("slide 1 of 2 is not the first column with 1/2")
	}
	if !strings.Contains(second, "聞き取りの結果") || strings.Contains(second, "段の流れ") || !strings.Contains(second, "2/2") {
		t.Errorf("slide 2 of 2 is not the second column with 2/2")
	}
}

// 非常用の段で入る columns は列ごとには分けない(#150 のまま)。分けられる columns の前後の
// ブロックで分けて入るスライドも列は分けない。
func TestFitSplitColumnsOnlyWhenNeeded(t *testing.T) {
	res := Check("c.bx.md", []byte(splitDeckMD("light", emergencyColumns(18))), CheckOptions{Fit: true})
	if len(res.Info) != 0 || len(fitIssues(res.Warnings, codeFitEmergency)) != 1 {
		t.Fatalf("emergency columns = warnings %+v info %+v", res.Warnings, res.Info)
	}
	res = Check("c.bx.md", []byte(splitDeckMD("light", columnsWithFigure(3, 8))), CheckOptions{Fit: true})
	if len(fitIssues(res.Info, codeFitSplit)) != 0 {
		t.Fatalf("fitting columns were split: %+v", res.Info)
	}
}

// 列ごとに分けても1列が入らなければ誤りのまま:hint に「あと N pt」と直し方、容量に fitShort。
func TestFitSplitColumnsStillOverflows(t *testing.T) {
	md := splitDeckMD("light", columnsWithFigure(14, 12))
	res := Check("c.bx.md", []byte(md), CheckOptions{Fit: true})
	over := fitIssues(res.Warnings, codeLayoutOverflow)
	if len(over) == 0 || !strings.Contains(over[0].Hint, "pt 足りません") || !strings.Contains(over[0].Hint, "columns の列を別のスライドに") {
		t.Fatalf("check -fit = %+v", over)
	}
	if c := bodyCapacity(res.Capacity); c.FitShort == nil || *c.FitShort <= 0 {
		t.Fatalf("capacity = %+v", c)
	}
	if plain := bodyCapacity(Check("c.bx.md", []byte(md), CheckOptions{}).Capacity); plain.FitShort != nil {
		t.Fatalf("fitShort without -fit: %+v", plain)
	}
	d, _, err := ParseMarkdown("c.bx.md", []byte(md))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, err = Build(&buf, d, BuildOptions{Fit: true})
	var oe *OverflowError
	if !errors.Is(err, ErrLayoutOverflow) || !errors.As(err, &oe) || !strings.Contains(oe.Issues[0].Hint, "pt 足りません") {
		t.Fatalf("build -fit = %v", err)
	}
}
