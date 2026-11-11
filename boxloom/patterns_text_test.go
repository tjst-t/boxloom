package boxloom

import (
	"archive/zip"
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// 文字のパターン(docs/patterns.md の section・statement・takeaway・stat・quote)の試験。

const textPatternsPath = "../examples/ir/text-patterns.json"

// スライドの種類は「# 題 {type=…}」で選び、# の行の文が section の title・
// statement の content になる。:::notes はどちらにも書ける。
func TestMarkdownSlideTypes(t *testing.T) {
	src := "# 第2部 移行の進め方 {type=section}\n:::notes\n章の狙い\n:::\n" +
		"# 移行は **2 年** 早める {type=statement variant=x}\n:::notes\n言い切る\n:::\n" +
		"# 本文 {type=content}\n本文の段落\n"
	ir, errs := MarkdownToIR("t.bx.md", []byte(src))
	if len(errs) > 0 {
		// variant=x はテンプレートが決めるので、パーサーは形だけを見て通す。
		t.Fatalf("errors: %v", errs)
	}
	var got map[string]any
	if err := json.Unmarshal(ir, &got); err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"type": "section", "title": "第2部 移行の進め方", "notes": "章の狙い"},
		map[string]any{"type": "statement", "variant": "x", "content": []any{"移行は ", map[string]any{"type": "em", "content": "2 年"}, " 早める"}, "notes": "言い切る"},
		map[string]any{"type": "content", "title": "本文", "body": []any{map[string]any{"type": "paragraph", "content": "本文の段落"}}},
	}
	if !reflect.DeepEqual(got["slides"], want) {
		b, _ := json.MarshalIndent(got["slides"], "", " ")
		t.Fatalf("slides:\n%s", b)
	}
}

// 本文のブロック takeaway・stat・quote の Markdown → IR。
func TestMarkdownTextBlocks(t *testing.T) {
	src := "# A\n:::takeaway {variant=x}\n結論は **これ**\n:::\n" +
		"::::stat\n:::value 3 → 12 {unit=機種}\n対応機種\n:::\n:::value −40\n問い合わせ\n:::\n::::\n" +
		":::quote 利用者 A、2026 年 9 月\n速くなった。\\\n迷わない。\n:::\n"
	v, errs := mdToAny(t, src)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	body := v.(map[string]any)["slides"].([]any)[0].(map[string]any)["body"]
	want := []any{
		map[string]any{"type": "takeaway", "variant": "x", "content": []any{"結論は ", map[string]any{"type": "em", "content": "これ"}}},
		map[string]any{"type": "stat", "items": []any{
			map[string]any{"value": "3 → 12", "unit": "機種", "label": "対応機種"},
			map[string]any{"value": "−40", "label": "問い合わせ"},
		}},
		map[string]any{"type": "quote", "content": []any{"速くなった。", map[string]any{"type": "br"}, "迷わない。"}, "source": "利用者 A、2026 年 9 月"},
	}
	if !reflect.DeepEqual(body, want) {
		b, _ := json.MarshalIndent(body, "", " ")
		t.Fatalf("body:\n%s", b)
	}
}

