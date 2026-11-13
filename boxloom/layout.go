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

package boxloom

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
	KindSubsystem // 葉: サブシステム(subsystem.go)
)

// LBox は確定した絶対座標(pt単位)。Python版のLBox/Boxに相当。
type LBox struct {
	X, Y, Width, Height float64
}

// CanvasRenderFunc は、自然な大きさを申告しない簡易なサブシステムを関数1つで
// 書くためのもの(Canvas ヘルパー用)。
type CanvasRenderFunc func(ctx RenderContext, box LBox)

// Spec はPython版のSpecデータクラスに相当する、レイアウト木のノード。
type Spec struct {
	Kind     Kind
	Children []*Spec

	Gap     float64
	Padding float64
	// PaddingY は上下の余白(nil なら Padding と同じ)。Padding は左右の余白になる。
	PaddingY       *float64
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
	Runs     []TextRun // 空でなければ Text の代わりに使う(強調などの装飾つき)
	FontSize float64
	Color    string
	Bold     bool
	Align    string // "left" | "center" | "right"
	Anchor   string // "top" | "middle" | "bottom"
	// Placeholder は文字の葉を PPTX のスライドのレイアウトのプレースホルダーとして
	// 書く種類("title" | "ctrTitle" | "subTitle"。空ならただのテキストボックス)。
	// 位置と字の大きさはエンジンの決めたままで、見た目は変わらない。
	Placeholder string
	// List は箇条の並び(中の文字の葉が項目)。測り方と置き方はふつうの Col と
	// 同じで、PPTX では項目を1つの文字の枠の段落として書く(renderList)。
	List bool
	// card はカード(compare・stat の1枚、takeaway の地)。PPTX では地と中の文字を
	// 文字を持つ1つの図形として書く(renderCard)。測り方と置き方はふつうの Col と同じ。
	card bool

	Subsystem Subsystem // KindSubsystem のときだけ
	Place     Place     // KindSubsystem: 割り当てが図より大きいときの置き方(既定は左上)

	// irType は本文のブロックを作った IR の type(はみ出しの警告の文に使う)。
	irType string
	// columns は並列のまとまり(columns)の行が持つ、まとまりごとの部品
	// (はみ出しをまとまりごとに測る。blocks.go)。
	columns *columnsInfo
	// cards は横に同じ幅のカードを並べる行(compare・stat)が持つ、IR の
	// 並びの名前("options"・"items")。容量の計測でカードごとの高さを出し、
	// 一番高いカードがブロックの高さを決めることを作者に見せる。
	cards string
	// pairs は対の並び(pairs)の箱が持つ、対ごとの部品(はみ出しを対ごとに測る。pairs.go)。
	pairs *pairsInfo
	// nestedSlack は、この箱が列の中で足りないとき、中のサブシステムの縮められる
	// 分を Phase3a で先に返すか(並列のまとまりで、画像を文字より先に縮めるため)。
	nestedSlack bool
	// noShrink は、列・行が足りないとき、この箱を縮めない印(骨組みの頭・出典・
	// フッター)。足りない分は印の無い兄弟(本文の箱)だけが縮めて受け持つ。
	noShrink bool
	// keepLine は、1行に収めたい文字の葉(Keep の塊だけの葉)の名前(警告の文に
	// 使う。例 "stat: items[1]")。幅に入りきらなければ stat-value の警告(keepline.go)。
	keepLine string
	// frame は骨組みの部品が付ける枠の印(frameBody・frameMain。skeleton.go)。
	// コアはこの印で本文の箱(容量・はみ出し・埋まり具合を測る)と、本文を持たない
	// スライドのページに収まるべき箱を見つける。
	frame frameMark
	// irPath はこの箱を作った IR の場所(fromIR。layout の出力の path に使う)。
	// "/" で始まれば文書の中の絶対の場所、そうでなければスライドからの相対
	// ("title"・"lead" など)。空なら親の場所を継ぐ。
	irPath string
	// tok は部品が書いたトークン(値の表の名前・段の刻み・役割名)。resolveSpec が
	// 上の数と色に直す(token.go)。layout の出力はこれを tokens に出す。
	tok specTokens

	Box LBox // compute_layout()実行後に確定する
}

