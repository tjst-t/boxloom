// YAML の部分集合を読む(Boxloom Markdown の文書の頭と、サブシステムの
// コードブロックの中身に使う)。標準ライブラリだけで書く。
//
// 読めるもの: ブロックの対応表(key: value)・ブロックの列(- …)・流れの書き方
// ({a: b} と [a, b]、1行の中だけ)・引用なしの文字列・"…" と '…' の文字列・
// # のコメント。
// 誤りにするもの(docs/content-syntax.md 3.5): 参照(& *)・タグ(!)・複数文書
// (---)・ブロックの文字列(| >)・タブの字下げ・同じキーの重複・複数行に
// またがる値。
//
// 型の自動変換はしない。すべての値は文字列として持ち、何として読むか(日付・
// 真偽値など)は使う側が決める(YAML の「yes が真になる」「日付が日付型になる」
// 落とし穴を持ち込まないため)。

package boxloom

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// yNode は YAML の値。kind: 's' 文字列 / 'm' 対応表 / 'q' 列 / 'n' 空。
type yNode struct {
	kind      byte
	line, col int

	str    string
	quoted bool

	keys    []string
	keyLine []int
	vals    []*yNode

	items []*yNode
}

type yLine struct {
	no, indent int
	text       string
}

type yamlError struct {
	line, col int
	msg, hint string
}

type yParser struct {
	lines []yLine
	i     int
	errs  []yamlError
}

func (p *yParser) fail(line, col int, msg, hint string) {
	p.errs = append(p.errs, yamlError{line, col, msg, hint})
}

// parseYAML は src(firstLine はその1行目のファイルの中の行番号)を読む。
func parseYAML(src []string, firstLine int) (*yNode, []yamlError) {
	p := &yParser{}
	for k, raw := range src {
		no := firstLine + k
		if t := strings.TrimSpace(raw); t == "---" || t == "..." {
			p.fail(no, 1, "YAML の複数文書(--- や ...)は使えません", "1つのブロックには1つの対応表だけを書いてください")
			continue
		}
		ind := 0
		for ind < len(raw) && (raw[ind] == ' ' || raw[ind] == '\t') {
			if raw[ind] == '\t' {
				p.fail(no, ind+1, "字下げにタブは使えません", "空白で字下げしてください")
			}
			ind++
		}
		text := strings.TrimRight(stripYAMLComment(raw[ind:]), " \t")
		if text == "" {
			continue
		}
		p.lines = append(p.lines, yLine{no: no, indent: ind, text: text})
	}
	if len(p.lines) == 0 {
		return &yNode{kind: 'n', line: firstLine, col: 1}, p.errs
	}
	n := p.block(p.lines[0].indent)
	for p.i < len(p.lines) {
		l := p.lines[p.i]
		p.fail(l.no, l.indent+1, "字下げがずれています", "同じ階層のキーは同じ字下げで揃えてください")
		p.i++
	}
	return n, p.errs
}

// stripYAMLComment は引用の外の「行頭か空白の後ろの #」から後ろを外す。
func stripYAMLComment(s string) string {
	var q rune
	prevSpace := true
	for i, r := range s {
		switch {
		case q != 0:
			if r == q {
				q = 0
			}
		case (r == '"' || r == '\'') && prevSpace:
			q = r
		case r == '#' && prevSpace:
			return s[:i]
		}
		prevSpace = r == ' ' || r == '\t' || r == '{' || r == '[' || r == ','
	}
	return s
}

func isSeqItem(t string) bool { return t == "-" || strings.HasPrefix(t, "- ") }

// splitKey は「key: value」「key:」を分ける。key は引用なしか引用つき。
func splitKey(t string) (key, rest string, ok bool) {
	if t == "" || t[0] == '{' || t[0] == '[' || isSeqItem(t) {
		return "", "", false
	}
	if t[0] == '"' || t[0] == '\'' {
		s, n, err := readQuoted(t)
		if err != nil {
			return "", "", false
		}
		after := t[n:]
		if after == ":" {
			return s, "", true
		}
		if strings.HasPrefix(after, ": ") {
			return s, strings.TrimSpace(after[2:]), true
		}
		return "", "", false
	}
	if i := strings.Index(t, ": "); i >= 0 {
		return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+2:]), true
	}
	if strings.HasSuffix(t, ":") {
		return strings.TrimSpace(t[:len(t)-1]), "", true
	}
	return "", "", false
}

func (p *yParser) skipDeeper(ind int) {
	for p.i < len(p.lines) && p.lines[p.i].indent > ind {
		p.i++
	}
}

func (p *yParser) block(ind int) *yNode {
	l := p.lines[p.i]
	if isSeqItem(l.text) {
		return p.seq(ind)
	}
	if _, _, ok := splitKey(l.text); ok {
		return p.mapping(ind)
	}
	n := p.inlineValue(l.text, l.no, l.indent+1)
	p.i++
	if p.i < len(p.lines) && p.lines[p.i].indent > ind {
		nl := p.lines[p.i]
		p.fail(nl.no, nl.indent+1, "複数行にまたがる値は使えません", "1行で書くか、列(- …)にしてください")
		p.skipDeeper(ind)
	}
	return n
}

