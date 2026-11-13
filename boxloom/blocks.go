// 第1層のブロックのうち、自分で大きさを測って描く葉:表(table)・画像(image)・
// 画像の並び(gallery)。と、並列のまとまり(columns)の部品。
//
// 表と画像は Box の木の葉として置き、Subsystem の約束(Measure・Render)で
// レイアウトと話す(登録簿には載せない。IR ではサブシステムではなくブロック。
// docs/patterns.md の 2.1・2.2)。
//
//   - 表:列の幅は中身から測る(折り返さない幅と、単語を割らない最小の幅と、
//     見出しの文字列・和文の4字の塊を守る最小の幅 floor)。割り当てられた幅いっぱいに
//     広げ、余りは折り返さない幅の比で配る。足りなければ floor まで、floor の合計も
//     入らなければ従来どおり単語を割らない最小の幅まで縮める(table-col-width の警告。
//     docs/ir.md の15章)。行の高さはセルの文を折り返した高さ。縮めない
//     (Min = Natural = Max)。数だけの列は右に揃える(テンプレートの方針)。
//   - 画像:縦横比を保つ(FixedAspect)。幅いっぱいを自然な大きさとし、高さが
//     足りなければ imageMinHeight まで縮む(幅も比で縮み、余りは Place の側 =
//     左・上に残る)。切り抜かない・引き伸ばさない。説明の文は画像の下。
//   - 画像の並び:2〜3枚は1段、4枚は 2×2、5〜6枚は 3 列で2段。枡は同じ幅で、
//     画像は枡の左上に比を保って置く。

package boxloom

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// --- 文字の小道具 ---

// minRunsWidth は単語(トークン)を割らずに折り返したときの最小の幅(余裕込み)。
func minRunsWidth(runs []TextRun, size float64) float64 {
	rs, style := richRunes(runs)
	m := 0.0
	start := 0
	for end := 0; end <= len(rs); end++ {
		if end < len(rs) && rs[end] != '\n' {
			continue
		}
		pos := start
		for _, tok := range tokenizeLine(string(rs[start:end])) {
			n := len([]rune(tok))
			if strings.TrimSpace(tok) != "" {
				w := 0.0
				for k := pos; k < pos+n; k++ {
					w += charEmWidth(rs[k], runs[style[k]].Bold) * size
				}
				if w > m {
					m = w
				}
			}
			pos += n
		}
		start = end + 1
	}
	return m
}

// cjkChunkWidth は、和文(漢字・仮名・長音)の続いた字を tableCJKChunk 字ずつの
// 塊とみなしたときの、一番広い塊の幅(余裕込み)。続きが短ければその続き全体。
func cjkChunkWidth(runs []TextRun, size float64) float64 {
	rs, style := richRunes(runs)
	m := 0.0
	for i := range rs {
		if !isCJKLetter(rs[i]) {
			continue
		}
		w := 0.0
		for k := i; k < len(rs) && k < i+tableCJKChunk && isCJKLetter(rs[k]); k++ {
			w += charEmWidth(rs[k], runs[style[k]].Bold) * size
		}
		m = math.Max(m, w)
	}
	return m
}

// isCJKLetter は和文の語をなす字(漢字・平仮名・片仮名・長音・々)。約物は含めない
// (句読点・括弧の前後では折れてよい)。
func isCJKLetter(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) || r == 'ー' || r == '々'
}

// maxLineWidth は "\n" で分けた行のうち一番長い行の幅(折り返さない幅)。
func maxLineWidth(runs []TextRun, size float64) float64 {
	m := 0.0
	for _, line := range wrapRuns(runs, size, math.Inf(1)) {
		if w := naturalRunsWidth(line, size); w > m {
			m = w
		}
	}
	return m
}

func runsLines(runs []TextRun, size, width float64) int {
	if len(runs) == 0 {
		return 0
	}
	return len(wrapRuns(runs, size, width))
}

// textLiner は中の文字の行の数を数えられる葉(容量の lines に使う)。
type textLiner interface {
	textLines(width float64) int
}

// --- 表 ---

type tableLeaf struct {
	cells   [][]TableCell // 1行目は見出し
	size    float64
	padX    float64
	padY    float64
	draw    TableDraw // 見た目(塗り・線)。幅と高さは Render で入れる
	caption []TextRun
	mark    []TextRun // 続けた表の「n/m」の印(説明の文の行の右端。続きの無い表では nil)
	// markProbe は分け方の案(split)で、分けた後の印の幅を見込むための印。
	markProbe []TextRun
	capSize   float64
	capGap    float64
	natural   []float64 // 列ごとの折り返さない幅(余白込み)
	minimum   []float64 // 列ごとの最小の幅(余白込み。語を割らない幅)
	// floor は列ごとの守りたい最小の幅(余白込み):見出しの文字列の幅と、セルの中の
	// 折れない塊(欧文の語・和文は続いた字の tableCJKChunk 字分)の幅の大きい方。
	// 合計が表の幅に収まる限り、列はこれより狭くしない(nil なら minimum と同じ)。
	floor []float64
	// fixed は余りの幅を配らない列(縦書きの帯の列。幅は縦の1行のまま)。
	fixed []bool
}

