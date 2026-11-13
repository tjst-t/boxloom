// report(読む資料のテンプレート)が自分で持つ組み方(Backlog #83)。
//
// 見た目の違いは値の表(templates/report.json)と、ここで差し替える骨組みの部品で
// 持つ。コアの標準の部品・骨組みには手を入れない(dark・light の出力は変わらない)。
//
//   - contentHeader:リード文を上の見出し(leadText・基調色・標準の太さ)に置き、題(話題)を
//     その下に slideTitle・字の色で置く。題はこれまでどおり title のプレースホルダー
//     (位置だけが下になる)。リードの無いスライドは、題を上の見出しの位置と大きさで置く。
//   - columns・pairs:列の見出し(まとまりの題・対の側の見出し)を基調色(accent)にする。
//     columns の組み方は標準のもの(columnsStyled)で、見た目の選べるところだけを渡す。
//   - columns(Backlog #84):まとまりの題の下に罫線(grid・0.75pt)、まとまりの間の
//     隙間の真ん中に縦の罫線(grid)を引く。flow(左が右につながる)のときは、縦の
//     罫線の縦の真ん中に基調色の丸(直径 24pt、隙間より広ければ隙間の幅)と、
//     地の色(bg)の山形(>)を置く。
//   - section(Backlog #86・#137):level 1・2 は frame(全面の基調色に白い枠、54pt の白い字)。
//     直下に level 3 の項目があれば題の下にその題の一覧(20pt・白)を並べる。level 3 は
//     題だけの簡素な扉(地の色に短い基調色の罫線と 28pt の基調色の題)。
//   - toc(Backlog #86):全面の基調色に白い字。題 28pt の下に白い罫線、章 20pt、
//     節 16pt で字下げ、頁番号は右寄せ。
//   - cover・closing(Backlog #92):表紙は全面の写真(meta.coverImage)の上に、左の縦の
//     中ほど(左の縁から pagePadding 離す。#137)に幅 0.53・高さ 0.45 の基調色の箱を置き(上下に写真が見える。#109)、白い字で
//     題・副題・作者・日付・機密区分・免責を箱の中に書く。終わりのスライドは同じ写真に基調色の箱(meta.closing の文が
//     あれば箱の中に)。箱の高さは 0.45 を下限に、中身が要るだけ伸ばす(#104)。
//   - pairs(Backlog #87):標準の組み方(pairsStyled)の見た目を差し替える。対ごとに
//     A・B・C… の四角い印(基調色・白い字)を subject の頭に、2つの側の間に丸い山形の印を
//     置き、対の間に罫線(grid)を引く。側の見出し(title)は薄い面(card)の帯の上に字の色で書く。
//     対の名前(label。#118)が全て2字までなら A・B・C の代わりに印の中に、3字以上が
//     あれば印の代わりに行の左の見出しの列(幅 80pt・基調色の太字)に書く。
//   - topics(#119):標準の組み方(topicsStyled)の見た目を差し替える。塊の左に丸い印
//     (基調色の円に白い字の label。直径 24pt)、右に基調色の太字の題と中身、塊の間に
//     罫線(grid)。印の無い塊の並びは、題と中身だけを罫線で区切る。
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
//   - table の行の群(#100):名前のセルは縦書きの細い帯(a:tcPr の vert="eaVert")。列の幅は
//     縦の1行で、余りの幅を配らない。
//   - image・gallery(#100):説明の文を図の上に、基調色(accent)で置く。
//   - quote(#88):薄い面(card)の吹き出し(角丸の面と、右の人の印へ向かう尾)の中に
//     引用の文、右に人の印(頭と肩。基調色)とその下に出どころ。
//
// 白は役割名 bg(report の地の色。白)で書く。測った大きさは段の刻み(type(n)・
// space(n))で書く(report が自分で持つ組み方の逃げ道。標準の部品は名前だけ)。

package boxloom

