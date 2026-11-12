package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

const tablePointsDoc = "# 題\n\n" +
	"| 課題 | 手 | 次の手 |\n| --- | --- | --- |\n" +
	"| 遅い | 自動で返す;当番を決める | 仕組みを1つに |\n" +
	"| 多い | 頁に載せる | 分類を見直す\\; 月ごとに;検索を直す;動画を作る |\n" +
	": 説明 {points=手,次の手}\n"

func tableBlockOf(t *testing.T, v any) map[string]any {
	t.Helper()
	return getPath(v, "slides").([]any)[0].(map[string]any)["body"].([]any)[0].(map[string]any)
}

// {points=列名}:名前で指した列のセルは ; で区切った箇条({points:[...]})。項目が1つの
// セルはただの文字列。\; は文の ;。ほかの列の ; はただの文字。
func TestMarkdownTablePoints(t *testing.T) {
	v, errs := mdToAny(t, tablePointsDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	rows := tableBlockOf(t, v)["rows"].([]any)
	r0, r1 := rows[0].([]any), rows[1].([]any)
	pts, ok := r0[1].(map[string]any)
	if !ok || len(pts["points"].([]any)) != 2 || pts["points"].([]any)[1] != "当番を決める" {
		t.Errorf("row 0 col 1: %v", r0[1])
	}
	if r0[2] != "仕組みを1つに" || r1[1] != "頁に載せる" {
		t.Errorf("single items must stay plain strings: %v %v", r0[2], r1[1])
	}
	items := r1[2].(map[string]any)["points"].([]any)
	if len(items) != 3 || items[0] != "分類を見直す; 月ごとに" {
		t.Errorf("\\; must stay a ; in the item: %v", items)
	}
	// 宣言の無い列の ; はただの文字。
	v, errs = mdToAny(t, "# 題\n\n| a | b |\n| --- | --- |\n| x;y | 1 |\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	if c := tableBlockOf(t, v)["rows"].([]any)[0].([]any)[0]; c != "x;y" {
		t.Errorf("a column without points must keep ;: %v", c)
	}
	// 群・合計と一緒に書ける(合計の行のセルも箇条の列なら箇条)。
	v, errs = mdToAny(t, "# 題\n\n| 群 | 項目 | 手 |\n| --- | --- | --- |\n| G | x | a;b |\n| | y | c |\n| 合計 | | d;e |\n: {groups=first total=last points=手}\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl := tableBlockOf(t, v)
	if _, ok := tbl["rows"].([]any)[0].([]any)[1].(map[string]any); !ok {
		t.Errorf("grouped row cell: %v", tbl["rows"])
	}
	if _, ok := tbl["total"].([]any)[2].(map[string]any); !ok {
		t.Errorf("total cell: %v", tbl["total"])
	}
}

func TestMarkdownTablePointsErrors(t *testing.T) {
	head := "# 題\n\n| 群 | 手 |\n| --- | --- |\n"
	cases := []struct {
		name, src, path string
	}{
		{"empty item", head + "| x | a;;b |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/1"},
		{"trailing", head + "| x | a;b; |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/2"},
		{"leading", head + "| x | ;a |\n: {points=手}\n", "/slides/0/body/0/rows/0/1/points/0"},
		{"unknown column", head + "| x | a;b |\n: {points=手立て}\n", "/slides/0/body/0/points"},
		{"one of two unknown", head + "| x | a;b |\n: {points=手,無い列}\n", "/slides/0/body/0/points"},
		{"group column", head + "| x | a;b |\n: {groups=first points=群}\n", "/slides/0/body/0/points"},
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

// IR の points の形の誤り:読み込み(ParseIR)は断り、check は場所つきの table-points を出す。
func TestIRTablePointsErrors(t *testing.T) {
	cases := []struct {
		name, cell, path string
	}{
		{"one item", `{"points":["a"]}`, "/slides/0/body/0/rows/0/1/points"},
		{"empty item", `{"points":["a",""]}`, "/slides/0/body/0/rows/0/1/points/1"},
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
	// 知らないキーは読み込みが断る。
	src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a"],"rows":[[{"points":["a","b"],"x":1}]]}]}]}`
	if _, err := ParseIR([]byte(src)); err == nil {
		t.Error("unknown key in a points cell was accepted")
	}
}

// PPTX:箇条のセルは項目ごとの段落で、印はテンプレートの marks.point の a:buChar。
func TestTablePointsInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(tablePointsDoc))
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
		tbl := regexp.MustCompile(`(?s)<a:tbl>.*</a:tbl>`).FindString(s)
		bu := `<a:buChar char="` + escapeXML(strings.TrimSpace(tm.Marks.Point)) + `"/>`
		if n := strings.Count(tbl, bu); n != 5 {
			t.Errorf("%s: want 5 bullets %s in the table, got %d", name, bu, n)
		}
		tcs := regexp.MustCompile(`(?s)<a:tc>.*?</a:tc>`).FindAllString(tbl, -1)
		if len(tcs) != 9 {
			t.Fatalf("%s: want 9 cells, got %d", name, len(tcs))
		}
		if n := strings.Count(tcs[4], "<a:p>"); n != 2 || !strings.Contains(tcs[4], "当番を決める") || strings.Contains(tcs[4], "<a:buChar") != true {
			t.Errorf("%s: the points cell must have 2 bullet paragraphs: %s", name, tcs[4])
		}
		if strings.Contains(tcs[5], "<a:buChar") {
			t.Errorf("%s: a plain cell must not have bullets: %s", name, tcs[5])
		}
		if !strings.Contains(tcs[8], "分類を見直す; 月ごとに") {
			t.Errorf("%s: \\; must be written as ;", name)
		}
	}
}

// 行の高さはセルの中の箇条の行(項目ごとに折り返した行の和)で決まる。
func TestTablePointsRowHeight(t *testing.T) {
	mark := TextRun{Text: "・", Mark: true}
	item := func(s string) []TextRun { return []TextRun{mark, {Text: s}} }
	long := strings.Repeat("長い項目の文", 8)
	tl := &tableLeaf{size: 10, padX: 4, padY: 2, cells: [][]TableCell{
		{{Runs: []TextRun{{Text: "a"}}}, {Runs: []TextRun{{Text: "b"}}}},
		{{Runs: []TextRun{{Text: "x"}}}, {Items: [][]TextRun{item("一"), item("二"), item("三")}}},
		{{Runs: []TextRun{{Text: "y"}}}, {Items: [][]TextRun{item(long), item("二")}}},
	}}
	widths := []float64{40, 120}
	hs, total := tl.rowHeights(widths)
	lineH := 10 * lineHeightMult
	if d := hs[1] - (3*lineH + 4); d > 1e-6 || d < -1e-6 {
		t.Errorf("3 items must make 3 lines: %v", hs)
	}
	want := runsLines(item(long), 10, 120-8) + 1
	if want < 3 {
		t.Fatalf("the long item should wrap: %d", want)
	}
	if d := hs[2] - (float64(want)*lineH + 4); d > 1e-6 || d < -1e-6 {
		t.Errorf("wrapped items must count their lines: %v (want %d lines)", hs, want)
	}
	if total != 1+3+want {
		t.Errorf("text lines: %d, want %d", total, 1+3+want)
	}
}
