// pillars サブシステム: 屋根・柱・土台の図(目指す姿を、いくつかの柱が支え、土台の
// 上に立つ)。
//
// 上から、屋根(台形。目指す姿の文を持つ)、柱(縦長の箱を横に等しい幅で並べる。題と
// 短い文の箇条を持つ)、土台(横長の箱。任意)を積む。位置(上・並ぶ・下)が意味を持つ
// ので、作者は向きも置き場所も書かない。柱に短い名前(label、2字まで)があれば、柱の
// 上に丸の印を並べる。
//
// 色は基調色(accent)の系統:屋根は基調色、柱と土台は基調色を地の色(bg)に寄せた
// 淡い色(寄せる割合は値の表の pillarTint・foundationTint)。文の色は塗りとの差が
// 大きい役割名(readableRole)。
//
// 縦の大きさは部品ごとの最小・自然・上限の和で申告し、割り当てられた高さを配る
// (flexSizes)。どの部品も中の文が収まる高さより低くしない。設計は docs/subsystem.md の
// 11節。IR の data の形は schema/subsystems/pillars.schema.json。

package boxloom

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// PillarsValues は pillars の寸法(pt。割合は単位なし)。既定は subsystems/pillars.json。
type PillarsValues struct {
	Padding float64 `json:"padding" scale:"space"`
	// RoofSlant は屋根(台形)の斜めの辺の横の長さ。
	RoofSlant         float64 `json:"roofSlant"`
	RoofMinHeight     float64 `json:"roofMinHeight"`
	RoofNaturalHeight float64 `json:"roofNaturalHeight"`
	RoofMaxHeight     float64 `json:"roofMaxHeight"`
	RoofInsetX        float64 `json:"roofInsetX" scale:"space"`
	RoofInsetY        float64 `json:"roofInsetY" scale:"space"`
	// RowGap は屋根と柱の間・柱と土台の間。
	RowGap    float64 `json:"rowGap" scale:"space"`
	PillarGap float64 `json:"pillarGap" scale:"space"`
	// PillarOverhang は屋根・土台が柱の列より左右に張り出す長さ。
	PillarOverhang      float64 `json:"pillarOverhang" scale:"space"`
	PillarMinWidth      float64 `json:"pillarMinWidth"`
	PillarMinHeight     float64 `json:"pillarMinHeight"`
	PillarNaturalHeight float64 `json:"pillarNaturalHeight"`
	PillarMaxHeight     float64 `json:"pillarMaxHeight"`
	PillarInsetX        float64 `json:"pillarInsetX" scale:"space"`
	PillarInsetY        float64 `json:"pillarInsetY" scale:"space"`
	// LabelDiameter は柱の名前の丸の直径の下限(名前が入らなければ広げる)。
	LabelDiameter           float64 `json:"labelDiameter"`
	LabelGap                float64 `json:"labelGap" scale:"space"` // 丸と柱の間
	FoundationMinHeight     float64 `json:"foundationMinHeight"`
	FoundationNaturalHeight float64 `json:"foundationNaturalHeight"`
	FoundationMaxHeight     float64 `json:"foundationMaxHeight"`
	FoundationInsetX        float64 `json:"foundationInsetX" scale:"space"`
	FoundationInsetY        float64 `json:"foundationInsetY" scale:"space"`
	NaturalWidth            float64 `json:"naturalWidth"`
	MinWidth                float64 `json:"minWidth"`
	// PillarTint・FoundationTint は柱・土台の塗りを基調色から地の色へ寄せる割合(1 未満)。
	PillarTint     float64 `json:"pillarTint"`
	FoundationTint float64 `json:"foundationTint"`
}

