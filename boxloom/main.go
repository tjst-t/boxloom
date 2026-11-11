// boxloom のデモ: box_engine.py(layout.go)+ Ganttサブエンジン(gantt.go)+
// 自前OOXMLライター(pptx.go)を組み合わせて、テーマ別に2枚のpptxを生成する。
//
// 本物のPythonソース(layout_pptx_poc_v11.py)を見ながら移植し直したバージョン。
// 構成はbuild_slide_spec()と1:1対応: header(title+subtitle) / chart_canvas
// (grow=1, basis=0) / footer(SPACE_BETWEEN)。凡例・注釈はすべてrender_gantt()
// 側(canvasの中)に属するので、outerのflexツリーには登場しない。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
		p.AddRect(s.Box, s.Background, radius)
	}
	switch s.Kind {
	case KindText:
		if len(s.Runs) > 0 {
			p.AddRichTextBox(s.Box, s.Runs, s.FontSize, s.Align, s.Anchor)
			return
		}
		p.AddTextBox(s.Box, s.Text, s.FontSize, s.Color, s.Bold, s.Align, s.Anchor)
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

	warnings := lintSubsystems(spec)
	if warning != "" {
		warnings = append([]string{warning}, warnings...)
	}

	p := NewPPTXBuilder(pageW, pageH)
	roles := newCheckedRoles(th)
	renderSpec(RenderContext{Out: p, Roles: roles}, spec)
	if missing := roles.Missing(); len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("[ROLE WARNING] theme has no value for roles: %v", missing))
	}
	for _, w := range warnings[boolToInt(warning != ""):] {
		fmt.Printf("[%s] %s\n", outPath, w)
	}

	if err := p.Save(outPath); err != nil {
		panic(err)
	}
	fmt.Println("saved:", outPath)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "build" {
		os.Exit(runBuild(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "check" {
		os.Exit(runCheck(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "fonts" {
		os.Exit(runFonts(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "schema" {
		os.Exit(runSchema(os.Args[2:], os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "parse" {
		os.Exit(runParse(os.Args[2:], os.Stdout, os.Stderr))
	}
	if err := os.MkdirAll("out", 0o755); err != nil {
		panic(err)
	}
	generate("out/boxloom_dark.pptx", themeDark(),
		"boxloom: v11(Python)をGoへ移植。box_engine.pyの箱配置アルゴリズムと意味役割名によるテーマ分離をそのまま踏襲")
	generate("out/boxloom_light.pptx", themeLight(),
		"boxloom: 自作エンジン + ブルー基調の明るいテーマ(内容は一切変更していない)")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// runBuild は「boxloom build」: IR(JSON)か Boxloom Markdown を読んで pptx を書く。
//
//	go run . build [-o out.pptx] [-template dark|light] deck.json
//
// テンプレートは -template > IR の template > light の順で決まる。
func runBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	out := fs.String("o", "", "書き出す pptx(省くと out/<IR の名前>.pptx)")
	tmpl := fs.String("template", "", "テンプレートの名前(省くと IR の template)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: boxloom build [-o out.pptx] [-template dark|light] deck.json|deck.bx.md")
		return 2
	}
	in := fs.Arg(0)
	deck, err := readDeck(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	name := deck.Template
	if *tmpl != "" {
		name = *tmpl
	}
	t, err := LookupTemplate(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if *out == "" {
		*out = filepath.Join("out", deckBaseName(in)+".pptx")
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	warnings, err := t.RenderDeck(deck, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", in, err)
		return 1
	}
	for _, w := range warnings {
		fmt.Println(w)
	}
	fmt.Printf("saved: %s (%d slides, template %s)\n", *out, len(deck.Slides), t.Name)
	return 0
}
