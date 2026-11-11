// 字体と字体ごとの寸法(文字の幅)。
//
// 方針(docs/fonts.md):字体はテンプレートの値の表(templates/<名前>.json の
// fonts)で選ぶ。欧文の字体(latin)と和文の字体(eastAsian)を分けて持つ
// (PPTX のテーマも a:latin と a:ea を分けて持ち、PowerPoint は文字の種類で
// どちらの字体で描くかを決めるため)。測るときも文字ごとに同じ振り分けをする。
//
// 寸法は fonts/<slug>.json(tools/export_font_metrics.py が Pillow で測った
// 「1em あたりの送り幅」)。字体のファイルそのものは持たない。寸法の無い字体を
// テンプレートが選ぶと読み込みで誤り。

package boxloom

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed fonts/*.json
var fontFiles embed.FS

// FontMetrics は字体1つの寸法。幅は em(字の大きさ 1pt あたりの pt)。
type FontMetrics struct {
	Family   string     `json:"family"`
	Source   string     `json:"source"` // measured(実測)。推定の寸法は同梱しない
	License  string     `json:"license"`
	Japanese bool       `json:"japanese"` // 仮名と漢字を持つ(和文の字体に選べる)
	Note     string     `json:"note"`
	Ascent   float64    `json:"ascent"`
	Descent  float64    `json:"descent"`
	Regular  fontWidths `json:"regular"`
	Bold     fontWidths `json:"bold"`
}

type fontWidths struct {
	Default float64            `json:"default"` // 表に無い文字(漢字など)の幅
	Widths  map[string]float64 `json:"widths"`  // default と違う幅の文字だけ
	runes   map[rune]float64
}

func (w *fontWidths) of(ch rune) float64 {
	if v, ok := w.runes[ch]; ok {
		return v
	}
	return w.Default
}

func (w *fontWidths) index(at string) error {
	if !(w.Default > 0) {
		return fmt.Errorf("%s.default: must be greater than 0", at)
	}
	w.runes = make(map[rune]float64, len(w.Widths))
	for k, v := range w.Widths {
		r, n := utf8.DecodeRuneInString(k)
		if n != len(k) || r == utf8.RuneError {
			return fmt.Errorf("%s.widths: key %q is not one character", at, k)
		}
		if v < 0 {
			return fmt.Errorf("%s.widths[%q]: must not be negative", at, k)
		}
		w.runes[r] = v
	}
	return nil
}

// LineHeightEm は字体が自分で決める1行の高さ(ascent+descent、em)。行の
// 送りを固定しないとき、描く側(PowerPoint・LibreOffice)はおおよそこれを使う。
func (m *FontMetrics) LineHeightEm() float64 { return m.Ascent + m.Descent }

var (
	fontsOnce     sync.Once
	fontsByFamily map[string]*FontMetrics
	fontsErr      error
)

// loadFonts は同梱した寸法を全部読む(読めなければ壊れたバイナリ)。
func loadFonts() (map[string]*FontMetrics, error) {
	fontsOnce.Do(func() {
		fontsByFamily = map[string]*FontMetrics{}
		ents, _ := fontFiles.ReadDir("fonts")
		for _, e := range ents {
			file := path.Join("fonts", e.Name())
			data, err := fontFiles.ReadFile(file)
			if err != nil {
				fontsErr = err
				return
			}
			m, err := parseFontMetrics(data)
			if err != nil {
				fontsErr = fmt.Errorf("%s: %w", file, err)
				return
			}
			if _, dup := fontsByFamily[m.Family]; dup {
				fontsErr = fmt.Errorf("%s: family %q appears twice", file, m.Family)
				return
			}
			fontsByFamily[m.Family] = m
		}
	})
	return fontsByFamily, fontsErr
}

func parseFontMetrics(data []byte) (*FontMetrics, error) {
	var m FontMetrics
	if err := decodeStrict(data, &m); err != nil {
		return nil, err
	}
	if m.Family == "" {
		return nil, fmt.Errorf("family: must not be empty")
	}
	if m.Source != "measured" {
		return nil, fmt.Errorf("source: %q (only measured metrics are bundled)", m.Source)
	}
	if !(m.Ascent > 0) || !(m.Descent >= 0) {
		return nil, fmt.Errorf("ascent/descent: must be positive")
	}
	if err := m.Regular.index("regular"); err != nil {
		return nil, err
	}
	if err := m.Bold.index("bold"); err != nil {
		return nil, err
	}
	return &m, nil
}

// FontFamilies は寸法を持つ字体の名前(並べたもの)。
func FontFamilies() []string {
	fs, err := loadFonts()
	if err != nil {
		panic(err)
	}
	var out []string
	for k := range fs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// LookupFont は字体の名前から寸法を引く。
func LookupFont(family string) (*FontMetrics, error) {
	fs, err := loadFonts()
	if err != nil {
		return nil, err
	}
	if m, ok := fs[family]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("no metrics for font %q (known: %s; add one with tools/export_font_metrics.py, see docs/fonts.md)",
		family, strings.Join(FontFamilies(), ", "))
}

// TemplateFonts は値の表の fonts(templates/<名前>.json)。
type TemplateFonts struct {
	Latin     string `json:"latin"`     // 欧文の字体(PPTX の a:latin)
	EastAsian string `json:"eastAsian"` // 和文の字体(PPTX の a:ea)
	// WidthMargin は測った幅に掛ける余裕(1.05 なら 5% 広めに見積もって
	// 折り返す)。字体が置き換わっても行が箱からはみ出さないためのもの。
	WidthMargin float64 `json:"widthMargin"`
	// FallbackFonts は置き換わり得る字体。測るときは文字ごとに、選んだ字体と
	// これらのうちいちばん広い幅を使う(和文を持たない字体は欧文の文字だけに
	// 効く)。空なら選んだ字体の幅だけで測る。PPTX には書かない。
	FallbackFonts []string `json:"fallbackFonts"`
	// FixedLineSpacing が true なら、PPTX の段落に行の送りを pt で書く
	// (字の大きさ×lineHeightMult)。字体が置き換わっても行の高さが測った
	// とおりになる。false なら字体まかせ(描く側の字体の ascent+descent)。
	FixedLineSpacing bool `json:"fixedLineSpacing"`
}

// FontSet は測るときと書き出すときに使う字体の一式。
type FontSet struct {
	Latin, EastAsian *FontMetrics
	Fallbacks        []*FontMetrics
	WidthMargin      float64
	FixedLineSpacing bool
}

// resolveFonts は値の表の fonts を検査して寸法を引く。
func resolveFonts(f TemplateFonts) (*FontSet, error) {
	lat, err := LookupFont(f.Latin)
	if err != nil {
		return nil, fmt.Errorf("fonts.latin: %w", err)
	}
	ea, err := LookupFont(f.EastAsian)
	if err != nil {
		return nil, fmt.Errorf("fonts.eastAsian: %w", err)
	}
	if !ea.Japanese {
		return nil, fmt.Errorf("fonts.eastAsian: %q has no Japanese glyphs (choose one of the fonts whose metrics say japanese: true)", f.EastAsian)
	}
	if !(f.WidthMargin >= 1 && f.WidthMargin <= 1.5) {
		return nil, fmt.Errorf("fonts.widthMargin: must be between 1 and 1.5 (got %v)", f.WidthMargin)
	}
	fs := &FontSet{Latin: lat, EastAsian: ea, WidthMargin: f.WidthMargin, FixedLineSpacing: f.FixedLineSpacing}
	seen := map[string]bool{}
	for i, fam := range f.FallbackFonts {
		if seen[fam] {
			return nil, fmt.Errorf("fonts.fallbackFonts[%d]: %q appears twice", i, fam)
		}
		seen[fam] = true
		m, err := LookupFont(fam)
		if err != nil {
			return nil, fmt.Errorf("fonts.fallbackFonts[%d]: %w", i, err)
		}
		fs.Fallbacks = append(fs.Fallbacks, m)
	}
	return fs, nil
}

// isEastAsianRune は PowerPoint が和文の字体(a:ea)で描く文字か。文の言語が
// ja-JP のとき、一般句読点(– “ ” …)・記号・CJK・全角形は和文の字体で描かれる。
// ラテン文字・ASCII(U+2000 より前)は欧文の字体。おおよその線で、細かな
// 例外(ギリシャ文字など)は docs/fonts.md。
func isEastAsianRune(ch rune) bool { return ch >= 0x2000 }

// emWidth は1文字の幅(em)。置き換わり得る字体(Fallbacks)があれば、その中で
// いちばん広い幅。
func (f *FontSet) emWidth(ch rune, bold bool) float64 {
	ea := isEastAsianRune(ch)
	m := f.Latin
	if ea {
		m = f.EastAsian
	}
	w := m.width(ch, bold)
	for _, fb := range f.Fallbacks {
		if ea && !fb.Japanese {
			continue // 和文を持たない字体は和文の文字を描かない
		}
		if v := fb.width(ch, bold); v > w {
			w = v
		}
	}
	return w
}

func (m *FontMetrics) width(ch rune, bold bool) float64 {
	if bold {
		return m.Bold.of(ch)
	}
	return m.Regular.of(ch)
}

// defaultFontSet は従来の既定(Noto Sans CJK JP、余裕 1.05、行の送りは字体まかせ)。
func defaultFontSet() *FontSet {
	noto, err := LookupFont("Noto Sans CJK JP")
	if err != nil {
		panic(err)
	}
	return &FontSet{Latin: noto, EastAsian: noto, WidthMargin: 1.05}
}

// activeFonts は文字を測るときに使う字体の一式。テンプレートが組み立てを
// 始めるとき(BuildDeck)に自分の一式に替える。文字を測る関数(text.go)は
// レイアウトの奥から呼ばれるので、引数で回さずここから引く。同時に2つの
// テンプレートで組むことはしない(コマンドは1回に1つの文書を作る)。
var activeFonts = defaultFontSet()

// useFonts は activeFonts を fs に替え、元に戻す関数を返す。
func useFonts(fs *FontSet) (restore func()) {
	prev := activeFonts
	activeFonts = fs
	return func() { activeFonts = prev }
}
