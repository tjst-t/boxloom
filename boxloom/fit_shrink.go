// -fit の詰め(#127、親 #125 の自動の収め)。
//
// Boxloom は既定では縮めない(はみ出しは layout-overflow)。作る側が BuildOptions.Fit・
// CheckOptions.Fit(CLI の build -fit・check -fit)を明示したときだけ、はみ出す本文の
// スライドを値の表の梯子で詰める。IR は変えない(組み立ての中の写しに印を付ける)。
//
//   - 梯子は今の密度から下りる:normal のスライドは dense の組 → fit.shrink[0] → …、
//     dense のスライドは fit.shrink[0] → …。値の表の fit が無ければ dense までしか下りない
//   - 各段で組み直し、check と同じ1回の測り(measureBody)ではみ出しを、組み立てと同じ
//     サブシステムの検査(lintBlockSubsystems)で図の崩れを見て、最初に収まった段で組む。
//     「収まった」は layout-overflow も図の崩れ(figure-text-overflow・subsystem-min・
//     column-floor。fitBrokenCodes)も無いこと(#145。本文の箱に収まっても図の字が入らない
//     スライドは崩れて見える)。はみ出さずに図だけが崩れるスライドも梯子を下りる。
//     最後の段でも収まらなければ元の組み方のまま(警告が残り、-fit の build は誤り。api.go)
//   - 続きの表(continues)のひと続きのスライドは、どれか1枚がはみ出せば全部を同じ段まで
//     下げる(全部が収まる最初の段。列の幅と字が枚の間でそろう。fitGroupOf、#140)。
//     梯子は組のどれかが normal なら dense から、全部が dense なら fit.shrink[0] から。
//     ひと続きは広げない(fit_grow.go)
//   - 詰めたスライドには info の [FIT-APPLIED INFO](code fit-applied。段と残り pt)を出し、
//     容量に fit(段の名前)を入れる
//
// 詰めた段の本文は dense と同じ1か所で切り替える:本文のブロックを段の値を重ねた
// テンプレートで組み(bodyTemplate)、本文の箱に印(fit)を付けてその下のトークンを
// 段の値の表で引く(token.go)。薄いスライドを広げる梯子(fit.grow、#128)も同じ印と
// 同じ測りを使う(fit_grow.go)。

package boxloom

import (
	"fmt"
	"strings"
)

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
	group     []int   // 同じ段で組んだ組(続きの表のひと続き。1枚なら長さ1)
	over      bool    // 詰めた理由:元の組み方で本文がはみ出した
	broken    bool    // 詰めた理由:元の組み方で図が崩れた(fitBrokenCodes、#145)
}

// fitStepTry は梯子の1段(名前・写しへの当て方・値)。
type fitStepTry struct {
	name string
	set  func(*IRSlide)
	v    TemplateValues
}

// fitSlide は -fit のとき i 枚目(本文のスライド)を梯子で詰める・広げる(i を含む組
// fitGroupOf を同じ段で組み、i 枚目の分を返す)。どちらもしない・どの段も使えないなら
// nil(元のまま組む)。
func (t *Template) fitSlide(d *IRDeck, i int) (*BuiltSlide, *fitChoice, error) {
	built, choices, err := t.fitGroup(d, fitGroupOf(d, i))
	if err != nil {
		return nil, nil, err
	}
	return built[i], choices[i], nil
}

// fitGroupOf は i 枚目と同じ段で組むスライドの番号(昇順)。続きの表(continues)の
// ひと続きのスライドは、どれか1枚がはみ出せば全部を同じ段まで下げ(列の幅と字が枚の間で
// そろう)、広げはしない(#140)。続きの表はすぐ前のスライドに続くので、
// 組は「次のスライドが続きの表を持つ」で隣どうしをつないだ並び。表の無いスライドは i だけ。
func fitGroupOf(d *IRDeck, i int) []int {
	continues := func(j int) bool {
		if j <= 0 || j >= len(d.Slides) {
			return false
		}
		for _, k := range bodyTables(&d.Slides[j]) {
			if d.Slides[j].Body[k].Continues {
				return true
			}
		}
		return false
	}
	lo, hi := i, i
	for continues(lo) {
		lo--
	}
	for continues(hi + 1) {
		hi++
	}
	var out []int
	for j := lo; j <= hi; j++ {
		out = append(out, j)
	}
	return out
}

