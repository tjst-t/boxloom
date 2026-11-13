package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

// splitDeckMD は目次つきの文書:表紙・区切り・はみ出す本文(body)・区切り。
func splitDeckMD(template, body string) string {
	return fmt.Sprintf(`---
title: 分ける試し
author: Boxloom 試験
template: %s
toc: true
---

# 第1部 長い本文 {type=section}

# 長い本文 {marker=A}

:::lead
長い本文を -fit が続きのスライドに分ける
:::

%s

:::source
試験のための架空の数
:::

:::notes
最初の枚だけに付くノート
:::

# 第2部 後ろ {type=section}
`, template, body)
}

const splitSentence = "利用者への聞き取りで分かったことを、次の打ち手と合わせて一つずつ書き出していく"

func splitPointsBody(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%d. %s(%d)\n", i+1, splitSentence, i+1)
		if i%3 == 0 {
			fmt.Fprintf(&b, "   - 入れ子の項目 %d は親と一緒に動く\n", i+1)
		}
	}
	return b.String()
}

func splitPairsBody(n int) string {
	var b strings.Builder
	b.WriteString(":::::pairs 分かったこと | 打ち手\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, ":::subject\n%s(%d)\n:::\n:::response\n%s\n:::\n", splitSentence, i+1, strings.Repeat(splitSentence+"。", 6))
	}
	b.WriteString(":::::\n")
	return b.String()
}

func splitTopicsBody(n int) string {
	var b strings.Builder
	b.WriteString(":::::topics\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, ":::topic 観点 %d\n%s\n:::\n", i+1, strings.Repeat(splitSentence+"。", 7))
	}
	b.WriteString(":::::\n")
	return b.String()
}

func splitTableBody(n int) string {
	var b strings.Builder
	b.WriteString("| 仕組み | 部署 | 人 | 気をつけること |\n| --- | --- | --- | --- |\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "| 仕組み %d | 営業 | %d | %s |\n", i+1, 100+i, splitSentence)
	}
	return b.String()
}

var splitBodies = []struct {
	name string
	body string
	key  string // 切れ端の欄(始まりの場所に出る)
}{
	{"table", splitTableBody(40), "rows"},
	{"points", splitPointsBody(30), "items"},
	{"pairs", splitPairsBody(8), "pairs"},
	{"topics", splitTopicsBody(6), "topics"},
}

// buildSlidesXML は Build(-fit)した PPTX の slideN.xml(N は 1 から)と結果。
func buildSlidesXML(t *testing.T, md string, opt BuildOptions) (map[int]string, *BuildResult) {
	t.Helper()
	d, _, err := ParseMarkdown("split.bx.md", []byte(md))
	if err != nil || d == nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	res, err := Build(&buf, d, opt)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]string{}
	re := regexp.MustCompile(`^ppt/slides/slide([0-9]+)\.xml$`)
	for _, f := range zr.File {
		if m := re.FindStringSubmatch(f.Name); m != nil {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			var n int
			fmt.Sscan(m[1], &n)
			out[n] = string(b)
		}
	}
	return out, res
}

