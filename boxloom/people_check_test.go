package boxloom

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// 人物の紹介(people)の誤り・意味の検査・代わりの組み方の試験(Backlog #120)。

// noPeopleTemplate は people の組み方を持たない試験用のテンプレート(light の写しから外す)。
func noPeopleTemplate(t *testing.T) *Template {
	t.Helper()
	tm := mustTemplate("light")
	delete(tm.components.components, peopleType)
	if tm.components.has(peopleType) || !mustTemplate("light").components.has(peopleType) {
		t.Fatal("template copies share the component table")
	}
	return tm
}

// 組み方を持たないテンプレートは、people ごとに component-fallback を出して標準の型で組む。
// 2〜3人は各人が1つのまとまりの columns、1人と4人(columns は2〜3個)と並列のまとまりの中
// (columns を入れ子にできない)は、名前・役職・写真・経歴のブロックを積む。
func TestPeopleFallback(t *testing.T) {
	data, err := os.ReadFile(peopleExampleMD)
	if err != nil {
		t.Fatal(err)
	}
	assets := os.DirFS("../examples/md")
	res := Check("people.bx.md", data, CheckOptions{Values: noPeopleTemplate(t), Assets: assets})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	var got []string
	for _, w := range res.Warnings {
		if w.Code == "component-fallback" {
			got = append(got, w.Path)
		}
	}
	want := []string{"/slides/1/body/0", "/slides/2/body/0", "/slides/3/body/0/columns/0/body/0"}
	if !slices.Equal(got, want) {
		t.Errorf("component-fallback at %v, want %v", got, want)
	}
	for _, name := range []string{"dark", "light", "report"} {
		for _, w := range Check("people.bx.md", data, CheckOptions{Template: name, Assets: assets}).Warnings {
			if w.Code == "component-fallback" {
				t.Errorf("%s: %+v", name, w)
			}
		}
	}

	person := func(i int) string {
		return fmt.Sprintf(`{"name": "人 %d", "role": "役 %d", "image": "a.png", "alt": "絵", "body": {"type": "paragraph", "content": "経歴"}}`, i, i)
	}
	for n := 1; n <= 4; n++ {
		var ps []string
		for i := 0; i < n; i++ {
			ps = append(ps, person(i))
		}
		b := &IRBlock{Type: peopleType, Raw: json.RawMessage(`{"type": "people", "people": [` + strings.Join(ps, ",") + `]}`)}
		alts, paths, err := peopleFallback(b)
		if err != nil {
			t.Fatal(err)
		}
		var types []string
		for _, a := range alts {
			types = append(types, a.Type)
			for _, c := range a.Columns {
				if !strings.HasPrefix(c.Title.Plain(), "人 ") || len(c.Body) != 3 {
					t.Errorf("%d people: column %+v", n, c)
				}
			}
		}
		switch n {
		case 1:
			if !slices.Equal(types, []string{"paragraph", "note", "image", "paragraph"}) {
				t.Errorf("1 person: %v", types)
			}
		case 2, 3:
			if !slices.Equal(types, []string{"columns"}) || len(alts[0].Columns) != n {
				t.Errorf("%d people: %v", n, types)
			}
			// 場所の表:まとまりは人、まとまりの中のブロックは人の欄に写す。
			if paths["/columns/1"].To != "/people/1" || paths["/columns/1/body/1"].To != "/people/1/image" || !paths["/columns/1/body/2"].Same {
				t.Errorf("%d people: paths %+v", n, paths)
			}
		case 4:
			if !slices.Equal(types, []string{"columns", "columns"}) || len(alts[0].Columns) != 2 || len(alts[1].Columns) != 2 {
				t.Errorf("4 people: %v", types)
			}
		}
		stacked, _, err := peopleFallbackStacked(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range stacked {
			if !contains(columnBlockTypes, a.Type) || a.Type == peopleType || a.Type == "columns" {
				t.Errorf("%d people stacked: %s cannot go in a column", n, a.Type)
			}
		}
	}
}

// IR の形の誤り(Schema と同じ決まり)は、Go でも場所つきで断る。
func TestPeopleIRErrors(t *testing.T) {
	ok := `{"name": "人", "role": "役"}`
	five := strings.Repeat(ok+",", 4) + ok
	for _, c := range []struct{ name, block, path string }{
		{"no people", `{"type": "people", "people": []}`, "/slides/1/body/0"},
		{"five people", `{"type": "people", "people": [` + five + `]}`, "/slides/1/body/0"},
		{"no name", `{"type": "people", "people": [{"role": "役"}]}`, "/slides/1/body/0/people/0/name"},
		{"image without alt", `{"type": "people", "people": [{"name": "人", "image": "wide.png"}]}`, "/slides/1/body/0/people/0/alt"},
		{"alt without image", `{"type": "people", "people": [{"name": "人", "alt": "絵"}]}`, "/slides/1/body/0/people/0/alt"},
		{"image url", `{"type": "people", "people": [{"name": "人", "image": "https://example.invalid/a.png", "alt": "絵"}]}`, "/slides/1/body/0/people/0/image"},
		{"note body", `{"type": "people", "people": [{"name": "人", "body": {"type": "note", "content": "a"}}]}`, "/slides/1/body/0/people/0/body"},
		{"photo size", `{"type": "people", "people": [{"name": "人", "size": 80}]}`, "/slides/1/body/0/people/0/size"},
		{"three in a column", `{"type": "columns", "columns": [{"body": [{"type": "people", "people": [` + ok + `,` + ok + `,` + ok + `]}]}, {"body": [{"type": "paragraph", "content": "a"}]}]}`, "/slides/1/body/0/columns/0/body/0"},
	} {
		res := Check("deck.json", pairDeck("", c.block), CheckOptions{Assets: testAssets(t)})
		if len(res.Errors) != 1 || res.Errors[0].Path != c.path {
			t.Errorf("%s: errors %+v (want one at %s)", c.name, res.Errors, c.path)
		}
	}
	// まとまりの中の2人は読める。
	two := `{"type": "columns", "columns": [{"body": [{"type": "people", "people": [` + ok + `,` + ok + `]}]}, {"body": [{"type": "paragraph", "content": "a"}]}]}`
	if res := Check("deck.json", pairDeck("", two), CheckOptions{}); len(res.Errors) > 0 {
		t.Errorf("two in a column: %+v", res.Errors)
	}
}

// Markdown の読み方:形の誤りは場所つきで返す。
func TestPeopleMarkdownErrors(t *testing.T) {
	head := "---\ntitle: T\n---\n\n# 題\n\n"
	person := func(attrs, body string) string { return ":::person 人 " + attrs + "\n" + body + ":::\n" }
	for _, c := range []struct {
		name, src, code, path string
	}{
		{"no name", "::::people\n:::person {role=役}\n:::\n::::\n", codeMissingKey, "/slides/1/body/0/people/0/name"},
		{"image without alt", "::::people\n" + person("{image=a.png}", "") + "::::\n", codeMissingKey, "/slides/1/body/0/people/0/alt"},
		{"alt without image", "::::people\n" + person("{alt=絵}", "") + "::::\n", codeMisplaced, "/slides/1/body/0/people/0/alt"},
		{"unknown attribute", "::::people\n" + person("{size=80}", "") + "::::\n", codeUnknownAttribute, "/slides/1/body/0/people/0"},
		{"two paragraphs", "::::people\n" + person("", "あ\n\nい\n") + "::::\n", codeMisplaced, "/slides/1/body/0/people/0/body"},
		{"paragraph in people", "::::people\n段落\n\n" + person("", "") + "::::\n", codeMisplaced, "/slides/1/body/0"},
		{"five people", "::::people\n" + strings.Repeat(person("", ""), 5) + "::::\n", codeMisplaced, "/slides/1/body/0/people"},
		{"person outside", person("", ""), codeMisplaced, "/slides/1/body/0"},
		{"three in a column", "::::::columns\n:::::column\n::::people\n" + strings.Repeat(person("", ""), 3) + "::::\n:::::\n:::::column\nあ\n:::::\n::::::\n",
			codeMisplaced, "/slides/1/body/0/columns/0/body/0/people"},
	} {
		_, issues := MarkdownToIR("t.bx.md", []byte(head+c.src))
		if len(issues) == 0 {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if issues[0].Code != c.code || issues[0].Path != c.path {
			t.Errorf("%s: %+v (want %s at %s)", c.name, issues[0], c.code, c.path)
		}
	}
}

// people の中も本文と同じ意味の検査(empty-text・画像の検査)を通る。場所は IR の場所で、
// 本文でも並列のまとまりの中でも、組み方を持つテンプレートでも代わりの組み方でも同じ。
func TestPeopleSemanticCheck(t *testing.T) {
	people := `{"type": "people", "people": [
	  {"name": " ", "role": "役", "image": "wide.png", "alt": "絵", "body": {"type": "paragraph", "content": " "}},
	  {"name": "人", "role": "", "image": "missing.png", "alt": " "}]}`
	rel := []string{"/people/0/name", "/people/0/body/content", "/people/1/role", "/people/1/alt", "/people/1/image"}
	codes := []string{"empty-text", "empty-text", "empty-text", "image-alt", "image-missing"}
	for _, at := range []struct{ block, base string }{
		{people, "/slides/1/body/0"},
		{`{"type": "columns", "columns": [{"body": [` + people + `]}, {"body": [{"type": "paragraph", "content": "a"}]}]}`, "/slides/1/body/0/columns/0/body/0"},
	} {
		for _, c := range []struct {
			name string
			opts CheckOptions
		}{
			{"light", CheckOptions{Template: "light", Assets: testAssets(t)}},
			{"report", CheckOptions{Template: "report", Assets: testAssets(t)}},
			{"fallback", CheckOptions{Values: noPeopleTemplate(t), Assets: testAssets(t)}},
		} {
			res := Check("deck.json", pairDeck("", at.block), c.opts)
			var got, want []string
			for _, e := range res.Errors {
				got = append(got, e.Code+" "+e.Path)
			}
			for i := range rel {
				want = append(want, codes[i]+" "+at.base+rel[i])
			}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s at %s: errors\n%s\nwant\n%s", c.name, at.base, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		}
	}
}

// IR の Schema(コアの block と、まとまりの中の columnBlock)は people を振り分ける。
func TestPeopleSchema(t *testing.T) {
	var core struct {
		Defs map[string]struct {
			Properties struct {
				Type struct {
					Enum []string `json:"enum"`
				} `json:"type"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(embeddedSchema("core.schema.json"), &core); err != nil {
		t.Fatal(err)
	}
	for _, def := range []string{"block", "columnBlock"} {
		if !slices.Contains(core.Defs[def].Properties.Type.Enum, peopleType) {
			t.Errorf("%s does not name people: %v", def, core.Defs[def].Properties.Type.Enum)
		}
	}
	d, _ := blockTypes.decl(peopleType)
	if id, err := schemaID(d.Schema); err != nil || !strings.HasSuffix(id, "/types/people.schema.json") {
		t.Errorf("people schema id %q %v", id, err)
	}
}
