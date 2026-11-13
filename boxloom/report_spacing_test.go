package boxloom

import (
	"testing"
)

// report の対の並びと表の余白(Backlog #103)の試験。例は架空のもの。

// reportSpacingPairsDeck は見出しつきの側の対が3行のスライド1枚。
const reportSpacingPairsDeck = `---
title: 対の余白の試し
---

# 窓口の待ち時間を見直す

:::::pairs 分かったこと | 打ち手
:::subject 午前に混む
- 月曜と金曜の午前に来る人が全体の半分を占める
:::
:::response 予約の枠を広げる
- 午後の枠を倍にする
:::
:::subject 書類が足りない
- 来た人の三割が書類の不足で二度来ている
:::
:::response 一覧を前もって送る
- 要る書類の一覧を予約の確認と一緒に送る
:::
:::subject 待つ場所が狭い
- 待合の椅子は 12 脚で、昼前は立って待つ人が出る
:::
:::response 呼び出しを知らせる
- 番号を携帯に知らせ、外で待てるようにする
:::
:::::
`

// reportSpacingTableDeck は6行の表のスライド1枚。
const reportSpacingTableDeck = `---
title: 表の余白の試し
---

# 端末は古い世代から順に入れ替える

| 世代 | 台数 | 保守の期限 | 入れ替えの方針 |
| --- | --- | --- | --- |
| 第1世代 | 120台 | 2027-03 | 先に止める。代わりは第3世代 |
| 第2世代 | 85台 | 2029-12 | 2028年度に半数、残りは翌年度 |
| 第3世代 | 40台 | 2032-06 | そのまま使う |
| 第4世代 | 6台 | — | 試験の結果を見て決める |
| 第5世代 | 3台 | — | 試験の結果を見て決める |
| 第6世代 | 1台 | — | 次の年度に決める |
`

// reportBodyNeed は tm でデッキの最後のスライドを組み、本文の箱の要る高さを返す。
func reportBodyNeed(t *testing.T, tm *Template, src string) float64 {
	t.Helper()
	d, _, err := ParseMarkdown("s.bx.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	b := built[len(built)-1]
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	return heightForWidth(b.Body, b.Body.Box.Width)
}

// report の対の並び(3行)と表は、#103 の前の余白(対の行の間 blockGap・側の見出しの
// 薄い面の帯・セルの余白 左右 6pt 上下 4pt)より低く組める。
func TestReportPairsTableSpacing(t *testing.T) {
	// 前の report:側の見出しは帯、対の行の間は blockGap、セルの余白は 6・4。
	before := func() *Template {
		tm := mustTemplate("report")
		old := reportPairsStyle
		old.rowGap = ""
		old.sideTitle = func(t *Template, title Inline) *Spec {
			return Col([]*Spec{t.text(title, "body", "text_primary", true)},
				tokBackground("card"), tokPadXY(spaceStep(4), spaceStep(2)))
		}
		tm.components.components[pairsType] = componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
			return pairsStyled(ctx, b, old)
		})
		tm.Values.TableCellPadX, tm.Values.TableCellPadY = 6, 4
		return tm
	}
	for _, c := range []struct {
		name, src string
		minSaved  float64
	}{
		// 対の行の間 2 つ × (18 − 4) × 2 と、見出しの帯の上下の余白 4 × 2 × 6 側。
		{"pairs", reportSpacingPairsDeck, 40},
		// 7 行 × 上下の余白 (4 − 2) × 2。
		{"table", reportSpacingTableDeck, 20},
	} {
		was := reportBodyNeed(t, before(), c.src)
		now := reportBodyNeed(t, mustTemplate("report"), c.src)
		t.Logf("%s: need %.1f -> %.1f", c.name, was, now)
		if now > was-c.minSaved {
			t.Errorf("%s: need %.1f, want at most %.1f (before %.1f)", c.name, now, was-c.minSaved, was)
		}
	}
	// 測った余白(docs/template-values.md の report の節)。
	v := mustTemplate("report").Values
	if v.TableCellPadX != 4 || v.TableCellPadY != 2 {
		t.Errorf("table cell padding %v・%v, want 4・2", v.TableCellPadX, v.TableCellPadY)
	}
}