// fitGroup は組 group(fitGroupOf)を1つの段で詰める・広げる。どれかがはみ出すなら
// 詰め(下の梯子。全部が収まる最初の段)、どれもはみ出さず、1枚の組で fill-ratio が
// 下回る normal のスライドなら広げ(fit_grow.go)。返すのは段を変えたスライドだけ
// (番号 → 組んだスライド・段)。
func (t *Template) fitGroup(d *IRDeck, group []int) (map[int]*BuiltSlide, map[int]*fitChoice, error) {
	base, err := t.measureFitGroup(d, group, func(*IRSlide) {})
	if err != nil || base == nil {
		return nil, nil, err
	}
	if !anyBad(base) {
		if len(group) > 1 {
			// 続きの表のひと続きは広げない(最後の枚が薄いのは分けた表の常で、その枚の
			// ために広げると満ちた枚まで字が変わる。全部の枚をテンプレートの大きさのまま)。
			return nil, nil, nil
		}
		return t.fitGrowGroup(d, group, base)
	}
	baseOver, baseBroken := false, false
	for _, m := range base {
		baseOver, baseBroken = baseOver || m.over, baseBroken || m.broken
	}
	anyNormal := false
	for _, j := range group {
		anyNormal = anyNormal || !d.Slides[j].dense()
	}
	var steps []fitStepTry
	if anyNormal && t.Dense != (DenseValues{}) {
		steps = append(steps, fitStepTry{fitStepDense, func(s *IRSlide) { s.Density = densityDense }, t.densed().Values})
	}
	for k := range t.Fit.Shrink {
		r := fitRef{n: k + 1}
		steps = append(steps, fitStepTry{r.name(), func(s *IRSlide) { s.fit = r }, r.values(t).apply(t.Values)})
	}
	for _, st := range steps {
		ms, err := t.measureFitGroup(d, group, st.set)
		if err != nil {
			return nil, nil, err
		}
		if ms == nil || anyBad(ms) {
			continue
		}
		built := map[int]*BuiltSlide{}
		choices := map[int]*fitChoice{}
		dd := fitDeckCopy(d, group, st.set)
		for n, j := range group {
			if st.name == fitStepDense && d.Slides[j].dense() {
				continue // もう dense の枚は、dense の段では変わらない
			}
			b, err := t.buildSlide(dd, j)
			if err != nil {
				return nil, nil, err
			}
			built[j] = b
			choices[j] = &fitChoice{step: st.name, body: st.v.Body, blockGap: st.v.BlockGap,
				remaining: ms[n].remaining, ratio: ms[n].ratio, group: group, over: baseOver, broken: baseBroken}
		}
		return built, choices, nil
	}
	return nil, nil, nil
}

// anyBad は組のどれかが収まらない(はみ出すか図が崩れる)か。
func anyBad(ms []fitMeasure) bool {
	for _, m := range ms {
		if m.bad() {
			return true
		}
	}
	return false
}

// fitDeckCopy は d の写しで、group の枚のスライドの写しに set を当てたもの(IR は変えない)。
func fitDeckCopy(d *IRDeck, group []int, set func(*IRSlide)) *IRDeck {
	dd := *d
	dd.Slides = append([]IRSlide(nil), d.Slides...)
	for _, j := range group {
		set(&dd.Slides[j])
	}
	return &dd
}

// fitMeasure は1つの段で組んだときの測り値。ok が false なら本文を測れない(段を選ばない)。
type fitMeasure struct {
	ok        bool
	over      bool    // layout-overflow になるか
	broken    bool    // 図が崩れるか(fitBrokenCodes の警告が出る。#145)
	remaining float64 // 本文の残り(pt)
	ratio     float64 // fill-ratio(組んだ後の警告と同じ bodyFillRatio)
	area      float64 // 本文の領域の高さ(pt)
	need      float64 // 本文のブロックを縮めずに積んだ高さ(pt)
}

// bad は -fit がその段を「収まった」とみなさないか:本文がはみ出すか、図が崩れる(#145)。
func (m fitMeasure) bad() bool { return m.over || m.broken }

