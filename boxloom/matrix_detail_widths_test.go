package boxloom

import (
	"testing"
	"time"
)

// matrixWideData は列の見出しが長めで、行ごとに長い要約を持つ matrix(元の資料の p28・29 の形を
// 架空の内容にしたもの)。評価のセルは印と短い文だけ。
func matrixWideData(detailColumn string) string {
	dc := ""
	if detailColumn != "" {
		dc = `"detailColumn": "` + detailColumn + `", `
	}
	cells := `[{"status": "support", "text": "可"}, {"status": "conditional"}, {"status": "danger", "text": "不可"}, {"status": "support"}]`
	return `{` + dc + `"columns": ["自社で構築する", "共同で利用する", "外部へ委託する", "今のまま続ける"], "rows": [
  {"label": "初期の費用と毎年の費用", "detail": "初年度に持ち出す費用と、毎年かかる保守と運用の費用を合わせて五年で比べる。人件費も含める", "cells": ` + cells + `},
  {"label": "切り替えの手間", "detail": ["夜間に切り替えて日中の業務を止めない", "週末の作業はできるだけ避ける", "戻す手順を先に試しておく"], "cells": ` + cells + `},
  {"label": "保守の期限", "detail": "今の機器の保守が切れる年と、新しい仕組みの保守が続く年を並べ、空白の期間が無いかを確かめる", "cells": ` + cells + `},
  {"label": "人の育成", "detail": "運用を担う人を社内で育てるのにかかる期間と、外の力を借りる場合の引き継ぎの手間", "cells": ` + cells + `}]}`
}

// detailColumn では評価の列は中身の自然な最小の幅(印・短い文・折り返した見出し)に留まり、
// 残りを題と要約の列が取る(#144)。列の中(500〜555pt)でも全幅でも、同じ内容で要約を
// 題の下に置くより Min の高さが小さい。
func TestMatrixDetailColumnWidths(t *testing.T) {
	for _, tmpl := range []string{"report", "light"} {
		restore := useFonts(mustTemplate(tmpl).Fonts)
		under := figureFromIR(t, tmpl, "matrix", matrixWideData("")).(*MatrixSubsystem)
		col := figureFromIR(t, tmpl, "matrix", matrixWideData("要約")).(*MatrixSubsystem)
		m := col.widths()
		widths := []float64{500, 555, 912} // 列の中と全幅
		for _, w := range widths {
			if w < m.minWidth(col.vals(), len(col.Data.Columns)) {
				t.Logf("%s width %v: below the Min width", tmpl, w)
				continue
			}
			start := time.Now()
			mu, mc := under.Measure(Constraint{Width: w}), col.Measure(Constraint{Width: w})
			if d := time.Since(start); d > 200*time.Millisecond {
				t.Errorf("%s width %v: Measure took %v", tmpl, w, d)
			}
			g := col.geom(w)
			t.Logf("%s width %v: Min height %.1f (detailColumn) vs %.1f (under); head %.1f detail %.1f col %.1f (floor %.1f nat %.1f)",
				tmpl, w, mc.Min.Height, mu.Min.Height, g.headW, g.detW, g.colW, m.colFloor, m.colNat)
			if !(mc.Min.Height < mu.Min.Height) {
				t.Errorf("%s width %v: Min height with detailColumn %.1f, under the label %.1f", tmpl, w, mc.Min.Height, mu.Min.Height)
			}
			// 評価の列は中身の幅(列の見出しは3行まで折り返す、#151)。広げても行は低くならない。
			if g.colW > m.colFloor+0.01 {
				t.Errorf("%s width %v: evaluation column %.1f, wider than its floor %.1f", tmpl, w, g.colW, m.colFloor)
			}
			// 題と要約の列が残りの幅を取る:要約の列は評価の列より広い。
			if !(g.detW > 2*g.colW && g.headW >= m.headFloor-0.01) {
				t.Errorf("%s width %v: head %.1f detail %.1f col %.1f", tmpl, w, g.headW, g.detW, g.colW)
			}
			// Natural の高さで描けば文ははみ出さない(Measure と描く形が同じ幅)。
			if warns := col.Lint(LBox{Width: w, Height: mc.Natural.Height}); len(warns) != 0 {
				t.Errorf("%s width %v: lint %v", tmpl, w, warns)
			}
			if warns := col.Lint(LBox{Width: w, Height: mc.Min.Height}); len(warns) != 0 {
				t.Errorf("%s width %v: lint at Min height %v", tmpl, w, warns)
			}
		}
		restore()
	}
}
