package boxloom

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// 標準の型は docs/layers.md の5章の一式で、囲みは defaultBlocks と同じものを名乗る。
// 型の意味は語彙にあり、標準の部品の表は組み方だけを持つ。
func TestStandardComponents(t *testing.T) {
	want := []string{"columns", "compare", "gallery", "image", "note", "paragraph", "points", "quote", "stat", "subsystem", "table", "takeaway"}
	var std []string
	for _, n := range blockTypes.names() {
		if blockTypes.isStandard(n) {
			std = append(std, n)
		}
	}
	if !reflect.DeepEqual(std, want) {
		t.Fatalf("standard types = %v, want %v", std, want)
	}
	// 標準でない型はコアの pairs・people・topics(docs/layers.md の8.1)。
	if got := blockTypes.extensions(); !reflect.DeepEqual(got, []string{"pairs", "people", "topics"}) {
		t.Fatalf("non-standard types = %v", got)
	}
	if got := defaultComponents().names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("components = %v, want %v", got, want)
	}
	var fences []BlockDecl
	for _, n := range want {
		d, ok := blockTypes.decl(n)
		if !ok || !blockTypes.isStandard(n) {
			t.Fatalf("%s: not a standard type", n)
		}
		if d.Schema != nil || d.Fallback != nil {
			t.Errorf("%s: standard type has its own Schema or a fallback", n)
		}
		fences = append(fences, d.Markdown...)
	}
	sort.Slice(fences, func(i, j int) bool { return fences[i].Name < fences[j].Name })
	if !reflect.DeepEqual(fences, defaultBlocks()) {
		t.Errorf("fences of the standard types differ from defaultBlocks")
	}
	// 誤りの文の順は前と同じ。
	if got := strings.Join(blockTypes.known(), ", "); got != "paragraph, points, compare, note, takeaway, stat, quote, columns, table, image, gallery, subsystem, pairs, topics, people" {
		t.Errorf("known = %s", got)
	}
}

// 試験用の標準でない型:左右の文の対。代わりの組み方は columns(左右の段落)。
const testPairType = "testpair"

var testPairSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://boxloom.invalid/schema/ir/0.1/types/testpair.schema.json",
  "type": "object",
  "properties": {"type": {"const": "testpair"}, "variant": {"type": "string"}, "left": {"type": "string"}, "right": {"type": "string"}},
  "required": ["type", "left", "right"],
  "additionalProperties": false
}`)

type testPair struct {
	Type    string `json:"type"`
	Variant string `json:"variant"`
	Left    string `json:"left"`
	Right   string `json:"right"`
}

func readTestPair(b *IRBlock) (testPair, error) {
	var p testPair
	err := strictUnmarshal(b.Raw, &p)
	return p, err
}

func testPairFallback(b *IRBlock) ([]*IRBlock, fallbackPaths, error) {
	p, err := readTestPair(b)
	if err != nil {
		return nil, nil, err
	}
	col := func(s string) IRColumn {
		return IRColumn{Body: []IRBlock{{Type: "paragraph", Content: Inline{{Text: s}}}}}
	}
	return []*IRBlock{{Type: "columns", Columns: []IRColumn{col(p.Left), col(p.Right)}}}, nil, nil
}

// withTestPair は試験の間だけ testpair を語彙に足す。
func withTestPair(t *testing.T) {
	t.Helper()
	if err := registerType(testPairType, typeDecl{Schema: testPairSchema, Fallback: testPairFallback}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { blockTypes.remove(testPairType) })
}

// pairTemplate は testpair の組み方を持つテンプレート(light の写しに足す)。
func pairTemplate(t *testing.T) *Template {
	t.Helper()
	tm := mustTemplate("light")
	if err := tm.components.register(testPairType, componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		p, err := readTestPair(b)
		if err != nil {
			return nil, err
		}
		return Row([]*Spec{
			ctx.t.text(Inline{{Text: p.Left}}, "body", "text_primary", false),
			ctx.t.text(Inline{{Text: p.Right}}, "body", "text_primary", false),
		}, tokGap("blockGap")), nil
	})); err != nil {
		t.Fatal(err)
	}
	return tm
}

func pairDeck(template, block string) []byte {
	head := ""
	if template != "" {
		head = `"template": "` + template + `", `
	}
	return []byte(`{"boxloom": "0.1", ` + head + `"meta": {"title": "T"}, "slides": [
  {"type": "cover"},
  {"type": "content", "title": "対", "body": [` + block + `]}]}`)
}

const pairBlock = `{"type": "testpair", "left": "左の文", "right": "右の文"}`

func TestComponentRegistry(t *testing.T) {
	withTestPair(t)
	r := defaultComponents()
	if err := r.register("note", componentFunc(nil)); err == nil {
		t.Error("register of a standard type succeeded")
	}
	if err := r.register("nosuchtype", componentFunc(nil)); err == nil {
		t.Error("register of a type outside the vocabulary succeeded")
	}
	if err := r.override("nosuchtype", componentFunc(nil)); err == nil {
		t.Error("override of an unknown name succeeded")
	}
	if err := r.override(testPairType, componentFunc(nil)); err == nil {
		t.Error("override of a non-standard type succeeded")
	}
	if err := r.register(testPairType, componentFunc(nil)); err != nil {
		t.Errorf("register of a non-standard type: %v", err)
	}
	if err := r.register(testPairType, componentFunc(nil)); err == nil {
		t.Error("second register succeeded")
	}
	if _, err := r.build(componentContext{}, &IRBlock{Type: "nosuchtype"}); err == nil || err.Error() != `unknown block type "nosuchtype"` {
		t.Errorf("unknown type: err = %v", err)
	}
	if err := registerType("note", typeDecl{}); err == nil {
		t.Error("registerType of a standard name succeeded")
	}
	if err := registerType("nofallback", typeDecl{Schema: testPairSchema}); err == nil {
		t.Error("registerType without a fallback succeeded")
	}
	if err := registerType("noschema", typeDecl{Fallback: testPairFallback}); err == nil {
		t.Error("registerType without a schema succeeded")
	}

	// テンプレートは写しを持つ。差し替えはほかのテンプレートにも標準の表にも及ばない。
	dark, light := mustTemplate("dark"), mustTemplate("light")
	marker := Col(nil)
	if err := dark.components.override("note", componentFunc(func(componentContext, *IRBlock) (*Spec, error) {
		return marker, nil
	})); err != nil {
		t.Fatal(err)
	}
	b := &IRBlock{Type: "note", Content: Inline{{Text: "x"}}}
	s, err := dark.buildBlock(b)
	if err != nil || s != marker || s.irType != "note" {
		t.Errorf("overridden note: spec %p (want %p), irType %q, err %v", s, marker, s.irType, err)
	}
	s, err = light.buildBlock(b)
	if err != nil || s == marker {
		t.Errorf("override leaked into another template (err %v)", err)
	}
	if again, _ := standardComponents.components["note"].build(componentContext{t: light}, b); again == marker {
		t.Error("override leaked into the standard table")
	}
	if pairTemplate(t); light.components.has(testPairType) || standardComponents.has(testPairType) {
		t.Error("register leaked into another template or the standard table")
	}
}

func codes(is []Issue) []string {
	var out []string
	for _, i := range is {
		out = append(out, i.Code)
	}
	return out
}

// 標準でない型:組み方の無いテンプレートは代わりの組み方で組んで警告1件、組み方の
// あるテンプレートは警告なし、テンプレートを指さなければ語彙だけで検査(警告なし)、
// 知らない type は誤り。
func TestTypeVocabularyFallback(t *testing.T) {
	withTestPair(t)

	fallbackOnly := func(is []Issue) []Issue {
		var out []Issue
		for _, i := range is {
			if i.Code == "component-fallback" {
				out = append(out, i)
			}
		}
		return out
	}

	// テンプレートを指す(-template、IR の template)と、組み方の無い light・dark は警告1件。
	for _, c := range []struct{ name, override, irTemplate string }{
		{"override light", "light", ""},
		{"override dark", "dark", ""},
		{"ir template", "", "light"},
	} {
		res := Check("deck.json", pairDeck(c.irTemplate, pairBlock), CheckOptions{Template: c.override})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: errors %+v", c.name, res.Errors)
		}
		fb := fallbackOnly(res.Warnings)
		if len(fb) != 1 {
			t.Fatalf("%s: component-fallback warnings = %+v (all: %v)", c.name, fb, codes(res.Warnings))
		}
		if fb[0].Path != "/slides/1/body/0" || !strings.Contains(fb[0].Message, `block type "testpair"`) || !strings.Contains(fb[0].Message, "columns") {
			t.Errorf("%s: warning = %+v", c.name, fb[0])
		}
	}

	// 指さなければ、語彙だけで検査する(誤りも component-fallback も無い)。
	res := Check("deck.json", pairDeck("", pairBlock), CheckOptions{})
	if len(res.Errors) > 0 || len(fallbackOnly(res.Warnings)) > 0 {
		t.Errorf("no template: errors %+v, warnings %v", res.Errors, codes(res.Warnings))
	}

	// 組み方のあるテンプレートは警告なし。
	res = Check("deck.json", pairDeck("", pairBlock), CheckOptions{Values: pairTemplate(t)})
	if len(res.Errors) > 0 || len(fallbackOnly(res.Warnings)) > 0 {
		t.Errorf("template with the component: errors %+v, warnings %v", res.Errors, codes(res.Warnings))
	}

	// 知らない type は誤り。
	res = Check("deck.json", pairDeck("light", `{"type": "nosuch", "left": "a", "right": "b"}`), CheckOptions{})
	if len(res.Errors) != 1 || res.Errors[0].Code != "schema" || !strings.Contains(res.Errors[0].Message, `unknown block type "nosuch"`) ||
		!strings.Contains(res.Errors[0].Message, "testpair") {
		t.Errorf("unknown type: errors %+v", res.Errors)
	}

	// 代わりの組み方の出力は、書き換えた IR(columns)をそのまま組んだものと同じ。
	build := func(deck []byte, tm *Template) ([]byte, []string) {
		t.Helper()
		d, err := ParseIR(deck)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		warns, err := tm.RenderDeck(d, &buf)
		if err != nil {
			t.Fatal(err)
		}
		return buf.Bytes(), warns
	}
	got, warns := build(pairDeck("", pairBlock), mustTemplate("light"))
	want, _ := build(pairDeck("", `{"type": "columns", "columns": [
		{"body": [{"type": "paragraph", "content": "左の文"}]},
		{"body": [{"type": "paragraph", "content": "右の文"}]}]}`), mustTemplate("light"))
	if !bytes.Equal(got, want) {
		t.Error("fallback build differs from building the rewritten columns block")
	}
	n := 0
	for _, w := range warns {
		if strings.Contains(w, "[COMPONENT-FALLBACK WARNING] body[0]: ") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("build warnings = %v", warns)
	}

	// 組み方のあるテンプレートは、その組み方で組む(代わりとは違う出力・警告なし)。
	own, warns := build(pairDeck("", pairBlock), pairTemplate(t))
	if bytes.Equal(own, got) {
		t.Error("template component was not used")
	}
	for _, w := range warns {
		if strings.Contains(w, "COMPONENT-FALLBACK") {
			t.Errorf("unexpected warning %s", w)
		}
	}
}

// schema -template は型を隠さず、型ごとに組み方を持つか・代わりで組むかを添える。
// 標準でない型の Schema は irSchema に同梱し、ブロックの type で振り分ける。
func TestSchemaTypes(t *testing.T) {
	withTestPair(t)
	out, err := buildSchemaOutput([]*Template{mustTemplate("light"), pairTemplate(t)})
	if err != nil {
		t.Fatal(err)
	}
	for i, wantBuild := range []string{"fallback", "template"} {
		ts := out.Templates[i]
		if len(ts.Types) != 16 {
			t.Fatalf("types = %+v", ts.Types)
		}
		for _, ty := range ts.Types {
			switch {
			case ty.Name == pairsType || ty.Name == topicsType || ty.Name == peopleType:
				if ty.Build != "template" || ty.Standard || ty.Schema != "/irSchema/$defs/type:"+ty.Name {
					t.Errorf("template %d: %+v", i, ty)
				}
			case ty.Name == testPairType:
				if ty.Build != wantBuild || ty.Standard || ty.Schema != "/irSchema/$defs/type:testpair" {
					t.Errorf("template %d: %+v", i, ty)
				}
			case ty.Build != "template" || !ty.Standard || ty.Schema != "":
				t.Errorf("template %d: %+v", i, ty)
			}
		}
	}
	var ir struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(out.IRSchema, &ir); err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir.Defs["type:testpair"], testPairSchema) {
		t.Errorf("type schema not bundled: %s", ir.Defs["type:testpair"])
	}
	var core struct {
		Defs struct {
			Block struct {
				Properties struct {
					Type struct {
						Enum []string `json:"enum"`
					} `json:"type"`
				} `json:"properties"`
				AllOf []json.RawMessage `json:"allOf"`
			} `json:"block"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(ir.Defs["core"], &core); err != nil {
		t.Fatal(err)
	}
	enum := core.Defs.Block.Properties.Type.Enum
	last := string(core.Defs.Block.AllOf[len(core.Defs.Block.AllOf)-1])
	if enum[len(enum)-1] != testPairType || !strings.Contains(last, "types/testpair.schema.json") {
		t.Errorf("core block does not dispatch testpair: enum %v, last allOf %s", enum, last)
	}

	// 語彙に標準でない型が無ければ、コアの Schema はそのまま同梱する。
	blockTypes.remove(testPairType)
	plain, err := buildSchemaOutput([]*Template{mustTemplate("light")})
	if err != nil {
		t.Fatal(err)
	}
	if err := registerType(testPairType, typeDecl{Schema: testPairSchema, Fallback: testPairFallback}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(plain.IRSchema, &ir); err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir.Defs["core"], embeddedSchema("core.schema.json")) {
		t.Error("core schema changed without non-standard types")
	}
	if _, err := typeSchema("nosuch"); err == nil {
		t.Error("typeSchema of a missing file succeeded")
	}
}

func sameJSON(a, b []byte) bool {
	var ca, cb bytes.Buffer
	return json.Compact(&ca, a) == nil && json.Compact(&cb, b) == nil && bytes.Equal(ca.Bytes(), cb.Bytes())
}
