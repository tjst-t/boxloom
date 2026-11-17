package boxloom

import (
	"fmt"
	"strings"
)

// 囲みの置き場所と書き方の誤りに添える hint(#188)。置ける場所は囲みの宣言
// (coreBlocks・型の語彙の BlockDecl の Inside)と、まとまりの中に置ける型の定義
// (columnBlockTypes・columnSubsystems から作る columnContains)から作る。手で書いた
// 一覧を持たないので、置ける型を足しても hint は食い違わない。

// fenceDecl は囲み name の宣言(コアと型の語彙)。
func fenceDecl(name string) (BlockDecl, bool) {
	for _, d := range append(coreBlocks(), blockTypes.fences()...) {
		if d.Name == name {
			return d, true
		}
	}
	return BlockDecl{}, false
}

// isCoreFence は name がスライドの欄になる囲み(lead・source・notes)か。
func isCoreFence(name string) bool {
	for _, d := range coreBlocks() {
		if d.Name == name {
			return true
		}
	}
	return false
}

// fencePlace は囲み name を置ける場所の言い方。宣言の無い名前は ""。
func fencePlace(name string) string {
	d, ok := fenceDecl(name)
	switch {
	case !ok:
		return ""
	case isCoreFence(name):
		return "スライドの一番外側(ほかの囲みの外)"
	case len(d.Inside) > 0:
		return ":::" + strings.Join(d.Inside, "・:::") + " の中"
	}
	return "スライドの本文(::::columns の外)"
}

// columnPlaceHint は :::column の中に置けないもの what を置いたときの hint。fence は
// 囲みの名前、lang はコードブロックの言語名(どちらか一方。無ければ空)。
func columnPlaceHint(fence, lang string) string {
	var b strings.Builder
	switch {
	case fence != "":
		if where := fencePlace(fence); where != "" {
			fmt.Fprintf(&b, ":::%s は%sに置きます。", fence, where)
		}
	case lang != "":
		if contains(DefaultSubsystems().Kinds(), lang) {
			fmt.Fprintf(&b, "```%s はスライドの本文(::::columns の外)に置きます。", lang)
		}
	}
	fmt.Fprintf(&b, "::::columns のまとまり(:::column)に置けるのは %s だけです", strings.Join(columnContains, "・"))
	return b.String()
}

// textFence は中に文(段落・本文)を書く囲みで、見出しの文を持たないものか
// (lead・source・notes・note・takeaway など)。宣言から決める。
func textFence(name string) bool {
	d, ok := fenceDecl(name)
	if !ok || d.Label != "" {
		return false
	}
	for _, c := range d.Contains {
		if c == "段落1つ" || c == "段落" || c == "本文" {
			return true
		}
	}
	return false
}

// oneLineFenceHint は、文を持つ囲みの開きの行に文を続けた(:::source 文)ときに、
// 正しい形(開き・本文・閉じの3行)を示す hint。当てはまらなければ ""。
func oneLineFenceHint(f *mdFenceBlk) string {
	if f == nil || len(f.label.r) == 0 || !textFence(f.name) {
		return ""
	}
	open := strings.Repeat(":", f.colons) + f.name
	text := strings.TrimSpace(strings.ReplaceAll(string(f.label.r), string(brRune), " "))
	return fmt.Sprintf("囲みは開き・本文・閉じを別々の行に書きます(開きの行に文を続けられません)。1行目に「%s」、次の行に「%s」、その次の行に「%s」と書いてください",
		open, text, strings.Repeat(":", f.colons))
}
