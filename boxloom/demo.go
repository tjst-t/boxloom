// boxloom のデモ: box_engine.py(layout.go)+ Ganttサブエンジン(gantt.go)+
// 自前OOXMLライター(pptx.go)を組み合わせて、テーマ別に1枚の pptx を書く
// (WriteDemo。コマンドは引数なしで out/ に dark と light の2つを書く)。
//
// 本物のPythonソース(layout_pptx_poc_v11.py)を見ながら移植し直したバージョン。
// 構成はbuild_slide_spec()と1:1対応: header(title+subtitle) / chart_canvas
// (grow=1, basis=0) / footer(SPACE_BETWEEN)。凡例・注釈はすべてrender_gantt()
// 側(canvasの中)に属するので、outerのflexツリーには登場しない。

package boxloom

import (
	"fmt"
	"io"
)

const (
	typeTitle   = 30.0
	typeCaption = 10.5
	spaceSM     = 8.0
	spaceLG     = 24.0
)

// buildSlideSpec はtitle+subtitle / Ganttチャート(canvas) / footer を縦に
// 並べる。本物のbuild_slide_spec()と同じく、凡例や注釈はここには出てこない
// (render_gantt()の内部で完結する)。
func buildSlideSpec(th Theme, subtitle string) *Spec {
	header := Col([]*Spec{
		TextNode("ボード世代ごとのランタイム選択肢と利用可能期間(架空のデモ)",
			WithFontSize(typeTitle-6), WithColor(th.c("text_primary")), WithBold(true), WithHeight(30)),
		TextNode(subtitle, WithFontSize(typeCaption-1), WithColor(th.c("text_muted")), WithHeight(14)),
	}, WithGap(4))

	// コンテンツ(buildChart)は意味だけを持ち、描き方は登録簿から引く。
	// 高さは Gantt の自己申告(自然な高さ)を出発点に、残りを grow で受け取る。
	chart, err := DefaultSubsystems().Build("gantt", buildChart())
	if err != nil {
		panic(err)
	}
	chartCanvas := SubsystemNode(chart,
		WithGrow(1), WithBackground(th.c("card")), WithRadius(12))

	footer := Row([]*Spec{
		TextNode("Boxloom デモ", WithFontSize(typeCaption), WithColor(th.c("text_muted"))),
		TextNode("v12(Go移植/boxloom)", WithFontSize(typeCaption), WithColor(th.c("text_muted")), WithAlign("right")),
	}, WithJustify(JustifySpaceBetween))

	return Col([]*Spec{header, chartCanvas, footer},
		WithGap(spaceSM), WithPadding(spaceLG), WithBackground(th.c("bg")))
}

func renderSpec(ctx RenderContext, s *Spec) {
	p := ctx.Out
	if s.Background != "" {
		radius := 0.0
		if s.HasRadius {
			radius = s.Radius
		}
		bg := s.Box
		if s.Kind == KindSubsystem && s.Subsystem != nil {
			// 図の地(カード)は図が使う矩形に合わせる。横・縦の余りは地の外に白く残る。
			bg = subsystemDrawBox(s)
		}
		p.AddRect(bg, s.Background, radius)
	}
	switch s.Kind {
	case KindText:
		if len(s.Runs) > 0 {
			p.AddRichTextBox(s.Box, s.Runs, s.FontSize, s.Align, s.Anchor)
		} else {
			p.AddTextBox(s.Box, s.Text, s.FontSize, s.Color, s.Bold, s.Align, s.Anchor)
		}
		if s.Placeholder != "" {
			p.Shapes[len(p.Shapes)-1].ph = s.Placeholder
		}
		return
	case KindSubsystem:
		if s.Subsystem != nil {
			s.Subsystem.Render(ctx, subsystemDrawBox(s))
		}
		return
	}
	for _, c := range s.Children {
		renderSpec(ctx, c)
	}
}

// --- fill-ratioリンター。本物のcontent_fill_ratio()/lint_fill_ratio()の移植。 ---

func contentFillRatio(root *Spec) float64 {
	availableH := root.Box.Height - 2*root.Padding

	var deepestBottom func(s *Spec) float64
	deepestBottom = func(s *Spec) float64 {
		bottom := (s.Box.Y - root.Box.Y) + s.Box.Height
		for _, ch := range s.Children {
			if b := deepestBottom(ch); b > bottom {
				bottom = b
			}
		}
		return bottom
	}

	usedH, first := 0.0, true
	for _, ch := range root.Children {
		b := deepestBottom(ch)
		if first || b > usedH {
			usedH, first = b, false
		}
	}
	usedH -= root.Padding
	return usedH / availableH
}

const (
	fillRatioMin = 0.65
	fillRatioMax = 1.00
	floatEps     = 0.01
)

func lintFillRatio(ratio float64) string { return lintFillRatioMin(ratio, fillRatioMin) }

// lintFillRatioMin は下限をテンプレートの値の表(fillRatioMin)から受け取る形。
func lintFillRatioMin(ratio, minRatio float64) string {
	if ratio > fillRatioMax+floatEps {
		return fmt.Sprintf("[FILL-RATIO WARNING] overflow: %.1f%% (> %.0f%%)", ratio*100, fillRatioMax*100)
	}
	if ratio < minRatio {
		return fmt.Sprintf("[FILL-RATIO WARNING] underfilled: %.1f%% (< %.0f%%)", ratio*100, minRatio*100)
	}
	return ""
}

const pageW = 960.0 // 13.333in widescreen
const pageH = 540.0 // 7.5in

// DemoResult は WriteDemo の結果。
type DemoResult struct {
	// FillRatio は本文の埋まり具合(0〜1)。
	FillRatio float64
	// FillWarning は埋まり具合の警告(無ければ空)。
	FillWarning string
	// Warnings はそれ以外の警告(サブシステムの最小の大きさ・テーマに無い役割名)。
	Warnings []string
}

// demoSubtitles はデモのテーマごとの副題。
var demoSubtitles = map[string]string{
	"dark":  "boxloom: v11(Python)をGoへ移植。box_engine.pyの箱配置アルゴリズムと意味役割名によるテーマ分離をそのまま踏襲",
	"light": "boxloom: 自作エンジン + ブルー基調の明るいテーマ(内容は一切変更していない)",
}

// WriteDemo は従来の Go で組んだデモ(Gantt の1枚。v11 の再現)を、theme
// ("dark" か "light")の色で w に書く。安定した API ではない(docs/library.md)。
func WriteDemo(w io.Writer, theme string) (*DemoResult, error) {
	subtitle, ok := demoSubtitles[theme]
	if !ok {
		return nil, unknownTemplateError{theme}
	}
	th := mustTemplate(theme).Theme
	// v11 の再現なので、テンプレートの字体ではなく既定の一式で測る。
	defer useFonts(defaultFontSet())()
	spec := buildSlideSpec(th, subtitle)
	computeLayout(spec, 0, 0, pageW, pageH)

	ratio := contentFillRatio(spec)
	res := &DemoResult{FillRatio: ratio, FillWarning: lintFillRatio(ratio)}
	res.Warnings = lintSubsystems(spec)

	p := NewPPTXBuilder(pageW, pageH)
	p.ColorTheme = &DeckColors{Name: theme, Colors: th}
	roles := newCheckedRoles(th)
	renderSpec(RenderContext{Out: p, Roles: roles}, spec)
	if missing := roles.Missing(); len(missing) > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("[ROLE WARNING] theme has no value for roles: %v", missing))
	}
	if err := WriteDeck(w, []*PPTXBuilder{p}, ""); err != nil {
		return nil, err
	}
	return res, nil
}