// check は割合が 1 未満で、高さが最小 ≤ 自然 ≤ 上限であることを見る(0 より大きいことは
// checkPositive が見る)。
func (v *PillarsValues) check(at string) error {
	for _, t := range []struct {
		name string
		x    float64
	}{{"pillarTint", v.PillarTint}, {"foundationTint", v.FoundationTint}} {
		if t.x >= 1 {
			return fmt.Errorf("%s.%s: must be less than 1 (got %v)", at, t.name, t.x)
		}
	}
	for _, h := range []struct {
		name          string
		min, nat, max float64
	}{
		{"roof", v.RoofMinHeight, v.RoofNaturalHeight, v.RoofMaxHeight},
		{"pillar", v.PillarMinHeight, v.PillarNaturalHeight, v.PillarMaxHeight},
		{"foundation", v.FoundationMinHeight, v.FoundationNaturalHeight, v.FoundationMaxHeight},
	} {
		if h.min > h.nat || h.nat > h.max {
			return fmt.Errorf("%s: %sMinHeight ≤ %sNaturalHeight ≤ %sMaxHeight must hold (got %v, %v, %v)",
				at, h.name, h.name, h.name, h.min, h.nat, h.max)
		}
	}
	return nil
}

// Pillar は1本の柱。
type Pillar struct {
	Title Inline
	Items []Inline
	Label string // 無ければ ""
}

// PillarsData は IR の data を読んだもの。
type PillarsData struct {
	Roof       Inline
	Pillars    []Pillar
	Foundation Inline // 無ければ空
}

// 柱の数の範囲。1本は支えにならず、7本からは柱が細くて題が読めない。
const (
	pillarsMin = 2
	pillarsMax = 6
	// pillarLabelMaxRunes は柱の名前(label)の字の数の上限(丸の印に入る長さ)。
	pillarLabelMaxRunes = 2
)

// PillarsSubsystem は PillarsData を描くサブシステム。
type PillarsSubsystem struct {
	Data *PillarsData
	V    *PillarsValues // nil なら既定(subsystems/pillars.json)
	Text *FigureText    // nil なら既定のテンプレート(light)の段
}

func NewPillarsSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*PillarsData); ok {
		return &PillarsSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("pillars", data)
	if err != nil {
		return nil, err
	}
	d, err := pillarsFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &PillarsSubsystem{Data: d}, nil
}

func (s *PillarsSubsystem) Name() string { return "pillars" }

func (s *PillarsSubsystem) vals() *PillarsValues {
	if s.V != nil {
		return s.V
	}
	return defaultPillarsValues()
}

func (s *PillarsSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *PillarsSubsystem) hasLabels() bool {
	for _, p := range s.Data.Pillars {
		if p.Label != "" {
			return true
		}
	}
	return false
}

func (s *PillarsSubsystem) pointMark() string {
	if m := s.text().Point; m != "" {
		return m
	}
	return "・"
}

// --- 文 ---

func (s *PillarsSubsystem) roofRuns(color string, roles Roles) []TextRun {
	return boldRuns(itemRuns(s.Data.Roof, color, roles))
}

func (s *PillarsSubsystem) pillarTitleRuns(p *Pillar, color string, roles Roles) []TextRun {
	return boldRuns(itemRuns(p.Title, color, roles))
}

// pillarItemParas は柱の箇条の段落(頭に印の塊)。
func (s *PillarsSubsystem) pillarItemParas(p *Pillar, color string, roles Roles) [][]TextRun {
	c := ""
	if roles != nil {
		c = roles.Color(color)
	}
	var out [][]TextRun
	for _, it := range p.Items {
		out = append(out, append([]TextRun{{Text: s.pointMark(), Color: c, Bold: true, Mark: true}}, itemRuns(it, color, roles)...))
	}
	return out
}

// pillarNeed は幅 w の柱に文(題と箇条)が収まる幅(一番長い語)と高さ。
func (s *PillarsSubsystem) pillarNeed(p *Pillar, w float64) (needW, needH float64) {
	v, tx := s.vals(), s.text()
	inner := w - 2*v.PillarInsetX
	title := s.pillarTitleRuns(p, "", nil)
	_, h := runsBox(title, tx.Title, inner)
	needW = runsChunkWidth(title, tx.Title)
	markW := runWidthPt(s.pointMark(), tx.Detail, true)
	for _, para := range s.pillarItemParas(p, "", nil) {
		body := para[1:] // 印の分は字下げとして幅から引く(2行目からも同じ字下げ)
		_, dh := runsBox(body, tx.Detail, inner-markW)
		h += dh
		needW = math.Max(needW, runsChunkWidth(body, tx.Detail)+markW)
	}
	return needW + 2*v.PillarInsetX, h + 2*v.PillarInsetY
}

