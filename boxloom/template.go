// テンプレート: IR(意味)を Box の木(Spec)に組む。
//
// 三層の分け方(CLAUDE.md)のうち「テンプレート=方針」にあたる。
//   - 値の表(Theme の色・TemplateValues の大きさ・TemplateMarks の印)は
//     データ(templates/<名前>.json。読み込みは values.go)。
//   - スライドの骨組み(表紙・本文・区切り・主張・フッター・出典)とブロックの部品
//     (箇条・比較・注記)はここのコード。どちらも登録簿(skeleton.go・component.go)に
//     載せ、テンプレートは名前ごとに差し替えられる。
//   - サブシステムは登録簿(Subsystems)から引く。テンプレートは Override で
//     描き方を差し替えられる。
//
// デモのテンプレート dark / light の方針: 中央揃えを使わない、SPACE_BETWEEN は
// フッターだけ、本文が少なくても引き伸ばして埋めない(足りなければ警告)。

package boxloom

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// Template は名前のついた方針の一式。
type Template struct {
	Name       string
	Theme      Theme
	Scale      TemplateScale // 値の表の段階(余白の刻み・字の大きさの段)
	Values     TemplateValues
	Marks      TemplateMarks
	Fonts      *FontSet // 字体と幅の余裕(値の表の fonts)
	Subsystems *SubsystemRegistry
	// components は部品の登録簿(IR のブロック → 箱の木。component.go)。
	// 標準の部品の写しで、組み方は override で差し替えられる。
	components *componentRegistry
	// skeleton は骨組みの部品の表(表紙・本文の頭と並べ方・区切り・主張・フッター・
	// 出典。skeleton.go)。標準の骨組みの写しで、override で名前ごとに差し替えられる。
	skeleton *skeletonRegistry
	// Variants はこのテンプレートが用意した変種の名前(IR の variant)。
	// いまのデモのテンプレートは1つも持たない。
	Variants map[string]bool
	// Blocks はこのテンプレートが名乗る囲み(:::名前)。`boxloom schema` が出す。
	Blocks []BlockDecl
	// Slides はこのテンプレートが骨組みを持つスライドの種類(# 題 {type=名前})。
	// `boxloom schema` が出す。
	Slides []SlideDecl

	// images は画像のファイルを読む口(SetAssets。1回の組み立ての中で同じ画像は
	// 1度だけ読む)。
	images *imageCache
}

// SetAssets は画像(image・gallery の src)を読む fs.FS を決める。根は文書の
// フォルダ(src は文書からの相対パス)。nil なら画像は読めない(image-missing)。
func (t *Template) SetAssets(fsys fs.FS) {
	t.images = &imageCache{fsys: fsys}
}

// LookupTemplate は名前からテンプレートを引く(templates/<名前>.json を読む。
// 空の名前は light)。
func LookupTemplate(name string) (*Template, error) {
	if name == "" {
		name = "light"
	}
	return loadTemplate(name)
}

// BuiltSlide は1枚分の組み立ての結果。
type BuiltSlide struct {
	Root     *Spec
	Body     *Spec // 本文の箱(本文の埋まり具合を測る。表紙・区切り・主張では nil)
	Notes    string
	Warnings []string
	// Main は本文を持たないスライド(区切り・主張)で、ページに収まるべき箱
	// (Root の直接の子)。はみ出しだけを測る(本文の埋まり具合は測らない)。
	Main *Spec
	// MainWhat は Main がはみ出したときの警告で、何がはみ出したかの名前。
	MainWhat string
	// Title は本文のスライドの題の葉(1行に収まるかを測る。ほかでは nil)。
	Title *Spec
	// Lead は本文のスライドのリード文の葉(行の数を値の表の leadMaxLines と比べる。
	// リード文の無いスライドとほかでは nil)。
	Lead *Spec
	// SlideIndex は組んだ元の IR のスライドの番号(d.Slides の添字)。組み立てで作った
	// 目次のスライド(meta.toc)は tocSlide。
	SlideIndex int
}

