package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

const tableContinuesExample = "../examples/md/table-continues.bx.md"

// pptxSlides は組んだ PPTX の slideN.xml(N は 1 から)。
func pptxSlides(t *testing.T, d *IRDeck, template string) map[int]string {
	t.Helper()
	tm, err := LookupTemplate(template)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]string{}
	re := regexp.MustCompile(`^ppt/slides/slide([0-9]+)\.xml$`)
	for _, f := range zr.File {
		if m := re.FindStringSubmatch(f.Name); m != nil {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			n := 0
			for _, c := range m[1] {
				n = n*10 + int(c-'0')
			}
			out[n] = string(b)
		}
	}
	return out
}

func readExampleDeck(t *testing.T, path string) *IRDeck {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ReadDeck(path, src)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return d
}

// 続けた表は、各枚に最初の表の見出しの行と「n/m」の印を持ち、列の幅が全部の枚で同じ。
func TestTableContinuesInPPTX(t *testing.T) {
	d := readExampleDeck(t, tableContinuesExample)
	gridRe := regexp.MustCompile(`<a:tblGrid>.*?</a:tblGrid>`)
	trRe := regexp.MustCompile(`(?s)<a:tr .*?</a:tr>`)
	for _, name := range []string{"dark", "light", "report"} {
		slides := pptxSlides(t, d, name)
		var grid string
		for n, want := range map[int]string{2: "1/3", 3: "2/3", 4: "3/3"} {
			s := slides[n]
			trs := trRe.FindAllString(s, -1)
			if len(trs) < 2 {
				t.Fatalf("%s slide %d: no table rows", name, n)
			}
			for _, h := range []string{"仕組み", "使う部署", "移す回", "利用者(人)", "移すときに気をつけること"} {
				if !strings.Contains(trs[0], ">"+h+"<") {
					t.Errorf("%s slide %d: header row lacks %q", name, n, h)
				}
			}
			if !strings.Contains(s, "<a:t>"+want+"</a:t>") {
				t.Errorf("%s slide %d: no mark %q", name, n, want)
			}
			g := gridRe.FindString(s)
			if g == "" {
				t.Fatalf("%s slide %d: no grid", name, n)
			}
			if grid == "" {
				grid = g
			} else if g != grid {
				t.Errorf("%s slide %d: column widths differ:\n%s\n%s", name, n, g, grid)
			}
			// 箇条の列(points)は最初の表から引き継ぐ。
			if !strings.Contains(trs[1], "<a:buChar") {
				t.Errorf("%s slide %d: the points column is not inherited", name, n)
			}
		}
		// 説明の文は続きの枚でも繰り返す。
		if !strings.Contains(slides[4], "移す仕組みの一覧") {
			t.Errorf("%s: the caption is not repeated", name)
		}
	}
}

// 続きの無い表には印を付けない(従来の表の出力は変わらない)。
func TestTableWithoutContinuesHasNoMark(t *testing.T) {
	d := readExampleDeck(t, "../examples/md/table-points.bx.md")
	for _, s := range pptxSlides(t, d, "light") {
		if regexp.MustCompile(`<a:t>[0-9]+/[0-9]+</a:t>`).MatchString(s) {
			t.Errorf("a table without continues has a mark")
		}
	}
}

