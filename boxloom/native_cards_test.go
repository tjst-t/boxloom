package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

// カード(compare の選択肢・stat の数・takeaway の地)は、地と文字を持つ1つの
// 図形で書く(地の上に別の文字の枠を重ねない)。takeaway は帯と合わせて2つ。
func TestCardsAreOneShape(t *testing.T) {
	src := `---
title: カード
template: light
cover: false
---

# カード

:::::compare
::::option 案A
:::pros
- 速い
:::
:::cons
- 高い
:::
::::
::::option 案B
:::pros
- 安い
:::
::::
:::::

::::stat
:::value 98 {unit=%}
稼働率
:::
::::

:::takeaway
結論の一文
:::
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
	tm, _ := LookupTemplate("light")
	card := hexColor(tm.Theme["card"])
	shapes := regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(xml, -1)
	var filled []string // 地の色を持つ図形
	for _, s := range shapes {
		if strings.Contains(s, `<a:srgbClr val="`+card+`"/></a:solidFill>
<a:ln>`) {
			filled = append(filled, s)
		}
	}
	// 選択肢2枚 + 数1枚 + takeaway の地1つ。
	if len(filled) != 4 {
		t.Fatalf("card shapes = %d, want 4", len(filled))
	}
	want := [][]string{{"案A", "速い", "高い"}, {"案B", "安い"}, {"98", "%", "稼働率"}, {"結論の一文"}}
	for i, w := range want {
		for _, text := range w {
			if !strings.Contains(filled[i], "<a:t>"+text) {
				t.Errorf("card %d has no %q: %s", i, text, filled[i])
			}
		}
	}
	// 文字は地の図形の中だけにある(別の文字の枠が無い)。
	for _, text := range []string{"案A", "速い", "高い", "稼働率", "結論の一文"} {
		if n := strings.Count(xml, "<a:t>"+text); n != 1 {
			t.Errorf("%q written %d times", text, n)
		}
	}
	if !strings.Contains(filled[0], `prst="roundRect"`) || !strings.Contains(filled[3], `prst="rect"`) {
		t.Errorf("card geometry: %s / %s", filled[0], filled[3])
	}
	// ○/× は段落の箇条の印で、色は support / danger。
	for _, role := range []string{"support", "danger"} {
		if !strings.Contains(filled[0], `<a:buClr><a:srgbClr val="`+hexColor(tm.Theme[role])+`"/></a:buClr>`) {
			t.Errorf("no %s bullet: %s", role, filled[0])
		}
	}
	// takeaway の帯は別の図形のまま。
	accent := hexColor(tm.Theme["accent"])
	bars := 0
	for _, s := range shapes {
		if strings.Contains(s, `<a:srgbClr val="`+accent+`"/></a:solidFill>
<a:ln>`) && !strings.Contains(s, "<a:t>") {
			bars++
		}
	}
	if bars != 1 {
		t.Errorf("accent bars = %d, want 1", bars)
	}
}
