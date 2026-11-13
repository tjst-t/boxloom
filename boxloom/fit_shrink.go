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
// テンプレートで組み(bodyTemplate)、本文の箱に印(fit)を付けてその下のトークンを
// 段の値の表で引く(token.go)。薄いスライドを広げる梯子(fit.grow、#128)も同じ印と
// 同じ測りを使う(fit_grow.go)。

package boxloom

import "fmt"

// fitStepDense は梯子の dense の段の名前(容量の fit・fit-applied の文)。
const fitStepDense = "dense"

// fitRef は -fit の段の印:ゼロなら印なし。grow なら fit.grow[n-1]、そうでなければ
// fit.shrink[n-1]。
type fitRef struct {
	grow bool
	n    int
}

func (r fitRef) set() bool { return r.n > 0 }

// values はその段の値の組。
func (r fitRef) values(t *Template) FitValues {
	if r.grow {
		return t.Fit.Grow[r.n-1]
	}
	return t.Fit.Shrink[r.n-1]
}

// name は段の名前(fit.shrink[K]・fit.grow[K])。
func (r fitRef) name() string {
	if r.grow {
		return fmt.Sprintf("fit.grow[%d]", r.n-1)
	}
	return fmt.Sprintf("fit.shrink[%d]", r.n-1)
}

// bodyTemplate は s の本文を組むテンプレート(-fit の段・dense・そのまま)。
func (t *Template) bodyTemplate(s *IRSlide) *Template {
	switch {
	case s.fit.set():
		return t.fitStepped(s.fit)
	case s.dense():
		return t.densed()
	}
	return t
}

// fitStepped は本文の値を梯子の段 r の組に替えたテンプレート(densed と同じ作り)。
func (t *Template) fitStepped(r fitRef) *Template {
	td := *t
	td.Values = r.values(t).apply(t.Values)
	if t.Subsystems != nil {
		td.Subsystems = t.Subsystems.withTextSizes(td.Values, t.Marks.Point)
	}
	return &td
}

// markFit は箱の印:この下のトークンは段 r の値の表で引く。
func markFit(r fitRef) SpecOpt { return func(s *Spec) { s.fit = r } }

// fitChoice は -fit で詰めた・広げたスライドの段。
type fitChoice struct {
	step      string  // 段の名前(dense・fit.shrink[K]・fit.grow[K])
	grow      bool    // 広げた(fit.grow)なら true
	body      float64 // その段の本文の字の大きさ
	blockGap  float64
	remaining float64 // その段で組んだときの本文の残り(pt)
	fromRatio float64 // 広げる前の fill-ratio(grow のときだけ)
	ratio     float64 // その段で組んだときの fill-ratio
}

// fitStepTry は梯子の1段(名前・写しへの当て方・値)。
type fitStepTry struct {
	name string
	set  func(*IRSlide)
	v    TemplateValues
}

// fitSlide は -fit のとき i 枚目(本文のスライド)を梯子で詰める・広げる。はみ出すなら
// 詰め(下の梯子)、収まって fill-ratio が下回る normal のスライドなら広げ(fit_grow.go)。
// どちらもしない・どの段も使えないなら nil(元のまま組む)。調整したら、その段で組んだ
// スライドを返す。
func (t *Template) fitSlide(d *IRDeck, i int) (*BuiltSlide, *fitChoice, error) {
	m, err := t.measureFitStep(d, i, func(*IRSlide) {})
	if err != nil || !m.ok {
		return nil, nil, err
	}
	if !m.over {
		return t.fitGrowSlide(d, i, m)
	}
	var steps []fitStepTry
	if !d.Slides[i].dense() && t.Dense != (DenseValues{}) {
		steps = append(steps, fitStepTry{fitStepDense, func(s *IRSlide) { s.Density = densityDense }, t.densed().Values})
	}
	for k := range t.Fit.Shrink {
		r := fitRef{n: k + 1}
		steps = append(steps, fitStepTry{r.name(), func(s *IRSlide) { s.fit = r }, r.values(t).apply(t.Values)})
	}
	for _, st := range steps {
		m, err := t.measureFitStep(d, i, st.set)
		if err != nil {
			return nil, nil, err
		}
		if m.over {
			continue
		}
		b, err := t.buildSlide(fitDeckCopy(d, i, st.set), i)
		if err != nil {
			return nil, nil, err
		}
		return b, &fitChoice{step: st.name, body: st.v.Body, blockGap: st.v.BlockGap, remaining: m.remaining, ratio: m.ratio}, nil
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

// fitMeasure は1つの段で組んだときの測り値。ok が false なら本文を測れない(段を選ばない)。
type fitMeasure struct {
	ok        bool
	over      bool    // layout-overflow になるか
	remaining float64 // 本文の残り(pt)
	ratio     float64 // fill-ratio(組んだ後の警告と同じ bodyFillRatio)
}

// measureFitStep は i 枚目を set を当てた写しで組み・並べ・描き、check と同じ
// measureBody で測る。
func (t *Template) measureFitStep(d *IRDeck, i int, set func(*IRSlide)) (fitMeasure, error) {
	b, err := t.buildSlide(fitDeckCopy(d, i, set), i)
	if err != nil {
		return fitMeasure{}, err
	}
	if b.Body == nil {
		return fitMeasure{}, nil
	}
	v := t.Values
	computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
	ratio := bodyFillRatio(b.Body) // renderPages と同じく、並べた直後・描く前に測る
	renderSlide(RenderContext{Out: NewPPTXBuilder(v.PageW, v.PageH), Roles: newCheckedRoles(t.Theme)}, b.Root)
	blocks, area, need, ok := measureBody(b.Root, b.Body)
	if !ok {
		return fitMeasure{}, nil
	}
	var kinds []string
	for _, blk := range d.Slides[i].Body {
		kinds = append(kinds, blk.Type)
	}
	return fitMeasure{ok: true, over: len(overflowWarnings(blocks, area, need, kinds)) > 0, remaining: round1(area - need), ratio: ratio}, nil
}

// fitAppliedWarning は調整したスライドの info の文(warningIssue が fit-applied にする)。
func fitAppliedWarning(c *fitChoice) string {
	if c.grow {
		return fmt.Sprintf("[FIT-APPLIED INFO] the body was underfilled (%.1f%%), so -fit built it with the template's %s step (body %gpt, blockGap %gpt), fill ratio %.1f%%, remaining %.1fpt. To keep the template's size, add content or merge it with a neighbouring thin slide",
			c.fromRatio*100, c.step, c.body, c.blockGap, c.ratio*100, c.remaining)
	}
	return fmt.Sprintf("[FIT-APPLIED INFO] the body overflowed, so -fit built it with the template's %s step (body %gpt, blockGap %gpt), remaining %.1fpt. To keep the template's size, shorten the text or split the slide",
		c.step, c.body, c.blockGap, c.remaining)
}
