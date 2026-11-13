// orgchart サブシステム: 親1つと子・孫の上下の関係(docs/patterns.md の R9)。
//
// 一番上に root、その下に units(2段目)、それぞれの下に members(3段目)を
// 並べ、親の下の辺から子の上の辺へ、曲がる本物のコネクタ(bentConnector3。
// 親と子の真ん中の高さで横に曲がる)で繋ぐ。横の幅は、3段目の箱の数に比例して
// 各 unit の列に配る(3段目の箱はどれも同じ幅)。枠が狭いと箱の中で文を
// 折り返し、そのぶん箱を高くする。それでも収まらない文は警告にする。
// 設計は docs/subsystem.md の11節。IR の data の形は
// schema/subsystems/orgchart.schema.json。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// OrgChartValues は orgchart の寸法(pt)。既定は subsystems/orgchart.json。
type OrgChartValues struct {
	Padding       float64 `json:"padding" scale:"space"`
	UnitGap       float64 `json:"unitGap" scale:"space"`   // 2段目の列と列の間
	MemberGap     float64 `json:"memberGap" scale:"space"` // 同じ列の3段目の箱の間
	MaxRootW      float64 `json:"maxRootWidth"`
	MaxUnitW      float64 `json:"maxUnitWidth"`
	MaxMemberW    float64 `json:"maxMemberWidth"`
	MinBoxH       float64 `json:"minBoxHeight"`
	NaturalBoxH   float64 `json:"naturalBoxHeight"`
	MaxBoxH       float64 `json:"maxBoxHeight"`
	MinLevelGap   float64 `json:"minLevelGap" scale:"space"` // 段と段の間(コネクタが曲がる)
	NaturalLevel  float64 `json:"naturalLevelGap" scale:"space"`
	MaxLevelGap   float64 `json:"maxLevelGap" scale:"space"`
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
	AssistantGap  float64 `json:"assistantGap" scale:"space"`    // 2段目の箱と補佐の箱の間(#131)
	StackIndent   float64 `json:"stackIndent" scale:"space"`     // 縦に積む3段目の字下げ(#134)
	MinAssistantW float64 `json:"minAssistantWidth"`
	MaxAssistantW float64 `json:"maxAssistantWidth"`
}

func (v *OrgChartValues) legendValues(radius float64) figureLegendValues {
	return figureLegendValues{gap: v.LegendGap, itemGap: v.LegendItemGap, swatchW: v.LegendSwatchW, swatchH: v.LegendSwatchH,
		swatchGap: v.LegendSwatchG, outlineW: v.OutlineWidth, radius: radius}
}

// OrgUnit は2段目の箱と、その下の3段目。Assistant は箱の横の補佐(任意、#131)。
type OrgUnit struct {
	FigureNode
	Members   []FigureNode
	Assistant *FigureNode
}

// OrgChartData は IR の data を読んだもの。
type OrgChartData struct {
	Root   FigureNode
	Units  []OrgUnit
	Legend []LegendItem // 状態の凡例の文(書いた順)
}

// legend は凡例の項目と、使ったのに文の無い状態。
func (s *OrgChartSubsystem) legend() ([]LegendItem, []string) {
	used := map[string]bool{s.Data.Root.Status: true}
	for _, u := range s.Data.Units {
		used[u.Status] = true
		for _, m := range u.Members {
			used[m.Status] = true
		}
	}
	return figureLegendItems(used, s.Data.Legend)
}

// OrgChartSubsystem は OrgChartData を描くサブシステム。
type OrgChartSubsystem struct {
	Data *OrgChartData
	V    *OrgChartValues // nil なら既定(subsystems/orgchart.json)
	Text *FigureText     // nil なら既定のテンプレート(light)の段
}

func NewOrgChartSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*OrgChartData); ok {
		return &OrgChartSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("orgchart", data)
	if err != nil {
		return nil, err
	}
	d, err := orgChartFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &OrgChartSubsystem{Data: d}, nil
}

