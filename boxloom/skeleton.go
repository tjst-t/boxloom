// 骨組みの部品の登録簿(docs/layers.md の2.3・8.4)。骨組みはスライドの種類ごとの
// 箱の木(表紙・本文の頭と並べ方・区切り・主張)と、その中の共通の枠(フッター・
// 出典の行)。部品の登録簿(component.go)と同じ作りで、コアは標準の表
// (defaultSkeleton)を持ち、テンプレートは Clone した表で名前ごとに override する。
//
// 骨組みの部品も組み方と同じトークンの決まり(#68)に従う:値の表の名前・段の刻み・
// 役割名だけを書き、数と色への解決は resolveSpec の1か所でする。
//
// コアが骨組みの木から読むのは印だけ:
//   - 本文の箱(markFrame(frameBody)。contentFrame が根の直接の子に付ける)→
//     容量・はみ出し・埋まり具合の測り。
//   - ページに収まるべき箱(markFrame(frameMain)。section・statement)→ はみ出しの測り。
//   - 題の葉(WithPlaceholder("title")。本文のスライド)→ 1行に収まるかの測り。
//
// 題・副題を PowerPoint のプレースホルダーに書く決まり(#1301886)は、部品が文字の葉に
// WithPlaceholder("title" | "ctrTitle" | "subTitle") を付けて保つ。
// 当面は package の中だけで使う(公開 API は増やさない。8.3)。

package boxloom

import (
	"fmt"
	"sort"
)

// 骨組みの部品の名前。
const (
	skelCover         = "cover"         // 表紙のスライド全体
	skelContentHeader = "contentHeader" // 本文のスライドの頭(題・リード)
	skelContentFrame  = "contentFrame"  // 本文の頭・本文・出典・フッターの並べ方
	skelSection       = "section"       // 区切りのスライド
	skelStatement     = "statement"     // 主張のスライド
	skelFooter        = "footer"        // 頁番号・機密区分
	skelSource        = "source"        // 出典の行
	skelToc           = "toc"           // 目次のスライド(meta.toc。toc.go)
	skelClosing       = "closing"       // 終わりのスライド(meta.closing。closing.go)
)

// skeletonNames は骨組みの部品の名前の一覧(override できるのはこれだけ)。
var skeletonNames = []string{skelCover, skelContentHeader, skelContentFrame, skelSection, skelStatement, skelFooter, skelSource, skelToc, skelClosing}

// frameMark は骨組みの枠の印(Spec.frame)。
type frameMark uint8

const (
	frameNone frameMark = iota
	frameBody           // 本文の箱
	frameMain           // 本文を持たないスライドでページに収まるべき箱
)

// skeletonContext は骨組みの部品に渡すもの:テンプレート・文書・スライドの番号と
// スライド、ほかの骨組みの部品を組む口(part)、本文のブロックを組む口(blocks)。
// 目次のスライド(toc.go)・終わりのスライド(closing.go)は IR に無いので index は
// tocSlide・closingSlide、slide は組み立てで作ったもの。
type skeletonContext struct {
	t     *Template
	deck  *IRDeck
	index int
	slide *IRSlide
	st    *skeletonState
	// pages は組み立てたあとの頁の並び(deckPages)、page はこのスライドの頁(0 から)。
	pages []int
	page  int
}

// tocEntries は目次に載せる区切り(level 1・2)と頁番号。
func (ctx skeletonContext) tocEntries() []tocEntry { return tocEntries(ctx.deck, ctx.pages) }

// skeletonState は1枚の組み立ての中で部品どうしが分け合うもの。
type skeletonState struct {
	warns  []string
	blocks []*Spec
	built  bool
}

// part は名前の骨組みの部品をこのテンプレートの表で組む(nil を返す部品もある。
// 出典の無いスライドの source など)。
func (ctx skeletonContext) part(name string) (*Spec, error) {
	return ctx.t.skeleton.build(name, ctx)
}

