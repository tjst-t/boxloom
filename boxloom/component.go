// 部品の登録簿(docs/layers.md の2.2・6章)。部品は IR の1ブロックを箱の木
// (Spec)に組む。サブシステムと違って自分では描かず、箱の木に展開されるだけ。
//
// 登録の形はサブシステムの登録簿(SubsystemRegistry)にそろえる。コアは標準の
// 部品の表(defaultComponents)を持ち、テンプレートは Clone した表の組み方を
// override で差し替えられる(名前と意味の形は替えられない)。
// 当面は package の中だけで使う(公開 API は増やさない)。

package boxloom

import (
	"encoding/json"
	"fmt"
	"sort"
)

// componentDecl は部品が名乗るもの。組み方(component)とは別に持ち、override で
// 組み方を差し替えても変わらない。
type componentDecl struct {
	// Schema は IR の意味の形(Schema の断片)。標準の部品は IR の Schema
	// (schema/)が持つので空。
	Schema json.RawMessage
	// Markdown は部品の囲み(:::名前)。中の囲み(compare の option など)も含む。
	// 囲みを持たない部品(段落・箇条・表…)は空。
	Markdown []BlockDecl
}

// componentContext は組み方に渡すもの:値の表・役割名・入れ子のブロックを組む口
// (いまはテンプレートそのもの)。
type componentContext struct {
	t *Template
}

// component は部品の組み方。
type component interface {
	// build は IR のブロックから箱の木を作る。
	build(ctx componentContext, b *IRBlock) (*Spec, error)
	// fallback は標準でない部品だけが持つ:標準の部品の IR に書き換える
	// (docs/layers.md の5章)。標準の部品は nil, nil を返す。
	fallback(b *IRBlock) ([]*IRBlock, error)
}

// componentFunc は代わりの組み方を持たない部品(標準の部品)。
type componentFunc func(ctx componentContext, b *IRBlock) (*Spec, error)

func (f componentFunc) build(ctx componentContext, b *IRBlock) (*Spec, error) { return f(ctx, b) }
func (f componentFunc) fallback(*IRBlock) ([]*IRBlock, error)                 { return nil, nil }

// componentRegistry は「IR の type → 部品」の表。
type componentRegistry struct {
	components map[string]component
	decls      map[string]componentDecl
}

func newComponentRegistry() *componentRegistry {
	return &componentRegistry{components: map[string]component{}, decls: map[string]componentDecl{}}
}

// register は部品を足す。同じ名前がすでにあれば誤り(うっかり上書きを防ぐ)。
func (r *componentRegistry) register(name string, d componentDecl, c component) error {
	if _, ok := r.components[name]; ok {
		return fmt.Errorf("component %q is already registered (use override to replace)", name)
	}
	r.components[name] = c
	r.decls[name] = d
	return nil
}

// override は既存の部品の組み方を差し替える。名乗るもの(decl)はそのまま。
// 無い名前は誤り(打ち間違いを黙って通さない)。
func (r *componentRegistry) override(name string, c component) error {
	if _, ok := r.components[name]; !ok {
		return fmt.Errorf("cannot override unknown component %q (known: %v)", name, r.names())
	}
	r.components[name] = c
	return nil
}

// decl は部品が名乗るものを返す(無ければ ok=false)。
func (r *componentRegistry) decl(name string) (componentDecl, bool) {
	d, ok := r.decls[name]
	return d, ok
}

// build は IR のブロックを、その type の部品で組む。
func (r *componentRegistry) build(ctx componentContext, b *IRBlock) (*Spec, error) {
	c, ok := r.components[b.Type]
	if !ok {
		return nil, fmt.Errorf("unknown block type %q", b.Type)
	}
	return c.build(ctx, b)
}

// clone はテンプレートごとに表を分けるために写しを作る。
func (r *componentRegistry) clone() *componentRegistry {
	c := newComponentRegistry()
	for k, v := range r.components {
		c.components[k] = v
	}
	for k, d := range r.decls {
		c.decls[k] = d
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

// defaultComponents は標準の部品の表(docs/layers.md の5章:どのテンプレートも
// 必ず組める一式)。組み方は template.go・blocks.go のもの。
func defaultComponents() *componentRegistry {
	r := newComponentRegistry()
	fences := map[string]BlockDecl{}
	for _, d := range defaultBlocks() {
		fences[d.Name] = d
	}
	markdown := func(names ...string) []BlockDecl {
		var out []BlockDecl
		for _, n := range names {
			d, ok := fences[n]
			if !ok {
				panic(fmt.Sprintf("defaultComponents: no fence %q", n))
			}
			out = append(out, d)
		}
		return out
	}
	reg := func(name string, fences []BlockDecl, f componentFunc) {
		must(r.register(name, componentDecl{Markdown: fences}, f))
	}
	reg("paragraph", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		return t.text(b.Content, t.Values.Body, t.Theme.c("text_primary"), false), nil
	})
	reg("note", markdown("note"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		return t.text(b.Content, t.Values.Small, t.Theme.c("text_muted"), false), nil
	})
	reg("points", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.points(b.Items, 0), nil
	})
	reg("compare", markdown("compare", "option", "pros", "cons"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.compare(b.Options), nil
	})
	reg("stat", markdown("stat", "value"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.stat(b.Stats), nil
	})
	reg("takeaway", markdown("takeaway"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.takeaway(b.Content), nil
	})
	reg("quote", markdown("quote"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.quote(b.Content, b.Source), nil
	})
	reg("columns", markdown("columns", "column"), func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.columns(b)
	})
	reg("table", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.table(b), nil
	})
	reg("image", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		leaf, err := ctx.t.imageLeaf(b.Src, b.Alt, b.Caption)
		if err != nil {
			return nil, fmt.Errorf("src: %w", err)
		}
		return SubsystemNode(leaf, WithGrow(1)), nil
	})
	reg("gallery", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.gallery(b)
	})
	reg("subsystem", nil, func(ctx componentContext, b *IRBlock) (*Spec, error) {
		t := ctx.t
		sub, err := t.Subsystems.Build(b.Subsystem, b.Data)
		if err != nil {
			return nil, err
		}
		// 高さは自己申告(自然な高さ)が出発点で、余りは grow で受け取る。
		return SubsystemNode(sub, WithGrow(1), WithBackground(t.Theme.c("card")), WithRadius(t.Values.SubsystemRadius)), nil
	})
	return r
}

// standardComponents は標準の部品の表の元(一度だけ作り、テンプレートは写しを持つ)。
var standardComponents = defaultComponents()