// Row / Col / TextNode / Canvas はPython版のrow()/col()/text()/canvas()ヘルパーに相当。
// 省略可能な引数はオプション関数(functional options)で渡す。

type SpecOpt func(*Spec)

func WithGap(v float64) SpecOpt     { return func(s *Spec) { s.Gap = v } }
func WithPadding(v float64) SpecOpt { return func(s *Spec) { s.Padding = v } }

// WithPaddingXY は左右(x)と上下(y)で違う余白。
func WithPaddingXY(x, y float64) SpecOpt {
	return func(s *Spec) { s.Padding = x; s.PaddingY = &y }
}

// padX は左右の余白、padY は上下の余白。
func (s *Spec) padX() float64 { return s.Padding }
func (s *Spec) padY() float64 {
	if s.PaddingY != nil {
		return *s.PaddingY
	}
	return s.Padding
}

// padOn は軸(w=横・h=縦)の側の余白。
func (s *Spec) padOn(axis byte) float64 {
	if axis == 'h' {
		return s.padY()
	}
	return s.padX()
}
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
func WithPlace(x, y PlaceAlign) SpecOpt    { return func(s *Spec) { s.Place = Place{X: x, Y: y} } }
func WithPlaceholder(v string) SpecOpt     { return func(s *Spec) { s.Placeholder = v } }

// --- 計測と書き出しの印(配置の段の語彙。docs/layers.md の2.1) ---
//
// 部品はこの語彙だけで印を付け、はみ出しの警告・容量・keepline・PPTX の書き出しは
// 印を読むだけにする。当面は package の中だけ(公開は後で決める)。

// markIRType は、はみ出しの警告に出す IR のブロックの種類。
func markIRType(t string) SpecOpt { return func(s *Spec) { s.irType = t } }

// measureColumnGroups は測る単位の印: 並列のまとまりの行で、まとまりごとにはみ出しを測る。
func measureColumnGroups(info *columnsInfo) SpecOpt { return func(s *Spec) { s.columns = info } }

// measureCardRow は測る単位の印: 横に同じ幅のカードを並べる行で、カードごとの高さを
// 容量に出す。name は IR の並びの名前("options"・"items")。
func measureCardRow(name string) SpecOpt { return func(s *Spec) { s.cards = name } }

// measurePairRows は測る単位の印: 対の並び(pairs)の箱で、対(行)ごとにはみ出しを測る。
func measurePairRows(info *pairsInfo) SpecOpt { return func(s *Spec) { s.pairs = info } }

// keepLine は文字の葉の属性: 1行に収めたい文字。name は警告の文に使う名前。
func keepLine(name string) SpecOpt { return func(s *Spec) { s.keepLine = name } }

// nestedSlack は箱の属性: 列の中で足りないとき、中の図を文字より先に縮める。
func nestedSlack() SpecOpt { return func(s *Spec) { s.nestedSlack = true } }

// noShrink は箱の属性: 親の主軸が足りないとき、この箱は自然の大きさのまま縮めない
// (Min を自然の大きさにする)。足りない分は印の無い兄弟だけで縮める。骨組みの頭・
// 出典・フッターに付け、本文のはみ出しを本文の箱の中に閉じる(#96)。
func noShrink() SpecOpt { return func(s *Spec) { s.noShrink = true } }

// fromIR は箱を作った IR の場所(layout の出力の path。読むのは layoutdump.go だけで、
// 配置・計測・書き出しは見ない)。rel はスライドからの相対("title" など)か、
// "/" で始まる文書の中の場所("/meta/title" など)。
func fromIR(rel string) SpecOpt { return func(s *Spec) { s.irPath = rel } }

