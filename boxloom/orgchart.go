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
)

// OrgChartValues は orgchart の寸法(pt)。既定は subsystems/orgchart.json。
type OrgChartValues struct {
	Padding      float64 `json:"padding" scale:"space"`
	UnitGap      float64 `json:"unitGap" scale:"space"`   // 2段目の列と列の間
	MemberGap    float64 `json:"memberGap" scale:"space"` // 同じ列の3段目の箱の間
	MaxRootW     float64 `json:"maxRootWidth"`
	MaxUnitW     float64 `json:"maxUnitWidth"`
	MaxMemberW   float64 `json:"maxMemberWidth"`
	MinBoxH      float64 `json:"minBoxHeight"`
	NaturalBoxH  float64 `json:"naturalBoxHeight"`
	MaxBoxH      float64 `json:"maxBoxHeight"`
	MinLevelGap  float64 `json:"minLevelGap" scale:"space"` // 段と段の間(コネクタが曲がる)
	NaturalLevel float64 `json:"naturalLevelGap" scale:"space"`
	MaxLevelGap  float64 `json:"maxLevelGap" scale:"space"`
	NaturalWidth float64 `json:"naturalWidth"`
	MinWidth     float64 `json:"minWidth"`
	TextInsetX   float64 `json:"textInsetX" scale:"space"`
	TextInsetY   float64 `json:"textInsetY" scale:"space"`
	LineWidth    float64 `json:"lineWidth"`
	OutlineWidth float64 `json:"outlineWidth"`
	CornerRadius float64 `json:"cornerRadius"`
}

// OrgUnit は2段目の箱と、その下の3段目。
type OrgUnit struct {
	FigureNode
	Members []FigureNode
}

// OrgChartData は IR の data を読んだもの。
type OrgChartData struct {
	Root  FigureNode
	Units []OrgUnit
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
	memberW      float64
	needRoot     float64
	needUnit     float64
	needMember   float64
	parts        []flexPart
}

const (
	orgPartRoot = 1 // parts の中の場所(0 は上の余白)
	orgPartGap1 = 2
	orgPartUnit = 3
	orgPartGap2 = 4
	orgPartMem  = 5
)

func (s *OrgChartSubsystem) geom(width float64) *orgGeom {
	v, tx := s.vals(), s.text()
	inner := math.Max(0, width-2*v.Padding)
	x0 := v.Padding // 枠の左端からの位置(描くときに枠の X を足す)
	g := &orgGeom{}
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
	for i, u := range s.Data.Units {
		x += margin
		g.unitW[i] = math.Min(colW[i], v.MaxUnitW)
		g.unitX[i] = x + (colW[i]-g.unitW[i])/2
		for j := range u.Members {
			g.memberX[i] = append(g.memberX[i], x+float64(j)*(g.memberW+v.MemberGap))
		}
		x += colW[i] + margin + v.UnitGap
	}
	g.rootW = math.Min(inner, v.MaxRootW)
	g.rootX = x0 + (inner-g.rootW)/2

	_, g.needRoot = nodeTextSize(&s.Data.Root, tx.Title, tx.Sub, g.rootW, s.boxValues("rect"))
	for i := range s.Data.Units {
		u := &s.Data.Units[i]
		_, h := nodeTextSize(&u.FigureNode, tx.Title, tx.Sub, g.unitW[i], s.boxValues("rect"))
		g.needUnit = math.Max(g.needUnit, h)
		for j := range u.Members {
			_, h := nodeTextSize(&u.Members[j], tx.Sub, tx.Detail, g.memberW, s.boxValues("roundRect"))
			g.needMember = math.Max(g.needMember, h)
		}
	}
	box := func(need float64) flexPart {
		return flexPart{math.Max(need, v.MinBoxH), math.Max(need, v.NaturalBoxH), math.Max(need, v.MaxBoxH)}
	}
	gap := flexPart{v.MinLevelGap, v.NaturalLevel, v.MaxLevelGap}
	g.parts = []flexPart{fixedPart(v.Padding), box(g.needRoot), gap, box(g.needUnit)}
	if s.hasMembers() {
		g.parts = append(g.parts, gap, box(g.needMember))
	}
	g.parts = append(g.parts, fixedPart(v.Padding))
	return g
}

// Measure: 高さは3つの段の箱(文が収まる高さ以上)と段の間の、最小・自然・上限の和。
// 箱の幅は枠の幅で決まるので、狭い枠では文を折り返して箱が高くなる。
func (s *OrgChartSubsystem) Measure(c Constraint) Measurement {
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

type orgPlaced struct {
	root    LBox
	units   []LBox
	members [][]LBox
}

func (s *OrgChartSubsystem) place(box LBox) *orgPlaced {
	g := s.geom(box.Width)
	sizes := flexSizes(g.parts, box.Height)
	y := make([]float64, len(sizes)+1)
	y[0] = box.Y
	for i, h := range sizes {
		y[i+1] = y[i] + h
	}
	pl := &orgPlaced{root: LBox{X: box.X + g.rootX, Y: y[orgPartRoot], Width: g.rootW, Height: sizes[orgPartRoot]}}
	for i, u := range s.Data.Units {
		pl.units = append(pl.units, LBox{X: box.X + g.unitX[i], Y: y[orgPartUnit], Width: g.unitW[i], Height: sizes[orgPartUnit]})
		var ms []LBox
		for j := range u.Members {
			ms = append(ms, LBox{X: box.X + g.memberX[i][j], Y: y[orgPartMem], Width: g.memberW, Height: sizes[orgPartMem]})
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
		for j := range u.Members {
			memberIDs[i] = append(memberIDs[i], drawFigureNode(ctx, pl.members[i][j], &u.Members[j], tx.Sub, tx.Detail, s.boxValues("roundRect")))
		}
	}
	color := ctx.Roles.Color("text_muted")
	link := func(a, b LBox, ida, idb int) {
		p.AddElbowConnector(a.X+a.Width/2, a.Y+a.Height, b.X+b.Width/2, b.Y, v.LineWidth, color, false,
			ConnectorEnd{ida, 2}, ConnectorEnd{idb, 0}, 0.5)
	}
	for i := range s.Data.Units {
		link(pl.root, pl.units[i], rootID, unitIDs[i])
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
		for j := range u.Members {
			check(fmt.Sprintf("units[%d].members[%d]", i, j), &u.Members[j], pl.members[i][j], tx.Sub, tx.Detail, s.boxValues("roundRect"))
		}
	}
	return out
}

// --- IR から読む ---

type orgUnitIR struct {
	figureNodeIR
	Members []figureNodeIR `json:"members"`
}

type orgChartIR struct {
	Root  figureNodeIR `json:"root"`
	Units []orgUnitIR  `json:"units"`
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
		for j, m := range u.Members {
			mn, err := m.node(fmt.Sprintf("%s.members[%d]", at, j))
			if err != nil {
				return nil, fmt.Errorf("orgchart: %w", err)
			}
			ou.Members = append(ou.Members, mn)
		}
		d.Units = append(d.Units, ou)
	}
	return d, nil
}

func orgChartDecl() SubsystemDecl {
	return figureDecl("subsystems/orgchart.schema.json",
		"上下の関係(親1つ・子 2〜8)。root の下に units、各 unit の下に members(3段まで)",
		"箱の文は title(1行目)と detail(その下。人の名前・人数など)",
	)
}
