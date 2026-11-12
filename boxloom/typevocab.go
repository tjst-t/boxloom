// 型の語彙(docs/layers.md の6章・8.2)。IR のブロックの type が何を意味するか
// (名前・意味の形・Markdown の囲み・代わりの組み方)は、テンプレートに依らず
// プロセスに1つだけ持つ。テンプレートが持つのは組み方(component.go)だけで、
// 新しい意味は作れない。
//
// 標準の型(defaultTypes の12)はどのテンプレートも組める一式で、代わりの組み方を
// 持たない。標準でない型は registerType で足し、Schema(schema/types/<名前>.schema.json
// の写し)と代わりの組み方(標準の型の IR への書き換え)を必ず持つ。組み方を持たない
// テンプレートは、代わりの組み方で組み、警告 component-fallback を出す。
// 当面は package の中だけで使う(公開 API は増やさない)。

package boxloom

import (
	"encoding/json"
	"fmt"
	"sort"
)

// typeDecl は1つの型が名乗るもの。
type typeDecl struct {
	// Schema は IR の意味の形(Schema の断片。$id を持つ1つの文書)。標準の型は
	// IR のコアの Schema(schema/core.schema.json)が持つので空。
	Schema json.RawMessage
	// Markdown は型の囲み(:::名前)。中の囲み(compare の option など)も含む。
	// 囲みを持たない型(段落・箇条・表…)は空。
	Markdown []BlockDecl
	// Fallback は標準でない型だけが持つ:そのブロックを標準の型のブロックに
	// 書き換える(docs/layers.md の5章)。組み方を持たないテンプレートはこれで組む。
	Fallback func(b *IRBlock) ([]*IRBlock, error)
	// Validate は標準でない型の形を IR を読むときに見る(任意)。Schema と同じ決まりを
	// Go でも見て、check と build が Schema の検査なしでも誤りを場所つきで返すため。
	Validate func(b *IRBlock) error
	// readMarkdown は型の本文の囲み(Markdown の Inside の無いもの)を IR にする読み方
	// (任意)。囲みの名乗り(BlockDecl)は置き場所と中に書けるものを宣言するだけで、
	// 中身から意味の欄への写し方は型ごとのコードが持つ(docs/layers.md の8.3)。
	readMarkdown func(p *mdParser, f *mdFenceBlk, path string) any
}

// typeVocabulary は「IR の type → 型の宣言」の表。
type typeVocabulary struct {
	decls    map[string]typeDecl
	order    []string // 登録の順(誤りの文で知っている型を並べる順)
	standard map[string]bool
}

func newTypeVocabulary() *typeVocabulary {
	return &typeVocabulary{decls: map[string]typeDecl{}, standard: map[string]bool{}}
}

// add は型を足す。同じ名前がすでにあれば誤り。
func (v *typeVocabulary) add(name string, d typeDecl, standard bool) error {
	if name == "" {
		return fmt.Errorf("a block type needs a name")
	}
	if _, ok := v.decls[name]; ok {
		return fmt.Errorf("block type %q is already in the vocabulary", name)
	}
	if !standard {
		if d.Fallback == nil {
			return fmt.Errorf("block type %q: a non-standard type needs a fallback (a rewrite to standard types)", name)
		}
		if _, err := schemaID(d.Schema); err != nil {
			return fmt.Errorf("block type %q: schema: %w", name, err)
		}
	}
	v.decls[name] = d
	v.order = append(v.order, name)
	v.standard[name] = standard
	return nil
}

// remove は型を除く(試験で足した型を片付けるため)。
func (v *typeVocabulary) remove(name string) {
	delete(v.decls, name)
	delete(v.standard, name)
	for i, n := range v.order {
		if n == name {
			v.order = append(v.order[:i:i], v.order[i+1:]...)
			break
		}
	}
}

// decl は型の宣言を返す(無ければ ok=false)。
func (v *typeVocabulary) decl(name string) (typeDecl, bool) {
	d, ok := v.decls[name]
	return d, ok
}

// isStandard は標準の型か。
func (v *typeVocabulary) isStandard(name string) bool { return v.standard[name] }

