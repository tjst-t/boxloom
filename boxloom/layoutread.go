// 配置の段の木を入力として読む(開発者・試験用。docs/layers.md の4章・docs/build.md)。
//
// `boxloom layout` が出した JSON(schema/layout.schema.json)を読み直し、エンジンで
// 配置して PPTX に書く。エンジンだけを試す・崩れた例を木のまま再現する、のための
// 口で、作者(AI)の入口ではない(`boxloom schema`・SKILL.md・作者向けの文書には
// 載せない。作者は意味(IR)だけを書く)。
//
// 読み方:
//   - 読むのはトークン(tokens・塊の role・sizeToken)。間隔・余白は scale:"space" の
//     名前か space(n)、幅・高さ・角は値の表の長さの名前か space(n)、字の大きさは
//     scale:"type" の名前か type(n)、色はテーマの役割名。外れたら誤り。
//   - 解決した数と色(gap・padding・fontSize・color・background・radius・width・
//     height・塊の color・size)は出力の控え。あってもよいが、トークンが無ければ
//     誤り(layout-raw)、トークンから引いた値と違えば誤り(layout-mismatch)。
//   - 座標(box)とサブシステムの申告(subsystem の width・natural…draw)は
//     エンジンの出力なので読まない(あっても誤りにしない。読み直しの往復のため)。
//     subsystem の natural・min・max・fixedAspect は、中身を持たない代わりの葉
//     (LayoutReadOptions.Deck が無いとき)の申告にだけ使う。
//   - grow・basis は比なのでただの数のまま(負なら誤り)。
//   - 文字の葉は色の役割名が要る(tokens.color か、すべての塊の role)。
//   - サブシステムの中身は木に無い。Deck を渡せば、その deck を同じテンプレートで
//     組んだ木のサブシステムを、スライドごとに出てきた順に当てる(名前が違えば
//     誤り)。発表者ノートと文書の題も Deck から取る。
//
// 誤りは Issue の形(path は読んだ JSON の中の場所。例
// /slides/0/root/children/1/tokens/gap)。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"strings"
)

// LayoutReadOptions は配置の段の木を読むときの指定。
type LayoutReadOptions struct {
	// Template はテンプレートの名前(空なら木の template)。Values があればそちら。
	Template string
	Values   *Template
	// Assets は Deck の画像を読む場所(BuildOptions.Assets と同じ)。
	Assets fs.FS
	// Deck はサブシステムの中身・発表者ノート・文書の題を取る deck(nil なら
	// サブシステムは申告だけの何も描かない葉)。
	Deck *IRDeck
}

// layoutTree は読んで値を引いた木(配置の前)。
type layoutTree struct {
	t      *Template
	slides []*BuiltSlide
	title  string
}

// BuildLayout は配置の段の木(`boxloom layout` の出力)を読み、エンジンで配置して
// PPTX を w に書く(開発者・試験用)。読めなければ MarkdownErrors(Issue の並び)を
// 返し、w には何も書かない。
func BuildLayout(w io.Writer, file string, src []byte, opt LayoutReadOptions) (*BuildResult, error) {
	defer useFonts(activeFonts)()
	tree, err := readLayoutTree(file, src, opt)
	if err != nil {
		return nil, err
	}
	t := tree.t
	v := t.Values
	var pages []*PPTXBuilder
	var warnings []string
	for i, b := range tree.slides {
		computeLayout(b.Root, 0, 0, v.PageW, v.PageH)
		p := NewPPTXBuilder(v.PageW, v.PageH)
		p.ColorTheme = &DeckColors{Name: t.Name, Colors: t.Theme}
		roles := newCheckedRoles(t.Theme)
		renderSlide(RenderContext{Out: p, Roles: roles}, b.Root)
		if missing := roles.Missing(); len(missing) > 0 {
			warnings = append(warnings, fmt.Sprintf("slide %d: [ROLE WARNING] theme has no value for roles: %v", i+1, missing))
		}
		p.SetNotes(b.Notes)
		pages = append(pages, p)
	}
	if err := WriteDeck(w, pages, tree.title); err != nil {
		return nil, err
	}
	return &BuildResult{Template: t.Name, Slides: len(pages), Warnings: warnings}, nil
}

// layoutReader は木を読むときの状態。
type layoutReader struct {
	file   string
	tt     *tokenTable
	issues []Issue
	// subs はスライドの中のサブシステム(Deck から。出てきた順)、next は次に当てる番。
	subs    []*Spec
	next    int
	useDeck bool
}

func (r *layoutReader) add(path, code, msg string, expected ...string) {
	r.issues = append(r.issues, Issue{File: r.file, Path: path, Code: code, Message: msg, Expected: expected})
}

