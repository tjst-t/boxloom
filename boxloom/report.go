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
//   - section(Backlog #86):level 1・2 は frame(全面の基調色に白い枠、54pt の白い字)、
//     level 3 は agenda(左の帯 1/3 に親の節の題、右に兄弟の項目の一覧 24pt。今の項目に
//     丸い矢印の印、ほかは薄い字)。
//   - toc(Backlog #86):全面の基調色に白い字。題 28pt の下に白い罫線、章 20pt、
//     節 16pt で字下げ、頁番号は右寄せ。
//
// 白は役割名 bg(report の地の色。白)で書く。測った大きさは段の刻み(type(n)・
// space(n))で書く(report が自分で持つ組み方の逃げ道。標準の部品は名前だけ)。

package boxloom

import (
	"fmt"
	"math"
)

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
	if err := t.skeleton.override(skelSection, skeletonFunc(reportSection)); err != nil {
		return err
	}
	if err := t.skeleton.override(skelToc, skeletonFunc(reportToc)); err != nil {
		return err
	}
	return t.skeleton.override(skelContentHeader, skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
		return ctx.t.reportContentHeader(ctx.slide), nil
	}))
}

// reportSection は report の区切り:level 1・2 は frame、level 3 は agenda。
func reportSection(ctx skeletonContext) (*Spec, error) {
	if sectionLevel(ctx.slide) >= 3 {
		return reportAgenda(ctx), nil
	}
	return reportFrame(ctx), nil
}

// reportFrame は章・節の区切り:全面の基調色の上に白い枠(細い白の地の内側に基調色)を
// 置き、その中に 54pt の白い太字で題を置く(上の余白 3 : 下の余白 2)。
func reportFrame(ctx skeletonContext) *Spec {
	t := ctx.t
	inner := Col([]*Spec{
		Col(nil, WithGrow(3)),
		t.text(ctx.slide.Title, typeStep(54), "bg", true, WithPlaceholder("title"), fromIR("title")),
		Col(nil, WithGrow(2)),
	}, tokPad("coverPadding"), tokBackground("accent"), WithGrow(1))
	frame := Col([]*Spec{inner}, tokPad(spaceStep(1)), tokBackground("bg"), WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{frame}, tokPad("pagePaddingY"), tokBackground("accent"))
}

// reportAgenda は節の中の項目の区切り:左の帯(幅 1/3、基調色)に親の節の題を白い字で、
// 右に同じ節の項目(兄弟)の一覧を 24pt で並べる。今の項目は丸い矢印の印と字の色、
// ほかは薄い字。
func reportAgenda(ctx skeletonContext) *Spec {
	t, d := ctx.t, ctx.deck
	parent, siblings := sectionFamily(d, ctx.index)
	var band []*Spec
	band = append(band, Col(nil, WithGrow(3)))
	if parent >= 0 {
		band = append(band, t.text(d.Slides[parent].Title, typeStep(28), "bg", true, fromIR(fmt.Sprintf("/slides/%d/title", parent))))
	}
	band = append(band, Col(nil, WithGrow(2)))
	var items []*Spec
	for _, j := range siblings {
		var item *Spec
		if j == ctx.index {
			item = t.text(d.Slides[j].Title, typeStep(24), "text_primary", true, WithPlaceholder("title"), fromIR("title"), WithGrow(1), WithBasis(0))
		} else {
			item = t.text(d.Slides[j].Title, typeStep(24), "text_muted", false, fromIR(fmt.Sprintf("/slides/%d/title", j)), WithGrow(1), WithBasis(0))
		}
		items = append(items, Row([]*Spec{reportAgendaMark(j == ctx.index), item}, tokGap("blockGap")))
	}
	list := Col(append(append([]*Spec{Col(nil, WithGrow(1))}, items...), Col(nil, WithGrow(1))), tokGap("blockGap"), WithGrow(1))
	row := Row([]*Spec{
		Col(band, tokPad("pagePadding"), tokBackground("accent"), WithGrow(1), WithBasis(0)),
		Col([]*Spec{list}, tokPad("pagePadding"), WithGrow(2), WithBasis(0)),
	}, WithGrow(1))
	main := Col([]*Spec{row}, WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{main}, tokBackground("bg"))
}

// reportAgendaMark は agenda の項目の頭の印:今の項目は基調色の丸に白い矢印(直径 24pt)、
// ほかは同じ幅の空き(項目の頭をそろえる)。
func reportAgendaMark(current bool) *Spec {
	if !current {
		return Col(nil, tokWidth(spaceStep(12)), tokHeight(spaceStep(12)))
	}
	arrow := TextNode("→", tokSize(typeStep(14)), tokColor("bg"), WithAlign("center"), WithAnchor("middle"), WithGrow(1))
	return Col([]*Spec{arrow}, tokWidth(spaceStep(12)), tokHeight(spaceStep(12)), tokRadius(spaceStep(6)), tokBackground("accent"))
}

// reportToc は report の目次:全面の基調色に白い字。題(28pt)の下に白い罫線、章は 20pt の
// 太字、節は 16pt で字下げ、頁番号は右寄せ。フッターは置かない(区切りと同じ流れの外の頁)。
func reportToc(ctx skeletonContext) (*Spec, error) {
	t := ctx.t
	title := t.text(ctx.slide.Title, typeStep(28), "bg", true, WithPlaceholder("title"), fromIR("/meta/toc"))
	rule := Col(nil, tokHeight(spaceStep(1)), tokBackground("bg"))
	var rows []*Spec
	for _, e := range ctx.tocEntries() {
		size, bold := typeStep(20), true
		if e.Level > 1 {
			size, bold = typeStep(16), false
		}
		row := Row([]*Spec{
			t.text(e.Title, size, "bg", bold, WithGrow(1), WithBasis(0)),
			TextNode(fmt.Sprint(e.Page), tokSize(size), tokColor("bg"), WithAlign("right")),
		}, tokGap("blockGap"))
		if e.Level > 1 {
			row = Row([]*Spec{Col(nil, tokWidth("tocIndent")), apply(row, []SpecOpt{WithGrow(1), WithBasis(0)})})
		}
		rows = append(rows, row)
	}
	main := Col(rows, tokGap("headerGap"), WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{title, rule, main}, tokGap("sectionGap"), tokPadXY("pagePadding", "pagePaddingY"), tokBackground("accent")), nil
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
