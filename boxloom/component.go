// 部品の登録簿(docs/layers.md の2.2・6章)。部品は IR の1ブロックを箱の木
// (Spec)に組む。サブシステムと違って自分では描かず、箱の木に展開されるだけ。
//
// 登録簿が持つのは組み方だけ。型の意味(名前・Schema・囲み・代わりの組み方)は
// プロセスに1つの型の語彙(typevocab.go)が持ち、登録できるのは語彙にある型だけ。
// コアは標準の部品の表(defaultComponents)を持ち、テンプレートは Clone した表で
// 標準の型の組み方を override で差し替え、標準でない型の組み方を register で足す。
// 組み方を持たない標準でない型は、語彙の代わりの組み方で標準の型に書き換えて組み、
// 警告 component-fallback を出す。
// 当面は package の中だけで使う(公開 API は増やさない)。

package boxloom

import (
	"fmt"
	"sort"
	"strings"
)

// componentContext は組み方に渡すもの:値の表・役割名・入れ子のブロックを組む口
// (いまはテンプレートそのもの)と、組むときの警告の出し口。
type componentContext struct {
	t *Template
	// warn は組むときの警告("[名前 WARNING] 文" の形)を受け取る。nil なら捨てる。
	// 入れ子のブロックを組む側(columns)は、場所(columns[C]: body[K]:)を足して渡す。
	warn func(string)
}

func (ctx componentContext) warnf(format string, args ...any) {
	if ctx.warn != nil {
		ctx.warn(fmt.Sprintf(format, args...))
	}
}

// within は入れ子の場所(例 "columns[0]: body[1]")を警告の文の頭に足す口を返す。
func (ctx componentContext) within(loc string) componentContext {
	parent := ctx.warn
	if parent == nil {
		return ctx
	}
	ctx.warn = func(w string) {
		if k := strings.Index(w, "] "); strings.HasPrefix(w, "[") && k >= 0 {
			w = w[:k] + "] " + loc + ": " + w[k+2:]
		}
		parent(w)
	}
	return ctx
}

// component は部品の組み方。
type component interface {
	// build は IR のブロックから箱の木を作る。
	build(ctx componentContext, b *IRBlock) (*Spec, error)
}

// componentFunc は関数の組み方。
type componentFunc func(ctx componentContext, b *IRBlock) (*Spec, error)

func (f componentFunc) build(ctx componentContext, b *IRBlock) (*Spec, error) { return f(ctx, b) }

// componentRegistry は「IR の type → 組み方」の表。
type componentRegistry struct {
	components map[string]component
	vocab      *typeVocabulary
}

func newComponentRegistry(vocab *typeVocabulary) *componentRegistry {
	return &componentRegistry{components: map[string]component{}, vocab: vocab}
}

// register は標準でない型の組み方を足す。語彙に無い型・標準の型(override で
// 差し替える)・すでに組み方がある型は誤り。
func (r *componentRegistry) register(name string, c component) error {
	if _, ok := r.vocab.decl(name); !ok {
		return fmt.Errorf("cannot register a component for %q: not in the type vocabulary (known: %v)", name, r.vocab.names())
	}
	if r.vocab.isStandard(name) {
		return fmt.Errorf("%q is a standard type (use override to replace its component)", name)
	}
	if _, ok := r.components[name]; ok {
		return fmt.Errorf("component %q is already registered (use override to replace)", name)
	}
	r.components[name] = c
	return nil
}

// override は標準の型の組み方を差し替える。語彙に無い型・標準でない型は誤り
// (打ち間違いを黙って通さない)。
func (r *componentRegistry) override(name string, c component) error {
	if _, ok := r.vocab.decl(name); !ok {
		return fmt.Errorf("cannot override unknown component %q (known: %v)", name, r.vocab.names())
	}
	if !r.vocab.isStandard(name) {
		return fmt.Errorf("%q is not a standard type (use register to add its component)", name)
	}
	r.components[name] = c
	return nil
}

