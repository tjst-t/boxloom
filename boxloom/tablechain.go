// 長い表を次のスライドへ続ける(continues。docs/ir.md の 14章)。
//
// Boxloom は表を自動で次のスライドへ送らない(IR の1スライド = PPTX の1枚を保つ)。
// 作者が表を分け、続きのスライドの表に continues: true を書く。続きの表は見出しの
// 行を書かず、Boxloom が最初の表の見出しの行を繰り返す。
//
// ひと続き(tableChain)の決まり:
//
//   - 続きの表は本文のブロック(並列のまとまりの中ではない)で、そのスライドの
//     本文の最初の表。
//   - 前の表は、すぐ前のスライドの本文の最後の表(間に別のスライドを挟まない)。
//   - 列の数・groups の有無は最初の表に従う。合計の行(total)はひと続きの最後の表だけ。
//
// Boxloom がそろえるもの:各枚の見出しの行、「n/m」の印(marks.tableContinued)、
// 列の幅(ひと続きの全部の行を合わせて測る)、数の列の右揃え。

package boxloom

import (
	"errors"
	"fmt"
	"slices"
)

// tableChainError は続きの表の形の外れ(Markdown の誤りの code と hint に使う)。
type tableChainError struct {
	Code string
	Hint string
	Err  error
}

func (e *tableChainError) Error() string { return e.Err.Error() }
func (e *tableChainError) Unwrap() error { return e.Err }

var errTableShape = errors.New("the rows follow the header of the first table")

const continuesHint = "続きの表({continues})は、すぐ前のスライドの最後の表に続けます。間にスライドを挟まず、列の数と groups は最初の表と同じにし、合計の行は最後の表だけに書きます"

// tableChain は続きの表のひと続き。Tables[0] が最初の表。
type tableChain struct {
	Tables []*IRBlock
}

func (c *tableChain) first() *IRBlock { return c.Tables[0] }

// tableHeader は表の見出しの行(続きの表は最初の表のもの)。
func (b *IRBlock) tableHeader() []Inline {
	if b.chain != nil {
		return b.chain.first().Header
	}
	return b.Header
}

// tableGrouped は表が先頭の列に行の群を持つか(続きの表は最初の表に従う)。
func (b *IRBlock) tableGrouped() bool {
	if b.chain != nil {
		return b.chain.first().Groups != nil
	}
	return b.Groups != nil
}

// tableGroupCols は群の列の番号(左から。先頭の列の groups は 0。続きの表は最初の表に従う)。
func (b *IRBlock) tableGroupCols() []int {
	f := b
	if b.chain != nil {
		f = b.chain.first()
	}
	return groupColumnsOf(f.Groups, f.GroupColumns)
}

// groupColumnsOf は groups と groupColumns の群の列の番号(左から)。
func groupColumnsOf(gs []IRTableGroup, gcs []IRTableGroupColumn) []int {
	var out []int
	if gs != nil {
		out = append(out, 0)
	}
	for _, gc := range gcs {
		out = append(out, gc.Column)
	}
	return out
}

// tableColumnGroups はこの表の群の列ごとの群(列の番号 → 群)。
func (b *IRBlock) tableColumnGroups() map[int][]IRTableGroup {
	out := map[int][]IRTableGroup{}
	if b.Groups != nil {
		out[0] = b.Groups
	}
	for _, gc := range b.GroupColumns {
		out[gc.Column] = gc.Groups
	}
	return out
}

// bodyTables は本文の表(並列のまとまりの中は数えない)の番号。
func bodyTables(s *IRSlide) []int {
	var out []int
	for k := range s.Body {
		if s.Body[k].Type == "table" {
			out = append(out, k)
		}
	}
	return out
}

// linkTableChains は続きの表をひと続きにつなぎ、形の外れを誤りにする。何度呼んでも
// 同じ結果になる(組み立てのたびに作り直す)。
func linkTableChains(d *IRDeck) error {
	if err := linkTableChainsRaw(d); err != nil {
		code := "table-continues"
		if errors.Is(err, errTableShape) {
			code = "table-shape"
		}
		return &tableChainError{Code: code, Hint: continuesHint, Err: err}
	}
	return nil
}

func linkTableChainsRaw(d *IRDeck) error {
	for i := range d.Slides {
		for k := range d.Slides[i].Body {
			b := &d.Slides[i].Body[k]
			b.chain, b.chainPos = nil, 0
			if b.Type != "columns" {
				continue
			}
			for ci := range b.Columns {
				for bi := range b.Columns[ci].Body {
					if ib := &b.Columns[ci].Body[bi]; ib.Type == "table" && ib.Continues {
						return fmt.Errorf("slides[%d]: body[%d]: columns[%d]: body[%d]: continues: %s", i, k, ci, bi, msgContinuesInColumns)
					}
				}
			}
		}
	}
	for i := range d.Slides {
		s := &d.Slides[i]
		for n, k := range bodyTables(s) {
			b := &s.Body[k]
			if !b.Continues {
				continue
			}
			at := fmt.Sprintf("slides[%d]: body[%d]", i, k)
			if n > 0 {
				return fmt.Errorf("%s: continues: %s", at, msgContinuesNotFirst)
			}
			var prev *IRBlock
			if i > 0 {
				if ts := bodyTables(&d.Slides[i-1]); len(ts) > 0 {
					prev = &d.Slides[i-1].Body[ts[len(ts)-1]]
				}
			}
			if prev == nil {
				return fmt.Errorf("%s: continues: %s", at, msgContinuesNoPrev)
			}
			if prev.Total != nil {
				return fmt.Errorf("slides[%d]: body[%d]: total: %s", i-1, indexOf(d.Slides[i-1].Body, prev), msgContinuesTotal)
			}
			if prev.chain == nil {
				prev.chain = &tableChain{Tables: []*IRBlock{prev}}
			}
			first := prev.chain.first()
			if (b.Groups != nil) != (first.Groups != nil) {
				return fmt.Errorf("%s: groups: %s", at, msgContinuesGroups(first.Groups != nil))
			}
			if !slices.Equal(groupColumnsOf(nil, b.GroupColumns), groupColumnsOf(nil, first.GroupColumns)) {
				return fmt.Errorf("%s: groupColumns: %s", at, msgContinuesGroupColumns)
			}
			if err := validateTableShape(first.Header, b.Rows, b.Total, b.Groups, b.GroupColumns); err != nil {
				return fmt.Errorf("%s: %w: %w", at, err, errTableShape)
			}
			b.chain, b.chainPos = prev.chain, len(prev.chain.Tables)
			prev.chain.Tables = append(prev.chain.Tables, b)
		}
	}
	return nil
}

