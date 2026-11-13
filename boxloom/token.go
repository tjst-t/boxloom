// 配置の段のトークン(docs/layers.md の2.1・8.3)。
//
// 部品とテンプレートの骨組みは、箱の木(Spec)に数と色を書かず、値の表の名前
// (blockGap・cardPadding・body・accent など)か段の刻み(space(n)・type(n))を
// 書く。組み立ての直後・計測の前に resolveSpec が1か所で値の表を引いて数と色に
// 直す。計測・配置・PPTX の書き出しは解決した数を読むので、エンジンは変わらない。
//
// 書けるもの:
//   - 間隔・余白(gap・padding):値の表の scale:"space" の名前か space(n)
//     (spacingUnit の n 倍、n は 1 以上の整数)
//   - 幅・高さ・角(width・height・radius):値の表の長さ(pt)の名前か space(n)。
//     em の名前(pointIndentEm)は同じ箱の字の大きさに掛ける
//   - 字の大きさ(fontSize・塊の size):値の表の scale:"type" の名前か type(n)
//     (n は typeSizes の段)
//   - 色(color・background・塊の色):テーマの役割名だけ(16進は書けない)
//   - grow・basis は比なのでトークンにせず、ただの数のまま
//
// 段に無い刻み・知らない名前・役割名、トークンの無い数や色は解決のときに誤り。
// トークンはサブシステムの葉の外側(余白・置き方)で終わる。葉の中の寸法は
// テンプレートを選んだ時に Factory に、色は描く時に Roles で渡す(今のまま)。

package boxloom

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// specTokens は箱に書いたトークン(空なら書いていない)。
type specTokens struct {
	gap, padX, padY, radius, width, height, fontSize, color, background string
}

func tokGap(t string) SpecOpt { return func(s *Spec) { s.tok.gap = t } }

// tokPad は上下左右の余白。
func tokPad(t string) SpecOpt { return func(s *Spec) { s.tok.padX, s.tok.padY = t, "" } }

// tokPadXY は左右(x)と上下(y)で違う余白。
func tokPadXY(x, y string) SpecOpt { return func(s *Spec) { s.tok.padX, s.tok.padY = x, y } }

func tokRadius(t string) SpecOpt     { return func(s *Spec) { s.tok.radius = t } }
func tokWidth(t string) SpecOpt      { return func(s *Spec) { s.tok.width = t } }
func tokHeight(t string) SpecOpt     { return func(s *Spec) { s.tok.height = t } }
func tokSize(t string) SpecOpt       { return func(s *Spec) { s.tok.fontSize = t } }
func tokColor(t string) SpecOpt      { return func(s *Spec) { s.tok.color = t } }
func tokBackground(t string) SpecOpt { return func(s *Spec) { s.tok.background = t } }

// spaceStep は余白の段の刻み(spacingUnit の n 倍)。足す部品・テンプレートの
// 組み方の逃げ道(標準の部品は名前だけで書く)。
func spaceStep(n int) string { return fmt.Sprintf("space(%d)", n) }

// typeStep は字の大きさの段(typeSizes の中の値)。逃げ道は spaceStep と同じ。
func typeStep(pt float64) string {
	return fmt.Sprintf("type(%s)", strconv.FormatFloat(pt, 'g', -1, 64))
}

// roleRun は役割名の色の塊。
func roleRun(text, role string, bold bool) TextRun {
	return TextRun{Text: text, role: role, Bold: bold}
}

// --- 解決 ---

// tokenTable はテンプレートの値の表の名前 → 値。
type tokenTable struct {
	spaces  map[string]float64 // scale:"space"
	types   map[string]float64 // scale:"type"
	lengths map[string]float64 // そのほかの長さ(pt)
	ems     map[string]float64 // unit:"em"(字の大きさの倍)
	scale   TemplateScale
	theme   Theme
	// tmpl は表を作ったテンプレート、denseTT は dense の値の表(印 dense の箱の下で
	// 使う。density.go。初めて要るときに作る)。dense の表の denseTT は自分。
	tmpl    *Template
	denseTT *tokenTable
	// fitShrink はこの表が fit.shrink の段のものなら k(段の番号+1)。
	fitShrink int
}

func newTokenTable(t *Template) *tokenTable {
	tt := &tokenTable{spaces: map[string]float64{}, types: map[string]float64{}, lengths: map[string]float64{},
		ems: map[string]float64{}, scale: t.Scale, theme: t.Theme, tmpl: t}
	rv := reflect.ValueOf(t.Values)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Type.Kind() != reflect.Float64 {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		x := rv.Field(i).Float()
		switch {
		case f.Tag.Get("scale") == "space":
			tt.spaces[name] = x
		case f.Tag.Get("scale") == "type":
			tt.types[name] = x
		case f.Tag.Get("unit") == "em":
			tt.ems[name] = x
		case f.Tag.Get("unit") == "ratio":
		default:
			tt.lengths[name] = x
		}
	}
	return tt
}

