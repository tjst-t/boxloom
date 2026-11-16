package boxloom

import (
	"strings"
	"testing"
)

// 数・数の表・グラフのある本文のスライドに出典が無ければ source-missing が1件(スライドを
// 指す)。:::source があれば出ない。文書の頭の fictional: true でも出ない。文字だけの表
// (予定の表)・箇条だけのスライドには出ない(#166)。
func TestSourceMissing(t *testing.T) {
	const table = "| 館 | 貸し出し |\n| --- | --- |\n| 中央館 | 12,000件 |\n| 駅前館 | 8,500件 |\n"
	const chart = "```chart\ncompare: item\ncategories: [A, B]\nseries:\n  - name: 数\n    values: [3, 5]\n```\n"
	const stat = "::::stat\n:::value 47 {unit=%}\n地区館の職員\n:::\n::::\n"
	const plan = "| 時期 | すること |\n| --- | --- |\n| 1月 | 住民の会 |\n| 3月 | 議会に報告 |\n"
	const cols = "::::columns\n:::column 左\n- 箇条\n:::\n:::column 右\n" + chart + ":::\n::::\n"
	const source = "\n:::source\n貸し出しの記録(2025年度)\n:::\n"
	doc := func(head, body string) []byte {
		return []byte("---\ntitle: 例\n" + head + "---\n\n# 題\n\n" + body)
	}
	got := func(src []byte) []Issue {
		errs, warns, _ := checkCapacity("source.bx.md", src, "", nil, nil)
		if len(errs) > 0 {
			t.Fatalf("errors: %v", errs)
		}
		var out []Issue
		for _, w := range warns {
			if w.Code == codeSourceMissing {
				out = append(out, w)
			}
		}
		return out
	}
	for name, c := range map[string]struct {
		body, at string
	}{
		"table":   {table, "/slides/1/body/0"},
		"chart":   {chart, "/slides/1/body/0"},
		"stat":    {stat, "/slides/1/body/0"},
		"columns": {cols, "/slides/1/body/0/columns/1/body/0"},
	} {
		w := got(doc("", c.body))
		if len(w) != 1 || w[0].Path != "/slides/1" || !strings.Contains(w[0].Message, c.at) || w[0].Hint == "" {
			t.Errorf("%s without source: %+v", name, w)
		}
		if w := got(doc("", c.body+source)); len(w) != 0 {
			t.Errorf("%s with source: %+v", name, w)
		}
		if w := got(doc("fictional: true\n", c.body)); len(w) != 0 {
			t.Errorf("%s in a fictional deck: %+v", name, w)
		}
	}
	for name, body := range map[string]string{"plan table": plan, "points": "- 箇条\n- 箇条\n"} {
		if w := got(doc("", body)); len(w) != 0 {
			t.Errorf("%s: %+v", name, w)
		}
	}
}

// fictional は文書の頭から IR の meta.fictional へ(false は書かない)。
func TestFictionalFrontMatter(t *testing.T) {
	ir, errs, _ := ParseMarkdownWithMap([]byte("---\ntitle: 例\nfictional: true\n---\n\n# 題\n\n- 箇条\n"), "f.bx.md")
	if len(errs) > 0 || !strings.Contains(string(ir), `"fictional": true`) {
		t.Fatalf("errs %v, ir %s", errs, ir)
	}
	_, errs, _ = ParseMarkdownWithMap([]byte("---\ntitle: 例\nfictional: maybe\n---\n\n# 題\n\n- 箇条\n"), "f.bx.md")
	if len(errs) != 1 || errs[0].Code != codeInvalidValue {
		t.Fatalf("errs %v", errs)
	}
}
