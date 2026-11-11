// 第1層のブロックのうち、自分で大きさを測って描く葉:表(table)・画像(image)・
// 画像の並び(gallery)。と、並列のまとまり(columns)の部品。
//
// 表と画像は Box の木の葉として置き、Subsystem の約束(Measure・Render)で
// レイアウトと話す(登録簿には載せない。IR ではサブシステムではなくブロック。
// docs/patterns.md の 2.1・2.2)。
//
//   - 表:列の幅は中身から測る(折り返さない幅と、単語を割らない最小の幅)。
//     割り当てられた幅いっぱいに広げ、余りは折り返さない幅の比で配る。足りなければ
//     最小の幅まで縮める。行の高さはセルの文を折り返した高さ。縮めない
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
				if w *= measureSafetyMargin(); w > m {
					m = w
				}
			}
			pos += n
		}
		start = end + 1
	}
	return m
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
	capSize float64
	capGap  float64
	natural []float64 // 列ごとの折り返さない幅(余白込み)
	minimum []float64 // 列ごとの最小の幅(余白込み)
}

func (t *tableLeaf) Name() string { return "table" }

// colWidths は幅 w に列の幅を配る。
func (t *tableLeaf) colWidths(w float64) []float64 {
	n := len(t.natural)
	out := make([]float64, n)
	sumN, sumM := 0.0, 0.0
	for i := range t.natural {
		sumN += t.natural[i]
		sumM += t.minimum[i]
	}
	switch {
	case w <= 0:
		copy(out, t.natural)
	case sumN <= w:
		for i := range out {
			out[i] = t.natural[i] + (w-sumN)*t.natural[i]/sumN
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

func (t *tableLeaf) rowHeights(widths []float64) ([]float64, int) {
	hs := make([]float64, len(t.cells))
	total := 0
	for r, row := range t.cells {
		lines := 1
		for c, cell := range row {
			if n := runsLines(cell.Runs, t.size, widths[c]-2*t.padX); n > lines {
				lines = n
			}
		}
		total += lines
		hs[r] = float64(lines)*t.size*lineHeightMult + 2*t.padY
	}
	return hs, total
}

func (t *tableLeaf) captionHeight(w float64) float64 {
	if len(t.caption) == 0 {
		return 0
	}
	return float64(runsLines(t.caption, t.capSize, w))*t.capSize*lineHeightMult + t.capGap
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

func (t *tableLeaf) textLines(width float64) int {
	_, n := t.rowHeights(t.colWidths(width))
	return n + runsLines(t.caption, t.capSize, width)
}

func (t *tableLeaf) Render(ctx RenderContext, box LBox) {
	y := box.Y
	if len(t.caption) > 0 {
		ch := t.captionHeight(box.Width) - t.capGap
		ctx.Out.AddRichTextBox(LBox{X: box.X, Y: y, Width: box.Width, Height: ch}, t.caption, t.capSize, "left", "top")
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
func numericColumns(rows [][]Inline, n int) []bool {
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

// table は表の葉を作る。
func (t *Template) table(b *IRBlock) *Spec {
	v, th := t.Values, t.Theme
	body := b.Rows
	if b.Total != nil {
		body = append(append([][]Inline{}, b.Rows...), b.Total)
	}
	numeric := numericColumns(body, len(b.Header))
	leaf := &tableLeaf{size: v.TableText, padX: v.TableCellPadX, padY: v.TableCellPadY,
		capSize: v.CaptionText, capGap: v.CaptionGap,
		draw: TableDraw{HeaderFill: th.c("card_alt"), BodyFill: th.c("card"), RuleColor: th.c("grid"), RuleWidth: v.TableRuleWidth}}
	cellRow := func(cells []Inline, header bool) []TableCell {
		var out []TableCell
		for c, in := range cells {
			align := "left"
			if numeric[c] {
				align = "right"
			}
			out = append(out, TableCell{Runs: t.runs(in, th.c("text_primary"), header), Align: align})
		}
		return out
	}
	leaf.cells = append(leaf.cells, cellRow(b.Header, true))
	for _, r := range b.Rows {
		leaf.cells = append(leaf.cells, cellRow(r, false))
	}
	if b.Total != nil {
		// 合計の行は本体の行と見分ける: 太字にし、上に濃い線を引く(TableDraw.TotalRow)。
		leaf.cells = append(leaf.cells, cellRow(b.Total, true))
		leaf.draw.TotalRow = true
		leaf.draw.TotalRuleColor = th.c("text_primary")
	}
	leaf.natural = make([]float64, len(b.Header))
	leaf.minimum = make([]float64, len(b.Header))
	for _, row := range leaf.cells {
		for c, cell := range row {
			leaf.natural[c] = math.Max(leaf.natural[c], maxLineWidth(cell.Runs, leaf.size)+2*leaf.padX)
			leaf.minimum[c] = math.Max(leaf.minimum[c], minRunsWidth(cell.Runs, leaf.size)+2*leaf.padX)
		}
	}
	if b.Caption != nil {
		leaf.caption = t.runs(b.Caption, th.c("text_muted"), false)
	}
	return SubsystemNode(leaf)
}

// --- 画像 ---

type imageLeaf struct {
	img     *imageAsset
	alt     string
	caption []TextRun
	capSize float64
	capGap  float64
	minH    float64
}

func (m *imageLeaf) Name() string { return "image" }

func (m *imageLeaf) captionHeight(w float64) float64 {
	if len(m.caption) == 0 {
		return 0
	}
	return float64(runsLines(m.caption, m.capSize, w))*m.capSize*lineHeightMult + m.capGap
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
	ctx.Out.AddPicture(LBox{X: box.X, Y: box.Y, Width: iw, Height: ih}, m.img, m.alt)
	if len(m.caption) > 0 {
		ch := m.captionHeight(box.Width) - m.capGap
		ctx.Out.AddRichTextBox(LBox{X: box.X, Y: box.Y + ih + m.capGap, Width: box.Width, Height: ch}, m.caption, m.capSize, "left", "top")
	}
}

func (t *Template) imageLeaf(src, alt string, caption Inline) (*imageLeaf, error) {
	img, err := t.images.get(src)
	if err != nil {
		return nil, err
	}
	v := t.Values
	m := &imageLeaf{img: img, alt: alt, capSize: v.CaptionText, capGap: v.CaptionGap, minH: v.ImageMinHeight}
	if caption != nil {
		m.caption = t.runs(caption, t.Theme.c("text_muted"), false)
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

func (g *galleryLeaf) grid() (cols, rows int) {
	cols = galleryColumns(len(g.items))
	return cols, (len(g.items) + cols - 1) / cols
}

func (g *galleryLeaf) cellWidth(w float64) float64 {
	cols, _ := g.grid()
	return math.Max(0, (w-g.gap*float64(cols-1))/float64(cols))
}

// rowsOf は段ごとの画像の高さ(枡の幅いっぱいにしたときの一番高いもの)と、
// 説明の文の高さ(一番高いもの)。
func (g *galleryLeaf) rowsOf(cw float64) (img, capH []float64) {
	cols, rows := g.grid()
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
		cols, _ := g.grid()
		for i := 0; i < cols; i++ {
			w += float64(g.items[i].img.W) * 0.75
		}
		w += g.gap * float64(cols-1)
	}
	img, capH := g.rowsOf(g.cellWidth(w))
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
	cols, _ := g.grid()
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
	cols, _ := g.grid()
	cw := g.cellWidth(box.Width)
	img, capH := g.rowsOf(cw)
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
			ctx.Out.AddPicture(LBox{X: x, Y: y, Width: iw, Height: ih}, it.img, it.alt)
			if len(it.caption) > 0 {
				ch := it.captionHeight(cw) - it.capGap
				ctx.Out.AddRichTextBox(LBox{X: x, Y: y + rh + it.capGap, Width: cw, Height: ch}, it.caption, it.capSize, "left", "top")
			}
		}
		y += rh + capH[r] + g.gap
	}
}

func (t *Template) gallery(b *IRBlock) (*Spec, error) {
	g := &galleryLeaf{gap: t.Values.ItemGap}
	for i, im := range b.Images {
		leaf, err := t.imageLeaf(im.Src, im.Alt, im.Caption)
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
// 主のまとまりは columnMainWeight、ほかは 1。題は本文の上に置く。
func (t *Template) columns(b *IRBlock) (*Spec, error) {
	v, th := t.Values, t.Theme
	info := &columnsInfo{}
	var cols []*Spec
	for ci := range b.Columns {
		c := &b.Columns[ci]
		var blocks []*Spec
		for bi := range c.Body {
			s, err := t.buildBlock(&c.Body[bi])
			if err != nil {
				return nil, fmt.Errorf("columns[%d]: body[%d]: %w", ci, bi, err)
			}
			blocks = append(blocks, s)
		}
		body := Col(blocks, WithGap(v.BlockGap), WithGrow(1))
		body.nestedSlack = true
		col := columnInfo{body: body}
		kids := []*Spec{body}
		if c.Title != nil {
			col.title = t.text(c.Title, v.ColumnTitle, th.c("text_primary"), true)
			kids = []*Spec{col.title, body}
		}
		weight := 1.0
		if c.Role == "main" {
			weight = v.ColumnMainWeight
		}
		col.spec = Col(kids, WithGap(v.ItemGap), WithGrow(weight), WithBasis(0))
		info.cols = append(info.cols, col)
		cols = append(cols, col.spec)
	}
	row := Row(cols, WithGap(v.ColumnGap))
	row.columns = info
	row.nestedSlack = true
	return row, nil
}