// tableCJKChunk は表の列の最小の幅で、和文(漢字・仮名)の続いた字を割らない塊の字数。
// 「地域中核」のような4字の語までは1行に置ける幅を、列の最小にする。
const tableCJKChunk = 4

func (t *tableLeaf) Name() string { return "table" }

// floorSum は守りたい最小の幅(floor)の合計。
func (t *tableLeaf) floorSum() float64 {
	sum := 0.0
	for i := range t.natural {
		if t.floor != nil {
			sum += t.floor[i]
		} else {
			sum += t.minimum[i]
		}
	}
	return sum
}

// colWidths は幅 w に列の幅を配る。収まれば折り返さない幅の比で広げる。足りなければ
// 守りたい最小の幅(floor)を先に取り、残りを折り返さない幅と floor の差の比で配る。
// floor の合計も入らなければ、語を割らない最小の幅(minimum)で同じように配る
// (Lint が table-col-width の警告にする)。
func (t *tableLeaf) colWidths(w float64) []float64 {
	n := len(t.natural)
	out := make([]float64, n)
	sumN, sumM, sumF := 0.0, 0.0, t.floorSum()
	for i := range t.natural {
		sumN += t.natural[i]
		sumM += t.minimum[i]
	}
	switch {
	case w <= 0:
		copy(out, t.natural)
	case sumN <= w:
		// 余りは折り返さない幅の比で配る(幅を決めた列 fixed には配らない)。
		grow := 0.0
		for i := range out {
			if !t.isFixed(i) {
				grow += t.natural[i]
			}
		}
		for i := range out {
			out[i] = t.natural[i]
			if !t.isFixed(i) && grow > 0 {
				out[i] += (w - sumN) * t.natural[i] / grow
			}
		}
	case t.floor != nil && sumF <= w:
		span := sumN - sumF
		for i := range out {
			out[i] = t.floor[i] + (w-sumF)*(t.natural[i]-t.floor[i])/span
		}
	case sumM <= w:
		span := sumN - sumM
		for i := range out {
			out[i] = t.minimum[i] + (w-sumM)*(t.natural[i]-t.minimum[i])/span
		}
	default:
		for i := range out {
			out[i] = t.minimum[i] * w / sumM
		}
	}
	return out
}

func (t *tableLeaf) isFixed(c int) bool { return c < len(t.fixed) && t.fixed[c] }

// vertBandWidth は縦書きのセルの列の幅(縦の1行の幅 + 左右の余白)。
func vertBandWidth(size, padX float64) float64 { return size*lineHeightMult() + 2*padX }

// vertLength は縦書きのセルの文の縦の長さ(折り返さない1行。段落は縦の行を分けずに足す)。
func (c TableCell) vertLength(size float64) float64 {
	n := 0.0
	for _, runs := range c.paras() {
		n += naturalRunsWidth(runs, size)
	}
	return n
}

// paras はセルの段落(文のセルは1つ、箇条のセルは項目ごと)。
func (c TableCell) paras() [][]TextRun {
	if c.Items != nil {
		return c.Items
	}
	return [][]TextRun{c.Runs}
}

// lines はセルの文の行の数(箇条のセルは項目ごとに折り返した行の和)。
func (c TableCell) lines(size, width float64) int {
	n := 0
	for _, runs := range c.paras() {
		n += runsLines(runs, size, width)
	}
	return n
}

// rowHeights は行ごとの高さと、文の行の数の和。縦に結合したセル(行の群の名前)は
// 行ごとの高さに入れず、結合した行の高さの和に収まらなければ、足りない分の行を
// 群の最後の行に足す(群の境目は行の境目なので、行ごとに測ったまま)。
func (t *tableLeaf) rowHeights(widths []float64) ([]float64, int) {
	hs := make([]float64, len(t.cells))
	lineH := t.size * lineHeightMult()
	rowLines := make([]int, len(t.cells))
	for r, row := range t.cells {
		lines := 1
		for c, cell := range row {
			if cell.VMerge || cell.RowSpan > 1 || cell.Vert {
				continue
			}
			if n := cell.lines(t.size, widths[c]-2*t.padX); n > lines {
				lines = n
			}
		}
		rowLines[r] = lines
	}
	for r, row := range t.cells {
		for c, cell := range row {
			if cell.Vert && !cell.VMerge {
				// 縦書きのセル:文は縦の1行。結合した行の高さに収まらなければ、足りない分の
				// 行を群の最後の行に足す。
				span := cell.RowSpan
				if span < 1 {
					span = 1
				}
				if r+span > len(t.cells) {
					continue
				}
				have := -2 * t.padY
				for k := r; k < r+span; k++ {
					have += float64(rowLines[k])*lineH + 2*t.padY
				}
				if short := cell.vertLength(t.size) - have; short > 0 {
					rowLines[r+span-1] += int(math.Ceil(short/lineH - 1e-9))
				}
				continue
			}
			if cell.RowSpan < 2 || r+cell.RowSpan > len(t.cells) {
				continue
			}
			need := cell.lines(t.size, widths[c]-2*t.padX)
			// 結合した行が持つ文の行の数(行ごとの上下の余白も文の行に数える)。
			have := 0.0
			for k := r; k < r+cell.RowSpan; k++ {
				have += float64(rowLines[k]) + 2*t.padY/lineH
			}
			have -= 2 * t.padY / lineH
			if short := float64(need) - have; short > 0 {
				rowLines[r+cell.RowSpan-1] += int(math.Ceil(short - 1e-9))
			}
		}
	}
	total := 0
	for r, lines := range rowLines {
		total += lines
		hs[r] = float64(lines)*lineH + 2*t.padY
	}
	return hs, total
}

