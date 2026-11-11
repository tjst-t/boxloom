// 1行に収めたい文字(stat の数と単位)の幅の検査。
//
// stat の数と単位は途中で折ると読み違える(「2,859 億ド / ル」「12,345, / 678」)
// ので、それぞれ折り返さない塊(TextRun.Keep)として書く。数と単位が1行に入らな
// ければ単位だけ次の行へ送り、数そのものが入らなければ1行のままカードから
// はみ出す。どちらもその葉ごとに警告 stat-value を1件出す。Boxloom は字を縮め
// たり値を書き換えたりはしない(直すのは作者:値を短く・単位を説明へ・数を減らす)。

package boxloom

import "fmt"

// keepLineEps は丸めの誤差(pt)。
const keepLineEps = 0.01

// keepLineWarnings は本文のブロックの中の keepLine の葉のうち、1行の幅が葉の
// 幅を越えるものごとに警告の文を作る(stat は並列のまとまりの中には置けない
// ので、本文のブロックだけを見る)。computeLayout の後に呼ぶ。
func keepLineWarnings(body *Spec) []string {
	if body == nil {
		return nil
	}
	var out []string
	for i, b := range body.Children {
		for _, w := range keepLineLeaves(b) {
			out = append(out, fmt.Sprintf("[STAT-VALUE WARNING] body[%d]: %s", i, w))
		}
	}
	return out
}

// keepLineLeaves は s の下の keepLine の葉のうち、入りきらないものの文。
func keepLineLeaves(s *Spec) []string {
	var out []string
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindText && s.keepLine != "" {
			need := textNaturalWidth(s)
			if need > s.Box.Width+keepLineEps {
				what := "the unit goes to the next line"
				if len(s.Runs) > 0 && naturalRunsWidth(s.Runs[:1], s.FontSize) > s.Box.Width+keepLineEps {
					what = "the value itself runs past the card"
				}
				out = append(out, fmt.Sprintf("%s (%q): value and unit do not fit on one line (needs %.1fpt, has %.1fpt); %s",
					s.keepLine, runsText(s.Runs), need, s.Box.Width, what))
			}
			return
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(s)
	return out
}
