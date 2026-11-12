package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

const tableGroupsDoc = "# 題\n\n" +
	"| 群 | 項目 | 件数 |\n| --- | --- | --- |\n" +
	"| 手続き | 依頼 | 4 |\n| | 承認 | 3 |\n| | 記録 | 2 |\n| 体制 | 窓口 | 1 |\n| 合計 | | 10 |\n" +
	": 説明 {groups=first total=last}\n"

// {groups=first}:先頭の列が群の列になり、空のセルは上の群の続き。本体の行は群の列を
// 除いたセル。合計の行は群に入らず、見出しと同じ数のセル。
func TestMarkdownTableGroups(t *testing.T) {
	v, errs := mdToAny(t, tableGroupsDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl := getPath(v, "slides").([]any)[0].(map[string]any)["body"].([]any)[0].(map[string]any)
	groups := tbl["groups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %v", groups)
	}
	g0 := groups[0].(map[string]any)
	g1 := groups[1].(map[string]any)
	if g0["label"] != "手続き" || g0["rows"] != float64(3) || g1["label"] != "体制" || g1["rows"] != float64(1) {
		t.Errorf("groups: %v", groups)
	}
	rows := tbl["rows"].([]any)
	if len(rows) != 4 || len(rows[0].([]any)) != 2 || rows[1].([]any)[0] != "承認" {
		t.Errorf("rows: %v", rows)
	}
	if total := tbl["total"].([]any); len(total) != 3 || total[0] != "合計" {
		t.Errorf("total: %v", total)
	}
	// {groups=first} の無い表の空のセルは、今までどおりただの空のセル。
	v, errs = mdToAny(t, "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n| | 2 |\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl = getPath(v, "slides").([]any)[0].(map[string]any)["body"].([]any)[0].(map[string]any)
	if _, ok := tbl["groups"]; ok || len(tbl["rows"].([]any)[1].([]any)) != 2 {
		t.Errorf("a table without groups must keep empty cells: %v", tbl)
	}
}

func TestMarkdownTableGroupsErrors(t *testing.T) {
	cases := []struct {
		name, src, code, path string
	}{
		{"first row empty", "# 題\n\n| 群 | b |\n| --- | --- |\n| | 1 |\n| x | 2 |\n: {groups=first}\n", codeTableGroups, "/slides/0/body/0/groups/0/label"},
		{"value", "# 題\n\n| 群 | b |\n| --- | --- |\n| x | 1 |\n: {groups=last}\n", codeInvalidValue, "/slides/0/body/0/groups"},
		{"one column", "# 題\n\n| 群 |\n| --- |\n| x |\n: {groups=first}\n", codeMisplaced, "/slides/0/body/0/groups"},
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

// IR の groups の形の誤り:読み込み(ParseIR)は断り、check は場所つきの table-groups を出す。
func TestIRTableGroupsErrors(t *testing.T) {
	cases := []struct {
		name, table, path, code string
	}{
		{"sum", `"header":["g","a"],"rows":[["x"],["y"]],"groups":[{"label":"G","rows":1}]`, "/slides/0/body/0/groups", "table-groups"},
		{"empty label", `"header":["g","a"],"rows":[["x"]],"groups":[{"label":" ","rows":1}]`, "/slides/0/body/0/groups/0/label", "table-groups"},
		{"cells", `"header":["g","a"],"rows":[["x","y"]],"groups":[{"label":"G","rows":1}]`, "/slides/0/body/0/rows/0", "table-shape"},
		{"one column", `"header":["g"],"rows":[[]],"groups":[{"label":"G","rows":1}]`, "/slides/0/body/0/groups", "table-groups"},
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
}

// PPTX:群の名前は先頭の列の縦に結合したセル(rowSpan と vMerge)。
func TestTableGroupsInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(tableGroupsDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dark", "light"} {
		tm, _ := LookupTemplate(name)
		var buf bytes.Buffer
		if _, err := tm.RenderDeck(d, &buf); err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		var s string
		for _, f := range zr.File {
			if f.Name == "ppt/slides/slide1.xml" {
				rc, _ := f.Open()
				b, _ := io.ReadAll(rc)
				rc.Close()
				s = string(b)
			}
		}
		trs := regexp.MustCompile(`(?s)<a:tr .*?</a:tr>`).FindAllString(s, -1)
		if len(trs) != 6 {
			t.Fatalf("%s: want 6 rows (header, 4 body, total), got %d", name, len(trs))
		}
		firstTC := func(tr string) string { return regexp.MustCompile(`<a:tc( [^>]*)?>`).FindString(tr) }
		want := []string{`<a:tc>`, `<a:tc rowSpan="3">`, `<a:tc vMerge="1">`, `<a:tc vMerge="1">`, `<a:tc>`, `<a:tc>`}
		for i, tr := range trs {
			got := firstTC(tr)
			if got != want[i] {
				t.Errorf("%s row %d: first cell %q, want %q", name, i, got, want[i])
			}
		}
		if !strings.Contains(trs[1], "手続き") || !strings.Contains(trs[4], "体制") {
			t.Errorf("%s: group labels are not in the first rows of their groups", name)
		}
		if strings.Count(s, "rowSpan=") != 1 || strings.Count(s, `vMerge="1"`) != 2 {
			t.Errorf("%s: want one rowSpan and two vMerge", name)
		}
	}
}

// 群の名前が結合した行の高さに収まらなければ、群の最後の行が伸びる(行ごとに測る)。
func TestTableGroupLabelGrowsLastRow(t *testing.T) {
	long := []TextRun{{Text: strings.Repeat("長い群の名前", 12)}}
	tl := &tableLeaf{size: 10, padX: 4, padY: 2, cells: [][]TableCell{
		{{Runs: []TextRun{{Text: "群"}}}, {Runs: []TextRun{{Text: "a"}}}},
		{{Runs: long, RowSpan: 2}, {Runs: []TextRun{{Text: "x"}}}},
		{{VMerge: true}, {Runs: []TextRun{{Text: "y"}}}},
		{{Runs: []TextRun{{Text: "G"}}}, {Runs: []TextRun{{Text: "z"}}}},
	}}
	hs, _ := tl.rowHeights([]float64{60, 200})
	if hs[1] != hs[3] || hs[2] <= hs[1] {
		t.Fatalf("the last row of the group should grow and the others stay: %v", hs)
	}
	need := float64(runsLines(long, 10, 60-8))*10*lineHeightMult + 4
	if hs[1]+hs[2] < need-1e-6 {
		t.Errorf("rows %v do not hold the label (%v)", hs, need)
	}
}
