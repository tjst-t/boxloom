// matrix サブシステム: 行×列の評価(docs/patterns.md の R6)。
//
// セルが「評価」(状態の役割名)を持つ表。セルの文字・数だけなら第1層の
// table(パターンの2.1)で、こちらは使わない。列の見出しは列の真ん中に、行の
// 見出しは左に揃えて縦の真ん中に置く。セルは塗りの箱に、状態の色の丸と文を
// 置く(値の表の cellStyle が mark なら塗らず、丸とチェックの下に文)。行の題の
// 下に説明(detail。detailColumn があれば題の右の見出しつきの列)、列の見出しの上に列の群(columnGroups)を置ける。下に、使った
// 状態の凡例を自動で付ける(文は data の legend、無ければ既定の文)。設計は docs/subsystem.md の11節。IR の data の形は
// schema/subsystems/matrix.schema.json。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// MatrixValues は matrix の寸法(pt)。既定は subsystems/matrix.json。
type MatrixValues struct {
	Padding        float64 `json:"padding" scale:"space"`
	CellGap        float64 `json:"cellGap" scale:"space"` // セルとセルの間
	HeaderH        float64 `json:"headerHeight"`          // 列の見出しの行の高さ(文がこれより高ければ文に合わせる)
	MinRowH        float64 `json:"minRowHeight"`
	NaturalRowH    float64 `json:"naturalRowHeight"`
	MaxRowH        float64 `json:"maxRowHeight"`
	RowHeaderFrac  float64 `json:"maxRowHeaderFraction"` // 行の見出しの列の幅の上限(内側の幅に対する割合)
	DotSize        float64 `json:"dotSize"`              // 状態の丸の直径
	DotGap         float64 `json:"dotGap" scale:"space"` // 丸と文の間
	LegendGap      float64 `json:"legendGap" scale:"space"`
	LegendItemGap  float64 `json:"legendItemGap" scale:"space"`
	LegendDotSize  float64 `json:"legendDotSize"`
	NaturalWidth   float64 `json:"naturalWidth"`
	MinWidth       float64 `json:"minWidth"`
	TextInsetX     float64 `json:"textInsetX" scale:"space"`
	TextInsetY     float64 `json:"textInsetY" scale:"space"`
	CellCornerRadi float64 `json:"cellCornerRadius"`
	EmphasisLineW  float64 `json:"emphasisLineWidth"`     // 推す列の枠線の太さ
	GroupRuleW     float64 `json:"groupRuleWidth"`        // 列の群の名前の下の罫線の太さ
	MarkSize       float64 `json:"markSize"`              // cellStyle mark の丸の直径
	MarkGap        float64 `json:"markGap" scale:"space"` // cellStyle mark の丸と文の間
	// CellStyle はセルの描き方: "filled"(塗りの箱に状態の色の丸と文)か "mark"
	// (塗らない。support は丸にチェック、none は何も描かない、その他は状態の色の丸。
	// 文は丸の下に小さい字で)。
	CellStyle string `json:"cellStyle"`
}

// matrixCellStyles は cellStyle に書ける形。
var matrixCellStyles = []string{"filled", "mark"}

func (v *MatrixValues) check(at string) error {
	if !contains(matrixCellStyles, v.CellStyle) {
		return fmt.Errorf("%s.cellStyle: unknown style %q (known: %s)", at, v.CellStyle, strings.Join(matrixCellStyles, ", "))
	}
	return nil
}

func (v *MatrixValues) mark() bool { return v.CellStyle == "mark" }

// MatrixCell は1つのセル。
type MatrixCell struct {
	Status string // figureStatuses のどれか(必須)
	Text   Inline // 無ければ丸だけ
}

// MatrixRow は1つの行。Detail は題の下の説明の1つの文(Points が nil)か、箇条(Points)。
type MatrixRow struct {
	Label  Inline
	Detail Inline
	Points []Inline
	Cells  []MatrixCell
}

// MatrixColumnGroup は列の群(列の見出しの上に、Span 列をまたいで置く名前)。
type MatrixColumnGroup struct {
	Title Inline
	Span  int
}

// MatrixData は IR の data を読んだもの。
type MatrixData struct {
	Columns []Inline
	// ColumnGroups は列の群(左から。span の合計は列の数)。無ければ nil。
	ColumnGroups []MatrixColumnGroup
	Rows         []MatrixRow
	Legend       []LegendItem // 書いた順(状態 → 文)
	// Emphasis は推す列(columns の位置)。無ければ -1。
	Emphasis int
	// DetailColumn は行の説明(detail)の列の見出し(#143)。あると説明を行の題の下で
	// なく、題の右の見出しつきの列に置く。無ければ nil(題の下)。
	DetailColumn Inline
}

// detailCol は行の説明を独立した列に置くか(detailColumn がある)。
func (s *MatrixSubsystem) detailCol() bool { return len(s.Data.DetailColumn) > 0 }

// matrixDefaultLegend は legend に書いていない状態の凡例の文。
var matrixDefaultLegend = map[string]string{
	"support": "対応", "conditional": "条件つき", "none": "非対応", "danger": "問題あり",
}

// MatrixSubsystem は MatrixData を描くサブシステム。
type MatrixSubsystem struct {
	Data *MatrixData
	V    *MatrixValues // nil なら既定(subsystems/matrix.json)
	Text *FigureText   // nil なら既定のテンプレート(light)の段
}

func NewMatrixSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*MatrixData); ok {
		return &MatrixSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("matrix", data)
	if err != nil {
		return nil, err
	}
	d, err := matrixFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &MatrixSubsystem{Data: d}, nil
}

func (s *MatrixSubsystem) Name() string { return "matrix" }

func (s *MatrixSubsystem) vals() *MatrixValues {
	if s.V != nil {
		return s.V
	}
	return defaultMatrixValues()
}

