// funnel サブシステム: 段階ごとに数が減っていく流れ(docs/patterns.md の R10)。
//
// 段を上から下へ、だんだん狭くなる帯で積む。帯の幅は値に比例させず、一番上の
// 幅から一番下の幅(minWidthFraction)まで等しい刻みで狭める(値は帯の中に数で
// 書く。比例させると小さい段が読めない細さになり、文の幅で下駄を履かせると
// 比例が嘘になるため)。ただし帯は中の文が収まる幅より狭くしない。注記(note)は
// 帯の右に、その段の縦の真ん中に揃えて置く。設計は docs/subsystem.md の11節。
// IR の data の形は schema/subsystems/funnel.schema.json。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FunnelValues は funnel の寸法(pt。割合は内側の幅に対するもの)。既定は
// subsystems/funnel.json。
type FunnelValues struct {
	Padding       float64 `json:"padding" scale:"space"`
	NoteFrac      float64 `json:"noteFraction"` // 注記の列の幅(注記があるとき)
	NoteGap       float64 `json:"noteGap" scale:"space"`
	MinWidthFrac  float64 `json:"minWidthFraction"` // 一番下の帯の幅(帯の列の幅に対する割合)
	MinBarH       float64 `json:"minBarHeight"`
	NaturalBarH   float64 `json:"naturalBarHeight"`
	MaxBarH       float64 `json:"maxBarHeight"`
	MinGap        float64 `json:"minGap" scale:"space"` // 帯と帯の間
	NaturalGap    float64 `json:"naturalGap" scale:"space"`
	MaxGap        float64 `json:"maxGap" scale:"space"`
	NaturalWidth  float64 `json:"naturalWidth"`
	MinWidth      float64 `json:"minWidth"`
	TextInsetX    float64 `json:"textInsetX" scale:"space"`
	TextInsetY    float64 `json:"textInsetY" scale:"space"`
	OutlineWidth  float64 `json:"outlineWidth"`
	LegendGap     float64 `json:"legendGap" scale:"space"` // 図と凡例の間
	LegendItemGap float64 `json:"legendItemGap" scale:"space"`
	LegendSwatchW float64 `json:"legendSwatchWidth"` // 凡例の見本の箱
	LegendSwatchH float64 `json:"legendSwatchHeight"`
	LegendSwatchG float64 `json:"legendSwatchGap" scale:"space"` // 見本と文の間
	RateGap       float64 `json:"rateGap" scale:"space"`         // 帯の列と転換率の列の間
}

func (v *FunnelValues) legendValues() figureLegendValues {
	return figureLegendValues{gap: v.LegendGap, itemGap: v.LegendItemGap, swatchW: v.LegendSwatchW, swatchH: v.LegendSwatchH,
		swatchGap: v.LegendSwatchG, outlineW: v.OutlineWidth}
}

// FunnelStage は1つの段。
type FunnelStage struct {
	Label    Inline
	Value    string   // 書く文字(数なら unit を付けた形)
	Number   *float64 // 値が数なら(減っているかの検査に使う)
	Note     Inline
	Status   string
	Emphasis bool
}

// FunnelData は IR の data を読んだもの。
type FunnelData struct {
	Unit   string
	Stages []FunnelStage
	Legend []LegendItem // 状態の凡例の文(書いた順)
}

// legend は凡例の項目と、使ったのに文の無い状態。
func (s *FunnelSubsystem) legend() ([]LegendItem, []string) {
	used := map[string]bool{}
	for _, st := range s.Data.Stages {
		used[st.Status] = true
	}
	return figureLegendItems(used, s.Data.Legend)
}

// rates は段 i と i+1 の間の転換率(前の段に対する割合)の文。前後の値が数で、
// 前が 0 より大きく、後が前を越えないときだけ(それ以外は "")。作者の数から
// 一つに決まる値なので、図が書き足す。
func (s *FunnelSubsystem) rates() []string {
	n := len(s.Data.Stages)
	out := make([]string, max(n-1, 0))
	for i := 0; i+1 < n; i++ {
		a, b := s.Data.Stages[i].Number, s.Data.Stages[i+1].Number
		if a == nil || b == nil || *a <= 0 || *b < 0 || *b > *a {
			continue
		}
		out[i] = "↓ " + formatRate(*b / *a * 100) + "%"
	}
	return out
}

