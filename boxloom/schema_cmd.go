// `boxloom schema`:いまのテンプレートとサブシステムが名乗る囲み・属性・
// サブシステムの Schema を、AI に渡せる1つの JSON にまとめて出す
// (docs/content-syntax.md の6章。出力の形は docs/schema-command.md)。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

const (
	schemaDraft   = "https://json-schema.org/draft/2020-12/schema"
	irSchemaBase  = "https://boxloom.invalid/schema/ir/0.1/"
	bundleIDBase  = irSchemaBase + "bundle/"
	schemaVersion = "0.1"
)

// SchemaOutput は `boxloom schema` の出力の根。
type SchemaOutput struct {
	Boxloom   string           `json:"boxloom"`
	About     string           `json:"about"`
	Workflow  []string         `json:"workflow"`
	Markdown  MarkdownSyntax   `json:"markdown"`
	Templates []TemplateSchema `json:"templates"`
	// IRSchema は IR(JSON)を直接書くときの JSON Schema。外を参照しない1つの
	// 文書(コアと、templates が名乗るサブシステムの Schema を $defs に同梱)。
	IRSchema json.RawMessage `json:"irSchema"`
}

// MarkdownSyntax はテンプレートに依らない記法(コアの記法)。
type MarkdownSyntax struct {
	FrontMatter     []frontMatterKey `json:"frontMatter"`
	SlideAttributes []string         `json:"slideAttributes"`
	// Blocks はテンプレートに依らない囲み(:::notes)。
	Blocks []BlockDecl      `json:"blocks"`
	Body   []bodySyntaxItem `json:"body"`
	Rules  []string         `json:"rules"`
}

// TemplateSchema は1つのテンプレートが名乗るもの。
type TemplateSchema struct {
	Name string `json:"name"`
	// Variants は variant 属性に書ける名前(スライド・囲み・サブシステムで共通)。
	Variants []string `json:"variants"`
	// Slides はスライドの種類(# 題 {type=名前} の名前。IR の slides[].type)。
	// 表紙は文書の頭から作るので入らない。
	Slides     []SlideDecl       `json:"slides"`
	Blocks     []BlockDecl       `json:"blocks"`
	Subsystems []SubsystemSchema `json:"subsystems"`
}

// SubsystemSchema は1つのサブシステムが名乗るもの。
type SubsystemSchema struct {
	Name string `json:"name"`
	// Fence は Markdown での書き出し(言語名つきのコードブロック)。
	Fence      string   `json:"fence"`
	Attributes []string `json:"attributes"`
	Markdown   []string `json:"markdown"`
	// Schema は IR のブロックの JSON Schema の場所(この出力の中の JSON Pointer)。
	// Markdown の YAML のキーは、この Schema の data と同じ。
	Schema string `json:"schema"`
	// SchemaID はその Schema の $id。
	SchemaID string `json:"schemaId"`
}

// BuildSchemaOutput は names のテンプレートが名乗るものをまとめる。
func BuildSchemaOutput(names []string) (*SchemaOutput, error) {
	var ts []*Template
	for _, name := range names {
		t, err := LookupTemplate(name)
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return buildSchemaOutput(ts)
}

// buildSchemaOutput は読んだテンプレートの一覧から一覧を作る。
func buildSchemaOutput(templates []*Template) (*SchemaOutput, error) {
	out := &SchemaOutput{
		Boxloom: schemaVersion,
		About: "Boxloom の文書の書き方。作者は意味だけを書き、見た目はテンプレートが決める。" +
			"Markdown(markdown と templates)か、IR の JSON(irSchema)のどちらかで書く。" +
			"templates に無い囲み・属性・サブシステム・variant は誤りになる。",
		Workflow: []string{
			"書く(Markdown なら .bx.md、IR なら .json)",
			"`boxloom check FILE` で検査する。誤りは {file, line?, column?, path, code, message, expected?, hint?} の形で返る",
			"誤りを渡して直す。誤りが無くなるまで繰り返す",
			"`boxloom build FILE` で PPTX にする",
		},
		Markdown: MarkdownSyntax{
			FrontMatter:     frontMatterKeys(),
			SlideAttributes: append([]string{}, slideAttributeNames...),
			Blocks:          coreBlocks(),
			Body:            bodySyntax(),
			Rules:           markdownRules(),
		},
	}
	subs := map[string]SubsystemDecl{} // 全テンプレートの和(irSchema に同梱する)
	for _, t := range templates {
		ts := TemplateSchema{Name: t.Name, Variants: []string{}, Slides: t.Slides, Blocks: t.Blocks}
		for v := range t.Variants {
			ts.Variants = append(ts.Variants, v)
		}
		sort.Strings(ts.Variants)
		for _, kind := range t.Subsystems.Kinds() {
			d, ok := t.Subsystems.Decl(kind)
			if !ok {
				return nil, fmt.Errorf("template %q: subsystem %q declares no schema", t.Name, kind)
			}
			if _, md := mdSubsystems[kind]; !md {
				// Markdown では書けない(IR でだけ書ける)サブシステムは fence を空にする。
				d.Markdown = nil
			}
			id, err := schemaID(d.Schema)
			if err != nil {
				return nil, fmt.Errorf("subsystem %q: %w", kind, err)
			}
			if prev, ok := subs[kind]; ok && !bytes.Equal(prev.Schema, d.Schema) {
				return nil, fmt.Errorf("subsystem %q has different schemas across templates", kind)
			}
			subs[kind] = d
			ss := SubsystemSchema{Name: kind, Attributes: []string{"variant"},
				Markdown: d.Markdown, Schema: "/irSchema/$defs/" + kind, SchemaID: id}
			if ss.Markdown == nil {
				ss.Markdown = []string{"Markdown では書けない。IR で書く"}
			} else {
				ss.Fence = "```" + kind
			}
			ts.Subsystems = append(ts.Subsystems, ss)
		}
		out.Templates = append(out.Templates, ts)
	}
	bundleName := "all"
	if len(templates) == 1 {
		bundleName = templates[0].Name
	}
	ir, err := bundleIRSchema(bundleName, subs)
	if err != nil {
		return nil, err
	}
	out.IRSchema = ir
	return out, nil
}

func schemaID(raw json.RawMessage) (string, error) {
	var head struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return "", err
	}
	if head.ID == "" {
		return "", fmt.Errorf("schema has no $id")
	}
	return head.ID, nil
}