func (s *MatrixSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

// headerRuns は列の見出しの文。推す列は太字で、色はアクセント。
func (s *MatrixSubsystem) headerRuns(j int, color string, roles Roles) []TextRun {
	if j != s.Data.Emphasis {
		return itemRuns(s.Data.Columns[j], color, roles)
	}
	runs := itemRuns(s.Data.Columns[j], "accent", roles)
	for k := range runs {
		runs[k].Bold = true
	}
	return runs
}

// legend は凡例: 使った状態を support・conditional・none・danger の順に、文は
// data の legend(無ければ既定)。
func (s *MatrixSubsystem) legend() []LegendItem {
	used := map[string]bool{}
	for _, r := range s.Data.Rows {
		for _, c := range r.Cells {
			used[c.Status] = true
		}
	}
	label := map[string]string{}
	for _, li := range s.Data.Legend {
		label[li.Role] = li.Label
	}
	var out []LegendItem
	for _, st := range figureStatuses {
		if !used[st] || (st == "none" && s.vals().mark()) {
			continue // mark の none は何も描かないので、凡例の印も無い
		}
		l, ok := label[st]
		if !ok {
			l = matrixDefaultLegend[st]
		}
		out = append(out, LegendItem{Role: st, Label: l})
	}
	return out
}

type matrixGeom struct {
	inner      float64
	groupNeed  float64 // 列の群の行の高さ(群が無ければ 0)
	headW      float64 // 行の見出しの列
	detW       float64 // 説明の列(detailColumn が無ければ 0)
	detGap     float64 // 行の見出しと説明の列の間(detailColumn が無ければ 0)
	colW       float64
	headerNeed float64
	rowNeed    []float64
	legendH    float64
	legendRows [][]int // 凡例の行ごとの項目
	parts      []flexPart
}

// cellTextWidth はセルの文に使える幅(丸と余白を除く。mark は丸の下なので余白だけ)。
func (s *MatrixSubsystem) cellTextWidth(colW float64) float64 {
	v := s.vals()
	if v.mark() {
		return colW - 2*v.TextInsetX
	}
	return colW - 2*v.TextInsetX - v.DotSize - v.DotGap
}

// cellTextSize はセルの文の字の大きさ(mark は小さい字)。
func (s *MatrixSubsystem) cellTextSize() float64 {
	if s.vals().mark() {
		return s.text().Small
	}
	return s.text().Sub
}

// cellNeed はセルの中身の高さ(余白を除く)。mark は丸と、文があれば空きと文。
func (s *MatrixSubsystem) cellNeed(c MatrixCell, colW float64) float64 {
	v := s.vals()
	_, h := runsBox(itemRuns(c.Text, "", nil), s.cellTextSize(), s.cellTextWidth(colW))
	if !v.mark() {
		return h
	}
	if len(c.Text) == 0 {
		return v.MarkSize
	}
	return v.MarkSize + v.MarkGap + h
}

// rowHeadParas は行の見出しの段落:題と、説明(1つの文か、印つきの箇条)。説明の
// 段落の字の大きさは tx.Detail。
func (s *MatrixSubsystem) rowHeadParas(r *MatrixRow, color, markColor string, roles Roles) (label []TextRun, detail [][]TextRun) {
	label = itemRuns(r.Label, color, roles)
	if len(r.Points) == 0 {
		if len(r.Detail) > 0 {
			detail = [][]TextRun{itemRuns(r.Detail, color, roles)}
		}
		return
	}
	mc := ""
	if roles != nil {
		mc = roles.Color(markColor)
	}
	for _, p := range r.Points {
		detail = append(detail, append([]TextRun{{Text: s.text().Point + " ", Color: mc, Bold: true, Mark: true}}, itemRuns(p, color, roles)...))
	}
	return
}

// rowHeadNeed は幅 w の行の見出しに題と説明が収まる幅(一番長い語)と高さ(余白を除く)。
// 説明が独立した列(detailColumn)なら題だけ。
func (s *MatrixSubsystem) rowHeadNeed(r *MatrixRow, w float64) (needW, needH float64) {
	tx := s.text()
	label, _ := s.rowHeadParas(r, "", "", nil)
	_, needH = runsBox(label, tx.RowTitle, w)
	needW = runsMinWidth(label, tx.RowTitle)
	if s.detailCol() {
		return needW, needH
	}
	dw, dh := s.detailNeed(r, w)
	return math.Max(needW, dw), needH + dh
}

// detailNeed は幅 w に行の説明が収まる幅(一番長い語と印)と高さ(余白を除く)。
func (s *MatrixSubsystem) detailNeed(r *MatrixRow, w float64) (needW, needH float64) {
	tx := s.text()
	_, detail := s.rowHeadParas(r, "", "", nil)
	markW := 0.0
	if len(r.Points) > 0 {
		markW = runWidthPt(tx.Point+" ", tx.Detail, true)
	}
	for _, p := range detail {
		if markW > 0 {
			p = p[1:] // 印の分は字下げとして幅から引く
		}
		_, h := runsBox(p, tx.Detail, w-markW)
		needH += h
		needW = math.Max(needW, runsMinWidth(p, tx.Detail)+markW)
	}
	return needW, needH
}

// detailHeight は幅 w に行の説明が収まる高さ(余白を除く。detailNeed の高さだけ)。
func (s *MatrixSubsystem) detailHeight(r *MatrixRow, w float64) (needH float64) {
	tx := s.text()
	_, detail := s.rowHeadParas(r, "", "", nil)
	markW := 0.0
	if len(r.Points) > 0 {
		markW = runWidthPt(tx.Point+" ", tx.Detail, true)
	}
	for _, p := range detail {
		if markW > 0 {
			p = p[1:]
		}
		_, h := runsBox(p, tx.Detail, w-markW)
		needH += h
	}
	return needH
}

// groupRuns は列の群の名前(太字)。
func groupRuns(g MatrixColumnGroup, color string, roles Roles) []TextRun {
	runs := itemRuns(g.Title, color, roles)
	for k := range runs {
		runs[k].Bold = true
	}
	return runs
}

// groupWidth は span 列をまたぐ幅。
func (s *MatrixSubsystem) groupWidth(colW float64, span int) float64 {
	return float64(span)*colW + float64(span-1)*s.vals().CellGap
}

// matrixDot は状態の丸の字。
const matrixDot = "●"

// matrixDotInk は ● の字の大きさに対する丸の直径の割合(和文の字体のおおよそ)。
const matrixDotInk = 0.9

// dotMark はセルの文の頭の印(● と、丸と文の間の空き)。● の字の大きさは、
// 丸の直径が DotSize になる大きさ。印の幅は丸と空きの和
// (DotSize+DotGap)になるように、空きの字の大きさで合わせる。PPTX では段落の
// 箇条の印になり、文は印の幅だけ字下げした位置から始まる。
func (s *MatrixSubsystem) dotMark(color string) []TextRun {
	v := s.vals()
	sz := v.DotSize / matrixDotInk
	mark := []TextRun{{Text: matrixDot, Color: color, Size: sz, Mark: true}}
	if gap := v.DotSize + v.DotGap - charEmWidth([]rune(matrixDot)[0], false)*sz; gap > 0 {
		mark = append(mark, TextRun{Text: " ", Color: color, Size: gap / charEmWidth(' ', false), Mark: true})
	}
	return mark
}

func (s *MatrixSubsystem) legendItemW(li LegendItem) float64 {
	v, tx := s.vals(), s.text()
	return v.LegendDotSize + v.DotGap/2 + runWidthPt(li.Label, tx.Small, false)
}

// matrixWidths は行の見出しの列と評価の列の幅の目安(余白を含む)。nat は折り返さない
// 幅、floor は語の途中で折らない幅(和文は tableCJKChunk 字の塊)。評価の列の min は
// 印(mark なら丸)と一番長い語が入る幅(これより狭いと figure-text-overflow)。行の題の
// 列の最小は floor(これより狭いと題が1〜2字ずつに折れて行が縦に伸びる)。headMin・detMin は
// 文の列の一番長い語が入る幅(欧文の語。Min の幅を割ったときに、評価の列の min より先に
// 譲る下限。#145)。
type matrixWidths struct {
	headNat, headFloor, headMin float64
	colNat, colFloor, colMin    float64
	// 説明の列(detailColumn、#143)。無ければどれも 0。detGap は題の列との間(CellGap)。
	detNat, detFloor, detMin, detGap float64
}

func (s *MatrixSubsystem) widths() matrixWidths {
	v, tx := s.vals(), s.text()
	pad := 2 * v.TextInsetX
	var m matrixWidths
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		label, detail := s.rowHeadParas(r, "", "", nil)
		m.headNat = math.Max(m.headNat, runsNaturalWidth(label, tx.RowTitle))
		m.headFloor = math.Max(m.headFloor, runsChunkWidth(label, tx.RowTitle))
		m.headMin = math.Max(m.headMin, runsMinWidth(label, tx.RowTitle))
		nat, floor, least := &m.headNat, &m.headFloor, &m.headMin
		if s.detailCol() {
			nat, floor, least = &m.detNat, &m.detFloor, &m.detMin
		}
		for _, p := range detail {
			*nat = math.Max(*nat, runsNaturalWidth(p, tx.Detail))
			markW := 0.0
			if len(r.Points) > 0 {
				markW = runWidthPt(tx.Point+" ", tx.Detail, true)
				p = p[1:]
			}
			*floor = math.Max(*floor, runsChunkWidth(p, tx.Detail)+markW)
			*least = math.Max(*least, runsMinWidth(p, tx.Detail)+markW)
		}
	}
	m.headNat += pad
	m.headFloor = math.Min(m.headFloor+pad, m.headNat)
	m.headMin = math.Min(m.headMin+pad, m.headFloor)
	if s.detailCol() {
		h := s.detailHeaderRuns("", nil)
		m.detNat = math.Max(m.detNat, runsNaturalWidth(h, tx.Title)) + pad
		m.detFloor = math.Min(math.Max(m.detFloor, runsChunkWidth(h, tx.Title))+pad, m.detNat)
		m.detMin = math.Min(math.Max(m.detMin, runsMinWidth(h, tx.Title))+pad, m.detFloor)
		m.detGap = v.CellGap
	}
	// 列は同じ幅なので、一番広く要る列で測る。
	for j := range s.Data.Columns {
		h := s.headerRuns(j, "", nil)
		m.colNat = math.Max(m.colNat, runsNaturalWidth(h, tx.Title))
		m.colFloor = math.Max(m.colFloor, runsChunkWidth(h, tx.Title))
		m.colMin = math.Max(m.colMin, runsMinWidth(h, tx.Title))
	}
	lead := v.DotSize + v.DotGap // filled: 丸と空きの後に文
	if v.mark() {
		lead = 0
		m.colNat, m.colFloor, m.colMin = math.Max(m.colNat, v.MarkSize), math.Max(m.colFloor, v.MarkSize), math.Max(m.colMin, v.MarkSize)
	}
	for _, r := range s.Data.Rows {
		for _, c := range r.Cells {
			if len(c.Text) == 0 {
				continue
			}
			runs := itemRuns(c.Text, "", nil)
			m.colNat = math.Max(m.colNat, lead+runsNaturalWidth(runs, s.cellTextSize()))
			m.colFloor = math.Max(m.colFloor, lead+runsChunkWidth(runs, s.cellTextSize()))
			m.colMin = math.Max(m.colMin, lead+runsMinWidth(runs, s.cellTextSize()))
		}
	}
	m.colNat += pad
	m.colFloor = math.Min(m.colFloor+pad, m.colNat)
	m.colMin = math.Min(m.colMin+pad, m.colFloor)
	return m
}

