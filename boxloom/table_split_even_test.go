package boxloom

import (
	"fmt"
	"strings"
	"testing"
)

// evenTableRows は report の表の本体の行(群の頭・手立ての点の数 pts)。groups は群ごとの
// 行の数(1つなら群の無い表)。
func evenTableRows(groups []int, pts int, from, to int) string {
	var b strings.Builder
	r := 0
	for g, n := range groups {
		for k := 0; k < n; k++ {
			if r >= from && r < to {
				head := ""
				if len(groups) > 1 && (k == 0 || r == from) {
					head = fmt.Sprintf("群%d", g+1)
				}
				var ps []string
				for p := 0; p < pts; p++ {
					ps = append(ps, fmt.Sprintf("手立て%dの%d番を進める", r+1, p+1))
				}
				fmt.Fprintf(&b, "| %s | 課題%dが残る | 地区館4つ | 18時に閉まる | %s |\n", head, r+1, strings.Join(ps, ";"))
			}
			r++
		}
	}
	return b.String()
}

// evenTableDeck は表を分け方 cuts(2枚目からの表が始まる本体の行)のとおりに分けた文書
// (cuts が空なら分けない)。各枚に同じ題とリードを置く。
func evenTableDeck(groups []int, pts int, total bool, cuts []int) []byte {
	n := 0
	for _, g := range groups {
		n += g
	}
	attr := "points=手立て"
	if len(groups) > 1 {
		attr = "groups=first " + attr
	}
	bounds := append(append([]int{0}, cuts...), n)
	src := "---\ntitle: 課題\ntemplate: report\n---\n"
	for k := 0; k+1 < len(bounds); k++ {
		last := k+2 == len(bounds)
		src += "\n# 課題の一覧\n\n:::lead\n課題は4つの群に分かれ、手立ては群ごとに2〜3ある。\n:::\n\n"
		if k == 0 {
			src += "| 群 | 課題 | 影響する館 | いまの状態 | 手立て |\n"
		}
		src += "| --- | --- | --- | --- | --- |\n" + evenTableRows(groups, pts, bounds[k], bounds[k+1])
		a := attr
		if total && last {
			src += "| 合計 | 12件 | | | 24の手立て |\n"
			a += " total=last"
		}
		if k > 0 {
			a = "continues " + a
		}
		src += ": 作業部会の整理 {" + a + "}\n"
	}
	return []byte(src)
}

func evenTableSplit(t *testing.T, res *CheckResult) *TableSplitCapacity {
	t.Helper()
	for _, b := range res.Capacity[0].Blocks {
		if b.Type == "table" && b.Split != nil {
			return b.Split
		}
	}
	t.Fatalf("no split proposal: %s", jsonOf(res.Warnings))
	return nil
}

// 13行の表がはみ出すと、分け方の案は前から詰めず(8・5 で続きの枚が薄くなる)、各枚の行を
// ならす。案のとおりに分けた2枚は、はみ出さず、どちらも fill-ratio を出さない(#169)。
func TestTableSplitEvensRows(t *testing.T) {
	groups := []int{13}
	res := Check("x.bx.md", evenTableDeck(groups, 3, false, nil), CheckOptions{})
	split := evenTableSplit(t, res)
	if split.Tables != 2 || len(split.Fill) != 2 {
		t.Fatalf("split = %+v, want 2 tables", split)
	}
	if split.Rows[0]-split.Rows[1] > 1 || split.Rows[1]-split.Rows[0] > 1 {
		t.Errorf("rows = %v, want even rows (7・6), not the greedy 8・5", split.Rows)
	}
	for _, f := range split.Fill {
		if f < 65 {
			t.Errorf("fill = %v, want every slide ≥ 65%%", split.Fill)
		}
	}
	after := Check("x.bx.md", evenTableDeck(groups, 3, false, split.SplitBefore), CheckOptions{})
	if len(after.Errors) > 0 {
		t.Fatalf("errors after the split: %s", jsonOf(after.Errors))
	}
	for _, w := range after.Warnings {
		if w.Code == "fill-ratio" || w.Code == codeLayoutOverflow {
			t.Errorf("after the split: %s %s: %s", w.Code, w.Path, w.Message)
		}
	}
	if len(after.Capacity) != 2 {
		t.Fatalf("slides after the split = %d, want 2", len(after.Capacity))
	}
	// 見込みの埋まりは分けた後の本文の埋まりと同じ。
	for k, c := range after.Capacity {
		if got := round1((c.BodyArea - c.Remaining) / c.BodyArea * 100); got-split.Fill[k] > 0.2 || split.Fill[k]-got > 0.2 {
			t.Errorf("slide %d fill = %v, proposal said %v", k+1, got, split.Fill[k])
		}
	}
}