// markFrame は骨組みの枠の印: 本文の箱(frameBody)か、本文を持たないスライド
// (区切り・主張)でページに収まるべき箱(frameMain)。どちらも骨組みの根の直接の子に付ける。
func markFrame(m frameMark) SpecOpt { return func(s *Spec) { s.frame = m } }

// asList は箱の種類: 箇条(PPTX では項目を1つの文字の枠の段落として書く)。
func asList() SpecOpt { return func(s *Spec) { s.List = true } }

// asCard は箱の種類: カード(PPTX では地と中の文字を1つの図形として書く)。
func asCard() SpecOpt { return func(s *Spec) { s.card = true } }

func apply(s *Spec, opts []SpecOpt) *Spec {
	for _, o := range opts {
		o(s)
	}
	return s
}

func Row(children []*Spec, opts ...SpecOpt) *Spec {
	return apply(&Spec{Kind: KindRow, Children: children}, opts)
}

func Col(children []*Spec, opts ...SpecOpt) *Spec {
	return apply(&Spec{Kind: KindCol, Children: children}, opts)
}

func TextNode(content string, opts ...SpecOpt) *Spec {
	s := &Spec{Kind: KindText, Text: content, FontSize: 12.5, Align: "left", Anchor: "top"}
	return apply(s, opts)
}

// RichTextNode は装飾つきの文字の葉。
func RichTextNode(runs []TextRun, opts ...SpecOpt) *Spec {
	s := &Spec{Kind: KindText, Runs: runs, Text: runsText(runs), FontSize: 12.5, Align: "left", Anchor: "top"}
	return apply(s, opts)
}

// textNaturalWidth / textHeightFor は、文字の葉の幅と、幅を決めたときの高さ。
func textNaturalWidth(s *Spec) float64 {
	if len(s.Runs) > 0 {
		return naturalRunsWidth(s.Runs, s.FontSize)
	}
	return naturalTextWidth(s)
}

func textHeightFor(s *Spec, width float64) float64 {
	if len(s.Runs) > 0 {
		return measureRunsHeight(s.Runs, s.FontSize, width)
	}
	return measureTextHeight(s.Text, s.FontSize, width, s.Bold)
}

// heightForWidth は、幅を決めたときの高さ。列(Col)の中に置いた入れ子の箱の
// 高さを、中の文字の折り返しまで含めて測るのに使う。naturalSize の 'h' は
// 文字を1行と見なすので、行(Row)の中で文字が折り返すと高さが足りなくなる
// (Python 版から引き継いだ割り切り)。それを補うための、Go 版での拡張。
func heightForWidth(s *Spec, width float64) float64 {
	if s.Height != nil {
		return *s.Height
	}
	if s.pairs != nil {
		splitPairs(s, width)
	}
	switch s.Kind {
	case KindText:
		return textHeightFor(s, width)
	case KindSubsystem:
		if s.Subsystem == nil {
			return 0
		}
		return s.Subsystem.Measure(Constraint{Width: width}).Natural.Height
	}
	if len(s.Children) == 0 {
		return 2 * s.padY()
	}
	inner := max0(width - 2*s.padX())
	if s.Kind == KindCol {
		total := s.Gap * float64(max(0, len(s.Children)-1))
		for _, c := range s.Children {
			cw := inner
			if c.Width != nil {
				cw = *c.Width
			} else if s.AlignItems != AlignStretch {
				cw = naturalSize(c, 'w')
			}
			total += heightForWidth(c, cw)
		}
		return total + 2*s.padY()
	}
	// 行: 子の幅を決めるために一度並べてみる(高さは仮)。並べた結果の Box は
	// あとで本当の computeLayout が上書きする。
	computeLayout(s, 0, 0, width, naturalSize(s, 'h'))
	m := 0.0
	for _, c := range s.Children {
		if v := heightForWidth(c, c.Box.Width); v > m {
			m = v
		}
	}
	return m + 2*s.padY()
}

