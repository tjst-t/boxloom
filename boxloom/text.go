// box_engine.py のテキスト計測ロジック(_tokens/wrap_text/measure_text_height/
// _natural_text_width)のGo移植。本物のPythonソース(box_engine.py)を見ながら
// 移植し直したバージョン。
//
// フォントファイルは一切読まない。fonts/<slug>.json に同梱した「1emあたりの
// グリフ幅」(tools/export_font_metrics.py が Pillow で測ったもの)だけを使う。
// どの字体で測るか・幅の余裕は、いまの字体の一式(activeFonts、fonts.go)が持つ。
// 表に無い文字はその字体の default(漢字の幅)を使う。
//
// 唯一の意図的な相違点: 本物はwrap_text()自体では明示的な改行("\n")を
// 特別扱いしない(Ganttサブエンジン内の複数行ラベルはbox_engine.wrap_text()を
// 経由せず、PowerPoint/LibreOffice側のネイティブ折り返し word_wrap=True に
// 任せている)。このGo版も PPTX には段落のまま wrap="square" で書いて折り返しは
// アプリに任せるが、高さを決めるために同じ文を自分でも折り返して行を数えるので、
// "\n"を段落境界として扱う機能が必要。そのため wrapText() は "\n" で
// 段落分割してから、各段落に対して本物と同じトークン化・折り返しロジックを
// 適用する。

package boxloom

import (
	"regexp"
	"strings"
)

const (
	lineHeightMult = 1.35 // 行間。box_engine.py の LINE_HEIGHT_MULT と同じ。
)

// measureSafetyMargin は測った幅に掛ける余裕(既定 1.05。box_engine.py の
// MEASURE_SAFETY_MARGIN と同じ)。テンプレートの fonts.widthMargin で決まる。
// 余裕は全角の文字(どの字体でも 1em)には掛けず、それより狭い文字(欧文・数字・
// 詰めた仮名など、字体によって幅が変わる文字)にだけ掛ける(charEmWidth)。
func measureSafetyMargin() float64 { return activeFonts.WidthMargin }

// fullWidthEm より広い文字は全角とみなし、余裕を掛けない。和文の字体は
// 漢字・仮名・全角の約物を 1em の正方形に描くので、置き換わっても広くならない。
const fullWidthEm = 0.99

// 本物の _tokens() と同じトークン化: 英数字記号の塊はひとつのトークンとして
// 分割しない(単語の途中で折り返さない)。空白の連続もひとつのトークン。
// それ以外は1文字ずつトークン化する。
var tokenRe = regexp.MustCompile(`(?s)[A-Za-z0-9._%+\-]+|[ \t]+|.`)

func tokenizeLine(s string) []string {
	return tokenRe.FindAllString(s, -1)
}

// charEmWidth は1文字分の「フォントサイズ1ptあたりの幅(pt)」を返す。
// 全角より狭い文字には幅の余裕(measureSafetyMargin)を掛けた値。
// 改行は幅0とする(Pillowでの実測に相当する扱いが無いため明示的にゼロ扱いにする)。
func charEmWidth(ch rune, bold bool) float64 {
	if ch == '\n' {
		return 0
	}
	w := activeFonts.emWidth(ch, bold)
	if w < fullWidthEm {
		w *= measureSafetyMargin()
	}
	return w
}

// runWidthPt は文字列全体の幅(pt)を安全マージン込みで返す(トークン化せず、
// 本物の get_font().getlength(label) 相当の単純合計。凡例ラベルの幅計算に使う)。
func runWidthPt(s string, fontSizePt float64, bold bool) float64 {
	total := 0.0
	for _, ch := range s {
		total += charEmWidth(ch, bold) * fontSizePt
	}
	return total
}

// tokenWidthPt は1トークン分の幅(pt)。安全マージン込み。
// box_engine.py の wrap_text() 内の width_of() に相当。
func tokenWidthPt(tok string, fontSizePt float64, bold bool) float64 {
	total := 0.0
	for _, ch := range tok {
		total += charEmWidth(ch, bold) * fontSizePt
	}
	return total
}

