package boxloom

// 題とリード文:本文のスライドの題は短い見出しで1行(折り返すと title-wrap)、
// 言いたいことの文はリード文(lead)として題の直下に置く。あわせて、本文の図が
// 大きいときに題・リード文が縮められないこと(本文の図のゆとりから先に返す)と、
// ページの左右と上下で違う余白(pagePadding・pagePaddingY)を見る。

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const leadDoc = `---
title: 例
---

# 雇用への影響

:::lead
世界の雇用の約 4 割が AI の影響を受け、先進国では 6 割に達する。
:::

- 一つ目の箇条
- 二つ目の箇条
`

func TestMarkdownLead(t *testing.T) {
	v, errs := mdToAny(t, leadDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	s := getPath(v, "slides").([]any)[1].(map[string]any)
	if s["lead"] != "世界の雇用の約 4 割が AI の影響を受け、先進国では 6 割に達する。" {
		t.Fatalf("lead: %v", s["lead"])
	}
	if body := s["body"].([]any); len(body) != 1 {
		t.Fatalf("the lead must not be a body block: %v", body)
	}
	b, _ := json.Marshal(v)
	d, err := ParseIR(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.Slides[1].Lead.Plain() == "" {
		t.Fatal("Go IR lost the lead")
	}
}

func TestMarkdownLeadErrors(t *testing.T) {
	head := "---\ntitle: 例\n---\n\n"
	cases := []struct{ name, src string }{
		{"section", head + "# 第1部 {type=section}\n\n:::lead\n文\n:::\n"},
		{"statement", head + "# 言いたいこと {type=statement}\n\n:::lead\n文\n:::\n"},
		{"two", head + "# 題\n\n:::lead\nA\n:::\n\n:::lead\nB\n:::\n\n本文\n"},
		{"inside", head + "# 題\n\n::::columns\n:::column\n本文\n:::\n:::column\n本文\n\n:::lead\nA\n:::\n:::\n::::\n"},
		{"before first slide", head + ":::lead\nA\n:::\n\n# 題\n\n本文\n"},
		{"label", head + "# 題\n\n:::lead 見出し\nA\n:::\n\n本文\n"},
	}
	for _, c := range cases {
		_, errs := mdToAny(t, c.src)
		found := false
		for _, e := range errs {
			if e.Code == codeMisplaced {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s, got %+v", c.name, codeMisplaced, errs)
		}
	}
}

// 長い題は title-wrap の警告(題を指し、.md ならその行)。短い題とリード文なら出ない。
func TestTitleWrapWarning(t *testing.T) {
	long := "---\ntitle: 例\n---\n\n# " + strings.Repeat("とても長い題の文", 12) + "\n\n- 箇条\n"
	_, warns, _ := checkCapacity("long.bx.md", []byte(long), "light", nil, nil)
	found := false
	for _, w := range warns {
		if w.Code == "title-wrap" {
			found = true
			if w.Path != "/slides/1/title" || w.Line != 5 {
				t.Errorf("title-wrap: %+v", w)
			}
		}
	}
	if !found {
		t.Fatalf("no title-wrap warning: %+v", warns)
	}
	_, warns, _ = checkCapacity("lead.bx.md", []byte(leadDoc), "light", nil, nil)
	for _, w := range warns {
		if w.Code == "title-wrap" {
			t.Errorf("short title warned: %+v", w)
		}
	}
}

// リード文は本文の外の枠なので、その高さと gap の分だけ本文の領域が狭くなる。
func TestLeadReducesBodyArea(t *testing.T) {
	noLead := strings.Replace(leadDoc, ":::lead\n世界の雇用の約 4 割が AI の影響を受け、先進国では 6 割に達する。\n:::\n\n", "", 1)
	_, _, a := checkCapacity("a.bx.md", []byte(noLead), "light", nil, nil)
	_, _, b := checkCapacity("b.bx.md", []byte(leadDoc), "light", nil, nil)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("capacity: %+v %+v", a, b)
	}
	tm, _ := LookupTemplate("light")
	v := tm.Values
	want := v.LeadText*lineHeightMult() + v.HeaderGap
	if d := a[0].BodyArea - b[0].BodyArea; d < want-0.5 || d > want+0.5 {
		t.Fatalf("body area shrank by %.1fpt, want %.1fpt", d, want)
	}
}

// 本文の図が自然な大きさではページに入らないとき、題とリード文は縮めず、図の
// ゆとり(自然〜最小)から返させる(題が図に重ならない)。
func TestTitleNotSqueezedByFigure(t *testing.T) {
	src := "---\ntitle: 例\n---\n\n# 電力\n\n:::lead\nデータセンターの電力は 2030 年に倍増する見込みである。\n:::\n\n" +
		"```chart\ncompare: component\ncategories: [米国, 中国, 欧州, その他]\nseries:\n  - name: 割合\n    values: [45, 25, 15, 15]\n```\n\n" +
		"- 一つ目の箇条\n- 二つ目の箇条\n- 三つ目の箇条\n"
	ir, errs := MarkdownToIR("t.bx.md", []byte(src))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	b := built[1]
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	head := b.Root.Children[0]
	if need := heightForWidth(head, head.Box.Width); head.Box.Height < need-0.01 {
		t.Fatalf("title and lead squeezed: %.1fpt < %.1fpt", head.Box.Height, need)
	}
	// 上下と左右で違う余白。
	if head.Box.X != tm.Values.PagePadding || head.Box.Y != tm.Values.PagePaddingY {
		t.Fatalf("page padding: head at (%.1f, %.1f)", head.Box.X, head.Box.Y)
	}
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
}

// 例(.md と、それを parse した IR)は、1枚目の題に title-wrap が1件、2枚目には出ない。
func TestTitleWrapExample(t *testing.T) {
	for _, f := range []string{"../examples/md/warnings/title-wrap.bx.md", "../examples/ir/check-warnings/title-wrap.json"} {
		errs, warns := CheckFile(f, "")
		if len(errs) > 0 {
			t.Fatalf("%s: %+v", f, errs)
		}
		var got []string
		for _, w := range warns {
			if w.Code == "title-wrap" {
				got = append(got, w.Path)
			}
		}
		if len(got) != 1 || got[0] != "/slides/1/title" {
			t.Errorf("%s: title-wrap at %v", f, got)
		}
	}
}

// リード文が値の表の上限(report は 2 行)を超えたときだけ lead-wrap の警告
// (リード文を指す)。2 行に収まるリードと、上限の無い light では出ない。
func TestLeadWrapWarning(t *testing.T) {
	doc := func(lead string) []byte {
		return []byte("---\ntitle: 例\n---\n\n# 題\n\n:::lead\n" + lead + "\n:::\n\n- 箇条\n")
	}
	count := func(tmpl string, src []byte) []Issue {
		_, warns, _ := checkCapacity("lead.bx.md", src, tmpl, nil, nil)
		var got []Issue
		for _, w := range warns {
			if w.Code == "lead-wrap" {
				got = append(got, w)
			}
		}
		return got
	}
	tm, _ := LookupTemplate("report")
	if tm.Values.LeadMaxLines != 2 {
		t.Fatalf("report leadMaxLines = %d, want 2", tm.Values.LeadMaxLines)
	}
	// 行の数を測ってから文を選ぶ(字の幅の変更で試験が崩れないように)。
	lines := func(s string) int {
		w := tm.Values.PageW - 2*tm.Values.PagePadding
		return len(wrapText(s, tm.Values.LeadText, w, false))
	}
	two, three := "", ""
	for k := 1; k < 60 && (two == "" || three == ""); k++ {
		s := strings.Repeat("架空の会社は来年度に新しい事業を始める。", k)
		switch lines(s) {
		case 2:
			if two == "" {
				two = s
			}
		case 3:
			if three == "" {
				three = s
			}
		}
	}
	if two == "" || three == "" {
		t.Fatal("could not make 2- and 3-line leads")
	}
	got := count("report", doc(three))
	if len(got) != 1 || got[0].Path != "/slides/1/lead" || !strings.Contains(got[0].Message, "3 lines") || !strings.Contains(got[0].Message, "limit of 2") {
		t.Fatalf("3-line lead: %+v", got)
	}
	if got := count("report", doc(two)); len(got) != 0 {
		t.Fatalf("2-line lead warned: %+v", got)
	}
	if got := count("light", doc(three)); len(got) != 0 {
		t.Fatalf("light has no limit but warned: %+v", got)
	}
}
