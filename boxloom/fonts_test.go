package main

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// 同梱した字体の寸法は全部読め、従来の Noto Sans CJK JP の幅と同じ。
func TestBundledFontMetrics(t *testing.T) {
	fams := FontFamilies()
	want := "BIZ UDGothic,BIZ UDPGothic,DejaVu Sans,IPAexGothic,Liberation Sans,Noto Sans CJK JP"
	if strings.Join(fams, ",") != want {
		t.Fatalf("families = %v", fams)
	}
	noto, _ := LookupFont("Noto Sans CJK JP")
	// 以前の metrics_gen.go の値(Pillow で測ったもの)。
	for ch, w := range map[rune]float64{' ': 0.224, '0': 0.555, '!': 0.323, 'あ': 1.0, '一': 1.0} {
		if got := noto.Regular.of(ch); got != w {
			t.Errorf("Noto %q = %v, want %v", ch, got, w)
		}
	}
	if !noto.Japanese || noto.LineHeightEm() < 1.4 {
		t.Errorf("noto: japanese=%v line height=%v", noto.Japanese, noto.LineHeightEm())
	}
	if dv, _ := LookupFont("DejaVu Sans"); dv.Japanese {
		t.Error("DejaVu Sans must not claim Japanese glyphs")
	}
	if _, err := LookupFont("Meiryo"); err == nil || !strings.Contains(err.Error(), "export_font_metrics.py") {
		t.Errorf("unknown font error = %v", err)
	}
}