func (s *OrgChartSubsystem) Name() string { return "orgchart" }

func (s *OrgChartSubsystem) vals() *OrgChartValues {
	if s.V != nil {
		return s.V
	}
	return defaultOrgChartValues()
}

func (s *OrgChartSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *OrgChartSubsystem) boxValues(prst string) figureBoxValues {
	v := s.vals()
	return figureBoxValues{prst: prst, radius: v.CornerRadius, outlineWidth: v.OutlineWidth, insetX: v.TextInsetX, insetY: v.TextInsetY}
}

func (s *OrgChartSubsystem) hasMembers() bool {
	for _, u := range s.Data.Units {
		if len(u.Members) > 0 {
			return true
		}
	}
	return false
}

// orgGeom は幅だけで決まる置き方。x と幅は確定、高さは parts を配って決める。
type orgGeom struct {
	rootX, rootW float64
	unitX, unitW []float64
	memberX      [][]float64
	memberW      float64   // 3段目の箱のもとの幅(全部の列で同じ)
	memberWs     []float64 // 列ごとの3段目の箱の幅(泣き別れを避けて広げた後)
	asstX, asstW []float64 // 補佐の箱(無い列は幅 0)
	needAsst     []float64
	needRoot     float64
	needUnit     float64
	needMember   float64
	legend       figureLegendLayout
	parts        []flexPart
	stacked      bool      // 3段目を親の下に字下げして縦に積む形(#134)
	memSub       float64   // 3段目の箱の題の字(横に並べる形で詰めたときは小さい段、#153)
	memDetail    float64   // 3段目の箱の補足の字
	memStep      int       // 3段目の字を値の表の段でいくつ下げたか(0 は下げない)
	spineX       []float64 // 縦に積む形の左の縦線の x(枠の左端から)
}

// memberPart は列の j 番目の3段目の箱の高さが parts の何番目か。
func (g *orgGeom) memberPart(j int) int {
	if g.stacked {
		return orgPartUnit + 2 + 2*j
	}
	return orgPartMem
}

const (
	orgPartRoot = 1 // parts の中の場所(0 は上の余白)
	orgPartGap1 = 2
	orgPartUnit = 3
	orgPartGap2 = 4
	orgPartMem  = 5
)

// geom は幅 width の置き方。3段目を横に並べる形と、親の下に縦に積む形(#134)の
// うち、最小の高さが低いほうを選ぶ(同じなら横に並べる形)。広い枠では横に並べる形の
// ほうが低いので、従来の置き方のまま。
//
// 縦に積む形のほうが低い(幅が足りない)ときは、縦に積む前に、3段目の字を値の表の段で
// 1〜2段小さくして横に並べる形も試し(#153。どの語も箱の幅に入るものだけ)、最小の高さが
// 一番低い形を選ぶ(差が 0.5pt 以内なら、字を下げない側・横に並べる形を先に)。Measure と
// 描画はどちらもこの geom を使うので、選んだ形で一致する。
func (s *OrgChartSubsystem) geom(width float64) *orgGeom {
	g := s.geomRow(width, 0)
	if !s.hasMembers() {
		return g
	}
	st := s.geomStacked(width)
	gmin, smin := orgMinHeight(g), orgMinHeight(st)
	if !(smin < gmin-0.5) {
		return g
	}
	cands := []*orgGeom{}
	for k := 1; k <= orgCompactSteps; k++ {
		c := s.geomRow(width, k)
		if c == nil {
			break // 値の表の段にこれ以上小さい字が無い
		}
		if s.membersFitWidth(c) {
			cands = append(cands, c)
		}
	}
	cands = append(cands, st)
	best := smin
	for _, c := range cands {
		best = math.Min(best, orgMinHeight(c))
	}
	for _, c := range cands {
		if orgMinHeight(c) <= best+0.5 {
			return c
		}
	}
	return st
}

