package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// #122:箇条の3段目と、表のセルの中の2段の箇条。

const nestedPointsDoc = "# 題\n\n" +
	"- 一\n  - 二\n    - 三\n    - 三の二\n  - 二の二\n- 一の二\n"

// Markdown の箇条は3段まで入れ子にでき、IR では {content, items} が2重になる。
func TestMarkdownPointsThreeLevels(t *testing.T) {
	v, errs := mdToAny(t, nestedPointsDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	items := tableBlockOf(t, v)["items"].([]any)
	if len(items) != 2 || items[1] != "一の二" {
		t.Fatalf("items = %v", items)
	}
	second := items[0].(map[string]any)["items"].([]any)
	if len(second) != 2 || second[1] != "二の二" {
		t.Fatalf("second level = %v", second)
	}
	third := second[0].(map[string]any)["items"].([]any)
	if fmt.Sprint(third) != "[三 三の二]" {
		t.Fatalf("third level = %v", third)
	}
	// 3段目の番号つきも書ける。
	_, errs = mdToAny(t, "# 題\n\n- 一\n  - 二\n    1. 三\n    1. 三の二\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
}

// IR の箇条の入れ子は3段まで:4段は読み込み(ParseIR)が断り、check も誤りにする。
func TestIRPointsDepth(t *testing.T) {
	deck := func(items string) string {
		return `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"points","items":` + items + `}]}]}`
	}
	three := deck(`[{"content":"a","items":[{"content":"b","items":["c",{"content":"c2"}]}]}]`)
	if _, err := ParseIR([]byte(three)); err != nil {
		t.Fatalf("3 levels rejected: %v", err)
	}
	four := deck(`[{"content":"a","items":[{"content":"b","items":[{"content":"c","items":["d"]}]}]}]`)
	if _, err := ParseIR([]byte(four)); err == nil || !strings.Contains(err.Error(), "at most 3") {
		t.Errorf("4 levels: err = %v", err)
	}
	errs, _ := CheckIR("x.json", []byte(four), "light")
	if len(errs) == 0 {
		t.Error("check accepted 4 levels")
	}
}

// Schema(同梱の写し)で、3段目の項目は items を持てず、セルの項目は {text, children}。
func TestNestedPointsSchema(t *testing.T) {
	b, err := schemaFS.ReadFile("schemas/core.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"pointItem", "pointItem2", "pointItem3", "tableCellPoint"} {
		if doc.Defs[k] == nil {
			t.Fatalf("$defs.%s is missing", k)
		}
	}
	if !strings.Contains(string(doc.Defs["pointItem"]), `"#/$defs/pointItem2"`) || !strings.Contains(string(doc.Defs["pointItem2"]), `"#/$defs/pointItem3"`) {
		t.Error("pointItem must nest pointItem2, and pointItem2 pointItem3")
	}
	if strings.Contains(string(doc.Defs["pointItem3"]), `"items"`) {
		t.Error("pointItem3 must not have items (3 levels at most)")
	}
	if !strings.Contains(string(doc.Defs["tableCellPoint"]), `"children"`) {
		t.Error("tableCellPoint must have children")
	}
}

const cellNestedDoc = "# 題\n\n" +
	"| 課題 | 手 |\n| --- | --- |\n" +
	"| 遅い | 自動で返す; - 受付の時刻; - 担当の名前; 当番を決める |\n" +
	"| 多い | 頁に載せる; - 毎月 |\n" +
	"| 少ない | \\- で始まる;次 |\n" +
	": 説明 {points=手}\n"

// セル:「- 」で始まる項目は直前の項目の子({text, children})。\- は文字の -。
func TestMarkdownTableCellChildren(t *testing.T) {
	v, errs := mdToAny(t, cellNestedDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	rows := tableBlockOf(t, v)["rows"].([]any)
	pts := rows[0].([]any)[1].(map[string]any)["points"].([]any)
	if len(pts) != 2 || pts[1] != "当番を決める" {
		t.Fatalf("row 0 = %v", pts)
	}
	first := pts[0].(map[string]any)
	if first["text"] != "自動で返す" || fmt.Sprint(first["children"]) != "[受付の時刻 担当の名前]" {
		t.Errorf("row 0 item 0 = %v", first)
	}
	// 親と子の2つの段落だけでも箇条のセル。
	one := rows[1].([]any)[1].(map[string]any)["points"].([]any)
	if len(one) != 1 || fmt.Sprint(one[0].(map[string]any)["children"]) != "[毎月]" {
		t.Errorf("row 1 = %v", one)
	}
	esc := rows[2].([]any)[1].(map[string]any)["points"].([]any)
	if esc[0] != "- で始まる" {
		t.Errorf(`\- must be a plain "-": %v`, esc)
	}
	// IR を読み直せる(Schema と同じ形)。
	ir, errs := MarkdownToIR("t.bx.md", []byte(cellNestedDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	if _, err := ParseIR(ir); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownTableCellChildrenErrors(t *testing.T) {
	head := "# 題\n\n| 群 | 手 |\n| --- | --- |\n"
	cases := []struct {
		name, src, path string
	}{
		{"leading child", head + "| x | - a;b |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/0"},
		{"only a child", head + "| x | - a |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/0"},
		{"empty child", head + "| x | a; - ;b |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/1"},
	}
	for _, c := range cases {
		_, errs := mdToAny(t, c.src)
		found := false
		for _, e := range errs {
			if e.Code == codeTablePoints && e.Path == c.path && e.Line > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at %s, got %+v", c.name, codeTablePoints, c.path, errs)
		}
	}
}

// IR のセルの項目の形の誤り:読み込みは断り、check は場所つきの table-points を出す。
func TestIRTableCellChildrenErrors(t *testing.T) {
	cases := []struct {
		name, cell, path string
	}{
		{"no children", `{"points":[{"text":"a","children":[]},"b"]}`, "/slides/0/body/0/rows/0/1/points/0/children"},
		{"empty child", `{"points":[{"text":"a","children":[""]}]}`, "/slides/0/body/0/rows/0/1/points/0/children/0"},
		{"empty text", `{"points":[{"text":"","children":["c"]}]}`, "/slides/0/body/0/rows/0/1/points/0/text"},
	}
	for _, c := range cases {
		src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x",` + c.cell + `]]}]}]}`
		if _, err := ParseIR([]byte(src)); err == nil {
			t.Errorf("%s: ParseIR accepted", c.name)
		}
		errs, _ := CheckIR("x.json", []byte(src), "light")
		found := false
		for _, e := range errs {
			if e.Code == "table-points" && e.Path == c.path {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want table-points at %s, got %+v", c.name, c.path, errs)
		}
	}
	// 子の子・知らないキーは読み込みが断る。
	for _, cell := range []string{
		`{"points":[{"text":"a","children":[{"text":"b","children":["c"]}]}]}`,
		`{"points":[{"text":"a","children":["b"],"x":1}]}`,
	} {
		src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x",` + cell + `]]}]}]}`
		if _, err := ParseIR([]byte(src)); err == nil {
			t.Errorf("accepted %s", cell)
		}
	}
}

func renderMDSlide(t *testing.T, src, template string) string {
	t.Helper()
	ir, errs := MarkdownToIR("t.bx.md", []byte(src))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate(template)
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	return string(pptxParts(t, buf.Bytes())["ppt/slides/slide1.xml"])
}

var rePara = regexp.MustCompile(`(?s)<a:p>.*?</a:p>`)

// paraWith は文 text を持つ段落。
func paraWith(t *testing.T, xml, text string) string {
	t.Helper()
	for _, p := range rePara.FindAllString(xml, -1) {
		if strings.Contains(p, ">"+text+"<") {
			return p
		}
	}
	t.Fatalf("no paragraph with %q", text)
	return ""
}

var reMarL = regexp.MustCompile(`marL="(\d+)"`)

func marL(t *testing.T, p string) int {
	t.Helper()
	m := reMarL.FindStringSubmatch(p)
	if m == nil {
		t.Fatalf("no marL: %s", p)
	}
	var n int
	fmt.Sscan(m[1], &n)
	return n
}

// PPTX:3段目の項目は lvl="2" の段落で、印は marks.subPoint、字下げは2段目より深い。
func TestThreeLevelPointsInPPTX(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		s := renderMDSlide(t, nestedPointsDoc, name)
		tm, _ := LookupTemplate(name)
		p1, p2, p3 := paraWith(t, s, "一"), paraWith(t, s, "二"), paraWith(t, s, "三")
		if strings.Contains(p1, "lvl=") || !strings.Contains(p2, `lvl="1"`) || !strings.Contains(p3, `lvl="2"`) {
			t.Errorf("%s: levels:\n%s\n%s\n%s", name, p1, p2, p3)
		}
		bu := `<a:buChar char="` + escapeXML(strings.TrimSpace(tm.Marks.SubPoint)) + `"/>`
		if !strings.Contains(p3, bu) {
			t.Errorf("%s: the 3rd level must use %s: %s", name, bu, p3)
		}
		if !(marL(t, p1) < marL(t, p2) && marL(t, p2) < marL(t, p3)) {
			t.Errorf("%s: each level must be indented more: %d %d %d", name, marL(t, p1), marL(t, p2), marL(t, p3))
		}
		if !strings.Contains(p3, fmt.Sprintf(`sz="%d"`, int(tm.Values.SubPoint*100))) {
			t.Errorf("%s: the 3rd level must use the subPoint size: %s", name, p3)
		}
	}
}

// PPTX:セルの子の項目は lvl="1" の段落で、印は marks.subPoint、親の印の幅だけ深く字下げする。
func TestTableCellChildrenInPPTX(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		s := renderMDSlide(t, cellNestedDoc, name)
		tm, _ := LookupTemplate(name)
		tbl := regexp.MustCompile(`(?s)<a:tbl>.*</a:tbl>`).FindString(s)
		parent, child := paraWith(t, tbl, "自動で返す"), paraWith(t, tbl, "受付の時刻")
		if strings.Contains(parent, "lvl=") || !strings.Contains(child, `lvl="1"`) {
			t.Errorf("%s: levels:\n%s\n%s", name, parent, child)
		}
		bu := `<a:buChar char="` + escapeXML(strings.TrimSpace(tm.Marks.SubPoint)) + `"/>`
		if !strings.Contains(child, bu) {
			t.Errorf("%s: the child must use %s: %s", name, bu, child)
		}
		if marL(t, child) <= marL(t, parent) {
			t.Errorf("%s: the child must be indented more: %d <= %d", name, marL(t, child), marL(t, parent))
		}
	}
}

// 行の高さにセルの子の項目の行が入り、子は字下げの分だけ狭い幅で折り返す。
func TestTableCellChildrenRowHeight(t *testing.T) {
	mark := TextRun{Text: "・", Mark: true}
	sub := TextRun{Text: "– ", Mark: true}
	long := strings.Repeat("長い項目の文", 8)
	cell := TableCell{
		Items:  [][]TextRun{{mark, {Text: "親"}}, {sub, {Text: "子"}}, {sub, {Text: long}}},
		Levels: []int{0, 1, 1},
	}
	tl := &tableLeaf{size: 10, padX: 4, padY: 2, cells: [][]TableCell{
		{{Runs: []TextRun{{Text: "a"}}}, {Runs: []TextRun{{Text: "b"}}}},
		{{Runs: []TextRun{{Text: "x"}}}, cell},
	}}
	widths := []float64{40, 120}
	hs, _ := tl.rowHeights(widths)
	ind := naturalRunsWidth([]TextRun{mark}, 10)
	if ind <= 0 {
		t.Fatal("the mark has no width")
	}
	want := 2 + runsLines([]TextRun{sub, {Text: long}}, 10, 120-8-ind)
	if want < 4 {
		t.Fatalf("the long child should wrap: %d", want)
	}
	lineH := 10 * lineHeightMult()
	if d := hs[1] - (float64(want)*lineH + 4); d > 1e-6 || d < -1e-6 {
		t.Errorf("row height %v, want %d lines", hs[1], want)
	}
	if cell.indent(0, 10) != 0 || cell.indent(2, 10) != ind {
		t.Errorf("indent = %v %v", cell.indent(0, 10), cell.indent(2, 10))
	}
}