// has はこの表が型の組み方を持つか(持たなければ代わりの組み方で組む)。
func (r *componentRegistry) has(name string) bool {
	_, ok := r.components[name]
	return ok
}

// build は IR のブロックを、その type の組み方で組む。組み方が無く、語彙が
// 代わりの組み方を持てば、標準の型に書き換えて組み、component-fallback を警告する。
func (r *componentRegistry) build(ctx componentContext, b *IRBlock) (*Spec, error) {
	if c, ok := r.components[b.Type]; ok {
		return c.build(ctx, b)
	}
	d, ok := r.vocab.decl(b.Type)
	if !ok {
		return nil, fmt.Errorf("unknown block type %q", b.Type)
	}
	if d.Fallback == nil {
		return nil, fmt.Errorf("template has no component for block type %q", b.Type)
	}
	alts, _, err := d.Fallback(b)
	if err != nil {
		return nil, fmt.Errorf("%s: fallback: %w", b.Type, err)
	}
	if len(alts) == 0 {
		return nil, fmt.Errorf("%s: fallback returned no blocks", b.Type)
	}
	var kids []*Spec
	var names []string
	for i, a := range alts {
		if !r.vocab.isStandard(a.Type) {
			return nil, fmt.Errorf("%s: fallback[%d]: %q is not a standard type", b.Type, i, a.Type)
		}
		s, err := ctx.t.buildBlockIn(ctx, a)
		if err != nil {
			return nil, fmt.Errorf("%s: fallback[%d]: %w", b.Type, i, err)
		}
		kids = append(kids, s)
		names = append(names, a.Type)
	}
	ctx.warnf("[COMPONENT-FALLBACK WARNING] template %q has no component for block type %q; built as %s (the type's fallback)",
		ctx.t.Name, b.Type, strings.Join(names, ", "))
	if len(kids) == 1 {
		return kids[0], nil
	}
	return Col(kids, tokGap("blockGap")), nil
}

// clone はテンプレートごとに表を分けるために写しを作る(語彙は共有)。
func (r *componentRegistry) clone() *componentRegistry {
	c := newComponentRegistry(r.vocab)
	for k, v := range r.components {
		c.components[k] = v
	}
	return c
}

func (r *componentRegistry) names() []string {
	ns := make([]string, 0, len(r.components))
	for k := range r.components {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	return ns
}

// defaultComponents は標準の型の組み方の表(docs/layers.md の5章:どのテンプレートも
// 必ず組める一式)。組み方は template.go・blocks.go のもの。
func defaultComponents() *componentRegistry {
	r := newComponentRegistry(blockTypes)
	reg := func(name string, f componentFunc) {
		if !blockTypes.isStandard(name) {
			panic(fmt.Sprintf("defaultComponents: %q is not a standard type", name))
		}
		r.components[name] = f
	}
	reg("paragraph", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		return t.text(b.Content, "body", "text_primary", false), nil
	})
	reg("note", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		return t.text(b.Content, "small", "text_muted", false), nil
	})
	reg("points", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.points(b.Items, 0, b.Ordered), nil
	})
	reg("compare", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.compare(b.Options), nil
	})
	reg("stat", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.stat(b.Stats), nil
	})
	reg("takeaway", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.takeaway(b.Content), nil
	})
	reg("quote", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.quote(b.Content, b.Source), nil
	})
	reg("columns", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.columns(ctx, b)
	})
	reg("table", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.table(b), nil
	})
	reg("image", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		leaf, err := ctx.t.imageLeaf(b.Src, b.Alt, b.Caption)
		if err != nil {
			return nil, fmt.Errorf("src: %w", err)
		}
		return SubsystemNode(leaf, WithGrow(1)), nil
	})
	reg("gallery", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.gallery(b)
	})
	reg("subsystem", func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		sub, err := t.Subsystems.Build(b.Subsystem, b.Data)
		if err != nil {
			return nil, err
		}
		// 高さは自己申告(自然な高さ)が出発点で、余りは grow で受け取る。
		return SubsystemNode(sub, WithGrow(1), tokBackground("card"), tokRadius("subsystemRadius")), nil
	})
	return r
}

