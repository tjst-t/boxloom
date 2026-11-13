// phases サブシステム: 段(フェーズ)ごとに箱を積む事業の全体像(docs/patterns.md の R7a)。
//
// 段(2〜5)を山形(左端は五角形の矢印、2つ目からは山形)で左から右へ並べ、各段の
// 下に番号つきの箱(題と、補足の文か箇条)を縦に積む。番号は段をまたいで通しで、
// 形(算用数字かローマ数字か)は値の表の numberStyle で選ぶ。段をまたぐ群の帯
// (groups)は山形の上に置く。余った高さは箱に配り、どの段の箱の列も下の端が
// そろう。設計は docs/subsystem.md の11節。IR の data の形は
// schema/subsystems/phases.schema.json。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// PhasesValues は phases の寸法(pt)と番号の形。既定は subsystems/phases.json。
type PhasesValues struct {
	Padding          float64 `json:"padding" scale:"space"`
	ColumnGap        float64 `json:"columnGap" scale:"space"` // 段の箱の列の間
	BandGap          float64 `json:"bandGap" scale:"space"`   // 群の帯と山形の間
	BandInsetX       float64 `json:"bandInsetX" scale:"space"`
	BandInsetY       float64 `json:"bandInsetY" scale:"space"`
	BandOutlineWidth float64 `json:"bandOutlineWidth"`
	ChevronDepth     float64 `json:"chevronDepth"`             // 山形の先の深さ
	ChevronGap       float64 `json:"chevronGap" scale:"space"` // 山形と山形の間
	HeaderInsetX     float64 `json:"headerInsetX" scale:"space"`
	HeaderInsetY     float64 `json:"headerInsetY" scale:"space"`
	MinHeaderH       float64 `json:"minHeaderHeight"`
	HeaderGap        float64 `json:"headerGap" scale:"space"` // 山形と箱の間
	ItemGap          float64 `json:"itemGap" scale:"space"`   // 箱と箱の間
	NaturalItemH     float64 `json:"naturalItemHeight"`
	MaxItemH         float64 `json:"maxItemHeight"`
	TextInsetX       float64 `json:"textInsetX" scale:"space"`
	TextInsetY       float64 `json:"textInsetY" scale:"space"`
	OutlineWidth     float64 `json:"outlineWidth"`
	CornerRadius     float64 `json:"cornerRadius"`
	NaturalWidth     float64 `json:"naturalWidth"`
	MinWidth         float64 `json:"minWidth"`
	LegendGap        float64 `json:"legendGap" scale:"space"`
	LegendItemGap    float64 `json:"legendItemGap" scale:"space"`
	LegendSwatchW    float64 `json:"legendSwatchWidth"`
	LegendSwatchH    float64 `json:"legendSwatchHeight"`
	LegendSwatchG    float64 `json:"legendSwatchGap" scale:"space"`
	// NumberStyle は箱の番号の形: "arabic"(1. 2. …)か "roman"(I. II. …)。
	NumberStyle string `json:"numberStyle"`
	// BandStyle は群の帯の見た目: "outline"(基調色の枠線と基調色の字)か
	// "fill"(基調色の塗りに on_conditional の字)。
	BandStyle string `json:"bandStyle"`
}

// phasesBandStyles は bandStyle に書ける見た目。
var phasesBandStyles = []string{"outline", "fill"}

// phasesNumberStyles は numberStyle に書ける形。
var phasesNumberStyles = []string{"arabic", "roman"}

func (v *PhasesValues) check(at string) error {
	if !contains(phasesNumberStyles, v.NumberStyle) {
		return fmt.Errorf("%s.numberStyle: unknown style %q (known: %s)", at, v.NumberStyle, strings.Join(phasesNumberStyles, ", "))
	}
	if !contains(phasesBandStyles, v.BandStyle) {
		return fmt.Errorf("%s.bandStyle: unknown style %q (known: %s)", at, v.BandStyle, strings.Join(phasesBandStyles, ", "))
	}
	return nil
}