// formatRate は割合を書く(10 以上は整数、それより小さければ小数1桁。末尾の .0 は省く)。
func formatRate(p float64) string {
	if p >= 10 {
		return strconv.FormatFloat(math.Round(p), 'f', -1, 64)
	}
	return strconv.FormatFloat(math.Round(p*10)/10, 'f', -1, 64)
}

// FunnelSubsystem は FunnelData を描くサブシステム。
type FunnelSubsystem struct {
	Data *FunnelData
	V    *FunnelValues // nil なら既定(subsystems/funnel.json)
	Text *FigureText   // nil なら既定のテンプレート(light)の段
}

func NewFunnelSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*FunnelData); ok {
		return &FunnelSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("funnel", data)
	if err != nil {
		return nil, err
	}
	d, err := funnelFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &FunnelSubsystem{Data: d}, nil
}

func (s *FunnelSubsystem) Name() string { return "funnel" }

func (s *FunnelSubsystem) vals() *FunnelValues {
	if s.V != nil {
		return s.V
	}
	return defaultFunnelValues()
}

func (s *FunnelSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *FunnelSubsystem) hasNotes() bool {
	for _, st := range s.Data.Stages {
		if len(st.Note) > 0 {
			return true
		}
	}
	return false
}

// barRuns は帯の中の文(段の名前と、太字の値)。
func barRuns(st *FunnelStage, color string, roles Roles) []TextRun {
	runs := itemRuns(st.Label, color, roles)
	c := ""
	if roles != nil {
		c = roles.Color(color)
	}
	return append(runs, TextRun{Text: " " + st.Value, Color: c, Bold: true})
}

type funnelGeom struct {
	barArea  float64 // 帯の列の幅
	rateX    float64 // 転換率の列の左端(内側の左端から)。無ければ 0
	rateW    float64
	inner    float64
	legend   figureLegendLayout
	noteX    float64 // 注記の列の左端(内側の左端から)
	noteW    float64
	widths   []float64
	needH    float64 // 帯の文・注記が収まる高さ(全部の段で同じ)
	parts    []flexPart
	barParts []int
}