// roofTextWidth は屋根(幅 w の台形)の文を折り返す幅:上の辺(一番狭いところ)から
// 内側の余白を引いたもの。
func (s *PillarsSubsystem) roofTextWidth(w float64) float64 {
	v := s.vals()
	return w - 2*v.RoofSlant - 2*v.RoofInsetX
}

// roofHeightFor は、文の高さ textH を幅 w の屋根に入れるのに要る屋根の高さ。台形の
// 文字の枠(prstGeom の rect)は上が it = 2·h·slant/(3·w) だけ下がるので、その分を足す。
func (s *PillarsSubsystem) roofHeightFor(textH, w float64) float64 {
	v := s.vals()
	k := 1 - 2*v.RoofSlant/(3*math.Max(w, 1))
	k = math.Max(k, 0.5)
	return (textH + 2*v.RoofInsetY) / k
}

// labelDiameter は柱の名前の丸の直径(全ての柱で同じ。名前が無ければ 0)。
func (s *PillarsSubsystem) labelDiameter() float64 {
	if !s.hasLabels() {
		return 0
	}
	v, tx := s.vals(), s.text()
	d := v.LabelDiameter
	for _, p := range s.Data.Pillars {
		if p.Label == "" {
			continue
		}
		// 丸の文字の枠は内接する矩形(直径の cos45°)。
		w := runWidthPt(p.Label, tx.Sub, true)
		h := tx.Sub * lineHeightMult()
		d = math.Max(d, math.Max(w, h)/0.70710678+2)
	}
	return math.Ceil(d)
}

// --- 形 ---

type pillarsGeom struct {
	inner      float64
	pillarX    []float64 // 内側の左端から
	pillarW    float64
	roofNeed   float64
	pillarNeed float64
	foundNeed  float64
	labelD     float64
	parts      []flexPart
	roofIdx    int
	pillarIdx  int
	foundIdx   int // 土台が無ければ -1
}

