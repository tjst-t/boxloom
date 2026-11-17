// 桁区切りの「,」で分かれた値の見つけ方と、誤りに添える直し方(#187、docs/eval.md 8.7 の F8)。
//
// YAML の流れの列([…])の中の「,」は値の区切りなので、引用符なしの
// [1,400万円, 7,980円] は 1・400万円・7・980円 の4つに分かれる。分かれた結果は
// 項目の数が合わない・数でない値がある、のような別の誤りとして出るので、読んだ
// 列に「数字で終わる値の次に、3桁の数字で始まる値」が並ぶ形があれば覚えておき、
// 同じサブシステムの data(YAML の deck では列の親)の中の誤りに、引用符で囲む
// 直し方を hint として添える。

package boxloom

import (
	"strings"
)

// thousandsSplit は、引用符なしの値の並びに桁区切りで分かれたと見られる所が
// あれば、つなぎ直した値(例 1,400万円。最初の1つ)を返す。無ければ ""。
func thousandsSplit(items []*yNode) string {
	for i := 1; i < len(items); i++ {
		if !yamlEndsWithDigit(items[i-1]) || !yamlStartsWithThreeDigits(items[i]) {
			continue
		}
		joined := items[i-1].str + "," + items[i].str
		for j := i + 1; j < len(items) && yamlEndsWithDigit(items[j-1]) && yamlStartsWithThreeDigits(items[j]); j++ {
			joined += "," + items[j].str
		}
		return joined
	}
	return ""
}

func yamlEndsWithDigit(n *yNode) bool {
	if n == nil || n.kind != 's' || n.quoted || n.str == "" {
		return false
	}
	c := n.str[len(n.str)-1]
	return c >= '0' && c <= '9'
}

// yamlStartsWithThreeDigits は、ちょうど3桁の数字で始まる(4桁目が数字でない)引用符なしの値か。
func yamlStartsWithThreeDigits(n *yNode) bool {
	if n == nil || n.kind != 's' || n.quoted || len(n.str) < 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if n.str[i] < '0' || n.str[i] > '9' {
			return false
		}
	}
	return len(n.str) == 3 || n.str[3] < '0' || n.str[3] > '9'
}

// thousandsHint は、つなぎ直した値 joined を引用符で囲む直し方。数だけなら
// 「,」を外す書き方も示す(グラフの values は引用しない数)。
func thousandsHint(joined string) string {
	h := "[…] と {…} の中の「,」は値の区切りとして読むので、桁区切りの「,」を含む " + joined +
		" が分かれています。「,」を含む値は \"" + joined + "\" のように引用符で囲んでください"
	if strings.Trim(joined, "0123456789,.-") == "" {
		h += "(グラフの values のような数なら " + strings.ReplaceAll(joined, ",", "") + " と「,」を外して書く)"
	}
	return h
}

// yamlSplitIn は n の中(入れ子を含む)で最初に見つかった、分かれた値。無ければ ""。
func yamlSplitIn(n *yNode) string {
	if n == nil {
		return ""
	}
	if n.split != "" {
		return n.split
	}
	for _, v := range n.vals {
		if s := yamlSplitIn(v); s != "" {
			return s
		}
	}
	for _, v := range n.items {
		if s := yamlSplitIn(v); s != "" {
			return s
		}
	}
	return ""
}

// splitScope は YAML の deck で分かれた列の場所 path から、hint を添える範囲を決める。
// サブシステムの data の中なら data、そうでなければ列の親。
func splitScope(path string) string {
	if k := strings.LastIndex(path, "/data/"); k >= 0 {
		return path[:k+len("/data")]
	}
	if k := strings.LastIndex(path, "/"); k >= 0 {
		return path[:k]
	}
	return ""
}

// addThousandsHints は、分かれた値のある範囲(scope → つなぎ直した値)の中の誤りに
// 引用符で囲む直し方を添える。もとの hint があれば後ろに残す。
func addThousandsHints(issues []Issue, splits map[string]string) {
	if len(splits) == 0 {
		return
	}
	for i := range issues {
		for scope, joined := range splits {
			if issues[i].Path != scope && !strings.HasPrefix(issues[i].Path, scope+"/") {
				continue
			}
			h := thousandsHint(joined)
			if strings.Contains(issues[i].Hint, h) {
				break
			}
			if issues[i].Hint != "" {
				h += "。" + issues[i].Hint
			}
			issues[i].Hint = h
			break
		}
	}
}
