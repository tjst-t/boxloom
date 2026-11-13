// 表紙の写真(meta.coverImage)と終わりのスライド(meta.closing。Backlog #92)。
//
// 表紙の写真は飾りなので代わりの文を持たない。置き場所の決まりは画像の src と同じ
// (image.go。PNG / JPEG、文書からの相対パス、Assets から読む)。写真はスライドの地
// (p:bg の a:blipFill)として全面に敷き、縦横比が違えば長い方の両端を切る
// (引き伸ばさない。coverCropXML)。
//
// 終わりのスライドは IR に書かない。文書の頭の closing が true なら、組み立てで最後に
// 1枚足す(deckPages)。組み方は骨組みの部品 closing(標準は closingFrame。テンプレートは
// 名前で差し替える)。

package boxloom

import "fmt"

// closingSlide は頁の並び(deckPages)の中の終わりのスライドの印。
const closingSlide = -2

// closingWarnPrefix は終わりのスライドの警告の頭(check はこれを場所 /meta/closing にする)。
const closingWarnPrefix = "closing slide (meta.closing): "

// generatedSlide は組み立てで作った(IR に無い)スライドの、IR の中の場所・種類・
// 警告の頭。IR のスライドなら ok=false。
func generatedSlide(i int) (path, typ, warnPrefix string, ok bool) {
	switch i {
	case tocSlide:
		return "/meta/toc", "toc", tocWarnPrefix, true
	case closingSlide:
		return "/meta/closing", "closing", closingWarnPrefix, true
	}
	return "", "", "", false
}

// coverImage は文書の表紙の写真(書いていなければ nil)。誤りは場所 meta.coverImage を添える。
func (t *Template) coverImage(d *IRDeck) (*imageAsset, error) {
	if d.Meta == nil || d.Meta.CoverImage == "" {
		return nil, nil
	}
	a, err := t.images.get(d.Meta.CoverImage)
	if err != nil {
		return nil, fmt.Errorf("meta.coverImage: %w", err)
	}
	return a, nil
}

// withCoverImage は根の箱 root に文書の表紙の写真を敷く(写真が無ければそのまま)。
// 骨組みの部品(表紙・終わりのスライド)が根に使う。
func (ctx skeletonContext) withCoverImage(root *Spec) (*Spec, error) {
	a, err := ctx.t.coverImage(ctx.deck)
	if err != nil || root == nil {
		return root, err
	}
	root.bgImage = a
	return root, nil
}

// closingContext は終わりのスライドを組むときの文脈。
func (t *Template) closingContext(d *IRDeck, pages []int) skeletonContext {
	s := &IRSlide{Type: "closing"}
	return skeletonContext{t: t, deck: d, index: closingSlide, slide: s, st: &skeletonState{},
		pages: pages, page: pageIndex(pages, closingSlide)}
}

// closingFrame は標準の終わりのスライド(骨組みの closing):地の色だけ。表紙の写真が
// あれば全面に敷く。
func closingFrame(ctx skeletonContext) (*Spec, error) {
	return ctx.withCoverImage(Col(nil, tokBackground("bg")))
}