// blocks は本文のブロックを部品の登録簿で組んだもの(1枚の中で1度だけ組む)。
// 組むときの警告は BuiltSlide.Warnings に入る。
func (ctx skeletonContext) blocks() ([]*Spec, error) {
	if ctx.st.built {
		return ctx.st.blocks, nil
	}
	cc := componentContext{t: ctx.t, warn: func(w string) { ctx.st.warns = append(ctx.st.warns, w) }}
	var out []*Spec
	for bi := range ctx.slide.Body {
		b, err := ctx.t.buildBlockIn(cc.within(fmt.Sprintf("body[%d]", bi)), &ctx.slide.Body[bi])
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", bi, err)
		}
		out = append(out, b)
	}
	ctx.st.blocks, ctx.st.built = out, true
	return out, nil
}

// skeletonPart は骨組みの部品の組み方。
type skeletonPart interface {
	build(ctx skeletonContext) (*Spec, error)
}

// skeletonFunc は関数の組み方。
type skeletonFunc func(ctx skeletonContext) (*Spec, error)

func (f skeletonFunc) build(ctx skeletonContext) (*Spec, error) { return f(ctx) }

// skeletonRegistry は「骨組みの部品の名前 → 組み方」の表。
type skeletonRegistry struct {
	parts map[string]skeletonPart
}

func isSkeletonName(name string) bool {
	for _, n := range skeletonNames {
		if n == name {
			return true
		}
	}
	return false
}

// override は名前の部品の組み方を差し替える。知らない名前は誤り(打ち間違いを
// 黙って通さない。骨組みの部品は足せない)。
func (r *skeletonRegistry) override(name string, p skeletonPart) error {
	if !isSkeletonName(name) {
		return fmt.Errorf("cannot override unknown skeleton part %q (known: %v)", name, skeletonNames)
	}
	if p == nil {
		return fmt.Errorf("skeleton part %q: nil", name)
	}
	r.parts[name] = p
	return nil
}

func (r *skeletonRegistry) build(name string, ctx skeletonContext) (*Spec, error) {
	p, ok := r.parts[name]
	if !ok {
		return nil, fmt.Errorf("template has no skeleton part %q", name)
	}
	// 誤りの文は部品の名前を足さない(本文のブロックの誤りは IR の場所 body[K]: …
	// だけで書く。作者は骨組みの部品の名前を知らない)。
	return p.build(ctx)
}

// clone はテンプレートごとに表を分けるために写しを作る。
func (r *skeletonRegistry) clone() *skeletonRegistry {
	c := &skeletonRegistry{parts: map[string]skeletonPart{}}
	for k, v := range r.parts {
		c.parts[k] = v
	}
	return c
}

func (r *skeletonRegistry) names() []string {
	ns := make([]string, 0, len(r.parts))
	for k := range r.parts {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	return ns
}

// defaultSkeleton は標準の骨組みの表(組み方は template.go のもの)。
func defaultSkeleton() *skeletonRegistry {
	r := &skeletonRegistry{parts: map[string]skeletonPart{
		skelCover: skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
			// 表紙の写真(meta.coverImage)があれば全面に敷き、その上に今の見た目を重ねる。
			return ctx.withCoverImage(ctx.t.cover(ctx.deck, ctx.slide))
		}),
		skelClosing:       skeletonFunc(closingFrame),
		skelContentHeader: skeletonFunc(func(ctx skeletonContext) (*Spec, error) { return ctx.t.contentHeader(ctx.slide), nil }),
		skelContentFrame:  skeletonFunc(contentFrame),
		skelSection:       skeletonFunc(func(ctx skeletonContext) (*Spec, error) { return ctx.t.section(ctx.deck, ctx.index, ctx.slide), nil }),
		skelStatement:     skeletonFunc(statementFrame),
		skelFooter:        skeletonFunc(func(ctx skeletonContext) (*Spec, error) { return ctx.t.footer(ctx.deck, ctx.page, len(ctx.pages)), nil }),
		skelToc:           skeletonFunc(tocFrame),
		skelSource: skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
			if ctx.slide.Source == nil {
				return nil, nil
			}
			return apply(ctx.t.slideSource(ctx.slide.Source), []SpecOpt{fromIR("source")}), nil
		}),
	}}
	for _, n := range skeletonNames {
		if _, ok := r.parts[n]; !ok {
			panic("defaultSkeleton: missing " + n)
		}
	}
	return r
}