func (v *PhasesValues) legendValues() figureLegendValues {
	return figureLegendValues{gap: v.LegendGap, itemGap: v.LegendItemGap, swatchW: v.LegendSwatchW, swatchH: v.LegendSwatchH,
		swatchGap: v.LegendSwatchG, outlineW: v.OutlineWidth, radius: v.CornerRadius / 2}
}

// phasesNumber は形 style の n 番目(1 から)の番号。
func phasesNumber(style string, n int) string {
	if style == "roman" {
		return strings.ToUpper(romanLower(n)) + "."
	}
	return strconv.Itoa(n) + "."
}

// PhaseItem は段の中の1つの箱。Detail は1つの文(Points が nil)か、箇条(Points)。
type PhaseItem struct {
	Title    Inline
	Detail   Inline
	Points   []Inline
	Status   string
	Emphasis bool
}

// PhaseStep は1つの段。
type PhaseStep struct {
	ID     string
	Title  Inline
	Period Inline
	Items  []PhaseItem
}

// PhaseGroup は段 From〜To(段の番号。0 から)をまたぐ群の帯。
type PhaseGroup struct {
	Title    Inline
	From, To int
}

// PhasesData は IR の data を読んだもの。
type PhasesData struct {
	Steps  []PhaseStep
	Groups []PhaseGroup
	Legend []LegendItem
}

// PhasesSubsystem は PhasesData を描くサブシステム。
type PhasesSubsystem struct {
	Data *PhasesData
	V    *PhasesValues // nil なら既定(subsystems/phases.json)
	Text *FigureText   // nil なら既定のテンプレート(light)の段
}

func NewPhasesSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*PhasesData); ok {
		return &PhasesSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("phases", data)
	if err != nil {
		return nil, err
	}
	d, err := phasesFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &PhasesSubsystem{Data: d}, nil
}

func (s *PhasesSubsystem) Name() string { return "phases" }

func (s *PhasesSubsystem) vals() *PhasesValues {
	if s.V != nil {
		return s.V
	}
	return defaultPhasesValues()
}

func (s *PhasesSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *PhasesSubsystem) legend() ([]LegendItem, []string) {
	used := map[string]bool{}
	for _, st := range s.Data.Steps {
		for _, it := range st.Items {
			used[it.Status] = true
		}
	}
	return figureLegendItems(used, s.Data.Legend)
}

// pointMark は箇条の印(テンプレートの marks.point。無ければ「・」)。
func (s *PhasesSubsystem) pointMark() string {
	if m := s.text().Point; m != "" {
		return m
	}
	return "・"
}

// itemTitleRuns は箱の題(通しの番号 n つき。太字)。
func (s *PhasesSubsystem) itemTitleRuns(n int, it *PhaseItem, numColor, color string, roles Roles) []TextRun {
	c := func(role string) string {
		if roles == nil {
			return ""
		}
		return roles.Color(role)
	}
	runs := []TextRun{{Text: phasesNumber(s.vals().NumberStyle, n) + " ", Color: c(numColor), Bold: true}}
	for _, r := range itemRuns(it.Title, color, roles) {
		r.Bold = true
		runs = append(runs, r)
	}
	return runs
}

// itemDetailParas は箱の補足の段落(箇条なら頭に印の塊)。
func (s *PhasesSubsystem) itemDetailParas(it *PhaseItem, markColor, color string, roles Roles) [][]TextRun {
	if len(it.Points) == 0 {
		if len(it.Detail) == 0 {
			return nil
		}
		return [][]TextRun{itemRuns(it.Detail, color, roles)}
	}
	mc := ""
	if roles != nil {
		mc = roles.Color(markColor)
	}
	var out [][]TextRun
	for _, p := range it.Points {
		out = append(out, append([]TextRun{{Text: s.pointMark(), Color: mc, Bold: true, Mark: true}}, itemRuns(p, color, roles)...))
	}
	return out
}

