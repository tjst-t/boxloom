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

// 区分(先頭の列)と支援事業(2列目)が群の列。県の最初の行の支援事業は、国の群の
// 最後と同じ名前でも左の群の境で新しい群になる(書き直す)。
const tableGroupColumnsDoc = "# 題\n\n" +
	"| 区分 | 支援事業 | 制度 | 上限 |\n| --- | --- | --- | --- |\n" +
	"| 国 | 創業 | 補助 | 200 |\n| | | 派遣 | 10 |\n| | 設備 | 補助 | 1000 |\n" +
	"| 県 | 設備 | 貸付 | 3000 |\n| | | 利子 | 1 |\n| 合計 | | | 4211 |\n" +
	": 説明 {groups=区分,支援事業 total=last}\n"

func firstTable(t *testing.T, v any) map[string]any {
	t.Helper()
	return getPath(v, "slides").([]any)[0].(map[string]any)["body"].([]any)[0].(map[string]any)
}

func groupSummary(gs any) string {
	var out []string
	for _, g := range gs.([]any) {
		gm := g.(map[string]any)
		b, _ := json.Marshal(gm["label"])
		out = append(out, string(b)+"x"+jsonNum(gm["rows"]))
	}
	return strings.Join(out, ",")
}

func jsonNum(v any) string { b, _ := json.Marshal(v); return string(b) }

