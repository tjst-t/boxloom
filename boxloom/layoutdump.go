// 配置の段の木を外に出す(`boxloom layout`。docs/layers.md の4章・docs/build.md)。
//
// 部品が組んだ箱の木(Spec)を、エンジンが決めた座標(box)と一緒に JSON にする。
// なぜその大きさ・はみ出しになったかを人と AI が読むためのもの。形は
// schema/layout.schema.json。
//
// 座標は build と同じ配置(computeLayout の直後。PPTX はこの box をそのまま
// a:off / a:ext に書く)。サブシステムは名前と申告した大きさだけで、中身は出さない。
// 間隔・余白・幅・高さ・角・字の大きさ・色は、解決した数と色に加えて、部品が
// 書いたトークン(値の表の名前・段の刻み・役割名。token.go)を tokens に出す。
// この形を開発者・試験用に読み直す口は layoutread.go(作者の入口ではない)。

package boxloom

import (
	"fmt"
	"math"
	"strings"
)

// LayoutFormat は layout の出力の形の名前と版(LayoutDump.Format)。
const LayoutFormat = "boxloom-layout/0.1"

// LayoutDump は文書の全スライドの配置の段の木。
type LayoutDump struct {
	Format   string        `json:"format"`
	File     string        `json:"file,omitempty"`
	Template string        `json:"template"`
	Page     LayoutSize    `json:"page"`
	Slides   []LayoutSlide `json:"slides"`
}

// LayoutSize は大きさ(pt)。
type LayoutSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// LayoutBox は確定した矩形(pt。ページの左上から)。
type LayoutBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// LayoutSlide は1枚のスライドの木。
type LayoutSlide struct {
	Path string      `json:"path"` // /slides/S
	Line int         `json:"line,omitempty"`
	Type string      `json:"type"` // IR のスライドの type
	Root *LayoutNode `json:"root"`
}

// LayoutPadding は箱の内側の余白(x は左右、y は上下)。
type LayoutPadding struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// LayoutRun は装飾つきの文字の塊。
type LayoutRun struct {
	Text  string  `json:"text"`
	Color string  `json:"color,omitempty"`
	Bold  bool    `json:"bold,omitempty"`
	Size  float64 `json:"size,omitempty"` // 0 なら葉の fontSize
	Mark  bool    `json:"mark,omitempty"` // 箇条などの頭の印
	Keep  bool    `json:"keep,omitempty"` // 塊の中で折り返さない
	// Phrase は折るところを文の切れ目・読点に寄せる(リード文など)。
	Phrase bool   `json:"phrase,omitempty"`
	Field  string `json:"field,omitempty"` // アプリが値を入れる欄(slidenum)
	// Role は色の役割名、SizeToken は字の大きさのトークン(部品が書いたもの)。
	Role      string `json:"role,omitempty"`
	SizeToken string `json:"sizeToken,omitempty"`
}

// LayoutMarks は計測と書き出しの印(docs/layers.md の2.1)。
type LayoutMarks struct {
	IRType       string `json:"irType,omitempty"`
	CardRow      string `json:"cardRow,omitempty"`      // カードの並びの IR の名前(options・items)
	ColumnGroups int    `json:"columnGroups,omitempty"` // 並列のまとまりの数
	PairRows     int    `json:"pairRows,omitempty"`     // 対の並びの対の数
	KeepLine     string `json:"keepLine,omitempty"`
	NestedSlack  bool   `json:"nestedSlack,omitempty"`
	List         bool   `json:"list,omitempty"`
	Card         bool   `json:"card,omitempty"`
}

// LayoutSubsystem はサブシステムの葉の、名前と申告した大きさ(中身は出さない)。
type LayoutSubsystem struct {
	Name string `json:"name"`
	// Width は申告を尋ねた幅(割り当てられた幅)。
	Width       float64    `json:"width"`
	Natural     LayoutSize `json:"natural"`
	Min         LayoutSize `json:"min"`
	Max         LayoutSize `json:"max"` // 0 は上限なし
	FixedAspect bool       `json:"fixedAspect,omitempty"`
	Draw        LayoutBox  `json:"draw"` // 描いた矩形(縦横比を保つ図は box より小さいことがある)
}

