package boxloom

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	trHeightRe = regexp.MustCompile(`<a:tr h="(\d+)">`)
	gridColRe  = regexp.MustCompile(`<a:gridCol w="(\d+)"/>`)
)

func emuList(t *testing.T, re *regexp.Regexp, s string) []float64 {
	t.Helper()
	var out []float64
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		v, _ := strconv.Atoi(m[1])
		out = append(out, float64(v)/12700)
	}
	return out
}

// #170:評価の資料(report の6枚目)の表。縦書きの帯の「拠点の館」(4字)が群の2行より
// 長くても、群の最後の行の下に空きを作らない(帯を縦の2行に広げる)。合計の行の「合計」は
// 帯の列と次の列をまたいで1行に置く。
func TestReportGroupBandFitsRows(t *testing.T) {
	src, err := os.ReadFile("../examples/eval/report.bx.md")
	if err != nil {
		t.Skip(err)
	}
	slide := builtSlideXML(t, string(src), "report", 6)
	if !strings.Contains(slide, "拠点の館") {
		t.Fatal("slide 6 is not the grouped table")
	}
	hs := emuList(t, trHeightRe, slide)
	cols := emuList(t, gridColRe, slide)
	if len(hs) != 8 || len(cols) != 5 {
		t.Fatalf("rows %v cols %v", hs, cols)
	}
	tm, _ := LookupTemplate("report")
	v := tm.Values
	restore := useFonts(tm.Fonts)
	defer restore()
	lineH := v.TableText * lineHeightMult()
	plain := lineH + 2*v.TableCellPadY
	// 本体の行(群の行)と合計の行はどれも1行の高さ(群の最後の行の下に空きが無い)。
	for r := 1; r < len(hs); r++ {
		if d := hs[r] - plain; d > 0.01 || d < -0.01 {
			t.Errorf("row %d height = %.2fpt, want %.2fpt (rows %v)", r, hs[r], plain, hs)
		}
	}
	// 帯の列は縦の2行。「拠点の館」は群の2行の高さで2行に折れ、帯の幅に入る。
	if want := 2*lineH + 2*v.TableCellPadX; cols[0] < want-0.01 {
		t.Errorf("band width = %.2fpt, want >= %.2fpt", cols[0], want)
	}
	label := []TextRun{{Text: "拠点の館", Bold: true}}
	have := hs[1] + hs[2] - 2*v.TableCellPadY
	if n := runsLines(label, v.TableText, have); float64(n)*lineH > cols[0]-2*v.TableCellPadX+0.01 {
		t.Errorf("group label needs %d vertical lines in %.2fpt; band holds %.2fpt", n, have, cols[0]-2*v.TableCellPadX)
	}
	// 合計の行:名前のセルが2列をまたぎ、またいだ幅に1行で入る。
	rows := tableCellsXML(slide)
	total := rows[len(rows)-1]
	if !strings.HasPrefix(total[0], ` gridSpan="2"`) || !strings.Contains(total[0], "合計") {
		t.Errorf("total label cell must span 2 columns: %.120s", total[0])
	}
	if !strings.HasPrefix(total[1], ` hMerge="1"`) {
		t.Errorf("second total cell must be merged: %.120s", total[1])
	}
	name := []TextRun{{Text: "合計", Bold: true}}
	if w := naturalRunsWidth(name, v.TableText) + 2*v.TableCellPadX; cols[0]+cols[1] < w {
		t.Errorf("spanned width %.2fpt < total label %.2fpt", cols[0]+cols[1], w)
	}
	if n := runsLines(name, v.TableText, cols[0]+cols[1]-2*v.TableCellPadX); n != 1 {
		t.Errorf("total label lines = %d, want 1", n)
	}
}

// 帯を縦の2行に広げても入らない長い名前は、群の行に等しく高さを足し(最後の行だけを
// 高くしない)、check が table-group-label を出す。
const longGroupLabelDeck = `---
title: 長い群の名前
---

# 窓口の見直し

| 群 | 項目 | いま |
| --- | --- | --- |
| とても長い群の名前の例です | 書類の確認 | 二度来る |
| | 予約 | 電話だけ |
| 体制 | 窓口 | 二つ |
| 合計 | | 三つ |
: 架空の窓口の記録 {groups=first total=last}
`

func TestReportLongGroupLabelSpreadsAndWarns(t *testing.T) {
	slide := builtSlideXML(t, longGroupLabelDeck, "report", 2)
	hs := emuList(t, trHeightRe, slide)
	if len(hs) != 5 {
		t.Fatalf("rows %v", hs)
	}
	if d := hs[1] - hs[2]; d > 0.01 || d < -0.01 {
		t.Errorf("group rows must grow equally: %v", hs)
	}
	if hs[1] <= hs[3]+0.01 {
		t.Errorf("group rows must be taller than a plain row: %v", hs)
	}
	res := Check("c.bx.md", []byte(longGroupLabelDeck), CheckOptions{Template: "report"})
	found := false
	for _, w := range res.Warnings {
		if w.Code == "table-group-label" {
			found = true
			if w.Path != "/slides/1/body/0" || w.Hint == "" {
				t.Errorf("warning = %+v", w)
			}
		}
	}
	if !found {
		t.Errorf("no table-group-label: %+v", res.Warnings)
	}
	// dark・light は横書きのまま(警告も合計のまたぎも無い)。
	if s := builtSlideXML(t, longGroupLabelDeck, "dark", 2); strings.Contains(s, "gridSpan") {
		t.Error("dark: total must not span")
	}
}