// orgCompactSteps は横に並べるために3段目の字を下げてよい段の数(#153)。
const orgCompactSteps = 2

func orgMinHeight(g *orgGeom) float64 {
	m, _, _ := flexTotals(g.parts)
	return m
}

// orgCompactMaxLines は字を下げて横に並べる形で、3段目の箱の題・補足の段落が折り返して
// よい行の数(#153。日本語はどの字の間でも折れるので、語が入るだけでは1行に数字ずつの
// 細い箱を選んでしまう)。
const orgCompactMaxLines = 2

// membersFitWidth は横に並べる形で、3段目のどの箱も一番長い語が箱の幅に入り、題と補足の
// どの段落も orgCompactMaxLines 行までに収まるか。
func (s *OrgChartSubsystem) membersFitWidth(g *orgGeom) bool {
	fv := s.boxValues("roundRect")
	lines := func(in Inline, size, inner float64) int {
		n := 0
		for _, p := range strings.Split(in.Plain(), "\n") {
			if p == "" {
				continue
			}
			n = max(n, len(wrapText(p, size, math.Max(inner, 0), false)))
		}
		return n
	}
	for i, u := range s.Data.Units {
		inner := g.memberWs[i] - 2*fv.insetX
		for j := range u.Members {
			m := &u.Members[j]
			if nodeMinWidth(m, g.memSub, g.memDetail, fv) > g.memberWs[i]+0.01 {
				return false
			}
			if lines(m.Title, g.memSub, inner) > orgCompactMaxLines || lines(m.Detail, g.memDetail, inner) > orgCompactMaxLines {
				return false
			}
		}
	}
	return true
}

// memberText は3段目の字(題・補足)を値の表の段で k 段下げたもの。どちらも下げられなければ
// ok は false(k が 0 ならそのまま)。
func (s *OrgChartSubsystem) memberText(k int) (sub, detail float64, ok bool) {
	tx := s.text()
	if k == 0 {
		return tx.Sub, tx.Detail, true
	}
	sub, detail = typeStepDown(tx.Sizes, tx.Sub, k), typeStepDown(tx.Sizes, tx.Detail, k)
	p1, p2 := typeStepDown(tx.Sizes, tx.Sub, k-1), typeStepDown(tx.Sizes, tx.Detail, k-1)
	return sub, detail, sub < p1 || detail < p2
}

// typeStepDown は字の大きさ x を段 sizes(小さい順)で k 段下げたもの。x が段に無ければ
// x より小さい一番大きい段を1段目と数える。下げきれなければ一番小さい段(それも x 以上なら x)。
func typeStepDown(sizes []float64, x float64, k int) float64 {
	out := x
	for i := len(sizes) - 1; i >= 0 && k > 0; i-- {
		if sizes[i] < out-scaleEps {
			out = sizes[i]
			k--
		}
	}
	return out
}

