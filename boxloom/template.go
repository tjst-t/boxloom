// テンプレート: IR(意味)を Box の木(Spec)に組む。
//
// 三層の分け方(CLAUDE.md)のうち「テンプレート=方針」にあたる。
//   - 値の表(Theme の色・TemplateValues の大きさ・TemplateMarks の印)は
//     データ(templates/<名前>.json。読み込みは values.go)。
//   - スライドの骨組み(表紙・本文)とブロックの部品(箇条・比較・注記)は
//     ここのコード。
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
}

// BuildDeck は IR の全スライドを組む。誤りは IR の中の場所を添えて返す。
func (t *Template) BuildDeck(d *IRDeck) ([]*BuiltSlide, error) {
	// 文字はこのテンプレートの字体で測る(fonts.go の activeFonts)。
	if t.Fonts != nil {
		activeFonts = t.Fonts
	}
	var out []*BuiltSlide
	for i := range d.Slides {
		s, err := t.BuildSlide(d, i)
		if err != nil {
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

// BuildSlide は i 枚目を組む。
func (t *Template) BuildSlide(d *IRDeck, i int) (*BuiltSlide, error) {
	s := &d.Slides[i]
	if err := t.checkVariant(s.Variant); err != nil {
		return nil, err
	}
	switch s.Type {
	case "cover":
		return t.buildCover(d, s), nil
	case "content":
		return t.buildContent(d, i, s)
	case "section":
		return t.buildSection(s), nil
	case "statement":
		return t.buildStatement(d, i, s), nil
	}
	return nil, fmt.Errorf("unknown slide type %q", s.Type)
}

// --- 文字の装飾: 意味(em など)を見た目に変える ---

// runs は inline を見た目の塊に変える。強調はアクセントの色の太字。
func (t *Template) runs(in Inline, color string, bold bool) []TextRun {
	var out []TextRun
	var walk func(in Inline, color string, bold bool)
	walk = func(in Inline, color string, bold bool) {
		for _, p := range in {
			switch p.Type {
			case "":
				out = append(out, TextRun{Text: p.Text, Color: color, Bold: bold})
			case "br":
				out = append(out, TextRun{Text: "\n", Color: color, Bold: bold})
			case "code":
				out = append(out, TextRun{Text: p.Text, Color: t.Theme.c("text_muted"), Bold: bold})
			case "em":
				walk(p.Content, t.Theme.c("accent"), true)
			case "link":
				walk(p.Content, t.Theme.c("accent"), bold)
			}
		}
	}
	walk(in, color, bold)
	return out
}

func (t *Template) text(in Inline, size float64, color string, bold bool, opts ...SpecOpt) *Spec {
	return RichTextNode(t.runs(in, color, bold), append([]SpecOpt{WithFontSize(size)}, opts...)...)
}

func plainInline(s string) Inline { return Inline{{Text: s}} }

// --- 骨組み: 表紙 ---

func (t *Template) buildCover(d *IRDeck, s *IRSlide) *BuiltSlide {
	v, th := t.Values, t.Theme
	title, subtitle, author := s.Title, s.Subtitle, s.Author
	if d.Meta != nil {
		if title == nil {
			title = d.Meta.Title
		}
		if subtitle == nil {
			subtitle = d.Meta.Subtitle
		}
		if author == nil {
			author = d.Meta.Author
		}
	}
	var kids []*Spec
	kids = append(kids, Col(nil, WithGrow(1))) // 上の余白
	if title != nil {
		kids = append(kids, t.text(title, v.CoverTitle, th.c("text_primary"), true, WithPlaceholder("ctrTitle")))
	}
	kids = append(kids, Col(nil, WithWidth(v.CoverRuleW), WithHeight(v.CoverRuleH), WithBackground(th.c("accent"))))
	if subtitle != nil {
		kids = append(kids, t.text(subtitle, v.CoverSubtitle, th.c("text_muted"), false, WithPlaceholder("subTitle")))
	}
	kids = append(kids, Col(nil, WithGrow(2))) // 下の余白(題を少し上に置く)
	if author != nil {
		kids = append(kids, t.text(author, v.CoverAuthor, th.c("text_muted"), false))
	}
	root := Col(kids, WithGap(v.SectionGap), WithPadding(v.CoverPadding), WithBackground(th.c("bg")))
	return &BuiltSlide{Root: root, Notes: s.Notes}
}

// --- 骨組み: 本文のスライド(題 / 本文 / フッター) ---

func (t *Template) buildContent(d *IRDeck, i int, s *IRSlide) (*BuiltSlide, error) {
	v, th := t.Values, t.Theme
	var blocks []*Spec
	for bi := range s.Body {
		b, err := t.buildBlock(&s.Body[bi])
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", bi, err)
		}
		blocks = append(blocks, b)
	}
	body := Col(blocks, WithGap(v.BlockGap), WithGrow(1))

	root := Col([]*Spec{
		t.text(s.Title, v.SlideTitle, th.c("text_primary"), true, WithPlaceholder("title")),
		body,
		t.footer(d, i),
	}, WithGap(v.SectionGap), WithPadding(v.PagePadding), WithBackground(th.c("bg")))
	return &BuiltSlide{Root: root, Body: body, Notes: s.Notes}, nil
}

// footer は本文のスライドの下の行(作者と「何枚目 / 全部」)。
func (t *Template) footer(d *IRDeck, i int) *Spec {
	v, th := t.Values, t.Theme
	left := ""
	if d.Meta != nil && d.Meta.Author != nil {
		left = d.Meta.Author.Plain()
	}
	return Row([]*Spec{
		TextNode(left, WithFontSize(v.Small), WithColor(th.c("text_muted"))),
		TextNode(fmt.Sprintf("%d / %d", i+1, len(d.Slides)), WithFontSize(v.Small), WithColor(th.c("text_muted")), WithAlign("right")),
	}, WithJustify(JustifySpaceBetween))
}

// accentRule は表紙の題の下と同じアクセントの短い線。
func (t *Template) accentRule() *Spec {
	v := t.Values
	return Col(nil, WithWidth(v.CoverRuleW), WithHeight(v.CoverRuleH), WithBackground(t.Theme.c("accent")))
}

// --- 骨組み: 区切り(章の変わり目) ---

// buildSection は章の題だけのスライド。背景を一段違う色(card_alt)にして本文の
// スライドと見分け、アクセントの線の下に大きな太字の章の題を左に寄せて置く。
// 題は中ほどより少し下(上の余白 3 : 下の余白 2)。フッターは置かない(表紙と同じく
// 流れの外のページ)。
func (t *Template) buildSection(s *IRSlide) *BuiltSlide {
	v, th := t.Values, t.Theme
	main := Col([]*Spec{
		Col(nil, WithGrow(3)),
		t.accentRule(),
		t.text(s.Title, v.SectionTitle, th.c("text_primary"), true, WithPlaceholder("title")),
		Col(nil, WithGrow(2)),
	}, WithGap(v.SectionGap), WithGrow(1))
	root := Col([]*Spec{main}, WithPadding(v.CoverPadding), WithBackground(th.c("card_alt")))
	return &BuiltSlide{Root: root, Main: main, MainWhat: "section title", Notes: s.Notes}
}

// --- 骨組み: 主張(言うことが1文だけ) ---

// buildStatement は1文だけのスライド。本文のスライドと同じ背景とフッターを持ち、
// 文は本文の外周よりさらに内側(statementInset)に、アクセントの線の下に大きな字で
// 左に寄せた太字で置く(強調の部分はアクセントの色)。
// 文は中ほどより少し上(上の余白 2 : 下の余白 3)。
func (t *Template) buildStatement(d *IRDeck, i int, s *IRSlide) *BuiltSlide {
	v, th := t.Values, t.Theme
	main := Col([]*Spec{
		Col(nil, WithGrow(2)),
		t.accentRule(),
		t.text(s.Content, v.StatementText, th.c("text_primary"), true, WithPlaceholder("title")),
		Col(nil, WithGrow(3)),
	}, WithGap(v.SectionGap), WithPadding(v.StatementInset), WithGrow(1))
	root := Col([]*Spec{main, t.footer(d, i)},
		WithGap(v.SectionGap), WithPadding(v.PagePadding), WithBackground(th.c("bg")))
	return &BuiltSlide{Root: root, Main: main, MainWhat: "statement", Notes: s.Notes}
}

// --- 部品: ブロック ---

func (t *Template) buildBlock(b *IRBlock) (*Spec, error) {
	if err := t.checkVariant(b.Variant); err != nil {
		return nil, err
	}
	s, err := t.buildBlockSpec(b)
	if s != nil {
		s.irType = b.Type
	}
	return s, err
}

func (t *Template) buildBlockSpec(b *IRBlock) (*Spec, error) {
	v, th := t.Values, t.Theme
	switch b.Type {
	case "columns":
		return t.columns(b)
	case "table":
		return t.table(b), nil
	case "image":
		leaf, err := t.imageLeaf(b.Src, b.Alt, b.Caption)
		if err != nil {
			return nil, fmt.Errorf("src: %w", err)
		}
		return SubsystemNode(leaf, WithGrow(1)), nil
	case "gallery":
		return t.gallery(b)
	case "paragraph":
		return t.text(b.Content, v.Body, th.c("text_primary"), false), nil
	case "note":
		return t.text(b.Content, v.Small, th.c("text_muted"), false), nil
	case "points":
		return t.points(b.Items, 0), nil
	case "compare":
		return t.compare(b.Options), nil
	case "takeaway":
		return t.takeaway(b.Content), nil
	case "stat":
		return t.stat(b.Stats), nil
	case "quote":
		return t.quote(b.Content, b.Source), nil
	case "subsystem":
		sub, err := t.Subsystems.Build(b.Subsystem, b.Data)
		if err != nil {
			return nil, err
		}
		// 高さは自己申告(自然な高さ)が出発点で、余りは grow で受け取る。
		return SubsystemNode(sub, WithGrow(1), WithBackground(th.c("card")), WithRadius(v.SubsystemRadius)), nil
	}
	return nil, fmt.Errorf("unknown block type %q", b.Type)
}

// points は箇条。印は先頭の文字に入れる(折り返した行は印の下に戻る)。
func (t *Template) points(items []IRPointItem, depth int) *Spec {
	v, th := t.Values, t.Theme
	mark := t.Marks.Point
	size := v.Body
	if depth > 0 {
		mark, size = t.Marks.SubPoint, v.SubPoint
	}
	var kids []*Spec
	for _, it := range items {
		runs := append([]TextRun{{Text: mark, Color: th.c("accent"), Bold: true}}, t.runs(it.Content, th.c("text_primary"), false)...)
		kids = append(kids, RichTextNode(runs, WithFontSize(size)))
		if len(it.Items) > 0 {
			// 字下げ: 左に固定幅の空き + 入れ子の箇条(残りの幅をもらう)。
			kids = append(kids, Row([]*Spec{Col(nil, WithWidth(size*v.PointIndentEm)), t.points(it.Items, depth+1)}))
		}
	}
	col := Col(kids, WithGap(v.ItemGap))
	if depth > 0 {
		col.Grow, col.Basis = 1, new(float64)
	}
	return col
}

// compare は選択肢の比較。選択肢ごとのカードを同じ幅で横に並べる。
func (t *Template) compare(opts []IRCompareOption) *Spec {
	v, th := t.Values, t.Theme
	var cards []*Spec
	for _, o := range opts {
		kids := []*Spec{t.text(o.Label, v.OptionLabel, th.c("text_primary"), true)}
		for _, p := range o.Pros {
			runs := append([]TextRun{{Text: t.Marks.Pro, Color: th.c("support"), Bold: true}}, t.runs(p, th.c("text_primary"), false)...)
			kids = append(kids, RichTextNode(runs, WithFontSize(v.OptionItem)))
		}
		for _, c := range o.Cons {
			runs := append([]TextRun{{Text: t.Marks.Con, Color: th.c("danger"), Bold: true}}, t.runs(c, th.c("text_primary"), false)...)
			kids = append(kids, RichTextNode(runs, WithFontSize(v.OptionItem)))
		}
		cards = append(cards, Col(kids, WithGap(v.ItemGap), WithPadding(v.CardPadding),
			WithBackground(th.c("card")), WithRadius(v.CardRadius), WithGrow(1), WithBasis(0)))
	}
	return Row(cards, WithGap(v.BlockGap))
}

// takeaway は本文の中の結論の1文。左にアクセントの帯を置き、カードの色の上に
// 一段大きな太字で書く。
func (t *Template) takeaway(in Inline) *Spec {
	v, th := t.Values, t.Theme
	return Row([]*Spec{
		Col(nil, WithWidth(v.AccentBarWidth), WithBackground(th.c("accent"))),
		Col([]*Spec{t.text(in, v.TakeawayText, th.c("text_primary"), true)},
			WithPadding(v.CardPadding), WithBackground(th.c("card")), WithGrow(1), WithBasis(0)),
	})
}

// stat は1〜4個の数。数の数にかかわらず、同じ幅のカードを1行に横に並べる
// (1個ならカード1枚が本文の幅いっぱい)。カードの中は、大きなアクセントの色の数と
// その後ろの単位(下端をそろえる)、その下に控えめな色の説明。
func (t *Template) stat(items []IRStatItem) *Spec {
	v, th := t.Values, t.Theme
	var cards []*Spec
	for _, it := range items {
		// 数と単位は1つの文字の葉の2つの塊にする(下の線はアプリがそろえる)。
		// 葉の大きさは大きいほう(数)で、単位の塊だけ小さくする。
		num := []TextRun{{Text: it.Value, Color: th.c("accent"), Bold: true}}
		if it.Unit != "" {
			num = append(num, TextRun{Text: t.Marks.Unit + it.Unit, Color: th.c("text_primary"), Bold: true, Size: v.StatUnit})
		}
		cards = append(cards, Col([]*Spec{
			RichTextNode(num, WithFontSize(v.StatValue)),
			t.text(it.Label, v.StatLabel, th.c("text_muted"), false),
		}, WithGap(v.ItemGap), WithPadding(v.CardPadding), WithBackground(th.c("card")), WithRadius(v.CardRadius),
			WithGrow(1), WithBasis(0)))
	}
	return Row(cards, WithGap(v.BlockGap))
}

// quote は他人の言葉と出どころ。頭に大きなアクセントの色の引用の印を置き、
// その右に言葉、下に控えめな色で出どころ(頭に marks.quoteSource)。
func (t *Template) quote(content, source Inline) *Spec {
	v, th := t.Values, t.Theme
	src := append([]TextRun{{Text: t.Marks.Source, Color: th.c("text_muted")}}, t.runs(source, th.c("text_muted"), false)...)
	return Row([]*Spec{
		TextNode(t.Marks.Quote, WithFontSize(v.QuoteMark), WithColor(th.c("accent")), WithBold(true)),
		Col([]*Spec{
			t.text(content, v.QuoteText, th.c("text_primary"), false),
			RichTextNode(src, WithFontSize(v.QuoteSource)),
		}, WithGap(v.ItemGap), WithGrow(1), WithBasis(0)),
	}, WithGap(v.ItemGap))
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
	for i, b := range built {
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

		p := NewPPTXBuilder(v.PageW, v.PageH)
		p.ColorTheme = &DeckColors{Name: t.Name, Colors: t.Theme}
		roles := newCheckedRoles(t.Theme)
		renderSpec(RenderContext{Out: p, Roles: roles}, b.Root)
		if missing := roles.Missing(); len(missing) > 0 {
			warnings = append(warnings, fmt.Sprintf("[ROLE WARNING] theme has no value for roles: %v", missing))
		}
		// はみ出しは描いた後に測る(測るときに行の Box を並べ直すため)。
		if b.Body != nil {
			var kinds []string
			for _, blk := range d.Slides[i].Body {
				kinds = append(kinds, blk.Type)
			}
			// 警告と容量は同じ測り値から作る(数字が食い違わないように)。
			if blocks, area, need, ok := measureBody(b.Root, b.Body); ok {
				warnings = append(warnings, overflowWarnings(blocks, area, need, kinds)...)
				caps = append(caps, slideCapacity(blocks, area, need, b.Body.Gap, i, kinds, v.Body))
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
			all = append(all, fmt.Sprintf("slide %d: %s", i+1, w))
		}
	}
	return pages, all, caps, nil
}