// minWidth は行の見出しの列と評価の列がどちらも最小(行の題は floor、評価は min)を
// 保てる一番狭い幅(余白とセルの間を含む)。これより狭いと評価の列の文が横に
// はみ出す(figure-text-overflow)。Measure の Min の幅(rowHeadWidth の段 4 の境)。
func (m matrixWidths) minWidth(v *MatrixValues, cols int) float64 {
	C := float64(cols)
	return 2*v.Padding + C*v.CellGap + m.headFloor + C*m.colMin + m.detGap + m.detFloor
}

// detailHeaderRuns は説明の列の見出しの文。
func (s *MatrixSubsystem) detailHeaderRuns(color string, roles Roles) []TextRun {
	return itemRuns(s.Data.DetailColumn, color, roles)
}

// colWidths は内側の幅 inner のときの行の見出しの列・説明の列・評価の列の幅。説明の列
// (detailColumn)が無ければ説明の列は 0 で、行の見出しは rowHeadWidth。
func (s *MatrixSubsystem) colWidths(inner float64) (headW, detW, colW float64) {
	v := s.vals()
	C := float64(len(s.Data.Columns))
	if !s.detailCol() {
		headW = s.rowHeadWidth(inner)
		return headW, 0, math.Max(0, (inner-headW-C*v.CellGap)/C)
	}
	headW, detW, colW = s.detailColWidths(inner)
	return
}

// detailColWidths は説明の列があるときの幅(#143・#144)。評価の列は中身の自然な幅
// (印・短い文・列の見出しの折り返し後)の最小に留め、残りの幅を題の列と説明の列に配る。
//  1. 文の列の floor と評価の列の min(印と一番長い語。列の見出しは折り返す)が入れば、
//     評価の列の幅(min〜自然の幅)と題の列の幅(floor〜自然の幅)を matrixDetailStep の刻みで
//     探し、行の高さ(見出しの行・列の群・各行の最小)の合計が一番小さい組を選ぶ(pairs の #115
//     と同じ考え)。高さが matrixDetailTie 以内で同じなら、評価の列が狭い方、次に題と説明の
//     幅の比が文の量(自然の幅)の比に近い方。評価の列を広げても行が低くならなければ広げない
//     ので、評価の列は中身が要る幅に留まる(#143 は自然の幅を取り、文の列が潰れた)。
//  2. 文の列が一番長い語の幅(headMin・detMin)を保てれば、評価の列は min のまま、文の列が
//     floor を割る(文は字の境で折れて行が伸びるが、評価の列の短い語「p.35」は1字ずつ
//     折れない。#145)。
//  3. それも入らなければ語の幅の比で配る(figure-text-overflow)。
//
// 題は内側の幅の maxRowHeaderFraction まで(越えた分は説明へ)。Measure・place・Lint は
// どれもこの幅で測るので、Natural・Min の高さは描く形と一致する。
func (s *MatrixSubsystem) detailColWidths(inner float64) (headW, detW, colW float64) {
	v := s.vals()
	m := s.widths()
	C := float64(len(s.Data.Columns))
	avail := inner - C*v.CellGap - m.detGap
	textFloor, textMin := m.headFloor+m.detFloor, m.headMin+m.detMin
	if textFloor+C*m.colMin <= avail {
		return s.searchDetailWidths(inner, avail, m)
	}
	if textMin+C*m.colMin <= avail {
		colW = m.colMin
	} else {
		colW = math.Max(0, avail) * m.colMin / (textMin + C*m.colMin)
	}
	text := math.Max(0, avail-C*colW)
	if textFloor > 0 {
		headW = math.Min(text, textFloor) * m.headFloor / textFloor
	}
	headW = math.Min(headW, inner*v.RowHeaderFrac)
	return headW, text - headW, colW
}