// geomStacked は3段目を縦に積む形:各 unit の列は同じ幅で、2段目の箱の下に3段目の箱を
// stackIndent だけ字下げして memberGap の間で積み、2段目の箱の下から左の縦線で繋ぐ。
func (s *OrgChartSubsystem) geomStacked(width float64) *orgGeom {
	v, tx := s.vals(), s.text()
	inner := math.Max(0, width-2*v.Padding)
	x0 := v.Padding
	g := &orgGeom{stacked: true, memSub: tx.Sub, memDetail: tx.Detail}
	U := len(s.Data.Units)
	colW := math.Max(0, (inner-float64(U-1)*v.UnitGap)/float64(U))
	g.unitX, g.unitW, g.memberX = make([]float64, U), make([]float64, U), make([][]float64, U)
	g.memberWs, g.spineX = make([]float64, U), make([]float64, U)
	g.asstX, g.asstW, g.needAsst = make([]float64, U), make([]float64, U), make([]float64, U)
	rows := 0
	for i, u := range s.Data.Units {
		x := x0 + float64(i)*(colW+v.UnitGap)
		g.unitW[i] = math.Min(colW, v.MaxUnitW)
		g.unitW[i] = nodeComfortWidth(&u.FigureNode, tx.Title, tx.Sub, g.unitW[i], colW, s.boxValues("rect"))
		g.unitX[i] = x + (colW-g.unitW[i])/2
		s.placeAssistant(g, i, x, colW, 0)
		indent := math.Min(v.StackIndent, g.unitW[i]/2)
		g.spineX[i] = g.unitX[i] + indent/2
		g.memberWs[i] = math.Max(0, g.unitW[i]-indent)
		for range u.Members {
			g.memberX[i] = append(g.memberX[i], g.unitX[i]+indent)
		}
		rows = max(rows, len(u.Members))
	}
	g.memberW = g.memberWs[0]
	g.rootW = math.Min(inner, v.MaxRootW)
	g.rootW = nodeComfortWidth(&s.Data.Root, tx.Title, tx.Sub, g.rootW, inner, s.boxValues("rect"))
	g.rootX = x0 + (inner-g.rootW)/2
	s.measureNeeds(g)
	box := s.boxPart
	gap := flexPart{v.MinLevelGap, v.NaturalLevel, v.MaxLevelGap}
	g.parts = []flexPart{fixedPart(v.Padding), box(g.needRoot), gap, box(g.needUnit)}
	for j := 0; j < rows; j++ {
		g.parts = append(g.parts, fixedPart(v.MemberGap), box(g.needMember))
	}
	s.finishParts(g, inner)
	return g
}

// measureNeeds は箱の幅から、各段の文が収まる高さを測る。
func (s *OrgChartSubsystem) measureNeeds(g *orgGeom) {
	tx := s.text()
	_, g.needRoot = nodeTextSize(&s.Data.Root, tx.Title, tx.Sub, g.rootW, s.boxValues("rect"))
	for i := range s.Data.Units {
		u := &s.Data.Units[i]
		_, h := nodeTextSize(&u.FigureNode, tx.Title, tx.Sub, g.unitW[i], s.boxValues("rect"))
		g.needUnit = math.Max(g.needUnit, h)
		if u.Assistant != nil {
			_, g.needAsst[i] = nodeTextSize(u.Assistant, tx.Sub, tx.Detail, g.asstW[i], s.boxValues("roundRect"))
			g.needUnit = math.Max(g.needUnit, g.needAsst[i])
		}
		for j := range u.Members {
			_, h := nodeTextSize(&u.Members[j], g.memSub, g.memDetail, g.memberWs[i], s.boxValues("roundRect"))
			g.needMember = math.Max(g.needMember, h)
		}
	}
}

// boxPart は文の高さ need の箱の段の最小・自然・上限。
func (s *OrgChartSubsystem) boxPart(need float64) flexPart {
	v := s.vals()
	return flexPart{math.Max(need, v.MinBoxH), math.Max(need, v.NaturalBoxH), math.Max(need, v.MaxBoxH)}
}

// finishParts は凡例と下の余白を足す。
func (s *OrgChartSubsystem) finishParts(g *orgGeom, inner float64) {
	v, tx := s.vals(), s.text()
	items, _ := s.legend()
	g.legend = layoutFigureLegend(items, inner, tx.Small, v.legendValues(0))
	g.parts = append(g.parts, g.legend.part(v.legendValues(0))...)
	g.parts = append(g.parts, fixedPart(v.Padding))
}

