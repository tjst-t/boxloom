// report(読む資料のテンプレート)が自分で持つ組み方(Backlog #83)。
//
// 見た目の違いは値の表(templates/report.json)と、ここで差し替える骨組みの部品で
// 持つ。コアの標準の部品・骨組みには手を入れない(dark・light の出力は変わらない)。
//
//   - contentHeader:リード文を上の見出し(leadText・基調色・太字)に置き、題(話題)を
//     その下に slideTitle・字の色で置く。題はこれまでどおり title のプレースホルダー
//     (位置だけが下になる)。リードの無いスライドは、題を上の見出しの位置と大きさで置く。
//   - columns・pairs:列の見出し(まとまりの題・対の側の見出し)を基調色(accent)にする。
//     組み方は標準のもの(columnsStyled・pairsHeaded)で、見た目の選べるところだけを渡す。
//   - columns(Backlog #84):まとまりの題の下に罫線(grid・0.75pt)、まとまりの間の
//     隙間の真ん中に縦の罫線(grid)を引く。flow(左が右につながる)のときは、縦の
//     罫線の縦の真ん中に基調色の丸(直径 24pt、隙間より広ければ隙間の幅)と、
//     地の色(bg)の山形(>)を置く。

package boxloom

import "math"

// bundledTemplateParts は同梱のテンプレートのうち、自分の組み方を持つものの登録
// (loadTemplate が読み込みの後に呼ぶ)。
var bundledTemplateParts = map[string]func(t *Template) error{
	"report": registerReportParts,
}

func registerReportParts(t *Template) error {
	if err := t.components.override("columns", componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		flow := b.Flow
		return ctx.t.columnsStyled(ctx, b, columnsStyle{
			headRole: "accent",
			headRule: func() Subsystem { return reportHeadRule{} },
			gapMark:  func() Subsystem { return reportColumnDivider{flow: flow} },
		})
	})); err != nil {
		return err
	}
	// pairs は標準でない型なので、同梱の組み方(registerBundledComponents)の代わりに登録する。
	if err := t.components.register(pairsType, componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return pairsHeaded(ctx, b, "accent")
	})); err != nil {
		return err
	}
	return t.skeleton.override(skelContentHeader, skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
		return ctx.t.reportContentHeader(ctx.slide), nil
	}))
}

// reportContentHeader は report の本文のスライドの頭:上にリード文(上の見出し)、
// 下に題(話題)。リードが無ければ題が上の見出しの位置と大きさを取る。
func (t *Template) reportContentHeader(s *IRSlide) *Spec {
	if s.Lead == nil {
		return t.text(s.Title, "leadText", "accent", true, WithPlaceholder("title"), fromIR("title"))
	}
	lead := t.prose(s.Lead, "leadText", "accent", true, fromIR("lead"))
	title := t.text(s.Title, "slideTitle", "text_primary", false, WithPlaceholder("title"), fromIR("title"))
	return Col([]*Spec{lead, title}, tokGap("headerGap"))
}

// report の列の印の寸法(pt)。罫線の太さと流れの丸の直径は、読む資料のサンプルを
// 測った値。
const (
	reportRuleWidth    = 0.75
	reportFlowDiameter = 24
	reportChevronWidth = 1.5
)

// reportHeadRule はまとまりの題の下の罫線(高さ 0 の葉の上端に横に引く)。
type reportHeadRule struct{}

func (reportHeadRule) Name() string                   { return "columnsHeadRule" }
func (reportHeadRule) Measure(Constraint) Measurement { return Measurement{} }
func (reportHeadRule) Render(ctx RenderContext, box LBox) {
	if box.Width <= 0 {
		return
	}
	ctx.Out.AddLine(box.X, box.Y, box.X+box.Width, box.Y, reportRuleWidth, ctx.Roles.Color("grid"), "")
}

// reportColumnDivider はまとまりの間の隙間に置く縦の罫線と、flow のときの丸い矢印。
type reportColumnDivider struct{ flow bool }

func (reportColumnDivider) Name() string                   { return "columnsDivider" }
func (reportColumnDivider) Measure(Constraint) Measurement { return Measurement{} }
func (d reportColumnDivider) Render(ctx RenderContext, box LBox) {
	if box.Height <= 0 {
		return
	}
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	ctx.Out.AddLine(cx, box.Y, cx, box.Y+box.Height, reportRuleWidth, ctx.Roles.Color("grid"), "")
	if !d.flow {
		return
	}
	dia := math.Min(reportFlowDiameter, math.Min(box.Width, box.Height))
	if dia <= 0 {
		return
	}
	r := dia / 2
	ctx.Out.AddShape(LBox{X: cx - r, Y: cy - r, Width: dia, Height: dia}, ShapeOpts{Prst: "ellipse", Fill: ctx.Roles.Color("accent")})
	// 山形(>):丸の中ほどに、2本の線で。
	hw, hh := dia*0.12, dia*0.2
	bg := ctx.Roles.Color("bg")
	ctx.Out.AddLine(cx-hw, cy-hh, cx+hw, cy, reportChevronWidth, bg, "")
	ctx.Out.AddLine(cx+hw, cy, cx-hw, cy+hh, reportChevronWidth, bg, "")
}
