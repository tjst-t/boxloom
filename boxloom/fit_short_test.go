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
	b.WriteString(":::\n:::column 聞き取りの結果\n")
	for i := 0; i < m; i++ {
		fmt.Fprintf(&b, "- %s(%d)\n", splitSentence, i+1)
	}
	b.WriteString(":::\n::::\n")
	return b.String()
}

// 非常用の段でも入らない2列の columns は -fit でも列ごとには分けず誤り:fit-split は出ず、
// hint に「あと N pt」と列を別のスライドに分ける直し方、容量に fitShort。
func TestFitColumnsNotSplit(t *testing.T) {
	md := splitDeckMD("light", columnsWithFigure(6, 12))
	res := Check("c.bx.md", []byte(md), CheckOptions{Fit: true})
	if got := fitIssues(res.Info, codeFitSplit); len(got) != 0 {
		t.Fatalf("fit-split = %+v", got)
	}
	over := fitIssues(res.Warnings, codeLayoutOverflow)
	if len(over) == 0 || !strings.Contains(over[0].Hint, "pt 足りません") || !strings.Contains(over[0].Hint, "columns の列を別のスライドに分ける") {
		t.Fatalf("check -fit = %+v", over)
	}
	if c := bodyCapacity(res.Capacity); c.FitShort == nil || *c.FitShort <= 0 || c.Split != nil {
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
	if !errors.Is(err, ErrLayoutOverflow) || !errors.As(err, &oe) || !strings.Contains(oe.Issues[0].Hint, "columns の列を別のスライドに分ける") {
		t.Fatalf("build -fit = %v", err)
	}
}