// geomRow は3段目を横に並べる形。step は3段目の字を値の表の段でいくつ下げるか(#153)。
// 下げられないときは nil。
func (s *OrgChartSubsystem) geomRow(width float64, step int) *orgGeom {
	v, tx := s.vals(), s.text()
	inner := math.Max(0, width-2*v.Padding)
	x0 := v.Padding // 枠の左端からの位置(描くときに枠の X を足す)
	ms, md, ok := s.memberText(step)
	if !ok {
		return nil
	}
	g := &orgGeom{memSub: ms, memDetail: md, memStep: step}
	U := len(s.Data.Units)
	weights, total, inGaps := make([]float64, U), 0.0, 0.0
	for i, u := range s.Data.Units {
		weights[i] = math.Max(1, float64(len(u.Members)))
		total += weights[i]
		inGaps += (weights[i] - 1) * v.MemberGap
	}
	g.memberW = math.Max(0, (inner-float64(U-1)*v.UnitGap-inGaps)/total)
	g.memberW = math.Min(g.memberW, v.MaxMemberW)
	colW := make([]float64, U)
	used := float64(U-1) * v.UnitGap
	for i := range colW {
		colW[i] = weights[i]*g.memberW + (weights[i]-1)*v.MemberGap
		used += colW[i]
	}
	// 余りは列の両側に等しく置く(列の真ん中が等しい間隔に近づく)。
	margin := math.Max(0, inner-used) / float64(U) / 2
	x := x0
	g.unitX, g.unitW, g.memberX = make([]float64, U), make([]float64, U), make([][]float64, U)
	g.memberWs = make([]float64, U)
	g.asstX, g.asstW, g.needAsst = make([]float64, U), make([]float64, U), make([]float64, U)
	for i, u := range s.Data.Units {
		x += margin
		g.unitW[i] = math.Min(colW[i], v.MaxUnitW)
		// 題が泣き別れ(「欧州 AI 委員 / 会」)になるなら、列の両側の余りの中で箱を広げる
		// (上限は maxUnitWidth。隣の列との間は unitGap のまま)。
		limit := math.Min(colW[i]+2*margin, v.MaxUnitW)
		g.unitW[i] = nodeComfortWidth(&u.FigureNode, tx.Title, tx.Sub, g.unitW[i], limit, s.boxValues("rect"))
		g.unitX[i] = x + (colW[i]-g.unitW[i])/2
		// 3段目も同じ。列の箱は揃えて同じ幅にし、列の余りを箱の数で分けた分まで広げる
		// (maxMemberWidth を越えてよいのはこのときだけ)。列の真ん中は動かさない。
		mw := g.memberW
		if k := float64(len(u.Members)); k > 0 {
			limit := g.memberW + 2*margin/k
			for j := range u.Members {
				mw = math.Max(mw, nodeComfortWidth(&u.Members[j], g.memSub, g.memDetail, g.memberW, limit, s.boxValues("roundRect")))
			}
			span := k*mw + (k-1)*v.MemberGap
			start := x + (colW[i]-span)/2
			for j := range u.Members {
				g.memberX[i] = append(g.memberX[i], start+float64(j)*(mw+v.MemberGap))
			}
		}
		g.memberWs[i] = mw
		s.placeAssistant(g, i, x, colW[i], margin)
		x += colW[i] + margin + v.UnitGap
	}
	g.rootW = math.Min(inner, v.MaxRootW)
	// root も泣き別れなら内側の幅まで広げる(同じ段にほかの箱は無い)。
	g.rootW = nodeComfortWidth(&s.Data.Root, tx.Title, tx.Sub, g.rootW, inner, s.boxValues("rect"))
	g.rootX = x0 + (inner-g.rootW)/2

	s.measureNeeds(g)
	box := s.boxPart
	gap := flexPart{v.MinLevelGap, v.NaturalLevel, v.MaxLevelGap}
	g.parts = []flexPart{fixedPart(v.Padding), box(g.needRoot), gap, box(g.needUnit)}
	if s.hasMembers() {
		g.parts = append(g.parts, gap, box(g.needMember))
	}
	s.finishParts(g, inner)
	return g
}