// fitBrokenCodes は -fit が「収まった」とみなさない図の崩れの警告(#145)。本文の箱に
// 収まっても、図の中の字が入らない・図が自分の最小を割る・並列のまとまりが図の下限を
// 割るスライドは崩れて見える(列の中の matrix で評価の列が 0.6pt 足りないだけで、
// 「p.35」がアプリで1字ずつ折れて行の高さが数倍になる)。
var fitBrokenCodes = map[string]bool{"figure-text-overflow": true, "subsystem-min": true, "column-floor": true}

// fitBrokenCode は組み立ての警告の文 w(頭の "slide N: " は有っても無くても)が図の崩れ
// なら、その code(fitBrokenCodes)。そうでなければ ""。
func fitBrokenCode(w string) string {
	if !strings.HasPrefix(w, "slide ") {
		w = "slide 1: " + w
	}
	if c := warningIssue("", w).Code; fitBrokenCodes[c] {
		return c
	}
	return ""
}

// measureFitGroup は group の全部の枚に set を当てた写しで、枚ごとに measureFitStep で測る。
// どれかの本文を測れなければ nil。
func (t *Template) measureFitGroup(d *IRDeck, group []int, set func(*IRSlide)) ([]fitMeasure, error) {
	dd := fitDeckCopy(d, group, set)
	var out []fitMeasure
	for _, j := range group {
		m, err := t.measureFitStep(dd, j)
		if err != nil || !m.ok {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// measureFitStep は i 枚目を(段を当てた写し d で)組み・並べ・描き、check と同じ
// measureBody で測る。
func (t *Template) measureFitStep(d *IRDeck, i int) (fitMeasure, error) {
	b, err := t.buildSlide(d, i)
	if err != nil {
		return fitMeasure{}, err
	}
	if b.Body == nil {
		return fitMeasure{}, nil
	}
	v := t.Values
	computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
	ratio := bodyFillRatio(b.Body) // renderPages と同じく、並べた直後・描く前に測る
	broken := false
	for _, w := range lintBlockSubsystems(b) { // renderPages と同じ検査(並べた後・描く前)
		broken = broken || fitBrokenCode(w) != ""
	}
	renderSlide(RenderContext{Out: NewPPTXBuilder(v.PageW, v.PageH), Roles: newCheckedRoles(t.Theme)}, b.Root)
	blocks, area, need, ok := measureBody(b.Root, b.Body)
	if !ok {
		return fitMeasure{}, nil
	}
	var kinds []string
	for _, blk := range d.Slides[i].Body {
		kinds = append(kinds, blk.Type)
	}
	return fitMeasure{ok: true, over: len(overflowWarnings(blocks, area, need, kinds)) > 0, broken: broken, remaining: round1(area - need), ratio: ratio, area: area, need: need}, nil
}

// fitAppliedWarning は調整したスライドの info の文(warningIssue が fit-applied にする)。
func fitAppliedWarning(c *fitChoice) string {
	chain := ""
	if len(c.group) > 1 {
		chain = fmt.Sprintf(" (the same step for every slide of the continued table, slides %d-%d)", c.group[0], c.group[len(c.group)-1])
	}
	if c.grow {
		return fmt.Sprintf("[FIT-APPLIED INFO] the body was underfilled (%.1f%%), so -fit built it with the template's %s step (body %gpt, blockGap %gpt), fill ratio %.1f%%, remaining %.1fpt. To keep the template's size, add content or merge it with a neighbouring thin slide",
			c.fromRatio*100, c.step, c.body, c.blockGap, c.ratio*100, c.remaining)
	}
	what := "the body overflowed"
	if len(c.group) > 1 {
		what = "the continued table overflowed"
	}
	switch {
	case c.broken && c.over:
		what += " and a figure broke (figure-text-overflow, subsystem-min or column-floor)"
	case c.broken:
		what = "a figure broke (figure-text-overflow, subsystem-min or column-floor)"
	}
	return fmt.Sprintf("[FIT-APPLIED INFO] %s, so -fit built it with the template's %s step (body %gpt, blockGap %gpt)%s, remaining %.1fpt. To keep the template's size, shorten the text or split the slide",
		what, c.step, c.body, c.blockGap, chain, c.remaining)
}