// itemNeed は幅 w の箱に文(題と補足)が収まる幅(一番長い語)と高さ。
func (s *PhasesSubsystem) itemNeed(n int, it *PhaseItem, w float64) (needW, needH float64) {
	v, tx := s.vals(), s.text()
	inner := w - 2*v.TextInsetX
	title := s.itemTitleRuns(n, it, "", "", nil)
	_, h := runsBox(title, tx.Title, inner)
	needW = runsMinWidth(title, tx.Title)
	markW := 0.0
	if len(it.Points) > 0 {
		markW = runWidthPt(s.pointMark(), tx.Detail, true)
	}
	for _, p := range s.itemDetailParas(it, "", "", nil) {
		if markW > 0 {
			p = p[1:] // 印の分は字下げとして幅から引く(2行目からも同じ字下げ)
		}
		_, dh := runsBox(p, tx.Detail, inner-markW)
		h += dh
		needW = math.Max(needW, runsMinWidth(p, tx.Detail)+markW)
	}
	return needW + 2*v.TextInsetX, h + 2*v.TextInsetY
}

// headerRuns は山形の文(段の題。期間は2つ目の段落)。
func headerParas(st *PhaseStep, color string, roles Roles) (title []TextRun, period []TextRun) {
	for _, r := range itemRuns(st.Title, color, roles) {
		r.Bold = true
		title = append(title, r)
	}
	if len(st.Period) > 0 {
		period = itemRuns(st.Period, color, roles)
	}
	return title, period
}

// chevronTextWidth は山形(prst)の文字の枠の幅(OOXML の preset の il〜ir)。
func chevronTextWidth(prst string, w, depth float64) float64 {
	switch prst {
	case "homePlate":
		return w - depth/2
	case "chevron":
		return w - 2*depth
	}
	return w
}

type phasesGeom struct {
	inner   float64
	colW    float64
	colX    []float64 // 内側の左端から
	bandH   float64   // 群の帯の高さ(群が無ければ 0)
	headH   float64
	need    [][]float64 // 段ごと・箱ごとの、文が収まる高さ
	col     []flexPart  // 段ごとの箱の列(最小・自然・上限)
	legend  figureLegendLayout
	parts   []flexPart
	bodyIdx int // parts の中の箱の列の場所
}

// chevronBox は段 i の山形の箱(内側の左端・山形の行の上端から)。山形の先は次の段の
// 列に食い込み、次の山形の切り欠きに chevronGap をあけて収まる。
func (g *phasesGeom) chevronBox(i, n int, v *PhasesValues) (x, w float64, prst string) {
	x, w = g.colX[i], g.colW
	if i+1 < n {
		w = g.colX[i+1] - g.colX[i] + v.ChevronDepth - v.ChevronGap
	}
	prst = "chevron"
	if i == 0 {
		prst = "homePlate"
	}
	return x, w, prst
}

func (s *PhasesSubsystem) geom(width float64) *phasesGeom {
	v, tx := s.vals(), s.text()
	n := len(s.Data.Steps)
	g := &phasesGeom{inner: math.Max(0, width-2*v.Padding)}
	g.colW = math.Max(0, (g.inner-float64(n-1)*v.ColumnGap)/float64(n))
	for i := 0; i < n; i++ {
		g.colX = append(g.colX, float64(i)*(g.colW+v.ColumnGap))
	}
	for _, gr := range s.Data.Groups {
		w := g.colX[gr.To] + g.colW - g.colX[gr.From] - 2*v.BandInsetX
		_, h := runsBox(boldRuns(itemRuns(gr.Title, "", nil)), tx.Sub, w)
		g.bandH = math.Max(g.bandH, h+2*v.BandInsetY)
	}
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		_, w, prst := g.chevronBox(i, n, v)
		tw := chevronTextWidth(prst, w, v.ChevronDepth) - 2*v.HeaderInsetX
		title, period := headerParas(st, "", nil)
		_, h := runsBox(title, tx.Title, tw)
		if len(period) > 0 {
			_, ph := runsBox(period, tx.Detail, tw)
			h += ph
		}
		g.headH = math.Max(g.headH, h+2*v.HeaderInsetY)
	}
	g.headH = math.Max(g.headH, v.MinHeaderH)
	num := 0
	var body flexPart
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		var need []float64
		gaps := float64(len(st.Items)-1) * v.ItemGap
		c := flexPart{gaps, gaps, gaps}
		for k := range st.Items {
			num++
			_, h := s.itemNeed(num, &st.Items[k], g.colW)
			need = append(need, h)
			c.min += h
			c.nat += math.Max(h, v.NaturalItemH)
			c.max += math.Max(h, v.MaxItemH)
		}
		g.need = append(g.need, need)
		g.col = append(g.col, c)
		body.min, body.nat, body.max = math.Max(body.min, c.min), math.Max(body.nat, c.nat), math.Max(body.max, c.max)
	}
	g.parts = []flexPart{fixedPart(v.Padding)}
	if g.bandH > 0 {
		g.parts = append(g.parts, fixedPart(g.bandH+v.BandGap))
	}
	g.parts = append(g.parts, fixedPart(g.headH+v.HeaderGap))
	g.bodyIdx = len(g.parts)
	g.parts = append(g.parts, body)
	items, _ := s.legend()
	g.legend = layoutFigureLegend(items, g.inner, tx.Small, v.legendValues())
	g.parts = append(g.parts, g.legend.part(v.legendValues())...)
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

