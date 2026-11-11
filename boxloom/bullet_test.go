package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

// 箇条・良い点・悪い点の頭の印は、本文の文字ではなく段落の箇条の印
// (a:buChar)として書く。描く側が和文と欧文の間に足す空きが、印と
// 数字・英字の間に入らないように(#42)。
func TestMarksAreParagraphBullets(t *testing.T) {
	src := `---
title: 印
template: light
cover: false
---

# 箇条の頭

- 2027年4月に限定公開
- 一般公開の前に監査

:::::compare
::::option A
:::pros
- 2028年に出せる
- 早く出せる
:::
:::cons
- 遅い
:::
::::
::::option B
:::pros
- 早い
:::
:::cons
- 遅い
:::
::::
:::::
`
	d, err := ReadDeck("t.bx.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	pptx, _ := buildBytes(t, d, BuildOptions{})
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	var xml string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			xml = string(b)
		}
	}
	if xml == "" {
		t.Fatal("slide1.xml not found")
	}
	tmpl, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{tmpl.Marks.Point, tmpl.Marks.Pro, tmpl.Marks.Con} {
		if strings.Contains(xml, "<a:t>"+m+"</a:t>") || strings.Contains(xml, "<a:t>"+strings.TrimSpace(m)+"</a:t>") {
			t.Errorf("mark %q is written as text", m)
		}
	}
	// 数字で始まる箇条と和文で始まる箇条の段落の頭(pPr)は同じ。
	pprOf := func(text string) string {
		re := regexp.MustCompile(`<a:p>(<a:pPr[^>]*>(?:<[^>]*>)*?</a:pPr>)<a:r><a:rPr[^>]*>(?:<a:solidFill><a:srgbClr[^>]*/></a:solidFill>)?</a:rPr><a:t>` + regexp.QuoteMeta(text))
		m := re.FindStringSubmatch(xml)
		if m == nil {
			t.Fatalf("paragraph %q not found", text)
		}
		// 項目の間(spcBef)は2つ目からだけ付くので比べない。
		return regexp.MustCompile(`<a:spcBef>.*?</a:spcBef>`).ReplaceAllString(m[1], "")
	}
	a, b := pprOf("2027年4月に限定公開"), pprOf("一般公開の前に監査")
	if a != b {
		t.Errorf("digit-first and kana-first bullets differ:\n%s\n%s", a, b)
	}
	if !strings.Contains(a, `<a:buChar char="・"/>`) || !strings.Contains(a, `indent="-`) {
		t.Errorf("point paragraph has no bullet: %s", a)
	}
	if p1, p2 := pprOf("2028年に出せる"), pprOf("早く出せる"); p1 != p2 || !strings.Contains(p1, `<a:buChar char="＋"/>`) {
		t.Errorf("pros bullets: %s / %s", p1, p2)
	}
}

// 印だけで行が終わる(印の後ろの語が長くて次の行に送られた)ときは、空の
// 段落に印を付けても見えないので、印を文字のまま書く。
func TestSplitMarkOnlyLine(t *testing.T) {
	mark, rest := splitMark([]TextRun{{Text: "・", Mark: true}})
	if mark == nil || len(rest) != 0 {
		t.Fatalf("splitMark = %v, %v", mark, rest)
	}
	if m, r := splitMark([]TextRun{{Text: "a"}}); m != nil || len(r) != 1 {
		t.Fatalf("no mark: %v %v", m, r)
	}
}