// naturalTextWidth はテキストノードの「折り返さない場合の自然な幅」。
// box_engine.py の _natural_text_width() と同じく、改行を含む文字列全体を
// 1行として扱った場合の合計幅を返す(改行自体の幅は0)。
func naturalTextWidth(s *Spec) float64 {
	total := 0.0
	for _, tok := range tokenizeLine(s.Text) {
		for _, ch := range tok {
			total += charEmWidth(ch, s.Bold)
		}
	}
	return total * s.FontSize
}

// wrapText はテキストを maxWidthPt に収まるよう折り返す。
// "\n"は段落境界として維持し(このGo版の自前ライターが行送りを自分で
// 制御する必要があるための意図的な拡張)、各段落の中は本物のwrap_text()と
// 同じトークン単位の折り返しロジックを使う: 空白トークンは折り返しの
// トリガーにならない(そのまま現在行に追加され続ける)。
func wrapText(text string, fontSizePt, maxWidthPt float64, bold bool) []string {
	var result []string
	for _, para := range strings.Split(text, "\n") {
		toks := tokenizeLine(para)
		var cur strings.Builder
		curWidth := 0.0
		curLen := 0
		for _, tok := range toks {
			tokW := tokenWidthPt(tok, fontSizePt, bold)
			isSpace := strings.TrimSpace(tok) == ""
			// maxWidthPt自体がnaturalTextWidth()(同じトークンの合計)から来ることが多く、
			// 浮動小数点の加算順序の違いだけで curWidth+tokW がごく僅かに上回り、最後の
			// 1トークンだけ次行に送られてしまうことがある。実用上無害な微小な許容誤差を入れる。
			const wrapEps = 1e-6
			if curLen > 0 && curWidth+tokW > maxWidthPt+wrapEps && !isSpace {
				result = append(result, cur.String())
				cur.Reset()
				curWidth = 0
				curLen = 0
			}
			cur.WriteString(tok)
			curWidth += tokW
			curLen += len([]rune(tok))
		}
		result = append(result, cur.String())
	}
	if len(result) == 0 {
		result = append(result, "")
	}
	return result
}

// measureTextHeight は折り返した結果の行数から高さ(pt)を計算する。
// box_engine.py の measure_text_height() に相当。
func measureTextHeight(text string, fontSizePt, maxWidthPt float64, bold bool) float64 {
	lines := wrapText(text, fontSizePt, maxWidthPt, bold)
	return float64(len(lines)) * fontSizePt * lineHeightMult
}

// --- 装飾つきの文字(強調などを含む1つの段落) ---

// TextRun は同じ見た目が続く文字の塊。意味(強調など)を見た目(色・太さ)に
// 変えるのはテンプレートで、ここには見た目だけが入る。
//
// Size は塊の字の大きさ(pt)。0 なら文字の葉の大きさ(FontSize)。行の高さは葉の
// 大きさで測るので、大きさの違う塊を混ぜるときは、葉の大きさを一番大きい塊に
// 合わせる(例 統計強調の数と、その後ろの小さな単位)。
type TextRun struct {
	Text  string
	Color string
	Bold  bool
	Size  float64
	// Mark は文の頭の印(箇条の「・」、良い点・悪い点の「＋」「－」など)。
	// 測るときはふつうの文字と同じに幅を数える。PPTX では印を本文の文字に
	// せず、段落の箇条の印(a:buChar)として書き、本文は印の幅だけ字下げした
	// 位置から始める(描く側が和文と欧文の間に足す空き(LibreOffice の
	// 「アジア言語と欧文の間隔」)が、印と数字・英字の間に入らないように)。
	Mark bool
	// Keep は塊の中で折り返さない(塊と塊の境目では折れる)。stat の数・単位の
	// ように、途中で折れると読み違える文字に使う。1つの塊が幅に入りきらなければ
	// 1行のまま幅からはみ出すので、使う側が幅を測って警告する(stat なら
	// stat-value、keepline.go)。
	Keep bool
	// Phrase は折るところを文の切れ目・読点に寄せる(行の数は増やさない)。
	// リード文のように文として読む段落に使う。塊のどれかが Phrase なら段落の
	// 全体に効く。
	Phrase bool
}

