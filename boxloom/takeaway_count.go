package boxloom

import "fmt"

// codeTakeawayCount は1枚の本文に takeaway が2つ以上あるときの警告の code(#165)。
// takeaway は「このスライドで強調する1文」なので、1枚に1つまで(SKILL.md)。IR・Schema は
// 数を縛らず、組み立ても2つ目を組めるので、誤りではなく警告にする(docs/ir.md)。
const codeTakeawayCount = "takeaway-count"

// takeawayCountIssues は本文に takeaway が2つ以上あるスライドごとに警告を1件出す。
// path は2つ目の takeaway(/slides/S/body/N)。takeaway は本文の一番外にだけ置ける
// (columns・pairs・topics の中には置けない)ので、本文の一番外だけを数える。
func takeawayCountIssues(file string, deck *IRDeck) []Issue {
	var out []Issue
	for si, s := range deck.Slides {
		var at []int
		for bi, b := range s.Body {
			if b.Type == "takeaway" {
				at = append(at, bi)
			}
		}
		if len(at) < 2 {
			continue
		}
		out = append(out, Issue{
			File:    file,
			Path:    fmt.Sprintf("/slides/%d/body/%d", si, at[1]),
			Message: fmt.Sprintf("slide has %d takeaways; a takeaway is the one sentence the slide emphasises, so write at most one per slide", len(at)),
			Code:    codeTakeawayCount,
			Hint:    "takeaway は1枚に1つまでです。いちばん言いたい1文だけを takeaway に残し、ほかは段落・箇条にするか、リード文(:::lead)に書いてください",
		})
	}
	return out
}
