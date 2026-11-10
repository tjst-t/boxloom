// box_engine.py (Python版の自作レイアウトエンジン)のGo移植。
//
// アルゴリズムはPython版と完全に同一(2パス: 交差軸をトップダウンで解決 →
// 主軸のbasisをボトムアップで計測 → grow/shrinkを一回だけ比例配分)。
// サポート範囲も意図的に同じだけ(行/列、gap、padding、grow、明示basis、
// AlignItems STRETCH/CENTER、JustifyContent START/SPACE_BETWEEN)に絞っている。
//
// Taffy(Rust)のflexible lengths反復凍結アルゴリズムは実装していない。これは
// Python版で実際のコンテンツに対して無害であることを数値比較で確認済みの割り切りを
// そのまま引き継いでいる。
package main

type AlignItems int

const (
	AlignStretch AlignItems = iota
	AlignCenter
)

type JustifyContent int

const (
	JustifyStart JustifyContent = iota
	JustifySpaceBetween
)

type Kind int

const (
	KindRow Kind = iota
	KindCol
	KindText
	KindCanvas
)

// LBox は確定した絶対座標(pt単位)。Python版のLBox/Boxに相当。
type LBox struct {
	X, Y, Width, Height float64
}

// CanvasRenderFunc は canvas 葉ノードに割り当てられた確定boxを受け取り、
// その領域の中身を自分で描画するコールバック(Ganttチャート等の専用サブエンジンが
// ここに差し込まれる)。
type CanvasRenderFunc func(p *PPTXBuilder, box LBox)

// Spec はPython版のSpecデータクラスに相当する、レイアウト木のノード。
type Spec struct {
	Kind     Kind
	Children []*Spec

	Gap            float64
	Padding        float64
	Background     string // 空文字列 = 背景なし
	Radius         float64
	HasRadius      bool
	AlignItems     AlignItems
	JustifyContent JustifyContent
	Grow           float64
	Basis          *float64
	Width          *float64
	Height         *float64

	Text     string
	FontSize float64
	Color    string
	Bold     bool
	Align    string // "left" | "center" | "right"
	Anchor   string // "top" | "middle" | "bottom"

	CanvasRender CanvasRenderFunc

	Box LBox // compute_layout()実行後に確定する
}

// Row / Col / TextNode / Canvas はPython版のrow()/col()/text()/canvas()ヘルパーに相当。
// 省略可能な引数はオプション関数(functional options)で渡す。

type SpecOpt func(*Spec)

func WithGap(v float64) SpecOpt            { return func(s *Spec) { s.Gap = v } }
func WithPadding(v float64) SpecOpt        { return func(s *Spec) { s.Padding = v } }
func WithBackground(v string) SpecOpt      { return func(s *Spec) { s.Background = v } }
func WithRadius(v float64) SpecOpt         { return func(s *Spec) { s.Radius = v; s.HasRadius = true } }
func WithAlignItems(v AlignItems) SpecOpt  { return func(s *Spec) { s.AlignItems = v } }
func WithJustify(v JustifyContent) SpecOpt { return func(s *Spec) { s.JustifyContent = v } }
func WithGrow(v float64) SpecOpt           { return func(s *Spec) { s.Grow = v } }
func WithBasis(v float64) SpecOpt          { return func(s *Spec) { s.Basis = &v } }
func WithWidth(v float64) SpecOpt          { return func(s *Spec) { s.Width = &v } }
func WithHeight(v float64) SpecOpt         { return func(s *Spec) { s.Height = &v } }
func WithFontSize(v float64) SpecOpt       { return func(s *Spec) { s.FontSize = v } }
func WithColor(v string) SpecOpt           { return func(s *Spec) { s.Color = v } }
func WithBold(v bool) SpecOpt              { return func(s *Spec) { s.Bold = v } }
func WithAlign(v string) SpecOpt           { return func(s *Spec) { s.Align = v } }
func WithAnchor(v string) SpecOpt          { return func(s *Spec) { s.Anchor = v } }

func apply(s *Spec, opts []SpecOpt) *Spec {
	for _, o := range opts {
		o(s)
	}
	return s
}

func Row(children []*Spec, opts ...SpecOpt) *Spec {
	return apply(&Spec{Kind: KindRow, Children: children, Color: defaultTextColor}, opts)
}

func Col(children []*Spec, opts ...SpecOpt) *Spec {
	return apply(&Spec{Kind: KindCol, Children: children, Color: defaultTextColor}, opts)
}

func TextNode(content string, opts ...SpecOpt) *Spec {
	s := &Spec{Kind: KindText, Text: content, FontSize: 12.5, Color: defaultTextColor, Align: "left", Anchor: "top"}
	return apply(s, opts)
}

func Canvas(render CanvasRenderFunc, opts ...SpecOpt) *Spec {
	s := &Spec{Kind: KindCanvas, CanvasRender: render, Color: defaultTextColor}
	return apply(s, opts)
}

const defaultTextColor = "F5F7FA"