// runSize は塊の字の大きさ(0 なら葉の大きさ)。
func runSize(r TextRun, fontSizePt float64) float64 {
	if r.Size > 0 {
		return r.Size
	}
	return fontSizePt
}

// runsText は塊をつないだ文字列。
func runsText(runs []TextRun) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// richRunes は塊を1文字ずつにほどいたもの(文字と、その文字の塊の番号)。
func richRunes(runs []TextRun) ([]rune, []int) {
	var rs []rune
	var style []int
	for i, r := range runs {
		for _, ch := range r.Text {
			rs = append(rs, ch)
			style = append(style, i)
		}
	}
	return rs, style
}

// naturalRunsWidth は折り返さないときの幅(安全マージン込み)。
func naturalRunsWidth(runs []TextRun, fontSizePt float64) float64 {
	total := 0.0
	for _, r := range runs {
		for _, ch := range r.Text {
			total += charEmWidth(ch, r.Bold) * runSize(r, fontSizePt)
		}
	}
	return total
}

// wrapRuns は wrapText と同じ規則(トークン単位・空白では折り返さない・"\n" は
// 段落の境目)で、装飾つきの文字を折り返す。太さは文字ごとに測る。
//
// 塊のどれかが Phrase なら、行の数を増やさない範囲で、文の切れ目(。!?)、
// 次に読点(、,)の後ろで折る(phraseBreaks)。
func wrapRuns(runs []TextRun, fontSizePt, maxWidthPt float64) [][]TextRun {
	rs, style := richRunes(runs)
	phrase := false
	for _, r := range runs {
		phrase = phrase || r.Phrase
	}
	var lines [][2]int // [start, end) の文字の位置
	start := 0
	for paraEnd := 0; paraEnd <= len(rs); paraEnd++ {
		if paraEnd < len(rs) && rs[paraEnd] != '\n' {
			continue
		}
		// 頭の印(箇条の「・」など)は、PPTX では段落の箇条の印として書き、本文は
		// 印の幅だけ字下げする(ぶら下げ)。2行目以降も印の幅だけ内側から始まるので、
		// 印を除いた本文を、幅から印の幅を引いた幅で折る。
		markEnd, markW := start, 0.0
		for markEnd < paraEnd && runs[style[markEnd]].Mark {
			markW += charEmWidth(rs[markEnd], runs[style[markEnd]].Bold) * runSize(runs[style[markEnd]], fontSizePt)
			markEnd++
		}
		var toks []wrapTok
		pos := markEnd
		for _, tok := range keepTokens(rs[markEnd:paraEnd], style[markEnd:paraEnd], runs) {
			n := len([]rune(tok))
			tokW := 0.0
			for k := pos; k < pos+n; k++ {
				tokW += charEmWidth(rs[k], runs[style[k]].Bold) * runSize(runs[style[k]], fontSizePt)
			}
			t := wrapTok{pos: pos, n: n, w: tokW, space: strings.TrimSpace(tok) == "", text: tok}
			pos += n
			// 禁則の追い出し(fonts の測り直しだけ):行の頭に来てはいけない閉じの約物は
			// 前のトークンにつなげ、前の字と一緒に次の行へ送る。
			if kinsokuGlue && len(toks) > 0 && !toks[len(toks)-1].space && strings.Contains(closers, tok) {
				last := &toks[len(toks)-1]
				last.n += t.n
				last.w += t.w
				last.text += t.text
				continue
			}
			toks = append(toks, t)
		}
		var breaks []int // 次の行の頭のトークンの番号
		if phrase {
			breaks = phraseBreaks(toks, maxWidthPt-markW)
		} else {
			breaks = greedyBreaks(toks, 0, maxWidthPt-markW)
		}
		lineStart := start
		for _, b := range breaks {
			lines = append(lines, [2]int{lineStart, toks[b].pos})
			lineStart = toks[b].pos
		}
		lines = append(lines, [2]int{lineStart, paraEnd})
		start = paraEnd + 1
	}
	out := make([][]TextRun, 0, len(lines))
	for _, ln := range lines {
		var line []TextRun
		for k := ln[0]; k < ln[1]; k++ {
			r := runs[style[k]]
			if len(line) > 0 && k > ln[0] && style[k] == style[k-1] {
				line[len(line)-1].Text += string(rs[k])
				continue
			}
			line = append(line, TextRun{Text: string(rs[k]), Color: r.Color, Bold: r.Bold, Size: r.Size, Mark: r.Mark, Keep: r.Keep, Phrase: r.Phrase})
		}
		out = append(out, line)
	}
	return out
}