// labelWidths は説明の文の行の、説明の文の幅と印の幅(印が無ければ 0)。印は右端に
// 置き、説明の文は印の左で折り返す。
func (t *tableLeaf) labelWidths(w float64) (capW, markW float64) {
	if len(t.mark) == 0 {
		return w, 0
	}
	markW = naturalRunsWidth(t.mark, t.capSize)
	return max0(w - markW - t.capGap*2), markW
}

// captionLines は説明の文の行の行の数(印だけなら1行)。
func (t *tableLeaf) captionLines(w float64) int {
	capW, _ := t.labelWidths(w)
	n := runsLines(t.caption, t.capSize, capW)
	if len(t.mark) > 0 && n < 1 {
		n = 1
	}
	return n
}

func (t *tableLeaf) captionHeight(w float64) float64 {
	n := t.captionLines(w)
	if n == 0 {
		return 0
	}
	return float64(n)*t.capSize*lineHeightMult() + t.capGap
}

func (t *tableLeaf) sizeAt(w float64) Size {
	ws := t.colWidths(w)
	if w <= 0 {
		for _, v := range ws {
			w += v
		}
	}
	hs, _ := t.rowHeights(ws)
	h := t.captionHeight(w)
	for _, v := range hs {
		h += v
	}
	return Size{Width: w, Height: h}
}

func (t *tableLeaf) Measure(c Constraint) Measurement {
	s := t.sizeAt(c.Width)
	minW := 0.0
	for _, v := range t.minimum {
		minW += v
	}
	return Measurement{Natural: s, Min: Size{Width: minW, Height: s.Height}, Max: Size{Height: s.Height}}
}

// Lint は、守りたい最小の幅(見出しの文字列・折れない塊)の合計が表の幅に入らず、
// 列の幅を語を割らない最小の幅で配ったこと(table-col-width)を警告にする。
// 名指しするのは、floor より狭くなった最初の列。
func (t *tableLeaf) Lint(box LBox) []string {
	if t.floor == nil || box.Width <= 0 {
		return nil
	}
	sumN, sumF := 0.0, t.floorSum()
	for _, v := range t.natural {
		sumN += v
	}
	if sumN <= box.Width || sumF <= box.Width+0.01 {
		return nil
	}
	ws := t.colWidths(box.Width)
	col := 0
	for i := range ws {
		if ws[i] < t.floor[i]-0.01 {
			col = i
			break
		}
	}
	head := ""
	if len(t.cells) > 0 && col < len(t.cells[0]) {
		for _, r := range t.cells[0][col].Runs {
			head += r.Text
		}
	}
	return []string{fmt.Sprintf("[SUBSYSTEM WARNING] table: header[%d] (%q): the columns do not fit at their minimum widths (the headings and unbreakable words need %.1fpt, the table has %.1fpt); short columns may wrap inside words",
		col, head, sumF, box.Width)}
}

func (t *tableLeaf) textLines(width float64) int {
	_, n := t.rowHeights(t.colWidths(width))
	return n + t.captionLines(width)
}

func (t *tableLeaf) Render(ctx RenderContext, box LBox) {
	y := box.Y
	if ch := t.captionHeight(box.Width); ch > 0 {
		ch -= t.capGap
		capW, markW := t.labelWidths(box.Width)
		if len(t.caption) > 0 {
			ctx.Out.AddRichTextBox(LBox{X: box.X, Y: y, Width: capW, Height: ch}, t.caption, t.capSize, "left", "top")
		}
		if len(t.mark) > 0 {
			// 「n/m」の印は説明の文の1行目の右端(表の右端にそろえる)。
			lh := t.capSize * lineHeightMult()
			ctx.Out.AddRichTextBox(LBox{X: box.X + box.Width - markW, Y: y, Width: markW, Height: lh}, t.mark, t.capSize, "right", "top")
		}
		y += ch + t.capGap
	}
	d := t.draw
	d.ColWidths = t.colWidths(box.Width)
	d.RowHeights, _ = t.rowHeights(d.ColWidths)
	d.Cells, d.FontSize, d.PadX, d.PadY = t.cells, t.size, t.padX, t.padY
	ctx.Out.AddTable(LBox{X: box.X, Y: y, Width: box.Width, Height: box.Y + box.Height - y}, d)
}