func (s *FunnelSubsystem) geom(width float64) *funnelGeom {
	v, tx := s.vals(), s.text()
	inner := math.Max(0, width-2*v.Padding)
	g := &funnelGeom{barArea: inner, inner: inner}
	if s.hasNotes() {
		g.noteW = inner * v.NoteFrac
		g.barArea = math.Max(0, inner-g.noteW-v.NoteGap)
		g.noteX = g.barArea + v.NoteGap
	}
	for _, r := range s.rates() {
		if r != "" {
			g.rateW = math.Max(g.rateW, runWidthPt(r, tx.Sub, false))
		}
	}
	if g.rateW > 0 {
		// 転換率の列は帯の列の右(注記の列の手前)に取る。
		g.barArea = math.Max(0, g.barArea-g.rateW-v.RateGap)
		g.rateX = g.barArea + v.RateGap
	}
	n := len(s.Data.Stages)
	g.widths = make([]float64, n)
	for i := range s.Data.Stages {
		f := 1.0
		if n > 1 {
			f = 1 - float64(i)*(1-v.MinWidthFrac)/float64(n-1)
		}
		need := runsNaturalWidth(barRuns(&s.Data.Stages[i], "", nil), tx.Title) + 2*v.TextInsetX
		g.widths[i] = math.Max(g.barArea*f, math.Min(need, g.barArea))
	}
	// 文に合わせて広げた帯が、上の帯より広くならないように(上から下へ狭まる)。
	for i := n - 2; i >= 0; i-- {
		g.widths[i] = math.Max(g.widths[i], g.widths[i+1])
	}
	for i := range s.Data.Stages {
		st := &s.Data.Stages[i]
		_, h := runsBox(barRuns(st, "", nil), tx.Title, g.widths[i]-2*v.TextInsetX)
		g.needH = math.Max(g.needH, h+2*v.TextInsetY)
		if len(st.Note) > 0 {
			_, h := runsBox(itemRuns(st.Note, "", nil), tx.Sub, g.noteW)
			g.needH = math.Max(g.needH, h)
		}
	}
	bar := flexPart{math.Max(g.needH, v.MinBarH), math.Max(g.needH, v.NaturalBarH), math.Max(g.needH, v.MaxBarH)}
	g.parts = []flexPart{fixedPart(v.Padding)}
	for i := range s.Data.Stages {
		if i > 0 {
			g.parts = append(g.parts, flexPart{v.MinGap, v.NaturalGap, v.MaxGap})
		}
		g.barParts = append(g.barParts, len(g.parts))
		g.parts = append(g.parts, bar)
	}
	items, _ := s.legend()
	g.legend = layoutFigureLegend(items, inner, tx.Small, v.legendValues())
	g.parts = append(g.parts, g.legend.part(v.legendValues())...)
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

// Measure: 高さは帯(文と注記が収まる高さ以上)と帯の間の、最小・自然・上限の和。
func (s *FunnelSubsystem) Measure(c Constraint) Measurement {
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

type funnelPlaced struct {
	bars, notes []LBox
	rates       []LBox // 段 i と i+1 の境目の高さの真ん中に置く文の枠
	legend      LBox
}

func (s *FunnelSubsystem) place(box LBox) (*funnelGeom, *funnelPlaced) {
	v := s.vals()
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	x0 := box.X + v.Padding
	pl := &funnelPlaced{}
	y := box.Y
	k := 0
	for i, h := range sizes {
		if k < len(g.barParts) && g.barParts[k] == i {
			w := g.widths[k]
			pl.bars = append(pl.bars, LBox{X: x0 + (g.barArea-w)/2, Y: y, Width: w, Height: h})
			pl.notes = append(pl.notes, LBox{X: x0 + g.noteX, Y: y, Width: g.noteW, Height: h})
			k++
		}
		y += h
	}
	lineH := s.text().Sub * lineHeightMult
	for i := 0; i+1 < len(pl.bars); i++ {
		mid := (pl.bars[i].Y + pl.bars[i].Height + pl.bars[i+1].Y) / 2
		pl.rates = append(pl.rates, LBox{X: x0 + g.rateX, Y: mid - lineH/2, Width: g.rateW + noWrapSlack, Height: lineH})
	}
	if g.legend.h > 0 {
		last := pl.bars[len(pl.bars)-1]
		pl.legend = LBox{X: x0, Y: last.Y + last.Height + v.LegendGap, Width: g.inner, Height: g.legend.h}
	}
	return g, pl
}

func (s *FunnelSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	g, pl := s.place(box)
	p := ctx.Out
	for i, r := range s.rates() {
		if r != "" {
			p.AddTextBoxNoWrap(pl.rates[i], r, tx.Sub, ctx.Roles.Color("text_muted"), true, "left", "middle")
		}
	}
	drawFigureLegend(ctx, pl.legend.X, pl.legend.Y, g.legend, tx.Small, v.legendValues())
	for i := range s.Data.Stages {
		st := &s.Data.Stages[i]
		fill, outline, text := figureStyle(st.Status, st.Emphasis)
		o := ShapeOpts{Prst: "rect", Fill: ctx.Roles.Color(fill), OutlineWidthPt: v.OutlineWidth,
			Runs: barRuns(st, text, ctx.Roles), FontSize: tx.Title, InsetX: v.TextInsetX, InsetY: v.TextInsetY}
		if outline != "" {
			o.Outline = ctx.Roles.Color(outline)
		}
		p.AddShape(pl.bars[i], o)
		if len(st.Note) > 0 {
			p.AddRichTextBox(pl.notes[i], itemRuns(st.Note, "text_muted", ctx.Roles), tx.Sub, "left", "middle")
		}
	}
}

// Lint は帯・注記の文が収まらないことと、数の値が前の段より増えていること
// (funnel は減っていく段階に使う。R10)を警告にする。
func (s *FunnelSubsystem) Lint(box LBox) []string {
	v, tx := s.vals(), s.text()
	_, pl := s.place(box)
	var out []string
	for i := range s.Data.Stages {
		st := &s.Data.Stages[i]
		b := pl.bars[i]
		runs := barRuns(st, "", nil)
		needW := runsMinWidth(runs, tx.Title) + 2*v.TextInsetX
		_, h := runsBox(runs, tx.Title, b.Width-2*v.TextInsetX)
		if needH := h + 2*v.TextInsetY; !textFits(needW, needH, b.Width, b.Height) {
			out = append(out, overflowWarning("funnel", fmt.Sprintf("stages[%d]", i), st.Label, needW, needH, b.Width, b.Height))
		}
		if len(st.Note) > 0 {
			nb := pl.notes[i]
			nr := itemRuns(st.Note, "", nil)
			needW := runsMinWidth(nr, tx.Sub)
			_, needH := runsBox(nr, tx.Sub, nb.Width)
			if !textFits(needW, needH, nb.Width, nb.Height) {
				out = append(out, overflowWarning("funnel", fmt.Sprintf("stages[%d].note", i), st.Note, needW, needH, nb.Width, nb.Height))
			}
		}
	}
	for i := 1; i < len(s.Data.Stages); i++ {
		a, b := s.Data.Stages[i-1].Number, s.Data.Stages[i].Number
		if a != nil && b != nil && *b > *a {
			out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] funnel: stages[%d].value: %s is larger than the previous stage (%s); a funnel is for values that decrease",
				i, s.Data.Stages[i].Value, s.Data.Stages[i-1].Value))
		}
	}
	_, missing := s.legend()
	return append(out, legendMissingWarnings("funnel", missing)...)
}