// placeAssistant は列 i の補佐の箱を、2段目の箱の右に置く(#131)。2段目の箱は列の
// 真ん中のまま(下の段への線と3段目の並びは変えない)。列の真ん中から右の、列の半分と
// 余りの中に「箱の半分+間+補佐」が収まるよう、補佐は maxAssistantWidth の幅のまま、
// 足りなければ2段目の箱を、それでも足りなければ両方を狭める(Measure の Min の幅は
// 補佐を minAssistantWidth と見て足す)。
func (s *OrgChartSubsystem) placeAssistant(g *orgGeom, i int, x, colW, margin float64) {
	if s.Data.Units[i].Assistant == nil {
		return
	}
	v := s.vals()
	// 文の幅に合わせると、字体が置き換わったときに折り返すので、幅は maxAssistantWidth に揃える。
	aw := v.MaxAssistantW
	room := math.Max(0, colW/2+margin-v.AssistantGap) // 箱の半分と補佐の幅に使える分
	uw := g.unitW[i]
	if uw/2+aw > room {
		if 2*(room-aw) >= aw {
			uw = 2 * (room - aw)
		} else {
			w := room / 1.5
			uw, aw = math.Min(uw, w), math.Min(aw, w)
		}
	}
	g.unitW[i] = uw
	g.unitX[i] = x + (colW-uw)/2
	g.asstW[i] = aw
	g.asstX[i] = g.unitX[i] + uw + v.AssistantGap
}

// assistantExtra は補佐のために足す横の幅(補佐1つにつき、列の真ん中の両側に「間+幅」)。
func (s *OrgChartSubsystem) assistantExtra(w float64) float64 {
	extra := 0.0
	for _, u := range s.Data.Units {
		if u.Assistant != nil {
			extra += 2 * (s.vals().AssistantGap + w)
		}
	}
	return extra
}

// Measure: 高さは3つの段の箱(文が収まる高さ以上)と段の間の、最小・自然・上限の和。
// 箱の幅は枠の幅で決まるので、狭い枠では文を折り返して箱が高くなる。
func (s *OrgChartSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	w := v.NaturalWidth + s.assistantExtra(v.MaxAssistantW)
	if c.Width > 0 {
		w = c.Width
	}
	min, nat, max := flexTotals(s.geom(w).parts)
	return Measurement{
		Natural: Size{Width: w, Height: nat},
		Min:     Size{Width: v.MinWidth + s.assistantExtra(v.MinAssistantW), Height: min},
		Max:     Size{Height: max},
	}
}

type orgPlaced struct {
	root    LBox
	units   []LBox
	members [][]LBox
	assts   []LBox // 補佐の箱(無い列は幅 0)
	g       *orgGeom
	legend  LBox // 凡例の左上
}

func (s *OrgChartSubsystem) place(box LBox) *orgPlaced {
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	y := make([]float64, len(sizes)+1)
	y[0] = box.Y
	for i, h := range sizes {
		y[i+1] = y[i] + h
	}
	pl := &orgPlaced{g: g, root: LBox{X: box.X + g.rootX, Y: y[orgPartRoot], Width: g.rootW, Height: sizes[orgPartRoot]}}
	// 凡例は最後の段の箱の下(下の余白の手前の部品)。
	if g.legend.h > 0 {
		k := len(sizes) - 2
		pl.legend = LBox{X: box.X + s.vals().Padding, Y: y[k] + s.vals().LegendGap, Height: g.legend.h}
	}
	for i, u := range s.Data.Units {
		pl.units = append(pl.units, LBox{X: box.X + g.unitX[i], Y: y[orgPartUnit], Width: g.unitW[i], Height: sizes[orgPartUnit]})
		var ab LBox
		if u.Assistant != nil {
			// 補佐は小さい箱:文の高さ(最小 minBoxHeight)で、2段目の箱の高さの真ん中に置く。
			h := math.Min(math.Max(g.needAsst[i], s.vals().MinBoxH), sizes[orgPartUnit])
			ab = LBox{X: box.X + g.asstX[i], Y: y[orgPartUnit] + (sizes[orgPartUnit]-h)/2, Width: g.asstW[i], Height: h}
		}
		pl.assts = append(pl.assts, ab)
		var ms []LBox
		for j := range u.Members {
			k := g.memberPart(j)
			ms = append(ms, LBox{X: box.X + g.memberX[i][j], Y: y[k], Width: g.memberWs[i], Height: sizes[k]})
		}
		pl.members = append(pl.members, ms)
	}
	return pl
}

