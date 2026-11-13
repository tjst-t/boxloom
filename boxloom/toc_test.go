package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// tocDeck は架空の3段の区切りと目次(toc: true)の文書。頁は
// 1 表紙・2 目次・3 章・4 本文・5 節・6 項目・7 本文・8 項目・9 本文・10 章・11 本文。
const tocDeck = `---
title: 架空の町の図書館の報告
author: 架空の研究会
toc: true
---

# 利用の今 {type=section}

# 来館の数

- 平日の夕方に来る人が増えた

# 返却の流れ {type=section level=2}

# 夜間の返却 {type=section level=3}

# 返却箱

- 閉館後の返却が半分を占める

# 案内の見直し {type=section level=3}

# 案内の板

- 入口の案内を大きくする

# 次の打ち手 {type=section}

# 予算

- 返却箱を二つ増やす
`

func parseTocDeck(t *testing.T) *IRDeck {
	t.Helper()
	d, _, err := ParseMarkdown("toc.bx.md", []byte(tocDeck))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 文書の頭の toc と区切りの level が IR に入る。
func TestTocParse(t *testing.T) {
	d := parseTocDeck(t)
	if d.Meta == nil || !d.Meta.Toc {
		t.Fatal("meta.toc is not set")
	}
	var levels []int
	for _, s := range d.Slides {
		if s.Type == "section" {
			levels = append(levels, sectionLevel(&s))
		}
	}
	if fmt.Sprint(levels) != "[1 2 3 3 1]" {
		t.Fatalf("levels = %v", levels)
	}
	b, issues, _ := ParseMarkdownWithMap([]byte(tocDeck), "toc.bx.md")
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	if !strings.Contains(string(b), `"toc": true`) || !strings.Contains(string(b), `"level": 2`) {
		t.Fatalf("IR has no toc/level:\n%s", b)
	}
}

// specTexts は木の中の文字の葉の文(塊をつないだもの)を順に集める。
func specTexts(s *Spec) []string {
	var out []string
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindText {
			if len(s.Runs) > 0 {
				out = append(out, runsText(s.Runs))
			} else {
				out = append(out, s.Text)
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(s)
	return out
}

// 目次は表紙の直後に1枚足され、level 1・2 の区切りを組み立てたあとの頁番号つきで
// 並べる(標準と report の両方)。フッターの番号も目次を数える。
func TestTocPages(t *testing.T) {
	d := parseTocDeck(t)
	want := []string{"目次", "利用の今", "3", "返却の流れ", "5", "次の打ち手", "10"}
	for _, name := range []string{"light", "dark", "report"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		built, err := tm.BuildDeck(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(built) != len(d.Slides)+1 {
			t.Fatalf("%s: %d slides, want %d", name, len(built), len(d.Slides)+1)
		}
		if built[0].SlideIndex != 0 || built[1].SlideIndex != tocSlide || built[2].SlideIndex != 1 {
			t.Fatalf("%s: slide indexes %d %d %d", name, built[0].SlideIndex, built[1].SlideIndex, built[2].SlideIndex)
		}
		got := specTexts(built[1].Root)
		if name != "report" {
			got = got[:len(got)-2] // フッター(作者と頁番号)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: toc = %q, want %q", name, got, want)
		}
		// 区切りのスライドの頁と目次の番号がそろう。
		for _, e := range tocEntries(d, deckPages(d)) {
			if built[e.Page-1].SlideIndex != e.Slide {
				t.Errorf("%s: toc page %d is slide %d, want %d", name, e.Page, built[e.Page-1].SlideIndex, e.Slide)
			}
		}
		if name != "report" {
			// 4頁目(本文)のフッターは「4 / 11」。
			texts := specTexts(built[3].Root)
			if f := texts[len(texts)-1]; f != "4 / 11" {
				t.Errorf("%s: footer = %q, want \"4 / 11\"", name, f)
			}
		}
	}
}

// PPTX にも目次が1枚入り、check は目次で場所をずらさない(誤り・警告なし)。
func TestTocBuildAndCheck(t *testing.T) {
	for _, name := range []string{"light", "report"} {
		res := Check("toc.bx.md", []byte(tocDeck), CheckOptions{Template: name})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: errors %+v", name, res.Errors)
		}
		for _, w := range res.Warnings {
			if w.Code == "layout-overflow" || w.Code == "title-wrap" {
				t.Errorf("%s: %+v", name, w)
			}
		}
		for _, c := range res.Capacity {
			if c.Path == "/slides/2" && (len(c.Blocks) != 1 || c.Blocks[0].Path != "/slides/2/body/0") {
				t.Errorf("%s: capacity %+v", name, c)
			}
		}
		var buf bytes.Buffer
		if _, err := Build(&buf, parseTocDeck(t), BuildOptions{Template: name}); err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
				n++
			}
		}
		if n != 11 {
			t.Errorf("%s: %d slides in the pptx, want 11", name, n)
		}
	}
}

// toc を書かない文書は頁を足さない。
func TestNoTocNoPage(t *testing.T) {
	d := parseTocDeck(t)
	d.Meta.Toc = false
	if p := deckPages(d); len(p) != len(d.Slides) || p[1] != 1 {
		t.Fatalf("pages = %v", p)
	}
}

// findSpec は木の中で ok を満たす最初の箱。
func findSpec(s *Spec, ok func(*Spec) bool) *Spec {
	if ok(s) {
		return s
	}
	for _, c := range s.Children {
		if r := findSpec(c, ok); r != nil {
			return r
		}
	}
	return nil
}

// report の level 1・2 は frame:全面の基調色、その上の白い枠、54pt の白い字の題。
func TestReportSectionFrame(t *testing.T) {
	d := parseTocDeck(t)
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	accent, white := tm.Theme["accent"], tm.Theme["bg"]
	for _, i := range []int{1, 3} { // 章・節
		b, err := tm.BuildSlide(d, i)
		if err != nil {
			t.Fatal(err)
		}
		if b.Root.Background != accent {
			t.Errorf("slide %d: page background %q, want accent", i, b.Root.Background)
		}
		frame := b.Main
		if frame == nil || frame.Background != white || len(frame.Children) != 1 || frame.Children[0].Background != accent {
			t.Fatalf("slide %d: no white frame around an accent inside", i)
		}
		title := placeholderLeaf(b.Root, "title")
		if title == nil || title.FontSize != 54 || runColor(title) != white || !findSpecIn(frame, title) {
			t.Errorf("slide %d: title %+v", i, title)
		}
	}
}

func findSpecIn(root, s *Spec) bool {
	return findSpec(root, func(x *Spec) bool { return x == s }) != nil
}

// report の level 3 は agenda:左の帯に親の節の題、右に兄弟の項目の一覧(24pt)、
// 今の項目に丸い矢印の印、ほかは薄い字。
func TestReportSectionAgenda(t *testing.T) {
	d := parseTocDeck(t)
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		slide          int
		current, other string
	}{{4, "夜間の返却", "案内の見直し"}, {6, "案内の見直し", "夜間の返却"}} {
		b, err := tm.BuildSlide(d, c.slide)
		if err != nil {
			t.Fatal(err)
		}
		band := findSpec(b.Root, func(s *Spec) bool { return s.Background == tm.Theme["accent"] && len(s.Children) > 0 })
		if band == nil || leafWithPlain(band, "返却の流れ") == nil {
			t.Fatalf("slide %d: no band with the parent's title", c.slide)
		}
		cur, other := leafWithPlain(b.Root, c.current), leafWithPlain(b.Root, c.other)
		if cur == nil || other == nil || cur.FontSize != 24 || other.FontSize != 24 {
			t.Fatalf("slide %d: siblings %+v %+v", c.slide, cur, other)
		}
		if cur.Placeholder != "title" || runColor(other) != tm.Theme["text_muted"] || runColor(cur) == runColor(other) {
			t.Errorf("slide %d: current %q / other %q colors", c.slide, runColor(cur), runColor(other))
		}
		// 印は今の項目の行にだけある(基調色の丸に矢印)。
		marks := 0
		var mark *Spec
		findSpec(b.Root, func(s *Spec) bool {
			if s.HasRadius && s.Background == tm.Theme["accent"] && leafWithPlain(s, "→") != nil {
				marks++
				mark = s
			}
			return false
		})
		if marks != 1 {
			t.Fatalf("slide %d: %d current marks", c.slide, marks)
		}
		row := findSpec(b.Root, func(s *Spec) bool { return s.Kind == KindRow && len(s.Children) == 2 && s.Children[0] == mark })
		if row == nil || row.Children[1] != cur {
			t.Errorf("slide %d: the mark is not on the current item's row", c.slide)
		}
		if leafWithPlain(b.Root, "利用の今") != nil || leafWithPlain(b.Root, "次の打ち手") != nil {
			t.Errorf("slide %d: items from other sections are listed", c.slide)
		}
	}
}