func boldRuns(runs []TextRun) []TextRun {
	for i := range runs {
		runs[i].Bold = true
	}
	return runs
}

// Measure: 幅は分かっていればそれ(無ければ naturalWidth)。高さは、群の帯・山形・
// 一番高い段の箱の列(箱の文が収まる高さの和が最小、naturalItemHeight・
// maxItemHeight まで広げた和が自然・上限)・凡例の和。
func (s *PhasesSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	w := v.NaturalWidth
	if c.Width > 0 {
		w = c.Width
	}
	min, nat, max := flexTotals(s.geom(w).parts)
	return Measurement{
		Natural: Size{Width: w, Height: nat},
		Min:     Size{Width: math.Max(v.MinWidth, s.minWidth()), Height: min},
		Max:     Size{Height: max},
	}
}

// minWidth は、どの箱も山形も一番長い語を割らずに置ける図の幅:段の列の幅を
// 一番広く要る箱(山形は文字の枠の両端の欠けを足した幅)にそろえた幅。
func (s *PhasesSubsystem) minWidth() float64 {
	v, tx := s.vals(), s.text()
	n := len(s.Data.Steps)
	col := 0.0
	num := 0
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		title, period := headerParas(st, "", nil)
		hw := runsMinWidth(title, tx.Title)
		if len(period) > 0 {
			hw = math.Max(hw, runsMinWidth(period, tx.Detail))
		}
		// 山形の文字の枠は、最後の段では列の幅から先と切り欠きの分(2つの深さ)を引いた幅。
		col = math.Max(col, hw+2*v.HeaderInsetX+2*v.ChevronDepth)
		for k := range st.Items {
			num++
			w, _ := s.itemNeed(num, &st.Items[k], 0)
			col = math.Max(col, w)
		}
	}
	return float64(n)*col + float64(n-1)*v.ColumnGap + 2*v.Padding
}

type phasesPlaced struct {
	bands    []LBox
	chevrons []LBox
	prsts    []string
	items    [][]LBox
	legend   LBox
}

// columnSizes は高さ b を段の箱に配る。上限を越えた分も箱に等しく配り(下の端を
// そろえる)。箱は文の要る高さより低くしない(b が最小に足りなくても、#107)。
func columnSizes(need []float64, c flexPart, b float64, v *PhasesValues) []float64 {
	k := len(need)
	gaps := float64(k-1) * v.ItemGap
	out := make([]float64, k)
	if b < c.min {
		copy(out, need)
		return out
	}
	parts := make([]flexPart, k)
	for i, h := range need {
		parts[i] = flexPart{h, math.Max(h, v.NaturalItemH), math.Max(h, v.MaxItemH)}
	}
	sizes := flexSizes(parts, b-gaps)
	sum := 0.0
	for _, h := range sizes {
		sum += h
	}
	for i, h := range sizes {
		out[i] = h + math.Max(0, b-gaps-sum)/float64(k)
	}
	return out
}