// 群の表(R3 の形:群は 2・3・4・3 行と合計の行の13行)は群の境で分ける。前から詰めると
// 9・3 で続きの枚が薄いので、行をならして 5・7。それでも最初の枚が下限を下回るなら、
// hint はそう言い、ほかの直し方を勧める(#169)。
func TestTableSplitEvensGroupsAndWarnsThin(t *testing.T) {
	groups := []int{2, 3, 4, 3}
	res := Check("x.bx.md", evenTableDeck(groups, 2, true, nil), CheckOptions{})
	split := evenTableSplit(t, res)
	if jsonOf(split.SplitBefore) != "[5]" || jsonOf(split.Rows) != "[5,7]" {
		t.Fatalf("split = %+v, want before rows[5] (5・7)", split)
	}
	hint := ""
	for _, w := range res.Warnings {
		if w.Code == codeLayoutOverflow {
			hint = w.Hint
		}
	}
	if !strings.Contains(hint, "1 枚目の埋まりが") || !strings.Contains(hint, "fill-ratio の下限(65%)を下回ります") {
		t.Errorf("hint does not tell the thin slide: %s", hint)
	}
	// 群を 3・3・3・4 行にすると、6・7 で分けてどちらも下限に届き、hint は薄い枚を言わない。
	res = Check("x.bx.md", evenTableDeck([]int{3, 3, 3, 4}, 3, false, nil), CheckOptions{})
	split = evenTableSplit(t, res)
	for _, w := range res.Warnings {
		if w.Code == codeLayoutOverflow && strings.Contains(w.Hint, "下回ります") {
			t.Errorf("hint tells a thin slide for %+v: %s", split, w.Hint)
		}
	}
	after := Check("x.bx.md", evenTableDeck([]int{3, 3, 3, 4}, 3, false, split.SplitBefore), CheckOptions{})
	for _, w := range append(after.Errors, after.Warnings...) {
		t.Errorf("after the split %v: %s %s: %s", split.SplitBefore, w.Code, w.Path, w.Message)
	}
}

// -fit の分けも同じ規則で各枚をならす:前から詰めると 34 行の表は 10・10・10・4 行で最後の
// 枚が薄い(44%)。ならすと、どの枚も fillRatioMin(65%)を下回らない(#169)。
func TestFitSplitEvensParts(t *testing.T) {
	res := Check("x.bx.md", evenTableDeck([]int{34}, 3, false, nil), CheckOptions{Fit: true})
	if len(res.Capacity) != 4 {
		t.Fatalf("parts = %d, want 4: %s", len(res.Capacity), jsonOf(res.Capacity))
	}
	for k, c := range res.Capacity {
		if c.Split == nil || c.Split.Part != k+1 {
			t.Fatalf("part %d: split %+v", k+1, c.Split)
		}
		if f := (c.BodyArea - c.Remaining) / c.BodyArea; f < 0.65 {
			t.Errorf("part %d fill = %.1f%%, want ≥ 65%% (starts %v)", k+1, f*100, c.Split.Starts)
		}
	}
}