// numericCellRe は「数だけ」のセル(右に揃える列を決める)。符号・通貨の印・桁の
// 区切り・小数・後ろの単位(% と短い単位の語)を許す。
var numericCellRe = regexp.MustCompile(`^[+\-−±]?[¥$€£]?[0-9][0-9,]*(\.[0-9]+)?[ ]?(%|pt|px|ms|s|h|件|台|人|円|万|億|倍|年|月|日|時間|分|秒|個|回|社|名|点|割)?$`)

// numericColumns は列ごとに、本体のセルがすべて数(空のセルは数えない)か。
func numericColumns(rows [][]IRTableCell, n int) []bool {
	out := make([]bool, n)
	for c := 0; c < n; c++ {
		seen := false
		ok := true
		for _, r := range rows {
			s := strings.TrimSpace(r[c].Plain())
			if s == "" {
				continue
			}
			seen = true
			if !numericCellRe.MatchString(s) {
				ok = false
				break
			}
		}
		out[c] = seen && ok
	}
	return out
}

// tableLook は表の見た目(役割名と値の表の名前)。標準の部品は standardTableLook、
// テンプレートは自分の組み方で別の見た目を渡せる(report.go)。色は役割名で書き、
// "" は塗らない・引かない。
type tableLook struct {
	headerFill, bodyFill string // 見出しの行・本体の行の面
	rule                 string // 行の間の線
	headerText           string // 見出しの行の字
	// headerRule は見出しの行の下の線("" なら行の間の線と同じ)。太さは値の表の
	// tableHeaderRuleWidth。
	headerRule string
	// groupFill・groupText は行の群の名前のセルの面と字("" の面は行の面のまま)。
	groupFill, groupText string
	// groupVertical は行の群の名前を縦書きの細い帯にする(列の幅は縦の1行。見出しの
	// 行のその列の文は単語を割らない最小の幅で折り返す)。
	groupVertical bool
}

// standardTableLook は標準の表:見出しの行は薄い面、本体はカードの面、行の間は罫線。
var standardTableLook = tableLook{headerFill: "card_alt", bodyFill: "card", rule: "grid",
	headerText: "text_primary", groupText: "text_primary"}

// table は表の葉を作る。続きの表(continues)は、最初の表の見出しの行を繰り返し、
// 列の幅と数の列をひと続きの全部の行で決め、「n/m」の印を付ける(tablechain.go)。
func (t *Template) table(b *IRBlock) *Spec { return t.tableStyled(b, standardTableLook) }

// tableStyled は見た目 look で表の葉を作る。
func (t *Template) tableStyled(b *IRBlock, look tableLook) *Spec {
	v, th := t.Values, t.Theme
	header, grouped := b.tableHeader(), b.tableGrouped()
	members := []*IRBlock{b}
	if b.chain != nil {
		members = b.chain.Tables
	}
	// 数の列を決める本体のセル(群の列は空のセルとして数えない)。
	var body [][]IRTableCell
	for _, m := range members {
		body = append(body, tableBodyCells(m, grouped)...)
	}
	numeric := numericColumns(body, len(header))
	if grouped {
		// 群の列は数の列として右に揃えない(群の名前の列)。
		numeric[0] = false
	}
	leaf := &tableLeaf{size: v.TableText, padX: v.TableCellPadX, padY: v.TableCellPadY,
		capSize: v.CaptionText, capGap: v.CaptionGap,
		draw: TableDraw{HeaderFill: roleColor(th, look.headerFill), BodyFill: roleColor(th, look.bodyFill),
			RuleColor: roleColor(th, look.rule), RuleWidth: v.TableRuleWidth}}
	if look.headerRule != "" {
		leaf.draw.HeaderRuleColor, leaf.draw.HeaderRuleWidth = th.c(look.headerRule), v.TableHeaderRuleWidth
	}
	leaf.cells = t.tableCells(b, header, numeric, look)
	if b.Total != nil {
		// 合計の行は本体の行と見分ける: 太字にし、上に濃い線を引く(TableDraw.TotalRow)。
		leaf.draw.TotalRow = true
		leaf.draw.TotalRuleColor = th.c("text_primary")
	}
	// 列の幅はひと続きの全部の表の行(と見出しの行)で測る(どの枚でも同じ幅になる)。
	leaf.natural = make([]float64, len(header))
	leaf.minimum = make([]float64, len(header))
	leaf.floor = make([]float64, len(header))
	band := grouped && look.groupVertical
	if band {
		leaf.fixed = make([]bool, len(header))
		leaf.fixed[0] = true
		w := vertBandWidth(leaf.size, leaf.padX)
		leaf.natural[0], leaf.minimum[0], leaf.floor[0] = w, w, w
	}
	for _, m := range members {
		cells := leaf.cells
		if m != b {
			cells = t.tableCells(m, header, numeric, look)
		}
		for r, row := range cells {
			for c, cell := range row {
				if band && c == 0 {
					// 帯の列:縦書きのセルは幅を決めず(縦の1行)、横書きのセル(見出しの行・
					// 合計の行)の文は最小の幅だけを求める(折り返して帯の幅に合わせる)。
					if !cell.Vert {
						for _, runs := range cell.paras() {
							w := minRunsWidth(runs, leaf.size) + 2*leaf.padX
							leaf.natural[0] = math.Max(leaf.natural[0], w)
							leaf.minimum[0] = math.Max(leaf.minimum[0], w)
							leaf.floor[0] = math.Max(leaf.floor[0], w)
						}
					}
					continue
				}
				for _, runs := range cell.paras() {
					natural := maxLineWidth(runs, leaf.size) + 2*leaf.padX
					minimum := minRunsWidth(runs, leaf.size) + 2*leaf.padX
					floor := math.Max(minimum, cjkChunkWidth(runs, leaf.size)+2*leaf.padX)
					if r == 0 {
						// 見出しの行は文字列を折らない幅を守る(1行目は見出し)。
						floor = natural
					}
					leaf.natural[c] = math.Max(leaf.natural[c], natural)
					leaf.minimum[c] = math.Max(leaf.minimum[c], minimum)
					leaf.floor[c] = math.Max(leaf.floor[c], floor)
				}
			}
		}
	}
	caption := b.Caption
	if caption == nil && b.chain != nil {
		// 続きの表に説明の文が無ければ、最初の表の説明の文を繰り返す。
		caption = b.chain.first().Caption
	}
	if caption != nil {
		leaf.caption = t.leafRuns(caption, "text_muted", false)
	}
	leaf.markProbe = t.leafRuns(Inline{{Text: t.Marks.continuedMark(99, 99)}}, "text_muted", false)
	if b.chain != nil {
		leaf.mark = t.leafRuns(Inline{{Text: t.Marks.continuedMark(b.chainPos+1, len(b.chain.Tables))}}, "text_muted", false)
	}
	return SubsystemNode(leaf)
}

