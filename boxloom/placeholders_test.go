package boxloom

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// 題はスライドのレイアウトのプレースホルダーとして書く:表紙は ctrTitle・subTitle
// で「Title Slide」のレイアウト、本文は title で「Title Only」のレイアウト。
// 位置(a:xfrm)は図形の側に書いたまま。
func TestTitlesArePlaceholders(t *testing.T) {
	d, err := readDeck("../examples/ir/demo-deck.json")
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	out := filepath.Join(t.TempDir(), "p.pptx")
	if _, err := renderDeckToFile(tm, d, out); err != nil {
		t.Fatal(err)
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
	cover := read("ppt/slides/slide1.xml")
	for _, want := range []string{`<p:ph type="ctrTitle"/>`, `<p:ph type="subTitle" idx="1"/>`} {
		if !strings.Contains(cover, want) {
			t.Errorf("cover lacks %s", want)
		}
	}
	if !strings.Contains(read("ppt/slides/_rels/slide1.xml.rels"), "slideLayout3.xml") {
		t.Error("cover does not use the title slide layout")
	}
	body := read("ppt/slides/slide2.xml")
	if strings.Count(body, `<p:ph type="title"/>`) != 1 {
		t.Error("content slide must have exactly one title placeholder")
	}
	i := strings.Index(body, `<p:ph type="title"/>`)
	if !strings.Contains(body[i:], "<a:xfrm>") {
		t.Error("title placeholder must keep its own position")
	}
	if !strings.Contains(read("ppt/slides/_rels/slide2.xml.rels"), "slideLayout2.xml") {
		t.Error("content slide does not use the title-only layout")
	}
	// フッター:番号は番号の置き場の中の番号の欄、左の欄はフッターの置き場。
	// 置き場はマスターとレイアウトにもあり、同じ idx で結ぶ。
	n := strings.Index(body, `<p:ph `+sldNumPhAttrs+`/>`)
	if n < 0 || !strings.Contains(body[n:], `<a:fld id="`+slideNumFieldID+`" type="slidenum">`) || !strings.Contains(body[n:], "<a:t>2</a:t></a:fld>") {
		t.Error("content slide number is not a slidenum field in the sldNum placeholder")
	}
	if !strings.Contains(body, `<p:ph `+ftrPhAttrs+`/>`) {
		t.Error("content slide lacks the footer placeholder")
	}
	for _, part := range []string{"ppt/slideMasters/slideMaster1.xml", "ppt/slideLayouts/slideLayout2.xml"} {
		x := read(part)
		if !strings.Contains(x, sldNumPhAttrs) || !strings.Contains(x, ftrPhAttrs) {
			t.Errorf("%s lacks the footer placeholders", part)
		}
	}
	if strings.Contains(cover, "slidenum") {
		t.Error("cover must not show the slide number")
	}
	if !strings.Contains(read("ppt/slideLayouts/slideLayout2.xml"), `type="titleOnly"`) ||
		!strings.Contains(read("ppt/slideLayouts/slideLayout3.xml"), `type="title"`) {
		t.Error("layouts are not typed")
	}
	if !strings.Contains(read("[Content_Types].xml"), "slideLayout3.xml") {
		t.Error("content types lack the layouts")
	}
}