// LayoutNode は箱の木の1つの箱。
type LayoutNode struct {
	Kind string `json:"kind"` // row・col・text・subsystem
	// Path は箱を作った IR の場所(無ければ一番近い親の場所)。
	Path string    `json:"path"`
	Line int       `json:"line,omitempty"`
	Box  LayoutBox `json:"box"`

	Gap            float64        `json:"gap,omitempty"`
	Padding        *LayoutPadding `json:"padding,omitempty"`
	Grow           float64        `json:"grow,omitempty"`
	Basis          *float64       `json:"basis,omitempty"`
	Width          *float64       `json:"width,omitempty"`
	Height         *float64       `json:"height,omitempty"`
	AlignItems     string         `json:"alignItems,omitempty"`     // center(既定の stretch は省く)
	JustifyContent string         `json:"justifyContent,omitempty"` // spaceBetween(既定の start は省く)
	Background     string         `json:"background,omitempty"`
	Radius         *float64       `json:"radius,omitempty"`

	Text        *string     `json:"text,omitempty"`
	Runs        []LayoutRun `json:"runs,omitempty"`
	FontSize    float64     `json:"fontSize,omitempty"`
	Color       string      `json:"color,omitempty"`
	Bold        bool        `json:"bold,omitempty"`
	TextAlign   string      `json:"textAlign,omitempty"` // left 以外のとき
	Anchor      string      `json:"anchor,omitempty"`    // top 以外のとき
	Placeholder string      `json:"placeholder,omitempty"`

	Marks     *LayoutMarks     `json:"marks,omitempty"`
	Subsystem *LayoutSubsystem `json:"subsystem,omitempty"`
	// Tokens は部品が書いたトークン(値の表の名前・space(n)・type(n)・役割名)。
	// キーは gap・padding・paddingY・width・height・fontSize・color・background・radius。
	Tokens   map[string]string `json:"tokens,omitempty"`
	Children []*LayoutNode     `json:"children,omitempty"`
}

// lineMap は IR の場所 → 元の文書の行(Markdown の SourceMap・YAML の YAMLSourceMap)。
type lineMap interface{ Line(path string) int }

// noLines は IR(JSON)を読んだとき(行を添えない)。
type noLines struct{}

func (noLines) Line(string) int { return 0 }

// Layout は文書を組み、スライドごとの配置の段の木を返す。file の拡張子が .md なら
// Boxloom Markdown、.yaml・.yml なら YAML の deck(記法の誤りはどちらも
// MarkdownErrors。箱に元の文書の行を添える)、それ以外は IR(JSON)。テンプレートの選び方と画像の読み方は Build と同じ。
func Layout(file string, src []byte, opt BuildOptions) (*LayoutDump, error) {
	var deck *IRDeck
	var lines lineMap = noLines{}
	if isYAMLPath(file) {
		// 読むのは ReadDeck(IR の誤りにも YAML の行と列を添える)。行の表だけここで取る。
		d, err := ReadDeck(file, src)
		if err != nil {
			return nil, err
		}
		_, _, m := parseYAMLDeck(file, src)
		deck, lines = d, m
	} else if isMarkdownPath(file) {
		ir, issues, m := ParseMarkdownWithMap(src, file)
		if len(issues) > 0 {
			return nil, MarkdownErrors(issues)
		}
		d, err := ReadIR(file, ir)
		if err != nil {
			return nil, err
		}
		deck, lines = d, m
	} else {
		d, err := ReadIR(file, src)
		if err != nil {
			return nil, err
		}
		deck = d
	}
	return layoutDeck(deck, opt, lines)
}

// layoutDeck は読んだ deck の配置の段の木。lines で箱に元の文書の行を添える。
func layoutDeck(deck *IRDeck, opt BuildOptions, lines lineMap) (*LayoutDump, error) {
	defer useFonts(activeFonts)()
	t := opt.Values
	if t == nil {
		name := deck.Template
		if opt.Template != "" {
			name = opt.Template
		}
		var err error
		if t, err = LookupTemplate(name); err != nil {
			return nil, err
		}
	}
	t.SetAssets(opt.Assets)
	built, err := t.BuildDeck(deck)
	if err != nil {
		return nil, err
	}
	v := t.Values
	out := &LayoutDump{Format: LayoutFormat, Template: t.Name, Page: LayoutSize{v.PageW, v.PageH}, Slides: []LayoutSlide{}}
	for i, b := range built {
		computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
		sp := fmt.Sprintf("/slides/%d", i)
		d := &layoutDumper{slide: sp, lines: lines, body: b.Body, groups: map[*Spec]columnInfo{}, pairParts: map[*Spec]string{}}
		out.Slides = append(out.Slides, LayoutSlide{Path: sp, Line: lines.Line(sp), Type: deck.Slides[i].Type, Root: d.node(b.Root, sp)})
	}
	return out, nil
}

type layoutDumper struct {
	slide string
	lines lineMap
	body  *Spec
	// groups は並列のまとまりの箱(まとまり全体・本文の列)→ そのまとまり。
	groups map[*Spec]columnInfo
	// pairParts は対の並びの中の箱 → "list"(対の行の列)か "row"(見出しの行・対の行)。
	pairParts map[*Spec]string
}

// round2 は 0.01pt に丸める(読みやすさのため。PPTX の EMU とは 127 未満の差)。
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func layoutBoxOf(b LBox) LayoutBox {
	return LayoutBox{round2(b.X), round2(b.Y), round2(b.Width), round2(b.Height)}
}

var layoutKinds = map[Kind]string{KindRow: "row", KindCol: "col", KindText: "text", KindSubsystem: "subsystem"}