// matrixDetailStep は説明の列があるときに幅を探す刻み(pt。同梱のテンプレートの spacingUnit)。
const matrixDetailStep = 2.0

// monotoneTable は f(0)…f(n-1) の表。f は x とともに増えるか減る(どちらか一方)とし、
// 区間の両端の値が同じなら間も同じ値で埋める(二分で、変わり目の近くだけ f を呼ぶ)。
func monotoneTable(n int, f func(int) float64) []float64 {
	out := make([]float64, n)
	if n == 0 {
		return out
	}
	var fill func(lo, hi int)
	fill = func(lo, hi int) {
		if hi-lo <= 1 {
			return
		}
		if out[lo] == out[hi] {
			for x := lo + 1; x < hi; x++ {
				out[x] = out[lo]
			}
			return
		}
		mid := (lo + hi) / 2
		out[mid] = f(mid)
		fill(lo, mid)
		fill(mid, hi)
	}
	out[0], out[n-1] = f(0), f(n-1)
	fill(0, n-1)
	return out
}

// matrixDetailTie は高さが同じとみなす差(pt)。
const matrixDetailTie = 0.5

// searchDetailWidths は detailColWidths の段 1:評価の列・題の列の幅を刻みで探す。評価の列は
// min(印と一番長い語。列の見出しは折り返す)から自然の幅まで、題の列は floor から自然の幅まで。
// 評価の列・題の列とも matrixDetailStep の刻みにとるので、説明の列の幅も同じ刻みの格子に乗り
// (評価の列を1刻み広げると C 刻み狭くなる)、題・説明の行ごとの高さは幅ごとに一度だけ測る。
func (s *MatrixSubsystem) searchDetailWidths(inner, avail float64, m matrixWidths) (headW, detW, colW float64) {
	v, tx := s.vals(), s.text()
	C0 := len(s.Data.Columns)
	C := float64(C0)
	n := len(s.Data.Rows)
	step := matrixDetailStep
	headLo := math.Min(m.headFloor, inner*v.RowHeaderFrac)
	base := avail - C*m.colMin - headLo // 評価の列が min、題が下限のときの説明の列の幅
	colHi := math.Min(m.colNat+noWrapSlack, (avail-headLo-m.detFloor)/C)
	headHi := math.Min(m.headNat+noWrapSlack, inner*v.RowHeaderFrac)
	share := (m.headNat + noWrapSlack) / (m.headNat + noWrapSlack + m.detNat) // 題の文の量の割合
	K := int(math.Floor((colHi-m.colMin)/step + 1e-9))
	J := int(math.Floor((headHi-headLo)/step + 1e-9))
	if J < 0 {
		J = 0
	}
	// 題の高さ(j ごと)と説明の高さ(C*k+j ごと)の表。文の高さは幅が広いほど低いか同じなので、
	// 両端で同じ高さなら間も同じとみなして二分で埋める(測るのは高さの変わり目の近くだけ)。
	// 説明が自然の幅で入る幅より広くても高さは同じなので、そこで止める。
	kjMin := min(max(0, int(math.Floor((base-m.detNat)/step))), C0*K+J)
	headTab := make([][]float64, n)
	detTab := make([][]float64, n+1) // 末尾は説明の列の見出し
	detHead := s.detailHeaderRuns("", nil)
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		headTab[i] = monotoneTable(J+1, func(j int) float64 {
			_, h := s.rowHeadNeed(r, headLo+float64(j)*step-2*v.TextInsetX)
			return h
		})
		detTab[i] = monotoneTable(C0*K+J+1-kjMin, func(x int) float64 {
			return s.detailHeight(r, math.Max(0, base-float64(kjMin+x)*step-2*v.TextInsetX))
		})
	}
	detTab[n] = monotoneTable(C0*K+J+1-kjMin, func(x int) float64 {
		_, h := runsBox(detHead, tx.Title, math.Max(0, base-float64(kjMin+x)*step-2*v.TextInsetX))
		return h
	})
	det := func(i, kj int) float64 { return detTab[i][max(kj, kjMin)-kjMin] }
	// 評価の列の幅(k ごと)で決まる高さ:列の見出し・列の群・セル。
	colAt := func(k int) float64 { return m.colMin + float64(k)*step }
	topTab := monotoneTable(K+1, func(k int) float64 {
		top := 0.0
		for j := range s.Data.Columns {
			_, h := runsBox(s.headerRuns(j, "", nil), tx.Title, colAt(k)-2*v.TextInsetX)
			top = math.Max(top, h+2*v.TextInsetY)
		}
		return top
	})
	groupTab := monotoneTable(K+1, func(k int) float64 {
		group := 0.0
		for _, gr := range s.Data.ColumnGroups {
			_, h := runsBox(groupRuns(gr, "", nil), tx.Title, s.groupWidth(colAt(k), gr.Span)-2*v.TextInsetX)
			group = math.Max(group, h+2*v.TextInsetY+v.GroupRuleW)
		}
		return group
	})
	cellTab := make([][]float64, n)
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		cellTab[i] = monotoneTable(K+1, func(k int) float64 {
			need := 0.0
			for _, c := range r.Cells {
				need = math.Max(need, s.cellNeed(c, colAt(k)))
			}
			return need
		})
	}
	type cand struct{ h, cw, hw, dw, d float64 }
	var cands []cand
	bestH := math.Inf(1)
	for k := 0; k <= K; k++ {
		cw := colAt(k)
		top, group := topTab[k], groupTab[k]
		cells := make([]float64, n)
		for i := range cells {
			cells[i] = cellTab[i][k]
		}
		text := avail - C*cw
		for j := 0; j <= J; j++ {
			hw := headLo + float64(j)*step
			dw := text - hw
			if dw < m.detFloor-1e-9 && j > 0 {
				break
			}
			kj := C0*k + j
			total := group + math.Max(top, det(n, kj)+2*v.TextInsetY)
			for i := 0; i < n; i++ {
				need := math.Max(math.Max(headTab[i][j], det(i, kj)), cells[i]) + 2*v.TextInsetY
				total += math.Max(need, v.MinRowH)
			}
			cands = append(cands, cand{total, cw, hw, dw, math.Abs(hw - text*share)})
			bestH = math.Min(bestH, total)
		}
	}
	// 一番低い高さから matrixDetailTie 以内のうち、評価の列が狭く、次に文の量の比に近いもの。
	var best *cand
	for k := range cands {
		c := &cands[k]
		if c.h > bestH+matrixDetailTie {
			continue
		}
		if best == nil || c.cw < best.cw-1e-9 || (math.Abs(c.cw-best.cw) <= 1e-9 && c.d < best.d) {
			best = c
		}
	}
	return best.hw, best.dw, best.cw
}