// 長い表・対の多い pairs・長い箇条・話題の多い topics は、梯子の最後の段でも入らなければ
// -fit で続きのスライドに分かれる:どの枚もはみ出さず、各枚の題の隣に「n/m」、info の
// fit-split と容量の split が出て、目次と頁番号は分けた後の枚で数える。
func TestFitSplitSlides(t *testing.T) {
	for _, tmpl := range []string{"light", "report"} {
		for _, c := range splitBodies {
			if c.name == "pairs" && tmpl != "light" {
				continue // pairs の組み方は対が多いと重い(1つのテンプレートで見る)
			}
			name := tmpl + "/" + c.name
			md := splitDeckMD(tmpl, c.body)
			plain := Check("split.bx.md", []byte(md), CheckOptions{})
			if len(fitIssues(plain.Warnings, codeLayoutOverflow)) == 0 {
				t.Fatalf("%s: without -fit the slide should overflow", name)
			}
			res := Check("split.bx.md", []byte(md), CheckOptions{Fit: true, Strict: true})
			if !res.OK || len(res.Warnings) != 0 {
				t.Fatalf("%s: -fit check = ok %v, warnings %+v", name, res.OK, res.Warnings)
			}
			splits := fitIssues(res.Info, codeFitSplit)
			if len(splits) != 1 || splits[0].Path != "/slides/2" || splits[0].Line == 0 ||
				!strings.Contains(splits[0].Message, "/slides/2/body/0/"+c.key+"/") {
				t.Fatalf("%s: info = %+v", name, res.Info)
			}
			var parts []SlideCapacity
			for _, sc := range res.Capacity {
				if sc.Remaining < -overflowEps {
					t.Errorf("%s: %s overflows: %+v", name, sc.Path, sc)
				}
				if sc.Split != nil {
					parts = append(parts, sc)
				}
			}
			m := len(parts)
			if m < 2 {
				t.Fatalf("%s: capacity has %d split parts: %+v", name, m, res.Capacity)
			}
			for k, sc := range parts {
				if sc.Path != "/slides/2" || sc.Split.Part != k+1 || sc.Split.Slides != m || len(sc.Split.Starts) != m-1 ||
					sc.Fit != parts[0].Fit || sc.Blocks[0].Path != "/slides/2/body/0" {
					t.Fatalf("%s: part %d capacity = %+v %+v", name, k+1, sc, sc.Split)
				}
			}
			if !strings.Contains(splits[0].Message, fmt.Sprintf("into %d slides", m)) {
				t.Fatalf("%s: info %q does not say %d slides", name, splits[0].Message, m)
			}

			slides, br := buildSlidesXML(t, md, BuildOptions{Fit: true})
			// 表紙・目次・区切り・本文 m 枚・区切り。
			total := 3 + m + 1
			if br.Slides != total || len(slides) != total || hasOverflowWarning(br.Warnings) {
				t.Fatalf("%s: build slides %d (xml %d), want %d; warnings %q", name, br.Slides, len(slides), total, br.Warnings)
			}
			if len(br.Info) != 1 || !strings.HasPrefix(br.Info[0], "slide 3: [FIT-SPLIT INFO]") {
				t.Fatalf("%s: build info %q", name, br.Info)
			}
			for k := 1; k <= m; k++ {
				x := slides[3+k]
				if !strings.Contains(x, fmt.Sprintf(" %d/%d</a:t>", k, m)) {
					t.Errorf("%s: slide %d has no %d/%d mark", name, 3+k, k, m)
				}
				if !strings.Contains(x, "試験のための架空の数") {
					t.Errorf("%s: slide %d has no source", name, 3+k)
				}
				if !strings.Contains(x, "長い本文を -fit が続きのスライドに分ける") {
					t.Errorf("%s: slide %d has no lead", name, 3+k)
				}
			}
			// 目次:後ろの区切りは分けた後の頁(表紙・目次・区切り・本文 m 枚の次)。
			later := 3 + m + 1
			if !strings.Contains(slides[2], fmt.Sprintf("<a:t>%d</a:t>", later)) {
				t.Errorf("%s: toc does not list page %d", name, later)
			}
			if tmpl == "light" {
				// フッターの頁番号は分けた後の全部の枚で数える。
				if !strings.Contains(slides[3+m], fmt.Sprintf("<a:t>%d</a:t>", 3+m)) || !strings.Contains(slides[3+m], fmt.Sprintf("<a:t> / %d</a:t>", total)) {
					t.Errorf("%s: footer of the last piece has no %d / %d", name, 3+m, total)
				}
			}
		}
	}
}

// 分けた枚:ノートは最初の枚だけ、隅の印・出典・リードは各枚、2枚目からの題は
// marks.continuedTitle、番号つきの箇条は前の枚の続きの番号から、表は続きの表になる。
func TestFitSplitPieces(t *testing.T) {
	for _, c := range splitBodies {
		if c.name == "pairs" {
			continue // 対の多い pairs は重い(TestFitSplitSlides で見る)
		}
		d, _, err := ParseMarkdown("split.bx.md", []byte(splitDeckMD("light", c.body)))
		if err != nil {
			t.Fatal(err)
		}
		tm := mustTemplate("light")
		tm.Marks.ContinuedTitle = "{title}(続き)"
		if _, err := tm.BuildDeck(d); err != nil {
			t.Fatal(err)
		}
		dx, _, err := tm.fitSplitDeck(d)
		if err != nil || dx == d {
			t.Fatalf("%s: fitSplitDeck = %v (split %v)", c.name, err, dx != d)
		}
		var pieces []IRSlide
		for _, s := range dx.Slides {
			if s.split != nil {
				pieces = append(pieces, s)
			}
		}
		if len(dx.Slides) != len(d.Slides)+len(pieces)-1 {
			t.Fatalf("%s: %d slides from %d with %d pieces", c.name, len(dx.Slides), len(d.Slides), len(pieces))
		}
		items := 0
		for k, s := range pieces {
			if s.split.from != 2 || s.split.n != k+1 || s.split.m != len(pieces) {
				t.Fatalf("%s: piece %d split = %+v", c.name, k, s.split)
			}
			if (s.Notes != "") != (k == 0) || s.Marker != "A" || s.Source == nil || s.Lead == nil {
				t.Fatalf("%s: piece %d notes %q marker %q source %v lead %v", c.name, k, s.Notes, s.Marker, s.Source, s.Lead)
			}
			wantTitle := "長い本文"
			if k > 0 {
				wantTitle += "(続き)"
			}
			if s.Title.Plain() != wantTitle {
				t.Fatalf("%s: piece %d title %q", c.name, k, s.Title.Plain())
			}
			b := s.Body[0]
			switch c.name {
			case "points":
				if b.itemStart != items {
					t.Fatalf("points: piece %d starts at %d, want %d", k, b.itemStart, items)
				}
				items += len(b.Items)
			case "table":
				if b.Continues != (k > 0) || (b.Header == nil) != (k > 0) {
					t.Fatalf("table: piece %d continues %v header %v", k, b.Continues, b.Header != nil)
				}
			}
		}
		if c.name == "points" && items != 30 {
			t.Fatalf("points: pieces hold %d items, want 30", items)
		}
	}
	// 番号つきの箇条の続きの枚は、PPTX の番号を続きから振る(startAt)。
	slides, _ := buildSlidesXML(t, splitDeckMD("light", splitPointsBody(30)), BuildOptions{Fit: true})
	if strings.Contains(slides[4], "startAt=") || !regexp.MustCompile(`<a:buAutoNum type="arabicPeriod" startAt="[0-9]+"/>`).MatchString(slides[5]) {
		t.Fatalf("ordered points: the second piece does not continue the numbering")
	}
}