func readLayoutTree(file string, src []byte, opt LayoutReadOptions) (*layoutTree, error) {
	r := &layoutReader{file: file}
	var dump LayoutDump
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&dump); err != nil {
		r.add("", "json", "not a layout tree: "+err.Error())
		return nil, MarkdownErrors(r.issues)
	}
	if dump.Format != LayoutFormat {
		r.add("/format", "layout-format", fmt.Sprintf("format %q is not %q", dump.Format, LayoutFormat), LayoutFormat)
		return nil, MarkdownErrors(r.issues)
	}
	t := opt.Values
	if t == nil {
		name := dump.Template
		if opt.Template != "" {
			name = opt.Template
		}
		var err error
		if t, err = LookupTemplate(name); err != nil {
			r.add("/template", "layout-template", err.Error(), TemplateNames...)
			return nil, MarkdownErrors(r.issues)
		}
	}
	if t.Fonts != nil {
		activeFonts = t.Fonts
	}
	v := t.Values
	if math.Abs(dump.Page.Width-v.PageW) > 1e-6 || math.Abs(dump.Page.Height-v.PageH) > 1e-6 {
		r.add("/page", "layout-mismatch", fmt.Sprintf("page %vx%v is not the template's %vx%v", dump.Page.Width, dump.Page.Height, v.PageW, v.PageH))
	}
	r.tt = newTokenTable(t)

	// Deck を同じテンプレートで組み、サブシステム・ノート・題を取る。
	var deckSlides []*BuiltSlide
	title := ""
	if opt.Deck != nil {
		t.SetAssets(opt.Assets)
		built, err := t.BuildDeck(opt.Deck)
		if err != nil {
			return nil, err
		}
		if len(built) != len(dump.Slides) {
			r.add("/slides", "layout-deck", fmt.Sprintf("the tree has %d slides, the deck %d", len(dump.Slides), len(built)))
			return nil, MarkdownErrors(r.issues)
		}
		deckSlides = built
		if opt.Deck.Meta != nil {
			title = opt.Deck.Meta.Title.Plain()
		}
	}
	if len(dump.Slides) == 0 {
		r.add("/slides", "layout-slides", "the tree has no slides")
	}

	r.useDeck = deckSlides != nil
	out := &layoutTree{t: t, title: title}
	for i, sl := range dump.Slides {
		at := fmt.Sprintf("/slides/%d", i)
		r.subs, r.next = nil, 0
		b := &BuiltSlide{}
		if deckSlides != nil {
			collectSubsystems(deckSlides[i].Root, &r.subs)
			b.Notes = deckSlides[i].Notes
		}
		if sl.Root == nil {
			r.add(at+"/root", "layout-node", "the slide has no root")
			continue
		}
		b.Root = r.node(sl.Root, at+"/root")
		if deckSlides != nil && r.next != len(r.subs) {
			r.add(at, "layout-deck", fmt.Sprintf("the tree has %d subsystem leaves, the deck's slide %d", r.next, len(r.subs)))
		}
		out.slides = append(out.slides, b)
	}
	if len(r.issues) > 0 {
		return nil, MarkdownErrors(r.issues)
	}
	// 検査を通った木は、組み立てと同じ1か所(resolveSpec)で数と色に直す。
	for i, b := range out.slides {
		if err := resolveSpec(t, b.Root); err != nil {
			return nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
	}
	return out, nil
}

// collectSubsystems はサブシステムの葉を描く順(木の前順)に集める。
func collectSubsystems(s *Spec, out *[]*Spec) {
	if s.Kind == KindSubsystem {
		*out = append(*out, s)
	}
	for _, c := range s.Children {
		collectSubsystems(c, out)
	}
}

var (
	layoutKindOf    = map[string]Kind{"row": KindRow, "col": KindCol, "text": KindText, "subsystem": KindSubsystem}
	layoutTokenKeys = []string{"gap", "padding", "paddingY", "width", "height", "fontSize", "color", "background", "radius"}
)

