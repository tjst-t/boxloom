package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

// splitFitTable は Markdown の表(列は3つ。rows は本体の行の番号)。続きの表は見出しの行を
// 書かず、note なら表の後に注記を置く。
func splitFitTable(rows []int, continues, note bool) string {
	var b strings.Builder
	if continues {
		b.WriteString("| --- | --- | --- |\n")
	} else {
		b.WriteString("| 項目 | 説明 | 数 |\n| --- | --- | --- |\n")
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "| 項目%d | 移すときに気をつけることを少し長めに書いておく行の説明 %d | %d |\n", r, r, r*10)
	}
	if continues {
		b.WriteString(": 一覧 {continues}\n")
	} else {
		b.WriteString(": 一覧\n")
	}
	if note {
		b.WriteString("\n:::note\n移す前に部署の担当に知らせ、移した後の最初の月は古い仕組みと突き合わせる。\n:::\n")
	}
	return b.String()
}

func splitFitRange(from, to int) []int {
	var out []int
	for r := from; r < to; r++ {
		out = append(out, r)
	}
	return out
}

// splitFitDeck は表のスライドを並べた文書(parts はスライドごとの本文)。
func splitFitDeck(parts ...string) []byte {
	src := "---\ntitle: 一覧\n---\n"
	for _, p := range parts {
		src += "\n# 移す仕組みの一覧\n\n" + p
	}
	return []byte(src)
}

// 表の後に注記のあるスライドで、分け方の案は注記と blockGap の分を最初の枚から引く。
// 案のとおりに分ける(注記は最初の枚に残す)と、どの枚もはみ出さない(#140)。表は
// 収まって注記だけがはみ出すときも、注記の警告に案が付く。
func TestTableSplitCountsFollowingBlocks(t *testing.T) {
	for _, tm := range []string{"light", "report"} {
		proposed := 0
		for n := 8; n <= 24; n++ {
			data := splitFitDeck(splitFitTable(splitFitRange(0, n), false, true))
			res := Check("x.bx.md", data, CheckOptions{Template: tm})
			var over []Issue
			for _, w := range res.Warnings {
				if w.Code == codeLayoutOverflow {
					over = append(over, w)
				}
			}
			if len(over) == 0 {
				continue
			}
			var split *TableSplitCapacity
			for _, b := range res.Capacity[0].Blocks {
				if b.Path == "/slides/1/body/0" {
					split = b.Split
				}
			}
			if split == nil || split.Tables != 2 {
				continue
			}
			for _, w := range over {
				if !strings.Contains(w.Hint, "この表は 2 枚に分けられます") || !strings.Contains(w.Hint, "最初のスライドに残す") {
					t.Errorf("%s %d rows: %s hint %q", tm, n, w.Path, w.Hint)
				}
			}
			at := split.SplitBefore[0]
			fixed := splitFitDeck(splitFitTable(splitFitRange(0, at), false, true), splitFitTable(splitFitRange(at, n), true, false))
			after := Check("x.bx.md", fixed, CheckOptions{Template: tm})
			if !after.OK {
				t.Fatalf("%s %d rows: %+v", tm, n, after.Errors)
			}
			for _, w := range after.Warnings {
				if w.Code == codeLayoutOverflow || w.Code == "subsystem-min" {
					t.Errorf("%s %d rows split before rows[%d]: still overflows: %+v", tm, n, at, w)
				}
			}
			proposed++
		}
		if proposed == 0 {
			t.Errorf("%s: no row count gave a two-table proposal", tm)
		}
	}
}

// 続きの表の1枚がはみ出すと、-fit はひと続きの全部の枚を同じ段まで下げて収める。
// 列の幅と字は枚の間でそろう。はみ出さない枚だけを広げることもしない(#140)。
func TestFitShrinksWholeTableChain(t *testing.T) {
	for _, c := range []struct {
		tm   string
		rows int
	}{{"light", 15}, {"report", 21}} {
		tm := c.tm
		data := splitFitDeck(splitFitTable(splitFitRange(0, 4), false, false), splitFitTable(splitFitRange(4, 4+c.rows), true, false))
		plain := Check("x.bx.md", data, CheckOptions{Template: tm})
		overflows := map[string]bool{}
		for _, w := range plain.Warnings {
			if w.Code == codeLayoutOverflow {
				overflows[fitSlidePath(w.Path)] = true
			}
		}
		if !overflows["/slides/2"] || overflows["/slides/1"] {
			t.Fatalf("%s: want only the continued slide to overflow without -fit: %+v", tm, plain.Warnings)
		}
		res := Check("x.bx.md", data, CheckOptions{Template: tm, Fit: true})
		for _, w := range res.Warnings {
			if w.Code == codeLayoutOverflow {
				t.Errorf("%s: -fit still overflows: %+v", tm, w)
			}
		}
		if len(res.Info) != 2 || res.Info[0].Path != "/slides/1" || res.Info[1].Path != "/slides/2" ||
			!strings.Contains(res.Info[0].Message, "continued table") {
			t.Fatalf("%s: want fit-applied on both slides of the chain: %+v", tm, res.Info)
		}
		if len(res.Capacity) != 2 || res.Capacity[0].Fit == "" || res.Capacity[0].Fit != res.Capacity[1].Fit {
			t.Fatalf("%s: both slides want the same step: %+v", tm, res.Capacity)
		}

		deck, err := ReadDeck("x.bx.md", data)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := Build(&buf, deck, BuildOptions{Template: tm, Fit: true}); err != nil {
			t.Fatalf("%s: -fit build: %v", tm, err)
		}
		grids, sizes := tableLooks(t, buf.Bytes())
		if len(grids) != 2 || grids[0] != grids[1] || sizes[0] != sizes[1] {
			t.Errorf("%s: the pages differ: grid %q, sizes %q", tm, grids, sizes)
		}
	}
}

// 続きの表の最後の枚が薄くても、-fit はその枚だけを広げない(字がそろわなくなる)。
func TestFitDoesNotGrowTableChain(t *testing.T) {
	data := splitFitDeck(splitFitTable(splitFitRange(0, 12), false, false), splitFitTable(splitFitRange(12, 13), true, false))
	res := Check("x.bx.md", data, CheckOptions{Template: "light", Fit: true})
	if len(res.Info) != 0 {
		t.Errorf("want no fit-applied on a chain that fits: %+v", res.Info)
	}
}

// tableLooks は PPTX の表ごとの列の幅(a:gridCol の並び)と、セルの字の大きさの並び。
func tableLooks(t *testing.T, pptx []byte) (grids, sizes []string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	gridRe := regexp.MustCompile(`<a:tblGrid>.*?</a:tblGrid>`)
	tblRe := regexp.MustCompile(`<a:tbl>.*?</a:tbl>`)
	szRe := regexp.MustCompile(`sz="[0-9]+"`)
	for i := 1; ; i++ {
		f, err := zr.Open(fmt.Sprintf("ppt/slides/slide%d.xml", i))
		if err != nil {
			break
		}
		b, _ := io.ReadAll(f)
		f.Close()
		for _, tbl := range tblRe.FindAllString(string(b), -1) {
			grids = append(grids, gridRe.FindString(tbl))
			szs := map[string]bool{}
			for _, s := range szRe.FindAllString(tbl, -1) {
				szs[s] = true
			}
			sizes = append(sizes, fmt.Sprint(szs))
		}
	}
	return grids, sizes
}