// 値の表の fonts は厳しく読む。
func TestTemplateFontsStrict(t *testing.T) {
	cases := []struct {
		name string
		f    func(m map[string]any)
		want string
	}{
		{"missing fonts", func(m map[string]any) { delete(m, "fonts") }, `missing key "fonts"`},
		{"missing latin", func(m map[string]any) { delete(sub(m, "fonts"), "latin") }, `missing key "fonts.latin"`},
		{"unknown key", func(m map[string]any) { sub(m, "fonts")["size"] = 1 }, `unknown field "size"`},
		{"no metrics", func(m map[string]any) { sub(m, "fonts")["latin"] = "Meiryo" }, `fonts.latin: no metrics for font "Meiryo"`},
		{"latin-only as eastAsian", func(m map[string]any) { sub(m, "fonts")["eastAsian"] = "DejaVu Sans" }, `has no Japanese glyphs`},
		{"margin too small", func(m map[string]any) { sub(m, "fonts")["widthMargin"] = 0.9 }, `fonts.widthMargin: must be between 1 and 1.5`},
		{"margin too large", func(m map[string]any) { sub(m, "fonts")["widthMargin"] = 2 }, `fonts.widthMargin: must be between 1 and 1.5`},
		{"fallback unknown", func(m map[string]any) { sub(m, "fonts")["fallbackFonts"] = []any{"Arial"} }, `fonts.fallbackFonts[0]: no metrics for font "Arial"`},
		{"fallback twice", func(m map[string]any) {
			sub(m, "fonts")["fallbackFonts"] = []any{"IPAexGothic", "IPAexGothic"}
		}, `fonts.fallbackFonts[1]: "IPAexGothic" appears twice`},
		{"fallback null", func(m map[string]any) { sub(m, "fonts")["fallbackFonts"] = nil }, `must not be null`},
	}
	for _, c := range cases {
		_, err := parseTemplateFile("light", mutate(t, c.f))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

// 置き換わり得る字体があれば、文字ごとにいちばん広い幅で測る。和文を持たない
// 字体は和文の文字に効かない。
func TestFontSetFallbacksTakeWidest(t *testing.T) {
	noto, _ := LookupFont("Noto Sans CJK JP")
	biz, _ := LookupFont("BIZ UDPGothic")
	dv, _ := LookupFont("DejaVu Sans")
	fs := &FontSet{Latin: noto, EastAsian: noto, Fallbacks: []*FontMetrics{biz, dv}, WidthMargin: 1}
	for _, ch := range "aW1 /" {
		want := noto.Regular.of(ch)
		for _, m := range []*FontMetrics{biz, dv} {
			if v := m.Regular.of(ch); v > want {
				want = v
			}
		}
		if got := fs.emWidth(ch, false); got != want {
			t.Errorf("%q = %v, want %v", ch, got, want)
		}
	}
	// 「、」は DejaVu では 0.6 だが和文の字体で描かれるので Noto(1.0)と BIZ(0.6)の最大。
	if got := fs.emWidth('、', false); got != 1.0 {
		t.Errorf("、 = %v, want 1.0", got)
	}
}

// テーマに欧文と和文の字体を分けて書き、行の送りを固定すると段落に pt で書く。
func TestPPTXFontsAndFixedLineSpacing(t *testing.T) {
	read := func(path, name string) string {
		zr, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name == name {
				r, _ := f.Open()
				b, _ := io.ReadAll(r)
				r.Close()
				return string(b)
			}
		}
		t.Fatalf("%s: %s not found", path, name)
		return ""
	}
	d, err := readDeck("../examples/ir/demo-deck.json")
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.pptx")
	if _, err := tm.RenderDeck(d, plain); err != nil {
		t.Fatal(err)
	}
	theme := read(plain, "ppt/theme/theme1.xml")
	if !strings.Contains(theme, `<a:latin typeface="Noto Sans CJK JP"/><a:ea typeface="Noto Sans CJK JP"/><a:cs typeface=""/>`) {
		t.Errorf("theme fonts: %s", theme[strings.Index(theme, "<a:fontScheme"):strings.Index(theme, "</a:fontScheme>")])
	}
	if strings.Contains(read(plain, "ppt/slides/slide2.xml"), "<a:lnSpc>") {
		t.Error("line spacing must not be fixed by default")
	}

	t2 := *tm
	fs := *tm.Fonts
	biz, _ := LookupFont("BIZ UDPGothic")
	fs.Latin, fs.EastAsian, fs.FixedLineSpacing = biz, biz, true
	t2.Fonts = &fs
	fixed := filepath.Join(dir, "fixed.pptx")
	if _, err := t2.RenderDeck(d, fixed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(fixed, "ppt/theme/theme1.xml"), `<a:ea typeface="BIZ UDPGothic"/>`) {
		t.Error("theme does not name the chosen east asian font")
	}
	// 本文 14pt × 1.35 = 18.9pt → 1890。
	if !strings.Contains(read(fixed, "ppt/slides/slide2.xml"), `<a:lnSpc><a:spcPts val="1890"/></a:lnSpc>`) {
		t.Error("fixed line spacing not written")
	}
}

// fonts コマンドの測り方:BIZ UDPGothic に置き換わると従来の余裕では右に
// はみ出す行があり、置き換わり得る字体に入れれば無くなる。
func TestFontCheckFallbacksRemoveOverflow(t *testing.T) {
	d, err := readDeck("../examples/ir/demo-deck.json")
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	biz, _ := LookupFont("BIZ UDPGothic")
	alt := &FontSet{Latin: biz, EastAsian: biz, WidthMargin: 1}

	pages, _, err := tm.RenderPages(d)
	if err != nil {
		t.Fatal(err)
	}
	if st := measurePages(pages, alt); st.WidthOver == 0 {
		t.Fatal("expected wider lines with BIZ UDPGothic and the default margin")
	}
	same := measurePages(pages, tm.Fonts)
	if same.WidthOver != 0 {
		t.Errorf("measured font itself overflows: %+v", same.Over)
	}

	t2 := *tm
	fs := *tm.Fonts
	fs.Fallbacks = []*FontMetrics{biz}
	t2.Fonts = &fs
	pages2, _, err := t2.RenderPages(d)
	if err != nil {
		t.Fatal(err)
	}
	if st := measurePages(pages2, alt); st.WidthOver != 0 {
		t.Errorf("with fallback, still wider lines: %+v", st.Over)
	}
	activeFonts = defaultFontSet()
}