var (
	spaceStepRe = regexp.MustCompile(`^space\(([0-9]+)\)$`)
	typeStepRe  = regexp.MustCompile(`^type\(([0-9]+(?:\.[0-9]+)?)\)$`)
	hexTokenRe  = regexp.MustCompile(`^#?[0-9A-Fa-f]{6}$`)
)

// step は space(n) の値(当たらなければ ok=false。n が段に無ければ誤り)。
func (tt *tokenTable) step(tok string) (float64, bool, error) {
	m := spaceStepRe.FindStringSubmatch(tok)
	if m == nil {
		return 0, false, nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, true, fmt.Errorf("%q: the step must be a whole number of at least 1 (spacingUnit %v)", tok, tt.scale.SpacingUnit)
	}
	return float64(n) * tt.scale.SpacingUnit, true, nil
}

func (tt *tokenTable) space(tok string) (float64, error) {
	if x, ok := tt.spaces[tok]; ok {
		return x, nil
	}
	if x, ok, err := tt.step(tok); ok {
		return x, err
	}
	return 0, fmt.Errorf("%q is neither a spacing name of the value table (scale \"space\") nor space(n)", tok)
}

// length は幅・高さ・角。em の名前は fontSize(この箱の字の大きさ)に掛ける。
func (tt *tokenTable) length(tok string, fontSize float64, hasFont bool) (float64, error) {
	if x, ok := tt.spaces[tok]; ok {
		return x, nil
	}
	if x, ok := tt.lengths[tok]; ok {
		return x, nil
	}
	if x, ok := tt.ems[tok]; ok {
		if !hasFont {
			return 0, fmt.Errorf("%q is relative to the font size, but the box has no fontSize token", tok)
		}
		return x * fontSize, nil
	}
	if x, ok, err := tt.step(tok); ok {
		return x, err
	}
	return 0, fmt.Errorf("%q is neither a length name of the value table nor space(n)", tok)
}

func (tt *tokenTable) typeSize(tok string) (float64, error) {
	if x, ok := tt.types[tok]; ok {
		return x, nil
	}
	if m := typeStepRe.FindStringSubmatch(tok); m != nil {
		x, _ := strconv.ParseFloat(m[1], 64)
		for _, s := range tt.scale.TypeSizes {
			if math.Abs(s-x) < 1e-9 {
				return s, nil
			}
		}
		return 0, fmt.Errorf("%q is not on the type scale %v", tok, tt.scale.TypeSizes)
	}
	return 0, fmt.Errorf("%q is neither a type size name of the value table (scale \"type\") nor type(n)", tok)
}

func (tt *tokenTable) color(tok string) (string, error) {
	if c, ok := tt.theme[tok]; ok {
		return c, nil
	}
	if hexTokenRe.MatchString(tok) {
		return "", fmt.Errorf("%q: colors are role names of the theme; hex colors cannot be written", tok)
	}
	return "", fmt.Errorf("%q is not a color role of the theme", tok)
}

// allRunsHaveRole は、塊があり、すべての塊が色の役割名を持つか。
func allRunsHaveRole(runs []TextRun) bool {
	for _, r := range runs {
		if r.role == "" {
			return false
		}
	}
	return len(runs) > 0
}

// resolveSpec は組み立てた木のトークンを値の表で数と色に直す(組み立ての直後・
// 計測の前の1か所)。トークンの無い数・色、段に無い刻み、知らない名前・役割名は誤り。
func resolveSpec(t *Template, root *Spec) error {
	tt := newTokenTable(t)
	return tt.resolve(root, "root")
}