// known は登録の順の名前(誤りの文に使う)。
func (v *typeVocabulary) known() []string { return append([]string{}, v.order...) }

// names は名前の順の名前。
func (v *typeVocabulary) names() []string {
	ns := v.known()
	sort.Strings(ns)
	return ns
}

// extensions は標準でない型の名前(名前の順)。
func (v *typeVocabulary) extensions() []string {
	var ns []string
	for _, n := range v.names() {
		if !v.standard[n] {
			ns = append(ns, n)
		}
	}
	return ns
}

// defaultTypes は標準の型の表(docs/layers.md の5章)。順は IR の読み込みの誤りの
// 文に出す順(paragraph, points, …, subsystem)。囲みは defaultBlocks を分け持つ。
func defaultTypes() *typeVocabulary {
	v := newTypeVocabulary()
	fences := map[string]BlockDecl{}
	for _, d := range defaultBlocks() {
		fences[d.Name] = d
	}
	markdown := func(names ...string) []BlockDecl {
		var out []BlockDecl
		for _, n := range names {
			d, ok := fences[n]
			if !ok {
				panic(fmt.Sprintf("defaultTypes: no fence %q", n))
			}
			out = append(out, d)
		}
		return out
	}
	for _, t := range []struct {
		name   string
		fences []BlockDecl
	}{
		{"paragraph", nil},
		{"points", nil},
		{"compare", markdown("compare", "option", "pros", "cons")},
		{"note", markdown("note")},
		{"takeaway", markdown("takeaway")},
		{"stat", markdown("stat", "value")},
		{"quote", markdown("quote")},
		{"columns", markdown("columns", "column")},
		{"table", nil},
		{"image", nil},
		{"gallery", nil},
		{"subsystem", nil},
	} {
		must(v.add(t.name, typeDecl{Markdown: t.fences}, true))
	}
	return v
}

// blockTypes はプロセスに1つの型の語彙。
var blockTypes = defaultTypes()

// コアが持つ標準でない型(pairs.go)。Markdown の読み方がパーサーを指すので、
// 変数の初期化の後(init)で足す。
func init() {
	must(registerType(pairsType, pairsTypeDecl()))
}

// fences は語彙のすべての型の囲み(名前の順)。テンプレートは型を隠さないので、
// どのテンプレートもこれを名乗る(docs/layers.md の8.2)。
func (v *typeVocabulary) fences() []BlockDecl {
	var out []BlockDecl
	for _, n := range v.order {
		out = append(out, v.decls[n].Markdown...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// fenceReader は本文に置ける囲み name の読み方を持つ標準でない型を探す。
func (v *typeVocabulary) fenceReader(name string) (func(p *mdParser, f *mdFenceBlk, path string) any, bool) {
	for _, n := range v.order {
		d := v.decls[n]
		for _, b := range d.Markdown {
			if b.Name == name && len(b.Inside) == 0 && d.readMarkdown != nil {
				return d.readMarkdown, true
			}
		}
	}
	return nil, false
}

// innerFence は、ほかの囲みの中にだけ置ける囲み name の宣言を探す(本文に置いた
// ときの誤りの文に使う)。
func (v *typeVocabulary) innerFence(name string) (BlockDecl, bool) {
	for _, n := range v.order {
		for _, b := range v.decls[n].Markdown {
			if b.Name == name && len(b.Inside) > 0 && !v.standard[n] {
				return b, true
			}
		}
	}
	return BlockDecl{}, false
}

// registerType は標準でない型を語彙に足す。Schema(schema/types/ の写し。
// typeSchema で読む)と Fallback は必須。
func registerType(name string, d typeDecl) error {
	return blockTypes.add(name, d, false)
}

// typeSchema は schema/types/<名前>.schema.json の写し(boxloom/schemas/types/)を
// 返す。標準でない型の Schema はここに型ごとのファイルで置く(正本は schema/)。
func typeSchema(name string) (json.RawMessage, error) {
	b, err := schemaFS.ReadFile("schemas/types/" + name + ".schema.json")
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