// roleColor は役割名の色("" なら "")。
func roleColor(th Theme, role string) string {
	if role == "" {
		return ""
	}
	return th.c(role)
}

// tableBodyCells は本体と合計の行のセル(群の表では先頭に群の列の空のセルを足す)。
func tableBodyCells(b *IRBlock, grouped bool) [][]IRTableCell {
	var body [][]IRTableCell
	for _, r := range b.Rows {
		if grouped {
			r = append([]IRTableCell{{}}, r...)
		}
		body = append(body, r)
	}
	if b.Total != nil {
		body = append(body, b.Total)
	}
	return body
}

// tableCells は表の見出しの行・本体の行・合計の行のセル。
func (t *Template) tableCells(b *IRBlock, header []Inline, numeric []bool, look tableLook) [][]TableCell {
	th := t.Theme
	grouped := b.Groups != nil
	cell := func(c int, in IRTableCell, bold bool, role string) TableCell {
		align := "left"
		if numeric[c] {
			align = "right"
		}
		if in.Points == nil {
			return TableCell{Runs: t.leafRuns(in.Text, role, bold), Align: align}
		}
		// セルの中の箇条: 項目ごとに1つの段落。印は本文の箇条と同じ marks.point
		// (先頭の塊の Mark。PPTX では a:buChar)。
		out := TableCell{Align: "left"}
		for _, it := range in.Points {
			mark := TextRun{Text: t.Marks.Point, Color: th.c("accent"), Bold: true, Mark: true}
			out.Items = append(out.Items, append([]TextRun{mark}, t.leafRuns(it, role, bold)...))
		}
		return out
	}
	cellRow := func(cells []IRTableCell, bold bool) []TableCell {
		var out []TableCell
		for c, in := range cells {
			out = append(out, cell(c, in, bold, "text_primary"))
		}
		return out
	}
	var rows [][]TableCell
	var hdr []TableCell
	for c, in := range header {
		hdr = append(hdr, cell(c, IRTableCell{Text: in}, true, look.headerText))
	}
	rows = append(rows, hdr)
	if grouped {
		// 群の名前は先頭の列の縦に結合したセル(a:tbl の rowSpan と vMerge)。
		// 名前は見出しと同じく太字にして、行の中身と見分ける。面は look.groupFill
		// (縦に結合した続きのセルも同じ面にする。アプリによって続きのセルの面が見えるため)。
		body := tableBodyCells(b, true)
		groupFill := roleColor(th, look.groupFill)
		ri := 0
		for _, g := range b.Groups {
			for k := 0; k < g.Rows; k++ {
				var lead TableCell
				if k == 0 {
					lead = TableCell{Runs: t.leafRuns(g.Label, look.groupText, true), Align: "left", RowSpan: g.Rows, Fill: groupFill, Vert: look.groupVertical}
				} else {
					lead = TableCell{Align: "left", VMerge: true, Fill: groupFill, Vert: look.groupVertical}
				}
				rest := cellRow(body[ri], false)[1:]
				rows = append(rows, append([]TableCell{lead}, rest...))
				ri++
			}
		}
	} else {
		for _, r := range b.Rows {
			rows = append(rows, cellRow(r, false))
		}
	}
	if b.Total != nil {
		rows = append(rows, cellRow(b.Total, true))
	}
	return rows
}

