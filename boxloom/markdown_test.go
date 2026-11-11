package boxloom

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mdToAny(t *testing.T, src string) (any, []Issue) {
	t.Helper()
	ir, errs := MarkdownToIR("t.bx.md", []byte(src))
	if len(errs) > 0 {
		return nil, errs
	}
	var v any
	if err := json.Unmarshal(ir, &v); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, ir)
	}
	return v, nil
}

func jsonFileAny(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// 例の Markdown を IR にすると、IR の例と意味として同じ(JSON として等しい)。
func TestMarkdownExamplesMatchIR(t *testing.T) {
	pairs := [][2]string{
		{"../examples/md/demo-deck.bx.md", "../examples/ir/demo-deck.json"},
		{"../examples/md/gantt-full.bx.md", "../examples/ir/gantt-full.json"},
		{"../examples/md/speaker-notes.bx.md", "../examples/ir/speaker-notes.json"},
		{"../examples/md/shapes.bx.md", "../examples/ir/shapes.json"},
		{"../examples/md/text-patterns.bx.md", "../examples/ir/text-patterns.json"},
	}
	for _, pr := range pairs {
		src, err := os.ReadFile(pr[0])
		if err != nil {
			t.Fatal(err)
		}
		got, errs := mdToAny(t, string(src))
		if len(errs) > 0 {
			t.Fatalf("%s: %v", pr[0], MarkdownErrors(errs))
		}
		want := jsonFileAny(t, pr[1])
		if !reflect.DeepEqual(got, want) {
			gb, _ := json.MarshalIndent(got, "", " ")
			t.Errorf("%s: IR differs from %s\n%s", pr[0], pr[1], gb)
		}
	}
}

// .bx.md をそのまま build できる。
func TestBuildFromMarkdown(t *testing.T) {
	d, err := readDeck("../examples/md/demo-deck.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	tm, err := LookupTemplate(d.Template)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderDeckToFile(tm, d, t.TempDir()+"/x.pptx"); err != nil {
		t.Fatal(err)
	}
}

func TestInlineAndLists(t *testing.T) {
	src := "# 題\n\n" +
		"日本語の文が\n続く行 and\nEnglish line\\\n改行のあと `code_x` と [リンク](https://example.com/a) と \\*星\\*\n\n" +
		"1. 一つ目 **強い `c`**\n  - 入れ子\n2. 二つ目\n"
	got, errs := mdToAny(t, src)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	body := got.(map[string]any)["slides"].([]any)[0].(map[string]any)["body"].([]any)
	para := body[0].(map[string]any)["content"].([]any)
	if para[0] != "日本語の文が続く行 and English line" {
		t.Fatalf("soft breaks: %q", para[0])
	}
	if para[1].(map[string]any)["type"] != "br" {
		t.Fatalf("hard break: %v", para[1])
	}
	if para[3].(map[string]any)["text"] != "code_x" || para[5].(map[string]any)["href"] != "https://example.com/a" || para[6] != " と *星*" {
		t.Fatalf("inline: %#v", para)
	}
	items := body[1].(map[string]any)["items"].([]any)
	first := items[0].(map[string]any)
	if first["items"].([]any)[0] != "入れ子" {
		t.Fatalf("nested: %#v", first)
	}
	em := first["content"].([]any)[1].(map[string]any)
	if em["type"] != "em" || em["content"].([]any)[0] != "強い " {
		t.Fatalf("em: %#v", em)
	}
	if items[1] != "二つ目" {
		t.Fatalf("second: %#v", items[1])
	}
	// 文書の頭が無ければ meta も表紙も無い。
	if _, ok := got.(map[string]any)["meta"]; ok {
		t.Fatal("meta without front matter")
	}
}

func TestFrontMatterCoverAndVariant(t *testing.T) {
	src := "---\ntitle: T\ndate: 2026-11-11\n---\n# A {variant=dense}\n\n:::note {variant=quiet}\n注記\n:::\n"
	got, errs := mdToAny(t, src)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	m := got.(map[string]any)
	slides := m["slides"].([]any)
	if slides[0].(map[string]any)["type"] != "cover" || m["meta"].(map[string]any)["date"] != "2026-11-11" {
		t.Fatalf("cover/meta: %#v", m)
	}
	s := slides[1].(map[string]any)
	if s["variant"] != "dense" || s["body"].([]any)[0].(map[string]any)["variant"] != "quiet" {
		t.Fatalf("variant: %#v", s)
	}
}

// 誤りは場所と code つきで、全部集める。
func TestMarkdownErrors(t *testing.T) {
	cases := []struct {
		name, src, code string
		line            int
	}{
		{"orphan", "前置き\n# A\n", codeMisplaced, 1},
		{"unknown block", "# A\n:::warning\nx\n:::\n", codeUnknownBlock, 2},
		{"planned block", "# A\n:::columns\nx\n:::\n", codePlannedSyntax, 2},
		{"unknown attr", "# A {color=red}\n", codeUnknownAttribute, 1},
		{"id attr planned", "# A {id=x}\n", codePlannedSyntax, 1},
		{"raw html block", "# A\n<div>x</div>\n", codeRawHTML, 2},
		{"raw html inline", "# A\n改行<br>です\n", codeRawHTML, 2},
		{"italic", "# A\n*斜体*\n", codeUnsupportedSyntax, 2},
		{"strike", "# A\n~~x~~\n", codeUnsupportedSyntax, 2},
		{"subheading", "# A\n## B\n", codeUnsupportedSyntax, 2},
		{"h3", "# A\n### B\n", codeUnsupportedSyntax, 2},
		{"hr", "# A\n\n---\n", codeUnsupportedSyntax, 3},
		{"quote", "# A\n> [!NOTE] x\n", codeUnsupportedSyntax, 2},
		{"table", "# A\n| a | b |\n", codePlannedSyntax, 2},
		{"image", "# A\n![図](a.png)\n", codePlannedSyntax, 2},
		{"star bullet", "# A\n* x\n", codeUnsupportedSyntax, 2},
		{"deep list", "# A\n- a\n  - b\n    - c\n", codeMisplaced, 4},
		{"unclosed fence", "# A\n:::note\nx\n", codeUnclosedBlock, 2},
		{"colon mismatch", "# A\n:::note\nx\n::::\n", codeUnclosedBlock, 4},
		{"inner colons", "# A\n:::compare\n:::option a\n", codeUnclosedBlock, 3},
		{"option outside", "# A\n:::option a\n:::pros\n- x\n:::\n:::\n", codeUnclosedBlock, 3},
		{"pros outside", "# A\n:::pros\n- x\n:::\n", codeMisplaced, 2},
		{"compare one option", "# A\n::::compare\n:::option a\n- x\n:::\n::::\n", codeMisplaced, 2},
		{"code block", "# A\n```go\nx\n```\n", codePlannedSyntax, 2},
		{"unclosed code", "# A\n```gantt\nspan: 2026-01 .. 2027-01\n", codeUnclosedBlock, 2},
		{"unclosed em", "# A\n**強調\n", codeUnclosedBlock, 2},
		{"bad link", "# A\n[a](b)\n", codeInvalidValue, 2},
		{"footnote", "# A\n文[^1]\n", codeUnsupportedSyntax, 2},
		{"fm unknown", "---\ntitle: T\ncolor: red\n---\n# A\n", codeUnknownKey, 3},
		{"fm no title", "---\nauthor: a\n---\n# A\n", codeMissingKey, 2},
		{"fm yes", "---\ntitle: T\ncover: yes\n---\n# A\n", codeInvalidValue, 3},
		{"note two paras", "# A\n:::note\na\n\nb\n:::\n", codeMisplaced, 5},
		{"no slides", "\n", codeMisplaced, 1},
		{"notes twice", "# A\n:::notes\na\n:::\n:::notes\nb\n:::\n", codeMisplaced, 5},
		{"notes nested", "# A\n::::note\n:::notes\na\n:::\n::::\n", codeMisplaced, 3},
		{"notes empty", "# A\n:::notes\n:::\n", codeMisplaced, 2},
		{"notes label", "# A\n:::notes 話すこと\na\n:::\n", codeMisplaced, 2},
		{"notes attr", "# A\n:::notes {variant=x}\na\n:::\n", codeUnknownAttribute, 2},
		{"notes code", "# A\n:::notes\n```gantt\nspan: 2026-01 .. 2027-01\n```\n:::\n", codeMisplaced, 3},
		{"notes html", "# A\n:::notes\n<b>x</b>\n:::\n", codeRawHTML, 3},
		{"cover notes no cover", ":::notes\na\n:::\n# A\n", codeMisplaced, 1},
	}
	for _, c := range cases {
		_, errs := MarkdownToIR("t.bx.md", []byte(c.src))
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.Line == c.line {
				found = true
			}
			if e.File != "t.bx.md" || e.Line <= 0 || e.Column <= 0 || e.Message == "" {
				t.Errorf("%s: incomplete error %+v", c.name, e)
			}
		}
		if !found {
			t.Errorf("%s: want %s at line %d, got %v", c.name, c.code, c.line, errs)
		}
	}
}

