// `boxloom fonts`:字体が置き換わったときに、文字が箱からはみ出すかを測る。
//
// Boxloom は折り返して行の数を測り、PPTX には段落のまま wrap="square" で書く
// (折り返しは描く側。pptx.go の flowXML)。開く環境で字体が置き換わると
//   - 1行の幅:置き換わった字体のほうが広いと、1行と測った文を描く側が2行に折り、
//     枠の下へはみ出す(wider lines はその行の数)
//   - 1行の高さ:行の送りを固定しない(fonts.fixedLineSpacing が false)と、
//     描く側は字体の ascent+descent で行を送るので、箱の下へはみ出す
//
// の2つ。ここでは組んだスライドの文字の図形を1つずつ見て、置き換わり得る
// 字体の寸法で測り直す。あわせて、幅の余裕(fonts.widthMargin)をいくつに
// すれば右へのはみ出しが無くなるかを、余裕を変えて組み直して探す。
// 方針と測った数字は docs/fonts.md。

package boxloom

import (
	"fmt"
	"io"
	"io/fs"
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
	Boxes, Lines  int
	Over          []overLine // 右へはみ出す行(-v で出す)
	WidthOver     int        // 右へはみ出す行
	WorstOverPt   float64    // いちばんはみ出した量(pt)
	WorstOverPct  float64    // その行の幅に対する割合
	WorstLine     string
	RewrapChanged int // 置き換わった字体で折り返し直すと行の数が変わる箱
	// RewrapMore・RewrapFewer はそのうち行が増える箱・減る箱。RewrapOverflow は
	// 増えた行が枠に入りきらず下へはみ出す箱(箇条は枠の中の余りと比べる)。
	RewrapMore, RewrapFewer, RewrapOverflow int
	Rewraps                                 []rewrapBox // 行が増える箱(-v で出す)
	HeightOver                              int         // 下へはみ出す箱(行の送りを固定しないとき)
	WorstHeightPct                          float64
}

// rewrapBox は置き換わった字体で折り返し直すと行が増える箱。
type rewrapBox struct {
	Text      string
	From, To  int  // Boxloom が測った行の数 → 折り返し直した行の数
	Overflows bool // 増えた行が枠に入りきらない
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
	for _, p := range pages {
		for i := range p.Shapes {
			s := &p.Shapes[i]
			if len(s.list) > 0 {
				// 箇条:項目(段落)ごとに、字下げを引いた幅で測る。増えた行は1つの枠の
				// 中で下の項目を押し下げるので、枠の余り(枠の高さ − 測った高さ)と比べる。
				// カードは内側の余白を除いた文字の枠で測る。
				tr := listRect(s)
				used, added := 0.0, 0.0
				var grown []int
				for _, it := range s.list {
					restore := useFonts(p.Fonts)
					box := LBox{X: tr.X + it.Indent, Y: tr.Y, Width: tr.Width - it.Indent, Height: math.Inf(1)}
					lines := wrapRuns(it.Runs, it.Size, box.Width)
					restore()
					used += it.SpaceBefore + float64(len(lines))*it.Size*lineHeightMult
					before := st.RewrapMore
					n := measureUnit(&st, alt, p, s, it.Runs, lines, box, it.Size)
					if st.RewrapMore > before {
						added += float64(n-len(lines)) * it.Size * lineHeightMult
						grown = append(grown, len(st.Rewraps)-1)
					}
				}
				if added > 0 && used+added > tr.Height+0.5 {
					st.RewrapOverflow++
					for _, k := range grown {
						st.Rewraps[k].Overflows = true
					}
				}
				continue
			}
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
			measureUnit(&st, alt, p, s, s.runs, lines, box, s.fontSize)
		}
	}
	return st
}

