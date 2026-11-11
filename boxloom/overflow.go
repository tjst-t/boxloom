// layout-overflow: 本文がページ(本文の領域)に収まらないときの検査。
//
// レイアウトエンジンは、足りないとき子を比例で縮めて必ず箱の中に収める
// (Python 版から引き継いだふるまい)。そのため並べた後の Box を見ても
// はみ出しは見えず、文字が重なったり切れたりするだけになる。ここでは
// 「縮めなければ要る高さ」を測り直し、本文の領域と比べる。
//
//   - 本文の領域 = ページの箱の内側の高さ − 本文以外の子(題・フッター)が
//     要る高さ − 子の間の gap。題やフッターも縮められているので、並べた後の
//     本文の箱ではなく、こう計算する。
//   - ブロックが要る高さ = 並べた幅で文字を折り返した高さ。サブシステムは
//     自己申告の最小の高さ(Min)まで縮めてよいので Min で測る。
//   - ブロックを上から積み、本文の領域の下端を越えた分を、そのブロックの
//     はみ出しとして返す(越えたブロックごとに1件)。
//
// 警告の文は "[LAYOUT-OVERFLOW WARNING] body[2]: ..." の形。check は
// body[N] を IR の場所(/slides/S/body/N)に変える。
package main

import "fmt"

// overflowEps より小さいはみ出しは丸めの誤差として見ない(pt)。
const overflowEps = 0.5

// BlockOverflow は1つのブロックのはみ出し。
type BlockOverflow struct {
	Index    int     // 本文の中の何番目のブロックか
	Need     float64 // そのブロックが要る高さ
	Overflow float64 // 本文の領域の下端を越えた高さ
}

// measureBodyOverflow は本文の各ブロックのはみ出しと、本文の領域の高さ・
// ブロック全体が要る高さを返す。root はページの箱(Col)で、body はその直接の子。
// 文字の折り返しを測るために行(Row)を仮に並べ直して Box を上書きするので、
// 描いた後に呼ぶこと。
func measureBodyOverflow(root, body *Spec) (over []BlockOverflow, capacity, need float64) {
	if root == nil || body == nil || root.Kind != KindCol {
		return nil, 0, 0
	}
	// 子の幅は Box から読むので、Box を上書きする前に控えておく。
	widths := map[*Spec]float64{}
	for _, c := range root.Children {
		widths[c] = c.Box.Width
	}
	for _, b := range body.Children {
		widths[b] = b.Box.Width
	}
	capacity = root.Box.Height - 2*root.Padding - root.Gap*float64(max(0, len(root.Children)-1))
	found := false
	for _, c := range root.Children {
		if c == body {
			found = true
			continue
		}
		capacity -= requiredHeight(c, widths[c])
	}
	if !found {
		return nil, 0, 0
	}
	capacity -= 2 * body.Padding

	cursor := 0.0
	for i, b := range body.Children {
		if i > 0 {
			cursor += body.Gap
		}
		h := requiredHeight(b, widths[b])
		start, end := cursor, cursor+h
		cursor = end
		if end > capacity+overflowEps {
			o := end - capacity
			if start > capacity {
				o = h
			}
			over = append(over, BlockOverflow{Index: i, Need: h, Overflow: o})
		}
	}
	return over, capacity, cursor
}

// lintBodyOverflow は measureBodyOverflow の結果を警告の文にする。
// kinds はブロックの種類の名前(IR の type。無ければ空)。
func lintBodyOverflow(root, body *Spec, kinds []string) []string {
	over, capacity, need := measureBodyOverflow(root, body)
	var out []string
	for _, o := range over {
		kind := ""
		if o.Index < len(kinds) && kinds[o.Index] != "" {
			kind = " (" + kinds[o.Index] + ")"
		}
		out = append(out, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] body[%d]: block%s overflows the body area by %.1fpt (block needs %.1fpt; body area %.1fpt, all blocks need %.1fpt)",
			o.Index, kind, o.Overflow, o.Need, capacity, need))
	}
	return out
}

// requiredHeight は、幅を決めたときに縮めずに要る高さ。heightForWidth と同じ
// 測り方で、サブシステムだけは自然な高さではなく最小の高さ(Min)を使う。
func requiredHeight(s *Spec, width float64) float64 {
	if s.Height != nil {
		return *s.Height
	}
	switch s.Kind {
	case KindText:
		return textHeightFor(s, width)
	case KindSubsystem:
		if s.Subsystem == nil {
			return 0
		}
		return s.Subsystem.Measure(Constraint{Width: width}).Min.Height
	}
	if len(s.Children) == 0 {
		return 2 * s.Padding
	}
	inner := max0(width - 2*s.Padding)
	if s.Kind == KindCol {
		total := s.Gap * float64(max(0, len(s.Children)-1))
		for _, c := range s.Children {
			cw := inner
			if c.Width != nil {
				cw = *c.Width
			} else if s.AlignItems != AlignStretch {
				cw = naturalSize(c, 'w')
			}
			total += requiredHeight(c, cw)
		}
		return total + 2*s.Padding
	}
	// 行: 子の幅を決めるために一度並べる(Box を上書きする)。
	computeLayout(s, 0, 0, width, naturalSize(s, 'h'))
	m := 0.0
	for _, c := range s.Children {
		if v := requiredHeight(c, c.Box.Width); v > m {
			m = v
		}
	}
	return m + 2*s.Padding
}