// standardComponents は標準の部品の表の元(一度だけ作り、テンプレートは写しを持つ)。
var standardComponents = defaultComponents()

// fallbackPath は代わりの組み方が書き換えたブロックの中の1か所を、元の IR のどこに
// 写すか。To は元のブロックからの相対の IR の場所("/pairs/3/response")。Same は
// 書き換えの前後で中身が同じブロックであること:その下の場所(…/items/2 など)も
// そのまま足して写せる。Same でなければ、その下の場所も To に丸める。
type fallbackPath struct {
	To   string
	Same bool
}

// fallbackPaths は書き換えたブロックの中の場所(相対の JSON Pointer。代わりの組み方が
// ブロックを1つだけ返したときの、そのブロックからの場所)→ 元の IR の場所。
type fallbackPaths map[string]fallbackPath

// fallbackBlocks は deck の本文のブロックのうち、tm が組み方を持たず代わりの組み方で
// 組んだもの(場所 /slides/S/body/N → 場所の表)。標準でない型は本文にだけ置ける
// (まとまりの中には置けない)ので、本文だけを見る。表が無いもの・ブロックを2つ以上に
// 書き換えたものは nil(ブロックに丸める)。
func fallbackBlocks(deck *IRDeck, tm *Template) map[string]fallbackPaths {
	out := map[string]fallbackPaths{}
	if deck == nil || tm == nil || tm.components == nil {
		return out
	}
	for si := range deck.Slides {
		for bi := range deck.Slides[si].Body {
			b := &deck.Slides[si].Body[bi]
			if tm.components.has(b.Type) {
				continue
			}
			d, ok := tm.components.vocab.decl(b.Type)
			if !ok || d.Fallback == nil {
				continue
			}
			alts, paths, err := d.Fallback(b)
			if err != nil || len(alts) != 1 {
				paths = nil
			}
			out[fmt.Sprintf("/slides/%d/body/%d", si, bi)] = paths
		}
	}
	return out
}

// remapFallbackPath は、代わりの組み方で組んだブロックの中を指す場所 p を元の IR の
// 場所に写す。表に無い場所はブロックの場所に丸める。ほかの場所はそのまま返す。
func remapFallbackPath(p string, fb map[string]fallbackPaths) string {
	for bp, paths := range fb {
		if !strings.HasPrefix(p, bp+"/") {
			continue
		}
		rel := p[len(bp):]
		best := ""
		for k := range paths {
			if (rel == k || strings.HasPrefix(rel, k+"/")) && len(k) > len(best) {
				best = k
			}
		}
		if best == "" {
			return bp
		}
		v := paths[best]
		if v.Same {
			return bp + v.To + rel[len(best):]
		}
		return bp + v.To
	}
	return p
}

// remapFallbackCapacity は容量の中の場所を remapFallbackPath で写す。
func remapFallbackCapacity(caps []SlideCapacity, fb map[string]fallbackPaths) {
	if len(fb) == 0 {
		return
	}
	var block func(b *BlockCapacity)
	block = func(b *BlockCapacity) {
		b.Path = remapFallbackPath(b.Path, fb)
		for i := range b.Cards {
			b.Cards[i].Path = remapFallbackPath(b.Cards[i].Path, fb)
		}
		for i := range b.Pairs {
			b.Pairs[i].Path = remapFallbackPath(b.Pairs[i].Path, fb)
		}
		for i := range b.Topics {
			b.Topics[i].Path = remapFallbackPath(b.Topics[i].Path, fb)
		}
		for i := range b.Columns {
			c := &b.Columns[i]
			c.Path = remapFallbackPath(c.Path, fb)
			for j := range c.Blocks {
				block(&c.Blocks[j])
			}
		}
	}
	for i := range caps {
		for j := range caps[i].Blocks {
			block(&caps[i].Blocks[j])
		}
	}
}
