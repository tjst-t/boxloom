// `boxloom fonts`:字体が置き換わったときに、文字が箱からはみ出すかを測る。
//
// PPTX は折り返しを自前で済ませて wrap="none" で書く(text.go)。だから
// 開く環境で字体が置き換わっても行の数は変わらない。変わるのは
//   - 1行の幅:置き換わった字体のほうが広いと、行が箱の右へはみ出す
//   - 1行の高さ:行の送りを固定しない(fonts.fixedLineSpacing が false)と、
//     描く側は字体の ascent+descent で行を送るので、箱の下へはみ出す
//
// の2つ。ここでは組んだスライドの文字の図形を1つずつ見て、置き換わり得る
// 字体の寸法で測り直す。あわせて、幅の余裕(fonts.widthMargin)をいくつに
// すれば右へのはみ出しが無くなるかを、余裕を変えて組み直して探す。
// 方針と測った数字は docs/fonts.md。
package main

import (
	"flag"
	"fmt"
	"io"
	"math"

	"strings"
)

// fontScenario は置き換わり方の1つ(欧文と和文の字体の組)。
type fontScenario struct {
	Label     string
	Latin, EA string
	Note      string
}

// defaultScenarios は既定で試す置き換わり方。同梱の寸法はすべて実測だが、
// 手元に無い字体(メイリオ・游ゴシック・ヒラギノ・Arial)は、形の近い自由な
// 字体を代わりに使う(推定。docs/fonts.md)。
var defaultScenarios = []fontScenario{
	{"BIZ UDPGothic", "BIZ UDPGothic", "BIZ UDPGothic", "Windows 10 以降に同梱。仮名・欧文がプロポーショナル"},
	{"BIZ UDGothic", "BIZ UDGothic", "BIZ UDGothic", "欧文が半角の等幅(MS ゴシックと同じ振る舞い)"},
	{"IPAexGothic", "IPAexGothic", "IPAexGothic", "欧文がプロポーショナル、仮名は全角"},
	{"Liberation Sans + IPAexGothic", "Liberation Sans", "IPAexGothic", "欧文が Arial と同じ幅(推定:Arial の代わり)"},
	{"DejaVu Sans + IPAexGothic", "DejaVu Sans", "IPAexGothic", "欧文が広い字体(推定:メイリオの欧文の代わり)"},
}

type fontStats struct {
	Boxes, Lines   int
	Over           []overLine // 右へはみ出す行(-v で出す)
	WidthOver      int        // 右へはみ出す行
	WorstOverPt    float64    // いちばんはみ出した量(pt)
	WorstOverPct   float64    // その行の幅に対する割合
	WorstLine      string
	RewrapChanged  int // 置き換わった字体で折り返し直すと行の数が変わる箱
	HeightOver     int // 下へはみ出す箱(行の送りを固定しないとき)
	WorstHeightPct float64
}

type overLine struct {
	Text    string
	OverPt  float64
	BoxW    float64
	Wrapped bool
	Align   string
}

// lineWidthPt は1行の幅(em×pt、余裕なし)。行末の空白は見えないので数えない。
func lineWidthPt(fs *FontSet, line []TextRun, size float64) float64 {
	var rs []rune
	var bold []bool
	for _, r := range line {
		for _, ch := range r.Text {
			rs = append(rs, ch)
			bold = append(bold, r.Bold)
		}
	}
	end := len(rs)
	for end > 0 && (rs[end-1] == ' ' || rs[end-1] == '\t' || rs[end-1] == '　') {
		end--
	}
	w := 0.0
	for i := 0; i < end; i++ {
		if rs[i] == '\n' {
			continue
		}
		w += fs.emWidth(rs[i], bold[i]) * size
	}
	return w
}

func hasEastAsian(lines [][]TextRun) bool {
	for _, l := range lines {
		for _, r := range l {
			for _, ch := range r.Text {
				if isEastAsianRune(ch) {
					return true
				}
			}
		}
	}
	return false
}