// rowHeadWidth は内側の幅 inner のときの行の見出しの列の幅。幅が減るにつれて次の
// 順に譲る(どの段の境でも幅はつながる、#138):
//  1. 両方が自然の幅で入れば行の見出しは自然の幅。
//  2. 両方の floor が入れば、floor を先に取り、残りを自然の幅と floor の差の比で配る
//     (表の列と同じ配り方)。
//     行の見出しは自然の幅と floor の差が大きいので先に縮む(題は折り返す)。
//  3. 行の見出しの列は最小(floor)のまま、評価の列を min(印と短い文が入る幅)まで
//     縮める。以前はここで自然の幅の比で配っていて、評価の列が数 pt に崩れた。
//  4. 行の見出しの列が一番長い語の幅(headMin)を保てれば、評価の列は min のまま、行の
//     見出しの列が floor を割る(#145。Min の幅をわずかに割ったとき、以前は最小の比で
//     配っていて、評価の列の短い語「p.35」が 0.x pt 足りずにアプリで1字ずつ折れ、行の
//     高さが数倍になった。題は字の境で折れて行が伸びるだけ)。
//  5. それも入らなければ語の幅の比で配る(文が横にはみ出し、figure-text-overflow)。
//
// どの場合も内側の幅の maxRowHeaderFraction まで。
func (s *MatrixSubsystem) rowHeadWidth(inner float64) float64 {
	v := s.vals()
	m := s.widths()
	C := float64(len(s.Data.Columns))
	avail := inner - C*v.CellGap
	var w float64
	switch {
	case m.headNat+C*m.colNat <= avail:
		w = m.headNat
	case m.headFloor+C*m.colFloor <= avail:
		span := (m.headNat - m.headFloor) + C*(m.colNat-m.colFloor)
		w = m.headFloor + (avail-m.headFloor-C*m.colFloor)*(m.headNat-m.headFloor)/span
	case m.headFloor+C*m.colMin <= avail:
		w = m.headFloor
	case m.headMin+C*m.colMin <= avail:
		w = avail - C*m.colMin
	default:
		w = math.Max(0, avail) * m.headMin / (m.headMin + C*m.colMin)
	}
	return math.Min(w, inner*v.RowHeaderFrac)
}

func (s *MatrixSubsystem) geom(width float64) *matrixGeom {
	v, tx := s.vals(), s.text()
	g := &matrixGeom{inner: math.Max(0, width-2*v.Padding)}
	g.headW, g.detW, g.colW = s.colWidths(g.inner)
	if s.detailCol() {
		g.detGap = v.CellGap
	}

	for j := range s.Data.Columns {
		_, h := runsBox(s.headerRuns(j, "", nil), tx.Title, g.colW-2*v.TextInsetX)
		g.headerNeed = math.Max(g.headerNeed, h+2*v.TextInsetY)
	}
	if s.detailCol() {
		_, h := runsBox(s.detailHeaderRuns("", nil), tx.Title, g.detW-2*v.TextInsetX)
		g.headerNeed = math.Max(g.headerNeed, h+2*v.TextInsetY)
	}
	for _, gr := range s.Data.ColumnGroups {
		_, h := runsBox(groupRuns(gr, "", nil), tx.Title, s.groupWidth(g.colW, gr.Span)-2*v.TextInsetX)
		g.groupNeed = math.Max(g.groupNeed, h+2*v.TextInsetY+v.GroupRuleW)
	}
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		_, need := s.rowHeadNeed(r, g.headW-2*v.TextInsetX)
		if s.detailCol() {
			_, dh := s.detailNeed(r, g.detW-2*v.TextInsetX)
			need = math.Max(need, dh)
		}
		for _, c := range r.Cells {
			need = math.Max(need, s.cellNeed(c, g.colW))
		}
		g.rowNeed = append(g.rowNeed, need+2*v.TextInsetY)
	}

	// 凡例: 1列目のセルの左端から並べ、幅を越えたら次の行へ。
	legend := s.legend()
	avail := g.inner - g.headW - v.CellGap // 凡例は行の見出しの右(説明の列があればその左端)から
	x := 0.0
	for i, li := range legend {
		w := s.legendItemW(li)
		if len(g.legendRows) == 0 || (x > 0 && x+w > avail) {
			g.legendRows = append(g.legendRows, nil)
			x = 0
		}
		g.legendRows[len(g.legendRows)-1] = append(g.legendRows[len(g.legendRows)-1], i)
		x += w + v.LegendItemGap
	}
	g.legendH = float64(len(g.legendRows)) * tx.Small * lineHeightMult()

	head := math.Max(g.headerNeed, v.HeaderH)
	g.parts = []flexPart{fixedPart(v.Padding)}
	if g.groupNeed > 0 {
		g.parts = append(g.parts, fixedPart(g.groupNeed))
	}
	g.parts = append(g.parts, flexPart{g.headerNeed, head, head})
	for _, need := range g.rowNeed {
		g.parts = append(g.parts, fixedPart(v.CellGap),
			flexPart{math.Max(need, v.MinRowH), math.Max(need, v.NaturalRowH), math.Max(need, v.MaxRowH)})
	}
	if g.legendH > 0 {
		g.parts = append(g.parts, fixedPart(v.LegendGap+g.legendH))
	}
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

// cellsX は内側の左端から、評価の列の前の空きまでの幅(行の見出しと説明の列)。
func (g *matrixGeom) cellsX() float64 {
	return g.headW + g.detGap + g.detW
}

