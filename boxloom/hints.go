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
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	fillRatioHint = "本文が少なすぎます。隣の薄いスライドと1枚にまとめる・根拠(数・出典・図・表)を足す・" +
		"1つの主張だけを見せたいなら主張のスライド(type=statement)にする、のどれかで直してください。" +
		"空きを埋めるために字や図を大きくしたり、同じことを言い換えて足したりはしないでください"
	spareWidthHint = "図・画像は縦横比(または横の上限)を保つので、横が空きます。columns に入れて図の横に" +
		"説明の文を置くか(画像・chart・cycle・pillars はまとまりの中に置けます)、図を縦に大きく見せられるよう同じスライドの文を減らしてください"
	spareColumnHint = "図(円グラフ・循環の図・画像)がまとまりの幅を使い切らず、横が空きます。図のまとまりの role=main を外す" +
		"(文のまとまりを main にする)か、図のまとまりに文を足して縦を使ってください。字や図を大きくはしません"
	sparePairHint = "対の側の図(画像)が側の幅を使い切らず、横が空きます。図の余りは、同じ列のどの対も高くならない分だけもう片側へ回してあり、" +
		"残りは同じ列の別の対の文がそれ以上狭められない分です。図のある対を別の対の並びに分けるか、図の側に文を足して縦を使ってください"
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
		case "layout-overflow":
			if w.Hint == "" {
				w.Hint = joinHints(densityHint(w.Path, capOf), tableSplitHint(w.Path, capOf))
			}
		case codeFitEmergency:
			if w.Hint == "" {
				w.Hint = fitEmergencyHint
			}
		case "stat-value":
			if w.Hint == "" {
				w.Hint = statValueHint
			}
		case "spare-width":
			if w.Hint == "" {
				w.Hint = spareWidthHint
				if strings.Contains(w.Path, "/columns/") {
					w.Hint = spareColumnHint
				}
				if strings.Contains(w.Path, "/pairs/") {
					w.Hint = sparePairHint
				}
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

// densityHint は、はみ出した本文のスライドを dense にすれば収まるかの案(容量の
// denseRemaining。実際に dense の値で組み直して測ったもの)。dense のスライドなら、
// これ以上は詰めないので分ける・短くする。本文のスライドでなければ空。
func densityHint(path string, capOf map[string]SlideCapacity) string {
	var s int
	if _, err := fmt.Sscanf(path, "/slides/%d/body/", &s); err != nil {
		return ""
	}
	c, ok := capOf[fmt.Sprintf("/slides/%d", s)]
	if !ok {
		return ""
	}
	if c.Density == densityDense {
		return fmt.Sprintf("このスライドは dense です。まだ %.1fpt 足りません。スライドを分けるか、文を短くしてください", math.Max(0, -c.Remaining))
	}
	r := c.DenseRemaining
	if r == nil {
		return ""
	}
	if *r >= -overflowEps {
		return fmt.Sprintf("{density=dense} で収まります(IR は density: \"dense\"。dense の残り %.1fpt)。dense は参照用に詰めて読ませるページ(表・一覧)のためのもので、"+
			"そうでなければスライドを分けるか、文を短くしてください", math.Max(0, *r))
	}
	return fmt.Sprintf("dense にしても %.1fpt 足りません。スライドを分けるか、文を短くしてください", -*r)
}

// joinHints は空でない hint を1つの文にする。
func joinHints(hs ...string) string {
	var out []string
	for _, h := range hs {
		if h != "" {
			out = append(out, h)
		}
	}
	return strings.Join(out, "。")
}

// tableSplitHint は、はみ出した本文の表の分け方の案(容量の split)を1文にする。
// はみ出したのがその表か、表の後のブロック(注記など。#140)のときに付ける。
// 案が無ければ空。
func tableSplitHint(path string, capOf map[string]SlideCapacity) string {
	var s, k int
	if _, err := fmt.Sscanf(path, "/slides/%d/body/%d", &s, &k); err != nil {
		return ""
	}
	blocks := capOf[fmt.Sprintf("/slides/%d", s)].Blocks
	for bk, b := range blocks {
		if b.Split == nil || bk > k {
			continue
		}
		rest := ""
		if bk < len(blocks)-1 {
			rest = fmt.Sprintf("表(%s)の後のブロックは最初のスライドに残す見込みで数えています。", b.Path)
		}
		var at, rows []string
		for _, r := range b.Split.SplitBefore {
			at = append(at, fmt.Sprintf("rows[%d](本体の %d 行目)", r, r+1))
		}
		for _, n := range b.Split.Rows {
			rows = append(rows, strconv.Itoa(n))
		}
		return rest + fmt.Sprintf("この表は %d 枚に分けられます。%s の前で分けます(表ごとの本体の行の数 %s。合計の行は最後の表)。"+
			"分けた表はすぐ次のスライドに置き、続きの表には continues: true(Markdown は説明の行の {continues})を書いて見出しの行を書きません。"+
			"見出しの行と「n/m」の印・列の幅は Boxloom がそろえます。字は小さくしません",
			b.Split.Tables, strings.Join(at, "・"), strings.Join(rows, "・"))
	}
	return ""
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