func indexOf(blocks []IRBlock, b *IRBlock) int {
	for k := range blocks {
		if &blocks[k] == b {
			return k
		}
	}
	return -1
}

// 続きの形の外れの文(Go の読み込みと check で同じ文を使う)。
const (
	msgContinuesInColumns    = "a continuing table must be a block of the slide body, not inside columns"
	msgContinuesNotFirst     = "a continuing table must be the first table of its slide (it continues the last table of the previous slide)"
	msgContinuesNoPrev       = "the previous slide has no table to continue (a continuing table follows the last table of the slide right before it, with no other slide between)"
	msgContinuesGroupColumns = "a continuing table has the same group columns (groupColumns[].column) as the first table"
	msgContinuesTotal        = "only the last table of a continued table can have a total row (the next slide continues this table)"
)

func msgContinuesGroups(firstGrouped bool) string {
	if firstGrouped {
		return "the first table has groups, so every continuing table needs groups too"
	}
	return "the first table has no groups, so a continuing table cannot have groups"
}

// continuedMark は「n/m」の印の文(marks.tableContinued の {n} と {m} を置き換える)。
func (m TemplateMarks) continuedMark(n, total int) string {
	return replaceNM(m.TableContinued, n, total)
}

// --- はみ出した表の分け方の案 ---

// tableSplit ははみ出した表を何枚に分けられるかの案(check の容量と hint)。
// 分けるのは作者で、Boxloom は案を出すだけ。
type tableSplit struct {
	Tables      int   // 分けた後の表の数
	SplitBefore []int // 2つ目からの表が始まる本体の行(IR の rows の番号)
	Rows        []int // 表ごとの本体の行の数
}

// leftmostMergedColumn は縦に結合したセルのある、いちばん左の列(無ければ -1)。
func (t *tableLeaf) leftmostMergedColumn() int {
	best := -1
	for _, row := range t.cells {
		for c, cell := range row {
			if cell.VMerge && (best < 0 || c < best) {
				best = c
			}
		}
	}
	return best
}

// split は表を、最初の枚は firstAvail、続きの枚は laterAvail の高さに収まるよう
// 本体の行で分ける案(width は表の幅)。各枚は説明の文の行(と「n/m」の印)と
// 見出しの行を持つ。群の表は群の境目で分け、1つの群が1枚に入らないときだけ
// 群の中で分ける。合計の行は最後の枚。1枚に入る・見出しの行も入らないなら nil。
func (t *tableLeaf) split(width, firstAvail, laterAvail float64) *tableSplit {
	hs, _ := t.rowHeights(t.colWidths(width))
	probe := *t
	if probe.mark == nil {
		probe.mark = t.markProbe
	}
	over := probe.captionHeight(width) + hs[0]
	body := hs[1:]
	total := 0.0
	if t.draw.TotalRow {
		total, body = body[len(body)-1], body[:len(body)-1]
	}
	if len(body) < 2 || laterAvail-over <= 0 {
		return nil
	}
	// 分けられる単位(群の表はいちばん左の群の列の群、ほかは行)。
	gc := t.leftmostMergedColumn()
	var units [][2]int // [始めの行, 終わりの行(含まない)]
	for r := range body {
		if r > 0 && gc >= 0 && len(t.cells[r+1]) > gc && t.cells[r+1][gc].VMerge {
			units[len(units)-1][1] = r + 1
			continue
		}
		units = append(units, [2]int{r, r + 1})
	}
	height := func(u [2]int) float64 {
		h := 0.0
		for r := u[0]; r < u[1]; r++ {
			h += body[r]
		}
		if u[1] == len(body) {
			h += total
		}
		return h
	}
	var expanded [][2]int
	for _, u := range units {
		if height(u) > laterAvail-over && u[1]-u[0] > 1 {
			for r := u[0]; r < u[1]; r++ {
				expanded = append(expanded, [2]int{r, r + 1})
			}
			continue
		}
		expanded = append(expanded, u)
	}
	s := &tableSplit{}
	avail := firstAvail - over
	cur, start := 0.0, 0
	for _, u := range expanded {
		h := height(u)
		if cur > 0 && cur+h > avail+overflowEps {
			s.Rows = append(s.Rows, u[0]-start)
			s.SplitBefore = append(s.SplitBefore, u[0])
			start, cur, avail = u[0], 0, laterAvail-over
		}
		cur += h
	}
	s.Rows = append(s.Rows, len(body)-start)
	s.Tables = len(s.Rows)
	if s.Tables < 2 {
		return nil
	}
	return s
}
