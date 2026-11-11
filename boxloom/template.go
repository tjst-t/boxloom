// テンプレート: IR(意味)を Box の木(Spec)に組む。
//
// 三層の分け方(CLAUDE.md)のうち「テンプレート=方針」にあたる。
//   - 値の表(Theme の色・TemplateValues の大きさ)はデータ。
//   - スライドの骨組み(表紙・本文)とブロックの部品(箇条・比較・注記)は
//     ここのコード。
//   - サブシステムは登録簿(Subsystems)から引く。テンプレートは Override で
//     描き方を差し替えられる。
//
// デモのテンプレート dark / light の方針: 中央揃えを使わない、SPACE_BETWEEN は
// フッターだけ、本文が少なくても引き伸ばして埋めない(足りなければ警告)。
package main

import (
	"fmt"
	"strings"
)

// TemplateValues はテンプレートの大きさの値の表(pt)。
type TemplateValues struct {
	PageW, PageH float64

	PagePadding float64 // ページの外周
	SectionGap  float64 // 見出し・本文・フッターの間
	BlockGap    float64 // 本文のブロックの間
	ItemGap     float64 // 箇条の項目の間

	CoverTitle    float64
	CoverSubtitle float64
	SlideTitle    float64
	Body          float64
	Small         float64 // 注記・フッター

	CardPadding float64
	CardRadius  float64
}

func defaultValues() TemplateValues {
	return TemplateValues{
		PageW: pageW, PageH: pageH,
		PagePadding: spaceLG, SectionGap: spaceMD, BlockGap: spaceMD, ItemGap: 6,
		CoverTitle: 36, CoverSubtitle: 16, SlideTitle: typeTitle - 6, Body: 14, Small: typeCaption,
		CardPadding: 14, CardRadius: 10,
	}
}

// Template は名前のついた方針の一式。
type Template struct {
	Name       string
	Theme      Theme
	Values     TemplateValues
	Subsystems *SubsystemRegistry
	// Variants はこのテンプレートが用意した変種の名前(IR の variant)。
	// いまのデモのテンプレートは1つも持たない。
	Variants map[string]bool
	// Blocks はこのテンプレートが名乗る囲み(:::名前)。`boxloom schema` が出す。
	Blocks []BlockDecl
}

// TemplateNames は LookupTemplate が引けるテンプレートの名前(既定は light)。
var TemplateNames = []string{"dark", "light"}

// LookupTemplate は名前からテンプレートを引く。
func LookupTemplate(name string) (*Template, error) {
	var th Theme
	switch name {
	case "dark":
		th = themeDark()
	case "light", "":
		name, th = "light", themeLight()
	default:
		return nil, fmt.Errorf("unknown template %q (known: dark, light)", name)
	}
	return &Template{Name: name, Theme: th, Values: defaultValues(), Subsystems: DefaultSubsystems().Clone(), Variants: map[string]bool{}, Blocks: defaultBlocks()}, nil
}

// BuiltSlide は1枚分の組み立ての結果。
type BuiltSlide struct {
	Root     *Spec
	Body     *Spec // 本文の箱(本文の埋まり具合を測る。表紙では nil)
	Notes    string
	Warnings []string
}

// BuildDeck は IR の全スライドを組む。誤りは IR の中の場所を添えて返す。
func (t *Template) BuildDeck(d *IRDeck) ([]*BuiltSlide, error) {
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

func (t *Template) text(in Inline, size float64, color string, bold bool) *Spec {
	return RichTextNode(t.runs(in, color, bold), WithFontSize(size))
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
		kids = append(kids, t.text(title, v.CoverTitle, th.c("text_primary"), true))
	}
	kids = append(kids, Col(nil, WithWidth(64), WithHeight(4), WithBackground(th.c("accent"))))
	if subtitle != nil {
		kids = append(kids, t.text(subtitle, v.CoverSubtitle, th.c("text_muted"), false))
	}
	kids = append(kids, Col(nil, WithGrow(2))) // 下の余白(題を少し上に置く)
	if author != nil {
		kids = append(kids, t.text(author, v.Small+1, th.c("text_muted"), false))
	}
	root := Col(kids, WithGap(v.SectionGap), WithPadding(v.PagePadding*2), WithBackground(th.c("bg")))
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

	left := ""
	if d.Meta != nil && d.Meta.Author != nil {
		left = d.Meta.Author.Plain()
	}
	footer := Row([]*Spec{
		TextNode(left, WithFontSize(v.Small), WithColor(th.c("text_muted"))),
		TextNode(fmt.Sprintf("%d / %d", i+1, len(d.Slides)), WithFontSize(v.Small), WithColor(th.c("text_muted")), WithAlign("right")),
	}, WithJustify(JustifySpaceBetween))

	root := Col([]*Spec{
		t.text(s.Title, v.SlideTitle, th.c("text_primary"), true),
		body,
		footer,
	}, WithGap(v.SectionGap), WithPadding(v.PagePadding), WithBackground(th.c("bg")))
	return &BuiltSlide{Root: root, Body: body, Notes: s.Notes}, nil
}