// --- 画像 ---

type imageLeaf struct {
	img     *imageAsset
	alt     string
	caption []TextRun
	capSize float64
	capGap  float64
	minH    float64
	// capAbove は説明の文を画像の上に置く(既定は下。高さの測り方は変わらない)。
	capAbove bool
}

func (m *imageLeaf) Name() string { return "image" }

func (m *imageLeaf) captionHeight(w float64) float64 {
	if len(m.caption) == 0 {
		return 0
	}
	return float64(runsLines(m.caption, m.capSize, w))*m.capSize*lineHeightMult() + m.capGap
}

// widthForHeight は高さ h に収まるときの幅(説明の文の高さは幅で変わるので数回詰める)。
func (m *imageLeaf) widthForHeight(h float64) float64 {
	a := m.img.aspect()
	w := h * a
	for i := 0; i < 4; i++ {
		w = math.Max(0, h-m.captionHeight(w)) * a
	}
	return w
}

func (m *imageLeaf) Measure(c Constraint) Measurement {
	a := m.img.aspect()
	var nat Size
	switch {
	case c.Width > 0:
		nat = Size{Width: c.Width, Height: c.Width/a + m.captionHeight(c.Width)}
	case c.Height > 0:
		nat = Size{Width: m.widthForHeight(c.Height), Height: c.Height}
	default:
		w := float64(m.img.W) * 0.75 // 96 dpi の1画素 = 0.75pt
		nat = Size{Width: w, Height: w/a + m.captionHeight(w)}
	}
	ih := nat.Height - m.captionHeight(nat.Width)
	minIH := math.Min(ih, m.minH)
	minW := minIH * a
	min := Size{Width: minW, Height: minIH + m.captionHeight(minW)}
	return Measurement{Natural: nat, Min: min, Max: nat, FixedAspect: true}
}

func (m *imageLeaf) textLines(width float64) int {
	return runsLines(m.caption, m.capSize, width)
}

func (m *imageLeaf) Render(ctx RenderContext, box LBox) {
	ih := math.Max(0, box.Height-m.captionHeight(box.Width))
	iw := ih * m.img.aspect()
	if iw > box.Width {
		iw, ih = box.Width, box.Width/m.img.aspect()
	}
	if len(m.caption) == 0 {
		ctx.Out.AddPicture(LBox{X: box.X, Y: box.Y, Width: iw, Height: ih}, m.img, m.alt)
		return
	}
	ch := m.captionHeight(box.Width) - m.capGap
	if m.capAbove {
		ctx.Out.AddRichTextBox(LBox{X: box.X, Y: box.Y, Width: box.Width, Height: ch}, m.caption, m.capSize, "left", "top")
		ctx.Out.AddPicture(LBox{X: box.X, Y: box.Y + ch + m.capGap, Width: iw, Height: ih}, m.img, m.alt)
		return
	}
	ctx.Out.AddPicture(LBox{X: box.X, Y: box.Y, Width: iw, Height: ih}, m.img, m.alt)
	ctx.Out.AddRichTextBox(LBox{X: box.X, Y: box.Y + ih + m.capGap, Width: box.Width, Height: ch}, m.caption, m.capSize, "left", "top")
}

// imageLook は画像・画像の並びの説明の文の見た目。標準は standardImageLook(下・薄い字)、
// テンプレートは自分の組み方で別の見た目を渡せる(report.go)。
type imageLook struct {
	captionRole  string // 説明の文の字の色の役割名
	captionAbove bool   // 説明の文を画像の上に置く
}

var standardImageLook = imageLook{captionRole: "text_muted"}

func (t *Template) imageLeaf(src, alt string, caption Inline) (*imageLeaf, error) {
	return t.imageLeafStyled(src, alt, caption, standardImageLook)
}

// imageLeafStyled は見た目 look で画像の葉を作る。
func (t *Template) imageLeafStyled(src, alt string, caption Inline, look imageLook) (*imageLeaf, error) {
	img, err := t.images.get(src)
	if err != nil {
		return nil, err
	}
	v := t.Values
	m := &imageLeaf{img: img, alt: alt, capSize: v.CaptionText, capGap: v.CaptionGap, minH: v.ImageMinHeight, capAbove: look.captionAbove}
	if caption != nil {
		m.caption = t.leafRuns(caption, look.captionRole, false)
	}
	return m, nil
}

// --- 画像の並び ---

type galleryLeaf struct {
	items []*imageLeaf
	gap   float64
}

func (g *galleryLeaf) Name() string { return "gallery" }

// galleryColumns は枚数から列の数を決める(2・3 → 1段、4 → 2×2、5・6 → 3列)。
func galleryColumns(n int) int {
	switch n {
	case 2, 4:
		return 2
	}
	return 3
}