// measurePages は組んだスライドの文字を alt の字体で測り直す。
func measurePages(pages []*PPTXBuilder, alt *FontSet) fontStats {
	var st fontStats
	const eps = 0.01
	for _, p := range pages {
		for i := range p.Shapes {
			s := &p.Shapes[i]
			var lines [][]TextRun
			box := s.box
			switch {
			case s.kind == skText:
				lines = p.textLines(s)
			case s.kind == skShape && len(s.runs) > 0:
				// 図形の中の文字(shapes のキャンバス):文字の枠の幅で折り返している。
				tr := shapeTextRect(s.prst, s.box)
				box = LBox{X: tr.X + s.insetX, Y: tr.Y, Width: tr.Width - 2*s.insetX, Height: tr.Height}
				restore := useFonts(p.Fonts)
				lines = wrapRuns(s.runs, s.fontSize, box.Width)
				restore()
			default:
				continue
			}
			st.Boxes++
			st.Lines += len(lines)
			for _, l := range lines {
				w := lineWidthPt(alt, l, s.fontSize)
				if over := w - box.Width; over > eps {
					st.Over = append(st.Over, overLine{Text: runsText(l), OverPt: over, BoxW: box.Width, Wrapped: s.wrap || len(s.runs) > 0, Align: s.align})
					st.WidthOver++
					if over > st.WorstOverPt {
						st.WorstOverPt = over
						st.WorstOverPct = 100 * over / box.Width
						st.WorstLine = runsText(l)
					}
				}
			}
			// 折り返し直したら行の数が変わるか(描く側が自分で折り返すときの目安)。
			if s.wrap || len(s.runs) > 0 {
				restore := useFonts(alt)
				var n int
				if len(s.runs) > 0 {
					n = len(wrapRuns(s.runs, s.fontSize, box.Width))
				} else {
					n = len(wrapText(s.text, s.fontSize, box.Width, s.bold))
				}
				restore()
				if n != len(lines) {
					st.RewrapChanged++
				}
			}
			// 行の高さ:固定しないなら字体の ascent+descent で送られる(目安)。
			lh := lineHeightMult
			if p.Fonts == nil || !p.Fonts.FixedLineSpacing {
				lh = alt.Latin.LineHeightEm()
				if hasEastAsian(lines) {
					lh = math.Max(lh, alt.EastAsian.LineHeightEm())
				}
			}
			h := float64(len(lines)) * s.fontSize * lh
			assumed := float64(len(lines)) * s.fontSize * lineHeightMult
			// 組むときの見積もり(行の数×字の大きさ×lineHeightMult)では収まって
			// いた箱だけを数える(エンジンが縮めた箱・固定の大きさの箱は別の話)。
			if h-box.Height > eps && assumed <= box.Height+eps {
				st.HeightOver++
				if pct := 100 * (h - box.Height) / box.Height; pct > st.WorstHeightPct {
					st.WorstHeightPct = pct
				}
			}
		}
	}
	return st
}

func scenarioFonts(sc fontScenario, base *FontSet) (*FontSet, error) {
	lat, err := LookupFont(sc.Latin)
	if err != nil {
		return nil, err
	}
	ea, err := LookupFont(sc.EA)
	if err != nil {
		return nil, err
	}
	// 描く側の字体なので、置き換わり得る字体の一覧は持たない(そのまま測る)。
	return &FontSet{Latin: lat, EastAsian: ea, WidthMargin: base.WidthMargin, FixedLineSpacing: base.FixedLineSpacing}, nil
}

// withMargin はテンプレートの写しの、幅の余裕だけを替えたもの。
func (t *Template) withMargin(m float64) *Template {
	t2 := *t
	fs := *t.Fonts
	fs.WidthMargin = m
	t2.Fonts = &fs
	return &t2
}

// neededMargin は alt で右へのはみ出しが無くなる最小の余裕(0.01 刻み、
// 1.00〜1.50)。無ければ 0。
func neededMargin(t *Template, d *IRDeck, alt *FontSet) (float64, error) {
	for k := 100; k <= 150; k++ {
		m := float64(k) / 100
		pages, _, err := t.withMargin(m).RenderPages(d)
		if err != nil {
			return 0, err
		}
		if measurePages(pages, alt).WidthOver == 0 {
			return m, nil
		}
	}
	return 0, nil
}