func (p *yParser) seq(ind int) *yNode {
	first := p.lines[p.i]
	n := &yNode{kind: 'q', line: first.no, col: ind + 1}
	for p.i < len(p.lines) && p.lines[p.i].indent == ind {
		l := p.lines[p.i]
		if !isSeqItem(l.text) {
			break // 同じ字下げの「キー: 値」は親の対応表の続き(親が確かめる)
		}
		rest := strings.TrimLeft(strings.TrimPrefix(l.text, "-"), " ")
		if rest == "" {
			p.i++
			if p.i < len(p.lines) && p.lines[p.i].indent > ind {
				n.items = append(n.items, p.block(p.lines[p.i].indent))
			} else {
				n.items = append(n.items, &yNode{kind: 'n', line: l.no, col: ind + 1})
			}
			continue
		}
		// 「- key: value」は、その位置から始まる対応表として読み直す。
		off := len(l.text) - len(rest)
		p.lines[p.i] = yLine{no: l.no, indent: ind + off, text: rest}
		n.items = append(n.items, p.block(ind+off))
	}
	return n
}

func (p *yParser) mapping(ind int) *yNode {
	first := p.lines[p.i]
	n := &yNode{kind: 'm', line: first.no, col: ind + 1}
	seen := map[string]bool{}
	for p.i < len(p.lines) && p.lines[p.i].indent == ind {
		l := p.lines[p.i]
		key, rest, ok := splitKey(l.text)
		if !ok {
			p.fail(l.no, ind+1, "「キー: 値」の形ではありません", "対応表の中の行は「キー: 値」で書いてください")
			p.i++
			p.skipDeeper(ind)
			continue
		}
		if seen[key] {
			p.fail(l.no, ind+1, fmt.Sprintf("キー %q が2回あります", key), "")
		}
		seen[key] = true
		var val *yNode
		if rest == "" {
			p.i++
			switch {
			case p.i < len(p.lines) && p.lines[p.i].indent > ind:
				val = p.block(p.lines[p.i].indent)
			case p.i < len(p.lines) && p.lines[p.i].indent == ind && isSeqItem(p.lines[p.i].text):
				val = p.seq(ind)
			default:
				val = &yNode{kind: 'n', line: l.no, col: ind + 1}
			}
			if p.i < len(p.lines) && p.lines[p.i].indent > ind {
				nl := p.lines[p.i]
				p.fail(nl.no, nl.indent+1, "字下げがずれています", "同じ階層の項目は同じ字下げで揃えてください")
				p.skipDeeper(ind)
			}
		} else {
			val = p.inlineValue(rest, l.no, ind+len(l.text)-len(rest)+1)
			p.i++
			if p.i < len(p.lines) && p.lines[p.i].indent > ind {
				nl := p.lines[p.i]
				p.fail(nl.no, nl.indent+1, "値のあとに、さらに字下げした行があります", "値を1行で書くか、キーの後ろを空けて次の行から字下げして書いてください")
				p.skipDeeper(ind)
			}
		}
		n.keys = append(n.keys, key)
		n.keyLine = append(n.keyLine, l.no)
		n.vals = append(n.vals, val)
	}
	return n
}

// inlineValue は1行に収まる値(流れの書き方・引用・引用なし)を読む。
func (p *yParser) inlineValue(s string, line, col int) *yNode {
	s = strings.TrimSpace(s)
	if bad := forbiddenYAMLStart(s); bad != "" {
		p.fail(line, col, bad, "ふつうの値として書いてください(記号から始めたいときは \"…\" で囲む)")
		return &yNode{kind: 's', line: line, col: col, str: s}
	}
	if s[0] == '{' || s[0] == '[' {
		f := &yFlow{p: p, r: []rune(s), line: line, col: col}
		n := f.value(false)
		if n == nil {
			return &yNode{kind: 'n', line: line, col: col}
		}
		f.skip()
		if f.i < len(f.r) {
			p.fail(line, f.at(), "値のあとに余計な文字があります", "")
		}
		return n
	}
	if s[0] == '"' || s[0] == '\'' {
		str, k, err := readQuoted(s)
		if err != nil {
			p.fail(line, col, err.Error(), "")
		} else if strings.TrimSpace(s[k:]) != "" {
			p.fail(line, col+utf8.RuneCountInString(s[:k]), "引用の後ろに余計な文字があります", "")
		}
		return &yNode{kind: 's', line: line, col: col, str: str, quoted: true}
	}
	if strings.Contains(s, ": ") {
		p.fail(line, col, "値の中に「: 」があります", "値を \"…\" で囲んでください")
	}
	return &yNode{kind: 's', line: line, col: col, str: s}
}

func forbiddenYAMLStart(s string) string {
	switch {
	case s == "":
		return ""
	case s[0] == '&' || s[0] == '*':
		return "YAML の参照(& と *)は使えません"
	case s[0] == '!':
		return "YAML のタグ(!)は使えません"
	case s[0] == '|' || s[0] == '>':
		return "YAML のブロックの文字列(| と >)は使えません"
	case s[0] == '@' || s[0] == '`' || s[0] == '%':
		return "値を " + s[:1] + " で始めることはできません"
	}
	return ""
}