// standardSkeleton は標準の骨組みの表の元(一度だけ作り、テンプレートは写しを持つ)。
var standardSkeleton = defaultSkeleton()

// buildSkeleton は i 枚目を骨組みの表で組み、印から本文の箱・ページに収まるべき箱・
// 題の葉を見つける(トークンのまま。解決は buildSlide)。
func (t *Template) buildSkeleton(d *IRDeck, i int) (*BuiltSlide, error) {
	pages := deckPages(d)
	var ctx skeletonContext
	switch i {
	case tocSlide:
		ctx = t.tocContext(d, pages)
	case closingSlide:
		ctx = t.closingContext(d, pages)
	default:
		ctx = skeletonContext{t: t, deck: d, index: i, slide: &d.Slides[i], st: &skeletonState{}, pages: pages, page: pageIndex(pages, i)}
	}
	s := ctx.slide
	var name, mainWhat string
	switch s.Type {
	case "cover":
		name = skelCover
	case "content":
		name = skelContentFrame
	case "section":
		name, mainWhat = skelSection, "section title"
	case "statement":
		name, mainWhat = skelStatement, "statement"
	case "toc":
		name, mainWhat = skelToc, "table of contents"
	case "closing":
		name = skelClosing
	default:
		return nil, fmt.Errorf("unknown slide type %q", s.Type)
	}
	root, err := ctx.part(name)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("%s: no slide", name)
	}
	out := &BuiltSlide{Root: root, Notes: s.Notes, Warnings: ctx.st.warns, SlideIndex: i}
	bodies, mains := framesOf(root)
	switch s.Type {
	case "content":
		if len(bodies) != 1 || len(mains) != 0 {
			return nil, fmt.Errorf("%s: a content slide needs exactly one body box (markFrame(frameBody)) and no main box; found %d and %d", name, len(bodies), len(mains))
		}
		out.Body = bodies[0]
		out.Title = placeholderLeaf(root, "title")
		out.Lead = irLeaf(root, "lead")
	case "section", "statement", "toc":
		if len(bodies) != 0 || len(mains) != 1 {
			return nil, fmt.Errorf("%s: a %s slide needs exactly one main box (markFrame(frameMain)) and no body box; found %d and %d", name, s.Type, len(mains), len(bodies))
		}
		out.Main, out.MainWhat = mains[0], mainWhat
	default:
		if len(bodies)+len(mains) != 0 {
			return nil, fmt.Errorf("%s: a %s slide has no body or main box", name, s.Type)
		}
	}
	// 本文の領域は根の内側から本文の箱以外の子を引いて測る(measureBody)ので、
	// 印の付いた箱は縦に並ぶ根の直接の子でなければならない。
	for _, f := range append(bodies, mains...) {
		if !isChildOf(root, f) {
			return nil, fmt.Errorf("%s: the body/main box must be a direct child of the slide's root column", name)
		}
	}
	return out, nil
}

// framesOf は木の中の枠の印の付いた箱を集める。
func framesOf(root *Spec) (bodies, mains []*Spec) {
	var walk func(s *Spec)
	walk = func(s *Spec) {
		switch s.frame {
		case frameBody:
			bodies = append(bodies, s)
		case frameMain:
			mains = append(mains, s)
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(root)
	return
}

// placeholderLeaf は木の中で最初に見つかる、プレースホルダーの種類 ph の文字の葉。
func placeholderLeaf(root *Spec, ph string) *Spec {
	if root.Kind == KindText && root.Placeholder == ph {
		return root
	}
	for _, c := range root.Children {
		if s := placeholderLeaf(c, ph); s != nil {
			return s
		}
	}
	return nil
}

// irLeaf は IR の場所(fromIR)が at の文字の葉を探す(リード文など)。無ければ nil。
func irLeaf(root *Spec, at string) *Spec {
	if root.Kind == KindText && root.irPath == at {
		return root
	}
	for _, c := range root.Children {
		if s := irLeaf(c, at); s != nil {
			return s
		}
	}
	return nil
}

func isChildOf(root, s *Spec) bool {
	if root.Kind != KindCol {
		return false
	}
	for _, c := range root.Children {
		if c == s {
			return true
		}
	}
	return false
}
