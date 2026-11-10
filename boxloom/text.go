// box_engine.py のテキスト計測ロジック(_tokens/wrap_text/measure_text_height/
// _natural_text_width)のGo移植。本物のPythonソース(box_engine.py)を見ながら
// 移植し直したバージョン。
//
// フォントファイルは一切読まない。metrics_gen.go に埋め込まれた「1emあたりの
// グリフ幅」テーブル(Pillowで事前計測済み)だけを使う。テーブルに無い文字は
// 全角相当(1.0em)にフォールバックする(export_font_metrics.py のコメント通りの割り切り)。
//
// 唯一の意図的な相違点: 本物はwrap_text()自体では明示的な改行("\n")を
// 特別扱いしない(Ganttサブエンジン内の複数行ラベルはbox_engine.wrap_text()を
// 経由せず、PowerPoint/LibreOffice側のネイティブ折り返し word_wrap=True に
// 任せている)。このGo版は自前のOOXMLライターで全テキストを事前折り返しして
// wrap="none"で出力する設計(レンダラに依存しないようにするため)なので、
// "\n"を段落境界として扱う機能が必要。そのため wrapText() は "\n" で
// 段落分割してから、各段落に対して本物と同じトークン化・折り返しロジックを
// 適用する。
package main

import (
	"regexp"
	"strings"
)

const (
	lineHeightMult      = 1.35 // 行間。box_engine.py の LINE_HEIGHT_MULT と同じ。
	measureSafetyMargin = 1.05 // 実測値への安全マージン。box_engine.py の MEASURE_SAFETY_MARGIN と同じ。
	fallbackEmWidth     = 1.0  // テーブルに無い文字のフォールバック(全角相当)。
)

// 本物の _tokens() と同じトークン化: 英数字記号の塊はひとつのトークンとして
// 分割しない(単語の途中で折り返さない)。空白の連続もひとつのトークン。
// それ以外は1文字ずつトークン化する。
var tokenRe = regexp.MustCompile(`(?s)[A-Za-z0-9._%+\-]+|[ \t]+|.`)

func tokenizeLine(s string) []string {
	return tokenRe.FindAllString(s, -1)
}

// charEmWidth は1文字分の「フォントサイズ1ptあたりの幅(pt)」を返す。
// 改行は幅0とする(Pillowでの実測に相当する扱いが無いため明示的にゼロ扱いにする)。
func charEmWidth(ch rune, bold bool) float64 {
	if ch == '\n' {
		return 0
	}
	table := glyphWidthRegular
	if bold {
		table = glyphWidthBold
	}
	if w, ok := table[ch]; ok {
		return w
	}
	return fallbackEmWidth
}

// runWidthPt は文字列全体の幅(pt)を安全マージン込みで返す(トークン化せず、
// 本物の get_font().getlength(label) 相当の単純合計。凡例ラベルの幅計算に使う)。
func runWidthPt(s string, fontSizePt float64, bold bool) float64 {
	total := 0.0
	for _, ch := range s {
		total += charEmWidth(ch, bold) * fontSizePt
	}
	return total * measureSafetyMargin
}

// tokenWidthPt は1トークン分の幅(pt)。安全マージン込み。
// box_engine.py の wrap_text() 内の width_of() に相当。
func tokenWidthPt(tok string, fontSizePt float64, bold bool) float64 {
	total := 0.0
	for _, ch := range tok {
		total += charEmWidth(ch, bold) * fontSizePt
	}
	return total * measureSafetyMargin
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
	return total * s.FontSize * measureSafetyMargin
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
type TextRun struct {
	Text  string
	Color string
	Bold  bool
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
			total += charEmWidth(ch, r.Bold)
		}
	}
	return total * fontSizePt * measureSafetyMargin
}

// wrapRuns は wrapText と同じ規則(トークン単位・空白では折り返さない・"\n" は
// 段落の境目)で、装飾つきの文字を折り返す。太さは文字ごとに測る。
func wrapRuns(runs []TextRun, fontSizePt, maxWidthPt float64) [][]TextRun {
	rs, style := richRunes(runs)
	var lines [][2]int // [start, end) の文字の位置
	start := 0
	for paraEnd := 0; paraEnd <= len(rs); paraEnd++ {
		if paraEnd < len(rs) && rs[paraEnd] != '\n' {
			continue
		}
		pos := start
		lineStart, curW, curLen := start, 0.0, 0
		for _, tok := range tokenizeLine(string(rs[start:paraEnd])) {
			n := len([]rune(tok))
			tokW := 0.0
			for k := pos; k < pos+n; k++ {
				tokW += charEmWidth(rs[k], runs[style[k]].Bold) * fontSizePt
			}
			tokW *= measureSafetyMargin
			const wrapEps = 1e-6
			if curLen > 0 && curW+tokW > maxWidthPt+wrapEps && strings.TrimSpace(tok) != "" {
				lines = append(lines, [2]int{lineStart, pos})
				lineStart, curW, curLen = pos, 0, 0
			}
			curW += tokW
			curLen += n
			pos += n
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
			line = append(line, TextRun{Text: string(rs[k]), Color: r.Color, Bold: r.Bold})
		}
		out = append(out, line)
	}
	return out
}

// measureRunsHeight は折り返した行数から高さを出す。
func measureRunsHeight(runs []TextRun, fontSizePt, maxWidthPt float64) float64 {
	return float64(len(wrapRuns(runs, fontSizePt, maxWidthPt))) * fontSizePt * lineHeightMult
}
