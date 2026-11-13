// 本文の密度(content の density。#116)。
//
// 作者は本文のスライドに density: dense(Markdown は `# 題 {density=dense}`)を書いて、
// 「このページは参照用に詰めて読ませる」ことを選ぶ。Boxloom は自分では縮めない
// (はみ出しは layout-overflow の警告と hint で知らせるだけ)。
//
// dense のスライドでは、本文の部品・サブシステムがテンプレートの値の表の dense の組
// (DenseValues:本文・入れ子の箇条・表・小さい字の大きさとブロック・項目の間、#136 で
// 表のセルの余白・pairs の対の行の間と印・matrix の行の題の字、#139 で図と引用の字・寸法)を使う。
// 題・リード・出典・フッターは values のまま。切り替えは部品の段の1か所:
//   - 本文のブロックを組む口(skeletonContext.blocks)が、dense の値を重ねた
//     テンプレート(densed)で部品を組む。部品が値の表を直接読む所(表の字など)と
//     サブシステム(図の字の段)はこれで dense になる。
//   - 本文の箱(frameBody)に印(dense)を付け、resolveSpec がその下のトークンを dense の
//     値の表で引く(ブロックの間・部品が書いた body・small などの名前)。
//
// 部品は密度を知らない(条件分岐を持たない)。測りも描きも同じ解決した値を読む。

package boxloom

import "fmt"

// densed は本文の値を dense の組に替えたテンプレート(部品の登録簿・骨組み・画像は
// 共有する)。サブシステムの登録簿は、作ったサブシステムの字の段を dense の値に
// 替える写し。
func (t *Template) densed() *Template {
	if t.Dense == (DenseValues{}) {
		return t // dense の組を持たない(値の表を通らずに作った)テンプレート
	}
	td := *t
	td.Values = t.Dense.apply(t.Values)
	if t.Subsystems != nil {
		td.Subsystems = t.Subsystems.withTextSizes(td.Values, t.Marks.Point)
	}
	return &td
}

// withTextSizes は、作ったサブシステム(図・shapes)の字の段を v から入れ直す
// 登録簿の写し。テンプレートが差し替えた描き方も、作ったものが同梱の図なら同じに効く。
func (r *SubsystemRegistry) withTextSizes(v TemplateValues, point string) *SubsystemRegistry {
	c := r.Clone()
	for k, f := range r.factories {
		c.factories[k] = func(data any) (Subsystem, error) {
			s, err := f(data)
			if err != nil {
				return nil, err
			}
			setTextSizes(s, v, point)
			return s, nil
		}
	}
	return c
}

// setTextSizes は図の字の段(FigureText)と shapes の字の段を v から入れる。orgchart・
// phases・matrix は dense・fit で詰められる寸法(v.fig、#139)も図の値の写しに当てる。
// 箇条の印は作ったものが持っていればそのまま。ほかのサブシステムは変えない
// (Gantt の字は subsystems.gantt の寸法)。
func setTextSizes(s Subsystem, v TemplateValues, point string) {
	var p **FigureText
	switch x := s.(type) {
	case *ProcessSubsystem:
		p = &x.Text
	case *OrgChartSubsystem:
		p = &x.Text
		if v.fig.set() {
			x.V = v.fig.orgchart(*x.vals())
		}
	case *MatrixSubsystem:
		p = &x.Text
		if v.fig.set() {
			x.V = v.fig.matrix(*x.vals())
		}
	case *FunnelSubsystem:
		p = &x.Text
	case *PhasesSubsystem:
		p = &x.Text
		if v.fig.set() {
			x.V = v.fig.phases(*x.vals())
		}
	case *ChartSubsystem:
		p = &x.Text
	case *CycleSubsystem:
		p = &x.Text
	case *ShapesSubsystem:
		x.TextSizes = shapeTextSizes(v)
		return
	default:
		return
	}
	ft := figureTextFrom(v)
	ft.Point = point
	if *p != nil {
		ft.Point, ft.Sizes = (*p).Point, (*p).Sizes
	}
	*p = &ft
}

// markDense は箱の印:この下のトークンは dense の値の表で引く(resolveSpec)。
func markDense() SpecOpt { return func(s *Spec) { s.dense = true } }

// addDenseRemaining は normal の本文のスライドの容量に、dense にしたときの remaining を
// 入れる(dense の値で組み直し、check と同じ measureBody で測る)。caps の path は
// /slides/S(IR のスライドの番号)。
func (t *Template) addDenseRemaining(d *IRDeck, caps []SlideCapacity) error {
	for k := range caps {
		var si int
		if _, err := fmt.Sscanf(caps[k].Path, "/slides/%d", &si); err != nil || si < 0 || si >= len(d.Slides) {
			continue
		}
		if d.Slides[si].Type != "content" || d.Slides[si].dense() || caps[k].Split != nil {
			continue
		}
		area, need, ok, err := t.measureDense(d, si)
		if err != nil {
			return fmt.Errorf("slides[%d]: %w", si, err)
		}
		if ok {
			r := round1(area - need)
			caps[k].DenseRemaining = &r
		}
	}
	return nil
}

// measureDense は si 枚目を dense にして組み・並べ・描き、本文の領域と要る高さを測る
// (IR は書き換えない。スライドの写しの density だけを変える)。
func (t *Template) measureDense(d *IRDeck, si int) (area, need float64, ok bool, err error) {
	dd := *d
	dd.Slides = append([]IRSlide(nil), d.Slides...)
	dd.Slides[si].Density = densityDense
	b, err := t.buildSlide(&dd, si)
	if err != nil {
		return 0, 0, false, err
	}
	if b.Body == nil {
		return 0, 0, false, nil
	}
	v := t.Values
	computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
	// measureBody は描いた後に測る(renderPages と同じ順)。描いたものは捨てる。
	renderSlide(RenderContext{Out: NewPPTXBuilder(v.PageW, v.PageH), Roles: newCheckedRoles(t.Theme)}, b.Root)
	_, area, need, ok = measureBody(b.Root, b.Body)
	return area, need, ok, nil
}