// node は s を出す。path は親から決まった IR の場所(s.irPath があればそちら)。
func (d *layoutDumper) node(s *Spec, path string) *LayoutNode {
	if s.irPath != "" {
		if strings.HasPrefix(s.irPath, "/") {
			path = s.irPath
		} else {
			path = d.slide + "/" + s.irPath
		}
	}
	if s == d.body {
		path = d.slide + "/body"
	}
	n := &LayoutNode{Kind: layoutKinds[s.Kind], Path: path, Line: d.lines.Line(path), Box: layoutBoxOf(s.Box)}
	tok := map[string]string{}
	addTok := func(key, name string) {
		if name != "" {
			tok[key] = name
		}
	}
	k := s.tok
	n.Gap = s.Gap
	addTok("gap", k.gap)
	if s.Padding != 0 || s.padY() != 0 {
		n.Padding = &LayoutPadding{X: s.padX(), Y: s.padY()}
	}
	addTok("padding", k.padX)
	if k.padY != k.padX {
		addTok("paddingY", k.padY)
	}
	addTok("width", k.width)
	addTok("height", k.height)
	// 字の大きさ(文字の葉。em の名前で幅を決めた空きの箱も持つ)。
	addTok("fontSize", k.fontSize)
	n.Grow, n.Basis, n.Width, n.Height = s.Grow, s.Basis, s.Width, s.Height
	if s.AlignItems == AlignCenter {
		n.AlignItems = "center"
	}
	if s.JustifyContent == JustifySpaceBetween {
		n.JustifyContent = "spaceBetween"
	}
	if s.Background != "" {
		n.Background = s.Background
	}
	addTok("background", k.background)
	addTok("radius", k.radius)
	if s.HasRadius {
		r := s.Radius
		n.Radius = &r
	}
	if s.Kind == KindText {
		txt := s.Text
		n.Text = &txt
		n.FontSize = s.FontSize
		if len(s.Runs) > 0 {
			for _, r := range s.Runs {
				n.Runs = append(n.Runs, LayoutRun{Text: r.Text, Color: r.Color, Bold: r.Bold, Size: r.Size, Mark: r.Mark, Keep: r.Keep, Phrase: r.Phrase, Field: r.Field, Role: r.role, SizeToken: r.sizeTok})
			}
		} else {
			n.Color, n.Bold = s.Color, s.Bold
		}
		addTok("color", k.color)
		if s.Align != "" && s.Align != "left" {
			n.TextAlign = s.Align
		}
		if s.Anchor != "" && s.Anchor != "top" {
			n.Anchor = s.Anchor
		}
		n.Placeholder = s.Placeholder
	}
	m := LayoutMarks{IRType: s.irType, CardRow: s.cards, KeepLine: s.keepLine, NestedSlack: s.nestedSlack, List: s.List, Card: s.card}
	if s.columns != nil {
		m.ColumnGroups = len(s.columns.cols)
	}
	if s.pairs != nil {
		m.PairRows = len(s.pairs.rows)
	}
	if m != (LayoutMarks{}) {
		n.Marks = &m
	}
	if s.Kind == KindSubsystem && s.Subsystem != nil {
		ms := s.Subsystem.Measure(Constraint{Width: s.Box.Width})
		size := func(z Size) LayoutSize { return LayoutSize{round2(z.Width), round2(z.Height)} }
		n.Subsystem = &LayoutSubsystem{Name: s.Subsystem.Name(), Width: round2(s.Box.Width),
			Natural: size(ms.Natural), Min: size(ms.Min), Max: size(ms.Max), FixedAspect: ms.FixedAspect,
			Draw: layoutBoxOf(subsystemDrawBox(s))}
	}
	if len(tok) > 0 {
		n.Tokens = tok
	}
	for i, c := range s.Children {
		n.Children = append(n.Children, d.node(c, d.childPath(s, i, c, path)))
	}
	return n
}

// childPath は子の IR の場所。本文のブロック(body/N)・カードの並び(options/K・
// items/K)・並列のまとまり(columns/C、その題と本文のブロック)は木の形と印から
// 決まる。ほかは親の場所を継ぐ。
func (d *layoutDumper) childPath(s *Spec, i int, c *Spec, path string) string {
	if s == d.body {
		return fmt.Sprintf("%s/body/%d", d.slide, i)
	}
	if s.cards != "" {
		return fmt.Sprintf("%s/%s/%d", path, s.cards, i)
	}
	if s.columns != nil {
		for ci, col := range s.columns.cols {
			d.groups[col.spec], d.groups[col.body] = col, col
			if col.spec == c {
				return fmt.Sprintf("%s/columns/%d", path, ci)
			}
		}
	}
	if s.pairs != nil {
		d.pairParts[s.pairs.list], d.pairParts[s.pairs.head] = "list", "row"
		for _, r := range s.pairs.rows {
			d.pairParts[r.row] = "row"
		}
		if c == s.pairs.head {
			return path + "/headings"
		}
		return path
	}
	switch d.pairParts[s] {
	case "list":
		return fmt.Sprintf("%s/pairs/%d", path, i)
	case "row":
		if i == 0 {
			return path + "/subject"
		}
		return path + "/response"
	}
	if col, ok := d.groups[s]; ok {
		switch {
		case s == col.body:
			return fmt.Sprintf("%s/body/%d", path, i)
		case c == col.title && c != nil:
			return path + "/title"
		}
	}
	return path
}
