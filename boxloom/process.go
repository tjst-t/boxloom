// process サブシステム: 日付の無い、方向のある段階の列(docs/patterns.md の R7)。
//
// 段階を左から右へ角丸の箱で並べ、本物のコネクタ(矢印)で繋ぐ。補足(detail)は
// 箱の下に、箱の左端に揃えて書く。幅が足りなくて段階が1行に並ばないときは、
// 次の行へ折り返す(行の終わりの箱の下から、次の行の始めの箱の上へ、曲がる
// コネクタで繋ぐ)。文は黙って削らない。折り返しても収まらない文は警告にする。
// 設計は docs/subsystem.md の11節。IR の data の形は
// schema/subsystems/process.schema.json。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
)

// ProcessValues は process の寸法(pt)。既定は subsystems/process.json。
type ProcessValues struct {
	Padding       float64 `json:"padding" scale:"space"`
	CaptionGap    float64 `json:"captionGap" scale:"space"` // 説明の文と箱の間
	ArrowGap      float64 `json:"arrowGap" scale:"space"`   // 箱と箱の間(矢印の長さ)の最小
	MinStepW      float64 `json:"minStepWidth"`             // これより狭くなるなら次の行へ折り返す
	MaxStepW      float64 `json:"maxStepWidth"`             // 箱の幅の上限(余りは矢印を長くする)
	MinBoxH       float64 `json:"minBoxHeight"`
	NaturalBoxH   float64 `json:"naturalBoxHeight"`
	MaxBoxH       float64 `json:"maxBoxHeight"`
	DetailGap     float64 `json:"detailGap" scale:"space"` // 箱と補足の間
	MinRowGap     float64 `json:"minRowGap" scale:"space"` // 折り返した行の間
	NaturalRowG   float64 `json:"naturalRowGap" scale:"space"`
	MaxRowGap     float64 `json:"maxRowGap" scale:"space"`
	NaturalWidth  float64 `json:"naturalWidth"`
	MinWidth      float64 `json:"minWidth"`
	TextInsetX    float64 `json:"textInsetX" scale:"space"`
	TextInsetY    float64 `json:"textInsetY" scale:"space"`
	LineWidth     float64 `json:"lineWidth"`
	OutlineWidth  float64 `json:"outlineWidth"`
	CornerRadius  float64 `json:"cornerRadius"`
	LegendGap     float64 `json:"legendGap" scale:"space"` // 図と凡例の間
	LegendItemGap float64 `json:"legendItemGap" scale:"space"`
	LegendSwatchW float64 `json:"legendSwatchWidth"` // 凡例の見本の箱
	LegendSwatchH float64 `json:"legendSwatchHeight"`
	LegendSwatchG float64 `json:"legendSwatchGap" scale:"space"` // 見本と文の間
}

func (v *ProcessValues) legendValues(radius float64) figureLegendValues {
	return figureLegendValues{gap: v.LegendGap, itemGap: v.LegendItemGap, swatchW: v.LegendSwatchW, swatchH: v.LegendSwatchH,
		swatchGap: v.LegendSwatchG, outlineW: v.OutlineWidth, radius: radius}
}

// ProcessData は IR の data を読んだもの。
type ProcessData struct {
	Caption  Inline
	Numbered bool // 題の前に「1. 」を付ける(既定 true)
	Steps    []FigureNode
	Legend   []LegendItem // 状態の凡例の文(書いた順)
}

// legend は凡例の項目と、使ったのに文の無い状態。
func (s *ProcessSubsystem) legend() ([]LegendItem, []string) {
	used := map[string]bool{}
	for _, st := range s.Data.Steps {
		used[st.Status] = true
	}
	return figureLegendItems(used, s.Data.Legend)
}

// ProcessSubsystem は ProcessData を描くサブシステム。
type ProcessSubsystem struct {
	Data *ProcessData
	V    *ProcessValues // nil なら既定(subsystems/process.json)
	Text *FigureText    // nil なら既定のテンプレート(light)の段
}

func NewProcessSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*ProcessData); ok {
		return &ProcessSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("process", data)
	if err != nil {
		return nil, err
	}
	d, err := processFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &ProcessSubsystem{Data: d}, nil
}

func (s *ProcessSubsystem) Name() string { return "process" }

func (s *ProcessSubsystem) vals() *ProcessValues {
	if s.V != nil {
		return s.V
	}
	return defaultProcessValues()
}

func (s *ProcessSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *ProcessSubsystem) boxValues() figureBoxValues {
	v := s.vals()
	return figureBoxValues{prst: "roundRect", radius: v.CornerRadius, outlineWidth: v.OutlineWidth, insetX: v.TextInsetX, insetY: v.TextInsetY}
}

// title は幅 boxW の箱に書く題(番号つき)。「1. 題」が1行に収まらず、題だけなら
// 収まるときは、番号の後ろで改行する(題が語の途中で折り返さないように)。
func (s *ProcessSubsystem) title(i int, boxW float64) Inline {
	t := s.Data.Steps[i].Title
	if !s.Data.Numbered {
		return t
	}
	num := Inline{{Text: fmt.Sprintf("%d. ", i+1)}}
	one := append(append(Inline{}, num...), t...)
	size, inner := s.text().Title, boxW-2*s.vals().TextInsetX
	if runsNaturalWidth(itemRuns(one, "", nil), size) <= inner || runsNaturalWidth(itemRuns(t, "", nil), size) > inner {
		return one
	}
	return append(Inline{{Text: fmt.Sprintf("%d.", i+1)}, {Type: "br"}}, t...)
}

// processGeom は幅だけで決まる置き方(高さは parts を flexSizes で配る)。
type processGeom struct {
	inner    float64 // 枠の内側の幅
	perRow   int     // 1行に並べる段階の数
	rows     int
	boxW     float64
	gap      float64 // 箱と箱の間
	captionH float64 // 説明の文の高さ(無ければ 0)
	needH    float64 // 箱の題が収まる高さ(全部の箱で同じ)
	detailH  []float64
	parts    []flexPart
	legend   figureLegendLayout
	// parts の中の場所
	boxPart, gapPart []int
}