func (s *PhasesSubsystem) place(box LBox) (*phasesGeom, *phasesPlaced) {
	v := s.vals()
	g := s.geom(box.Width)
	// 枠が最小より低くても箱は縮めない(縮めると箱の文が次の箱に重なる、#107)。図は
	// 最小の高さで描き、枠の下にはみ出す分は layout-overflow・subsystem-min で知らせる。
	min, _, _ := flexTotals(g.parts)
	sizes := flexSizes(g.parts, math.Max(box.Height, min))
	x0 := box.X + v.Padding
	y := box.Y + v.Padding
	pl := &phasesPlaced{}
	n := len(s.Data.Steps)
	if g.bandH > 0 {
		for _, gr := range s.Data.Groups {
			x := x0 + g.colX[gr.From]
			pl.bands = append(pl.bands, LBox{X: x, Y: y, Width: x0 + g.colX[gr.To] + g.colW - x, Height: g.bandH})
		}
		y += g.bandH + v.BandGap
	}
	for i := 0; i < n; i++ {
		x, w, prst := g.chevronBox(i, n, v)
		pl.chevrons = append(pl.chevrons, LBox{X: x0 + x, Y: y, Width: w, Height: g.headH})
		pl.prsts = append(pl.prsts, prst)
	}
	y += g.headH + v.HeaderGap
	b := sizes[g.bodyIdx]
	for i := range s.Data.Steps {
		hs := columnSizes(g.need[i], g.col[i], b, v)
		var col []LBox
		yy := y
		for _, h := range hs {
			col = append(col, LBox{X: x0 + g.colX[i], Y: yy, Width: g.colW, Height: h})
			yy += h + v.ItemGap
		}
		pl.items = append(pl.items, col)
	}
	pl.legend = LBox{X: x0, Y: y + b + v.LegendGap, Width: g.inner, Height: g.legend.h}
	return g, pl
}

func (s *PhasesSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	g, pl := s.place(box)
	p := ctx.Out
	for i, gr := range s.Data.Groups {
		o := ShapeOpts{Prst: "rect", Outline: ctx.Roles.Color("accent"), OutlineWidthPt: v.BandOutlineWidth,
			Runs: boldRuns(itemRuns(gr.Title, "accent", ctx.Roles)), FontSize: tx.Sub,
			InsetX: v.BandInsetX, InsetY: v.BandInsetY, Align: "left", Anchor: "middle"}
		if v.BandStyle == "fill" {
			o.Outline, o.OutlineWidthPt = "", 0
			o.Fill = ctx.Roles.Color("accent")
			o.Runs = boldRuns(itemRuns(gr.Title, "on_conditional", ctx.Roles))
		}
		p.AddShape(pl.bands[i], o)
	}
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		title, period := headerParas(st, "on_conditional", ctx.Roles)
		o := ShapeOpts{Prst: pl.prsts[i], Fill: ctx.Roles.Color("accent"), Runs: title, FontSize: tx.Title,
			InsetX: v.HeaderInsetX, InsetY: v.HeaderInsetY, Align: "left", Anchor: "middle", Depth: v.ChevronDepth}
		if len(period) > 0 {
			o.Paras, o.ParaSizes = [][]TextRun{period}, []float64{tx.Detail}
		}
		p.AddShape(pl.chevrons[i], o)
	}
	num := 0
	for i := range s.Data.Steps {
		for k := range s.Data.Steps[i].Items {
			num++
			it := &s.Data.Steps[i].Items[k]
			fill, outline, text := figureStyle(it.Status, it.Emphasis)
			numColor, detailColor, markColor := "accent", "text_primary", "accent"
			if it.Emphasis {
				numColor, detailColor, markColor = text, text, text
			}
			o := ShapeOpts{Prst: "roundRect", Fill: ctx.Roles.Color(fill), OutlineWidthPt: v.OutlineWidth, Radius: v.CornerRadius,
				Runs: s.itemTitleRuns(num, it, numColor, text, ctx.Roles), FontSize: tx.Title,
				InsetX: v.TextInsetX, InsetY: v.TextInsetY, Align: "left", Anchor: "top"}
			if outline != "" {
				o.Outline = ctx.Roles.Color(outline)
			}
			for _, para := range s.itemDetailParas(it, markColor, detailColor, ctx.Roles) {
				o.Paras = append(o.Paras, para)
				o.ParaSizes = append(o.ParaSizes, tx.Detail)
			}
			p.AddShape(pl.items[i][k], o)
		}
	}
	drawFigureLegend(ctx, pl.legend.X, pl.legend.Y, g.legend, tx.Small, v.legendValues())
}