// grid は幅 w での列と段の数。枚数で決めた列の数(galleryColumns)で枡の幅が画像の
// 下限(imageMinHeight)に満たなければ、満たすまで列を減らす(並列のまとまりの狭い
// 列の中、#117)。w が 0 なら枚数で決めたまま。本文の幅では枡が十分広いので変わらない。
func (g *galleryLeaf) grid(w float64) (cols, rows int) {
	cols = galleryColumns(len(g.items))
	for w > 0 && cols > 1 && g.cellWidthOf(w, cols) < g.items[0].minH {
		cols--
	}
	return cols, (len(g.items) + cols - 1) / cols
}

func (g *galleryLeaf) cellWidthOf(w float64, cols int) float64 {
	return math.Max(0, (w-g.gap*float64(cols-1))/float64(cols))
}

func (g *galleryLeaf) cellWidth(w float64) float64 {
	cols, _ := g.grid(w)
	return g.cellWidthOf(w, cols)
}

// rowsOf は段ごとの画像の高さ(枡の幅いっぱいにしたときの一番高いもの)と、
// 説明の文の高さ(一番高いもの)。
func (g *galleryLeaf) rowsOf(w float64) (img, capH []float64) {
	cols, rows := g.grid(w)
	cw := g.cellWidthOf(w, cols)
	img, capH = make([]float64, rows), make([]float64, rows)
	for i, it := range g.items {
		r := i / cols
		img[r] = math.Max(img[r], cw/it.img.aspect())
		capH[r] = math.Max(capH[r], it.captionHeight(cw))
	}
	return img, capH
}

func (g *galleryLeaf) Measure(c Constraint) Measurement {
	w := c.Width
	if w <= 0 {
		cols, _ := g.grid(0)
		for i := 0; i < cols; i++ {
			w += float64(g.items[i].img.W) * 0.75
		}
		w += g.gap * float64(cols-1)
	}
	img, capH := g.rowsOf(w)
	nat, min := 0.0, 0.0
	for r := range img {
		nat += img[r] + capH[r]
		min += math.Min(img[r], g.items[0].minH) + capH[r]
	}
	gaps := g.gap * float64(len(img)-1)
	return Measurement{
		Natural: Size{Width: w, Height: nat + gaps},
		Min:     Size{Width: w, Height: min + gaps},
		Max:     Size{Height: nat + gaps},
	}
}

func (g *galleryLeaf) textLines(width float64) int {
	cols, _ := g.grid(width)
	cw := g.cellWidth(width)
	n := 0
	for r := 0; r*cols < len(g.items); r++ {
		m := 0
		for i := r * cols; i < len(g.items) && i < (r+1)*cols; i++ {
			if k := g.items[i].textLines(cw); k > m {
				m = k
			}
		}
		n += m
	}
	return n
}

func (g *galleryLeaf) Render(ctx RenderContext, box LBox) {
	cols, _ := g.grid(box.Width)
	cw := g.cellWidth(box.Width)
	img, capH := g.rowsOf(box.Width)
	want, fixed := 0.0, g.gap*float64(len(img)-1)
	for r := range img {
		want += img[r]
		fixed += capH[r]
	}
	s := 1.0
	if want > 0 && box.Height-fixed < want {
		s = math.Max(0, box.Height-fixed) / want
	}
	y := box.Y
	for r := range img {
		rh := img[r] * s
		for i := r * cols; i < len(g.items) && i < (r+1)*cols; i++ {
			it := g.items[i]
			x := box.X + float64(i-r*cols)*(cw+g.gap)
			iw, ih := cw, cw/it.img.aspect()
			if ih > rh {
				ih, iw = rh, rh*it.img.aspect()
			}
			if it.capAbove {
				// 説明の文を段の上にそろえて置き、画像は段の説明の文の高さの下から。
				if len(it.caption) > 0 {
					ch := it.captionHeight(cw) - it.capGap
					ctx.Out.AddRichTextBox(LBox{X: x, Y: y, Width: cw, Height: ch}, it.caption, it.capSize, "left", "top")
				}
				ctx.Out.AddPicture(LBox{X: x, Y: y + capH[r], Width: iw, Height: ih}, it.img, it.alt)
				continue
			}
			ctx.Out.AddPicture(LBox{X: x, Y: y, Width: iw, Height: ih}, it.img, it.alt)
			if len(it.caption) > 0 {
				ch := it.captionHeight(cw) - it.capGap
				ctx.Out.AddRichTextBox(LBox{X: x, Y: y + rh + it.capGap, Width: cw, Height: ch}, it.caption, it.capSize, "left", "top")
			}
		}
		y += rh + capH[r] + g.gap
	}
}

func (t *Template) gallery(b *IRBlock) (*Spec, error) { return t.galleryStyled(b, standardImageLook) }

// galleryStyled は見た目 look で画像の並びを作る。
func (t *Template) galleryStyled(b *IRBlock, look imageLook) (*Spec, error) {
	g := &galleryLeaf{gap: t.Values.ItemGap}
	for i, im := range b.Images {
		leaf, err := t.imageLeafStyled(im.Src, im.Alt, im.Caption, look)
		if err != nil {
			return nil, fmt.Errorf("images[%d]: src: %w", i, err)
		}
		g.items = append(g.items, leaf)
	}
	return SubsystemNode(g, WithGrow(1)), nil
}