// 表の行の群の途中では切らない。分けられる位置は群の境だけ。
func TestFitSplitTableGroupCuts(t *testing.T) {
	b := &IRBlock{Type: "table", Rows: make([][]IRTableCell, 6),
		Groups:       []IRTableGroup{{Rows: 3}, {Rows: 1}, {Rows: 2}},
		GroupColumns: []IRTableGroupColumn{{Column: 1, Groups: []IRTableGroup{{Rows: 1}, {Rows: 2}, {Rows: 1}, {Rows: 2}}}}}
	key, n, cuts, err := splitKeyOf(b)
	if err != nil || key != "rows" || n != 6 || fmt.Sprint(cuts) != "[3 4]" {
		t.Fatalf("splitKeyOf = %q %d %v %v", key, n, cuts, err)
	}
	p, err := pieceBlock(*b, "rows", 3, 6, 6)
	if err != nil || !p.Continues || len(p.Rows) != 3 || fmt.Sprint(p.Groups) != fmt.Sprint([]IRTableGroup{{Rows: 1}, {Rows: 2}}) ||
		fmt.Sprint(p.GroupColumns[0].Groups) != fmt.Sprint([]IRTableGroup{{Rows: 1}, {Rows: 2}}) {
		t.Fatalf("piece = %+v %v", p, err)
	}
}

// 分けられない(1つの単位でも最後の段で1枚に入らない)スライドは分けず、はみ出しが残る。
func TestFitSplitUnsplittable(t *testing.T) {
	long := strings.Repeat(splitSentence+"。", 60)
	md := splitDeckMD("light", long)
	res := Check("split.bx.md", []byte(md), CheckOptions{Fit: true})
	if len(fitIssues(res.Info, codeFitSplit)) != 0 || len(fitIssues(res.Warnings, codeLayoutOverflow)) == 0 {
		t.Fatalf("an unsplittable paragraph: info %+v warnings %+v", res.Info, res.Warnings)
	}
}

// 分けた枚の警告と容量の場所は IR の番号(本文のブロックと、切れ端の中の番号)。
func TestFitSplitRemap(t *testing.T) {
	sp := &slideSplit{from: 4, blocks: []splitBlockRef{{index: 2, key: "pairs", from: 5}, {index: 3}}}
	if got := sp.remapWarning("[LAYOUT-OVERFLOW WARNING] body[0]: pairs[1]: pair 1 (pairs) overflows"); got != "[LAYOUT-OVERFLOW WARNING] body[2]: pairs[6]: pair 1 (pairs) overflows" {
		t.Fatalf("remapWarning = %q", got)
	}
	if got := sp.remapWarning("[SUBSYSTEM WARNING] body[1]: chart: x"); got != "[SUBSYSTEM WARNING] body[3]: chart: x" {
		t.Fatalf("remapWarning = %q", got)
	}
	if got := sp.remapPath("/slides/4/body/0/pairs/2"); got != "/slides/4/body/2/pairs/7" {
		t.Fatalf("remapPath = %q", got)
	}
	if got := sp.remapPath("/slides/4/body/1/columns/0/body/1"); got != "/slides/4/body/3/columns/0/body/1" {
		t.Fatalf("remapPath = %q", got)
	}
}