// SubsystemNode はサブシステムを葉として置く。大きさは Measure の自己申告で
// 決まる(明示の Width/Height/Basis があればそちらが勝つ)。
func SubsystemNode(sub Subsystem, opts ...SpecOpt) *Spec {
	s := &Spec{Kind: KindSubsystem, Subsystem: sub}
	return apply(s, opts)
}

// Canvas は関数1つで描く簡易なサブシステムを置く。自然な大きさは 0 を申告する
// ので、場所は grow で受け取る(旧来の canvas と同じふるまい)。
func Canvas(render CanvasRenderFunc, opts ...SpecOpt) *Spec {
	return SubsystemNode(funcSubsystem{render}, opts...)
}

type funcSubsystem struct{ render CanvasRenderFunc }

func (funcSubsystem) Name() string                         { return "canvas" }
func (funcSubsystem) Measure(Constraint) Measurement       { return Measurement{} }
func (f funcSubsystem) Render(ctx RenderContext, box LBox) { f.render(ctx, box) }

// subsystemNatural は Measure の自然な大きさのうち axis の値を返す。
func subsystemNatural(s *Spec, axis byte, c Constraint) float64 {
	if s.Subsystem == nil {
		return 0
	}
	n := s.Subsystem.Measure(c).Natural
	if axis == 'w' {
		return n.Width
	}
	return n.Height
}

// subsystemMaxMain はサブシステムが申告した主軸の上限(無ければ 0)。
func subsystemMaxMain(c *Spec, isRow bool, cross float64) float64 {
	if c.Kind != KindSubsystem || c.Subsystem == nil {
		return 0
	}
	if isRow {
		return c.Subsystem.Measure(Constraint{Height: cross}).Max.Width
	}
	return c.Subsystem.Measure(Constraint{Width: cross}).Max.Height
}

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
			return textNaturalWidth(s)
		}
		return s.FontSize * lineHeightMult
	case KindSubsystem:
		return subsystemNatural(s, axis, Constraint{})
	}

	// row / col コンテナ
	if len(s.Children) == 0 {
		return 2 * s.padOn(axis)
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
		return total + 2*s.padOn(axis)
	}
	m := naturalSize(s.Children[0], axis)
	for _, c := range s.Children[1:] {
		if v := naturalSize(c, axis); v > m {
			m = v
		}
	}
	return m + 2*s.padOn(axis)
}

