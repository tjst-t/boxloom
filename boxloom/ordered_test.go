package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 番号つきの箇条(#85)。架空の手順。
const orderedDoc = `---
title: 番号
template: light
cover: false
---

# 申し込みの手順

1. 窓口で用紙を受け取る
2. 用紙に名前を書く
   - 通称でもよい
   - 2行に分けない
3. 係に渡す
   1. 係が確かめる
   1. 係が控えを返す
`

func orderedPart(t *testing.T, pptx []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return string(b)
		}
	}
	t.Fatalf("%s not found", name)
	return ""
}

// Markdown の 1. は IR の ordered: true。入れ子は入れ子ごとに独立。
func TestMarkdownOrderedPoints(t *testing.T) {
	v, errs := mdToAny(t, orderedDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	b := tableBlockOf(t, v)
	if b["type"] != "points" || b["ordered"] != true {
		t.Fatalf("points must be ordered: %v", b)
	}
	items := b["items"].([]any)
	if it := items[1].(map[string]any); it["ordered"] != nil {
		t.Errorf("a nested - list must not be ordered: %v", it)
	}
	if it := items[2].(map[string]any); it["ordered"] != true {
		t.Errorf("a nested 1. list must be ordered: %v", it)
	}
	// - の箇条には ordered を書かない(既存の IR は変わらない)。
	v, errs = mdToAny(t, "# 題\n\n- a\n- b\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	if _, ok := tableBlockOf(t, v)["ordered"]; ok {
		t.Error("a - list must not write ordered")
	}
}

// 2 以上で始まる番号・- と番号の混ぜ書き・pros の番号は invalid-value。
func TestMarkdownOrderedPointsErrors(t *testing.T) {
	cases := []struct {
		name, src, path string
	}{
		{"starts at 2", "# 題\n\n2. a\n3. b\n", "/slides/0/body/0/items"},
		{"nested starts at 3", "# 題\n\n1. a\n   3. b\n", "/slides/0/body/0/items/0/items"},
		{"mixed", "# 題\n\n1. a\n- b\n", "/slides/0/body/0/items"},
	}
	for _, c := range cases {
		_, errs := mdToAny(t, c.src)
		found := false
		for _, e := range errs {
			if e.Code == codeInvalidValue && e.Path == c.path && e.Line > 0 && e.Hint != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s at %s, got %+v", c.name, codeInvalidValue, c.path, errs)
		}
	}
	src := "# 題\n\n:::::compare\n::::option A\n:::pros\n1. 早い\n:::\n::::\n::::option B\n:::pros\n- 安い\n:::\n::::\n:::::\n"
	_, errs := mdToAny(t, src)
	found := false
	for _, e := range errs {
		found = found || e.Code == codeInvalidValue && strings.Contains(e.Message, "番号")
	}
	if !found {
		t.Errorf("pros with 1.: want invalid-value, got %+v", errs)
	}
}

// IR の ordered は真偽。入れ子の項目の ordered は items が要る。
func TestIROrderedPoints(t *testing.T) {
	ok := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"points","ordered":true,"items":["a",{"content":"b","ordered":true,"items":["c"]}]}]}]}`
	d, err := ParseIR([]byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	pb := d.Slides[0].Body[0]
	if !pb.Ordered || !pb.Items[1].Ordered {
		t.Errorf("ordered not read: %+v", pb)
	}
	for _, bad := range []string{
		`{"type":"points","ordered":"yes","items":["a"]}`,
		`{"type":"points","items":[{"content":"b","ordered":true}]}`,
	} {
		src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[` + bad + `]}]}`
		if _, err := ParseIR([]byte(src)); err == nil {
			t.Errorf("accepted: %s", bad)
		}
	}
}

