package boxloom

import (
	"strings"
	"testing"
)

// narrowFontCandidates は和文の送りが全角より狭い自由な字体の候補(#147、docs/fonts.md の9章)。
// 寸法は実測で同梱し、値の表の fonts から選べる。report の既定は BIZ UDPGothic のまま。
var narrowFontCandidates = []string{"IPAPGothic", "Mochiy Pop P One"}

// reportFontsWithEastAsian は report の fonts の和文の字体だけを替えた一式。
func reportFontsWithEastAsian(t *testing.T, ea string) *FontSet {
	t.Helper()
	fs, err := resolveFonts(TemplateFonts{
		Latin: "BIZ UDPGothic", EastAsian: ea,
		FallbackFonts: []string{"Liberation Sans", "DejaVu Sans"},
		WidthMargin:   1.05, FixedLineSpacing: true, LineHeight: 1.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

// averageAdvance は架空の段落の1字あたりの平均の送り(em)。raw は字体の送りそのもの、
// measured は組むときの見積もり(全角より狭い文字に widthMargin を掛けた値)。
func averageAdvance(fs *FontSet, text string) (raw, measured float64) {
	defer useFonts(fs)()
	n := 0
	for _, ch := range text {
		raw += fs.EastAsian.width(ch, false)
		measured += charEmWidth(ch, false)
		n++
	}
	return raw / float64(n), measured / float64(n)
}

// 候補の字体は値の表から選べ(和文を持つ)、和文の送りは BIZ UDPGothic 以下。
// 数は docs/fonts.md の9章の表(-v で出る)。
func TestNarrowFontCandidates(t *testing.T) {
	biz := reportFontsWithEastAsian(t, "BIZ UDPGothic")
	bizRaw, _ := averageAdvance(biz, densityParagraph)
	text := strings.Repeat(densityParagraph, 2)
	for _, fam := range append([]string{"BIZ UDPGothic"}, narrowFontCandidates...) {
		fs := reportFontsWithEastAsian(t, fam)
		if !fs.EastAsian.Japanese {
			t.Errorf("%s: must have Japanese glyphs", fam)
		}
		raw, measured := averageAdvance(fs, densityParagraph)
		h420, l420 := paragraphHeight(fs, text, 14, 420)
		h860, l860 := paragraphHeight(fs, text, 14, 860)
		t.Logf("%-18s raw %.3fem measured %.3fem  420pt: %d lines %.1fpt  860pt: %d lines %.1fpt  ascent+descent %.3fem",
			fam, raw, measured, l420, h420, l860, h860, fs.EastAsian.LineHeightEm())
		if raw > bizRaw+1e-9 || raw >= 1 {
			t.Errorf("%s: average advance %.3fem must not exceed BIZ UDPGothic %.3fem", fam, raw, bizRaw)
		}
	}
	if tm := mustTemplate("report"); tm.Fonts.EastAsian.Family != "BIZ UDPGothic" {
		t.Errorf("report eastAsian = %q; the default stays BIZ UDPGothic (#147)", tm.Fonts.EastAsian.Family)
	}
}