func parseScenarios(s string) ([]fontScenario, error) {
	if s == "" {
		return defaultScenarios, nil
	}
	var out []fontScenario
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lat, ea, ok := strings.Cut(part, "/")
		if !ok {
			ea = lat
		}
		lat, ea = strings.TrimSpace(lat), strings.TrimSpace(ea)
		label := lat
		if ea != lat {
			label = lat + " + " + ea
		}
		out = append(out, fontScenario{Label: label, Latin: lat, EA: ea})
	}
	return out, nil
}

func runFonts(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fonts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "テンプレート(省けば文書の template か light)")
	against := fs.String("against", "", `試す置き換わり方。"欧文/和文" か "字体" を ; で区切る(省けば既定の5つ)`)
	list := fs.Bool("list", false, "寸法を持つ字体の一覧を出して終わる")
	verbose := fs.Bool("v", false, "はみ出す行を1行ずつ出す")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: boxloom fonts [-template 名前] [-against \"欧文/和文;…\"] deck.json|deck.bx.md …")
		fmt.Fprintln(stderr, "       boxloom fonts -list")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *list {
		for _, f := range FontFamilies() {
			m, _ := LookupFont(f)
			fmt.Fprintf(stdout, "%-20s japanese=%-5v line-height=%.3fem license=%s\n", f, m.Japanese, m.LineHeightEm(), m.License)
		}
		return 0
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	scs, err := parseScenarios(*against)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	worstNeeded := map[string]float64{}
	for _, in := range fs.Args() {
		d, err := readDeck(in)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		name := d.Template
		if *tmpl != "" {
			name = *tmpl
		}
		t, err := LookupTemplate(name)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		pages, _, err := t.RenderPages(d)
		if err != nil {
			fmt.Fprintf(stderr, "error: %s: %v\n", in, err)
			return 1
		}
		var fb []string
		for _, m := range t.Fonts.Fallbacks {
			fb = append(fb, m.Family)
		}
		fmt.Fprintf(stdout, "%s (template %s: latin %q, eastAsian %q, fallbackFonts %q, widthMargin %.2f, fixedLineSpacing %v)\n",
			in, t.Name, t.Fonts.Latin.Family, t.Fonts.EastAsian.Family, fb, t.Fonts.WidthMargin, t.Fonts.FixedLineSpacing)
		rows := append([]fontScenario{{Label: "(測った字体のまま)", Latin: t.Fonts.Latin.Family, EA: t.Fonts.EastAsian.Family}}, scs...)
		for _, sc := range rows {
			alt, err := scenarioFonts(sc, t.Fonts)
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			st := measurePages(pages, alt)
			need, err := neededMargin(t, d, alt)
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			needS := fmt.Sprintf("%.2f", need)
			if need == 0 {
				needS = "> 1.50"
				need = 9
			}
			if need > worstNeeded[sc.Label] {
				worstNeeded[sc.Label] = need
			}
			fmt.Fprintf(stdout, "  %-32s boxes %3d lines %3d | wider lines %3d (worst +%.1fpt, %.1f%%) | rewrap changes %3d | taller boxes %3d (worst +%.0f%%) | widthMargin needed %s\n",
				sc.Label, st.Boxes, st.Lines, st.WidthOver, st.WorstOverPt, st.WorstOverPct, st.RewrapChanged, st.HeightOver, st.WorstHeightPct, needS)
			if st.WorstLine != "" && !*verbose {
				fmt.Fprintf(stdout, "  %-32s   worst line: %q\n", "", st.WorstLine)
			}
			if *verbose {
				for _, o := range st.Over {
					kind := "label"
					if o.Wrapped {
						kind = "wrapped"
					}
					fmt.Fprintf(stdout, "      +%5.1fpt box %6.1fpt %-7s align=%-5s %q\n", o.OverPt, o.BoxW, kind, o.Align, o.Text)
				}
			}
		}
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(stdout, "widthMargin needed over all decks:")
		for _, sc := range append([]fontScenario{{Label: "(測った字体のまま)"}}, scs...) {
			v := worstNeeded[sc.Label]
			s := fmt.Sprintf("%.2f", v)
			if v >= 9 {
				s = "> 1.50"
			}
			fmt.Fprintf(stdout, "  %-32s %s\n", sc.Label, s)
		}
	}
	return 0
}
