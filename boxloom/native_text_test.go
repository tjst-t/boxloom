package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

func slideFromBytes(t *testing.T, pptx []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
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

var spRe = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)

// 箇条は1つのテキストボックスに項目ごとの段落で書く(PowerPoint の箇条の機能:
// 印は a:buChar、入れ子は lvl、項目の間は a:spcBef)。折り返しはアプリに任せる。
// リード文の文の切れ目だけ a:br で改行する。
func TestNativeListAndFlow(t *testing.T) {
	md := "# 箇条\n\n:::lead\n経済全体の生産性への効果は、推計で年 0.1〜1.5 ポイントと幅が大きい。分かれ目は業種ごとの導入の速さである。\n:::\n\n" +
		"- 一つ目の項目で、十分に長い文にして折り返しが起きるようにするためにさらに言葉を足して、もう少しだけ続けておくことにする\n  - 入れ子の項目\n- 二つ目の項目\n- 三つ目の項目\n"
	deck, issues, err := ParseMarkdown("t.bx.md", []byte(md))
	if err != nil || len(issues) > 0 {
		t.Fatalf("parse: %v %v", issues, err)
	}
	pptx, _ := buildBytes(t, deck, BuildOptions{Template: "light"})
	xml := slideFromBytes(t, pptx, "ppt/slides/slide1.xml")
	var list, lead string
	lists := 0
	for _, sp := range spRe.FindAllString(xml, -1) {
		if strings.Contains(sp, "<a:buChar") {
			lists++
			list = sp
		}
		if strings.Contains(sp, "分かれ目") {
			lead = sp
		}
	}
	if lists != 1 {
		t.Fatalf("the bullets must be one text box, got %d boxes with bullets", lists)
	}
	if n := strings.Count(list, "<a:p>"); n != 4 {
		t.Errorf("want 4 paragraphs (one per item), got %d", n)
	}
	for _, want := range []string{`wrap="square"`, `lvl="1"`, `<a:spcBef><a:spcPts val="800"/></a:spcBef>`} {
		if !strings.Contains(list, want) {
			t.Errorf("list box lacks %s", want)
		}
	}
	if strings.Contains(list, "<a:br") {
		t.Errorf("a wrapped bullet must not carry forced breaks (the app wraps it)")
	}
	if strings.Count(lead, "<a:p>") != 1 || strings.Count(lead, "<a:br>") != 1 || !strings.Contains(lead, `wrap="square"`) {
		t.Errorf("lead must be one paragraph broken once after the sentence end: %s", lead)
	}
	if i, j := strings.Index(lead, "大きい。"), strings.Index(lead, "<a:br>"); i < 0 || j < i {
		t.Errorf("the forced break must come after 「大きい。」")
	}
}