// Gantt の YAML の誤りは、YAML の中の行と IR の中の場所で返る。
func TestGanttYAMLErrors(t *testing.T) {
	head := "# A\n```gantt\n" // YAML の1行目はファイルの3行目
	cases := []struct {
		name, yaml, code, path string
		line                   int
	}{
		{"unknown key", "span: 2026-01 .. 2027-01\ngroups:\n  - label: g\n    color: red\n    rows: [{label: r, segments: []}]\n", codeUnknownKey, "/slides/0/body/0/data/groups/0", 6},
		{"status", "span: 2026-01 .. 2027-01\ngroups:\n  - label: g\n    rows:\n      - {label: r, segments: [{from: 2026-01, to: 2026-05, status: \"#34C08F\"}]}\n", codeInvalidValue, "/slides/0/body/0/data/groups/0/rows/0/segments/0/status", 7},
		{"missing span", "groups:\n  - label: g\n    rows: [{label: r, segments: []}]\n", codeMissingKey, "/slides/0/body/0/data", 3},
		{"bad range", "span: 2026-01 to 2027-01\ngroups:\n  - label: g\n    rows: [{label: r, segments: []}]\n", codeInvalidValue, "/slides/0/body/0/data/span", 3},
		{"bool", "span: 2026-01 .. 2027-01\ngroups:\n  - label: g\n    rows:\n      - {label: r, segments: [{from: 2026-01, to: 2026-05, status: none, openEnd: yes}]}\n", codeInvalidValue, "/slides/0/body/0/data/groups/0/rows/0/segments/0/openEnd", 7},
		{"anchor", "span: &a 2026-01 .. 2027-01\ngroups: []\n", codeYAML, "/slides/0/body/0/data", 3},
		{"target", "span: 2026-01 .. 2027-01\ngroups:\n  - label: g\n    rows: [{label: r, segments: []}]\nannotations:\n  - {at: 2026-05, target: nope, text: x}\n", codeInvalidValue, "/slides/0/body/0/data", 2},
		{"row needs segments", "span: 2026-01 .. 2027-01\ngroups:\n  - label: g\n    rows:\n      - label: r\n", codeMissingKey, "/slides/0/body/0/data/groups/0/rows/0", 7},
	}
	for _, c := range cases {
		_, errs := MarkdownToIR("t.bx.md", []byte(head+c.yaml+"```\n"))
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.Line == c.line && strings.HasPrefix(e.Path, c.path) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at line %d path %s, got %v", c.name, c.code, c.line, c.path, errs)
		}
	}
}

