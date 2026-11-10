// boxloom のデモ: box_engine.py(layout.go)+ Ganttサブエンジン(gantt.go)+
// 自前OOXMLライター(pptx.go)を組み合わせて、テーマ別に2枚のpptxを生成する。
//
// 本物のPythonソース(layout_pptx_poc_v11.py)を見ながら移植し直したバージョン。
// 構成はbuild_slide_spec()と1:1対応: header(title+subtitle) / chart_canvas
// (grow=1, basis=0) / footer(SPACE_BETWEEN)。凡例・注釈はすべてrender_gantt()
// 側(canvasの中)に属するので、outerのflexツリーには登場しない。
package main

import (
	"fmt"
	"os"
)

func themeDark() Theme {
	return Theme{
		"bg": "12151C", "card": "1C212C", "card_alt": "232939",
		"text_primary": "F5F7FA", "text_muted": "97A1B3", "grid": "2E3444",
		"support": "34C08F", "conditional": "4C8DFF", "none": "3A4150",
		"danger": "E0555A", "risk": "6B3540", "accent": "F2A93B",
		"on_conditional": "F5F7FA",
	}
}

func themeLight() Theme {
	return Theme{
		"bg": "FFFFFF", "card": "F7F9FD", "card_alt": "E7EDFA",
		"text_primary": "12203D", "text_muted": "5C6B85", "grid": "D7E0F2",
		"support": "1E9E6B", "conditional": "0033CC", "none": "AAB4C6",
		"danger": "C0293C", "risk": "F4D9DE", "accent": "0033CC",
		"on_conditional": "FFFFFF",
	}
}

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

	chart := buildChart()
	chartCanvas := Canvas(func(p *PPTXBuilder, box LBox) { renderGantt(p, box, chart, th) },
		WithGrow(1), WithBasis(0), WithBackground(th.c("card")), WithRadius(12))

	footer := Row([]*Spec{
		TextNode("Boxloom デモ", WithFontSize(typeCaption), WithColor(th.c("text_muted"))),
		TextNode("v12(Go移植/boxloom)", WithFontSize(typeCaption), WithColor(th.c("text_muted")), WithAlign("right")),
	}, WithJustify(JustifySpaceBetween))

	return Col([]*Spec{header, chartCanvas, footer},
		WithGap(spaceSM), WithPadding(spaceLG), WithBackground(th.c("bg")))
}

func renderSpec(p *PPTXBuilder, s *Spec) {
	if s.Background != "" {
		radius := 0.0
		if s.HasRadius {
			radius = s.Radius
		}
		p.AddRect(s.Box, s.Background, radius)
	}
	switch s.Kind {
	case KindText:
		p.AddTextBox(s.Box, s.Text, s.FontSize, s.Color, s.Bold, s.Align, s.Anchor)
		return
	case KindCanvas:
		if s.CanvasRender != nil {
			s.CanvasRender(p, s.Box)
		}
		return
	}
	for _, c := range s.Children {
		renderSpec(p, c)
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

func lintFillRatio(ratio float64) string {
	if ratio > fillRatioMax+floatEps {
		return fmt.Sprintf("[FILL-RATIO WARNING] overflow: %.1f%% (> %.0f%%)", ratio*100, fillRatioMax*100)
	}
	if ratio < fillRatioMin {
		return fmt.Sprintf("[FILL-RATIO WARNING] underfilled: %.1f%% (< %.0f%%)", ratio*100, fillRatioMin*100)
	}
	return ""
}

const pageW = 960.0 // 13.333in widescreen
const pageH = 540.0 // 7.5in

func generate(outPath string, th Theme, subtitle string) {
	spec := buildSlideSpec(th, subtitle)
	computeLayout(spec, 0, 0, pageW, pageH)

	ratio := contentFillRatio(spec)
	warning := lintFillRatio(ratio)
	if warning != "" {
		fmt.Printf("[%s] content fill ratio: %.1f%%  %s\n", outPath, ratio*100, warning)
	} else {
		fmt.Printf("[%s] content fill ratio: %.1f%%  OK\n", outPath, ratio*100)
	}

	p := NewPPTXBuilder(pageW, pageH)
	renderSpec(p, spec)
	if warning != "" {
		p.SetNotes(warning)
	}

	if err := p.Save(outPath); err != nil {
		panic(err)
	}
	fmt.Println("saved:", outPath)
}

func main() {
	if err := os.MkdirAll("out", 0o755); err != nil {
		panic(err)
	}
	generate("out/boxloom_dark.pptx", themeDark(),
		"boxloom: v11(Python)をGoへ移植。box_engine.pyの箱配置アルゴリズムと意味役割名によるテーマ分離をそのまま踏襲")
	generate("out/boxloom_light.pptx", themeLight(),
		"boxloom: 自作エンジン + ブルー基調の明るいテーマ(内容は一切変更していない)")
}
