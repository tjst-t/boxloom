// report(読む資料のテンプレート)が自分で持つ組み方(Backlog #83)。
//
// 見た目の違いは値の表(templates/report.json)と、ここで差し替える骨組みの部品で
// 持つ。コアの標準の部品・骨組みには手を入れない(dark・light の出力は変わらない)。
//
//   - contentHeader:リード文を上の見出し(leadText・基調色・太字)に置き、題(話題)を
//     その下に slideTitle・字の色で置く。題はこれまでどおり title のプレースホルダー
//     (位置だけが下になる)。リードの無いスライドは、題を上の見出しの位置と大きさで置く。
//   - columns・pairs:列の見出し(まとまりの題・対の側の見出し)を基調色(accent)にする。
//     columns の組み方は標準のもの(columnsStyled)で、見た目の選べるところだけを渡す。
//   - columns(Backlog #84):まとまりの題の下に罫線(grid・0.75pt)、まとまりの間の
//     隙間の真ん中に縦の罫線(grid)を引く。flow(左が右につながる)のときは、縦の
//     罫線の縦の真ん中に基調色の丸(直径 24pt、隙間より広ければ隙間の幅)と、
//     地の色(bg)の山形(>)を置く。
//   - section(Backlog #86):level 1・2 は frame(全面の基調色に白い枠、54pt の白い字)、
//     level 3 は agenda(左の帯 1/3 に親の節の題、右に兄弟の項目の一覧 24pt。今の項目に
//     丸い矢印の印、ほかは薄い字)。
//   - toc(Backlog #86):全面の基調色に白い字。題 28pt の下に白い罫線、章 20pt、
//     節 16pt で字下げ、頁番号は右寄せ。
//   - cover・closing(Backlog #92):表紙は全面の写真(meta.coverImage)の上に、左上から
//     幅 0.53・高さ 0.45 の基調色の箱を置き、白い字で題・副題を書く。作者・日付・機密区分・
//     免責は箱の下に置く。終わりのスライドは同じ写真に基調色の箱だけ。
//   - pairs(Backlog #87):標準の組み方(pairsStyled)の見た目を差し替える。対ごとに
//     A・B・C… の四角い印(基調色・白い字)を subject の頭に、2つの側の間に丸い山形の印を
//     置き、対の間に罫線(grid)を引く。側の見出し(title)は薄い面(card)の帯の上に字の色で書く。
//   - table(#88):見出しの行は面を塗らず基調色の字にし、下に基調色の罫線
//     (tableHeaderRuleWidth)を引く。行の間は灰色(grid)の細い罫線で、本体の行も
//     面を塗らない。行の群の名前のセルは濃い基調色(support)の面に白い字(bg)。
//     本物の a:tbl のまま、罫線と面はセルの書式で書く(reportTableLook)。
//   - takeaway(#88):左に丸い矢印(基調色の円に白い右向きの矢印。直径 calloutMark)、
//     右に基調色の太字の結論の文。地の面も左の帯も置かない(上下の余白はカードと同じ)。
//   - contentHeader の隅の印(#93):marker のある本文のスライドは、ページの右上の隅に
//     三角の地(幅・高さ 48pt、役割 reportMarkerRole)を置き、その中の丸(地の色)に印を
//     置く。その左に小さい字(small・薄い字)で今の項目の名前(パンくず。直前の区切りの題)。
//     どちらもページの上の余白の中なので、題・本文の配置は変わらない。
//   - quote(#88):薄い面(card)の吹き出し(角丸の面と、右の人の印へ向かう尾)の中に
//     引用の文、右に人の印(頭と肩。基調色)とその下に出どころ。
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
		return pairsStyled(ctx, b, reportPairsStyle)
	})); err != nil {
		return err
	}
	if err := t.skeleton.override(skelSection, skeletonFunc(reportSection)); err != nil {
		return err
	}
	if err := t.skeleton.override(skelToc, skeletonFunc(reportToc)); err != nil {
		return err
	}
	if err := t.skeleton.override(skelCover, skeletonFunc(reportCover)); err != nil {
		return err
	}
	if err := t.skeleton.override(skelClosing, skeletonFunc(reportClosing)); err != nil {
		return err
	}
	for name, f := range map[string]componentFunc{
		"table": func(ctx componentContext, b *IRBlock) (*Spec, error) {
			return ctx.t.tableStyled(b, reportTableLook), nil
		},
		"takeaway": func(ctx componentContext, b *IRBlock) (*Spec, error) {
			return ctx.t.reportTakeaway(b.Content), nil
		},
		"quote": func(ctx componentContext, b *IRBlock) (*Spec, error) {
			return ctx.t.reportQuote(b.Content, b.Source), nil
		},
	} {
		if err := t.components.override(name, f); err != nil {
			return err
		}
	}
	return t.skeleton.override(skelContentHeader, skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
		head := ctx.t.reportContentHeader(ctx.slide)
		if ctx.slide.Marker == "" {
			return head, nil
		}
		return ctx.t.reportMarked(head, ctx.slide.Marker, breadcrumb(ctx.deck, ctx.index)), nil
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

// report の表紙・終わりのスライドの基調色の箱の大きさ(ページの幅・高さに対する比)。
// 読む資料のサンプルを測った値。
const (
	reportCoverBoxW = 0.53
	reportCoverBoxH = 0.45
)

// reportCoverBand は表紙・終わりのスライドの上の帯:左に基調色の箱(幅 reportCoverBoxW)、
// 右は写真が見える空き。高さは呼ぶ側が grow で決める。
func reportCoverBand(box *Spec) *Spec {
	return Row([]*Spec{
		apply(box, []SpecOpt{tokBackground("accent"), WithGrow(reportCoverBoxW), WithBasis(0)}),
		Col(nil, WithGrow(1-reportCoverBoxW), WithBasis(0)),
	}, WithGrow(reportCoverBoxH), WithBasis(0))
}

// reportCover は report の表紙:全面に写真(meta.coverImage)を敷き、左上の基調色の箱
// (幅 0.53・高さ 0.45)の下寄せに白い字で題・副題を書く。作者・日付・機密区分・免責は
// 箱の下に置く。写真が無ければ地の色の上に同じ形。
func reportCover(ctx skeletonContext) (*Spec, error) {
	t, d, s := ctx.t, ctx.deck, ctx.slide
	title, subtitle, author := s.Title, s.Subtitle, s.Author
	titleAt, subtitleAt, authorAt := "title", "subtitle", "author"
	if d.Meta != nil {
		if title == nil {
			title, titleAt = d.Meta.Title, "/meta/title"
		}
		if subtitle == nil {
			subtitle, subtitleAt = d.Meta.Subtitle, "/meta/subtitle"
		}
		if author == nil {
			author, authorAt = d.Meta.Author, "/meta/author"
		}
	}
	inBox := []*Spec{Col(nil, WithGrow(1))}
	if title != nil {
		inBox = append(inBox, t.text(title, "coverTitle", "bg", true, WithPlaceholder("ctrTitle"), fromIR(titleAt)))
	}
	if subtitle != nil {
		inBox = append(inBox, t.text(subtitle, "coverSubtitle", "bg", false, WithPlaceholder("subTitle"), fromIR(subtitleAt)))
	}
	box := Col(inBox, tokGap("itemGap"), tokPad("coverPadding"))

	var foot []*Spec
	if author != nil {
		foot = append(foot, t.text(author, "coverAuthor", "text_primary", false, fromIR(authorAt)))
	}
	if d.Meta != nil && d.Meta.Date != "" {
		foot = append(foot, TextNode(t.Marks.formatDate(d.Meta.Date), tokSize("coverAuthor"), tokColor("text_primary"), fromIR("/meta/date")))
	}
	if d.Meta != nil && d.Meta.Confidential != nil {
		foot = append(foot, t.text(d.Meta.Confidential, "small", "text_primary", true, fromIR("/meta/confidential")))
	}
	if d.Meta != nil && d.Meta.Disclaimer != nil {
		foot = append(foot, t.text(d.Meta.Disclaimer, "small", "text_muted", false, fromIR("/meta/disclaimer")))
	}
	below := Col(foot, tokGap("itemGap"), tokPadXY("coverPadding", "pagePaddingY"), WithGrow(1-reportCoverBoxH), WithBasis(0))
	return ctx.withCoverImage(Col([]*Spec{reportCoverBand(box), below}, tokBackground("bg")))
}

// reportClosing は report の終わりのスライド:表紙と同じ写真に、同じ位置の基調色の箱だけ。
func reportClosing(ctx skeletonContext) (*Spec, error) {
	band := reportCoverBand(Col(nil))
	return ctx.withCoverImage(Col([]*Spec{band, Col(nil, WithGrow(1-reportCoverBoxH), WithBasis(0))}, tokBackground("bg")))
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

// reportPairMarkSize は report の対の印(A・B・C の四角・側の間の丸)の一辺(段の刻み。24pt)。
var reportPairMarkSize = spaceStep(12)

// reportPairsStyle は report の pairs の見た目。
var reportPairsStyle = pairsStyle{
	headRole: "accent",
	sideTitle: func(t *Template, title Inline) *Spec {
		return Col([]*Spec{t.text(title, "body", "text_primary", true)},
			tokBackground("card"), tokPadXY(spaceStep(4), spaceStep(2)))
	},
	mark: func(t *Template, k int) *Spec {
		return SubsystemNode(&pairMark{kind: pairMarkLetter, letter: pairLetter(k), fontSize: t.Values.Body},
			tokWidth(reportPairMarkSize), tokHeight(reportPairMarkSize))
	},
	between: func(t *Template) *Spec {
		return SubsystemNode(&pairMark{kind: pairMarkChevron},
			tokWidth(reportPairMarkSize), tokHeight(reportPairMarkSize))
	},
	rule: func(t *Template) *Spec {
		return Col(nil, tokHeight("tableRuleWidth"), tokBackground("grid"))
	},
	spacer: func(t *Template, mark *Spec) *Spec {
		return Col(nil, tokWidth(mark.tok.width))
	},
}

// pairLetter は k 番目(0 から)の対の印の文字(A・B…Z・AA・AB…)。
func pairLetter(k int) string {
	s := ""
	for k++; k > 0; k = (k - 1) / 26 {
		s = string(rune('A'+(k-1)%26)) + s
	}
	return s
}

type pairMarkKind int

const (
	pairMarkLetter  pairMarkKind = iota // 四角い地(基調色)に白い字の A・B・C
	pairMarkChevron                     // 丸い地(基調色)に白い山形(側の間の流れ)
)

// pairMark は report の対の印の葉。大きさは箱の幅・高さ(トークン)で決まり、
// 自然な大きさは申告しない。色は描くときに役割名で引く。
type pairMark struct {
	kind     pairMarkKind
	letter   string
	fontSize float64
}

func (m *pairMark) Name() string {
	if m.kind == pairMarkLetter {
		return "pairLetter"
	}
	return "pairArrow"
}

func (m *pairMark) Measure(Constraint) Measurement { return Measurement{} }

func (m *pairMark) ornament() {}

func (m *pairMark) Render(ctx RenderContext, box LBox) {
	fill, on := ctx.Roles.Color("accent"), ctx.Roles.Color("bg")
	if m.kind == pairMarkLetter {
		ctx.Out.AddShape(box, ShapeOpts{Prst: "rect", Fill: fill,
			Runs: []TextRun{{Text: m.letter, Color: on, Bold: true}}, FontSize: m.fontSize})
		return
	}
	ctx.Out.AddShape(box, ShapeOpts{Prst: "ellipse", Fill: fill})
	// 山形は丸の中央に、幅 0.36・高さ 0.44 の割合で置く。
	w, h := box.Width*0.36, box.Height*0.44
	ctx.Out.AddShape(LBox{X: box.X + (box.Width-w)/2 + box.Width*0.03, Y: box.Y + (box.Height-h)/2, Width: w, Height: h},
		ShapeOpts{Prst: "chevron", Fill: on})
}

// reportTableLook は report の表の見た目:見出しの行は面なし・基調色の字・基調色の
// 罫線、本体は面なしで灰色の罫線、行の群の名前は濃い基調色の面に白い字。
var reportTableLook = tableLook{headerText: "accent", headerRule: "accent", rule: "grid",
	groupFill: "support", groupText: "bg"}

// reportTakeaway は結論:左に丸い矢印、右に基調色の太字の文。地の面は置かないが、
// 上下にカードと同じ余白(cardPadding)を取り、本文から離して置く。
func (t *Template) reportTakeaway(in Inline) *Spec {
	return Row([]*Spec{
		SubsystemNode(&calloutMark{kind: calloutArrow, size: t.Values.CalloutMark}),
		t.prose(in, "takeawayText", "accent", true, WithGrow(1), WithBasis(0)),
	}, tokGap("itemGap"), tokPadXY("", "cardPadding"))
}

// reportQuote は引用:吹き出し(薄い面と、右へ向かう尾)の中に言葉、右に人の印と出どころ。
// 吹き出しと人の印の列の幅は 3:1。
func (t *Template) reportQuote(content, source Inline) *Spec {
	size := t.Values.CalloutMark
	bubble := Row([]*Spec{
		Col([]*Spec{t.prose(content, "quoteText", "text_primary", false)},
			tokPad("cardPadding"), tokBackground("card"), tokRadius("cardRadius"), WithGrow(1), WithBasis(0), asCard()),
		SubsystemNode(&calloutMark{kind: calloutTail, size: size}),
	}, WithGrow(3), WithBasis(0))
	person := []*Spec{SubsystemNode(&calloutMark{kind: calloutPerson, size: size})}
	if len(source) > 0 {
		person = append(person, t.prose(source, "quoteSource", "text_muted", false))
	}
	return Row([]*Spec{bubble, Col(person, tokGap("itemGap"), WithGrow(1), WithBasis(0))}, tokGap("itemGap"))
}

// calloutKind は report の結論・引用に添える小さな図形の種類。
type calloutKind int

const (
	calloutArrow  calloutKind = iota // 丸い矢印(基調色の円に白い右向きの矢印)
	calloutPerson                    // 人の印(基調色の頭と肩)
	calloutTail                      // 吹き出しの尾(薄い面の三角。左の面から右へ)
)

// calloutMark は report の組み方の中の図形の葉(shapes ではない)。大きさは値の表の
// calloutMark(尾は幅がその半分)、色は描くときに役割名で引く。葉は自分の大きさの
// 正方形を上の端に描き、割り当てがそれより高ければ下を空ける。
type calloutMark struct {
	kind calloutKind
	size float64
}

func (c *calloutMark) Name() string { return "callout" }

// ornament:文に添える印で、図ではない(横の余り spare-width に数えない)。
func (c *calloutMark) ornament() {}

func (c *calloutMark) width() float64 {
	if c.kind == calloutTail {
		return c.size / 2
	}
	return c.size
}

func (c *calloutMark) Measure(Constraint) Measurement {
	s := Size{Width: c.width(), Height: c.size}
	return Measurement{Natural: s, Min: s, Max: Size{Width: s.Width}}
}

func (c *calloutMark) Render(ctx RenderContext, box LBox) {
	s := c.size
	switch c.kind {
	case calloutArrow:
		ctx.Out.AddShape(LBox{X: box.X, Y: box.Y, Width: s, Height: s}, ShapeOpts{Prst: "ellipse", Fill: ctx.Roles.Color("accent")})
		ctx.Out.AddShape(LBox{X: box.X + s*0.25, Y: box.Y + s*0.3, Width: s * 0.5, Height: s * 0.4},
			ShapeOpts{Prst: "rightArrow", Fill: ctx.Roles.Color("bg")})
	case calloutPerson:
		fill := ctx.Roles.Color("accent")
		head := s * 0.45
		ctx.Out.AddShape(LBox{X: box.X + (s-head)/2, Y: box.Y, Width: head, Height: head}, ShapeOpts{Prst: "ellipse", Fill: fill})
		ctx.Out.AddShape(LBox{X: box.X + s*0.1, Y: box.Y + s*0.5, Width: s * 0.8, Height: s * 0.5}, ShapeOpts{Prst: "round2SameRect", Fill: fill})
	case calloutTail:
		// 尾は人の印の頭の高さに置く(左の面に少し重ねて継ぎ目を見せない)。
		h := math.Min(s*0.5, box.Height)
		ctx.Out.AddShape(LBox{X: box.X - 0.5, Y: box.Y + s*0.15, Width: c.width() + 0.5, Height: h},
			ShapeOpts{Prst: "rtTriangle", Fill: ctx.Roles.Color("card")})
	}
}

// reportMarkerSize は隅の印の三角の幅・高さ(pt。読む資料のサンプルを測った値)。
const reportMarkerSize float64 = 48

// reportMarkerCircle は三角の中の丸の直径(pt)。三角の内接円(半径 約 14pt)に収まり、中の文字の枠に2字が入る。
const reportMarkerCircle = 24

// reportMarkerRole は隅の印の三角の地の色の役割(report の濃い基調色)。
const reportMarkerRole = "support"

// breadcrumb はスライド i のパンくず:手前で最も近い区切りの題(無ければ nil)。区切りは
// 浅い階層が来ると深い階層を閉じるので、最も近い区切りがいま開いているいちばん深い階層。
func breadcrumb(d *IRDeck, i int) Inline {
	for j := i - 1; j >= 0; j-- {
		if d.Slides[j].Type == "section" {
			return d.Slides[j].Title
		}
	}
	return nil
}

// reportMarked は本文のスライドの頭 head の右の端に、隅の印とパンくずを描く葉(幅 0)を
// 添える。葉は頭の配置を変えず、描くのはページの右上の隅(上の余白の中)。
func (t *Template) reportMarked(head *Spec, marker string, crumb Inline) *Spec {
	leaf := SubsystemNode(&reportCornerMark{
		marker: marker, crumb: crumb.Plain(),
		pageW: t.Values.PageW, padX: t.Values.PagePadding, size: t.Values.Small,
	})
	return Row([]*Spec{apply(head, []SpecOpt{WithGrow(1), WithBasis(0)}), leaf})
}

// reportCornerMark は隅の印(三角の地と丸の中の印)とパンくずを描く。
type reportCornerMark struct {
	marker, crumb string
	pageW, padX   float64 // ページの幅と左右の外周
	size          float64 // 印とパンくずの字の大きさ(small)
}

func (reportCornerMark) Name() string                   { return "cornerMarker" }
func (reportCornerMark) Measure(Constraint) Measurement { return Measurement{} }
func (m *reportCornerMark) Render(ctx RenderContext, _ LBox) {
	x0 := m.pageW - reportMarkerSize
	// 右の角が直角の三角(rtTriangle の直角は左下にあるので、左右と上下を返す)。
	ctx.Out.AddShape(LBox{X: x0, Y: 0, Width: reportMarkerSize, Height: reportMarkerSize},
		ShapeOpts{Prst: "rtTriangle", Fill: ctx.Roles.Color(reportMarkerRole), FlipH: true, FlipV: true})
	// 丸は三角の内接円の中心に置く(直角の2辺 a の内接円の半径は a(2-√2)/2)。
	r := reportMarkerSize * (2 - math.Sqrt2) / 2
	cx, cy := m.pageW-r, r
	d := float64(reportMarkerCircle)
	ctx.Out.AddShape(LBox{X: cx - d/2, Y: cy - d/2, Width: d, Height: d}, ShapeOpts{
		Prst: "ellipse", Fill: ctx.Roles.Color("bg"), FontSize: m.size,
		Runs: []TextRun{{Text: m.marker, Color: ctx.Roles.Color(reportMarkerRole), Bold: true}},
	})
	if m.crumb == "" {
		return
	}
	gap := reportMarkerSize / 6
	right := x0 - gap
	if right <= m.padX {
		return
	}
	ctx.Out.AddTextBoxNoWrap(LBox{X: m.padX, Y: cy - d/2, Width: right - m.padX, Height: d},
		m.crumb, m.size, ctx.Roles.Color("text_muted"), false, "right", "middle")
}