// keepTokens は tokenizeLine と同じ区切りで、ただし Keep の塊は塊ごとに1つの
// 区切りにする(塊の中では折らない。隣り合う Keep の塊の境目では折れる)。
func keepTokens(rs []rune, style []int, runs []TextRun) []string {
	var out []string
	for i := 0; i < len(rs); {
		j := i
		if runs[style[i]].Keep {
			for j < len(rs) && style[j] == style[i] {
				j++
			}
			out = append(out, string(rs[i:j]))
			i = j
			continue
		}
		for j < len(rs) && !runs[style[j]].Keep {
			j++
		}
		out = append(out, tokenizeLine(string(rs[i:j]))...)
		i = j
	}
	return out
}

// measureRunsHeight は折り返した行数から高さを出す。
func measureRunsHeight(runs []TextRun, fontSizePt, maxWidthPt float64) float64 {
	return float64(len(wrapRuns(runs, fontSizePt, maxWidthPt))) * fontSizePt * lineHeightMult
}

// kinsokuGlue が true なら、閉じの約物(、。」など)を前のトークンにつなげて折る
// (行の頭に来ない。PowerPoint の禁則の追い出しに近い)。組むときは false で、
// fonts の測り直し(fontcheck.go)だけが描く側を真似るために立てる。
var kinsokuGlue bool

// wrapTok は折り返しの単位(トークン)。pos は文字の位置、w は幅(pt)。
type wrapTok struct {
	pos, n int
	w      float64
	space  bool
	text   string
}

// greedyBreaks は from 番目のトークンから、入るだけ詰めて折った行の頭の
// トークンの番号(from の行は含めない)。空白では折らない。
func greedyBreaks(toks []wrapTok, from int, maxW float64) []int {
	// maxW 自体が同じトークンの合計から来ることが多く、浮動小数点の加算順序の
	// 違いだけで僅かに上回ることがある。実用上無害な微小な許容誤差を入れる。
	const wrapEps = 1e-6
	var breaks []int
	curW, curLen := 0.0, 0
	for i := from; i < len(toks); i++ {
		t := toks[i]
		if curLen > 0 && curW+t.w > maxW+wrapEps && !t.space {
			breaks = append(breaks, i)
			curW, curLen = 0, 0
		}
		curW += t.w
		curLen += t.n
	}
	return breaks
}

// 文の切れ目と読点。closers は行の頭に来てはいけない閉じの約物(その前では折らない)。
const (
	sentenceEnds = "。．！？!?"
	commaMarks   = "、，,"
	closers      = "」』）)】〕”’、。，．,.!?！？・:：;；"
)

// phraseBreaks は詰めて折ったときと同じ行の数のまま、行の終わりを文の切れ目、
// 次に読点の後ろに寄せる。どちらも無い行は詰めて折る。
func phraseBreaks(toks []wrapTok, maxW float64) []int {
	total := len(greedyBreaks(toks, 0, maxW)) + 1
	var breaks []int
	for i := 0; ; {
		g := greedyBreaks(toks, i, maxW)
		if len(g) == 0 {
			return breaks
		}
		end := g[0]                     // 詰めたときの次の行の頭
		left := total - len(breaks) - 1 // この行のあとに使える行の数
		best := end
	search:
		for _, marks := range []string{sentenceEnds, commaMarks} {
			for j := end; j > i+1; j-- {
				prev := toks[j-1]
				if prev.space || !strings.Contains(marks, prev.text) {
					continue
				}
				k := j
				for k < len(toks) && toks[k].space {
					k++ // 後ろの空白はこの行に残す
				}
				if k >= len(toks) || strings.Contains(closers, toks[k].text) {
					continue
				}
				if len(greedyBreaks(toks, k, maxW))+1 <= left {
					best = k
					break search
				}
			}
		}
		breaks = append(breaks, best)
		i = best
	}
}