func (s *ProcessSubsystem) geom(width float64) *processGeom {
	v, tx, fv := s.vals(), s.text(), s.boxValues()
	n := len(s.Data.Steps)
	g := &processGeom{inner: math.Max(0, width-2*v.Padding)}
	boxW := func(k int) float64 {
		w := (g.inner - float64(k-1)*v.ArrowGap) / float64(k)
		return math.Min(w, v.MaxStepW)
	}
	g.perRow = n
	for g.perRow > 1 && boxW(g.perRow) < v.MinStepW {
		g.perRow--
	}
	g.rows = (n + g.perRow - 1) / g.perRow
	// 折り返すときは行の長さを揃える(7段階を 6+1 ではなく 4+3 に)。
	g.perRow = (n + g.rows - 1) / g.rows
	g.boxW = math.Max(0, boxW(g.perRow))
	if g.perRow > 1 {
		g.gap = (g.inner - float64(g.perRow)*g.boxW) / float64(g.perRow-1)
	}
	if len(s.Data.Caption) > 0 {
		_, g.captionH = runsBox(itemRuns(s.Data.Caption, "", nil), tx.Detail, g.inner)
	}
	for i := range s.Data.Steps {
		n := FigureNode{Title: s.title(i, g.boxW)}
		_, h := nodeTextSize(&n, tx.Title, tx.Detail, g.boxW, fv)
		g.needH = math.Max(g.needH, h)
	}
	g.detailH = make([]float64, g.rows)
	for i, st := range s.Data.Steps {
		if len(st.Detail) == 0 {
			continue
		}
		_, h := runsBox(itemRuns(st.Detail, "", nil), tx.Detail, g.boxW)
		r := i / g.perRow
		g.detailH[r] = math.Max(g.detailH[r], h)
	}

	g.parts = []flexPart{fixedPart(v.Padding)}
	if g.captionH > 0 {
		g.parts = append(g.parts, fixedPart(g.captionH+v.CaptionGap))
	}
	box := flexPart{math.Max(g.needH, v.MinBoxH), math.Max(g.needH, v.NaturalBoxH), math.Max(g.needH, v.MaxBoxH)}
	for r := 0; r < g.rows; r++ {
		if r > 0 {
			g.gapPart = append(g.gapPart, len(g.parts))
			g.parts = append(g.parts, flexPart{v.MinRowGap, v.NaturalRowG, v.MaxRowGap})
		}
		g.boxPart = append(g.boxPart, len(g.parts))
		g.parts = append(g.parts, box)
		if g.detailH[r] > 0 {
			g.parts = append(g.parts, fixedPart(v.DetailGap+g.detailH[r]))
		}
	}
	items, _ := s.legend()
	g.legend = layoutFigureLegend(items, g.inner, tx.Small, v.legendValues(v.CornerRadius/2))
	g.parts = append(g.parts, g.legend.part(v.legendValues(0))...)
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

// Measure: 幅は分かっていればそれ(無ければ naturalWidth)。高さは、箱の高さ・
// 行の間の最小・自然・上限と、説明・補足の高さ(幅で折り返した高さ)の和。
func (s *ProcessSubsystem) Measure(c Constraint) Measurement {
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

// processPlaced は描く場所。
type processPlaced struct {
	boxes   []LBox
	details []LBox
	rowGapY []float64 // 行 r と r+1 の間の真ん中の y
	caption LBox
	legend  LBox // 凡例の左上(幅は内側の幅)
}

func (s *ProcessSubsystem) place(box LBox) (*processGeom, *processPlaced) {
	v := s.vals()
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	x0 := box.X + v.Padding
	y := box.Y + v.Padding
	pl := &processPlaced{}
	if g.captionH > 0 {
		pl.caption = LBox{X: x0, Y: y, Width: g.inner, Height: g.captionH}
		y += g.captionH + v.CaptionGap
	}
	for r := 0; r < g.rows; r++ {
		if r > 0 {
			gh := sizes[g.gapPart[r-1]]
			pl.rowGapY = append(pl.rowGapY, y+gh/2)
			y += gh
		}
		bh := sizes[g.boxPart[r]]
		for k := 0; k < g.perRow; k++ {
			i := r*g.perRow + k
			if i >= len(s.Data.Steps) {
				break
			}
			x := x0 + float64(k)*(g.boxW+g.gap)
			pl.boxes = append(pl.boxes, LBox{X: x, Y: y, Width: g.boxW, Height: bh})
			pl.details = append(pl.details, LBox{X: x, Y: y + bh + v.DetailGap, Width: g.boxW, Height: g.detailH[r]})
		}
		y += bh
		if g.detailH[r] > 0 {
			y += v.DetailGap + g.detailH[r]
		}
	}
	pl.legend = LBox{X: x0, Y: y + v.LegendGap, Width: g.inner, Height: g.legend.h}
	return g, pl
}

func (s *ProcessSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx, fv := s.vals(), s.text(), s.boxValues()
	g, pl := s.place(box)
	p := ctx.Out
	if g.captionH > 0 {
		p.AddRichTextBox(pl.caption, itemRuns(s.Data.Caption, "text_muted", ctx.Roles), tx.Detail, "left", "top")
	}
	ids := make([]int, len(s.Data.Steps))
	for i := range s.Data.Steps {
		n := s.Data.Steps[i]
		n.Title, n.Detail = s.title(i, pl.boxes[i].Width), nil
		ids[i] = drawFigureNode(ctx, pl.boxes[i], &n, tx.Title, tx.Detail, fv)
	}
	for i, st := range s.Data.Steps {
		if len(st.Detail) > 0 {
			p.AddRichTextBox(pl.details[i], itemRuns(st.Detail, "text_muted", ctx.Roles), tx.Detail, "left", "top")
		}
	}
	drawFigureLegend(ctx, pl.legend.X, pl.legend.Y, g.legend, tx.Small, v.legendValues(v.CornerRadius/2))
	color := ctx.Roles.Color("text_muted")
	for i := 0; i+1 < len(s.Data.Steps); i++ {
		a, b := pl.boxes[i], pl.boxes[i+1]
		if (i+1)%g.perRow != 0 {
			p.AddConnector(a.X+a.Width, a.Y+a.Height/2, b.X, b.Y+b.Height/2, v.LineWidth, color, true,
				ConnectorEnd{ids[i], 3}, ConnectorEnd{ids[i+1], 1})
			continue
		}
		// 行の終わり → 次の行の始め: 箱の下から出て、行の間で曲がり、箱の上に入る。
		x1, y1 := a.X+a.Width/2, a.Y+a.Height
		x2, y2 := b.X+b.Width/2, b.Y
		bend := pl.rowGapY[i/g.perRow]
		adj := 0.5
		if y2 > y1 {
			adj = (bend - y1) / (y2 - y1)
		}
		p.AddElbowConnector(x1, y1, x2, y2, v.LineWidth, color, true, ConnectorEnd{ids[i], 2}, ConnectorEnd{ids[i+1], 0}, adj)
	}
}

// Lint は箱の題・補足が収まらないこと(一番長い語が箱より広い、箱の高さが
// 足りない)を警告にする。
func (s *ProcessSubsystem) Lint(box LBox) []string {
	tx, fv := s.text(), s.boxValues()
	_, pl := s.place(box)
	var out []string
	for i, st := range s.Data.Steps {
		b := pl.boxes[i]
		n := FigureNode{Title: s.title(i, b.Width)}
		needW := nodeMinWidth(&n, tx.Title, tx.Detail, fv)
		_, needH := nodeTextSize(&n, tx.Title, tx.Detail, b.Width, fv)
		if !textFits(needW, needH, b.Width, b.Height) {
			out = append(out, overflowWarning("process", fmt.Sprintf("steps[%d]", i), st.Title, needW, needH, b.Width, b.Height))
		}
		if len(st.Detail) > 0 {
			d := pl.details[i]
			needW := runsMinWidth(itemRuns(st.Detail, "", nil), tx.Detail)
			if !textFits(needW, 0, d.Width, 0) {
				out = append(out, overflowWarning("process", fmt.Sprintf("steps[%d].detail", i), st.Detail, needW, d.Height, d.Width, d.Height))
			}
		}
	}
	_, missing := s.legend()
	return append(out, legendMissingWarnings("process", missing)...)
}

// --- IR から読む ---

type processIR struct {
	Caption  Inline         `json:"caption"`
	Numbered *bool          `json:"numbered"`
	Steps    []figureNodeIR `json:"steps"`
	Legend   orderedLegend  `json:"legend"`
}

func processFromIR(raw json.RawMessage) (*ProcessData, error) {
	var in processIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("process: %w", err)
	}
	if len(in.Steps) < 2 {
		return nil, fmt.Errorf("process: steps needs at least two steps")
	}
	d := &ProcessData{Caption: in.Caption, Numbered: in.Numbered == nil || *in.Numbered}
	for i, st := range in.Steps {
		n, err := st.node(fmt.Sprintf("steps[%d]", i))
		if err != nil {
			return nil, fmt.Errorf("process: %w", err)
		}
		d.Steps = append(d.Steps, n)
	}
	lg, err := figureLegendIR("process", in.Legend)
	if err != nil {
		return nil, err
	}
	d.Legend = lg
	return d, nil
}

func processDecl() SubsystemDecl {
	return figureDecl("subsystems/process.schema.json",
		"日付の無い、方向のある段階の列(3〜7段階)。段階は左から右へ並び、矢印で繋がる。幅が足りなければ次の行へ折り返す",
		"題の前の番号(1. 2. …)はサブシステムが付ける。付けないときは numbered: false",
		"status を使ったら legend: {conditional: 意味, …} で枠線の色の意味を書く(凡例が図の下に出る。無ければ figure-legend の警告)",
	)
}