// --- IR から読む ---

type funnelIR struct {
	Unit   string `json:"unit"`
	Stages []struct {
		Label    Inline          `json:"label"`
		Value    json.RawMessage `json:"value"`
		Note     Inline          `json:"note"`
		Status   string          `json:"status"`
		Emphasis bool            `json:"emphasis"`
	} `json:"stages"`
	Legend orderedLegend `json:"legend"`
}

// formatFunnelNumber は数を3桁ごとの区切りつきで書く(小数はそのまま)。
func formatFunnelNumber(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	if frac != "" {
		out += "." + frac
	}
	return out
}

func funnelFromIR(raw json.RawMessage) (*FunnelData, error) {
	var in funnelIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("funnel: %w", err)
	}
	if len(in.Stages) < 2 {
		return nil, fmt.Errorf("funnel: stages needs at least two stages")
	}
	d := &FunnelData{Unit: in.Unit}
	for i, st := range in.Stages {
		at := fmt.Sprintf("stages[%d]", i)
		if len(st.Label) == 0 {
			return nil, fmt.Errorf("funnel: %s.label: must not be empty", at)
		}
		if err := checkFigureStatus(at, st.Status); err != nil {
			return nil, fmt.Errorf("funnel: %w", err)
		}
		fs := FunnelStage{Label: st.Label, Note: st.Note, Status: st.Status, Emphasis: st.Emphasis}
		v := bytes.TrimSpace(st.Value)
		switch {
		case len(v) == 0:
			return nil, fmt.Errorf("funnel: %s.value: a stage needs a value", at)
		case v[0] == '"':
			var s string
			json.Unmarshal(v, &s)
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("funnel: %s.value: must not be empty", at)
			}
			fs.Value = s
		default:
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return nil, fmt.Errorf("funnel: %s.value: must be a number or a string", at)
			}
			fs.Number = &f
			fs.Value = formatFunnelNumber(f) + in.Unit
		}
		d.Stages = append(d.Stages, fs)
	}
	lg, err := figureLegendIR("funnel", in.Legend)
	if err != nil {
		return nil, err
	}
	d.Legend = lg
	return d, nil
}

func funnelDecl() SubsystemDecl {
	return figureDecl("subsystems/funnel.schema.json",
		"段階ごとに数が減る流れ。帯の幅は値に比例しない(上から等しい刻みで狭まる)。値は帯の中に書く",
		"value は数(引用しない。unit を後ろに付けて書く)か文字列。数が前の段より増えると警告",
		"note は帯の右に、その段に揃えて置く",
		"段と段の間の転換率(前の段に対する %)は、前後の値が数ならサブシステムが帯の右に書く。note に転換率を書かない",
		"status を使ったら legend: {conditional: 意味, …} で枠線の色の意味を書く(凡例が図の下に出る。無ければ figure-legend の警告)",
	)
}