// 標準の区切りは level によらず今の見た目のまま。
func TestStandardSectionIgnoresLevel(t *testing.T) {
	d := parseTocDeck(t)
	tm, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	b2, err := tm.BuildSlide(d, 3) // level 2
	if err != nil {
		t.Fatal(err)
	}
	d.Slides[3].Level = 0
	b1, err := tm.BuildSlide(d, 3)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(specTexts(b1.Root)) != fmt.Sprint(specTexts(b2.Root)) || b1.Root.Background != b2.Root.Background {
		t.Fatal("standard section changes with level")
	}
}

// 親の無い level は check の誤り。Markdown の level は区切りに 1〜3 だけ。
func TestSectionLevelErrors(t *testing.T) {
	for _, c := range []struct {
		name, md, code, path string
	}{
		{"no parent", "---\ntitle: 試し\n---\n\n# 節 {type=section level=2}\n", "section-level", "/slides/1/level"},
		{"no parent after chapter", "---\ntitle: 試し\n---\n\n# 章 {type=section}\n\n# 節 {type=section level=2}\n\n# 項目 {type=section level=3}\n\n# 章2 {type=section}\n\n# 項目2 {type=section level=3}\n", "section-level", "/slides/5/level"},
		{"out of range", "---\ntitle: 試し\n---\n\n# 章 {type=section level=4}\n", "invalid-value", "/slides/1/level"},
		{"not a section", "---\ntitle: 試し\n---\n\n# 題 {level=2}\n\n- 文\n", "misplaced-content", "/slides/1"},
		{"toc not bool", "---\ntitle: 試し\ntoc: yes\n---\n\n# 題\n\n- 文\n", "invalid-value", "/meta/toc"},
	} {
		res := Check("x.bx.md", []byte(c.md), CheckOptions{})
		found := false
		for _, e := range res.Errors {
			if e.Code == c.code && e.Path == c.path {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: errors = %+v, want %s at %s", c.name, res.Errors, c.code, c.path)
		}
	}
	ok := "---\ntitle: 試し\n---\n\n# 章 {type=section}\n\n# 節 {type=section level=2}\n\n# 項目 {type=section level=3}\n\n# 節2 {type=section level=2}\n\n# 項目2 {type=section level=3}\n"
	if res := Check("x.bx.md", []byte(ok), CheckOptions{}); len(res.Errors) > 0 {
		t.Errorf("valid levels: %+v", res.Errors)
	}
	// IR の level の範囲は読み込みでも断る。
	if _, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[{"type":"section","title":"章","level":5}]}`)); err == nil {
		t.Error("level 5 was accepted")
	}
}

// 目次のスライドの警告(長すぎる目次のはみ出し)は場所 /meta/toc を指す。
func TestTocWarningPath(t *testing.T) {
	var b strings.Builder
	b.WriteString("---\ntitle: 長い目次\ntoc: true\n---\n\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "# 章 %d {type=section}\n\n", i)
	}
	res := Check("x.bx.md", []byte(b.String()), CheckOptions{Template: "light"})
	found := false
	for _, w := range res.Warnings {
		if w.Code == "layout-overflow" && w.Path == "/meta/toc" && strings.HasPrefix(w.Message, "table of contents overflows") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
}