// bundleIRSchema は IR の入口の Schema(schema/boxloom-ir.schema.json と同じ働き)を、
// 外を参照しない1つの文書に組む。コアと各サブシステムの Schema は、それぞれの
// $id のまま $defs に同梱する(JSON Schema 2020-12 の複合文書)。中の相対の
// $ref は各文書の $id から解決されるので、書き換えない。差し込み口(#subsystem)は
// 根の $defs.subsystem が埋め、subs のサブシステムだけを受け付ける。
func bundleIRSchema(name string, subs map[string]SubsystemDecl) (json.RawMessage, error) {
	kinds := make([]string, 0, len(subs))
	for k := range subs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	type obj = map[string]any
	var allOf []any
	defs := obj{}
	for _, k := range kinds {
		id, err := schemaID(subs[k].Schema)
		if err != nil {
			return nil, err
		}
		allOf = append(allOf, obj{
			"if":   obj{"properties": obj{"subsystem": obj{"const": k}}, "required": []string{"subsystem"}},
			"then": obj{"$ref": id},
		})
		defs[k] = subs[k].Schema
	}
	slot := obj{
		"$dynamicAnchor": "subsystem",
		"description":    "このテンプレートが名乗るサブシステム。subsystem の名前で振り分ける。",
		"type":           "object",
		"properties":     obj{"subsystem": obj{"enum": kinds}},
		"required":       []string{"subsystem"},
	}
	if len(allOf) > 0 {
		slot["allOf"] = allOf
	}
	core := embeddedSchema("core.schema.json")
	coreID, err := schemaID(core)
	if err != nil {
		return nil, err
	}
	if _, clash := defs["core"]; clash {
		return nil, fmt.Errorf("a subsystem must not be named \"core\"")
	}
	if _, clash := defs["subsystem"]; clash {
		return nil, fmt.Errorf("a subsystem must not be named \"subsystem\"")
	}
	defs["core"] = core
	defs["subsystem"] = slot

	// 根のキーの順を読みやすく保つため、順を決めて書く。
	var buf bytes.Buffer
	buf.WriteString("{")
	put := func(k string, v any, last bool) error {
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(v)
		if err != nil {
			return err
		}
		buf.Write(kb)
		buf.WriteString(":")
		buf.Write(vb)
		if !last {
			buf.WriteString(",")
		}
		return nil
	}
	for _, kv := range []struct {
		k string
		v any
	}{
		{"$schema", schemaDraft},
		{"$id", bundleIDBase + name + ".schema.json"},
		{"title", "Boxloom IR (" + schemaVersion + ") — テンプレート " + name + " で書けるもの"},
		{"description", "IR の文書の Schema。コア(core)と、テンプレートが名乗るサブシステムの Schema を $defs に同梱した1つの文書。"},
		{"$ref", coreID},
	} {
		if err := put(kv.k, kv.v, false); err != nil {
			return nil, err
		}
	}
	if err := put("$defs", defs, true); err != nil {
		return nil, err
	}
	buf.WriteString("}")
	return json.RawMessage(buf.Bytes()), nil
}
