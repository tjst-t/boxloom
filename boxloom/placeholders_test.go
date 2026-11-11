package main

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
	if _, err := tm.RenderDeck(d, out); err != nil {
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
	if !strings.Contains(read("ppt/slideLayouts/slideLayout2.xml"), `type="titleOnly"`) ||
		!strings.Contains(read("ppt/slideLayouts/slideLayout3.xml"), `type="title"`) {
		t.Error("layouts are not typed")
	}
	if !strings.Contains(read("[Content_Types].xml"), "slideLayout3.xml") {
		t.Error("content types lack the layouts")
	}
}
