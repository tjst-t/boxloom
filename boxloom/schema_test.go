package boxloom

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// schemas/ が ../schema/ の写しのままか(ずれたら `go generate`)。
func TestEmbeddedSchemasInSync(t *testing.T) {
	n := 0
	err := filepath.WalkDir("../schema", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("../schema", p)
		want, _ := os.ReadFile(p)
		got, err := schemaFS.ReadFile("schemas/" + filepath.ToSlash(rel))
		if err != nil {
			t.Errorf("schemas/%s is missing (run go generate)", rel)
			return nil
		}
		if !bytes.Equal(got, want) {
			t.Errorf("schemas/%s differs from ../schema (run go generate)", rel)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fs.WalkDir(schemaFS, "schemas", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if _, e := os.Stat(filepath.Join("..", "schema", strings.TrimPrefix(p, "schemas/"))); e != nil {
				t.Errorf("%s is not in ../schema (run go generate)", p)
			}
		}
		return nil
	})
	if n == 0 {
		t.Fatal("no schema files found")
	}
}

func schemaOut(t *testing.T, template string) map[string]any {
	t.Helper()
	b, err := Schema(template)
	if err != nil {
		t.Fatalf("Schema(%q): %v", template, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSchemaCommandTemplates(t *testing.T) {
	all := schemaOut(t, "")
	if n := len(all["templates"].([]any)); n != len(TemplateNames) {
		t.Fatalf("templates = %d, want %d", n, len(TemplateNames))
	}
	dark := schemaOut(t, "dark")
	ts := dark["templates"].([]any)
	if len(ts) != 1 || ts[0].(map[string]any)["name"] != "dark" {
		t.Fatalf("-template dark: %v", ts)
	}
	id := dark["irSchema"].(map[string]any)["$id"]
	if !strings.HasSuffix(id.(string), "/bundle/dark.schema.json") {
		t.Errorf("$id = %v", id)
	}
	if _, err := Schema("nope"); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("unknown template: err %v, want ErrUnknownTemplate", err)
	}
}

// 出力が名乗る囲み・文書の頭のキー・サブシステムが、パーサーが受け付けるものと
// 揃っているか(AI に渡す表と、実際の検査がずれないように)。
func TestSchemaMatchesParser(t *testing.T) {
	out, err := BuildSchemaOutput(TemplateNames)
	if err != nil {
		t.Fatal(err)
	}
	for _, ts := range out.Templates {
		if got := strings.Join(blockNames(append(append([]BlockDecl{}, out.Markdown.Blocks...), ts.Blocks...)), ","); got != strings.Join(knownFences, ",") {
			t.Errorf("%s: blocks %s, parser accepts %s", ts.Name, got, strings.Join(knownFences, ","))
		}
		for _, ss := range ts.Subsystems {
			if _, ok := mdSubsystems[ss.Name]; ok != (ss.Fence != "") {
				t.Errorf("%s: subsystem %s fence %q, parser knows it: %v", ts.Name, ss.Name, ss.Fence, ok)
			}
		}
	}
	// 名乗っていない囲みを書くと、expected に名乗る囲みの一覧が返る。
	_, issues := MarkdownToIR("x.bx.md", []byte("# 題\n\n:::nope\n文\n:::\n"))
	if len(issues) == 0 || strings.Join(issues[0].Expected, ",") != strings.Join(knownFences, ",") {
		t.Errorf("unknown block issues = %+v", issues)
	}
	// 文書の頭のキーも同じ表から。
	_, issues = MarkdownToIR("x.bx.md", []byte("---\ntitle: 題\nnope: 1\n---\n# 題\n\n文\n"))
	if len(issues) == 0 || strings.Join(issues[0].Expected, ",") != strings.Join(frontMatterNames(), ",") {
		t.Errorf("unknown front matter key issues = %+v", issues)
	}
}

// irSchema の $ref がすべて、同梱した文書($id)の中で解決できるか。
func TestSchemaBundleIsSelfContained(t *testing.T) {
	m := schemaOut(t, "")
	root := m["irSchema"].(map[string]any)
	docs := map[string]any{} // $id → 文書
	var collect func(v any)
	collect = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := x["$id"].(string); ok {
				docs[id] = x
			}
			for _, c := range x {
				collect(c)
			}
		case []any:
			for _, c := range x {
				collect(c)
			}
		}
	}
	collect(root)
	if len(docs) < 3 {
		t.Fatalf("bundled documents = %d, want root+core+gantt", len(docs))
	}
	refs := 0
	var walk func(v any, base string)
	walk = func(v any, base string) {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := x["$id"].(string); ok {
				base = id
			}
			for _, key := range []string{"$ref", "$dynamicRef"} {
				ref, ok := x[key].(string)
				if !ok {
					continue
				}
				refs++
				b, _ := url.Parse(base)
				r, err := url.Parse(ref)
				if err != nil {
					t.Errorf("bad %s %q", key, ref)
					continue
				}
				abs := b.ResolveReference(r)
				frag := abs.Fragment
				abs.Fragment = ""
				doc, ok := docs[abs.String()]
				if !ok {
					t.Errorf("%s %q (base %s) points outside the bundle", key, ref, base)
					continue
				}
				if frag == "" || !strings.HasPrefix(frag, "/") {
					continue // 文書そのもの・アンカー(#subsystem)
				}
				cur := doc
				for _, tok := range strings.Split(frag[1:], "/") {
					tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
					obj, ok := cur.(map[string]any)
					if !ok || obj[tok] == nil {
						t.Errorf("%s %q: #%s not found", key, ref, frag)
						break
					}
					cur = obj[tok]
				}
			}
			for _, c := range x {
				walk(c, base)
			}
		case []any:
			for _, c := range x {
				walk(c, base)
			}
		}
	}
	walk(root, "")
	if refs == 0 {
		t.Fatal("no $ref found")
	}
}
