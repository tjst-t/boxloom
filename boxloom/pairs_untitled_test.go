package boxloom

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// 対の側の見出しを省いて複数のブロックを書く形(Backlog #130)の試験。例は架空の
// examples/md/pairs-side-untitled.bx.md(単独の pairs・名前 {label=…} つきの pairs・
// 列の中の pairs。側は 箇条+表 と 引用×2)と、それを parse した
// examples/ir/pairs-side-untitled.json。

const untitledMD = "../examples/md/pairs-side-untitled.bx.md"
const untitledIR = "../examples/ir/pairs-side-untitled.json"

// untitledSides は例の中の見出しの無い複数のブロックの側と、中のブロックの種類。
var untitledSides = []struct {
	path  string
	types []string
}{
	{"/slides/1/body/0/pairs/0/response", []string{"points", "table"}},
	{"/slides/1/body/0/pairs/1/response", []string{"quote", "quote"}},
	{"/slides/2/body/0/pairs/0/subject", []string{"points", "table"}},
	{"/slides/2/body/0/pairs/1/subject", []string{"quote", "quote"}},
	{"/slides/3/body/0/columns/0/body/0/pairs/0/response", []string{"points", "table"}},
	{"/slides/3/body/0/columns/1/body/0/pairs/1/response", []string{"quote", "quote"}},
}

// Markdown の見出しの無い :::subject・:::response の中の複数のブロックは、IR の
// title の無い {body} になる(名前 {label=…} の有無・列の中でも)。IR の例は Markdown を
// parse したものと同じ。
func TestPairsSideUntitledParse(t *testing.T) {
	md, err := os.ReadFile(untitledMD)
	if err != nil {
		t.Fatal(err)
	}
	ir, issues := MarkdownToIR("p.bx.md", md)
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	want, err := os.ReadFile(untitledIR)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir, want) {
		t.Errorf("%s differs from parsing %s", untitledIR, untitledMD)
	}
	deck, err := ReadIR("p.json", want)
	if err != nil {
		t.Fatal(err)
	}
	pairsAt := func(b *IRBlock) *irPairs {
		p, err := readPairs(b.Raw)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	sides := []*irPairSide{
		&pairsAt(&deck.Slides[1].Body[0]).Pairs[0].Response,
		&pairsAt(&deck.Slides[1].Body[0]).Pairs[1].Response,
		&pairsAt(&deck.Slides[2].Body[0]).Pairs[0].Subject,
		&pairsAt(&deck.Slides[2].Body[0]).Pairs[1].Subject,
		&pairsAt(&deck.Slides[3].Body[0].Columns[0].Body[0]).Pairs[0].Response,
		&pairsAt(&deck.Slides[3].Body[0].Columns[1].Body[0]).Pairs[1].Response,
	}
	for i, sd := range sides {
		var got []string
		for _, b := range sd.Body {
			got = append(got, b.Type)
		}
		if !sd.Grouped || sd.Title != nil || strings.Join(got, ",") != strings.Join(untitledSides[i].types, ",") {
			t.Errorf("%s = grouped %v, title %v, body %v", untitledSides[i].path, sd.Grouped, sd.Title, got)
		}
	}
	if l := pairsAt(&deck.Slides[2].Body[0]).Pairs[1].Label; l != "B" {
		t.Errorf("label of the quote side = %q", l)
	}
}

// Markdown と IR のどちらから組んでも、どのテンプレートでも check・build を通り、
// 見出しの無い側には見出しの箱(帯)が無く、body のブロックが側の上の端から積まれる。
func TestPairsSideUntitledBuild(t *testing.T) {
	for _, in := range []string{untitledMD, untitledIR} {
		data, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		name := "p.bx.md"
		if strings.HasSuffix(in, ".json") {
			name = "p.json"
		}
		for _, tmpl := range []string{"dark", "light", "report"} {
			res := Check(name, data, CheckOptions{Template: tmpl})
			if len(res.Errors) > 0 {
				t.Fatalf("%s %s: %+v", in, tmpl, res.Errors)
			}
			for _, w := range res.Warnings {
				if w.Code != "fill-ratio" { // 例の本文が少なく見えるのは別の話
					t.Errorf("%s %s: unexpected warning %+v", in, tmpl, w)
				}
			}
			deck, err := ReadDeck(name, data)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := Build(&buf, deck, BuildOptions{Template: tmpl}); err != nil {
				t.Fatalf("%s %s: build: %v", in, tmpl, err)
			}
			dump, err := Layout(name, data, BuildOptions{Template: tmpl})
			if err != nil {
				t.Fatal(err)
			}
			seen := pairsLayoutNodes(dump)
			for _, side := range untitledSides {
				s := seen[side.path]
				if s == nil {
					t.Fatalf("%s %s: no box %s", in, tmpl, side.path)
				}
				if seen[side.path+"/title"] != nil {
					t.Errorf("%s %s: %s has a title box without a title", in, tmpl, side.path)
				}
				first := seen[side.path+"/body/0"]
				second := seen[side.path+"/body/1"]
				if first == nil || second == nil {
					t.Fatalf("%s %s: %s lacks its body boxes", in, tmpl, side.path)
				}
				// dark・light は対の名前を subject の頭に太字で置く(帯ではない)ので、名前つきの
				// subject の側だけは最初のブロックの上にその1行がある。
				labelled := strings.HasPrefix(side.path, "/slides/2/") && strings.HasSuffix(side.path, "/subject") && tmpl != "report"
				if !labelled && !nearPt(first.Box.Y, s.Box.Y) || !nearPt(first.Box.X, s.Box.X) {
					t.Errorf("%s %s: %s/body/0 at (%.1f, %.1f), side at (%.1f, %.1f): something sits above the first block",
						in, tmpl, side.path, first.Box.X, first.Box.Y, s.Box.X, s.Box.Y)
				}
				if second.Box.Y < first.Box.Y+first.Box.Height-0.01 {
					t.Errorf("%s %s: %s/body/1 is not under body/0", in, tmpl, side.path)
				}
			}
		}
	}
}