// --- 並列のまとまり ---

// columnsInfo は columns の行(Row)が持つ、まとまりごとの部品(はみ出しを
// まとまりごとに測るため)。
type columnsInfo struct {
	cols []columnInfo
}

type columnInfo struct {
	spec  *Spec // まとまり全体(題 + 本文)
	title *Spec // 題(無ければ nil)
	body  *Spec // 本文のブロックの列(Col)
}

// columns は並列のまとまりを横に並べる。幅の比は意味(role: main)から決める:
// 主のまとまりは columnMainWeight、ほかは 1。題は本文の上に置き、字の色は text_primary。
// flow(左が右につながる)のときだけ、まとまりの間の隙間の縦の真ん中に小さい矢印
// (基調色)を置く。flow の無い columns の組み方はこれまでと同じ。
func (t *Template) columns(ctx componentContext, b *IRBlock) (*Spec, error) {
	return t.columnsHeaded(ctx, b, "text_primary")
}

// columnsHeaded は columns の題の色の役割名を headRole にしたもの(flow の印は標準の
// 矢印)。
func (t *Template) columnsHeaded(ctx componentContext, b *IRBlock, headRole string) (*Spec, error) {
	style := columnsStyle{headRole: headRole}
	if b.Flow {
		style.gapMark = func() Subsystem { return flowArrowMark{} }
	}
	return t.columnsStyled(ctx, b, style)
}

// columnsStyle は columns の組み方のうち、テンプレートが部品を差し替えるときに
// 選べるところ(report.go)。
type columnsStyle struct {
	// headRole はまとまりの題の字の色の役割名。
	headRole string
	// headRule が nil でなければ、題の下に置く葉(罫線など)を作る。題と葉の
	// 間は itemGap。題の無いまとまりには置かない。
	headRule func() Subsystem
	// gapMark が nil でなければ、まとまりの間の隙間(幅 columnGap)に置く葉を
	// 作る(縦は行の高さいっぱい)。nil なら隙間は行の間隔のまま(何も置かない)。
	gapMark func() Subsystem
}

// columnsStyled は columns を style の見た目で組む。
func (t *Template) columnsStyled(ctx componentContext, b *IRBlock, style columnsStyle) (*Spec, error) {
	v := t.Values
	info := &columnsInfo{}
	var cols []*Spec
	for ci := range b.Columns {
		c := &b.Columns[ci]
		var blocks []*Spec
		for bi := range c.Body {
			cctx := ctx.within(fmt.Sprintf("columns[%d]: body[%d]", ci, bi))
			cctx.inColumn = true
			s, err := t.buildBlockIn(cctx, &c.Body[bi])
			if err != nil {
				return nil, fmt.Errorf("columns[%d]: body[%d]: %w", ci, bi, err)
			}
			blocks = append(blocks, s)
		}
		body := Col(blocks, tokGap("blockGap"), WithGrow(1), nestedSlack())
		col := columnInfo{body: body}
		kids := []*Spec{body}
		if c.Title != nil {
			col.title = t.text(c.Title, "columnTitle", style.headRole, true)
			if style.headRule != nil {
				col.title = Col([]*Spec{col.title, SubsystemNode(style.headRule())}, tokGap("itemGap"))
			}
			kids = []*Spec{col.title, body}
		}
		weight := 1.0
		if c.Role == "main" {
			weight = v.ColumnMainWeight
		}
		col.spec = Col(kids, tokGap("itemGap"), WithGrow(weight), WithBasis(0))
		info.cols = append(info.cols, col)
		cols = append(cols, col.spec)
	}
	if style.gapMark == nil {
		return Row(cols, tokGap("columnGap"), measureColumnGroups(info), nestedSlack()), nil
	}
	// 隙間を葉にする(幅は columnGap のまま。まとまりの幅は変わらない)。
	var kids []*Spec
	for i, c := range cols {
		if i > 0 {
			kids = append(kids, SubsystemNode(style.gapMark(), tokWidth("columnGap")))
		}
		kids = append(kids, c)
	}
	return Row(kids, measureColumnGroups(info), nestedSlack()), nil
}

// flowArrowMark は標準の columns の flow の印:隙間の真ん中に置く小さい右向きの
// 矢印(基調色)。大きさは隙間の幅の半分まで(12pt を上限)。
type flowArrowMark struct{}

func (flowArrowMark) Name() string                   { return "columnsFlow" }
func (flowArrowMark) Measure(Constraint) Measurement { return Measurement{} }
func (flowArrowMark) Render(ctx RenderContext, box LBox) {
	w := math.Min(12, box.Width/2)
	h := w * 0.8
	if w <= 0 || box.Height < h {
		return
	}
	ctx.Out.AddShape(LBox{X: box.X + (box.Width-w)/2, Y: box.Y + (box.Height-h)/2, Width: w, Height: h},
		ShapeOpts{Prst: "rightArrow", Fill: ctx.Roles.Color("accent")})
}