func (s *OrgChartSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	pl := s.place(box)
	p := ctx.Out
	rootID := drawFigureNode(ctx, pl.root, &s.Data.Root, tx.Title, tx.Sub, s.boxValues("rect"))
	unitIDs := make([]int, len(s.Data.Units))
	memberIDs := make([][]int, len(s.Data.Units))
	for i := range s.Data.Units {
		u := &s.Data.Units[i]
		unitIDs[i] = drawFigureNode(ctx, pl.units[i], &u.FigureNode, tx.Title, tx.Sub, s.boxValues("rect"))
		if u.Assistant != nil {
			// 補佐:2段目の箱の右の辺から補佐の左の辺へ、破線の真っすぐなコネクタ。
			id := drawFigureNode(ctx, pl.assts[i], u.Assistant, tx.Sub, tx.Detail, s.boxValues("roundRect"))
			ub, ab := pl.units[i], pl.assts[i]
			yc := ab.Y + ab.Height/2
			p.AddConnector(ub.X+ub.Width, yc, ab.X, yc, v.LineWidth, ctx.Roles.Color("text_muted"), false,
				ConnectorEnd{unitIDs[i], 3}, ConnectorEnd{id, 1})
			p.Shapes[len(p.Shapes)-1].dash = "dash"
		}
		for j := range u.Members {
			memberIDs[i] = append(memberIDs[i], drawFigureNode(ctx, pl.members[i][j], &u.Members[j], pl.g.memSub, pl.g.memDetail, s.boxValues("roundRect")))
		}
	}
	drawFigureLegend(ctx, pl.legend.X, pl.legend.Y, pl.g.legend, tx.Small, v.legendValues(v.CornerRadius/2))
	color := ctx.Roles.Color("text_muted")
	link := func(a, b LBox, ida, idb int) {
		p.AddElbowConnector(a.X+a.Width/2, a.Y+a.Height, b.X+b.Width/2, b.Y, v.LineWidth, color, false,
			ConnectorEnd{ida, 2}, ConnectorEnd{idb, 0}, 0.5)
	}
	for i := range s.Data.Units {
		link(pl.root, pl.units[i], rootID, unitIDs[i])
		if ms := pl.members[i]; pl.g.stacked && len(ms) > 0 {
			// 縦に積む形:2段目の箱の下から左の縦線を下ろし、3段目の箱の左の辺へ横の線。
			ub := pl.units[i]
			x := box.X + pl.g.spineX[i]
			last := ms[len(ms)-1]
			p.AddLine(x, ub.Y+ub.Height, x, last.Y+last.Height/2, v.LineWidth, color, "")
			for j, m := range ms {
				yc := m.Y + m.Height/2
				p.AddConnector(x, yc, m.X, yc, v.LineWidth, color, false, ConnectorEnd{}, ConnectorEnd{memberIDs[i][j], 1})
			}
			continue
		}
		for j := range s.Data.Units[i].Members {
			link(pl.units[i], pl.members[i][j], unitIDs[i], memberIDs[i][j])
		}
	}
}

