// 本文のスライドの題の上の部の札(#178)。
//
// 本文のスライドは、手前で最も近い level 1 の区切り(属する部)の題を、題の上に小さい字で
// 置く。作者は何も書かない(IR は変えず、組み立てで直前の区切りから引く)。区切りの無い
// 資料・最初の区切りより前の本文のスライドには何も出さない。
//
// 出すかどうか・形はテンプレートが決める:dark・light は骨組みの contentHeader を
// 差し替えて札を出す(sectionLabelHeader)。標準の contentHeader と report は変えない
// (report は隅の印のパンくずを持つ)。札は頭の箱の中の葉(配置の段で測る)。札の高さは
// 題の上の余白から取る:札のあるスライドはページの上の外周を pagePaddingY から
// 値の表の sectionLabelPaddingTop に狭め、札と題を間を空けずに積む(札の行の高さが題との間になる)。
// 札の分は上の余白で賄われ、本文の箱(容量・layout-overflow の測り)は札の無いときと
// ほぼ同じ(dark・light は上の外周 24pt を 12pt にし、札の行 13.5pt で本文の箱が 1.5pt 狭い)。測りと描画は同じ配置の木なので
// 食い違わない。

package boxloom

import "fmt"

// sectionOf は本文のスライド i が属する部:手前で最も近い level 1 の区切りの番号
// (無ければ -1)。
func sectionOf(d *IRDeck, i int) int {
	if i < 0 || i >= len(d.Slides) {
		return -1
	}
	for j := i - 1; j >= 0; j-- {
		s := &d.Slides[j]
		if s.Type == "section" && sectionLevel(s) == 1 {
			return j
		}
	}
	return -1
}

// sectionLabelHeader は標準の contentHeader の上に、属する部の札(small・基調色)を
// 置く骨組みの部品(dark・light)。
func sectionLabelHeader(ctx skeletonContext) (*Spec, error) {
	t := ctx.t
	head := t.contentHeader(ctx.slide)
	j := sectionOf(ctx.deck, ctx.index)
	if j < 0 {
		return head, nil
	}
	label := t.text(ctx.deck.Slides[j].Title, "small", "accent", false, fromIR(fmt.Sprintf("/slides/%d/title", j)))
	return Col([]*Spec{label, head}), nil
}

// sectionLabelFrame は標準の contentFrame で組み、札があればページの上の外周を
// sectionLabelPaddingTop にする(下の外周は pagePaddingBottom のまま)。
func sectionLabelFrame(ctx skeletonContext) (*Spec, error) {
	root, err := contentFrame(ctx)
	if err != nil || root == nil || sectionOf(ctx.deck, ctx.index) < 0 {
		return root, err
	}
	root.tok.padY, root.tok.padBottom = "sectionLabelPaddingTop", "pagePaddingBottom"
	return root, nil
}

// registerSectionLabelParts は dark・light の組み方の登録(部の札)。
func registerSectionLabelParts(t *Template) error {
	if err := t.skeleton.override(skelContentFrame, skeletonFunc(sectionLabelFrame)); err != nil {
		return err
	}
	return t.skeleton.override(skelContentHeader, skeletonFunc(sectionLabelHeader))
}