func (s *PillarsSubsystem) geom(width float64) *pillarsGeom {
	v, tx := s.vals(), s.text()
	inner := math.Max(0, width-2*v.Padding)
	g := &pillarsGeom{inner: inner, foundIdx: -1}
	n := len(s.Data.Pillars)
	area := math.Max(0, inner-2*v.PillarOverhang)
	g.pillarW = math.Max(0, (area-float64(n-1)*v.PillarGap)/float64(n))
	for i := 0; i < n; i++ {
		g.pillarX = append(g.pillarX, v.PillarOverhang+float64(i)*(g.pillarW+v.PillarGap))
	}
	_, th := runsBox(s.roofRuns("", nil), tx.Title, s.roofTextWidth(inner))
	g.roofNeed = s.roofHeightFor(th, inner)
	for i := range s.Data.Pillars {
		_, h := s.pillarNeed(&s.Data.Pillars[i], g.pillarW)
		g.pillarNeed = math.Max(g.pillarNeed, h)
	}
	if len(s.Data.Foundation) > 0 {
		_, h := runsBox(itemRuns(s.Data.Foundation, "", nil), tx.Sub, inner-2*v.FoundationInsetX)
		g.foundNeed = h + 2*v.FoundationInsetY
	}
	g.labelD = s.labelDiameter()
	grow := func(need, min, nat, max float64) flexPart {
		return flexPart{math.Max(need, min), math.Max(need, nat), math.Max(need, max)}
	}
	g.parts = []flexPart{fixedPart(v.Padding)}
	g.roofIdx = len(g.parts)
	g.parts = append(g.parts, grow(g.roofNeed, v.RoofMinHeight, v.RoofNaturalHeight, v.RoofMaxHeight), fixedPart(v.RowGap))
	if g.labelD > 0 {
		g.parts = append(g.parts, fixedPart(g.labelD+v.LabelGap))
	}
	g.pillarIdx = len(g.parts)
	g.parts = append(g.parts, grow(g.pillarNeed, v.PillarMinHeight, v.PillarNaturalHeight, v.PillarMaxHeight))
	if len(s.Data.Foundation) > 0 {
		g.parts = append(g.parts, fixedPart(v.RowGap))
		g.foundIdx = len(g.parts)
		g.parts = append(g.parts, grow(g.foundNeed, v.FoundationMinHeight, v.FoundationNaturalHeight, v.FoundationMaxHeight))
	}
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

// minWidth は文の一番長い語(和文は塊)が屋根・柱・土台に収まる図の幅の下限。
func (s *PillarsSubsystem) minWidth() float64 {
	v, tx := s.vals(), s.text()
	n := float64(len(s.Data.Pillars))
	pw := v.PillarMinWidth
	for i := range s.Data.Pillars {
		w, _ := s.pillarNeed(&s.Data.Pillars[i], math.Inf(1))
		pw = math.Max(pw, w)
	}
	pw = math.Max(pw, s.labelDiameter())
	inner := n*pw + (n-1)*v.PillarGap + 2*v.PillarOverhang
	inner = math.Max(inner, runsChunkWidth(s.roofRuns("", nil), tx.Title)+2*v.RoofSlant+2*v.RoofInsetX)
	if len(s.Data.Foundation) > 0 {
		inner = math.Max(inner, runsChunkWidth(itemRuns(s.Data.Foundation, "", nil), tx.Sub)+2*v.FoundationInsetX)
	}
	return math.Max(v.MinWidth, math.Ceil(inner+2*v.Padding))
}

// Measure: 幅の下限は文の一番長い語が入る幅、高さは部品(屋根・柱・土台。どれも文が
// 収まる高さ以上)の最小・自然・上限の和。
func (s *PillarsSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	w := v.NaturalWidth
	if c.Width > 0 {
		w = c.Width
	}
	minW := s.minWidth()
	min, nat, max := flexTotals(s.geom(math.Max(w, minW)).parts)
	return Measurement{
		Natural: Size{Width: w, Height: nat},
		Min:     Size{Width: minW, Height: min},
		Max:     Size{Height: max},
	}
}

// columnMinWidth は並列のまとまりの中で要る幅(columnFloor):柱の文が読める幅。
func (s *PillarsSubsystem) columnMinWidth() float64 { return s.minWidth() }

type pillarsPlaced struct {
	roof       LBox
	pillars    []LBox
	labels     []LBox // 名前の無い柱は幅 0
	foundation LBox   // 無ければ幅 0
}

func (s *PillarsSubsystem) place(box LBox) (*pillarsGeom, *pillarsPlaced) {
	v := s.vals()
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	ys := make([]float64, len(sizes))
	y := box.Y
	for i, h := range sizes {
		ys[i] = y
		y += h
	}
	x0 := box.X + v.Padding
	pl := &pillarsPlaced{roof: LBox{X: x0, Y: ys[g.roofIdx], Width: g.inner, Height: sizes[g.roofIdx]}}
	py, ph := ys[g.pillarIdx], sizes[g.pillarIdx]
	for i, p := range s.Data.Pillars {
		pb := LBox{X: x0 + g.pillarX[i], Y: py, Width: g.pillarW, Height: ph}
		pl.pillars = append(pl.pillars, pb)
		lb := LBox{}
		if p.Label != "" {
			lb = LBox{X: pb.X + (pb.Width-g.labelD)/2, Y: py - v.LabelGap - g.labelD, Width: g.labelD, Height: g.labelD}
		}
		pl.labels = append(pl.labels, lb)
	}
	if g.foundIdx >= 0 {
		pl.foundation = LBox{X: x0, Y: ys[g.foundIdx], Width: g.inner, Height: sizes[g.foundIdx]}
	}
	return g, pl
}

// colors は屋根・柱・土台の塗り(基調色の系統)。
func (s *PillarsSubsystem) colors(roles Roles) (roof, pillar, foundation string) {
	v := s.vals()
	accent, bg := roles.Color("accent"), roles.Color("bg")
	return accent, mixHex(accent, bg, v.PillarTint), mixHex(accent, bg, v.FoundationTint)
}

// onFill は塗り fill の上の文の役割名。
func onFill(fill string, roles Roles) string {
	return readableRole(fill, roles, []string{"text_primary", "on_conditional"}, "text_muted")
}

func (s *PillarsSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	_, pl := s.place(box)
	p := ctx.Out
	roofFill, pillarFill, foundFill := s.colors(ctx.Roles)

	// 屋根:台形(上の辺が狭い)。斜めの辺の横の長さは adj(短い辺に対する割合)。
	r := pl.roof
	adj := 0
	if ss := math.Min(r.Width, r.Height); ss > 0 {
		adj = int(math.Round(v.RoofSlant / ss * 100000))
	}
	roofText := onFillRole(roofFill, ctx.Roles)
	p.AddShape(r, ShapeOpts{Prst: "trapezoid", Fill: roofFill, Depth: v.RoofSlant, Adj: []ShapeGuide{{"adj", adj}},
		Runs: s.roofRuns(roofText, ctx.Roles), FontSize: tx.Title,
		// 台形の文字の枠は斜めの辺の 2/3 だけ内側にある。上の辺まで下げるため残りの 1/3 を足す。
		InsetX: v.RoofSlant/3 + v.RoofInsetX, InsetY: v.RoofInsetY})

	pillarText := onFill(pillarFill, ctx.Roles)
	for i := range s.Data.Pillars {
		pi := &s.Data.Pillars[i]
		if lb := pl.labels[i]; lb.Width > 0 {
			p.AddShape(lb, ShapeOpts{Prst: "ellipse", Fill: roofFill, FontSize: tx.Sub,
				Runs: []TextRun{{Text: pi.Label, Color: ctx.Roles.Color(onFillRole(roofFill, ctx.Roles)), Bold: true}}})
		}
		o := ShapeOpts{Prst: "rect", Fill: pillarFill, FontSize: tx.Title,
			Runs:   s.pillarTitleRuns(pi, pillarText, ctx.Roles),
			InsetX: v.PillarInsetX, InsetY: v.PillarInsetY, Align: "left", Anchor: "top"}
		for _, para := range s.pillarItemParas(pi, pillarText, ctx.Roles) {
			o.Paras = append(o.Paras, para)
			o.ParaSizes = append(o.ParaSizes, tx.Detail)
		}
		p.AddShape(pl.pillars[i], o)
	}
	if pl.foundation.Width > 0 {
		p.AddShape(pl.foundation, ShapeOpts{Prst: "rect", Fill: foundFill, FontSize: tx.Sub,
			Runs:   itemRuns(s.Data.Foundation, onFill(foundFill, ctx.Roles), ctx.Roles),
			InsetX: v.FoundationInsetX, InsetY: v.FoundationInsetY})
	}
}

// Lint は屋根・柱・土台・柱の名前の文が収まらないことを警告にする(figure-text-overflow)。
func (s *PillarsSubsystem) Lint(box LBox) []string {
	v, tx := s.vals(), s.text()
	_, pl := s.place(box)
	var out []string
	{
		r := pl.roof
		runs := s.roofRuns("", nil)
		tw := s.roofTextWidth(r.Width)
		needW := runsChunkWidth(runs, tx.Title)
		_, th := runsBox(runs, tx.Title, tw)
		if needH := s.roofHeightFor(th, r.Width); !textFits(needW, needH, tw, r.Height) {
			out = append(out, overflowWarning("pillars", "roof", s.Data.Roof, needW+2*v.RoofSlant+2*v.RoofInsetX, needH, r.Width, r.Height))
		}
	}
	for i := range s.Data.Pillars {
		pi := &s.Data.Pillars[i]
		b := pl.pillars[i]
		needW, needH := s.pillarNeed(pi, b.Width)
		if !textFits(needW, needH, b.Width, b.Height) {
			out = append(out, overflowWarning("pillars", fmt.Sprintf("pillars[%d]", i), pi.Title, needW, needH, b.Width, b.Height))
		}
		if lb := pl.labels[i]; lb.Width > 0 && lb.Width > b.Width+0.5 {
			out = append(out, overflowWarning("pillars", fmt.Sprintf("pillars[%d].label", i), Inline{{Text: pi.Label}}, lb.Width, lb.Height, b.Width, lb.Height))
		}
	}
	if f := pl.foundation; f.Width > 0 {
		runs := itemRuns(s.Data.Foundation, "", nil)
		needW := runsChunkWidth(runs, tx.Sub) + 2*v.FoundationInsetX
		_, h := runsBox(runs, tx.Sub, f.Width-2*v.FoundationInsetX)
		if needH := h + 2*v.FoundationInsetY; !textFits(needW, needH, f.Width, f.Height) {
			out = append(out, overflowWarning("pillars", "foundation", s.Data.Foundation, needW, needH, f.Width, f.Height))
		}
	}
	return out
}

// --- IR から読む ---

type pillarsIR struct {
	Roof    Inline `json:"roof"`
	Pillars []struct {
		Title Inline   `json:"title"`
		Items []Inline `json:"items"`
		Label *string  `json:"label"`
	} `json:"pillars"`
	Foundation Inline `json:"foundation"`
}

// checkPillarLabel は柱の名前(1〜2字の見える字)を見る。
func checkPillarLabel(l string) error {
	if strings.TrimSpace(l) == "" {
		return fmt.Errorf("the label must not be blank")
	}
	if n := utf8.RuneCountInString(l); n > pillarLabelMaxRunes {
		return fmt.Errorf("the label %q is longer than %d characters (got %d; it goes in a small circle)", l, pillarLabelMaxRunes, n)
	}
	return nil
}

func pillarsFromIR(raw []byte) (*PillarsData, error) {
	var in pillarsIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("pillars: %w", err)
	}
	if len(in.Roof) == 0 {
		return nil, fmt.Errorf("pillars: roof: must not be empty")
	}
	if len(in.Pillars) < pillarsMin || len(in.Pillars) > pillarsMax {
		return nil, fmt.Errorf("pillars: pillars needs %d to %d pillars (has %d)", pillarsMin, pillarsMax, len(in.Pillars))
	}
	d := &PillarsData{Roof: in.Roof, Foundation: in.Foundation}
	for i, p := range in.Pillars {
		at := fmt.Sprintf("pillars[%d]", i)
		if len(p.Title) == 0 {
			return nil, fmt.Errorf("pillars: %s.title: must not be empty", at)
		}
		pi := Pillar{Title: p.Title}
		if p.Items != nil && len(p.Items) == 0 {
			return nil, fmt.Errorf("pillars: %s.items: needs at least one item (or leave items out)", at)
		}
		for k, it := range p.Items {
			if len(it) == 0 {
				return nil, fmt.Errorf("pillars: %s.items[%d]: must not be empty", at, k)
			}
			pi.Items = append(pi.Items, it)
		}
		if p.Label != nil {
			if err := checkPillarLabel(*p.Label); err != nil {
				return nil, fmt.Errorf("pillars: %s.label: %w", at, err)
			}
			pi.Label = *p.Label
		}
		d.Pillars = append(d.Pillars, pi)
	}
	return d, nil
}

func pillarsDecl() SubsystemDecl {
	return SubsystemDecl{
		Schema: embeddedSchema("subsystems/pillars.schema.json"),
		Markdown: []string{
			"中身は YAML で、キーは IR の data と同じ。知らないキーは誤り",
			"屋根・柱・土台の図。目指す姿(roof)を、2〜6本の柱(pillars)が支え、土台(foundation、任意)の上に立つ。向き・置き場所・大きさ・色は書けない(屋根は上、柱は横に並び、土台は下。色は基調色の系統)",
			"柱は {title, items?, label?}。items は短い文の列(柱の中の箇条)。label は柱の短い名前(1〜2字。柱の上の丸の印。数字だけなら \"1\" のように引用してよい)",
			"文の値は本文と同じ文中の記法で読む。roof・foundation を YAML の列で書くと各行を意味のある改行でつなぐ",
			"柱ごとの長い説明は図に詰めず、::::columns で図の横のまとまりに書く(pillars は :::column の中に置ける)",
		},
	}
}
