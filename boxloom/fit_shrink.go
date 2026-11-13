// -fit の詰め(#127、親 #125 の自動の収め)。
//
// Boxloom は既定では縮めない(はみ出しは layout-overflow)。作る側が BuildOptions.Fit・
// CheckOptions.Fit(CLI の build -fit・check -fit)を明示したときだけ、はみ出す本文の
// スライドを値の表の梯子で詰める。IR は変えない(組み立ての中の写しに印を付ける)。
//
//   - 梯子は今の密度から下りる:normal のスライドは dense の組 → fit.shrink[0] → …、
//     dense のスライドは fit.shrink[0] → …。値の表の fit が無ければ dense までしか下りない
//   - 各段で組み直し、check と同じ1回の測り(measureBody)ではみ出しを見て、最初に
//     収まった段で組む。最後の段でも収まらなければ元の組み方のまま(layout-overflow)
//   - 詰めたスライドには info の [FIT-APPLIED INFO](code fit-applied。段と残り pt)を出し、
//     容量に fit(段の名前)を入れる
//
// 詰めた段の本文は dense と同じ1か所で切り替える:本文のブロックを段の値を重ねた
// テンプレートで組み(bodyTemplate)、本文の箱に印(fitShrink)を付けてその下のトークンを
// 段の値の表で引く(token.go)。

package boxloom

import "fmt"

// fitStepDense は梯子の dense の段の名前(容量の fit・fit-applied の文)。
const fitStepDense = "dense"

// bodyTemplate は s の本文を組むテンプレート(-fit で詰めた段・dense・そのまま)。
func (t *Template) bodyTemplate(s *IRSlide) *Template {
	switch {
	case s.fitShrink > 0:
		return t.fitShrunk(s.fitShrink - 1)
	case s.dense():
		return t.densed()
	}
	return t
}

// fitShrunk は本文の値を fit.shrink[k] の組に替えたテンプレート(densed と同じ作り)。
func (t *Template) fitShrunk(k int) *Template {
	td := *t
	td.Values = t.Fit.Shrink[k].apply(t.Values)
	if t.Subsystems != nil {
		td.Subsystems = t.Subsystems.withTextSizes(td.Values, t.Marks.Point)
	}
	return &td
}

// markFitShrink は箱の印:この下のトークンは fit.shrink[k-1] の値の表で引く。
func markFitShrink(k int) SpecOpt { return func(s *Spec) { s.fitShrink = k } }

// fitStepName は fit.shrink[k] の段の名前。
func fitStepName(k int) string { return fmt.Sprintf("fit.shrink[%d]", k) }

// fitChoice は -fit で詰めたスライドの段。
type fitChoice struct {
	step      string  // 段の名前(dense・fit.shrink[K])
	body      float64 // その段の本文の字の大きさ
	blockGap  float64
	remaining float64 // その段で組んだときの本文の残り(pt)
}

// fitSlide は -fit のとき i 枚目(本文のスライド)を梯子で詰める。元の組み方で収まる・
// どの段でも収まらないなら nil(元のまま組む)。詰めたら、その段で組んだスライドを返す。
func (t *Template) fitSlide(d *IRDeck, i int) (*BuiltSlide, *fitChoice, error) {
	over, _, err := t.measureFitStep(d, i, func(*IRSlide) {})
	if err != nil || !over {
		return nil, nil, err
	}
	type step struct {
		name string
		set  func(*IRSlide)
		v    TemplateValues
	}
	var steps []step
	if !d.Slides[i].dense() && t.Dense != (DenseValues{}) {
		steps = append(steps, step{fitStepDense, func(s *IRSlide) { s.Density = densityDense }, t.densed().Values})
	}
	for k := range t.Fit.Shrink {
		steps = append(steps, step{fitStepName(k), func(s *IRSlide) { s.fitShrink = k + 1 }, t.Fit.Shrink[k].apply(t.Values)})
	}
	for _, st := range steps {
		over, remaining, err := t.measureFitStep(d, i, st.set)
		if err != nil {
			return nil, nil, err
		}
		if over {
			continue
		}
		b, err := t.buildSlide(fitDeckCopy(d, i, st.set), i)
		if err != nil {
			return nil, nil, err
		}
		return b, &fitChoice{step: st.name, body: st.v.Body, blockGap: st.v.BlockGap, remaining: remaining}, nil
	}
	return nil, nil, nil
}

// fitDeckCopy は d の写しで、i 枚目のスライドの写しに set を当てたもの(IR は変えない)。
func fitDeckCopy(d *IRDeck, i int, set func(*IRSlide)) *IRDeck {
	dd := *d
	dd.Slides = append([]IRSlide(nil), d.Slides...)
	set(&dd.Slides[i])
	return &dd
}

// measureFitStep は i 枚目を set を当てた写しで組み・並べ・描き、check と同じ
// measureBody で測る。over は layout-overflow になるか、remaining は本文の残り。
func (t *Template) measureFitStep(d *IRDeck, i int, set func(*IRSlide)) (over bool, remaining float64, err error) {
	b, err := t.buildSlide(fitDeckCopy(d, i, set), i)
	if err != nil {
		return false, 0, err
	}
	if b.Body == nil {
		return false, 0, nil
	}
	v := t.Values
	computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
	renderSlide(RenderContext{Out: NewPPTXBuilder(v.PageW, v.PageH), Roles: newCheckedRoles(t.Theme)}, b.Root)
	blocks, area, need, ok := measureBody(b.Root, b.Body)
	if !ok {
		return false, 0, nil
	}
	var kinds []string
	for _, blk := range d.Slides[i].Body {
		kinds = append(kinds, blk.Type)
	}
	return len(overflowWarnings(blocks, area, need, kinds)) > 0, round1(area - need), nil
}

// fitAppliedWarning は詰めたスライドの info の文(warningIssue が fit-applied にする)。
func fitAppliedWarning(c *fitChoice) string {
	return fmt.Sprintf("[FIT-APPLIED INFO] the body overflowed, so -fit built it with the template's %s step (body %gpt, blockGap %gpt), remaining %.1fpt. To keep the template's size, shorten the text or split the slide",
		c.step, c.body, c.blockGap, c.remaining)
}
