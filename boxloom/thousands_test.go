package boxloom

import (
	"strings"
	"testing"
)

// 桁区切りの「,」を引用符なしで書いて値が分かれた誤りに、引用符で囲む hint が付く(#187)。
// 値は架空。
func TestThousandsSplitHint(t *testing.T) {
	deck := func(chart string) string {
		return "---\ntitle: 例\nfictional: true\n---\n\n# 売上\n\n```chart\n" + chart + "```\n"
	}
	quote := func(v string) string { return "\"" + v + "\" のように引用符で囲んで" }
	cases := []struct {
		name, chart, want string
		parseErr          bool
	}{
		// 項目が分かれて、値の数が合わない(check の chart-values)
		{"categories", "compare: item\ncategories: [1,400万円, 7,980円]\nseries:\n  - name: 売上\n    values: [1, 2]\n", quote("1,400万円"), false},
		// 値が分かれて、数でない値になる(parse の誤り)
		{"values-text", "compare: item\ncategories: [東店, 西店]\nseries:\n  - name: 売上\n    values: [1,400万円, 7,980円]\n", quote("1,400万円"), true},
		// 数が分かれて、値の数が合わない
		{"values-number", "compare: item\ncategories: [東店, 西店]\nseries:\n  - name: 売上\n    values: [1,400, 2,250]\n", "1400 と「,」を外して", false},
	}
	for _, c := range cases {
		src := []byte(deck(c.chart))
		_, perrs := MarkdownToIR("t.bx.md", src)
		if c.parseErr != (len(perrs) > 0) {
			t.Fatalf("%s: parse errors = %v", c.name, perrs)
		}
		if c.parseErr && !strings.Contains(perrs[0].Hint, c.want) {
			t.Errorf("%s: parse hint = %q, want %q", c.name, perrs[0].Hint, c.want)
		}
		res := Check("t.bx.md", src, CheckOptions{})
		if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Hint, c.want) {
			t.Errorf("%s: check errors = %+v, want hint %q", c.name, res.Errors, c.want)
		}
	}
}

// 引用符で囲めば分かれず、桁区切りでない並び([1, 2]・[2024, 2025])には hint を付けない。
func TestThousandsSplitDetect(t *testing.T) {
	for src, want := range map[string]string{
		`["1,400万円", "7,980円"]`: "",
		`[1, 2]`:                "",
		`[2024, 2025]`:          "",
		`[10, 1000]`:            "",
		`[1,234,567円, 8]`:       "1,234,567円",
		`[A, 1,400万円]`:          "1,400万円",
	} {
		n, errs := parseYAML([]string{"v: " + src}, 1)
		if len(errs) > 0 {
			t.Fatalf("%s: %v", src, errs)
		}
		if got := yamlSplitIn(n); got != want {
			t.Errorf("%s: split = %q, want %q", src, got, want)
		}
	}
	// {…} の中で分かれると「キー: 値」の誤りになり、その hint にも出る
	_, errs := parseYAML([]string{"v: {a: 1,400円}"}, 1)
	if len(errs) != 1 || !strings.Contains(errs[0].hint, `"1,400円"`) {
		t.Errorf("flow map: %+v", errs)
	}
}

// YAML の deck でも同じ hint が付く。
func TestThousandsSplitHintYAMLDeck(t *testing.T) {
	src := "boxloom: \"0.1\"\nmeta:\n  title: 例\n  fictional: true\nslides:\n  - type: content\n    title: 売上\n    body:\n      - type: subsystem\n        subsystem: chart\n        data:\n          compare: item\n          categories: [1,400万円, 7,980円]\n          series:\n            - name: 売上\n              values: [1, 2]\n"
	res := Check("t.yaml", []byte(src), CheckOptions{})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Hint, `"1,400万円"`) {
		t.Errorf("errors = %+v", res.Errors)
	}
}
