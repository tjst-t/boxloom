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
// 広げたスライドには info の fit-applied(段・広げる前と後の fill-ratio・残り pt)を出し、
// 容量に fit を入れる。fill-ratio の警告は組んだ後の値で出す(renderPages)。

package boxloom

// fitGrowSlide は、元の組み方の測り値 m で薄い i 枚目を fit.grow で広げる。
func (t *Template) fitGrowSlide(d *IRDeck, i int, m fitMeasure) (*BuiltSlide, *fitChoice, error) {
	target := t.Values.FillRatioMin
	if d.Slides[i].dense() || len(t.Fit.Grow) == 0 || !(m.ratio < target) {
		return nil, nil, nil
	}
	var best *fitChoice
	ratio := m.ratio
	var bestSet func(*IRSlide)
	for k := range t.Fit.Grow {
		r := fitRef{grow: true, n: k + 1}
		set := func(s *IRSlide) { s.fit = r }
		mk, err := t.measureFitStep(d, i, set)
		if err != nil {
			return nil, nil, err
		}
		if !mk.ok || mk.over {
			break // 広げてはみ出すなら、その手前の段まで
		}
		if !(mk.ratio > ratio+floatEps) {
			continue // fill-ratio が上がらない段(図が大きさを決める本文など)は選ばない
		}
		ratio = mk.ratio
		v := r.values(t)
		best = &fitChoice{step: r.name(), grow: true, body: v.Body, blockGap: v.BlockGap,
			remaining: mk.remaining, fromRatio: m.ratio, ratio: mk.ratio}
		bestSet = set
		if mk.ratio >= target {
			break // 目標に届いたらそこで止める
		}
	}
	if best == nil {
		return nil, nil, nil
	}
	b, err := t.buildSlide(fitDeckCopy(d, i, bestSet), i)
	if err != nil {
		return nil, nil, err
	}
	return b, best, nil
}