// naturalSize は親からの確定サイズを一切前提にしない、コンテンツだけから決まる
// 自然サイズ。axis: 'w' または 'h'。box_engine.pyのnatural_size()と同一ロジック。
func naturalSize(s *Spec, axis byte) float64 {
	if axis == 'w' && s.Width != nil {
		return *s.Width
	}
	if axis == 'h' && s.Height != nil {
		return *s.Height
	}

	switch s.Kind {
	case KindText:
		if axis == 'w' {
			return naturalTextWidth(s)
		}
		return s.FontSize * lineHeightMult
	case KindCanvas:
		return 0.0
	}

	// row / col コンテナ
	if len(s.Children) == 0 {
		return 2 * s.Padding
	}
	isRow := s.Kind == KindRow
	mainAxis := byte('h')
	if isRow {
		mainAxis = 'w'
	}
	if axis == mainAxis {
		total := 0.0
		for _, c := range s.Children {
			total += naturalSize(c, axis)
		}
		total += s.Gap * float64(max(0, len(s.Children)-1))
		return total + 2*s.Padding
	}
	m := naturalSize(s.Children[0], axis)
	for _, c := range s.Children[1:] {
		if v := naturalSize(c, axis); v > m {
			m = v
		}
	}
	return m + 2*s.Padding
}

// computeLayout はspec(とその子孫)に確定した絶対座標(spec.Box)を割り当てる。
// box_engine.pyのcompute_layout()と同一アルゴリズム。
func computeLayout(s *Spec, x, y, width, height float64) {
	s.Box = LBox{X: x, Y: y, Width: width, Height: height}

	if (s.Kind != KindRow && s.Kind != KindCol) || len(s.Children) == 0 {
		return
	}

	isRow := s.Kind == KindRow
	ix, iy := x+s.Padding, y+s.Padding
	iw := max0(width - 2*s.Padding)
	ih := max0(height - 2*s.Padding)
	innerMain, innerCross := ih, iw
	if isRow {
		innerMain, innerCross = iw, ih
	}

	children := s.Children
	n := len(children)
	gapTotal := s.Gap * float64(max(0, n-1))

	// --- Phase 1: 交差軸サイズ(トップダウン) ---
	crossSizes := make([]float64, n)
	for i, c := range children {
		var explicitCross *float64
		if isRow {
			explicitCross = c.Height
		} else {
			explicitCross = c.Width
		}
		switch {
		case explicitCross != nil:
			crossSizes[i] = *explicitCross
		case s.AlignItems == AlignStretch:
			crossSizes[i] = innerCross
		default: // CENTER
			ax := byte('w')
			if isRow {
				ax = 'h'
			}
			crossSizes[i] = naturalSize(c, ax)
		}
	}

	// --- Phase 2: 主軸方向のbasis(ボトムアップ) ---
	basisSizes := make([]float64, n)
	for i, c := range children {
		var explicitMain *float64
		if isRow {
			explicitMain = c.Width
		} else {
			explicitMain = c.Height
		}
		cs := crossSizes[i]
		switch {
		case c.Basis != nil:
			basisSizes[i] = *c.Basis
		case explicitMain != nil:
			basisSizes[i] = *explicitMain
		case c.Kind == KindText:
			if isRow {
				basisSizes[i] = naturalTextWidth(c)
			} else {
				basisSizes[i] = measureTextHeight(c.Text, c.FontSize, cs, c.Bold)
			}
		case c.Kind == KindCanvas:
			basisSizes[i] = 0.0
		default:
			mainAxisOfParent := byte('h')
			if isRow {
				mainAxisOfParent = 'w'
			}
			basisSizes[i] = naturalSize(c, mainAxisOfParent)
		}
	}

	// --- Phase 3: grow/shrinkの単純な一回配分(凍結ループなし) ---
	basisTotal := 0.0
	for _, b := range basisSizes {
		basisTotal += b
	}
	free := innerMain - basisTotal - gapTotal
	totalGrow := 0.0
	for _, c := range children {
		totalGrow += c.Grow
	}

	mainSizes := make([]float64, n)
	switch {
	case free > 0 && totalGrow > 0:
		for i, c := range children {
			mainSizes[i] = basisSizes[i] + free*(c.Grow/totalGrow)
		}
	case free < 0 && basisTotal > 0:
		for i, b := range basisSizes {
			v := b + free*(b/basisTotal)
			if v < 0 {
				v = 0
			}
			mainSizes[i] = v
		}
	default:
		copy(mainSizes, basisSizes)
	}

	// --- Phase 4: justify_content ---
	usedMain := gapTotal
	for _, m := range mainSizes {
		usedMain += m
	}
	leftover := innerMain - usedMain
	gapExtra, startOffset := 0.0, 0.0
	if s.JustifyContent == JustifySpaceBetween && n > 1 && leftover > 0 {
		gapExtra = leftover / float64(n-1)
	}

	// --- Phase 5: 位置確定 + 再帰 ---
	cursor := iy + startOffset
	if isRow {
		cursor = ix + startOffset
	}
	for i, c := range children {
		cs := crossSizes[i]
		var explicitCross *float64
		if isRow {
			explicitCross = c.Height
		} else {
			explicitCross = c.Width
		}
		var crossPos float64
		if explicitCross != nil || s.AlignItems == AlignStretch {
			crossPos = ix
			if isRow {
				crossPos = iy
			}
		} else { // CENTER
			if isRow {
				crossPos = iy + (innerCross-cs)/2
			} else {
				crossPos = ix + (innerCross-cs)/2
			}
		}

		if isRow {
			computeLayout(c, cursor, crossPos, mainSizes[i], cs)
		} else {
			computeLayout(c, crossPos, cursor, cs, mainSizes[i])
		}
		cursor += mainSizes[i] + s.Gap + gapExtra
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max0(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
