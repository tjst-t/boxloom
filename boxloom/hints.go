// 組み立ての警告に添える hint(どう直すか)。
//
// Boxloom は内容の良し悪しを判定しないが、量で測れる空き(fill-ratio・
// spare-width)は数字のまま知らせ、作者が選べる直し方を hint に書く。直し方を
// 選ぶのは作者で、Boxloom は埋めたり寄せたりしない。
//
// fill-ratio には、隣のスライドも薄いとき、2枚の本文を1枚にまとめたときの
// 見込みの割合を添える(容量の計測と同じ measureBody の数字から)。

package boxloom

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	fillRatioHint = "本文が少なすぎます。隣の薄いスライドと1枚にまとめる・根拠(数・出典・図・表)を足す・" +
		"1つの主張だけを見せたいなら主張のスライド(type=statement)にする、のどれかで直してください。" +
		"空きを埋めるために字や図を大きくしたり、同じことを言い換えて足したりはしないでください"
	spareWidthHint = "図・画像は縦横比(または横の上限)を保つので、横が空きます。columns に入れて図の横に" +
		"説明の文を置くか、図を縦に大きく見せられるよう同じスライドの文を減らしてください"
	statValueHint = "数と単位はそれぞれ途中で折らないので、カードの幅に入らないと単位が次の行に落ちるか、数がカードからはみ出します。" +
		"値を短くする(桁を丸める・大きな単位にする)・" +
		"単位や補足は label に移す・同じ stat の数を減らしてカードを広くする、のどれかで直してください。字は小さくしません"
)

var fillUnderRe = regexp.MustCompile(`^underfilled: ([0-9.]+)%`)

// addLayoutHints は警告の fill-ratio・spare-width・stat-value に hint を足す。caps は同じ
// 組み立ての容量(path が /slides/S)。
func addLayoutHints(warns []Issue, caps []SlideCapacity) {
	capOf := map[string]SlideCapacity{}
	for _, c := range caps {
		capOf[c.Path] = c
	}
	// 薄い(underfilled)スライドの番号 → 埋まりの割合(%)。
	thin := map[int]float64{}
	for _, w := range warns {
		if w.Code != "fill-ratio" {
			continue
		}
		if m := fillUnderRe.FindStringSubmatch(w.Message); m != nil {
			var s int
			if _, err := fmt.Sscanf(w.Path, "/slides/%d", &s); err == nil {
				r, _ := strconv.ParseFloat(m[1], 64)
				thin[s] = r
			}
		}
	}
	for i := range warns {
		w := &warns[i]
		switch w.Code {
		case "stat-value":
			if w.Hint == "" {
				w.Hint = statValueHint
			}
		case "spare-width":
			if w.Hint == "" {
				w.Hint = spareWidthHint
			}
		case "fill-ratio":
			var s int
			if _, err := fmt.Sscanf(w.Path, "/slides/%d", &s); err != nil {
				continue
			}
			if _, ok := thin[s]; !ok {
				continue // 入りすぎ(overflow)の側は layout-overflow が直し方を言う
			}
			w.Hint = fillRatioHint
			if m := mergeHint(s, thin, capOf); m != "" {
				w.Hint = m + "。" + fillRatioHint
			}
			if c := cardsHint(capOf[w.Path]); c != "" {
				w.Hint += "。" + c
			}
		}
	}
}

// cardsHint は、本文に横に並ぶカード(compare の選択肢・stat の数)・並列の
// まとまり(columns)があるとき、高さを決めているのは一番高いカード・まとまりで、
// ほかに文を足しても本文の高さは増えないことを1文にする。無ければ空。
func cardsHint(c SlideCapacity) string {
	var parts []string
	for _, b := range c.Blocks {
		for _, k := range b.Cards {
			if k.Tallest && len(b.Cards) > 1 {
				parts = append(parts, fmt.Sprintf("%s の高さは一番高い %s(%.1fpt)で決まります", b.Path, k.Path, k.Need))
			}
		}
		if len(b.Columns) > 1 {
			best := 0
			for i, col := range b.Columns {
				if col.Need > b.Columns[best].Need+overflowEps {
					best = i
				}
			}
			parts = append(parts, fmt.Sprintf("%s の高さは一番高い %s(%.1fpt)で決まります", b.Path, b.Columns[best].Path, b.Columns[best].Need))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "。") + "。ほかのカード・まとまりに文を足しても本文の高さは増えないので、根拠を足すならそこに足すか、別のブロックとして足してください"
}

// mergeHint は、隣(前か後ろ)のスライドも薄く、2枚の本文を1枚にまとめると
// 本文の領域に収まるとき、その見込みを1文にする。両方収まるなら、まとめた
// 割合が大きい(ちょうど埋まる)ほうを選ぶ。無ければ空。
func mergeHint(s int, thin map[int]float64, capOf map[string]SlideCapacity) string {
	a, ok := capOf[fmt.Sprintf("/slides/%d", s)]
	if !ok {
		return ""
	}
	best, bestRatio := -1, 0.0
	for _, t := range []int{s - 1, s + 1} {
		if _, ok := thin[t]; !ok {
			continue
		}
		b, ok := capOf[fmt.Sprintf("/slides/%d", t)]
		if !ok || len(a.Blocks) == 0 || len(b.Blocks) == 0 {
			continue
		}
		area := min(a.BodyArea, b.BodyArea)
		if area <= 0 {
			continue
		}
		r := (a.Need + a.BlockGap + b.Need) / area
		if r <= 1 && r > bestRatio {
			best, bestRatio = t, r
		}
	}
	if best < 0 {
		return ""
	}
	return fmt.Sprintf("隣の /slides/%d も本文が少なく(%.1f%%)、2枚の本文を1枚にまとめると本文の領域の約 %.0f%% で収まる見込みです(題は1つにします)",
		best, thin[best], bestRatio*100)
}