// PPTX:番号つきの箇条は a:buAutoNum(1段目はテンプレートの orderedStyle、入れ子は
// arabicPeriod)。番号の分の字下げは pointIndentEm。- の箇条は a:buChar のまま。
func TestOrderedPointsInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(orderedDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []string{"arabicPeriod", "romanLcPeriod", "circleNumDbPlain"} {
		tmpl, err := LookupTemplate("light")
		if err != nil {
			t.Fatal(err)
		}
		tmpl.Values.OrderedStyle = style
		var buf bytes.Buffer
		if _, err := tmpl.RenderDeck(d, &buf); err != nil {
			t.Fatal(err)
		}
		xml := orderedPart(t, buf.Bytes(), "ppt/slides/slide1.xml")
		top := regexp.MustCompile(`<a:pPr marL="(\d+)" indent="(-\d+)"[^>]*>(?:<[^>]*>)*?<a:buAutoNum type="`+style+`"/>`).FindAllStringSubmatch(xml, -1)
		if len(top) != 3 {
			t.Fatalf("%s: want 3 top-level numbered paragraphs, got %d\n%s", style, len(top), xml)
		}
		// 番号の分の幅は pointIndentEm × 本文の字の大きさ。
		want := ptToEMU(tmpl.Values.PointIndentEm * tmpl.Values.Body)
		if top[0][1] != strconv.FormatInt(want, 10) || top[0][2] != "-"+strconv.FormatInt(want, 10) {
			t.Errorf("%s: marL/indent = %s/%s, want %d", style, top[0][1], top[0][2], want)
		}
		nested := regexp.MustCompile(`<a:pPr lvl="1"[^>]*>(?:<[^>]*>)*?<a:buAutoNum type="arabicPeriod"/>`).FindAllString(xml, -1)
		if len(nested) != 2 {
			t.Errorf("%s: want 2 nested arabicPeriod paragraphs, got %d", style, len(nested))
		}
		if n := strings.Count(xml, "<a:buChar"); n != 2 {
			t.Errorf("%s: the nested - list must keep a:buChar (got %d)", style, n)
		}
		if strings.Contains(xml, "<a:t>1.</a:t>") || strings.Contains(xml, "<a:t>i.</a:t>") {
			t.Errorf("%s: the number is written as text", style)
		}
	}
}

// check の容量は箇条と同じに測る(番号の分の幅で折り、1行の項目なら高さは同じ)。
func TestOrderedPointsCapacity(t *testing.T) {
	doc := func(mark string) string {
		return "---\ntitle: 容量\ntemplate: light\ncover: false\n---\n\n# 題\n\n" +
			mark + " 一つ目\n" + mark + " 二つ目\n" + mark + " 三つ目\n"
	}
	need := func(mark string) BlockCapacity {
		res := Check("t.bx.md", []byte(doc(mark)), CheckOptions{})
		if len(res.Errors) > 0 {
			t.Fatalf("%v", res.Errors)
		}
		return res.Capacity[0].Blocks[0]
	}
	a, b := need("-"), need("1.")
	if a.Type != "points" || b.Type != "points" || a.Lines != 3 || b.Lines != 3 || a.Need != b.Need {
		t.Errorf("ordered %+v vs unordered %+v", b, a)
	}
	// 長い項目は、番号の分(pointIndentEm)を引いた幅で折る。
	tmpl, _ := LookupTemplate("light")
	size := tmpl.Values.Body
	text := strings.Repeat("あ", 200)
	mark := TextRun{Text: "1.", Mark: true, AutoNum: "arabicPeriod", Width: tmpl.Values.PointIndentEm * size}
	runs := []TextRun{mark, {Text: text}}
	w := 300.0
	got := wrapRuns(runs, size, w)
	if want := wrapRuns([]TextRun{{Text: text}}, size, w-mark.Width); len(got) != len(want) {
		t.Errorf("lines %d, want %d", len(got), len(want))
	}
	if naturalRunsWidth([]TextRun{mark}, size) != mark.Width {
		t.Error("a mark with Width must measure as Width")
	}
}
