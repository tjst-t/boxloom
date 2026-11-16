package boxloom

import (
	"math"
	"strings"
	"testing"
)

// 部の札(#178):dark・light の本文のスライドは、属する部(手前で最も近い level 1 の
// 区切り)の題を題の上に基調色の小さい字で出す。最初の区切りより前の本文・区切り・
// 目次・主張のスライドには出ない。札の高さは上の外周から取り、本文の領域はほぼ変わらない。

const sectionLabelDeck = `---
title: 試験
toc: true
---

# 前置き

- 点

# 第1部 背景 {type=section}

# 本文の1枚

- 点

# 節 {type=section level=2}

# 本文の2枚

- 点

# 主張の文 {type=statement}
`

// labelLines はスライドの、題(プレースホルダー)以外で accent の色の、部の題の行。
func labelLines(tm *Template, s *Spec) []string {
	var out []string
	for _, l := range trackerLines(s) {
		if l.color == tm.Theme.c("accent") && strings.HasPrefix(l.text, "第") {
			out = append(out, l.text)
		}
	}
	return out
}

func TestSectionLabelOnContentSlides(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		d, err := ReadDeck("t.bx.md", []byte(sectionLabelDeck))
		if err != nil {
			t.Fatal(err)
		}
		tm, _ := LookupTemplate(name)
		slides, err := tm.BuildDeck(d)
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]string{3: "第1部 背景", 5: "第1部 背景"} // IR の番号(0 は表紙。level 2 の節は部ではない)
		for _, s := range slides {
			got := labelLines(tm, s.Root)
			w, ok := want[s.SlideIndex]
			if ok && (len(got) != 1 || got[0] != w) {
				t.Errorf("%s slide %d: labels %q, want %q", name, s.SlideIndex, got, w)
			}
			if !ok && (s.SlideIndex < 0 || d.Slides[s.SlideIndex].Type != "section") && len(got) != 0 {
				t.Errorf("%s slide %d: unexpected labels %q", name, s.SlideIndex, got)
			}
		}
	}
}

// report は札を出さない(隅の印のパンくずを持つ)。
func TestSectionLabelNotInReport(t *testing.T) {
	d, err := ReadDeck("t.bx.md", []byte(sectionLabelDeck))
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("report")
	slides, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range slides {
		if s.SlideIndex == 2 {
			for _, l := range trackerLines(s.Root) {
				if strings.Contains(l.text, "第1部") {
					t.Errorf("report content slide has label %q", l.text)
				}
			}
		}
	}
}

// 札のある本文のスライドと札の無い本文のスライドで、本文の領域はほぼ同じ(札は上の
// 外周から取る)。容量と警告の測りは同じ配置の木。
func TestSectionLabelKeepsBodyArea(t *testing.T) {
	src := "---\ntitle: 試験\ntemplate: light\n---\n\n# 前\n\n- 点\n\n# 第1部 背景 {type=section}\n\n# 後\n\n- 点\n"
	r := Check("t.bx.md", []byte(src), CheckOptions{})
	if len(r.Capacity) != 2 {
		t.Fatalf("capacity %+v", r.Capacity)
	}
	before, after := r.Capacity[0].BodyArea, r.Capacity[1].BodyArea
	if math.Abs(before-after) > 4 {
		t.Errorf("body area %v without label, %v with label", before, after)
	}
}