// Measure: 高さは見出しの行・各行(文が収まる高さ以上)・凡例の、最小・自然・
// 上限の和。行の見出しの列は一番長い見出しの幅(内側の幅の一定の割合まで)、
// 残りを列に等しく分ける。
func (s *MatrixSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	w := v.NaturalWidth
	if c.Width > 0 {
		w = c.Width
	}
	min, nat, max := flexTotals(s.geom(w).parts)
	return Measurement{
		Natural: Size{Width: w, Height: nat},
		Min:     Size{Width: math.Max(v.MinWidth, s.widths().minWidth(v, len(s.Data.Columns))), Height: min},
		Max:     Size{Height: max},
	}
}

type matrixPlaced struct {
	g       *matrixGeom
	groups  []LBox // 列の群の名前の箱(下の端に罫線)
	header  []LBox
	heads   []LBox
	detHead LBox   // 説明の列の見出し(detailColumn が無ければ空)
	dets    []LBox // 行ごとの説明の箱(detailColumn が無ければ nil)
	cells   [][]LBox
	legendY float64
	x0      float64 // 内側の左端
}

func (s *MatrixSubsystem) place(box LBox) *matrixPlaced {
	v := s.vals()
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	pl := &matrixPlaced{g: g, x0: box.X + v.Padding}
	colX := func(j int) float64 { return pl.x0 + g.cellsX() + v.CellGap + float64(j)*(g.colW+v.CellGap) }
	detX := pl.x0 + g.headW + g.detGap
	y := box.Y + sizes[0]
	k := 1
	if g.groupNeed > 0 {
		j := 0
		for _, gr := range s.Data.ColumnGroups {
			pl.groups = append(pl.groups, LBox{X: colX(j), Y: y, Width: s.groupWidth(g.colW, gr.Span), Height: sizes[k]})
			j += gr.Span
		}
		y += sizes[k]
		k++
	}
	for j := range s.Data.Columns {
		pl.header = append(pl.header, LBox{X: colX(j), Y: y, Width: g.colW, Height: sizes[k]})
	}
	if s.detailCol() {
		pl.detHead = LBox{X: detX, Y: y, Width: g.detW, Height: sizes[k]}
	}
	y += sizes[k]
	for i, r := range s.Data.Rows {
		y += sizes[k+1+2*i]
		h := sizes[k+2+2*i]
		pl.heads = append(pl.heads, LBox{X: pl.x0, Y: y, Width: g.headW, Height: h})
		if s.detailCol() {
			pl.dets = append(pl.dets, LBox{X: detX, Y: y, Width: g.detW, Height: h})
		}
		var row []LBox
		for j := range r.Cells {
			row = append(row, LBox{X: colX(j), Y: y, Width: g.colW, Height: h})
		}
		pl.cells = append(pl.cells, row)
		y += h
	}
	pl.legendY = y + v.LegendGap
	return pl
}

func (s *MatrixSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	pl := s.place(box)
	p := ctx.Out
	header := func(b LBox, runs []TextRun, size float64, align string) {
		p.AddShape(b, ShapeOpts{Prst: "rect", Runs: runs, FontSize: size,
			InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: align})
	}
	for k, gr := range s.Data.ColumnGroups {
		b := pl.groups[k]
		p.AddShape(LBox{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height - v.GroupRuleW}, ShapeOpts{Prst: "rect",
			Runs: groupRuns(gr, "text_primary", ctx.Roles), FontSize: tx.Title, InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "center"})
		ry := b.Y + b.Height - v.GroupRuleW/2
		p.AddLine(b.X, ry, b.X+b.Width, ry, v.GroupRuleW, ctx.Roles.Color("grid"), "")
	}
	for j := range s.Data.Columns {
		header(pl.header[j], s.headerRuns(j, "text_primary", ctx.Roles), tx.Title, "center")
	}
	if s.detailCol() {
		header(pl.detHead, s.detailHeaderRuns("text_primary", ctx.Roles), tx.Title, "left")
	}
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		label, detail := s.rowHeadParas(r, "text_primary", "accent", ctx.Roles)
		if s.detailCol() {
			header(pl.heads[i], label, tx.RowTitle, "left")
			if len(detail) > 0 {
				// 説明の列:題と同じく縦の真ん中、左に揃える。
				o := ShapeOpts{Prst: "rect", Runs: detail[0], FontSize: tx.Detail, InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "left"}
				for _, para := range detail[1:] {
					o.Paras = append(o.Paras, para)
					o.ParaSizes = append(o.ParaSizes, tx.Detail)
				}
				p.AddShape(pl.dets[i], o)
			}
		} else if len(detail) == 0 {
			header(pl.heads[i], label, tx.RowTitle, "left")
		} else {
			o := ShapeOpts{Prst: "rect", Runs: label, FontSize: tx.RowTitle, InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "left", Anchor: "middle"}
			for _, para := range detail {
				o.Paras = append(o.Paras, para)
				o.ParaSizes = append(o.ParaSizes, tx.Detail)
			}
			p.AddShape(pl.heads[i], o)
		}
		if v.mark() {
			for j, c := range r.Cells {
				s.renderMarkCell(ctx, pl.cells[i][j], c)
			}
			continue
		}
		for j, c := range r.Cells {
			// 状態の丸は文の段落の印(a:buChar ●、a:buClr に状態の色)。文が無いセルは
			// ● だけを真ん中に置く。丸はセルの図形の中にあるので、セルと一緒に動く。
			o := ShapeOpts{Prst: "roundRect", Fill: ctx.Roles.Color("card_alt"), Radius: v.CellCornerRadi, FontSize: tx.Sub,
				InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "left"}
			b := pl.cells[i][j]
			color := ctx.Roles.Color(c.Status)
			if len(c.Text) > 0 {
				o.Runs = append(s.dotMark(color), itemRuns(c.Text, "text_primary", ctx.Roles)...)
			} else {
				// 印は文のある段落にしか出ないので、幅の無い文字(U+200B)を置き、
				// 丸が真ん中に来るように左の余白を取る。
				o.Runs = append(s.dotMark(color)[:1], TextRun{Text: "\u200b", Color: color})
				o.InsetX = math.Max(0, (b.Width-v.DotSize)/2)
			}
			p.AddShape(b, o)
		}
	}
	// 推す列: 見出しから最後の行までを、アクセントの色の枠で囲む(塗らない)。
	if j := s.Data.Emphasis; j >= 0 {
		top, last := pl.header[j], pl.cells[len(pl.cells)-1][j]
		pad := v.CellGap / 2 // 枠はセルとセルの間の真ん中に
		p.AddShape(LBox{X: top.X - pad, Y: top.Y, Width: top.Width + 2*pad, Height: last.Y + last.Height + pad - top.Y},
			ShapeOpts{Prst: "roundRect", Outline: ctx.Roles.Color("accent"), OutlineWidthPt: v.EmphasisLineW, Radius: v.CellCornerRadi + pad})
	}
	// 凡例(1列目のセルの左端から)。
	legend := s.legend()
	lineH := tx.Small * lineHeightMult()
	for k, row := range pl.g.legendRows {
		x := pl.x0 + pl.g.headW + v.CellGap
		y := pl.legendY + float64(k)*lineH
		for _, i := range row {
			li := legend[i]
			if v.mark() {
				drawMatrixMark(ctx, x, y+(lineH-v.LegendDotSize)/2, v.LegendDotSize, li.Role)
			} else {
				p.AddDot(x+v.LegendDotSize/2, y+lineH/2, v.LegendDotSize/2, ctx.Roles.Color(li.Role))
			}
			lx := x + v.LegendDotSize + v.DotGap/2
			w := runWidthPt(li.Label, tx.Small, false)
			p.AddTextBoxNoWrap(LBox{X: lx, Y: y, Width: w + noWrapSlack, Height: lineH}, li.Label, tx.Small, ctx.Roles.Color("text_muted"), false, "left", "top")
			x += s.legendItemW(li) + v.LegendItemGap
		}
	}
}

