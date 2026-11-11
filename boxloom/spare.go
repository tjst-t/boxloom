// 横の余り(spare width):本文のブロックの中で、図(サブシステム)や画像が
// 使わずに残した横の幅。
//
// 縦横比を保つ図(FixedAspect の画像・shapes など)や、横の上限(Max)を持つ図は、
// 割り当てられた幅より細く描かれ、余りは Place の側(既定は右)に白く残る。
// 縦の埋まり(fill-ratio)では見えないこの空きを、容量(capacity の
// remainingWidth)と警告 spare-width で知らせる。直すのは作者で、Boxloom は
// 図を広げたり文を寄せたりはしない(columns で文を横に置けば埋まる)。
// 詳しくは docs/ir.md の「容量の計測」。

package boxloom

import (
	"fmt"
	"math"
)

// spareWidthWarnRatio は、横の余りがブロックの幅のこの割合以上なら警告する。
const spareWidthWarnRatio = 0.20

// spareWidthMin は、これより小さい余り(pt)は無いものとする(丸めの誤差)。
const spareWidthMin = 0.5

// blockSpareWidth は、ブロック b の中の図が左右に残した幅(pt)の合計。図の
// 描く矩形(subsystemDrawBox)の左端の最小〜右端の最大の外側を、ブロックの幅から
// 測る。図を持たないブロックと、並列のまとまり(columns。まとまりごとに幅が
// 違う)は 0。computeLayout の後に呼ぶ。
func blockSpareWidth(b *Spec) float64 {
	if b == nil || b.columns != nil || b.Box.Width <= 0 {
		return 0
	}
	left, right := math.Inf(1), math.Inf(-1)
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindSubsystem && s.Subsystem != nil {
			d := subsystemDrawBox(s)
			left = math.Min(left, d.X)
			right = math.Max(right, d.X+d.Width)
			return
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(b)
	if math.IsInf(left, 1) {
		return 0
	}
	spare := (left - b.Box.X) + (b.Box.X + b.Box.Width - right)
	if spare < spareWidthMin {
		return 0
	}
	return spare
}

// bodySpareWidths は本文のブロックごとの横の余り(余りの無いブロックは入れない)。
func bodySpareWidths(body *Spec) map[int]float64 {
	out := map[int]float64{}
	if body == nil {
		return out
	}
	for i, b := range body.Children {
		if w := blockSpareWidth(b); w > 0 {
			out[i] = w
		}
	}
	return out
}

// spareWidthWarnings は、横の余りがブロックの幅の spareWidthWarnRatio 以上の
// ブロックごとに警告の文を1つ作る。kinds はブロックの IR の type。
func spareWidthWarnings(body *Spec, spare map[int]float64, kinds []string) []string {
	var out []string
	for i, b := range body.Children {
		w, ok := spare[i]
		if !ok || b.Box.Width <= 0 || w/b.Box.Width < spareWidthWarnRatio-1e-9 {
			continue
		}
		kind := ""
		if i < len(kinds) && kinds[i] != "" {
			kind = " (" + kinds[i] + ")"
		}
		out = append(out, fmt.Sprintf("[SPARE-WIDTH WARNING] body[%d]: the figure in block%s leaves %.1fpt (%.0f%%) of the block width empty beside it (block width %.1fpt)",
			i, kind, w, 100*w/b.Box.Width, b.Box.Width))
	}
	return out
}

// spareKey は並列のまとまりの中のブロック(本文のブロック・まとまり・まとまりの中のブロック)。
type spareKey struct{ block, column, inner int }

// columnSpareWidths は並列のまとまり(columns)の中のブロックごとの横の余り。
// まとまりの中のブロックの幅はまとまりの幅なので、図(グラフ)がまとまりの幅を
// 使い切らないときの空きになる(余りの無いブロックは入れない)。
func columnSpareWidths(body *Spec) map[spareKey]float64 {
	out := map[spareKey]float64{}
	if body == nil {
		return out
	}
	for i, b := range body.Children {
		if b.columns == nil {
			continue
		}
		for ci, c := range b.columns.cols {
			if c.body == nil {
				continue
			}
			for k, inner := range c.body.Children {
				if w := blockSpareWidth(inner); w > 0 {
					out[spareKey{i, ci, k}] = w
				}
			}
		}
	}
	return out
}

// columnSpareWarnings は、まとまりの中のブロックの横の余りが、そのブロック(=まとまり)の
// 幅の spareWidthWarnRatio 以上のものごとに警告の文を1つ作る。
func columnSpareWarnings(body *Spec, spare map[spareKey]float64) []string {
	var out []string
	for i, b := range body.Children {
		if b.columns == nil {
			continue
		}
		for ci, c := range b.columns.cols {
			if c.body == nil {
				continue
			}
			for k, inner := range c.body.Children {
				w, ok := spare[spareKey{i, ci, k}]
				if !ok || inner.Box.Width <= 0 || w/inner.Box.Width < spareWidthWarnRatio-1e-9 {
					continue
				}
				kind := ""
				if inner.irType != "" {
					kind = " (" + inner.irType + ")"
				}
				out = append(out, fmt.Sprintf("[SPARE-WIDTH WARNING] body[%d]: columns[%d]: body[%d]: the figure in block%s leaves %.1fpt (%.0f%%) of the column width empty beside it (column width %.1fpt)",
					i, ci, k, kind, w, 100*w/inner.Box.Width, inner.Box.Width))
			}
		}
	}
	return out
}
