package boxloom

import (
	"strings"
	"testing"
)

// 区切りのスライドの章の一覧(sectionTracker):区切りが2つ以上ある文書では、
// 区切りのスライドの題の下に全部の章の題が順に並び、いまの章だけが text_primary
// の太字。区切りが1つだけの文書・値の表で false のときは出ない。

const trackerDeck = `---
title: 試験
template: light
---

# 第1部 背景 {type=section}

# 本文の1枚

- 点

# 第2部 提案 {type=section}

# 第3部 進め方 {type=section}
`

type trackerLine struct {
	text  string
	color string
	bold  bool
}

// trackerLines は区切りのスライドの、題(プレースホルダー)以外の文の行。
func trackerLines(s *Spec) []trackerLine {
	var out []trackerLine
	var walk func(*Spec)
	walk = func(s *Spec) {
		if s.Placeholder == "" && len(s.Runs) > 0 {
			var b strings.Builder
			for _, r := range s.Runs {
				b.WriteString(r.Text)
			}
			out = append(out, trackerLine{b.String(), s.Runs[0].Color, s.Runs[0].Bold})
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(s)
	return out
}

func buildTrackerDeck(t *testing.T, src string, tracker bool) []*BuiltSlide {
	t.Helper()
	d, err := ReadDeck("t.bx.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	tm.Values.SectionTracker = tracker
	slides, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	return slides
}

func TestSectionTrackerListsChaptersAndMarksCurrent(t *testing.T) {
	slides := buildTrackerDeck(t, trackerDeck, true)
	tm, _ := LookupTemplate("light")
	primary, muted := tm.Theme.c("text_primary"), tm.Theme.c("text_muted")
	want := []string{"第1部 背景", "第2部 提案", "第3部 進め方"}
	for cur, si := range []int{1, 3, 4} { // 表紙が 0 枚目
		lines := trackerLines(slides[si].Root)
		if len(lines) != len(want) {
			t.Fatalf("slide %d: %d tracker lines, want %d: %+v", si, len(lines), len(want), lines)
		}
		for k, l := range lines {
			if l.text != want[k] {
				t.Errorf("slide %d line %d = %q, want %q", si, k, l.text, want[k])
			}
			isCur := k == cur
			if isCur && (l.color != primary || !l.bold) {
				t.Errorf("slide %d: current chapter %q not primary bold: %+v", si, l.text, l)
			}
			if !isCur && (l.color != muted || l.bold) {
				t.Errorf("slide %d: other chapter %q not muted: %+v", si, l.text, l)
			}
		}
	}
	// 本文のスライドには出ない
	for _, l := range trackerLines(slides[2].Root) {
		if strings.HasPrefix(l.text, "第") {
			t.Errorf("content slide has tracker line %q", l.text)
		}
	}
}

func TestSectionTrackerOff(t *testing.T) {
	for _, s := range buildTrackerDeck(t, trackerDeck, false)[1:2] {
		if n := len(trackerLines(s.Root)); n != 0 {
			t.Errorf("tracker off: %d lines", n)
		}
	}
	one := "---\ntitle: x\ntemplate: light\n---\n\n# 第1部 だけ {type=section}\n\n# 本文\n\n- 点\n"
	if n := len(trackerLines(buildTrackerDeck(t, one, true)[1].Root)); n != 0 {
		t.Errorf("single section: %d tracker lines, want 0", n)
	}
}
