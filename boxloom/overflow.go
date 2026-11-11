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

package boxloom

import "fmt"

// overflowEps より小さいはみ出しは丸めの誤差として見ない(pt)。
const overflowEps = 0.5

// BlockOverflow は1つのブロックのはみ出し。
type BlockOverflow struct {
	Index    int     // 本文の中の何番目のブロックか
	Need     float64 // そのブロックが要る高さ
	Overflow float64 // 本文の領域の下端を越えた高さ
}

// BlockMeasure は本文の1つのブロックを縮めずに積んだときの測り値。
// 容量の計測(capacity)とはみ出しの検査は、どちらもこれから作る。
type BlockMeasure struct {
	Index    int     // 本文の中の何番目のブロックか
	Need     float64 // 縮めずに要る高さ
	Lines    int     // 中の文字の行の数(折り返し後。行(Row)の中は一番多い列)
	Top      float64 // 本文の領域の上端からの位置(gap を含めて積んだもの)
	Bottom   float64 // Top + Need
	Overflow float64 // 本文の領域の下端を越えた高さ(越えなければ 0)
	Type     string  // IR の type(並列のまとまりの中のブロックだけ。本文のブロックは kinds で渡す)

	// Columns は並列のまとまり(columns)のときだけ、まとまりごとの測り値。
	// ブロックの Need は一番高いまとまりの Need。
	Columns []ColumnMeasure
}

// ColumnMeasure は並列のまとまりの1つの測り値。Top・Bottom は本文の領域の
// 上端から(ブロックと同じ)。
type ColumnMeasure struct {
	Index    int
	Need     float64 // 題と本文のブロックを縮めずに積んだ高さ
	Lines    int
	Top      float64
	Bottom   float64
	Overflow float64
	Blocks   []BlockMeasure // まとまりの本文のブロック(Index はまとまりの中の番号)
}

// measureBody は本文の各ブロックを縮めずに積み、本文の領域の高さ(capacity)と
// ブロック全体が要る高さ(need)を返す。root はページの箱(Col)で、body はその
// 直接の子。文字の折り返しを測るために行(Row)を仮に並べ直して Box を上書き
// するので、描いた後に呼ぶこと。ok が false なら測れない骨組み。
func measureBody(root, body *Spec) (blocks []BlockMeasure, capacity, need float64, ok bool) {
	if root == nil || body == nil || root.Kind != KindCol {
		return nil, 0, 0, false
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
		return nil, 0, 0, false
	}
	capacity -= 2 * body.Padding

	cursor := 0.0
	for i, b := range body.Children {
		if i > 0 {
			cursor += body.Gap
		}
		h, lines := requiredMeasure(b, widths[b])
		start, end := cursor, cursor+h
		cursor = end
		m := BlockMeasure{Index: i, Need: h, Lines: lines, Top: start, Bottom: end}
		m.Overflow = overflowOf(start, end, capacity)
		if b.columns != nil {
			m.Columns = measureColumns(b.columns, start, capacity)
		}
		blocks = append(blocks, m)
	}
	return blocks, capacity, cursor, true
}

// overflowOf は start〜end に積んだものが、本文の領域(capacity)の下端を越えた高さ。
func overflowOf(start, end, capacity float64) float64 {
	switch {
	case end <= capacity+overflowEps:
		return 0
	case start > capacity:
		return end - start
	}
	return end - capacity
}

// measureColumns は並列のまとまりを1つずつ、題と本文のブロックを縮めずに積んで
// 測る(top はブロックの上端)。まとまりの幅は requiredMeasure が並べた Box から読む。
func measureColumns(info *columnsInfo, top, capacity float64) []ColumnMeasure {
	var out []ColumnMeasure
	for ci, col := range info.cols {
		w := col.spec.Box.Width
		cm := ColumnMeasure{Index: ci, Top: top}
		cursor := top
		if col.title != nil {
			h, n := requiredMeasure(col.title, w)
			cursor += h + col.spec.Gap
			cm.Lines += n
		}
		for bi, b := range col.body.Children {
			if bi > 0 {
				cursor += col.body.Gap
			}
			h, n := requiredMeasure(b, w)
			bm := BlockMeasure{Index: bi, Need: h, Lines: n, Top: cursor, Bottom: cursor + h, Type: b.irType}
			bm.Overflow = overflowOf(bm.Top, bm.Bottom, capacity)
			cm.Blocks = append(cm.Blocks, bm)
			cm.Lines += n
			cursor += h
		}
		cm.Bottom, cm.Need = cursor, cursor-top
		cm.Overflow = overflowOf(top, cursor, capacity)
		out = append(out, cm)
	}
	return out
}