// {groups=列名,…}:名前で指した列が群の列。first と先頭の列の名前は IR の groups、
// 2列目以降は groupColumns。本体の行は群の列を除いたセル、合計の行は見出しと同じ数。
func TestMarkdownTableGroupColumns(t *testing.T) {
	v, errs := mdToAny(t, tableGroupColumnsDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl := firstTable(t, v)
	if got := groupSummary(tbl["groups"]); got != `"国"x3,"県"x2` {
		t.Errorf("groups: %s", got)
	}
	gcs := tbl["groupColumns"].([]any)
	if len(gcs) != 1 || gcs[0].(map[string]any)["column"] != float64(1) {
		t.Fatalf("groupColumns: %v", gcs)
	}
	if got := groupSummary(gcs[0].(map[string]any)["groups"]); got != `"創業"x2,"設備"x1,"設備"x2` {
		t.Errorf("groupColumns[0].groups: %s", got)
	}
	rows := tbl["rows"].([]any)
	if len(rows) != 5 || len(rows[0].([]any)) != 2 || rows[1].([]any)[0] != "派遣" {
		t.Errorf("rows: %v", rows)
	}
	if total := tbl["total"].([]any); len(total) != 4 {
		t.Errorf("total: %v", total)
	}
	// first と先頭の列の名前は同じ(今までの {groups=first} と同じ IR)。
	for _, attr := range []string{"groups=first", "groups=群"} {
		a, errs := mdToAny(t, strings.Replace(tableGroupsDoc, "groups=first", attr, 1))
		if len(errs) > 0 {
			t.Fatal(MarkdownErrors(errs))
		}
		b, _ := mdToAny(t, tableGroupsDoc)
		if !jsonEqual(a, b) {
			t.Errorf("{%s} differs from {groups=first}", attr)
		}
	}
	// 2列目だけ:groups は無く groupColumns だけ。
	v, errs = mdToAny(t, "# 題\n\n| 月 | 事業 | 窓口 |\n| --- | --- | --- |\n| 4 | 創業 | A |\n| 5 | | A |\n| 6 | 設備 | B |\n: {groups=事業}\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl = firstTable(t, v)
	if _, ok := tbl["groups"]; ok {
		t.Errorf("groups must be absent: %v", tbl)
	}
	gc := tbl["groupColumns"].([]any)[0].(map[string]any)
	if gc["column"] != float64(1) || groupSummary(gc["groups"]) != `"創業"x2,"設備"x1` {
		t.Errorf("groupColumns: %v", gc)
	}
	if r := tbl["rows"].([]any); len(r[1].([]any)) != 2 || r[1].([]any)[0] != "5" {
		t.Errorf("rows: %v", r)
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func TestMarkdownTableGroupColumnsErrors(t *testing.T) {
	head := "# 題\n\n| 区分 | 事業 | b |\n| --- | --- | --- |\n"
	cases := []struct {
		name, src, code, path string
	}{
		{"unknown name", head + "| x | y | 1 |\n: {groups=区分,支援}\n", codeInvalidValue, "/slides/0/body/0/groups"},
		{"twice", head + "| x | y | 1 |\n: {groups=first,区分}\n", codeInvalidValue, "/slides/0/body/0/groups"},
		{"no other column", head + "| x | y | 1 |\n: {groups=区分,事業,b}\n", codeMisplaced, "/slides/0/body/0/groups"},
		{"first row empty", head + "| x | | 1 |\n: {groups=区分,事業}\n", codeTableGroups, "/slides/0/body/0/groupColumns/0/groups/0/label"},
		{"crosses", head + "| x | y | 1 |\n| z | | 2 |\n: {groups=区分,事業}\n", codeTableGroups, "/slides/0/body/0/groupColumns/0/groups/1/label"},
		{"points on a group column", head + "| x | y | 1 |\n: {groups=事業 points=事業}\n", codeTablePoints, "/slides/0/body/0/points"},
		{"continues differs", head + "| x | y | 1 |\n: {groups=区分,事業}\n\n# 続き\n\n| --- | --- | --- |\n| x | y | 1 |\n: {continues groups=区分}\n", codeTableContinues, "/slides/1/body/0/groups"},
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
}

// IR の groupColumns の形の誤り:読み込み(ParseIR)は断り、check は場所つきの誤りを出す。
func TestIRTableGroupColumnsErrors(t *testing.T) {
	g := func(rows ...int) string {
		var parts []string
		for i, r := range rows {
			parts = append(parts, `{"label":"G`+string(rune('a'+i))+`","rows":`+jsonNum(r)+`}`)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	hdr := `"header":["a","b","c"],"rows":[["x"],["y"],["z"]]`
	cases := []struct {
		name, table, path, code string
	}{
		{"crosses", hdr + `,"groups":` + g(1, 2) + `,"groupColumns":[{"column":1,"groups":` + g(2, 1) + `}]`, "/slides/0/body/0/groupColumns/0/groups", "table-groups"},
		{"column 0", `"header":["a","b"],"rows":[["x"]],"groupColumns":[{"column":0,"groups":` + g(1) + `}]`, "/slides/0/body/0/groupColumns/0/column", "table-groups"},
		{"order", `"header":["a","b","c","d"],"rows":[["x"]],"groupColumns":[{"column":2,"groups":` + g(1) + `},{"column":1,"groups":` + g(1) + `}]`, "/slides/0/body/0/groupColumns/1/column", "table-groups"},
		{"past the header", `"header":["a","b"],"rows":[["x"]],"groupColumns":[{"column":2,"groups":` + g(1) + `}]`, "/slides/0/body/0/groupColumns/0/column", "table-groups"},
		{"sum", hdr + `,"groups":` + g(3) + `,"groupColumns":[{"column":1,"groups":` + g(2) + `}]`, "/slides/0/body/0/groupColumns/0/groups", "table-groups"},
		{"cells", `"header":["a","b","c"],"rows":[["x","y"]],"groups":` + g(1) + `,"groupColumns":[{"column":1,"groups":` + g(1) + `}]`, "/slides/0/body/0/rows/0", "table-shape"},
		{"only group columns", `"header":["a","b"],"rows":[[]],"groups":` + g(1) + `,"groupColumns":[{"column":1,"groups":` + g(1) + `}]`, "/slides/0/body/0/groupColumns", "table-groups"},
	}
	for _, c := range cases {
		src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table",` + c.table + `}]}]}`
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
	// 続きの表は最初の表と同じ群の列を持つ。
	src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a","b","c"],"rows":[["x"]],"groupColumns":[{"column":1,"groups":` + g(1) + `}]}]},` +
		`{"type":"content","title":"u","body":[{"type":"table","continues":true,"rows":[["x"]],"groupColumns":[{"column":2,"groups":` + g(1) + `}]}]}]}`
	if d, err := ParseIR([]byte(src)); err == nil {
		if err := linkTableChains(d); err == nil {
			t.Errorf("continues with other group columns: accepted")
		}
	}
	errs, _ := CheckIR("x.json", []byte(src), "light")
	found := false
	for _, e := range errs {
		found = found || (e.Code == "table-continues" && e.Path == "/slides/1/body/0/groupColumns")
	}
	if !found {
		t.Errorf("continues with other group columns: want table-continues, got %+v", errs)
	}
}

func tgcSlideXML(t *testing.T, pptx []byte, n int) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide"+jsonNum(n)+".xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return string(b)
		}
	}
	t.Fatalf("no slide %d", n)
	return ""
}

// PPTX:2列目の群も縦に結合したセル(rowSpan と vMerge)。左の群の境(県の始まり)で
// 2列目の結合も切れる。report は先頭の群だけ縦書きの帯で、2列目は横書き。
func TestTableGroupColumnsInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(tableGroupColumnsDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tcRe := regexp.MustCompile(`<a:tc( [^>]*)?>`)
	for _, name := range []string{"dark", "light", "report"} {
		tm, _ := LookupTemplate(name)
		var buf bytes.Buffer
		if _, err := tm.RenderDeck(d, &buf); err != nil {
			t.Fatal(err)
		}
		s := tgcSlideXML(t, buf.Bytes(), 1)
		trs := regexp.MustCompile(`(?s)<a:tr .*?</a:tr>`).FindAllString(s, -1)
		if len(trs) != 7 {
			t.Fatalf("%s: want 7 rows (header, 5 body, total), got %d", name, len(trs))
		}
		want := [][2]string{
			{`<a:tc>`, `<a:tc>`},
			{`<a:tc rowSpan="3">`, `<a:tc rowSpan="2">`},
			{`<a:tc vMerge="1">`, `<a:tc vMerge="1">`},
			{`<a:tc vMerge="1">`, `<a:tc>`},              // 国の中の「設備」は1行(結合しない)
			{`<a:tc rowSpan="2">`, `<a:tc rowSpan="2">`}, // 県の境で2列目も新しい群
			{`<a:tc vMerge="1">`, `<a:tc vMerge="1">`},
			{`<a:tc>`, `<a:tc>`},
		}
		for i, tr := range trs {
			tcs := tcRe.FindAllString(tr, -1)
			if len(tcs) != 4 {
				t.Fatalf("%s row %d: %d cells", name, i, len(tcs))
			}
			if tcs[0] != want[i][0] || tcs[1] != want[i][1] {
				t.Errorf("%s row %d: cells %q %q, want %q %q", name, i, tcs[0], tcs[1], want[i][0], want[i][1])
			}
			for _, tc := range tcs[2:] {
				if tc != `<a:tc>` {
					t.Errorf("%s row %d: a content cell is merged: %q", name, i, tc)
				}
			}
		}
		if !strings.Contains(trs[4], "設備") || !strings.Contains(trs[4], "県") {
			t.Errorf("%s: the first row of 県 must name its 支援事業 again", name)
		}
		// 縦書きは report の先頭の列だけ(2列目は横書き)。
		cells := regexp.MustCompile(`(?s)<a:tc[ >].*?</a:tc>`).FindAllString(trs[1], -1)
		if (name == "report") != strings.Contains(cells[0], `vert="eaVert"`) || strings.Contains(cells[1], `vert="eaVert"`) {
			t.Errorf("%s: vertical text must be only on the first group column of report", name)
		}
	}
}

// 続きの表でも2列目の群の列を引き継ぐ(例の deck の2枚目)。
func TestTableGroupColumnsContinues(t *testing.T) {
	src, err := os.ReadFile("../examples/md/table-group-columns.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	ir, errs := MarkdownToIR("t.bx.md", src)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	s := tgcSlideXML(t, buf.Bytes(), 3) // 表紙・1枚目・続き
	trs := regexp.MustCompile(`(?s)<a:tr .*?</a:tr>`).FindAllString(s, -1)
	if len(trs) != 4 || !strings.Contains(trs[0], "支援事業") {
		t.Fatalf("the continuing table repeats the header: %d rows", len(trs))
	}
	tcs := regexp.MustCompile(`<a:tc( [^>]*)?>`).FindAllString(trs[1], -1)
	if tcs[0] != `<a:tc rowSpan="3">` || tcs[1] != `<a:tc rowSpan="2">` {
		t.Errorf("continuing table: first body row cells %q %q", tcs[0], tcs[1])
	}
}