func TestYAMLSubset(t *testing.T) {
	n, errs := parseYAML([]string{
		"a: 1 # comment",
		"b: \"x: y # not comment\"",
		"c: [p, 'q''s', {k: v}]",
		"d:",
		"- e",
		"- f: g",
		"  h: i",
		"j: yes",
	}, 1)
	if len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
	get := func(k string) *yNode {
		for i, kk := range n.keys {
			if kk == k {
				return n.vals[i]
			}
		}
		t.Fatalf("no key %s", k)
		return nil
	}
	if get("a").str != "1" || get("b").str != "x: y # not comment" || get("j").str != "yes" {
		t.Fatal("scalars")
	}
	c := get("c")
	if c.items[1].str != "q's" || c.items[2].vals[0].str != "v" {
		t.Fatal("flow")
	}
	d := get("d")
	if d.kind != 'q' || d.items[0].str != "e" || d.items[1].keys[1] != "h" {
		t.Fatalf("block seq: %+v", d)
	}
	for _, bad := range [][]string{{"a: !tag x"}, {"a: *ref"}, {"a: |"}, {"a: 1", "a: 2"}, {"a: [1,", "2]"}, {"\ta: 1"}} {
		if _, errs := parseYAML(bad, 1); len(errs) == 0 {
			t.Errorf("%q: want error", bad)
		}
	}
}

