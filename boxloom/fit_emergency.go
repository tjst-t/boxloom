// -fit の非常用の段(#150、親 #148 の fit-split)。
//
// -fit の順は:梯子(dense → fit.shrink)で詰める → 入らなければ分けられる境で続きのスライドに
// 分ける(fit_split.go)→ 分けられないもの(columns・図・画像、または分けた後も1枚に入らない
// 1つの単位)は値の表の任意の fit.emergency を1段ずつ下りる → それでも入らなければ元の
// 組み方のまま(layout-overflow・図の崩れが残り、-fit の build は誤り。-allow-overflow なら作る)。
//
//   - 分けずに組める枚は、梯子と同じ fitGroup が shrink の後に emergency を続けて試し、最初に
//     収まった段で組む(続きの表のひと続きも同じ段にそろえる)
//   - 1つの単位が梯子の最後の段で入らない枚は、emergency の段ごとに、分けずに入るか
//     (入れば上と同じく分けずに組む)、その段で分けて入るか(入れば分けた全部の枚をその段で
//     組む。fit-split の知らせも出す)を見る
//   - 「入った」は #145 の判定(はみ出しも図の崩れも無い)
//   - 非常用の段で組んだ枚には警告の [FIT-EMERGENCY WARNING](code fit-emergency)を出す
//     (知らせではなく警告:字が値の表の決まりより小さい。-strict なら誤り)。hint は分ける・
//     削ることを勧める。容量の fit は fit.emergency[K]

package boxloom

import "fmt"

// codeFitEmergency は -fit で非常用の段(fit.emergency)で組んだスライドの警告の code。
const codeFitEmergency = "fit-emergency"

// fitEmergencyHint は fit-emergency の直し方(check の hint)。
const fitEmergencyHint = "the text is smaller than the template's normal ladder allows; split the slide into two by content (each with its own title), move a figure or column to its own slide, or cut text, so it fits without -fit's emergency step"

// emergencySteps は値の表の非常用の段(fit.emergency)を梯子の段の形にしたもの。
func (t *Template) emergencySteps() []fitStepTry {
	var out []fitStepTry
	for k := range t.Fit.Emergency {
		r := fitRef{emergency: true, n: k + 1}
		out = append(out, fitStepTry{r.name(), func(s *IRSlide) { s.fit = r }, r.values(t).apply(t.Values), true})
	}
	return out
}

// fitEmergencyWarning は非常用の段で組んだスライドの警告の文(warningIssue が fit-emergency に
// する)。parts は分けた枚数(分けていなければ 0)。
func fitEmergencyWarning(c *fitChoice, parts int) string {
	what := "the body overflows"
	switch {
	case c.broken && c.over:
		what = "the body overflows and a figure breaks"
	case c.broken:
		what = "a figure breaks"
	}
	how := "it cannot be split"
	if c.byColumn {
		how = "it was split by column (#152) and this column does not fit a slide otherwise"
		if parts > 0 {
			how = fmt.Sprintf("it was split by column and this column was split into %d slides", parts)
		}
	} else if parts > 0 {
		how = fmt.Sprintf("one part does not fit a slide even when split, so it was split into %d slides", parts)
	} else if len(c.group) > 1 {
		how = fmt.Sprintf("the continued table (slides %d-%d) is not split", c.group[0], c.group[len(c.group)-1])
	}
	return fmt.Sprintf("[FIT-EMERGENCY WARNING] %s even at the template's last -fit step and %s, so -fit built it with the template's emergency step %s (body %gpt, blockGap %gpt). The text is smaller than the template intends; split the slide by content or cut text",
		what, how, c.step, c.body, c.blockGap)
}
