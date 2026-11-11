// matrix サブシステム: 行×列の評価(docs/patterns.md の R6)。
//
// セルが「評価」(状態の役割名)を持つ表。セルの文字・数だけなら第1層の
// table(パターンの2.1)で、こちらは使わない。列の見出しは列の真ん中に、行の
// 見出しは左に揃えて縦の真ん中に置く。セルは塗りの箱に、状態の色の丸と文を
// 置く。下に、使った状態の凡例を自動で付ける(文は data の legend、無ければ
// 既定の文)。設計は docs/subsystem.md の11節。IR の data の形は
// schema/subsystems/matrix.schema.json。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
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
}

// MatrixCell は1つのセル。
type MatrixCell struct {
	Status string // figureStatuses のどれか(必須)
	Text   Inline // 無ければ丸だけ
}

// MatrixRow は1つの行。
type MatrixRow struct {
	Label Inline
	Cells []MatrixCell
}

// MatrixData は IR の data を読んだもの。
type MatrixData struct {
	Columns []Inline
	Rows    []MatrixRow
	Legend  []LegendItem // 書いた順(状態 → 文)
}

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
		if !used[st] {
			continue
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
	headW      float64 // 行の見出しの列
	colW       float64
	headerNeed float64
	rowNeed    []float64
	legendH    float64
	legendRows [][]int // 凡例の行ごとの項目
	parts      []flexPart
}

// cellTextWidth はセルの文に使える幅(丸と余白を除く)。
func (s *MatrixSubsystem) cellTextWidth(colW float64) float64 {
	v := s.vals()
	return colW - 2*v.TextInsetX - v.DotSize - v.DotGap
}

func (s *MatrixSubsystem) legendItemW(li LegendItem) float64 {
	v, tx := s.vals(), s.text()
	return v.LegendDotSize + v.DotGap/2 + runWidthPt(li.Label, tx.Small, false)
}

func (s *MatrixSubsystem) geom(width float64) *matrixGeom {
	v, tx := s.vals(), s.text()
	g := &matrixGeom{inner: math.Max(0, width-2*v.Padding)}
	C := float64(len(s.Data.Columns))
	for _, r := range s.Data.Rows {
		g.headW = math.Max(g.headW, runsNaturalWidth(itemRuns(r.Label, "", nil), tx.Title)+2*v.TextInsetX)
	}
	g.headW = math.Min(g.headW, g.inner*v.RowHeaderFrac)
	g.colW = math.Max(0, (g.inner-g.headW-C*v.CellGap)/C)

	for _, c := range s.Data.Columns {
		_, h := runsBox(itemRuns(c, "", nil), tx.Title, g.colW-2*v.TextInsetX)
		g.headerNeed = math.Max(g.headerNeed, h+2*v.TextInsetY)
	}
	for _, r := range s.Data.Rows {
		_, need := runsBox(itemRuns(r.Label, "", nil), tx.Title, g.headW-2*v.TextInsetX)
		for _, c := range r.Cells {
			_, h := runsBox(itemRuns(c.Text, "", nil), tx.Sub, s.cellTextWidth(g.colW))
			need = math.Max(need, h)
		}
		g.rowNeed = append(g.rowNeed, need+2*v.TextInsetY)
	}

	// 凡例: 1列目のセルの左端から並べ、幅を越えたら次の行へ。
	legend := s.legend()
	avail := g.inner - g.headW - v.CellGap
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
	g.legendH = float64(len(g.legendRows)) * tx.Small * lineHeightMult

	head := math.Max(g.headerNeed, v.HeaderH)
	g.parts = []flexPart{fixedPart(v.Padding), {g.headerNeed, head, head}}
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
		Min:     Size{Width: v.MinWidth, Height: min},
		Max:     Size{Height: max},
	}
}

type matrixPlaced struct {
	g       *matrixGeom
	header  []LBox
	heads   []LBox
	cells   [][]LBox
	legendY float64
	x0      float64 // 内側の左端
}