// 仕様(docs/content-syntax.md の4章)の例は examples/md/demo-deck.bx.md と同じ。
func TestSpecExampleIsTheExampleFile(t *testing.T) {
	spec, err := os.ReadFile("../docs/content-syntax.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(spec)
	i := strings.Index(s, "````markdown\n")
	j := strings.Index(s[i+13:], "\n````")
	if i < 0 || j < 0 {
		t.Fatal("example block not found in the spec")
	}
	ex := s[i+13:i+13+j] + "\n"
	md, err := os.ReadFile("../examples/md/demo-deck.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	if ex != string(md) {
		t.Fatal("the spec example and examples/md/demo-deck.bx.md differ")
	}
}

// :::notes はスライドの notes(文字だけ)になり、本文には入らない。
func TestSpeakerNotes(t *testing.T) {
	src := "---\ntitle: T\n---\n:::notes\n表紙で話す\n:::\n# A\n本文\n\n:::notes\n**強調** と `x`\\\n次の行\n\n- 項目\n  - 子\n:::\n"
	v, errs := mdToAny(t, src)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	slides := v.(map[string]any)["slides"].([]any)
	if got := slides[0].(map[string]any)["notes"]; got != "表紙で話す" {
		t.Errorf("cover notes = %q", got)
	}
	s1 := slides[1].(map[string]any)
	if got, want := s1["notes"], "強調 と x\n次の行\n- 項目\n  - 子"; got != want {
		t.Errorf("notes = %q, want %q", got, want)
	}
	if n := len(s1["body"].([]any)); n != 1 {
		t.Errorf("body has %d blocks, want 1", n)
	}
	_, _, lines := ParseMarkdownWithMap([]byte(src), "t.bx.md")
	if l := lines.Line("/slides/1/notes"); l != 10 {
		t.Errorf("notes line = %d, want 10", l)
	}
}

// ノートは1行ずつ段落になり、ノートのページにはスライドの縮図と文の置き場がある。
// 組み立ての警告は返り値にだけ出て、ノートには入らない。
func TestSpeakerNotesPPTX(t *testing.T) {
	d, err := readDeck("../examples/md/speaker-notes.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	tm, err := LookupTemplate(d.Template)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir() + "/n.pptx"
	warnings, err := renderDeckToFile(tm, d, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected build warnings (fill-ratio) for this example")
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	read := func(name string) string {
		for _, f := range zr.File {
			if f.Name == name {
				r, _ := f.Open()
				b, _ := io.ReadAll(r)
				r.Close()
				return string(b)
			}
		}
		t.Fatalf("%s not found", name)
		return ""
	}
	ns := read("ppt/notesSlides/notesSlide2.xml")
	for _, want := range []string{"<a:t>第1段階の「5 人」は目安。</a:t>", "<a:t>  - 見た目より流れを確かめる</a:t>"} {
		if !strings.Contains(ns, want) {
			t.Errorf("notesSlide2 lacks %s", want)
		}
	}
	if !strings.Contains(read("ppt/notesSlides/notesSlide1.xml"), "挨拶のあと") {
		t.Error("cover notes missing")
	}
	// 組み立ての警告はノートに入れない(ノートは作者の文だけ)。
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/notesSlides/notesSlide") {
			if c := read(f.Name); strings.Contains(c, "WARNING") {
				t.Errorf("%s contains a build warning", f.Name)
			}
		}
	}
	nm := read("ppt/notesMasters/notesMaster1.xml")
	if !strings.Contains(nm, `type="sldImg"`) || !strings.Contains(nm, `type="body"`) {
		t.Error("notes master lacks placeholders")
	}
}