// Lint は箱・山形・群の帯の文が収まらないことと、状態の凡例の文が無いことを
// 警告にする。
func (s *PhasesSubsystem) Lint(box LBox) []string {
	v, tx := s.vals(), s.text()
	_, pl := s.place(box)
	var out []string
	for i := range s.Data.Groups {
		gr := &s.Data.Groups[i]
		b := pl.bands[i]
		runs := boldRuns(itemRuns(gr.Title, "", nil))
		needW := runsMinWidth(runs, tx.Sub) + 2*v.BandInsetX
		_, h := runsBox(runs, tx.Sub, b.Width-2*v.BandInsetX)
		if !textFits(needW, h+2*v.BandInsetY, b.Width, b.Height) {
			out = append(out, overflowWarning("phases", fmt.Sprintf("groups[%d]", i), gr.Title, needW, h+2*v.BandInsetY, b.Width, b.Height))
		}
	}
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		b := pl.chevrons[i]
		tw := chevronTextWidth(pl.prsts[i], b.Width, v.ChevronDepth)
		title, period := headerParas(st, "", nil)
		needW := runsMinWidth(title, tx.Title)
		_, h := runsBox(title, tx.Title, tw-2*v.HeaderInsetX)
		if len(period) > 0 {
			needW = math.Max(needW, runsMinWidth(period, tx.Detail))
			_, ph := runsBox(period, tx.Detail, tw-2*v.HeaderInsetX)
			h += ph
		}
		needW += 2 * v.HeaderInsetX
		if !textFits(needW, h+2*v.HeaderInsetY, tw, b.Height) {
			out = append(out, overflowWarning("phases", fmt.Sprintf("steps[%d]", i), st.Title, needW, h+2*v.HeaderInsetY, tw, b.Height))
		}
	}
	num := 0
	for i := range s.Data.Steps {
		for k := range s.Data.Steps[i].Items {
			num++
			it := &s.Data.Steps[i].Items[k]
			b := pl.items[i][k]
			needW, needH := s.itemNeed(num, it, b.Width)
			if !textFits(needW, needH, b.Width, b.Height) {
				out = append(out, overflowWarning("phases", fmt.Sprintf("steps[%d].items[%d]", i, k), it.Title, needW, needH, b.Width, b.Height))
			}
		}
	}
	_, missing := s.legend()
	return append(out, legendMissingWarnings("phases", missing)...)
}

// --- IR から読む ---