// 記法の誤りは、行・列・決まった code で返る。
func TestMarkdownTextPatternErrors(t *testing.T) {
	stat := func(n int) string {
		s := "# A\n::::stat\n"
		for i := 0; i < n; i++ {
			s += ":::value 1\nx\n:::\n"
		}
		return s + "::::\n"
	}
	cases := []struct {
		name, src, code string
		line            int
	}{
		{"section body", "# 章 {type=section}\n:::notes\nn\n:::\n- 目次\n", codeMisplaced, 5},
		{"statement body", "# 1文 {type=statement}\n\n根拠の段落\n", codeMisplaced, 3},
		{"type cover", "# A {type=cover}\n", codeInvalidValue, 1},
		{"type unknown", "# A {type=agenda}\n", codeInvalidValue, 1},
		{"quote no source", "# A\n:::quote\n言葉\n:::\n", codeMissingKey, 2},
		{"quote two paras", "# A\n:::quote 出どころ\na\n\nb\n:::\n", codeMisplaced, 5},
		{"takeaway empty", "# A\n:::takeaway\n:::\n", codeMisplaced, 2},
		{"takeaway label", "# A\n:::takeaway 見出し\nx\n:::\n", codeMisplaced, 2},
		{"stat five", stat(5), codeMisplaced, 2},
		{"stat empty", "# A\n:::stat\n:::\n", codeMisplaced, 2},
		{"stat list", "# A\n:::stat\n- 12 台\n:::\n", codeMisplaced, 3},
		{"stat label", "# A\n::::stat 見出し\n:::value 1\nx\n:::\n::::\n", codeMisplaced, 2},
		{"value no digit", "# A\n::::stat\n:::value 多数\nx\n:::\n::::\n", codeInvalidValue, 3},
		{"value no number", "# A\n::::stat\n:::value {unit=台}\nx\n:::\n::::\n", codeMissingKey, 3},
		{"value no label", "# A\n::::stat\n:::value 12\n:::\n::::\n", codeMisplaced, 3},
		{"value outside", "# A\n:::value 12\nx\n:::\n", codeMisplaced, 2},
		{"value attr", "# A\n::::stat\n:::value 12 {color=red}\nx\n:::\n::::\n", codeUnknownAttribute, 3},
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
	// 5つの :::value の誤りは「4つまで」を示す。
	_, errs := MarkdownToIR("t.bx.md", []byte(stat(5)))
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "4 つまで") {
		t.Errorf("stat five: %v", errs)
	}
}

