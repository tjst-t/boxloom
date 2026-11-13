package boxloom

import (
	"strings"
	"testing"
)

// 架空の和文の段落。report の本文(14pt)で、2列の1列の幅(420pt)と本文の全幅(860pt)で測る。
const densityParagraph = "架空の市では、二〇三〇年までに公共交通の利用者を増やすため、バスの路線を見直し、駅前の広場を歩きやすく整える計画を立てている。住民への聞き取りでは、「乗り換えが分かりにくい」という声が多く、案内の表示とダイヤの組み方をあわせて改める方針である。"

// reportDensityBefore は変更前の report の fonts(Noto Sans CJK JP・置き換わり得る
// 字体5つ・余裕 1.05・行の送り 1.35)。
func reportDensityBefore(t *testing.T) *FontSet {
	t.Helper()
	fs, err := resolveFonts(TemplateFonts{
		Latin: "Noto Sans CJK JP", EastAsian: "Noto Sans CJK JP",
		FallbackFonts: []string{"BIZ UDPGothic", "BIZ UDGothic", "IPAexGothic", "Liberation Sans", "DejaVu Sans"},
		WidthMargin:   1.05, FixedLineSpacing: true, LineHeight: 1.35,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func paragraphHeight(fs *FontSet, text string, size, width float64) (h float64, lines int) {
	defer useFonts(fs)()
	return measureTextHeight(text, size, width, false), len(wrapText(text, size, width, false))
}

// report の行送りと字体(docs/fonts.md の6章)で、同じ段落の要る高さが減る。
// dark・light の行送りは 1.35 のまま。
func TestReportFontDensity(t *testing.T) {
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	if tm.Fonts.LineHeight != 1.2 || tm.Fonts.EastAsian.Family != "BIZ UDPGothic" {
		t.Fatalf("report fonts = lineHeight %v, eastAsian %q", tm.Fonts.LineHeight, tm.Fonts.EastAsian.Family)
	}
	size := tm.Values.Body
	for _, width := range []float64{420, 860} {
		text := strings.Repeat(densityParagraph, 2)
		hb, lb := paragraphHeight(reportDensityBefore(t), text, size, width)
		ha, la := paragraphHeight(tm.Fonts, text, size, width)
		t.Logf("width %.0f: before %d lines %.1fpt, after %d lines %.1fpt, ratio %.3f", width, lb, hb, la, ha, ha/hb)
		if !(ha < hb) || la > lb {
			t.Errorf("width %.0f: after %.1fpt (%d lines) must be lower than before %.1fpt (%d lines)", width, ha, la, hb, lb)
		}
	}
	for _, name := range []string{"dark", "light"} {
		if lt := mustTemplate(name).Fonts.LineHeight; lt != defaultLineHeight {
			t.Errorf("%s: lineHeight %v, want %v", name, lt, defaultLineHeight)
		}
	}
}