func layoutSame(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// node は n を Spec にする(トークンだけを持たせ、数と色は resolveSpec が引く)。
// トークンを値の表で引いて検査し、控えの値と食い違えば誤りを足す。
func (r *layoutReader) node(n *LayoutNode, at string) *Spec {
	kind, ok := layoutKindOf[n.Kind]
	if !ok {
		r.add(at+"/kind", "layout-node", fmt.Sprintf("unknown kind %q", n.Kind), "row", "col", "text", "subsystem")
		return &Spec{Kind: KindCol}
	}
	s := &Spec{Kind: kind}
	if kind == KindText {
		s.Align, s.Anchor = "left", "top"
	}
	tok := n.Tokens
	for k := range tok {
		if !containsString(layoutTokenKeys, k) {
			r.add(at+"/tokens/"+k, "layout-token", fmt.Sprintf("unknown token key %q", k), layoutTokenKeys...)
		}
	}
	tokAt := func(k string) string { return at + "/tokens/" + k }
	// check は控えの値 has/got と、トークン name を resolve で引いた値を比べる。
	check := func(field, key string, has bool, got float64, resolve func(string) (float64, error)) (string, float64) {
		name := tok[key]
		if name == "" {
			if has {
				r.add(at+"/"+field, "layout-raw", fmt.Sprintf("%s %v is written without a token (write a value-table name or a scale step in tokens.%s)", field, got, key))
			}
			return "", 0
		}
		x, err := resolve(name)
		if err != nil {
			r.add(tokAt(key), "layout-token", err.Error())
			return name, 0
		}
		if has && !layoutSame(x, got) {
			r.add(at+"/"+field, "layout-mismatch", fmt.Sprintf("%s %v differs from %v given by the token %q", field, got, x, name))
		}
		return name, x
	}
	checkColor := func(field, key, got string) string {
		name := tok[key]
		if name == "" {
			if got != "" {
				r.add(at+"/"+field, "layout-raw", fmt.Sprintf("%s %s is written without a role name in tokens.%s", field, got, key))
			}
			return ""
		}
		c, err := r.tt.color(name)
		if err != nil {
			r.add(tokAt(key), "layout-token", err.Error())
			return name
		}
		if got != "" && !strings.EqualFold(got, c) {
			r.add(at+"/"+field, "layout-mismatch", fmt.Sprintf("%s %s differs from %s given by the role %q", field, got, c, name))
		}
		return name
	}

	s.tok.gap, _ = check("gap", "gap", n.Gap != 0, n.Gap, r.tt.space)
	var px, py float64
	hasPad := n.Padding != nil
	if hasPad {
		px, py = n.Padding.X, n.Padding.Y
	}
	var xv float64
	s.tok.padX, xv = check("padding/x", "padding", hasPad && px != 0, px, r.tt.space)
	if tok["paddingY"] != "" {
		s.tok.padY, _ = check("padding/y", "paddingY", hasPad, py, r.tt.space)
	} else if s.tok.padX != "" && hasPad && !layoutSame(py, xv) {
		r.add(at+"/padding/y", "layout-mismatch", fmt.Sprintf("padding y %v differs from %v given by the token %q (write tokens.paddingY)", py, xv, s.tok.padX))
	} else if s.tok.padX == "" && hasPad && py != 0 {
		r.add(at+"/padding/y", "layout-raw", fmt.Sprintf("padding y %v is written without a token (write tokens.paddingY)", py))
	}
	var font float64
	s.tok.fontSize, font = check("fontSize", "fontSize", n.FontSize != 0, n.FontSize, r.tt.typeSize)
	if kind == KindText && s.tok.fontSize == "" && tok["fontSize"] == "" {
		r.add(at+"/tokens", "layout-raw", "a text leaf needs a type size token (tokens.fontSize)")
	}
	hasFont := s.tok.fontSize != ""
	length := func(name string) (float64, error) { return r.tt.length(name, font, hasFont) }
	opt := func(p *float64) (bool, float64) {
		if p == nil {
			return false, 0
		}
		return true, *p
	}
	has, x := opt(n.Width)
	s.tok.width, _ = check("width", "width", has, x, length)
	has, x = opt(n.Height)
	s.tok.height, _ = check("height", "height", has, x, length)
	has, x = opt(n.Radius)
	s.tok.radius, _ = check("radius", "radius", has, x, length)
	s.tok.background = checkColor("background", "background", n.Background)
	// 色はどの箱でもトークン(役割名)だけ。row・col の生の色も捨てずに誤りにする。
	s.tok.color = checkColor("color", "color", n.Color)

	if n.Grow < 0 {
		r.add(at+"/grow", "layout-node", fmt.Sprintf("grow %v is negative", n.Grow))
	}
	s.Grow = n.Grow
	if n.Basis != nil {
		b := *n.Basis
		if b < 0 {
			r.add(at+"/basis", "layout-node", fmt.Sprintf("basis %v is negative", b))
		}
		s.Basis = &b
	}
	switch n.AlignItems {
	case "":
	case "center":
		s.AlignItems = AlignCenter
	default:
		r.add(at+"/alignItems", "layout-node", fmt.Sprintf("unknown alignItems %q", n.AlignItems), "center")
	}
	switch n.JustifyContent {
	case "":
	case "spaceBetween":
		s.JustifyContent = JustifySpaceBetween
	default:
		r.add(at+"/justifyContent", "layout-node", fmt.Sprintf("unknown justifyContent %q", n.JustifyContent), "spaceBetween")
	}
	if m := n.Marks; m != nil {
		// 印は語彙(layout.go の SpecOpt)で付ける。並列のまとまり・対の並びの
		// 部品(columnGroups・pairRows)は木から作り直さない(警告を測らないため)。
		var opts []SpecOpt
		if m.IRType != "" {
			opts = append(opts, markIRType(m.IRType))
		}
		if m.CardRow != "" {
			opts = append(opts, measureCardRow(m.CardRow))
		}
		if m.KeepLine != "" {
			opts = append(opts, keepLine(m.KeepLine))
		}
		if m.NestedSlack {
			opts = append(opts, nestedSlack())
		}
		if m.NoShrink {
			opts = append(opts, noShrink())
		}
		if m.List {
			opts = append(opts, asList())
		}
		if m.Card {
			opts = append(opts, asCard())
		}
		apply(s, opts)
	}

	switch kind {
	case KindText:
		r.text(n, s, at)
	case KindSubsystem:
		r.subsystem(n, s, at)
	default:
		if n.Text != nil || len(n.Runs) > 0 || n.Subsystem != nil {
			r.add(at, "layout-node", fmt.Sprintf("a %s box has no text or subsystem", n.Kind))
		}
	}
	if kind != KindText {
		if n.TextAlign != "" || n.Anchor != "" || n.Placeholder != "" || n.Bold {
			r.add(at, "layout-node", fmt.Sprintf("textAlign, anchor, placeholder and bold belong to text leaves, not %s", n.Kind))
		}
	}
	if (kind == KindText || kind == KindSubsystem) && len(n.Children) > 0 {
		r.add(at+"/children", "layout-node", fmt.Sprintf("a %s leaf has no children", n.Kind))
	}
	for i, c := range n.Children {
		if c == nil {
			r.add(fmt.Sprintf("%s/children/%d", at, i), "layout-node", "null box")
			continue
		}
		s.Children = append(s.Children, r.node(c, fmt.Sprintf("%s/children/%d", at, i)))
	}
	return s
}

func (r *layoutReader) text(n *LayoutNode, s *Spec, at string) {
	if n.Subsystem != nil {
		r.add(at+"/subsystem", "layout-node", "a text leaf has no subsystem")
	}
	s.Bold = n.Bold
	switch n.TextAlign {
	case "", "left":
	case "center", "right":
		s.Align = n.TextAlign
	default:
		r.add(at+"/textAlign", "layout-node", fmt.Sprintf("unknown textAlign %q", n.TextAlign), "center", "right")
	}
	switch n.Anchor {
	case "", "top":
	case "middle", "bottom":
		s.Anchor = n.Anchor
	default:
		r.add(at+"/anchor", "layout-node", fmt.Sprintf("unknown anchor %q", n.Anchor), "middle", "bottom")
	}
	switch n.Placeholder {
	case "", "title", "ctrTitle", "subTitle", "ftr", "sldNum":
		s.Placeholder = n.Placeholder
	default:
		r.add(at+"/placeholder", "layout-node", fmt.Sprintf("unknown placeholder %q", n.Placeholder), "title", "ctrTitle", "subTitle", "ftr", "sldNum")
	}
	if len(n.Runs) == 0 {
		if n.Text == nil {
			r.add(at, "layout-node", "a text leaf needs text or runs")
		} else {
			s.Text = *n.Text
		}
		r.needColor(n, s, at)
		return
	}
	for i, lr := range n.Runs {
		ra := fmt.Sprintf("%s/runs/%d", at, i)
		run := TextRun{Text: lr.Text, Bold: lr.Bold, Mark: lr.Mark, Keep: lr.Keep, Phrase: lr.Phrase, Field: lr.Field}
		switch {
		case lr.Role != "":
			c, err := r.tt.color(lr.Role)
			if err != nil {
				r.add(ra+"/role", "layout-token", err.Error())
			} else if lr.Color != "" && !strings.EqualFold(lr.Color, c) {
				r.add(ra+"/color", "layout-mismatch", fmt.Sprintf("color %s differs from %s given by the role %q", lr.Color, c, lr.Role))
			}
			run.role = lr.Role
		case lr.Color != "":
			r.add(ra+"/color", "layout-raw", fmt.Sprintf("color %s is written without a role name (role)", lr.Color))
		}
		switch {
		case lr.SizeToken != "":
			x, err := r.tt.typeSize(lr.SizeToken)
			if err != nil {
				r.add(ra+"/sizeToken", "layout-token", err.Error())
			} else if lr.Size != 0 && !layoutSame(lr.Size, x) {
				r.add(ra+"/size", "layout-mismatch", fmt.Sprintf("size %v differs from %v given by the token %q", lr.Size, x, lr.SizeToken))
			}
			run.sizeTok = lr.SizeToken
		case lr.Size != 0:
			r.add(ra+"/size", "layout-raw", fmt.Sprintf("size %v is written without a token (sizeToken)", lr.Size))
		}
		switch {
		case lr.WidthToken != "":
			if _, ok := r.tt.ems[lr.WidthToken]; !ok {
				r.add(ra+"/widthToken", "layout-token", fmt.Sprintf("%q is not an em name of the value table (unit \"em\")", lr.WidthToken))
			}
			run.widthTok = lr.WidthToken
		case lr.Width != 0:
			r.add(ra+"/width", "layout-raw", fmt.Sprintf("width %v is written without a token (widthToken)", lr.Width))
		}
		if lr.AutoNum != "" {
			if _, ok := orderedStyles[lr.AutoNum]; !ok {
				r.add(ra+"/autoNum", "layout-token", fmt.Sprintf("unknown numbering style %q", lr.AutoNum), sortedKeys(orderedStyles)...)
			}
			run.AutoNum = lr.AutoNum
		}
		s.Runs = append(s.Runs, run)
	}
	s.Text = runsText(s.Runs)
	r.needColor(n, s, at)
	if n.Text != nil && *n.Text != s.Text {
		r.add(at+"/text", "layout-mismatch", "text differs from the runs joined (the runs are read)")
	}
}

// needColor は文字の葉の色の必須(resolveSpec と同じ:葉の color トークンか、すべての
// 塊の役割名)を、読み直しのときに Issue として掛ける。生の色を書いた葉は checkColor が
// layout-raw にしているので重ねない。
func (r *layoutReader) needColor(n *LayoutNode, s *Spec, at string) {
	if n.Tokens["color"] != "" || n.Color != "" || allRunsHaveRole(s.Runs) {
		return
	}
	r.add(at+"/tokens", "layout-raw", "a text leaf needs a color role (tokens.color, or a role on every run)")
}

func (r *layoutReader) subsystem(n *LayoutNode, s *Spec, at string) {
	if n.Text != nil || len(n.Runs) > 0 {
		r.add(at, "layout-node", "a subsystem leaf has no text or runs")
	}
	if n.Subsystem == nil {
		r.add(at, "layout-node", "a subsystem leaf needs subsystem (name and measurement)")
		return
	}
	ls := n.Subsystem
	if r.useDeck {
		if r.next >= len(r.subs) {
			r.next++
			return
		}
		from := r.subs[r.next]
		r.next++
		if from.Subsystem == nil || from.Subsystem.Name() != ls.Name {
			got := ""
			if from.Subsystem != nil {
				got = from.Subsystem.Name()
			}
			r.add(at+"/subsystem/name", "layout-deck", fmt.Sprintf("the tree has subsystem %q here, the deck %q", ls.Name, got))
			return
		}
		s.Subsystem, s.Place = from.Subsystem, from.Place
		return
	}
	s.Subsystem = &stubSubsystem{name: ls.Name, m: Measurement{
		Natural: Size{ls.Natural.Width, ls.Natural.Height}, Min: Size{ls.Min.Width, ls.Min.Height},
		Max: Size{ls.Max.Width, ls.Max.Height}, FixedAspect: ls.FixedAspect}}
}

// stubSubsystem は中身を持たないサブシステムの葉:木に書いた申告を返し、何も描かない
// (Deck を渡さずに木を読んだとき。エンジンだけを試す)。縦横比を保つ図は、
// 分かっている軸から自然な大きさを比で決める。
type stubSubsystem struct {
	name string
	m    Measurement
}

func (s *stubSubsystem) Name() string { return s.name }

func (s *stubSubsystem) Measure(c Constraint) Measurement {
	m := s.m
	if m.FixedAspect && m.Natural.Width > 0 && m.Natural.Height > 0 {
		switch {
		case c.Width > 0:
			m.Natural = Size{c.Width, c.Width * s.m.Natural.Height / s.m.Natural.Width}
		case c.Height > 0:
			m.Natural = Size{c.Height * s.m.Natural.Width / s.m.Natural.Height, c.Height}
		}
	}
	return m
}

func (s *stubSubsystem) Render(RenderContext, LBox) {}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