// IR の読み込みは形の誤りを断る(Schema と同じ結論。check では code schema)。
func TestParseIRTextPatterns(t *testing.T) {
	d := mustReadIR(t, textPatternsPath)
	if d.Slides[1].Type != "section" || d.Slides[1].Title.Plain() != "第1部 何が変わったか" {
		t.Fatalf("section: %+v", d.Slides[1])
	}
	if d.Slides[4].Type != "statement" || d.Slides[4].Content.Plain() != "移行は 2 年早める必要がある" || d.Slides[4].Notes == "" {
		t.Fatalf("statement: %+v", d.Slides[4])
	}
	st := d.Slides[3].Body[0]
	if st.Type != "stat" || len(st.Stats) != 4 || st.Stats[0].Unit != "%" || st.Stats[2].Unit != "" {
		t.Fatalf("stat: %+v", st)
	}
	q := d.Slides[6].Body[0]
	if q.Type != "quote" || q.Source.Plain() == "" || q.Content.Plain() == "" {
		t.Fatalf("quote: %+v", q)
	}
	bad := map[string]string{
		"statement with body":  `{"type":"statement","content":"x","body":[]}`,
		"statement no content": `{"type":"statement","title":"x"}`,
		"section no title":     `{"type":"section"}`,
		"section body":         `{"type":"section","title":"x","body":[]}`,
		"stat empty":           `{"type":"content","title":"t","body":[{"type":"stat","items":[]}]}`,
		"stat five":            `{"type":"content","title":"t","body":[{"type":"stat","items":[{"value":"1","label":"a"},{"value":"2","label":"a"},{"value":"3","label":"a"},{"value":"4","label":"a"},{"value":"5","label":"a"}]}]}`,
		"stat no digit":        `{"type":"content","title":"t","body":[{"type":"stat","items":[{"value":"多数","label":"a"}]}]}`,
		"stat no label":        `{"type":"content","title":"t","body":[{"type":"stat","items":[{"value":"1"}]}]}`,
		"stat color":           `{"type":"content","title":"t","body":[{"type":"stat","items":[{"value":"1","label":"a","color":"accent"}]}]}`,
		"quote no source":      `{"type":"content","title":"t","body":[{"type":"quote","content":"a"}]}`,
		"takeaway no content":  `{"type":"content","title":"t","body":[{"type":"takeaway"}]}`,
	}
	for name, slide := range bad {
		if _, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[` + slide + `]}`)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// 見える文字の無い章の題・主張・結論・引用・数の説明は check の誤り(empty-text)。
func TestCheckEmptyText(t *testing.T) {
	src := `{"boxloom":"0.1","slides":[
	  {"type":"section","title":" "},
	  {"type":"statement","content":[{"type":"em","content":""}]},
	  {"type":"content","title":"t","body":[
	    {"type":"takeaway","content":""},
	    {"type":"quote","content":"言葉","source":""},
	    {"type":"stat","items":[{"value":"1","label":" "}]}]}]}`
	errs, _ := CheckIR("x.json", []byte(src), "")
	var got []string
	for _, e := range errs {
		if e.Code != "empty-text" {
			t.Errorf("unexpected %+v", e)
		}
		got = append(got, e.Path)
	}
	want := []string{"/slides/0/title", "/slides/1/content", "/slides/2/body/0/content", "/slides/2/body/1/source", "/slides/2/body/2/items/0/label"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

func readZipFile(t *testing.T, zr *zip.ReadCloser, name string) string {
	t.Helper()
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

// 組み立てた PPTX: 区切りと主張は題のプレースホルダー(Title Only のレイアウト)に
// 文を書き、区切りは背景が card_alt。統計強調は数と単位を1つの文字の図形の2つの
// 塊(大きさが違う)にし、引用は印と出どころの頭を付ける。ノートは区切り・主張にも入る。
func TestBuildTextPatternsPPTX(t *testing.T) {
	d := mustReadIR(t, textPatternsPath)
	for _, name := range []string{"light", "dark"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "p.pptx")
		warns, err := renderDeckToFile(tm, d, out)
		if err != nil {
			t.Fatal(err)
		}
		if len(warns) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warns)
		}
		zr, err := zip.OpenReader(out)
		if err != nil {
			t.Fatal(err)
		}
		read := func(n string) string { return readZipFile(t, zr, n) }

		section := read("ppt/slides/slide2.xml")
		if strings.Count(section, `<p:ph type="title"/>`) != 1 || !strings.Contains(section, "<a:t>第1部 何が変わったか</a:t>") {
			t.Errorf("%s: section title is not a title placeholder", name)
		}
		if !strings.Contains(section, `val="`+tm.Theme["card_alt"]+`"`) {
			t.Errorf("%s: section background is not card_alt", name)
		}
		if !strings.Contains(read("ppt/slides/_rels/slide2.xml.rels"), "slideLayout2.xml") {
			t.Errorf("%s: section does not use the title-only layout", name)
		}
		if !strings.Contains(read("ppt/notesSlides/notesSlide2.xml"), "第1部は数字で結果を示す") {
			t.Errorf("%s: section notes missing", name)
		}

		statement := read("ppt/slides/slide5.xml")
		i := strings.Index(statement, `<p:ph type="title"/>`)
		if i < 0 || !strings.Contains(statement[i:], "<a:t>移行は 2 年早める必要がある</a:t>") || !strings.Contains(statement[i:], `sz="3600" b="1"`) {
			t.Errorf("%s: statement is not written as a large title placeholder", name)
		}
		if !strings.Contains(statement, "5 / 7") {
			t.Errorf("%s: statement lacks the footer", name)
		}
		if !strings.Contains(read("ppt/notesSlides/notesSlide5.xml"), "1文だけを言い切る") {
			t.Errorf("%s: statement notes missing", name)
		}

		stats := read("ppt/slides/slide4.xml")
		for _, want := range []string{
			`sz="3600" b="1"><a:solidFill><a:srgbClr val="` + tm.Theme["accent"] + `"/></a:solidFill></a:rPr><a:t>98.2</a:t>`,
			`sz="1600" b="1"><a:solidFill><a:srgbClr val="` + tm.Theme["text_primary"] + `"/></a:solidFill></a:rPr><a:t> %</a:t>`,
			"<a:t>月の稼働率</a:t>",
			"<a:t>数字の改善は、どれも </a:t>",
		} {
			if !strings.Contains(stats, want) {
				t.Errorf("%s: stat slide lacks %s", name, want)
			}
		}
		quote := read("ppt/slides/slide7.xml")
		for _, want := range []string{"<a:t>“</a:t>", "<a:t>— </a:t>", "<a:t>窓口担当 A、2026 年 9 月の聞き取り</a:t>"} {
			if !strings.Contains(quote, want) {
				t.Errorf("%s: quote slide lacks %s", name, want)
			}
		}
		zr.Close()
	}
}

// 新しいブロックは容量(capacity)とはみ出しの測りに入る。区切り・主張は本文を
// 持たないので容量を出さない。
func TestTextPatternsCapacity(t *testing.T) {
	errs, warns, caps := CheckFileCapacity(textPatternsPath, "")
	if len(errs) != 0 || len(warns) != 0 {
		t.Fatalf("errs %v warns %v", errs, warns)
	}
	var paths []string
	types := map[string]int{}
	for _, c := range caps {
		paths = append(paths, c.Path)
		for _, b := range c.Blocks {
			types[b.Type]++
			if b.Need <= 0 || b.Lines <= 0 {
				t.Errorf("%s: block not measured: %+v", b.Path, b)
			}
		}
	}
	if !reflect.DeepEqual(paths, []string{"/slides/2", "/slides/3", "/slides/6"}) {
		t.Errorf("capacity slides = %v (section and statement must not have one)", paths)
	}
	if types["stat"] != 3 || types["takeaway"] != 3 || types["quote"] != 1 {
		t.Errorf("block types = %v", types)
	}
	// 4つ並んだ数のカードは、行の数が一番多いカードの行(数1行・説明1行)。
	if c := caps[1].Blocks[0]; c.Type != "stat" || c.Lines != 2 {
		t.Errorf("stat block = %+v", c)
	}
}

// 主張の1文が長すぎると、そのスライドに layout-overflow の警告(本文の無い
// スライドなので path はスライド)。区切り・主張は fill-ratio を出さない。
func TestStatementOverflowWarning(t *testing.T) {
	errs, warns := CheckFile("../examples/ir/check-warnings/statement-overflow.json", "")
	if len(errs) != 0 || len(warns) != 1 {
		t.Fatalf("errs %v warns %v", errs, warns)
	}
	w := warns[0]
	if w.Code != "layout-overflow" || w.Path != "/slides/4" || !strings.HasPrefix(w.Message, "statement overflows the slide by ") {
		t.Fatalf("warning = %+v", w)
	}
}

// boxloom schema はスライドの種類と新しい囲みを名乗る。
func TestSchemaListsTextPatterns(t *testing.T) {
	out := schemaOut(t, "light")
	md := out["markdown"].(map[string]any)
	if !reflect.DeepEqual(md["slideAttributes"], []any{"type", "variant"}) {
		t.Errorf("slideAttributes = %v", md["slideAttributes"])
	}
	tm := out["templates"].([]any)[0].(map[string]any)
	var slides, blocks []string
	for _, s := range tm["slides"].([]any) {
		slides = append(slides, s.(map[string]any)["name"].(string))
	}
	for _, b := range tm["blocks"].([]any) {
		blocks = append(blocks, b.(map[string]any)["name"].(string))
	}
	if !reflect.DeepEqual(slides, []string{"content", "section", "statement"}) {
		t.Errorf("slides = %v", slides)
	}
	for _, want := range []string{"takeaway", "stat", "value", "quote"} {
		if !strings.Contains(","+strings.Join(blocks, ",")+",", ","+want+",") {
			t.Errorf("blocks lack %s: %v", want, blocks)
		}
	}
	core := out["irSchema"].(map[string]any)["$defs"].(map[string]any)["core"].(map[string]any)["$defs"].(map[string]any)
	for _, def := range []string{"sectionSlide", "statementSlide", "takeaway", "stat", "statItem", "quote"} {
		if _, ok := core[def]; !ok {
			t.Errorf("irSchema lacks $defs.%s", def)
		}
	}
}
