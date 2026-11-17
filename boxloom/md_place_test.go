package boxloom

import (
	"slices"
	"strings"
	"testing"
)

// #188: 囲みの書き方・置き場所の誤りの hint。文は架空のもの。

func checkErrors(t *testing.T, src string) []Issue {
	t.Helper()
	res := Check("deck.bx.md", []byte(src), CheckOptions{})
	if len(res.Errors) == 0 {
		t.Fatalf("check found no errors for\n%s", src)
	}
	return res.Errors
}

func TestOneLineFenceHint(t *testing.T) {
	for _, tc := range []struct {
		name, fence, text string
	}{
		{"source", ":::source", "架空の調査 2026 年"},
		{"lead", ":::lead", "架空の町の利用者は 3 年で倍になった"},
		{"notes", ":::notes", "架空の話の補足"},
		{"note", ":::note", "数は架空のもの"},
		{"takeaway", ":::takeaway", "架空の窓口をまとめる"},
	} {
		for _, closed := range []bool{false, true} {
			src := "---\ntitle: 架空の資料\nfictional: true\n---\n\n# 架空の窓口\n\n- 架空の項目\n\n" + tc.fence + " " + tc.text + "\n"
			if closed {
				src += ":::\n"
			}
			errs := checkErrors(t, src)
			want := "1行目に「" + tc.fence + "」、次の行に「" + tc.text + "」、その次の行に「:::」"
			found := false
			for _, e := range errs {
				if strings.Contains(e.Hint, want) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s closed=%v: no hint with the correct form %q in %+v", tc.name, closed, want, errs)
			}
		}
	}
}

// 見出しの文を持つ囲み(quote)や見出しを持たない囲み(stat)には出さない。
func TestOneLineFenceHintOnlyForTextFences(t *testing.T) {
	for _, name := range []string{"quote", "stat", "columns", "compare", "value", "option"} {
		if textFence(name) {
			t.Errorf("textFence(%q) = true", name)
		}
	}
	for _, name := range []string{"source", "lead", "notes", "note", "takeaway"} {
		if !textFence(name) {
			t.Errorf("textFence(%q) = false", name)
		}
	}
}

const statInColumnMD = `---
title: 架空の資料
fictional: true
---

# 架空の窓口の数

::::columns
:::column 左
- 架空の項目
:::
::::column 右
:::stat
:::value 42 {unit=%}
架空の割合
:::
:::
::::
::::
`

func TestColumnMisplacedHint(t *testing.T) {
	errs := checkErrors(t, statInColumnMD)
	var hit *Issue
	for i, e := range errs {
		if e.Code == codeMisplaced && strings.Contains(e.Message, ":::stat") {
			hit = &errs[i]
		}
	}
	if hit == nil {
		t.Fatalf("no misplaced error for :::stat in %+v", errs)
	}
	if !strings.Contains(hit.Hint, ":::stat はスライドの本文(::::columns の外)に置きます") {
		t.Errorf("hint does not say where stat goes: %q", hit.Hint)
	}
	if !strings.Contains(hit.Hint, "::::columns のまとまり(:::column)に置けるのは "+strings.Join(columnContains, "・")) {
		t.Errorf("hint does not list what a column holds: %q", hit.Hint)
	}
}

// まとまりの中に置けないサブシステム(process)と、外側にだけ置く囲み(source)。
func TestColumnMisplacedHintOthers(t *testing.T) {
	for _, tc := range []struct{ inner, want string }{
		{"```process\nsteps:\n  - title: 架空の受付\n  - title: 架空の審査\n  - title: 架空の通知\n```", "```process はスライドの本文(::::columns の外)に置きます"},
		{":::source\n架空の調査\n:::", ":::source はスライドの一番外側(ほかの囲みの外)に置きます"},
	} {
		src := "---\ntitle: 架空の資料\nfictional: true\n---\n\n# 架空の窓口\n\n::::columns\n:::column 左\n- 架空の項目\n:::\n::::column 右\n" + tc.inner + "\n::::\n::::\n"
		errs := checkErrors(t, src)
		found := false
		for _, e := range errs {
			if strings.Contains(e.Hint, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no hint %q in %+v", tc.want, errs)
		}
	}
}

// columnContains は置ける型の定義(columnBlockTypes・columnSubsystems)から作る。
func TestColumnContainsFromDefinitions(t *testing.T) {
	n := 0
	for _, ty := range columnBlockTypes {
		if ty != "subsystem" {
			n++
			if !slices.Contains(columnContains, columnBlockWords[ty]) {
				t.Errorf("columnContains lacks %q (type %s)", columnBlockWords[ty], ty)
			}
		}
	}
	for _, s := range columnSubsystems {
		n++
		if !slices.Contains(columnContains, "```"+s) {
			t.Errorf("columnContains lacks ```%s", s)
		}
	}
	if n != len(columnContains) {
		t.Errorf("columnContains %v has %d entries, definitions have %d", columnContains, len(columnContains), n)
	}
}