import (
	"fmt"
	"math"
	"unicode/utf8"
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
	if err := t.components.register(topicsType, componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return topicsStyled(ctx, b, reportTopicsStyle)
	})); err != nil {
		return err
	}
	// people も標準でない型。名前を基調色にする。
	if err := t.components.register(peopleType, componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return peopleStyled(ctx, b, peopleStyle{nameRole: "accent"})
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
		"image": func(ctx componentContext, b *IRBlock) (*Spec, error) {
			leaf, err := ctx.t.imageLeafStyled(b.Src, b.Alt, b.Caption, reportImageLook)
			if err != nil {
				return nil, fmt.Errorf("src: %w", err)
			}
			return SubsystemNode(leaf, WithGrow(1)), nil
		},
		"gallery": func(ctx componentContext, b *IRBlock) (*Spec, error) {
			return ctx.t.galleryStyled(b, reportImageLook)
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

// reportSection は report の区切り(#137):level 1・2 は frame(下に level 3 の項目が
// あれば、その題の一覧を題の下に並べる)、level 3 は題だけの簡素な扉(door)。agenda の
// level 2・3 は親の題の frame に同じ親の下の区切りの一覧を並べ、今の区切りを強調する(#142)。
func reportSection(ctx skeletonContext) (*Spec, error) {
	if sectionAgenda(ctx.deck, ctx.index) {
		return reportAgenda(ctx), nil
	}
	if sectionLevel(ctx.slide) >= 3 {
		return reportDoor(ctx), nil
	}
	return reportFrame(ctx), nil
}

// reportFrame は章・節の区切り:全面の基調色の上に白い枠(細い白の地の内側に基調色)を
// 置き、その中に 54pt の白い太字で題を置く(上の余白 3 : 下の余白 2)。直下に level 3 の
// 区切り(項目)があれば、題の下に白い縦の罫線と項目の題の一覧(20pt・白)を並べる
// (#137。数は組み立てで数える。IR には書かない)。
func reportFrame(ctx skeletonContext) *Spec {
	t, d := ctx.t, ctx.deck
	head := t.text(ctx.slide.Title, typeStep(54), "bg", true, WithPlaceholder("title"), fromIR("title"))
	var items []*Spec
	for _, j := range sectionChildren(d, ctx.index) {
		items = append(items, t.text(d.Slides[j].Title, typeStep(20), "bg", false, fromIR(fmt.Sprintf("/slides/%d/title", j))))
	}
	return reportFramePage(head, items)
}

// reportAgenda は agenda の区切り(#142):親の区切りの扉と同じ frame・同じ一覧(同じ親の
// 下の区切りの題、20pt)。題の場所には親の題を置き、一覧の中の今の区切りを白の太字
// (題のプレースホルダー)に、ほかを淡い色(risk)にする。親が無ければ自分の題を置く。
func reportAgenda(ctx skeletonContext) *Spec {
	t, d := ctx.t, ctx.deck
	headTitle, headFrom := ctx.slide.Title, "title"
	if p := sectionParent(d, ctx.index); p >= 0 {
		headTitle, headFrom = d.Slides[p].Title, fmt.Sprintf("/slides/%d/title", p)
	}
	head := t.text(headTitle, typeStep(54), "bg", true, fromIR(headFrom))
	var items []*Spec
	for _, j := range sectionSiblings(d, ctx.index) {
		if j == ctx.index {
			items = append(items, t.text(ctx.slide.Title, typeStep(20), "bg", true, WithPlaceholder("title"), fromIR("title")))
			continue
		}
		items = append(items, t.text(d.Slides[j].Title, typeStep(20), "risk", false, fromIR(fmt.Sprintf("/slides/%d/title", j))))
	}
	return reportFramePage(head, items)
}

// reportFramePage は frame の頁:題 head の下に、items があれば白い縦の罫線と一覧を並べる。
func reportFramePage(title *Spec, items []*Spec) *Spec {
	head := []*Spec{title}
	if len(items) > 0 {
		list := Row([]*Spec{
			Col(nil, tokWidth(spaceStep(1)), tokBackground("bg")),
			Col(items, tokGap("itemGap"), WithGrow(1), WithBasis(0)),
		}, tokGap("blockGap"))
		head = append(head, list)
	}
	inner := Col([]*Spec{
		Col(nil, WithGrow(3)),
		Col(head, tokGap("sectionGap")),
		Col(nil, WithGrow(2)),
	}, tokPad("coverPadding"), tokBackground("accent"), WithGrow(1))
	frame := Col([]*Spec{inner}, tokPad(spaceStep(1)), tokBackground("bg"), WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{frame}, tokPad("pagePaddingY"), tokBackground("accent"))
}

// reportDoor は節の中の項目(level 3)の扉(#137):地の色のページの縦の中ほどに、短い
// 基調色の罫線とその下に 28pt の基調色の太字の題だけを置く(左は本文と同じ外周)。
func reportDoor(ctx skeletonContext) *Spec {
	t := ctx.t
	rule := Row([]*Spec{Col(nil, tokWidth(spaceStep(24)), tokHeight(spaceStep(2)), tokBackground("accent"))})
	title := t.text(ctx.slide.Title, typeStep(28), "accent", true, WithPlaceholder("title"), fromIR("title"))
	main := Col([]*Spec{
		Col(nil, WithGrow(1)),
		Col([]*Spec{rule, title}, tokGap("itemGap")),
		Col(nil, WithGrow(1)),
	}, WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{main}, tokPadXY("pagePadding", "pagePaddingY"), tokBackground("bg"))
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
// 読む資料のサンプルを測った値。高さは下限で、中身(題の折り返した行・副題・日付)が要れば
// 箱を伸ばす(#104)。箱はページの縦の中ほどに置き、上下に写真が見える空きを
// reportCoverMinGap 以上残す(#109。上下の縁に接しない)。
const (
	reportCoverBoxW   = 0.53
	reportCoverBoxH   = 0.45
	reportCoverMinGap = 0.1
)

// reportCoverPage は表紙・終わりのスライドの根:縦の中ほどに帯(左の外周 pagePadding の
// 右に基調色の箱。幅 reportCoverBoxW、右は写真が見える空き)を置き、上下は写真が見える空き。帯の高さは
// ページの高さの share。
func reportCoverPage(ctx skeletonContext, box *Spec, share float64) (*Spec, error) {
	gap := (1 - share) / 2
	// 箱は左の縁に付けず、本文と同じ外周(pagePadding)だけ離す(#137)。幅はページの
	// reportCoverBoxW のまま(右の空きの伸びる比から外周の分を引く)。
	pad := ctx.t.Values.PagePadding / ctx.t.Values.PageW
	band := Row([]*Spec{
		Col(nil, tokWidth("pagePadding")),
		apply(box, []SpecOpt{tokBackground("accent"), WithGrow(reportCoverBoxW), WithBasis(0)}),
		Col(nil, WithGrow(1-reportCoverBoxW-pad), WithBasis(0)),
	}, WithGrow(share), WithBasis(0))
	return ctx.withCoverImage(Col([]*Spec{
		Col(nil, WithGrow(gap), WithBasis(0)), band, Col(nil, WithGrow(gap), WithBasis(0)),
	}, tokBackground("bg")))
}

// reportCoverShare は表紙・終わりのスライドの箱の高さ(ページの高さに対する比)を、箱の
// 中身 box(幅はページの reportCoverBoxW)の要る高さから決める(#104)。箱は
// reportCoverBoxH を下限に中身が要るだけ伸ばし、上下の空き reportCoverMinGap は残す
// (#109)。組み立ての途中で値の表を引いて測る(解決は何度してもよい)。
func reportCoverShare(t *Template, box *Spec) (float64, error) {
	if t.Fonts != nil {
		defer useFonts(t.Fonts)()
	}
	W, H := t.Values.PageW, t.Values.PageH
	if err := resolveSpec(t, box); err != nil {
		return 0, err
	}
	need := heightForWidth(box, W*reportCoverBoxW) / H
	return math.Max(reportCoverBoxH, math.Min(1-2*reportCoverMinGap, need)), nil
}

// reportCover は report の表紙:全面に写真(meta.coverImage)を敷き、縦の中ほど・左の
// 基調色の箱(幅 0.53・高さは 0.45 を下限に中身が要るだけ)の下寄せに白い字で題・副題・
// 作者・日付・機密区分・免責を書く(#109。写真の上に字を置かない)。写真が無ければ地の
// 色の上に同じ形。
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
	head := []*Spec{}
	if title != nil {
		head = append(head, t.text(title, "coverTitle", "bg", true, WithPlaceholder("ctrTitle"), fromIR(titleAt)))
	}
	if subtitle != nil {
		head = append(head, t.text(subtitle, "coverSubtitle", "bg", false, WithPlaceholder("subTitle"), fromIR(subtitleAt)))
	}
	var foot []*Spec
	if author != nil {
		foot = append(foot, t.text(author, "coverAuthor", "bg", false, fromIR(authorAt)))
	}
	if d.Meta != nil && d.Meta.Date != "" {
		foot = append(foot, TextNode(t.Marks.formatDate(d.Meta.Date), tokSize("coverAuthor"), tokColor("bg"), fromIR("/meta/date")))
	}
	if d.Meta != nil && d.Meta.Confidential != nil {
		foot = append(foot, t.text(d.Meta.Confidential, "small", "bg", true, fromIR("/meta/confidential")))
	}
	if d.Meta != nil && d.Meta.Disclaimer != nil {
		foot = append(foot, t.text(d.Meta.Disclaimer, "small", "bg", false, fromIR("/meta/disclaimer")))
	}
	inBox := []*Spec{Col(nil, WithGrow(1))}
	if len(head) > 0 {
		inBox = append(inBox, Col(head, tokGap("itemGap")))
	}
	if len(foot) > 0 {
		inBox = append(inBox, Col(foot, tokGap("itemGap")))
	}
	box := Col(inBox, tokGap("sectionGap"), tokPad("coverPadding"))
	share, err := reportCoverShare(t, box)
	if err != nil {
		return nil, err
	}
	return reportCoverPage(ctx, box, share)
}

// reportClosing は report の終わりのスライド:表紙と同じ写真に、同じ位置の基調色の箱。
// 文(meta.closing の文)があれば箱の下寄せに表紙の題と同じ白い太字で置き、箱は文が
// 要るだけ伸ばす(#104)。文が無ければ箱だけ。
func reportClosing(ctx skeletonContext) (*Spec, error) {
	t, d := ctx.t, ctx.deck
	inBox := []*Spec{Col(nil, WithGrow(1))}
	if d.Meta != nil && d.Meta.Closing.Text != nil {
		inBox = append(inBox, t.text(d.Meta.Closing.Text, "coverTitle", "bg", true, fromIR("/meta/closing")))
	}
	box := Col(inBox, tokPad("coverPadding"))
	share, err := reportCoverShare(t, box)
	if err != nil {
		return nil, err
	}
	return reportCoverPage(ctx, box, share)
}

// reportContentHeader は report の本文のスライドの頭:上にリード文(上の見出し)、
// 下に題(話題)。リードが無ければ題が上の見出しの位置と大きさを取る。
func (t *Template) reportContentHeader(s *IRSlide) *Spec {
	if s.Lead == nil {
		return t.text(s.Title, "leadText", "accent", false, WithPlaceholder("title"), fromIR("title"))
	}
	// リードは標準の太さ(#100。元の資料も太字でない。太字は字幅が広がり折れやすい)。
	lead := t.prose(s.Lead, "leadText", "accent", false, fromIR("lead"))
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

// reportPairMarkSize は report の topics の丸い印の一辺(段の刻み。24pt)。pairs の印は
// 値の表の pairMarkSize(normal は同じ 24pt。dense で詰める、#136)。
var reportPairMarkSize = spaceStep(12)

// reportPairLabelWidth は report の対の名前の見出しの列の幅(段の刻み。80pt。本文の字で
// 5〜6字。長い名前は列の中で折り返す)。
var reportPairLabelWidth = spaceStep(40)

// reportPairRowGap は report の topics の塊と罫線の間の空き(段の刻み。4pt)。pairs は
// 値の表の pairsRowGap(normal は同じ 4pt、#136)で、対の行と罫線の間の空き(#103)。罫線の上と
// 下に取るので、対の行の間は 4 + 0.75 + 4 ≈ 8.75pt(元の資料の目安 8〜12pt)。
var reportPairRowGap = spaceStep(2)

// reportPairsStyle は report の pairs の見た目。側の見出しは帯(面)を置かず、基調色の
// 太字の本文1行で書く(#103。元の資料はただの1行)。
var reportPairsStyle = pairsStyle{
	headRole: "accent",
	sideTitle: func(t *Template, title Inline) *Spec {
		return t.text(title, "body", "accent", true)
	},
	rowGap: "pairsRowGap", // 値の表(normal は reportPairRowGap と同じ 4pt。dense で詰める、#136)
	mark: func(t *Template, k int, label string) *Spec {
		if label == "" {
			label = pairLetter(k)
		}
		return SubsystemNode(&pairMark{kind: pairMarkLetter, letter: label, fontSize: t.Values.Body, smallSize: t.Values.Small},
			tokWidth("pairMarkSize"), tokHeight("pairMarkSize"))
	},
	// 2字までの対の名前(要因の番号など)は A・B・C の代わりに印の中に、3字以上は
	// 行の左の見出しの列(基調色の太字。#118)に書く。
	markLabelMax: pairLabelMarkMax,
	labelColumn: func(t *Template, label string) *Spec {
		return Col([]*Spec{t.text(plainInline(label), "body", "accent", true)}, tokWidth(reportPairLabelWidth), noShrink())
	},
	between: func(t *Template) *Spec {
		return SubsystemNode(&pairMark{kind: pairMarkChevron},
			tokWidth("pairMarkSize"), tokHeight("pairMarkSize"))
	},
	rule: func(t *Template) *Spec {
		return Col(nil, tokHeight("tableRuleWidth"), tokBackground("grid"))
	},
	spacer: func(t *Template, mark *Spec) *Spec {
		return Col(nil, tokWidth(mark.tok.width))
	},
}

// reportTopicsStyle は report の topics の見た目:左に丸い印、基調色の題、塊の間の罫線。
// 空きは対の並びと同じ(罫線の上と下に 4pt)。
var reportTopicsStyle = topicsStyle{
	titleRole: "accent",
	mark: func(t *Template, label string) *Spec {
		size := t.Values.Body
		if utf8.RuneCountInString(label) > 1 {
			size = t.Values.Small
		}
		return SubsystemNode(&pairMark{kind: pairMarkRound, letter: label, fontSize: size},
			tokWidth(reportPairMarkSize), tokHeight(reportPairMarkSize))
	},
	rule: func(t *Template) *Spec {
		return Col(nil, tokHeight("tableRuleWidth"), tokBackground("grid"))
	},
	rowGap: reportPairRowGap,
}

// pairLetter は k 番目(0 から)の対の印の文字(A・B…Z・AA・AB…)。
func pairLetter(k int) string {
	s := ""
	for k++; k > 0; k = (k - 1) / 26 {
		s = string(rune('A'+(k-1)%26)) + s
	}
	return s
}

// pairMarkFill は印の文字が印の幅に占めてよい割合(越えれば小さい字にする)。
const pairMarkFill = 0.85

type pairMarkKind int

const (
	pairMarkLetter  pairMarkKind = iota // 四角い地(基調色)に白い字の A・B・C
	pairMarkChevron                     // 丸い地(基調色)に白い山形(側の間の流れ)
	pairMarkRound                       // 丸い地(基調色)に白い字の印(topics の塊の印)
)

// pairMark は report の対の印の葉。大きさは箱の幅・高さ(トークン)で決まり、
// 自然な大きさは申告しない。色は描くときに役割名で引く。
type pairMark struct {
	kind     pairMarkKind
	letter   string
	fontSize float64
	// smallSize は、印の文字(2字の対の名前)が fontSize では印の幅に入らないときの大きさ。
	smallSize float64
}

func (m *pairMark) Name() string {
	switch m.kind {
	case pairMarkLetter:
		return "pairLetter"
	case pairMarkRound:
		return "topicLabel"
	}
	return "pairArrow"
}

func (m *pairMark) Measure(Constraint) Measurement { return Measurement{} }

func (m *pairMark) ornament() {}

func (m *pairMark) Render(ctx RenderContext, box LBox) {
	fill, on := ctx.Roles.Color("accent"), ctx.Roles.Color("bg")
	if m.kind == pairMarkLetter {
		size := m.fontSize
		if m.smallSize > 0 && runWidthPt(m.letter, size, true) > box.Width*pairMarkFill {
			size = m.smallSize
		}
		ctx.Out.AddShape(box, ShapeOpts{Prst: "rect", Fill: fill,
			Runs: []TextRun{{Text: m.letter, Color: on, Bold: true}}, FontSize: size})
		return
	}
	if m.kind == pairMarkRound {
		ctx.Out.AddShape(box, ShapeOpts{Prst: "ellipse", Fill: fill,
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
// 行の群の名前は縦書きの細い帯(#100。a:tcPr の vert="eaVert")。
var reportTableLook = tableLook{headerText: "accent", headerRule: "accent", rule: "grid",
	groupFill: "support", groupText: "bg", groupVertical: true}

// reportImageLook は report の図(image・gallery)の説明の文:図の上に、基調色で(#100)。
var reportImageLook = imageLook{captionRole: "accent", captionAbove: true}

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
	// 人の印は縮めない(縮めた箱に印を描くと、下の出どころの文と重なる)。
	person := []*Spec{SubsystemNode(&calloutMark{kind: calloutPerson, size: size}, noShrink())}
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