// --- 部品: ブロック ---

func (t *Template) buildBlock(b *IRBlock) (*Spec, error) {
	if err := t.checkVariant(b.Variant); err != nil {
		return nil, err
	}
	v, th := t.Values, t.Theme
	switch b.Type {
	case "paragraph":
		return t.text(b.Content, v.Body, th.c("text_primary"), false), nil
	case "note":
		return t.text(b.Content, v.Small, th.c("text_muted"), false), nil
	case "points":
		return t.points(b.Items, 0), nil
	case "compare":
		return t.compare(b.Options), nil
	case "subsystem":
		sub, err := t.Subsystems.Build(b.Subsystem, b.Data)
		if err != nil {
			return nil, err
		}
		// 高さは自己申告(自然な高さ)が出発点で、余りは grow で受け取る。
		return SubsystemNode(sub, WithGrow(1), WithBackground(th.c("card")), WithRadius(12)), nil
	}
	return nil, fmt.Errorf("unknown block type %q", b.Type)
}

// points は箇条。印は先頭の文字に入れる(折り返した行は印の下に戻る)。
func (t *Template) points(items []IRPointItem, depth int) *Spec {
	v, th := t.Values, t.Theme
	mark := "・"
	size := v.Body
	if depth > 0 {
		mark, size = "– ", v.Body-1.5
	}
	var kids []*Spec
	for _, it := range items {
		runs := append([]TextRun{{Text: mark, Color: th.c("accent"), Bold: true}}, t.runs(it.Content, th.c("text_primary"), false)...)
		kids = append(kids, RichTextNode(runs, WithFontSize(size)))
		if len(it.Items) > 0 {
			// 字下げ: 左に固定幅の空き + 入れ子の箇条(残りの幅をもらう)。
			kids = append(kids, Row([]*Spec{Col(nil, WithWidth(size*1.5)), t.points(it.Items, depth+1)}))
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
		kids := []*Spec{t.text(o.Label, v.Body+1, th.c("text_primary"), true)}
		for _, p := range o.Pros {
			runs := append([]TextRun{{Text: "＋ ", Color: th.c("support"), Bold: true}}, t.runs(p, th.c("text_primary"), false)...)
			kids = append(kids, RichTextNode(runs, WithFontSize(v.Body-1)))
		}
		for _, c := range o.Cons {
			runs := append([]TextRun{{Text: "－ ", Color: th.c("danger"), Bold: true}}, t.runs(c, th.c("text_primary"), false)...)
			kids = append(kids, RichTextNode(runs, WithFontSize(v.Body-1)))
		}
		cards = append(cards, Col(kids, WithGap(v.ItemGap), WithPadding(v.CardPadding),
			WithBackground(th.c("card")), WithRadius(v.CardRadius), WithGrow(1), WithBasis(0)))
	}
	return Row(cards, WithGap(v.BlockGap))
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

// --- 書き出し ---

// RenderDeck は IR を組み、並べ、描いて pptx に書く。警告(本文の埋まり具合・
// サブシステムの最小の大きさ・テーマに無い役割名)は返り値と各スライドの
// ノートに入れる。
func (t *Template) RenderDeck(d *IRDeck, outPath string) ([]string, error) {
	built, err := t.BuildDeck(d)
	if err != nil {
		return nil, err
	}
	var pages []*PPTXBuilder
	var all []string
	for i, b := range built {
		v := t.Values
		computeLayout(b.Root, 0, 0, v.PageW, v.PageH)

		warnings := append([]string{}, b.Warnings...)
		if b.Body != nil {
			ratio := bodyFillRatio(b.Body)
			if w := lintFillRatio(ratio); w != "" {
				warnings = append(warnings, w)
			}
		}
		warnings = append(warnings, lintSubsystems(b.Root)...)

		p := NewPPTXBuilder(v.PageW, v.PageH)
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
			warnings = append(warnings, lintBodyOverflow(b.Root, b.Body, kinds)...)
		}

		notes := b.Notes
		if len(warnings) > 0 {
			if notes != "" {
				notes += "\n"
			}
			notes += strings.Join(warnings, "\n")
		}
		p.SetNotes(notes)
		pages = append(pages, p)
		for _, w := range warnings {
			all = append(all, fmt.Sprintf("slide %d: %s", i+1, w))
		}
	}
	title := ""
	if d.Meta != nil {
		title = d.Meta.Title.Plain()
	}
	return all, SaveDeck(outPath, pages, title)
}