// 列の幅はひと続きの全部の行で測る:長いセルが続きの表にだけあっても、最初の表の
// 列も同じ幅になる。
func TestTableContinuesWidthsFromAllRows(t *testing.T) {
	src := `{"boxloom":"0.1","slides":[
	 {"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x","1"]]}]},
	 {"type":"content","title":"t","body":[{"type":"table","continues":true,"rows":[["とても長い長い長い長い長い長い長いセルの文","2"]]}]}]}`
	d, err := ParseIR([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	slides := pptxSlides(t, d, "light")
	g1 := regexp.MustCompile(`<a:tblGrid>.*?</a:tblGrid>`).FindString(slides[1])
	g2 := regexp.MustCompile(`<a:tblGrid>.*?</a:tblGrid>`).FindString(slides[2])
	if g1 == "" || g1 != g2 {
		t.Errorf("widths differ: %s / %s", g1, g2)
	}
	// 1つだけの表なら幅は違う(試験が意味を持つことを確かめる)。
	alone, _ := ParseIR([]byte(`{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x","1"]]}]}]}`))
	if g := regexp.MustCompile(`<a:tblGrid>.*?</a:tblGrid>`).FindString(pptxSlides(t, alone, "light")[1]); g == g1 {
		t.Errorf("a lone table has the same widths as the chain: %s", g)
	}
}

// 続きの形の外れは、読み込み(ParseIR)が断り、check は場所つきの誤りを出す。
func TestTableContinuesErrors(t *testing.T) {
	first := `{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x","1"]]}]}`
	cont := func(table string) string {
		return `{"type":"content","title":"t","body":[{"type":"table","continues":true,` + table + `}]}`
	}
	cases := []struct {
		name, slides, code, path string
	}{
		{"slide between", first + `,{"type":"section","title":"s"},` + cont(`"rows":[["y","2"]]`), "table-continues", "/slides/2/body/0/continues"},
		{"no previous table", `{"type":"content","title":"t","body":[{"type":"paragraph","content":"p"}]},` + cont(`"rows":[["y","2"]]`), "table-continues", "/slides/1/body/0/continues"},
		{"first slide", cont(`"rows":[["y","2"]]`), "table-continues", "/slides/0/body/0/continues"},
		{"columns differ", first + `,` + cont(`"rows":[["y","2","3"]]`), "table-shape", "/slides/1/body/0/rows/0"},
		{"header written", first + `,` + cont(`"header":["a","b"],"rows":[["y","2"]]`), "table-continues", "/slides/1/body/0/header"},
		{"groups differ", first + `,` + cont(`"rows":[["2"]],"groups":[{"label":"G","rows":1}]`), "table-continues", "/slides/1/body/0/groups"},
		{"total before the end", `{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x","1"]],"total":["計","1"]}]},` + cont(`"rows":[["y","2"]]`), "table-continues", "/slides/0/body/0/total"},
		{"not the first table", first + `,{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["z","3"]]},{"type":"table","continues":true,"rows":[["y","2"]]}]}`, "table-continues", "/slides/1/body/1/continues"},
		{"inside columns", first + `,{"type":"content","title":"t","body":[{"type":"columns","columns":[{"body":[{"type":"table","continues":true,"rows":[["y","2"]]}]},{"body":[{"type":"paragraph","content":"p"}]}]}]}`, "table-continues", "/slides/1/body/0/columns/0/body/0/continues"},
	}
	for _, c := range cases {
		src := `{"boxloom":"0.1","slides":[` + c.slides + `]}`
		var probe any
		if err := json.Unmarshal([]byte(src), &probe); err != nil {
			t.Fatalf("%s: bad test JSON: %v", c.name, err)
		}
		if _, err := ParseIR([]byte(src)); err == nil {
			t.Errorf("%s: ParseIR accepted", c.name)
		}
		errs, _ := CheckIR("x.json", []byte(src), "light")
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.Path == c.path {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at %s, got %+v", c.name, c.code, c.path, errs)
		}
	}
	// 正しいひと続き(3枚、合計の行は最後)は通る。
	ok := `{"boxloom":"0.1","slides":[` + first + `,` + cont(`"rows":[["y","2"]]`) + `,` + cont(`"rows":[["z","3"]],"total":["計","6"]`) + `]}`
	if _, err := ParseIR([]byte(ok)); err != nil {
		t.Fatal(err)
	}
	if errs, _ := CheckIR("x.json", []byte(ok), "light"); len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
}

// Markdown:{continues} の表は区切りの行から始め、見出しの行・groups・points を前の表から
// 引き継ぐ。外れは Issue(行つき)。
func TestMarkdownTableContinues(t *testing.T) {
	doc := "# 題\n\n| 群 | 項目 | メモ |\n| --- | --- | --- |\n| G | a | x; y |\n: 説明 {groups=first points=メモ}\n\n" +
		"# 題\n\n| --- | --- | --- |\n| H | b | p; q |\n| | c | r |\n: {continues}\n"
	v, errs := mdToAny(t, doc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl := getPath(v, "slides").([]any)[1].(map[string]any)["body"].([]any)[0].(map[string]any)
	if tbl["continues"] != true {
		t.Errorf("continues: %v", tbl)
	}
	if _, ok := tbl["header"]; ok {
		t.Errorf("a continuing table has no header: %v", tbl)
	}
	if g, ok := tbl["groups"].([]any); !ok || len(g) != 1 || g[0].(map[string]any)["rows"] != float64(2) {
		t.Errorf("groups are not inherited: %v", tbl["groups"])
	}
	rows := tbl["rows"].([]any)
	if p, ok := rows[0].([]any)[1].(map[string]any); !ok || len(p["points"].([]any)) != 2 {
		t.Errorf("points are not inherited: %v", rows)
	}

	cases := []struct {
		name, src, code, path string
	}{
		{"header written", "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 題\n\n| a | b |\n| --- | --- |\n| y | 2 |\n: {continues}\n", codeTableContinues, "/slides/1/body/0/header"},
		{"columns differ", "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 題\n\n| --- | --- | --- |\n| y | 2 | 3 |\n: {continues}\n", codeTableContinues, "/slides/1/body/0"},
		{"no continues", "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 題\n\n| --- | --- |\n| y | 2 |\n", codeMisplaced, "/slides/1/body/0"},
		{"value", "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 題\n\n| --- | --- |\n| y | 2 |\n: {continues=true}\n", codeInvalidValue, "/slides/1/body/0"},
		{"groups differ", "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 題\n\n| --- | --- |\n| y | 2 |\n: {continues groups=first}\n", codeTableContinues, "/slides/1/body/0/groups"},
	}
	for _, c := range cases {
		_, errs := mdToAny(t, c.src)
		found := false
		for _, e := range errs {
			if e.Code == c.code && e.Path == c.path && e.Line > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at %s, got %+v", c.name, c.code, c.path, errs)
		}
	}
	// 間にスライドが挟まる形は、IR の検査が Markdown の行つきで言う。
	src := "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n\n# 間\n\n本文\n\n# 題\n\n| --- | --- |\n| y | 2 |\n: {continues}\n"
	res := Check("x.bx.md", []byte(src), CheckOptions{Template: "light"})
	found := false
	for _, e := range res.Errors {
		if e.Code == "table-continues" && e.Path == "/slides/2/body/0" && e.Line == 13 {
			found = true
		}
	}
	if !found {
		t.Errorf("want table-continues with a line, got %+v", res.Errors)
	}
}

// はみ出した表に、check は分け方の案(枚数と分ける行)を hint と容量に出す。案に
// 従って分けると、どの枚もはみ出さない。
func TestTableOverflowSplitProposal(t *testing.T) {
	for _, f := range []string{"../examples/ir/check-warnings/table-overflow.json", "../examples/md/warnings/table-overflow.bx.md"} {
		errs, warns, caps := CheckFileCapacity(f, "")
		if len(errs) > 0 {
			t.Fatalf("%s: %+v", f, errs)
		}
		var hint string
		for _, w := range warns {
			if w.Code == "layout-overflow" && w.Path == "/slides/1/body/0" {
				hint = w.Hint
			}
		}
		if !strings.Contains(hint, "3 枚に分けられます") || !strings.Contains(hint, "rows[7]") || !strings.Contains(hint, "continues") {
			t.Errorf("%s: hint %q", f, hint)
		}
		var split *TableSplitCapacity
		for _, c := range caps {
			for _, b := range c.Blocks {
				if b.Path == "/slides/1/body/0" {
					split = b.Split
				}
			}
		}
		if split == nil || split.Tables != 3 || len(split.SplitBefore) != 2 || split.SplitBefore[0] != 7 || split.SplitBefore[1] != 14 ||
			jsonOf(split.Rows) != "[7,7,4]" {
			t.Errorf("%s: split %+v", f, split)
		}
	}

	// 案のとおりに分ける。
	src, err := os.ReadFile("../examples/ir/check-warnings/table-overflow.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	slides := doc["slides"].([]any)
	slide := slides[1].(map[string]any)
	tbl := slide["body"].([]any)[0].(map[string]any)
	rows := tbl["rows"].([]any)
	total := tbl["total"]
	delete(tbl, "total")
	tbl["rows"] = rows[:7]
	mk := func(rs []any, last bool) map[string]any {
		t := map[string]any{"type": "table", "continues": true, "rows": rs}
		if last {
			t["total"] = total
		}
		return map[string]any{"type": "content", "title": slide["title"], "body": []any{t}}
	}
	doc["slides"] = append(slides, mk(rows[7:14], false), mk(rows[14:], true))
	b, _ := json.Marshal(doc)
	errs, warns := CheckIR("split.json", b, "")
	if len(errs) > 0 {
		t.Fatalf("%+v", errs)
	}
	for _, w := range warns {
		if w.Code == "layout-overflow" || w.Code == "subsystem-min" {
			t.Errorf("the split table still overflows: %+v", w)
		}
	}
}

func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// 印の書き方は {n} と {m} を持つ(値の表の検査)。
func TestTableContinuedMark(t *testing.T) {
	m := TemplateMarks{TableContinued: "続く {n}/{m}"}
	if got := m.continuedMark(2, 7); got != "続く 2/7" {
		t.Errorf("got %q", got)
	}
	tm, _ := LookupTemplate("light")
	bad := tm.Marks
	bad.TableContinued = "{n}"
	if err := checkMarks(bad); err == nil || !strings.Contains(err.Error(), "tableContinued") {
		t.Errorf("a mark without {m} must be rejected: %v", err)
	}
}
