// 区切りの階層(section の level)と目次(meta.toc。Backlog #86)。
//
// 目次のスライドは IR に書かない。文書の頭の toc が true なら、組み立てで表紙の直後に
// 1枚足す(deckPages)。目次には level 1・2 の区切りを、目次を足したあとの頁番号つきで
// 並べる。組み方は骨組みの部品 toc(標準は tocFrame。テンプレートは名前で差し替える)。
//
// 頁番号(フッターの「何枚目 / 全部」と目次の番号)は頁の並び(deckPages)から数える。
// 警告と容量の場所(slide N・/slides/N)は IR のスライドの番号のまま(作者が直す場所)。

package boxloom

import "fmt"

// tocWarnPrefix は目次のスライドの警告の頭(check はこれを場所 /meta/toc にする)。
const tocWarnPrefix = "table of contents (meta.toc): "

// tocSlide は頁の並び(deckPages)の中の目次の印。
const tocSlide = -1

// sectionLevel は区切りの階層(書かなければ 1)。
func sectionLevel(s *IRSlide) int {
	if s.Level < 1 {
		return 1
	}
	return s.Level
}

// deckPages は組み立てたあとの頁の並び:頁ごとに IR のスライドの番号(目次は tocSlide)。
// 目次は表紙の直後(表紙が無ければ先頭)。toc が false なら IR の並びそのもの。
func deckPages(d *IRDeck) []int {
	pages := make([]int, 0, len(d.Slides)+1)
	toc := d.Meta != nil && d.Meta.Toc
	at := 0
	if len(d.Slides) > 0 && d.Slides[0].Type == "cover" {
		at = 1
	}
	for i := range d.Slides {
		if toc && i == at {
			pages = append(pages, tocSlide)
		}
		pages = append(pages, i)
	}
	if toc && at >= len(d.Slides) {
		pages = append(pages, tocSlide)
	}
	return pages
}

// pageIndex は IR のスライド i(目次は tocSlide)の頁の番号(0 から)。
func pageIndex(pages []int, i int) int {
	for p, j := range pages {
		if j == i {
			return p
		}
	}
	return i
}

// tocEntry は目次の1行。
type tocEntry struct {
	Title Inline
	Level int // 1 章・2 節
	Page  int // 組み立てたあとの頁番号(1 から)
	Slide int // IR のスライドの番号
}

// tocEntries は目次に載せる区切り(level 1・2)を頁の順に並べる。
func tocEntries(d *IRDeck, pages []int) []tocEntry {
	var out []tocEntry
	for p, i := range pages {
		if i < 0 || d.Slides[i].Type != "section" {
			continue
		}
		if lv := sectionLevel(&d.Slides[i]); lv <= 2 {
			out = append(out, tocEntry{Title: d.Slides[i].Title, Level: lv, Page: p + 1, Slide: i})
		}
	}
	return out
}

// sectionFamily は level 3 の区切り cur の親(手前で最も近い level 1・2 の区切り。
// 無ければ -1)と、同じ親の下の level 3 の区切り(兄弟。cur を含む)。
func sectionFamily(d *IRDeck, cur int) (parent int, siblings []int) {
	parent = -1
	start := 0
	for j := cur - 1; j >= 0; j-- {
		if d.Slides[j].Type == "section" && sectionLevel(&d.Slides[j]) < 3 {
			parent, start = j, j+1
			break
		}
	}
	for j := start; j < len(d.Slides); j++ {
		s := &d.Slides[j]
		if s.Type != "section" {
			continue
		}
		if sectionLevel(s) < 3 {
			if j > cur {
				break
			}
			continue
		}
		siblings = append(siblings, j)
	}
	return parent, siblings
}

// tocContext は目次のスライドを組むときの文脈(題は値の表の marks.toc)。
func (t *Template) tocContext(d *IRDeck, pages []int) skeletonContext {
	s := &IRSlide{Type: "toc", Title: plainInline(t.Marks.Toc)}
	return skeletonContext{t: t, deck: d, index: tocSlide, slide: s, st: &skeletonState{},
		pages: pages, page: pageIndex(pages, tocSlide)}
}

// tocFrame は標準の目次(骨組みの toc):題と、level 1・2 の区切りの一覧。節(level 2)は
// 字下げし、頁番号は右に寄せる。本文のスライドと同じ外枠(フッターつき)。
func tocFrame(ctx skeletonContext) (*Spec, error) {
	t := ctx.t
	title := t.text(ctx.slide.Title, "slideTitle", "text_primary", true, WithPlaceholder("title"), fromIR("/meta/toc"))
	var rows []*Spec
	for _, e := range ctx.tocEntries() {
		bold := e.Level == 1
		row := Row([]*Spec{
			t.text(e.Title, "body", "text_primary", bold, WithGrow(1), WithBasis(0)),
			TextNode(fmt.Sprint(e.Page), tokSize("body"), tokColor("text_muted"), WithAlign("right")),
		}, tokGap("blockGap"))
		if e.Level > 1 {
			row = Row([]*Spec{Col(nil, tokWidth("tocIndent")), apply(row, []SpecOpt{WithGrow(1), WithBasis(0)})})
		}
		rows = append(rows, row)
	}
	main := Col(rows, tokGap("itemGap"), WithGrow(1), markFrame(frameMain))
	return t.pageFrame(ctx, []*Spec{title, main})
}