// matrixCheck はチェックの折れ線の3つの点(丸の直径に対する割合)と、線の太さの割合。
var matrixCheck = [3][2]float64{{0.27, 0.52}, {0.43, 0.68}, {0.73, 0.35}}

const matrixCheckWidth = 0.12

// drawMatrixMark は直径 d の丸(左上 x, y)を状態の色で描く。support は丸に白い
// チェック(bg の色の2本の線)を重ねる。
func drawMatrixMark(ctx RenderContext, x, y, d float64, status string) {
	ctx.Out.AddShape(LBox{X: x, Y: y, Width: d, Height: d}, ShapeOpts{Prst: "ellipse", Fill: ctx.Roles.Color(status)})
	if status != "support" {
		return
	}
	c, w := ctx.Roles.Color("bg"), d*matrixCheckWidth
	for k := 0; k < 2; k++ {
		a, b := matrixCheck[k], matrixCheck[k+1]
		ctx.Out.AddLine(x+a[0]*d, y+a[1]*d, x+b[0]*d, y+b[1]*d, w, c, "")
	}
}

// renderMarkCell は cellStyle mark のセル:面は塗らず、丸(none は無し)を横の
// 真ん中に、文をその下に小さい字で置く。丸と文の塊はセルの縦の真ん中。
func (s *MatrixSubsystem) renderMarkCell(ctx RenderContext, b LBox, c MatrixCell) {
	v := s.vals()
	hasMark := c.Status != "none"
	if !hasMark && len(c.Text) == 0 {
		return // 当てはまらないセルは空ける
	}
	top := b.Y + (b.Height-s.cellNeed(c, b.Width))/2
	if hasMark {
		drawMatrixMark(ctx, b.X+(b.Width-v.MarkSize)/2, top, v.MarkSize, c.Status)
	}
	if len(c.Text) == 0 {
		return
	}
	insetTop := top - b.Y + v.MarkSize + v.MarkGap
	ctx.Out.AddShape(LBox{X: b.X, Y: b.Y + insetTop - v.TextInsetY, Width: b.Width, Height: b.Height - insetTop + v.TextInsetY},
		ShapeOpts{Prst: "rect", Runs: itemRuns(c.Text, "text_primary", ctx.Roles), FontSize: s.cellTextSize(),
			InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "center", Anchor: "top"})
}

// Lint は見出し・セルの文が収まらないことを警告にする。
func (s *MatrixSubsystem) Lint(box LBox) []string {
	v, tx := s.vals(), s.text()
	pl := s.place(box)
	var out []string
	check := func(where string, in Inline, size, textW float64, b LBox) {
		if len(in) == 0 {
			return
		}
		runs := itemRuns(in, "", nil)
		needW := runsMinWidth(runs, size)
		_, needH := runsBox(runs, size, textW)
		haveH := b.Height - 2*v.TextInsetY
		if !textFits(needW, needH, textW, haveH) {
			out = append(out, overflowWarning("matrix", where, in, needW, needH, textW, haveH))
		}
	}
	for j, c := range s.Data.Columns {
		b := pl.header[j]
		if j == s.Data.Emphasis {
			c = Inline{{Type: "em", Content: c}} // 太字で測る
		}
		check(fmt.Sprintf("columns[%d]", j), c, tx.Title, b.Width-2*v.TextInsetX, b)
	}
	if s.detailCol() {
		check("detailColumn", s.Data.DetailColumn, tx.Title, pl.detHead.Width-2*v.TextInsetX, pl.detHead)
	}
	for k, gr := range s.Data.ColumnGroups {
		b := pl.groups[k]
		check(fmt.Sprintf("columnGroups[%d]", k), Inline{{Type: "em", Content: gr.Title}}, tx.Title, b.Width-2*v.TextInsetX,
			LBox{Width: b.Width, Height: b.Height - v.GroupRuleW})
	}
	for i := range s.Data.Rows {
		r := &s.Data.Rows[i]
		b := pl.heads[i]
		if s.detailCol() && (len(r.Detail) > 0 || len(r.Points) > 0) {
			db := pl.dets[i]
			textW, haveH := db.Width-2*v.TextInsetX, db.Height-2*v.TextInsetY
			if needW, needH := s.detailNeed(r, textW); !textFits(needW, needH, textW, haveH) {
				in := r.Detail
				if len(in) == 0 {
					in = r.Points[0]
				}
				out = append(out, overflowWarning("matrix", fmt.Sprintf("rows[%d].detail", i), in, needW, needH, textW, haveH))
			}
		}
		if s.detailCol() || (len(r.Detail) == 0 && len(r.Points) == 0) {
			check(fmt.Sprintf("rows[%d].label", i), r.Label, tx.RowTitle, b.Width-2*v.TextInsetX, b)
		} else {
			textW, haveH := b.Width-2*v.TextInsetX, b.Height-2*v.TextInsetY
			if needW, needH := s.rowHeadNeed(r, textW); !textFits(needW, needH, textW, haveH) {
				out = append(out, overflowWarning("matrix", fmt.Sprintf("rows[%d].label", i), r.Label, needW, needH, textW, haveH))
			}
		}
		for j, c := range r.Cells {
			cb := pl.cells[i][j]
			if v.mark() && len(c.Text) > 0 {
				// 丸の下の文:セルの高さから丸と空きを引いた分に収まるか。
				textW := s.cellTextWidth(cb.Width)
				runs := itemRuns(c.Text, "", nil)
				needW := runsMinWidth(runs, s.cellTextSize())
				needH := s.cellNeed(c, cb.Width)
				if haveH := cb.Height - 2*v.TextInsetY; !textFits(needW, needH, textW, haveH) {
					out = append(out, overflowWarning("matrix", fmt.Sprintf("rows[%d].cells[%d]", i, j), c.Text, needW, needH, textW, haveH))
				}
				continue
			}
			check(fmt.Sprintf("rows[%d].cells[%d]", i, j), c.Text, s.cellTextSize(), s.cellTextWidth(cb.Width), cb)
		}
	}
	return out
}