func (s *MatrixSubsystem) place(box LBox) *matrixPlaced {
	v := s.vals()
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	pl := &matrixPlaced{g: g, x0: box.X + v.Padding}
	colX := func(j int) float64 { return pl.x0 + g.headW + v.CellGap + float64(j)*(g.colW+v.CellGap) }
	y := box.Y + sizes[0]
	for j := range s.Data.Columns {
		pl.header = append(pl.header, LBox{X: colX(j), Y: y, Width: g.colW, Height: sizes[1]})
	}
	y += sizes[1]
	for i, r := range s.Data.Rows {
		y += sizes[2+2*i]
		h := sizes[3+2*i]
		pl.heads = append(pl.heads, LBox{X: pl.x0, Y: y, Width: g.headW, Height: h})
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
	header := func(b LBox, in Inline, align string) {
		p.AddShape(b, ShapeOpts{Prst: "rect", Runs: itemRuns(in, "text_primary", ctx.Roles), FontSize: tx.Title,
			InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: align})
	}
	for j, c := range s.Data.Columns {
		header(pl.header[j], c, "center")
	}
	for i, r := range s.Data.Rows {
		header(pl.heads[i], r.Label, "left")
		for j, c := range r.Cells {
			b := pl.cells[i][j]
			o := ShapeOpts{Prst: "roundRect", Fill: ctx.Roles.Color("card_alt"), Radius: v.CellCornerRadi, FontSize: tx.Sub,
				InsetX: v.TextInsetX + v.DotSize + v.DotGap, InsetY: v.TextInsetY, Align: "left"}
			dotX := b.X + v.TextInsetX
			if len(c.Text) > 0 {
				o.Runs = itemRuns(c.Text, "text_primary", ctx.Roles)
			} else {
				dotX = b.X + (b.Width-v.DotSize)/2
			}
			p.AddShape(b, o)
			p.AddDot(dotX+v.DotSize/2, b.Y+b.Height/2, v.DotSize/2, ctx.Roles.Color(c.Status))
		}
	}
	// 凡例(1列目のセルの左端から)。
	legend := s.legend()
	lineH := tx.Small * lineHeightMult
	for k, row := range pl.g.legendRows {
		x := pl.x0 + pl.g.headW + v.CellGap
		y := pl.legendY + float64(k)*lineH
		for _, i := range row {
			li := legend[i]
			p.AddDot(x+v.LegendDotSize/2, y+lineH/2, v.LegendDotSize/2, ctx.Roles.Color(li.Role))
			lx := x + v.LegendDotSize + v.DotGap/2
			w := runWidthPt(li.Label, tx.Small, false)
			p.AddTextBoxNoWrap(LBox{X: lx, Y: y, Width: w + 4, Height: lineH}, li.Label, tx.Small, ctx.Roles.Color("text_muted"), false, "left", "top")
			x += s.legendItemW(li) + v.LegendItemGap
		}
	}
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
		check(fmt.Sprintf("columns[%d]", j), c, tx.Title, b.Width-2*v.TextInsetX, b)
	}
	for i, r := range s.Data.Rows {
		b := pl.heads[i]
		check(fmt.Sprintf("rows[%d].label", i), r.Label, tx.Title, b.Width-2*v.TextInsetX, b)
		for j, c := range r.Cells {
			check(fmt.Sprintf("rows[%d].cells[%d]", i, j), c.Text, tx.Sub, s.cellTextWidth(pl.cells[i][j].Width), pl.cells[i][j])
		}
	}
	return out
}

// --- IR から読む ---

type matrixIR struct {
	Columns []Inline `json:"columns"`
	Rows    []struct {
		Label Inline `json:"label"`
		Cells []struct {
			Status string `json:"status"`
			Text   Inline `json:"text"`
		} `json:"cells"`
	} `json:"rows"`
	Legend orderedLegend `json:"legend"`
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
	d := &MatrixData{Columns: in.Columns}
	for j, c := range in.Columns {
		if len(c) == 0 {
			return nil, fmt.Errorf("matrix: columns[%d]: must not be empty", j)
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
	)
}