func (tt *tokenTable) resolve(s *Spec, at string) error {
	if s.fitShrink > 0 && tt.fitShrink != s.fitShrink {
		// -fit で詰めた本文の箱:この下は fit.shrink の段の値の表で引く(fit_shrink.go)。
		sub := newTokenTable(tt.tmpl.fitShrunk(s.fitShrink - 1))
		sub.fitShrink, sub.denseTT = s.fitShrink, sub
		return sub.resolve(s, at)
	}
	if s.dense && tt.denseTT != tt {
		// 密なスライドの本文の箱:この下は dense の値の表で引く(density.go)。
		if tt.denseTT == nil {
			tt.denseTT = newTokenTable(tt.tmpl.densed())
			tt.denseTT.denseTT = tt.denseTT
		}
		return tt.denseTT.resolve(s, at)
	}
	fail := func(field string, err error) error { return fmt.Errorf("layout token at %s: %s: %w", at, field, err) }
	raw := func(field string, v any) error {
		return fmt.Errorf("layout token at %s: %s: %v is written without a token (write a value-table name or a scale step)", at, field, v)
	}
	k := &s.tok
	switch {
	case k.gap == "" && s.Gap != 0:
		return raw("gap", s.Gap)
	case k.padX == "" && s.Padding != 0:
		return raw("padding", s.Padding)
	case k.padY == "" && s.PaddingY != nil:
		return raw("paddingY", *s.PaddingY)
	case k.radius == "" && s.HasRadius:
		return raw("radius", s.Radius)
	case k.width == "" && s.Width != nil:
		return raw("width", *s.Width)
	case k.height == "" && s.Height != nil:
		return raw("height", *s.Height)
	case k.background == "" && s.Background != "":
		return raw("background", s.Background)
	case k.color == "" && s.Color != "":
		return raw("color", s.Color)
	case k.fontSize == "" && s.Kind == KindText:
		return fmt.Errorf("layout token at %s: fontSize: a text leaf needs a type size token", at)
	case k.color == "" && s.Kind == KindText && !allRunsHaveRole(s.Runs):
		// コアは文字の既定の色を持たない(色は役割名だけ)。
		return fmt.Errorf("layout token at %s: color: a text leaf needs a color role (a color token on the leaf, or a role on every run)", at)
	}
	var err error
	if k.gap != "" {
		if s.Gap, err = tt.space(k.gap); err != nil {
			return fail("gap", err)
		}
	}
	if k.padX != "" {
		if s.Padding, err = tt.space(k.padX); err != nil {
			return fail("padding", err)
		}
	}
	if k.padY != "" {
		y, err := tt.space(k.padY)
		if err != nil {
			return fail("paddingY", err)
		}
		s.PaddingY = &y
	}
	if k.fontSize != "" {
		if s.FontSize, err = tt.typeSize(k.fontSize); err != nil {
			return fail("fontSize", err)
		}
	}
	hasFont := k.fontSize != ""
	if k.radius != "" {
		if s.Radius, err = tt.length(k.radius, s.FontSize, hasFont); err != nil {
			return fail("radius", err)
		}
		s.HasRadius = true
	}
	if k.width != "" {
		w, err := tt.length(k.width, s.FontSize, hasFont)
		if err != nil {
			return fail("width", err)
		}
		s.Width = &w
	}
	if k.height != "" {
		h, err := tt.length(k.height, s.FontSize, hasFont)
		if err != nil {
			return fail("height", err)
		}
		s.Height = &h
	}
	if k.color != "" {
		if s.Color, err = tt.color(k.color); err != nil {
			return fail("color", err)
		}
	}
	if k.background != "" {
		if s.Background, err = tt.color(k.background); err != nil {
			return fail("background", err)
		}
	}
	for i := range s.Runs {
		r := &s.Runs[i]
		field := fmt.Sprintf("runs[%d]", i)
		switch {
		case r.role == "" && r.Color != "":
			return raw(field+".color", r.Color)
		case r.sizeTok == "" && r.Size != 0:
			return raw(field+".size", r.Size)
		case r.widthTok == "" && r.Width != 0:
			return raw(field+".width", r.Width)
		}
		if r.role != "" {
			if r.Color, err = tt.color(r.role); err != nil {
				return fail(field+".color", err)
			}
		} else {
			r.Color = s.Color // 役割名の無い塊は葉の色(葉の color トークンは上で必須にした)
		}
		if r.sizeTok != "" {
			if r.Size, err = tt.typeSize(r.sizeTok); err != nil {
				return fail(field+".size", err)
			}
		}
		if r.widthTok != "" {
			// 塊の幅は em の名前だけ(塊の字の大きさに掛ける)。
			x, ok := tt.ems[r.widthTok]
			if !ok {
				return fail(field+".width", fmt.Errorf("%q is not an em name of the value table (unit \"em\")", r.widthTok))
			}
			if !hasFont {
				return fail(field+".width", fmt.Errorf("%q is relative to the font size, but the leaf has no fontSize token", r.widthTok))
			}
			r.Width = x * runSize(*r, s.FontSize)
		}
	}
	for i, c := range s.Children {
		if err := tt.resolve(c, fmt.Sprintf("%s/children/%d", at, i)); err != nil {
			return err
		}
	}
	return nil
}