// --- IR から読む ---

type matrixIR struct {
	Columns      []Inline `json:"columns"`
	ColumnGroups []struct {
		Title Inline `json:"title"`
		Span  int    `json:"span"`
	} `json:"columnGroups"`
	Rows []struct {
		Label  Inline          `json:"label"`
		Detail json.RawMessage `json:"detail"`
		Cells  []struct {
			Status string `json:"status"`
			Text   Inline `json:"text"`
		} `json:"cells"`
	} `json:"rows"`
	Legend       orderedLegend `json:"legend"`
	Emphasis     *string       `json:"emphasis"`
	DetailColumn *Inline       `json:"detailColumn"`
}

// matrixEmphasisColumn は emphasis(列の見出しの文)が指す列。見つからなければ
// 誤りの文を返す(check の matrix-emphasis と読み込みが同じものを使う)。
func matrixEmphasisColumn(columns []string, name string) (int, string) {
	for j, c := range columns {
		if c == name {
			return j, ""
		}
	}
	return -1, fmt.Sprintf("emphasis %q is not one of the columns (%s)", name, strings.Join(columns, ", "))
}

// matrixGroupSpans は列の群の span の合計が列の数と同じかを見る。違えば誤りの文
// (check の matrix-column-groups と読み込みが同じものを使う)。
func matrixGroupSpans(spans []int, columns int) string {
	sum := 0
	for _, n := range spans {
		sum += n
	}
	if sum != columns {
		return fmt.Sprintf("columnGroups spans add up to %d but there are %d columns (the spans must cover every column once)", sum, columns)
	}
	return ""
}

func matrixFromIR(raw json.RawMessage) (*MatrixData, error) {
	var in matrixIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("matrix: %w", err)
	}
	if len(in.Columns) == 0 {
		return nil, fmt.Errorf("matrix: columns needs at least one column")
	}
	if len(in.Rows) == 0 {
		return nil, fmt.Errorf("matrix: rows needs at least one row")
	}
	d := &MatrixData{Columns: in.Columns, Emphasis: -1}
	if in.DetailColumn != nil {
		if in.DetailColumn.Plain() == "" {
			return nil, fmt.Errorf("matrix: detailColumn: must not be empty")
		}
		d.DetailColumn = *in.DetailColumn
	}
	var names []string
	for j, c := range in.Columns {
		if len(c) == 0 {
			return nil, fmt.Errorf("matrix: columns[%d]: must not be empty", j)
		}
		names = append(names, c.Plain())
	}
	if in.Emphasis != nil {
		j, msg := matrixEmphasisColumn(names, *in.Emphasis)
		if j < 0 {
			return nil, fmt.Errorf("matrix: %s", msg)
		}
		d.Emphasis = j
	}
	if in.ColumnGroups != nil {
		var spans []int
		for k, gr := range in.ColumnGroups {
			if len(gr.Title) == 0 {
				return nil, fmt.Errorf("matrix: columnGroups[%d].title: must not be empty", k)
			}
			if gr.Span < 1 {
				return nil, fmt.Errorf("matrix: columnGroups[%d].span: must be 1 or more (got %d)", k, gr.Span)
			}
			spans = append(spans, gr.Span)
			d.ColumnGroups = append(d.ColumnGroups, MatrixColumnGroup{Title: gr.Title, Span: gr.Span})
		}
		if msg := matrixGroupSpans(spans, len(in.Columns)); msg != "" {
			return nil, fmt.Errorf("matrix: %s", msg)
		}
	}
	for _, li := range in.Legend {
		if !contains(figureStatuses, li.Role) {
			return nil, fmt.Errorf("matrix: legend: unknown key %q (known: support, conditional, none, danger)", li.Role)
		}
		d.Legend = append(d.Legend, li)
	}
	for i, r := range in.Rows {
		if len(r.Label) == 0 {
			return nil, fmt.Errorf("matrix: rows[%d].label: must not be empty", i)
		}
		if len(r.Cells) != len(in.Columns) {
			return nil, fmt.Errorf("matrix: rows[%d].cells: has %d cells but there are %d columns", i, len(r.Cells), len(in.Columns))
		}
		row := MatrixRow{Label: r.Label}
		if det := bytes.TrimSpace(r.Detail); isSentenceList(det) {
			if err := json.Unmarshal(det, &row.Points); err != nil {
				return nil, fmt.Errorf("matrix: rows[%d].detail: %w", i, err)
			}
			if len(row.Points) == 0 {
				return nil, fmt.Errorf("matrix: rows[%d].detail: a list needs at least one sentence", i)
			}
		} else if len(det) > 0 {
			if err := json.Unmarshal(det, &row.Detail); err != nil {
				return nil, fmt.Errorf("matrix: rows[%d].detail: %w", i, err)
			}
		}
		for j, c := range r.Cells {
			at := fmt.Sprintf("rows[%d].cells[%d]", i, j)
			if c.Status == "" {
				return nil, fmt.Errorf("matrix: %s.status: a cell needs a status", at)
			}
			if err := checkFigureStatus(at, c.Status); err != nil {
				return nil, fmt.Errorf("matrix: %w", err)
			}
			row.Cells = append(row.Cells, MatrixCell{Status: c.Status, Text: c.Text})
		}
		d.Rows = append(d.Rows, row)
	}
	return d, nil
}

func matrixDecl() SubsystemDecl {
	return figureDecl("subsystems/matrix.schema.json",
		"行×列の評価。セルは status(評価)と text(任意)。セルが文字・数だけなら matrix ではなく表(table)",
		"各行の cells の数は columns の数と同じ(違えば boxloom check が matrix-cells の誤りにする)",
		"凡例は使った状態から自動で作る。文を変えるときは legend: {support: 良い, …}",
		"行の題の下の説明は detail(1つの文か、文の列。列は箇条になる)",
		"detailColumn: <見出しの文> を書くと、各行の detail を題の下でなく題の右の見出しつきの列に置く(行が低くなる。評価の列は印・短い文・折り返した見出しが入る一番狭い幅になり、残りを題と説明の列が行の高さが一番低くなるように分ける)",
		"列の見出しの上の群の名前は columnGroups: [{title, span: またぐ列の数}](左から。span の合計は列の数。違えば check の matrix-column-groups の誤り)",
		"推す列(おすすめの選択肢)は emphasis: <列の見出しの文>。見出しと列がアクセントの色で目立つ。見出しに無い名前は check の matrix-emphasis の誤り",
	)
}