// readQuoted は先頭の "…" か '…' を読み、中身と読んだバイト数を返す。
func readQuoted(s string) (string, int, error) {
	q := s[0]
	var b strings.Builder
	for i := 1; i < len(s); {
		c := s[i]
		if q == '\'' {
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					b.WriteByte('\'')
					i += 2
					continue
				}
				return b.String(), i + 1, nil
			}
			b.WriteByte(c)
			i++
			continue
		}
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			if i+1 >= len(s) {
				return "", 0, fmt.Errorf("引用の中の \\ のあとに文字がありません")
			}
			switch e := s[i+1]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'u':
				if i+6 > len(s) {
					return "", 0, fmt.Errorf("\\u のあとに4桁の16進が要ります")
				}
				v, err := strconv.ParseUint(s[i+2:i+6], 16, 32)
				if err != nil {
					return "", 0, fmt.Errorf("\\u のあとに4桁の16進が要ります")
				}
				b.WriteRune(rune(v))
				i += 4
			default:
				return "", 0, fmt.Errorf("引用の中の \\%c は使えません", e)
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", 0, fmt.Errorf("引用(%c)が閉じていません", q)
}

// yFlow は1行の中の流れの書き方({…} と […])を読む。
type yFlow struct {
	p         *yParser
	r         []rune
	i         int
	line, col int
}

func (f *yFlow) skip() {
	for f.i < len(f.r) && (f.r[f.i] == ' ' || f.r[f.i] == '\t') {
		f.i++
	}
}

func (f *yFlow) at() int { return f.col + f.i }

// value は値を1つ読む。isKey なら ':' でも止まる。誤りなら nil。
func (f *yFlow) value(isKey bool) *yNode {
	f.skip()
	if f.i >= len(f.r) {
		f.p.fail(f.line, f.at(), "値がありません", "")
		return nil
	}
	start := f.at()
	switch c := f.r[f.i]; c {
	case '{':
		n := &yNode{kind: 'm', line: f.line, col: start}
		f.i++
		seen := map[string]bool{}
		for {
			f.skip()
			if f.i < len(f.r) && f.r[f.i] == '}' {
				f.i++
				return n
			}
			kcol := f.at()
			k := f.value(true)
			if k == nil {
				return nil
			}
			f.skip()
			if f.i >= len(f.r) || f.r[f.i] != ':' {
				f.p.fail(f.line, f.at(), "{…} の中は「キー: 値」で書いてください", "")
				return nil
			}
			f.i++
			if k.kind != 's' {
				f.p.fail(f.line, kcol, "キーは文字列にしてください", "")
				return nil
			}
			if seen[k.str] {
				f.p.fail(f.line, kcol, fmt.Sprintf("キー %q が2回あります", k.str), "")
			}
			seen[k.str] = true
			v := f.value(false)
			if v == nil {
				return nil
			}
			n.keys = append(n.keys, k.str)
			n.keyLine = append(n.keyLine, f.line)
			n.vals = append(n.vals, v)
			f.skip()
			if f.i < len(f.r) && f.r[f.i] == ',' {
				f.i++
				continue
			}
			if f.i < len(f.r) && f.r[f.i] == '}' {
				f.i++
				return n
			}
			f.p.fail(f.line, f.at(), "{…} が閉じていません", "1行の中で } まで書いてください(複数行にまたがる {…} は使えません)")
			return nil
		}
	case '[':
		n := &yNode{kind: 'q', line: f.line, col: start}
		f.i++
		for {
			f.skip()
			if f.i < len(f.r) && f.r[f.i] == ']' {
				f.i++
				return n
			}
			v := f.value(false)
			if v == nil {
				return nil
			}
			n.items = append(n.items, v)
			f.skip()
			if f.i < len(f.r) && f.r[f.i] == ',' {
				f.i++
				continue
			}
			if f.i < len(f.r) && f.r[f.i] == ']' {
				f.i++
				return n
			}
			f.p.fail(f.line, f.at(), "[…] が閉じていません", "1行の中で ] まで書いてください(複数行にまたがる […] は使えません)")
			return nil
		}
	case '"', '\'':
		rest := string(f.r[f.i:])
		s, k, err := readQuoted(rest)
		if err != nil {
			f.p.fail(f.line, start, err.Error(), "")
			return nil
		}
		f.i += utf8.RuneCountInString(rest[:k])
		return &yNode{kind: 's', line: f.line, col: start, str: s, quoted: true}
	default:
		if bad := forbiddenYAMLStart(string(c)); bad != "" {
			f.p.fail(f.line, start, bad, "")
			return nil
		}
		j := f.i
		for j < len(f.r) {
			ch := f.r[j]
			if ch == ',' || ch == '}' || ch == ']' || (isKey && ch == ':') {
				break
			}
			j++
		}
		s := strings.TrimSpace(string(f.r[f.i:j]))
		f.i = j
		if s == "" {
			return &yNode{kind: 'n', line: f.line, col: start}
		}
		return &yNode{kind: 's', line: f.line, col: start, str: s}
	}
}