// measureUnit は1つの文字の枠(箇条なら1つの段落)を alt の字体で測り直して数える。
// 返り値は置き換わった字体で折り返し直した行の数(折り返さない文字なら測った行の数)。
func measureUnit(st *fontStats, alt *FontSet, p *PPTXBuilder, s *shape, runs []TextRun, lines [][]TextRun, box LBox, size float64) int {
	const eps = 0.01
	st.Boxes++
	st.Lines += len(lines)
	for _, l := range lines {
		w := lineWidthPt(alt, l, size)
		if over := w - box.Width; over > eps {
			st.Over = append(st.Over, overLine{Text: runsText(l), OverPt: over, BoxW: box.Width, Wrapped: s.wrap || len(runs) > 0, Align: s.align})
			st.WidthOver++
			if over > st.WorstOverPt {
				st.WorstOverPt = over
				st.WorstOverPct = 100 * over / box.Width
				st.WorstLine = runsText(l)
			}
		}
	}
	// 折り返し直したら行の数が変わるか(描く側が自分で折り返すときの目安)。
	n := len(lines)
	if s.wrap || len(runs) > 0 {
		// 描く側は余裕を掛けずに測る(置き換わった字体そのものの幅)。行の頭に
		// 閉じの約物を置かない禁則(追い出し)も真似る。
		bare := *alt
		bare.WidthMargin = 1
		restore := useFonts(&bare)
		kinsokuGlue = true
		defer func() { kinsokuGlue = false }()
		if len(runs) > 0 {
			n = len(wrapRuns(runs, size, box.Width))
		} else {
			n = len(wrapText(s.text, size, box.Width, s.bold))
		}
		restore()
		switch {
		case n > len(lines):
			st.RewrapChanged++
			st.RewrapMore++
			over := float64(n)*size*lineHeightMult > box.Height+0.5
			if over {
				st.RewrapOverflow++
			}
			text := runsText(runs)
			if text == "" {
				text = s.text
			}
			st.Rewraps = append(st.Rewraps, rewrapBox{Text: text, From: len(lines), To: n, Overflows: over})
		case n < len(lines):
			st.RewrapChanged++
			st.RewrapFewer++
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
	h := float64(len(lines)) * size * lh
	assumed := float64(len(lines)) * size * lineHeightMult
	// 組むときの見積もり(行の数×字の大きさ×lineHeightMult)では収まって
	// いた箱だけを数える(エンジンが縮めた箱・固定の大きさの箱は別の話)。
	if h-box.Height > eps && assumed <= box.Height+eps {
		st.HeightOver++
		if pct := 100 * (h - box.Height) / box.Height; pct > st.WorstHeightPct {
			st.WorstHeightPct = pct
		}
	}
	return n
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

// WriteFontList は寸法を持つ字体の一覧を w に書く(`boxloom fonts -list`)。
func WriteFontList(w io.Writer) {
	for _, f := range FontFamilies() {
		m, _ := LookupFont(f)
		fmt.Fprintf(w, "%-20s japanese=%-5v line-height=%.3fem license=%s\n", f, m.Japanese, m.LineHeightEm(), m.License)
	}
}

// FontCheck は `boxloom fonts` の測り(文書を1つずつ Deck に渡し、最後に
// Summary)。against は試す置き換わり方("欧文/和文" か "字体" を ; で区切る。
// 空なら既定の5つ)。
type FontCheck struct {
	// Template は文書の template より優先するテンプレートの名前。
	Template string
	// Values は外から読んだテンプレートの値の表(ParseTemplate)。nil でなければ
	// Template より優先する。
	Values *Template
	// Verbose ははみ出す行を1行ずつ出す。
	Verbose bool
	// Assets は画像を読む fs.FS(BuildOptions.Assets と同じ)。Deck ごとに替えてよい。
	Assets fs.FS

	scenarios   []fontScenario
	worstNeeded map[string]float64
	decks       int
}

// NewFontCheck は against を読んで FontCheck を作る。
func NewFontCheck(against string) (*FontCheck, error) {
	scs, err := parseScenarios(against)
	if err != nil {
		return nil, err
	}
	return &FontCheck{scenarios: scs, worstNeeded: map[string]float64{}}, nil
}

// Deck は1つの文書(file は表に出す名前)を測って w に書く。組めなかった
// ときの誤りには file を添える。
func (fc *FontCheck) Deck(w io.Writer, file string, d *IRDeck) error {
	fc.decks++
	name := d.Template
	if fc.Template != "" {
		name = fc.Template
	}
	t := fc.Values
	if t == nil {
		var err error
		if t, err = LookupTemplate(name); err != nil {
			return err
		}
	}
	t.SetAssets(fc.Assets)
	pages, _, err := t.RenderPages(d)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	var fb []string
	for _, m := range t.Fonts.Fallbacks {
		fb = append(fb, m.Family)
	}
	fmt.Fprintf(w, "%s (template %s: latin %q, eastAsian %q, fallbackFonts %q, widthMargin %.2f, fixedLineSpacing %v)\n",
		file, t.Name, t.Fonts.Latin.Family, t.Fonts.EastAsian.Family, fb, t.Fonts.WidthMargin, t.Fonts.FixedLineSpacing)
	rows := append([]fontScenario{{Label: "(測った字体のまま)", Latin: t.Fonts.Latin.Family, EA: t.Fonts.EastAsian.Family}}, fc.scenarios...)
	for _, sc := range rows {
		alt, err := scenarioFonts(sc, t.Fonts)
		if err != nil {
			return err
		}
		st := measurePages(pages, alt)
		need, err := neededMargin(t, d, alt)
		if err != nil {
			return err
		}
		needS := fmt.Sprintf("%.2f", need)
		if need == 0 {
			needS = "> 1.50"
			need = 9
		}
		if need > fc.worstNeeded[sc.Label] {
			fc.worstNeeded[sc.Label] = need
		}
		fmt.Fprintf(w, "  %-32s boxes %3d lines %3d | wider lines %3d (worst +%.1fpt, %.1f%%) | rewrap changes %3d (more %d, of which overflow %d; fewer %d) | taller boxes %3d (worst +%.0f%%) | widthMargin needed %s\n",
			sc.Label, st.Boxes, st.Lines, st.WidthOver, st.WorstOverPt, st.WorstOverPct, st.RewrapChanged, st.RewrapMore, st.RewrapOverflow, st.RewrapFewer, st.HeightOver, st.WorstHeightPct, needS)
		if st.WorstLine != "" && !fc.Verbose {
			fmt.Fprintf(w, "  %-32s   worst line: %q\n", "", st.WorstLine)
		}
		if fc.Verbose {
			for _, r := range st.Rewraps {
				mark := "fits"
				if r.Overflows {
					mark = "OVERFLOWS"
				}
				fmt.Fprintf(w, "      rewrap %d -> %d lines %-9s %q\n", r.From, r.To, mark, r.Text)
			}
			for _, o := range st.Over {
				kind := "label"
				if o.Wrapped {
					kind = "wrapped"
				}
				fmt.Fprintf(w, "      +%5.1fpt box %6.1fpt %-7s align=%-5s %q\n", o.OverPt, o.BoxW, kind, o.Align, o.Text)
			}
		}
	}
	return nil
}

// Summary は2つ以上の文書を測ったとき、全部を通して要る幅の余裕を w に書く
// (1つなら何も書かない)。
func (fc *FontCheck) Summary(w io.Writer) {
	if fc.decks <= 1 {
		return
	}
	fmt.Fprintln(w, "widthMargin needed over all decks:")
	for _, sc := range append([]fontScenario{{Label: "(測った字体のまま)"}}, fc.scenarios...) {
		v := fc.worstNeeded[sc.Label]
		s := fmt.Sprintf("%.2f", v)
		if v >= 9 {
			s = "> 1.50"
		}
		fmt.Fprintf(w, "  %-32s %s\n", sc.Label, s)
	}
}