// Lint は箱の文が収まらないこと(一番長い語が箱より広い、箱の高さが足りない)を
// 警告にする。
func (s *OrgChartSubsystem) Lint(box LBox) []string {
	tx := s.text()
	pl := s.place(box)
	var out []string
	check := func(where string, n *FigureNode, b LBox, t1, t2 float64, fv figureBoxValues) {
		needW := nodeMinWidth(n, t1, t2, fv)
		_, needH := nodeTextSize(n, t1, t2, b.Width, fv)
		if !textFits(needW, needH, b.Width, b.Height) {
			out = append(out, overflowWarning("orgchart", where, n.Title, needW, needH, b.Width, b.Height))
		}
	}
	check("root", &s.Data.Root, pl.root, tx.Title, tx.Sub, s.boxValues("rect"))
	for i := range s.Data.Units {
		u := &s.Data.Units[i]
		check(fmt.Sprintf("units[%d]", i), &u.FigureNode, pl.units[i], tx.Title, tx.Sub, s.boxValues("rect"))
		if u.Assistant != nil {
			check(fmt.Sprintf("units[%d].assistant", i), u.Assistant, pl.assts[i], tx.Sub, tx.Detail, s.boxValues("roundRect"))
		}
		for j := range u.Members {
			check(fmt.Sprintf("units[%d].members[%d]", i, j), &u.Members[j], pl.members[i][j], pl.g.memSub, pl.g.memDetail, s.boxValues("roundRect"))
		}
	}
	_, missing := s.legend()
	return append(out, legendMissingWarnings("orgchart", missing)...)
}

// --- IR から読む ---

type orgUnitIR struct {
	figureNodeIR
	Members   []figureNodeIR  `json:"members"`
	Assistant *orgAssistantIR `json:"assistant"`
}

// orgAssistantIR は補佐の箱(題と補足だけ。状態・強調は持たない)。
type orgAssistantIR struct {
	Title  Inline `json:"title"`
	Detail Inline `json:"detail"`
}

type orgChartIR struct {
	Root   figureNodeIR  `json:"root"`
	Units  []orgUnitIR   `json:"units"`
	Legend orderedLegend `json:"legend"`
}

func orgChartFromIR(raw json.RawMessage) (*OrgChartData, error) {
	var in orgChartIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("orgchart: %w", err)
	}
	root, err := in.Root.node("root")
	if err != nil {
		return nil, fmt.Errorf("orgchart: %w", err)
	}
	if len(in.Units) == 0 {
		return nil, fmt.Errorf("orgchart: units needs at least one unit")
	}
	d := &OrgChartData{Root: root}
	for i, u := range in.Units {
		at := fmt.Sprintf("units[%d]", i)
		n, err := u.node(at)
		if err != nil {
			return nil, fmt.Errorf("orgchart: %w", err)
		}
		ou := OrgUnit{FigureNode: n}
		if a := u.Assistant; a != nil {
			an, err := figureNodeIR{Title: a.Title, Detail: a.Detail}.node(at + ".assistant")
			if err != nil {
				return nil, fmt.Errorf("orgchart: %w", err)
			}
			ou.Assistant = &an
		}
		for j, m := range u.Members {
			mn, err := m.node(fmt.Sprintf("%s.members[%d]", at, j))
			if err != nil {
				return nil, fmt.Errorf("orgchart: %w", err)
			}
			ou.Members = append(ou.Members, mn)
		}
		d.Units = append(d.Units, ou)
	}
	if d.Legend, err = figureLegendIR("orgchart", in.Legend); err != nil {
		return nil, err
	}
	return d, nil
}

func orgChartDecl() SubsystemDecl {
	return figureDecl("subsystems/orgchart.schema.json",
		"上下の関係(親1つ・子 2〜8)。root の下に units、各 unit の下に members(3段まで)",
		"箱の文は title(1行目)と detail(その下。人の名前・人数など)",
		"unit の横の補佐(副総括など)は assistant: {title, detail}(1つまで。破線で繋がる小さい箱)",
		"status を使ったら legend: {conditional: 意味, …} で枠線の色の意味を書く(凡例が図の下に出る。無ければ figure-legend の警告)",
	)
}