// computeLayout はspec(とその子孫)に確定した絶対座標(spec.Box)を割り当てる。
// box_engine.pyのcompute_layout()と同一アルゴリズム。
func computeLayout(s *Spec, x, y, width, height float64) {
	s.Box = LBox{X: x, Y: y, Width: width, Height: height}
	if s.pairs != nil {
		splitPairs(s, width)
	}

	if (s.Kind != KindRow && s.Kind != KindCol) || len(s.Children) == 0 {
		return
	}

	isRow := s.Kind == KindRow
	ix, iy := x+s.padX(), y+s.padY()
	iw := max0(width - 2*s.padX())
	ih := max0(height - 2*s.padY())
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
				basisSizes[i] = textNaturalWidth(c)
			} else {
				basisSizes[i] = textHeightFor(c, cs)
			}
		case c.Kind == KindSubsystem:
			// 交差軸の大きさ(cs)を伝えて、主軸の自然な大きさを自己申告させる。
			if isRow {
				basisSizes[i] = subsystemNatural(c, 'w', Constraint{Height: cs})
			} else {
				basisSizes[i] = subsystemNatural(c, 'h', Constraint{Width: cs})
			}
		case !isRow:
			// 列の中の入れ子の箱: 幅(cs)を決めたときの高さ。中に文字の折り返しが
			// 無ければ naturalSize と同じ値になる。
			basisSizes[i] = heightForWidth(c, cs)
		default:
			basisSizes[i] = naturalSize(c, 'w')
		}
	}

	// --- Phase 3: grow/shrinkの単純な一回配分(凍結ループなし) ---
	basisTotal := 0.0
	for _, b := range basisSizes {
		basisTotal += b
	}
	free := innerMain - basisTotal - gapTotal

	// --- Phase 3a: 足りないとき、サブシステムが自己申告した「自然な大きさと
	// 最小の大きさの間のゆとり」を先に返してもらう。テキストなどほかの子は
	// ここでは縮めない。サブシステムを含まない木では何もしない(Python版と同じ)。
	if free < 0 {
		slack := make([]float64, n)
		totalSlack := 0.0
		for i, c := range children {
			// 印を付けた入れ子の箱(並列のまとまりの行と、まとまりの本文)も、中の
			// サブシステムが縮められる分(縮めずに要る高さとの差)を先に返す。
			// 印の無い箱はこれまでどおり(従来の出力を変えない)。
			if !isRow && c.nestedSlack && c.Basis == nil && c.Height == nil && hasSubsystem(c) {
				if v := basisSizes[i] - requiredHeight(c, crossSizes[i]); v > 0 {
					slack[i] = v
					totalSlack += v
				}
				continue
			}
			if c.Kind != KindSubsystem || c.Subsystem == nil || c.Basis != nil {
				continue
			}
			var explicitMain *float64
			if isRow {
				explicitMain = c.Width
			} else {
				explicitMain = c.Height
			}
			if explicitMain != nil {
				continue
			}
			var minMain float64
			if isRow {
				minMain = c.Subsystem.Measure(Constraint{Height: crossSizes[i]}).Min.Width
			} else {
				minMain = c.Subsystem.Measure(Constraint{Width: crossSizes[i]}).Min.Height
			}
			if v := basisSizes[i] - minMain; v > 0 {
				slack[i] = v
				totalSlack += v
			}
		}
		if totalSlack > 0 {
			take := -free
			if take > totalSlack {
				take = totalSlack
			}
			for i := range children {
				if slack[i] > 0 {
					d := take * slack[i] / totalSlack
					basisSizes[i] -= d
					basisTotal -= d
				}
			}
			free = innerMain - basisTotal - gapTotal
			if free > -1e-9 && free < 0 {
				free = 0
			}
		}
	}
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
		// サブシステムが申告した上限(Max)を超えた分は、上限の無いほかの伸びる
		// 子へ grow の比で回す(1回だけ。回す先が無ければ空きのまま)。
		excess, restGrow := 0.0, 0.0
		capped := make([]bool, n)
		for i, c := range children {
			if c.Grow <= 0 {
				continue
			}
			if mx := subsystemMaxMain(c, isRow, crossSizes[i]); mx > 0 && mainSizes[i] > mx {
				excess += mainSizes[i] - mx
				mainSizes[i] = mx
				capped[i] = true
				continue
			}
			restGrow += c.Grow
		}
		if excess > 0 && restGrow > 0 {
			for i, c := range children {
				if c.Grow > 0 && !capped[i] {
					mainSizes[i] += excess * (c.Grow / restGrow)
				}
			}
		}
	case free < 0 && basisTotal > 0:
		// 縮めない印(noShrink)の子は basis のまま、足りない分は印の無い子が
		// basis の比で受け持つ。印の無い木では従来と同じ(shrinkTotal == basisTotal)。
		shrinkTotal := 0.0
		for i, c := range children {
			if !c.noShrink {
				shrinkTotal += basisSizes[i]
			}
		}
		for i, b := range basisSizes {
			if children[i].noShrink || shrinkTotal <= 0 {
				mainSizes[i] = b
				continue
			}
			v := b + free*(b/shrinkTotal)
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

// hasSubsystem は s の下にサブシステムの葉があるか。
func hasSubsystem(s *Spec) bool {
	if s.Kind == KindSubsystem {
		return s.Subsystem != nil
	}
	for _, c := range s.Children {
		if hasSubsystem(c) {
			return true
		}
	}
	return false
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