type phasesIR struct {
	Steps []struct {
		ID     string `json:"id"`
		Title  Inline `json:"title"`
		Period Inline `json:"period"`
		Items  []struct {
			Title    Inline          `json:"title"`
			Detail   json.RawMessage `json:"detail"`
			Status   string          `json:"status"`
			Emphasis bool            `json:"emphasis"`
		} `json:"items"`
	} `json:"steps"`
	Groups []struct {
		Title Inline `json:"title"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"groups"`
	Legend orderedLegend `json:"legend"`
}

// phasesMinSteps・phasesMaxSteps は段の数の範囲。
const (
	phasesMinSteps = 2
	phasesMaxSteps = 5
)

func phasesFromIR(raw json.RawMessage) (*PhasesData, error) {
	var in phasesIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("phases: %w", err)
	}
	if len(in.Steps) < phasesMinSteps || len(in.Steps) > phasesMaxSteps {
		return nil, fmt.Errorf("phases: steps needs %d to %d steps (got %d)", phasesMinSteps, phasesMaxSteps, len(in.Steps))
	}
	d := &PhasesData{}
	ids := map[string]int{}
	for i, st := range in.Steps {
		at := fmt.Sprintf("steps[%d]", i)
		if len(st.Title) == 0 {
			return nil, fmt.Errorf("phases: %s.title: must not be empty", at)
		}
		if st.ID != "" {
			if _, dup := ids[st.ID]; dup {
				return nil, fmt.Errorf("phases: %s.id: %q is used by another step", at, st.ID)
			}
			ids[st.ID] = i
		}
		if len(st.Items) == 0 {
			return nil, fmt.Errorf("phases: %s.items: a step needs at least one item", at)
		}
		ps := PhaseStep{ID: st.ID, Title: st.Title, Period: st.Period}
		for k, it := range st.Items {
			iat := fmt.Sprintf("%s.items[%d]", at, k)
			if len(it.Title) == 0 {
				return nil, fmt.Errorf("phases: %s.title: must not be empty", iat)
			}
			if err := checkFigureStatus(iat, it.Status); err != nil {
				return nil, fmt.Errorf("phases: %w", err)
			}
			pi := PhaseItem{Title: it.Title, Status: it.Status, Emphasis: it.Emphasis}
			if det := bytes.TrimSpace(it.Detail); isSentenceList(det) {
				if err := json.Unmarshal(det, &pi.Points); err != nil {
					return nil, fmt.Errorf("phases: %s.detail: %w", iat, err)
				}
				if len(pi.Points) == 0 {
					return nil, fmt.Errorf("phases: %s.detail: a list needs at least one sentence", iat)
				}
			} else if len(det) > 0 {
				if err := json.Unmarshal(det, &pi.Detail); err != nil {
					return nil, fmt.Errorf("phases: %s.detail: %w", iat, err)
				}
			}
			ps.Items = append(ps.Items, pi)
		}
		d.Steps = append(d.Steps, ps)
	}
	for i, gr := range in.Groups {
		at := fmt.Sprintf("groups[%d]", i)
		if len(gr.Title) == 0 {
			return nil, fmt.Errorf("phases: %s.title: must not be empty", at)
		}
		from, ok := ids[gr.From]
		if !ok {
			return nil, fmt.Errorf("phases: %s.from: no step has the id %q", at, gr.From)
		}
		to, ok := ids[gr.To]
		if !ok {
			return nil, fmt.Errorf("phases: %s.to: no step has the id %q", at, gr.To)
		}
		if to < from {
			return nil, fmt.Errorf("phases: %s.to: step %q comes before %q", at, gr.To, gr.From)
		}
		if n := len(d.Groups); n > 0 && from <= d.Groups[n-1].To {
			return nil, fmt.Errorf("phases: %s.from: groups must not overlap and must be written left to right", at)
		}
		d.Groups = append(d.Groups, PhaseGroup{Title: gr.Title, From: from, To: to})
	}
	lg, err := figureLegendIR("phases", in.Legend)
	if err != nil {
		return nil, err
	}
	d.Legend = lg
	return d, nil
}

// isSentenceList は detail が文の列(箇条)か。列の項目に文中の要素(em・code・link
// などの object)が直に並ぶなら、それは1つの文(inline の列)。文字列と列だけなら
// 文の列(1つの文を文字列だけの列で書く意味は無いので、こう読み分ける)。
func isSentenceList(det []byte) bool {
	if len(det) == 0 || det[0] != '[' {
		return false
	}
	var raw []json.RawMessage
	if json.Unmarshal(det, &raw) != nil {
		return false
	}
	for _, r := range raw {
		if r = bytes.TrimSpace(r); len(r) > 0 && r[0] == '{' {
			return false
		}
	}
	return true
}

func phasesDecl() SubsystemDecl {
	return figureDecl("subsystems/phases.schema.json",
		"段(フェーズ。2〜5段)を山形で左から右へ並べ、各段の下に箱(題と補足)を縦に積む事業の全体像。段に箱を持たない手順は process",
		"箱の番号は段をまたいで通しでサブシステムが振る(形はテンプレートが決める)。題に番号を書かない",
		"detail は1つの文か、文の列(列は箇条になる)",
		"段をまたぐ群の帯は groups: [{title, from: 段の id, to: 段の id}](左から順に、重ねない)。帯は山形の上に出る",
		"status を使ったら legend: {conditional: 意味, …} で枠線の色の意味を書く(凡例が図の下に出る。無ければ figure-legend の警告)",
	)
}