// BuildDeck は IR の全スライドを組む。誤りは IR の中の場所を添えて返す。
func (t *Template) BuildDeck(d *IRDeck) ([]*BuiltSlide, error) {
	// 文字はこのテンプレートの字体で測る(fonts.go の activeFonts)。
	if t.Fonts != nil {
		activeFonts = t.Fonts
	}
	// 続きの表(continues)のひと続きは文書全体から決まる(ParseIR を通らずに
	// 作った IR もここでつなぐ)。
	if err := linkTableChains(d); err != nil {
		return nil, err
	}
	// 頁の並び(目次があれば表紙の直後に足す。toc.go)の順に組む。
	var out []*BuiltSlide
	for _, i := range deckPages(d) {
		s, err := t.buildSlide(d, i)
		if err != nil {
			if path, _, _, ok := generatedSlide(i); ok {
				return nil, fmt.Errorf("%s: %w", strings.ReplaceAll(path[1:], "/", "."), err)
			}
			return nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
		out = append(out, s)
	}
	return out, nil
}

func (t *Template) checkVariant(v string) error {
	if v != "" && !t.Variants[v] {
		return fmt.Errorf("template %q has no variant %q", t.Name, v)
	}
	return nil
}

// BuildSlide は i 枚目を組む。部品と骨組みが書いたトークン(値の表の名前・段の
// 刻み・役割名)は、組み立ての直後・計測の前にここで1度だけ数と色に直す
// (resolveSpec。token.go)。
//
// 続きの表のひと続きは文書全体から決まるので、ここでつなぎ直す(誤りの場所は
// 文書の中の場所。slides[S]: …)。
func (t *Template) BuildSlide(d *IRDeck, i int) (*BuiltSlide, error) {
	if err := linkTableChains(d); err != nil {
		return nil, err
	}
	return t.buildSlide(d, i)
}

// buildSlide は BuildSlide のうち、ひと続きをつながないもの(BuildDeck が先につなぐ)。
func (t *Template) buildSlide(d *IRDeck, i int) (*BuiltSlide, error) {
	b, err := t.buildSlideTokens(d, i)
	if err != nil {
		return nil, err
	}
	if err := resolveSpec(t, b.Root); err != nil {
		return nil, err
	}
	return b, nil
}

// buildSlideTokens は i 枚目をトークンのまま組む(解決はしない)。組むのは骨組みの
// 部品の表(skeleton.go)。
func (t *Template) buildSlideTokens(d *IRDeck, i int) (*BuiltSlide, error) {
	if _, _, _, gen := generatedSlide(i); !gen {
		if err := t.checkVariant(d.Slides[i].Variant); err != nil {
			return nil, err
		}
	}
	return t.buildSkeleton(d, i)
}

// --- 文字の装飾: 意味(em など)を見た目に変える ---

// runs は inline を見た目の塊に変える。強調はアクセントの色の太字。
// role は色の役割名(値は resolveSpec が引く)。
func (t *Template) runs(in Inline, role string, bold bool) []TextRun {
	var out []TextRun
	var walk func(in Inline, role string, bold bool)
	walk = func(in Inline, role string, bold bool) {
		for _, p := range in {
			switch p.Type {
			case "":
				out = append(out, roleRun(p.Text, role, bold))
			case "br":
				out = append(out, roleRun("\n", role, bold))
			case "code":
				out = append(out, roleRun(p.Text, "text_muted", bold))
			case "em":
				walk(p.Content, "accent", true)
			case "link":
				walk(p.Content, "accent", bold)
			}
		}
	}
	walk(in, role, bold)
	return out
}

// leafRuns は表・画像の葉の中の文字の塊。葉の中はトークンの外(サブシステムと
// 同じ)なので、色はここでテーマから引いておく。
func (t *Template) leafRuns(in Inline, role string, bold bool) []TextRun {
	runs := t.runs(in, role, bold)
	for i := range runs {
		runs[i].Color, runs[i].role = t.Theme.c(runs[i].role), ""
	}
	return runs
}

// text は文字の葉。size は字の大きさのトークン、role は色の役割名。
func (t *Template) text(in Inline, size, role string, bold bool, opts ...SpecOpt) *Spec {
	return RichTextNode(t.runs(in, role, bold), append([]SpecOpt{tokSize(size)}, opts...)...)
}

// prose は文として読む段落(リード文・ステートメント・要点・引用)。折るところを
// 文の切れ目・読点に寄せる(TextRun.Phrase。行の数は増やさない)。
func (t *Template) prose(in Inline, size, role string, bold bool, opts ...SpecOpt) *Spec {
	runs := t.runs(in, role, bold)
	for i := range runs {
		runs[i].Phrase = true
	}
	return RichTextNode(runs, append([]SpecOpt{tokSize(size)}, opts...)...)
}

func plainInline(s string) Inline { return Inline{{Text: s}} }

// --- 骨組み: 表紙 ---
//
// 骨組みの標準の部品(defaultSkeleton。skeleton.go)の組み方。テンプレートが
// 差し替えるときにも部品を作る手として使える。

// cover は表紙のスライド全体(標準の骨組みの cover)。
func (t *Template) cover(d *IRDeck, s *IRSlide) *Spec {
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
	var kids []*Spec
	kids = append(kids, Col(nil, WithGrow(1))) // 上の余白
	if title != nil {
		kids = append(kids, t.text(title, "coverTitle", "text_primary", true, WithPlaceholder("ctrTitle"), fromIR(titleAt)))
	}
	kids = append(kids, Col(nil, tokWidth("coverRuleWidth"), tokHeight("coverRuleHeight"), tokBackground("accent")))
	if subtitle != nil {
		kids = append(kids, t.text(subtitle, "coverSubtitle", "text_muted", false, WithPlaceholder("subTitle"), fromIR(subtitleAt)))
	}
	kids = append(kids, Col(nil, WithGrow(2))) // 下の余白(題を少し上に置く)
	// 下の段: 作者・日付、その下に機密区分・免責(文書の頭の値)。
	var foot []*Spec
	if author != nil {
		foot = append(foot, t.text(author, "coverAuthor", "text_muted", false, fromIR(authorAt)))
	}
	if d.Meta != nil && d.Meta.Date != "" {
		foot = append(foot, TextNode(t.Marks.formatDate(d.Meta.Date), tokSize("coverAuthor"), tokColor("text_muted"), fromIR("/meta/date")))
	}
	if d.Meta != nil && d.Meta.Confidential != nil {
		foot = append(foot, t.text(d.Meta.Confidential, "small", "text_primary", true, fromIR("/meta/confidential")))
	}
	if d.Meta != nil && d.Meta.Disclaimer != nil {
		foot = append(foot, t.text(d.Meta.Disclaimer, "small", "text_muted", false, fromIR("/meta/disclaimer")))
	}
	switch len(foot) {
	case 0:
	case 1:
		kids = append(kids, foot[0])
	default:
		kids = append(kids, Col(foot, tokGap("itemGap")))
	}
	return Col(kids, tokGap("sectionGap"), tokPad("coverPadding"), tokBackground("bg"))
}

// --- 骨組み: 本文のスライド(題 / 本文 / フッター) ---

// contentFrame は本文のスライドの並べ方(標準の骨組みの contentFrame):頭
// (contentHeader)・本文・出典(source)・フッター(footer)を縦に並べる。本文の箱に
// 印(frameBody)を付け、容量・はみ出し・埋まり具合はこの箱で測る。
func contentFrame(ctx skeletonContext) (*Spec, error) {
	blocks, err := ctx.blocks()
	if err != nil {
		return nil, err
	}
	// 本文の図が自然な大きさのままだとページに入らないとき、題・リード文・出典・
	// フッターを縮めず、先に本文の図のゆとり(自然〜最小)から返させる。
	body := Col(blocks, tokGap("blockGap"), WithGrow(1), nestedSlack(), markFrame(frameBody))
	head, err := ctx.part(skelContentHeader)
	if err != nil {
		return nil, err
	}
	// 本文が収まらないときも頭(題・リード文)は縮めない。はみ出しは本文の箱の中に
	// 閉じ、layout-overflow の警告で知らせる(#96)。
	kids := []*Spec{apply(head, []SpecOpt{noShrink()}), body}
	return ctx.t.pageFrame(ctx, kids)
}

// pageFrame は本文のスライドと主張のスライドの外枠:kids の下に出典(source)と
// フッター(footer)を足し、ページの余白と地の色の縦の箱にする。
func (t *Template) pageFrame(ctx skeletonContext, kids []*Spec) (*Spec, error) {
	src, err := ctx.part(skelSource)
	if err != nil {
		return nil, err
	}
	// 出典とフッターは本文が収まらなくても縮めない(#96)。
	if src != nil {
		kids = append(kids, apply(src, []SpecOpt{noShrink()}))
	}
	foot, err := ctx.part(skelFooter)
	if err != nil {
		return nil, err
	}
	if foot != nil {
		kids = append(kids, apply(foot, []SpecOpt{noShrink()}))
	}
	return Col(kids, tokGap("sectionGap"), tokPadXY("pagePadding", "pagePaddingY"), tokBackground("bg")), nil
}

// contentHeader は本文のスライドの頭(標準の骨組みの contentHeader):題(title の
// プレースホルダー)と、あればその下にリード文。
func (t *Template) contentHeader(s *IRSlide) *Spec {
	title := t.text(s.Title, "slideTitle", "text_primary", true, WithPlaceholder("title"), fromIR("title"))
	if s.Lead == nil {
		return title
	}
	return Col([]*Spec{title, t.leadText(s.Lead)}, tokGap("headerGap"))
}

// leadText はリード文の葉。
func (t *Template) leadText(in Inline) *Spec {
	return t.prose(in, "leadText", "text_primary", false, fromIR("lead"))
}

// footer は本文のスライドの下の行(左に作者と機密区分、右に「何枚目 / 全部」)。
// page は組み立てたあとの頁(0 から。目次を足せば IR の番号とずれる)、total は全部の頁。
// 番号はスライドの番号の置き場(sldNum)の中の番号の欄(a:fld slidenum)なので、
// PowerPoint でスライドを足す・並べ替えると振り直る。全部の枚数の欄は PowerPoint に
// 無いので「/ 全部」は組み立てたときの数の文字のまま。左の欄はフッターの置き場(ftr。
// 作者と機密区分の両方があれば作者の側)。
func (t *Template) footer(d *IRDeck, page, total int) *Spec {
	left := ""
	if d.Meta != nil && d.Meta.Author != nil {
		left = d.Meta.Author.Plain()
	}
	leftSpec := TextNode(left, tokSize("small"), tokColor("text_muted"))
	if left != "" {
		leftSpec.Placeholder = "ftr"
	}
	if d.Meta != nil && d.Meta.Confidential != nil {
		var opts []SpecOpt
		if left == "" {
			opts = append(opts, WithPlaceholder("ftr"))
		}
		conf := t.text(d.Meta.Confidential, "small", "text_primary", true, opts...)
		if left == "" {
			leftSpec = conf
		} else {
			leftSpec = Row([]*Spec{leftSpec, conf}, tokGap("blockGap"))
		}
	}
	return Row([]*Spec{
		leftSpec,
		RichTextNode([]TextRun{
			{Text: fmt.Sprint(page + 1), role: "text_muted", Field: "slidenum"},
			{Text: fmt.Sprintf(" / %d", total), role: "text_muted"},
		}, tokSize("small"), WithAlign("right"), WithPlaceholder("sldNum")),
	}, WithJustify(JustifySpaceBetween))
}

// slideSource はスライドの出典。本文の箱の外(本文の下・フッターの上)に、控えめな
// 色の小さな字で、頭に marks.slideSource を付けて置く。骨組みの名前つきの枠なので、
// 本文の領域(容量・はみ出しの測り)はこの高さを引いたものになる。
func (t *Template) slideSource(in Inline) *Spec {
	runs := append([]TextRun{{Text: t.Marks.SlideSource, role: "text_muted"}}, t.runs(in, "text_muted", false)...)
	return RichTextNode(runs, tokSize("small"))
}

// accentRule は表紙の題の下と同じアクセントの短い線。
func (t *Template) accentRule() *Spec {
	return Col(nil, tokWidth("coverRuleWidth"), tokHeight("coverRuleHeight"), tokBackground("accent"))
}

// --- 骨組み: 区切り(章の変わり目) ---

// section は章の題だけのスライド(標準の骨組みの section)。背景を一段違う色(card_alt)にして本文の
// スライドと見分け、アクセントの線の下に大きな太字の章の題を左に寄せて置く。
// 題は中ほどより少し下(上の余白 3 : 下の余白 2)。フッターは置かない(表紙と同じく
// 流れの外のページ)。
//
// 文書に区切りが2つ以上あり、値の表の sectionTracker が true なら、題の下に
// 章の一覧と現在地(sectionTracker)を置く。作者は何も書かない。
func (t *Template) section(d *IRDeck, i int, s *IRSlide) *Spec {
	kids := []*Spec{
		Col(nil, WithGrow(3)),
		t.accentRule(),
		t.text(s.Title, "sectionTitle", "text_primary", true, WithPlaceholder("title"), fromIR("title")),
	}
	if tr := t.sectionTracker(d, i); tr != nil {
		kids = append(kids, Col(nil, tokHeight("sectionGap")), tr)
	}
	kids = append(kids, Col(nil, WithGrow(2)))
	main := Col(kids, tokGap("sectionGap"), WithGrow(1), markFrame(frameMain))
	return Col([]*Spec{main}, tokPad("coverPadding"), tokBackground("card_alt"))
}

// sectionTracker は区切りのスライドに置く章の一覧。文書の区切り(type: section)の
// 題を順に縦に並べ、左に細い帯を通す。いまの章は帯をアクセントの色にして題を
// text_primary の太字に、ほかの章は帯を grid・題を text_muted にする。
// 区切りが1つだけの文書や、値の表の sectionTracker が false なら nil。
func (t *Template) sectionTracker(d *IRDeck, cur int) *Spec {
	v := t.Values
	if !v.SectionTracker {
		return nil
	}
	var secs []int
	for j := range d.Slides {
		if d.Slides[j].Type == "section" {
			secs = append(secs, j)
		}
	}
	if len(secs) < 2 {
		return nil
	}
	var items []*Spec
	for _, j := range secs {
		bar, color, bold := "grid", "text_muted", false
		if j == cur {
			bar, color, bold = "accent", "text_primary", true
		}
		items = append(items, Row([]*Spec{
			Col(nil, tokWidth("accentBarWidth"), tokBackground(bar)),
			Col([]*Spec{t.text(d.Slides[j].Title, "sectionTrackerText", color, bold)},
				tokPad("sectionTrackerPadding"), WithGrow(1), WithBasis(0)),
		}, tokGap("itemGap")))
	}
	return Col(items)
}

// --- 骨組み: 主張(言うことが1文だけ) ---

// statementFrame は1文だけのスライド(標準の骨組みの statement)。本文のスライドと
// 同じ背景・出典・フッターを持ち、文は本文の外周よりさらに内側(statementInset)に、
// アクセントの線の下に大きな字で左に寄せた太字で置く(強調の部分はアクセントの色)。
// 文は中ほどより少し上(上の余白 2 : 下の余白 3)。
func statementFrame(ctx skeletonContext) (*Spec, error) {
	t := ctx.t
	main := Col([]*Spec{
		Col(nil, WithGrow(2)),
		t.accentRule(),
		t.prose(ctx.slide.Content, "statementText", "text_primary", true, WithPlaceholder("title"), fromIR("content")),
		Col(nil, WithGrow(3)),
	}, tokGap("sectionGap"), tokPad("statementInset"), WithGrow(1), markFrame(frameMain))
	return t.pageFrame(ctx, []*Spec{main})
}

// --- 部品: ブロック ---

func (t *Template) buildBlock(b *IRBlock) (*Spec, error) {
	return t.buildBlockIn(componentContext{t: t}, b)
}

// buildBlockIn は buildBlock に、警告の出し口(ctx.warn)を添えたもの。
func (t *Template) buildBlockIn(ctx componentContext, b *IRBlock) (*Spec, error) {
	if err := t.checkVariant(b.Variant); err != nil {
		return nil, err
	}
	s, err := t.components.build(ctx, b)
	if s != nil {
		apply(s, []SpecOpt{markIRType(b.Type)})
	}
	return s, err
}

// points は箇条。印は先頭の塊(Mark)に入れ、PPTX では段落の箇条の印として
// 書く(折り返した行は印の下に戻る)。ordered なら印は番号(AutoNum。1段目は
// 値の表の orderedStyle、入れ子は orderedNestedStyle)で、番号の分の幅は
// 箇条の字下げ(pointIndentEm)。
func (t *Template) points(items []IRPointItem, depth int, ordered bool) *Spec {
	mark := t.Marks.Point
	size := "body"
	style := t.Values.OrderedStyle
	if depth > 0 {
		mark, size = t.Marks.SubPoint, "subPoint"
		style = orderedNestedStyle
	}
	var kids []*Spec
	for i, it := range items {
		head := TextRun{Text: mark, role: "accent", Bold: true, Mark: true}
		if ordered {
			head = TextRun{Text: orderedLabel(style, i+1), role: "accent", Bold: true, Mark: true,
				AutoNum: style, widthTok: "pointIndentEm"}
		}
		runs := append([]TextRun{head}, t.runs(it.Content, "text_primary", false)...)
		kids = append(kids, RichTextNode(runs, tokSize(size)))
		if len(it.Items) > 0 {
			// 字下げ: 左に固定幅の空き + 入れ子の箇条(残りの幅をもらう)。空きの幅は
			// 字の大きさの倍(pointIndentEm は em の名前なので、空きの箱の字の大きさに掛ける)。
			kids = append(kids, Row([]*Spec{Col(nil, tokSize(size), tokWidth("pointIndentEm")), t.points(it.Items, depth+1, it.Ordered)}))
		}
	}
	col := Col(kids, tokGap("itemGap"), asList())
	if depth > 0 {
		col.Grow, col.Basis = 1, new(float64)
	}
	return col
}

// compare は選択肢の比較。選択肢ごとのカードを同じ幅で横に並べる。
func (t *Template) compare(opts []IRCompareOption) *Spec {
	var cards []*Spec
	for _, o := range opts {
		kids := []*Spec{t.text(o.Label, "optionLabel", "text_primary", true)}
		for _, p := range o.Pros {
			runs := append([]TextRun{{Text: t.Marks.Pro, role: "support", Bold: true, Mark: true}}, t.runs(p, "text_primary", false)...)
			kids = append(kids, RichTextNode(runs, tokSize("optionItem")))
		}
		for _, c := range o.Cons {
			runs := append([]TextRun{{Text: t.Marks.Con, role: "danger", Bold: true, Mark: true}}, t.runs(c, "text_primary", false)...)
			kids = append(kids, RichTextNode(runs, tokSize("optionItem")))
		}
		card := Col(kids, tokGap("itemGap"), tokPad("cardPadding"),
			tokBackground("card"), tokRadius("cardRadius"), WithGrow(1), WithBasis(0), asCard())
		cards = append(cards, card)
	}
	return Row(cards, tokGap("blockGap"), measureCardRow("options"))
}

// takeaway は本文の中の結論の1文。左にアクセントの帯を置き、カードの色の上に
// 一段大きな太字で書く。
func (t *Template) takeaway(in Inline) *Spec {
	// 帯は別の図形のまま(片側だけの枠は DrawingML に無い)。
	card := Col([]*Spec{t.prose(in, "takeawayText", "text_primary", true)},
		tokPad("cardPadding"), tokBackground("card"), WithGrow(1), WithBasis(0), asCard())
	return Row([]*Spec{
		Col(nil, tokWidth("accentBarWidth"), tokBackground("accent")),
		card,
	})
}

// stat は1〜4個の数。数の数にかかわらず、同じ幅のカードを1行に横に並べる
// (1個ならカード1枚が本文の幅いっぱい)。カードの中は、大きなアクセントの色の数と
// その後ろの単位(下端をそろえる)、その下に控えめな色の説明。
func (t *Template) stat(items []IRStatItem) *Spec {
	var cards []*Spec
	for i, it := range items {
		// 数と単位は1つの文字の葉の2つの塊にする(下の線はアプリがそろえる)。
		// 葉の大きさは大きいほう(数)で、単位の塊だけ小さくする。
		// 数と単位はそれぞれ途中で折らない(Keep)。1行に入らなければ単位を次の
		// 行へ送り、数そのものが入らなければ1行のままはみ出す。どちらも警告
		// stat-value で知らせる(keepline.go)。
		num := []TextRun{{Text: it.Value, role: "accent", Bold: true, Keep: true}}
		if it.Unit != "" {
			num = append(num, TextRun{Text: t.Marks.Unit + it.Unit, role: "text_primary", Bold: true, sizeTok: "statUnit", Keep: true})
		}
		value := RichTextNode(num, tokSize("statValue"), keepLine(fmt.Sprintf("stat: items[%d]", i)))
		card := Col([]*Spec{
			value,
			t.text(it.Label, "statLabel", "text_muted", false),
		}, tokGap("itemGap"), tokPad("cardPadding"), tokBackground("card"), tokRadius("cardRadius"),
			WithGrow(1), WithBasis(0), asCard())
		cards = append(cards, card)
	}
	return Row(cards, tokGap("blockGap"), measureCardRow("items"))
}

// quote は他人の言葉と出どころ。頭に大きなアクセントの色の引用の印を置き、
// その右に言葉、下に控えめな色で出どころ(頭に marks.quoteSource)。
func (t *Template) quote(content, source Inline) *Spec {
	src := append([]TextRun{{Text: t.Marks.Source, role: "text_muted"}}, t.runs(source, "text_muted", false)...)
	return Row([]*Spec{
		TextNode(t.Marks.Quote, tokSize("quoteMark"), tokColor("accent"), WithBold(true)),
		Col([]*Spec{
			t.prose(content, "quoteText", "text_primary", false),
			RichTextNode(src, tokSize("quoteSource")),
		}, tokGap("itemGap"), WithGrow(1), WithBasis(0)),
	}, tokGap("itemGap"))
}

// --- 検査 ---

// bodyFillRatio は本文の箱のうち、ブロックが使った高さの割合。本文の箱は
// フッターを下に置くために伸びるので、ページ全体ではなく本文の箱で測る。
func bodyFillRatio(body *Spec) float64 {
	if body == nil || body.Box.Height <= 0 {
		return 1
	}
	bottom := body.Box.Y
	for _, c := range body.Children {
		if b := c.Box.Y + c.Box.Height; b > bottom {
			bottom = b
		}
	}
	return (bottom - body.Box.Y) / body.Box.Height
}

// lintBlockSubsystems はサブシステムの検査(lintSubsystems)を本文のブロックごとに
// 行い、警告の文に本文の中の場所(body[N]: …)を付ける。check はこれで警告の
// path をそのブロックにし、.md なら行を添える。本文の無いスライドは丸ごと見る。
func lintBlockSubsystems(b *BuiltSlide) []string {
	if b.Body == nil {
		return lintSubsystems(b.Root)
	}
	var out []string
	for n, c := range b.Body.Children {
		for _, w := range lintSubsystems(c) {
			if k := strings.Index(w, "] "); strings.HasPrefix(w, "[") && k >= 0 {
				w = fmt.Sprintf("%s] body[%d]: %s", w[:k], n, w[k+2:])
			}
			out = append(out, w)
		}
	}
	return out
}

// --- 書き出し ---

// RenderDeck は IR を組み、並べ、描いて pptx を w に書く。警告(本文の埋まり具合・
// サブシステムの最小の大きさ・テーマに無い役割名・はみ出し)は返り値で返し、
// ノートには入れない(ノートは作者の文だけ)。
func (t *Template) RenderDeck(d *IRDeck, w io.Writer) ([]string, error) {
	pages, all, err := t.RenderPages(d)
	if err != nil {
		return nil, err
	}
	title := ""
	if d.Meta != nil {
		title = d.Meta.Title.Plain()
	}
	return all, WriteDeck(w, pages, title)
}

// RenderPages は RenderDeck のうち、ファイルに書く前まで(組み・並べ・描いた
// スライドと警告)。字体の検査(fonts コマンド)もこれを使う。
func (t *Template) RenderPages(d *IRDeck) ([]*PPTXBuilder, []string, error) {
	pages, all, _, err := t.renderPages(d)
	return pages, all, err
}

// renderPages は RenderPages に、本文を持つスライドごとの容量(capacity.go)を
// 足したもの。check が使う。
func (t *Template) renderPages(d *IRDeck) ([]*PPTXBuilder, []string, []SlideCapacity, error) {
	built, err := t.BuildDeck(d)
	if err != nil {
		return nil, nil, nil, err
	}
	var pages []*PPTXBuilder
	var all []string
	caps := []SlideCapacity{}
	for _, b := range built {
		// 警告と容量の場所は IR のスライドの番号(目次のスライドは IR に無いので
		// 警告の頭は "table of contents"、容量は出さない)。
		i := b.SlideIndex
		v := t.Values
		computeLayout(b.Root, 0, 0, v.PageW, v.PageH)

		warnings := append([]string{}, b.Warnings...)
		if b.Body != nil {
			ratio := bodyFillRatio(b.Body)
			if w := lintFillRatioMin(ratio, v.FillRatioMin); w != "" {
				warnings = append(warnings, w)
			}
		}
		warnings = append(warnings, lintBlockSubsystems(b)...)
		if b.Title != nil {
			// 題は短い見出しで1行に収める(言いたいことの文はリード文に)。
			if n := textLinesFor(b.Title, b.Title.Box.Width); n > 1 {
				warnings = append(warnings, fmt.Sprintf("[TITLE-WRAP WARNING] title: the slide title wraps to %d lines (width %.1fpt at %.0fpt); keep it to one line",
					n, b.Title.Box.Width, b.Title.FontSize))
			}
		}
		if b.Lead != nil && v.LeadMaxLines > 0 {
			// リード文は値の表の上限の行の数まで(超えたら短くする)。
			if n := textLinesFor(b.Lead, b.Lead.Box.Width); n > v.LeadMaxLines {
				warnings = append(warnings, fmt.Sprintf("[LEAD-WRAP WARNING] lead: the lead wraps to %d lines, more than the template's limit of %d (width %.1fpt at %.0fpt); shorten it",
					n, v.LeadMaxLines, b.Lead.Box.Width, b.Lead.FontSize))
			}
		}

		p := NewPPTXBuilder(v.PageW, v.PageH)
		p.ColorTheme = &DeckColors{Name: t.Name, Colors: t.Theme}
		roles := newCheckedRoles(t.Theme)
		renderSlide(RenderContext{Out: p, Roles: roles}, b.Root)
		if missing := roles.Missing(); len(missing) > 0 {
			warnings = append(warnings, fmt.Sprintf("[ROLE WARNING] theme has no value for roles: %v", missing))
		}
		// はみ出しは描いた後に測る(測るときに行の Box を並べ直すため)。
		if b.Body != nil {
			var kinds []string
			for _, blk := range d.Slides[i].Body {
				kinds = append(kinds, blk.Type)
			}
			// 横の余りは並べたままの Box で測る(measureBody が Box を並べ直す前に)。
			spare := bodySpareWidths(b.Body)
			warnings = append(warnings, spareWidthWarnings(b.Body, spare, kinds)...)
			colSpare := columnSpareWidths(b.Body)
			warnings = append(warnings, columnSpareWarnings(b.Body, colSpare)...)
			warnings = append(warnings, pairSpareWarnings(pairSpareWidths(b.Body))...)
			warnings = append(warnings, keepLineWarnings(b.Body)...)
			// 警告と容量は同じ測り値から作る(数字が食い違わないように)。
			if blocks, area, need, ok := measureBody(b.Root, b.Body); ok {
				warnings = append(warnings, overflowWarnings(blocks, area, need, kinds)...)
				sc := slideCapacity(blocks, area, need, b.Body.Gap, i, kinds, v.Body)
				for j := range sc.Blocks {
					if w, ok := spare[j]; ok {
						sc.Blocks[j].RemainingWidth = round1(w)
					}
					for ci := range sc.Blocks[j].Columns {
						cc := &sc.Blocks[j].Columns[ci]
						for k := range cc.Blocks {
							if w, ok := colSpare[spareKey{j, ci, k}]; ok {
								cc.Blocks[k].RemainingWidth = round1(w)
							}
						}
					}
				}
				caps = append(caps, sc)
			}
		}
		if b.Main != nil {
			// 区切り・主張は本文を持たないので、ページに収まるかだけを見る(容量は出さない)。
			if _, area, need, ok := measureBody(b.Root, b.Main); ok && need > area+overflowEps {
				warnings = append(warnings, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] %s overflows the slide by %.1fpt (needs %.1fpt; available %.1fpt)",
					b.MainWhat, need-area, need, area))
			}
		}

		// ノートには作者の文だけを入れる。警告は返り値(check・build の出力)で知らせる。
		p.SetNotes(b.Notes)
		pages = append(pages, p)
		for _, w := range warnings {
			if _, _, prefix, ok := generatedSlide(i); ok {
				all = append(all, prefix+w)
				continue
			}
			all = append(all, fmt.Sprintf("slide %d: %s", i+1, w))
		}
	}
	return pages, all, caps, nil
}
