// -fit でも収まらなかった本文の「あと N pt」(容量の fitShort と hint)。
//
// -fit の順(梯子 → 分ける → 非常用の段。fit_shrink.go・fit_split.go・fit_emergency.go)の
// どれでも収まらなかった本文のスライドは誤りのまま(エンジンは中身の形を変えない)。作者が
// 直せるように、最後の段で組んでも足りない高さと直し方を知らせる。

package boxloom

import "fmt"

// lastFitStep は -fit が試す最後の段(非常用の段、無ければ梯子の最後の段)。
func (t *Template) lastFitStep(s *IRSlide) fitStepTry {
	if em := t.emergencySteps(); len(em) > 0 {
		return em[len(em)-1]
	}
	steps := t.splitSteps(s)
	return steps[len(steps)-1]
}

// fitShortfall は -fit でも収まらなかった i 枚目を最後の段で組んだときに足りない高さ(pt)。
// 測れなければ ok が false。
func (t *Template) fitShortfall(d *IRDeck, i int) (float64, bool, error) {
	ms, err := t.measureFitGroup(d, []int{i}, t.lastFitStep(&d.Slides[i]).set)
	if err != nil || ms == nil {
		return 0, false, err
	}
	return round1(ms[0].need - ms[0].area), true, nil
}

// fitShortHint は -fit でも収まらなかった本文の直し方(容量の fitShort から)。
func fitShortHint(c SlideCapacity) string {
	if c.FitShort == nil {
		return ""
	}
	how := "内容でスライドを分ける(それぞれに題を付けて)か、図を別のスライドに移すか、文を削ってください"
	for _, b := range c.Blocks {
		if b.Type == "columns" {
			how = "columns の列を別のスライドに分ける(列ごとに題とリードを付けて)か、図を別のスライドに移すか、文を削ってください"
		}
	}
	if *c.FitShort <= overflowEps {
		return "-fit の最後の段(非常用の段まで)でも図が崩れます。" + how
	}
	return fmt.Sprintf("-fit の最後の段(非常用の段まで)・分けでも、あと %.1fpt 足りません。%s", *c.FitShort, how)
}

// anyFitBroken は警告の文のどれかが図の崩れ(fitBrokenCodes)か。
func anyFitBroken(warnings []string) bool {
	for _, w := range warnings {
		if fitBrokenCode(w) != "" {
			return true
		}
	}
	return false
}
