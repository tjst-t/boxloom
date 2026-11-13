// -fit の広げ(#128、親 #125 の自動の収め)。
//
// 既定では薄いスライドは fill-ratio の警告を出すだけで広げない(本文の下の余りをブロックの
// 間に配ることもしない)。BuildOptions.Fit・CheckOptions.Fit のときだけ、fill-ratio が
// 値の表の fillRatioMin を下回る normal の本文のスライドを値の表の梯子 fit.grow で広げる:
//
//   - fit.grow[0] → fit.grow[1] → … と1段ずつ上り、各段で組み直して詰めと同じ測り
//     (measureFitStep)ではみ出しと fill-ratio を見る
//   - はみ出す段があればその手前で止める。fill-ratio が目標に届いたらその段で止める
//   - はみ出さない最も上の段で組む。fit.grow[0] でもはみ出すなら元のまま。fill-ratio が
//     前の段より上がらない段は選ばない(図が高さを決める本文は広げても変わらない)
//   - dense のスライドは広げない(作者が詰めると決めたもの)。字と間合いの段を上げる
//     だけで、任意の倍率は使わない
//
// 続きの表のひと続き(fitGroupOf)は広げない(最後の枚が薄いのは分けた表の常。#140)。
//
// 広げたスライドには info の fit-applied(段・広げる前と後の fill-ratio・残り pt)を出し、
// 容量に fit を入れる。fill-ratio の警告は組んだ後の値で出す(renderPages)。

package boxloom

// fitGrowGroup は、元の組み方の測り値 base(group の枚ごと)で薄い組を fit.grow で広げる。
// 組の全部の枚に同じ段を当て、fill-ratio は一番低い枚で見る(いまは1枚の組だけを渡す。
// fitGroup)。
func (t *Template) fitGrowGroup(d *IRDeck, group []int, base []fitMeasure) (map[int]*BuiltSlide, map[int]*fitChoice, error) {
	target := t.Values.FillRatioMin
	ratio := minRatio(base)
	if len(t.Fit.Grow) == 0 || !(ratio < target) {
		return nil, nil, nil
	}
	for _, j := range group {
		if d.Slides[j].dense() {
			return nil, nil, nil
		}
	}
	var best []fitMeasure
	var bestRef fitRef
	for k := range t.Fit.Grow {
		r := fitRef{grow: true, n: k + 1}
		ms, err := t.measureFitGroup(d, group, func(s *IRSlide) { s.fit = r })
		if err != nil {
			return nil, nil, err
		}
		if ms == nil || anyBad(ms) {
			break // 広げてはみ出す・図が崩れるなら、その手前の段まで
		}
		mr := minRatio(ms)
		if !(mr > ratio+floatEps) {
			continue // fill-ratio が上がらない段(図が大きさを決める本文など)は選ばない
		}
		ratio, best, bestRef = mr, ms, r
		if mr >= target {
			break // 目標に届いたらそこで止める
		}
	}
	if best == nil {
		return nil, nil, nil
	}
	set := func(s *IRSlide) { s.fit = bestRef }
	dd := fitDeckCopy(d, group, set)
	v := bestRef.values(t)
	built := map[int]*BuiltSlide{}
	choices := map[int]*fitChoice{}
	for n, j := range group {
		b, err := t.buildSlide(dd, j)
		if err != nil {
			return nil, nil, err
		}
		built[j] = b
		choices[j] = &fitChoice{step: bestRef.name(), grow: true, body: v.Body, blockGap: v.BlockGap,
			remaining: best[n].remaining, fromRatio: base[n].ratio, ratio: best[n].ratio, group: group}
	}
	return built, choices, nil
}

// minRatio は一番低い fill-ratio。
func minRatio(ms []fitMeasure) float64 {
	r := ms[0].ratio
	for _, m := range ms[1:] {
		r = min(r, m.ratio)
	}
	return r
}