// measureBodyOverflow は本文の各ブロックのはみ出しと、本文の領域の高さ・
// ブロック全体が要る高さを返す(measureBody のうち、はみ出したものだけ)。
func measureBodyOverflow(root, body *Spec) (over []BlockOverflow, capacity, need float64) {
	blocks, capacity, need, ok := measureBody(root, body)
	if !ok {
		return nil, 0, 0
	}
	for _, b := range blocks {
		if b.Overflow > 0 {
			over = append(over, BlockOverflow{Index: b.Index, Need: b.Need, Overflow: b.Overflow})
		}
	}
	return over, capacity, need
}

// lintBodyOverflow は measureBody の結果を警告の文にする。
// kinds はブロックの種類の名前(IR の type。無ければ空)。
func lintBodyOverflow(root, body *Spec, kinds []string) []string {
	blocks, capacity, need, ok := measureBody(root, body)
	if !ok {
		return nil
	}
	return overflowWarnings(blocks, capacity, need, kinds)
}

// overflowWarnings ははみ出したブロックごとに警告の文を1つ作る。
func overflowWarnings(blocks []BlockMeasure, capacity, need float64, kinds []string) []string {
	var out []string
	for _, o := range blocks {
		if o.Overflow <= 0 {
			continue
		}
		kind := ""
		if o.Index < len(kinds) && kinds[o.Index] != "" {
			kind = " (" + kinds[o.Index] + ")"
		}
		if len(o.Columns) > 0 {
			// 並列のまとまりは、まとまりの中ではみ出したブロックごとに出す
			// (一番高いまとまりがブロックの高さを決める)。
			for _, c := range o.Columns {
				for _, ib := range c.Blocks {
					if ib.Overflow <= 0 {
						continue
					}
					out = append(out, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] body[%d]: columns[%d]: body[%d]: block (%s) in column %d overflows the body area by %.1fpt (block needs %.1fpt; column needs %.1fpt; body area %.1fpt, all blocks need %.1fpt)",
						o.Index, c.Index, ib.Index, ib.Type, c.Index, ib.Overflow, ib.Need, c.Need, capacity, need))
				}
			}
			continue
		}
		out = append(out, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] body[%d]: block%s overflows the body area by %.1fpt (block needs %.1fpt; body area %.1fpt, all blocks need %.1fpt)",
			o.Index, kind, o.Overflow, o.Need, capacity, need))
	}
	return out
}

// requiredHeight は、幅を決めたときに縮めずに要る高さ。heightForWidth と同じ
// 測り方で、サブシステムだけは自然な高さではなく最小の高さ(Min)を使う。
func requiredHeight(s *Spec, width float64) float64 {
	h, _ := requiredMeasure(s, width)
	return h
}

// requiredMeasure は requiredHeight と、その中の文字の行の数。行の数は、列(Col)
// では子の和、行(Row)では一番多い子(横に並ぶので、高さを決めるのはそれ)。
// サブシステムは文字の行を数えない(0)。
func requiredMeasure(s *Spec, width float64) (float64, int) {
	if s.Height != nil {
		return *s.Height, 0
	}
	switch s.Kind {
	case KindText:
		return textHeightFor(s, width), textLinesFor(s, width)
	case KindSubsystem:
		if s.Subsystem == nil {
			return 0, 0
		}
		lines := 0
		if l, ok := s.Subsystem.(textLiner); ok {
			lines = l.textLines(width)
		}
		return s.Subsystem.Measure(Constraint{Width: width}).Min.Height, lines
	}
	if len(s.Children) == 0 {
		return 2 * s.Padding, 0
	}
	inner := max0(width - 2*s.Padding)
	if s.Kind == KindCol {
		total := s.Gap * float64(max(0, len(s.Children)-1))
		lines := 0
		for _, c := range s.Children {
			cw := inner
			if c.Width != nil {
				cw = *c.Width
			} else if s.AlignItems != AlignStretch {
				cw = naturalSize(c, 'w')
			}
			h, n := requiredMeasure(c, cw)
			total += h
			lines += n
		}
		return total + 2*s.Padding, lines
	}
	// 行: 子の幅を決めるために一度並べる(Box を上書きする)。
	computeLayout(s, 0, 0, width, naturalSize(s, 'h'))
	m, lines := 0.0, 0
	for _, c := range s.Children {
		v, n := requiredMeasure(c, c.Box.Width)
		if v > m {
			m = v
		}
		if n > lines {
			lines = n
		}
	}
	return m + 2*s.Padding, lines
}

// textLinesFor は文字の葉を幅 width で折り返したときの行の数。
func textLinesFor(s *Spec, width float64) int {
	if len(s.Runs) > 0 {
		return len(wrapRuns(s.Runs, s.FontSize, width))
	}
	return len(wrapText(s.Text, s.FontSize, width, s.Bold))
}
