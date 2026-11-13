package boxloom

import (
	"strconv"
	"strings"
)

// 番号つきの箇条(points の ordered)。番号はアプリが振る(PPTX の a:buAutoNum)。
// 番号の形はテンプレートの値の表 orderedStyle で選び、入れ子の段は
// orderedNestedStyle に決めてある。番号の分の幅は箇条の字下げ(pointIndentEm)で
// 測る(番号の字の幅は数によって変わるので、決まった幅を取る)。

// orderedStyles は選べる番号の形(a:buAutoNum の type → 見本)。
var orderedStyles = map[string]string{
	"arabicPeriod":     "1.",
	"romanLcPeriod":    "i.",
	"circleNumDbPlain": "①",
}

// orderedNestedStyle は入れ子の段の番号の形。
const orderedNestedStyle = "arabicPeriod"

// orderedLabel は形 style の n 番目(1 から)の番号の文字。PPTX の印の見本と、
// 配置の段の木(layout)で読むためのもので、測るのには使わない。
func orderedLabel(style string, n int) string {
	switch style {
	case "romanLcPeriod":
		return romanLower(n) + "."
	case "circleNumDbPlain":
		if n >= 1 && n <= 20 {
			return string(rune('①' + n - 1))
		}
		return strconv.Itoa(n)
	}
	return strconv.Itoa(n) + "."
}

// romanLower は n の小文字のローマ数字(1〜3999。外は数字のまま)。
func romanLower(n int) string {
	if n < 1 || n > 3999 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}
