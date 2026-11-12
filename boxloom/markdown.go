// Boxloom Markdown(docs/content-syntax.md)を読んで IR(docs/ir.md)にする。
//
// 方針は「厳格な Markdown」: 意味の決まらない書き方は黙って通さず、
// ファイル・行・列・IR の中の場所・直し方つきの誤りにする。誤りは最初の1件で
// 止めず、読めるところまで全部集める。
//
// 流れ: 行を読んで「文書の頭 / スライド / 段落・箇条・囲み・コードブロック」の
// 木を作る(parseBlocks)→ 木を IR の JSON に変える(toIR)。サブシステムの
// コードブロックの中身は yaml.go で読み、md_gantt.go が IR の data に変える。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Issue は誤り・警告の1件(docs/content-syntax.md の5章の形)。Markdown の
// パーサー・boxloom check・tools/validate_ir.py が同じ形で出す。
//   - line・column は Markdown の中の場所。IR(JSON)を直接検査したときは無い
//     (0 なら JSON に出さない)。column は無くて line だけのこともある。
//   - path は IR の中の場所(JSON Pointer)。"" は文書全体。
//   - code は決まった一覧(5章・8章)。AI はこれで分岐できる。
type Issue struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Path     string   `json:"path"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Expected []string `json:"expected,omitempty"`
	Hint     string   `json:"hint,omitempty"`
}

func (e Issue) String() string {
	loc := e.File
	switch {
	case e.Line > 0 && e.Column > 0:
		loc = fmt.Sprintf("%s:%d:%d", e.File, e.Line, e.Column)
	case e.Line > 0:
		loc = fmt.Sprintf("%s:%d", e.File, e.Line)
	}
	if e.Path != "" {
		loc += " " + e.Path
	}
	s := fmt.Sprintf("%s: %s: %s", loc, e.Code, e.Message)
	if len(e.Expected) > 0 {
		s += " (使えるもの: " + strings.Join(e.Expected, ", ") + ")"
	}
	if e.Hint != "" {
		s += " — " + e.Hint
	}
	return s
}

// 誤りの code の一覧(AI はこれで分岐できる)。
const (
	codeUnknownBlock      = "unknown-block"      // 知らない囲み
	codeUnknownAttribute  = "unknown-attribute"  // 知らない属性
	codeRawHTML           = "raw-html"           // 生の HTML
	codeUnsupportedSyntax = "unsupported-syntax" // 対応しない Markdown の構文
	codePlannedSyntax     = "planned-syntax"     // 仕様で「予定」(IR に入るまでは誤り)
	codeUnclosedBlock     = "unclosed-block"     // 囲み・コードブロック・強調の閉じ忘れ
	codeMisplaced         = "misplaced-content"  // その場所には書けないもの
	codeUnknownKey        = "unknown-key"        // YAML の知らないキー
	codeMissingKey        = "missing-key"        // YAML の必須のキーが無い
	codeInvalidValue      = "invalid-value"      // 値の形が違う
	codeYAML              = "yaml"               // YAML の部分集合の外
	codeSchema            = "schema"             // できた IR が通らない
	codeTableShape        = "table-shape"        // 表の行のセルの数が見出しと違う
	codeTableGroups       = "table-groups"       // 表の行の群の形が違う(最初の行の群の名前が空など)
	codeTablePoints       = "table-points"       // 表のセルの箇条の書き方が違う(知らない列名・空の項目)
	codeImagePath         = "image-path"         // 画像の src が絶対パス・URL・文書のフォルダの外
	codeImageAlt          = "image-alt"          // 画像の代わりの文が無い
)

// --- 出力: キーの順を保つ JSON の object ---

type jobj struct {
	keys []string
	vals []any
}

func (o *jobj) set(k string, v any) *jobj {
	for i, kk := range o.keys {
		if kk == k {
			o.vals[i] = v
			return o
		}
	}
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

func obj(kv ...any) *jobj {
	o := &jobj{}
	for i := 0; i+1 < len(kv); i += 2 {
		o.set(kv[i].(string), kv[i+1])
	}
	return o
}

func (o *jobj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := marshalNoEscape(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// --- 読んだ結果の木 ---

type srcPos struct{ line, col int }

// mdText は行をまたいでつないだ文と、1文字ごとの元の場所。
type mdText struct {
	r   []rune
	pos []srcPos
}

const brRune = '\uE000' // 行末の \ (意味のある改行)の印

type mdItemKind int

const (
	mdPara mdItemKind = iota
	mdList
	mdFence
	mdCode
	mdTable
)

type mdItem struct {
	kind mdItemKind
	line int

	text  mdText       // mdPara
	items []*mdListEnt // mdList
	fence *mdFenceBlk  // mdFence

	lang, info string   // mdCode
	code       []string // mdCode: 中身の行
	codeLine   int      // mdCode: 中身の1行目の行番号

	table *mdTableBlk // mdTable
}

type mdListEnt struct {
	line, indent int
	text         mdText
	children     []*mdListEnt
}

type mdFenceBlk struct {
	colons   int
	name     string
	line     int
	label    mdText
	attrs    string
	attrsCol int
	items    []*mdItem
	closed   bool
}

type mdSlide struct {
	line     int
	title    mdText
	attrs    string
	attrsCol int
	items    []*mdItem
}

type mdDoc struct {
	fm         *yNode
	slides     []*mdSlide
	coverNotes []*mdFenceBlk // 最初のスライドより前の :::notes(表紙のノート)
}

// --- 行ごとの読み取り ---

type mdParser struct {
	file  string
	errs  []Issue
	lines map[string]int // IR の場所(JSON Pointer)→ Markdown の行
}

// mark は IR の場所が Markdown の何行目から来たかを覚える(boxloom check が
// IR に対する誤りを Markdown の行に戻すのに使う)。最初に覚えた行を残す。
func (p *mdParser) mark(path string, line int) {
	if line <= 0 {
		return
	}
	if p.lines == nil {
		p.lines = map[string]int{}
	}
	if _, ok := p.lines[path]; !ok {
		p.lines[path] = line
	}
}

// SourceMap は IR の場所 → Markdown の行。
type SourceMap map[string]int

// Line は path か、その一番近い親の行(無ければ 0)。
func (m SourceMap) Line(path string) int {
	for {
		if l, ok := m[path]; ok {
			return l
		}
		k := strings.LastIndex(path, "/")
		if k < 0 {
			return 0
		}
		path = path[:k]
	}
}

func (p *mdParser) fail(line, col int, path, code, msg string, expected []string, hint string) {
	p.errs = append(p.errs, Issue{File: p.file, Line: line, Column: col, Path: path, Code: code, Message: msg, Expected: expected, Hint: hint})
}

var (
	reHeading  = regexp.MustCompile(`^(#{1,6})(?:[ \t]+(.*?))?[ \t]*#*[ \t]*$`)
	reFence    = regexp.MustCompile(`^(:{3,})[ \t]*(.*)$`)
	reCode     = regexp.MustCompile("^(`{3,}|~{3,})[ \\t]*(.*)$")
	reBullet   = regexp.MustCompile(`^([-*+]|\d{1,9}[.)])([ \t]+(.*))?$`)
	reThematic = regexp.MustCompile(`^(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	reSetext   = regexp.MustCompile(`^=+[ \t]*$`)
	reAttrs    = regexp.MustCompile(`[ \t]*\{([^{}]*)\}[ \t]*$`)
	reName     = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	reID       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	reDate     = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])(-(0[1-9]|[12][0-9]|3[01]))?$`)
	reHTMLLine = regexp.MustCompile(`^<[A-Za-z/!?]`)
)

// 囲みの名前。
var (
	plannedFences = map[string]bool{}
)

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

// lineText は1行を mdText にする(col0 はその行の中の始まりの列、1から)。
func lineText(s string, line, col0 int) mdText {
	var t mdText
	col := col0
	for _, r := range s {
		t.r = append(t.r, r)
		t.pos = append(t.pos, srcPos{line, col})
		col++
	}
	return t
}

// appendLine は段落の次の行をつなぐ。日本語どうしは空白を入れずにつなぎ、
// 英数字どうしは空白1つでつなぐ(ふつうの Markdown は常に空白になり、
// 日本語の文の途中に空白が入ってしまうため)。行末の \ は意味のある改行。
func (t *mdText) appendLine(s string, line, col0 int) {
	s = strings.TrimRight(s, " \t")
	hard := false
	if strings.HasSuffix(s, "\\") && !strings.HasSuffix(s, "\\\\") {
		s = strings.TrimRight(strings.TrimSuffix(s, "\\"), " \t")
		hard = true
	}
	if len(t.r) > 0 {
		last := t.r[len(t.r)-1]
		first, _ := utf8.DecodeRuneInString(s)
		if last != brRune && last < 0x80 && first < 0x80 && s != "" {
			t.r = append(t.r, ' ')
			t.pos = append(t.pos, srcPos{line, col0})
		}
	}
	nt := lineText(s, line, col0)
	t.r = append(t.r, nt.r...)
	t.pos = append(t.pos, nt.pos...)
	if hard {
		t.r = append(t.r, brRune)
		t.pos = append(t.pos, srcPos{line, col0 + utf8.RuneCountInString(s)})
	}
}

func (t mdText) trimBR() mdText {
	for len(t.r) > 0 && t.r[len(t.r)-1] == brRune {
		t.r, t.pos = t.r[:len(t.r)-1], t.pos[:len(t.pos)-1]
	}
	return t
}

// parseBlocks は文書を行ごとに読んで木にする。
func (p *mdParser) parseBlocks(src string) *mdDoc {
	src = strings.TrimPrefix(src, "\uFEFF")
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	doc := &mdDoc{}
	i := 0

	// 文書の頭(front matter)
	if len(lines) > 0 && strings.TrimRight(lines[0], " \t") == "---" {
		end := -1
		for k := 1; k < len(lines); k++ {
			if strings.TrimRight(lines[k], " \t") == "---" {
				end = k
				break
			}
		}
		if end < 0 {
			p.fail(1, 1, "", codeUnclosedBlock, "文書の頭(---)が閉じていません", nil, "文書の頭の終わりにも --- の行を置いてください")
			return doc
		}
		node, yerrs := parseYAML(lines[1:end], 2)
		for _, e := range yerrs {
			p.fail(e.line, e.col, "/meta", codeYAML, e.msg, nil, e.hint)
		}
		doc.fm = node
		i = end + 1
	}

	var slide *mdSlide
	var stack []*mdFenceBlk
	var para *mdItem
	var list *mdItem
	blankSinceList := false
	orphanReported := false

	container := func() *[]*mdItem {
		if len(stack) > 0 {
			return &stack[len(stack)-1].items
		}
		if slide != nil {
			return &slide.items
		}
		return nil
	}
	add := func(it *mdItem) {
		c := container()
		if c == nil && it.kind == mdFence && it.fence.name == "notes" {
			doc.coverNotes = append(doc.coverNotes, it.fence)
			return
		}
		if c == nil && it.kind == mdFence && it.fence.name == "lead" {
			p.fail(it.line, 1, "", codeMisplaced, "最初のスライド(# 題)より前に :::lead は書けません", nil, "リード文はそのスライドの「# 題」の行の後ろに置いてください")
			return
		}
		if c == nil && it.kind == mdFence && it.fence.name == "source" {
			p.fail(it.line, 1, "", codeMisplaced, "最初のスライド(# 題)より前に :::source は書けません(表紙は出典を持ちません)", nil, "出典はそれを示すスライドの「# 題」の行の後ろに置いてください")
			return
		}
		if c == nil {
			if !orphanReported {
				p.fail(it.line, 1, "", codeMisplaced, "最初のスライド(# 題)より前に本文があります", nil, "本文の前に「# 題」の行を置いてください")
				orphanReported = true
			}
			return
		}
		*c = append(*c, it)
	}
	flush := func() {
		if para != nil {
			add(para)
			para = nil
		}
		if list != nil {
			add(list)
			list = nil
		}
		blankSinceList = false
	}
	closeAllFences := func(atLine int, why string) {
		for k := len(stack) - 1; k >= 0; k-- {
			f := stack[k]
			p.fail(f.line, 1, "", codeUnclosedBlock,
				fmt.Sprintf("囲み :::%s(%d 行目)が閉じていません%s", f.name, f.line, why), nil,
				fmt.Sprintf("中身の終わりに %s の行を置いてください", strings.Repeat(":", f.colons)))
		}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			add(&mdItem{kind: mdFence, line: f.line, fence: f})
		}
	}

	for ; i < len(lines); i++ {
		raw := strings.TrimRight(lines[i], " \t")
		no := i + 1
		if strings.TrimSpace(raw) == "" {
			if para != nil {
				add(para)
				para = nil
			}
			if list != nil {
				blankSinceList = true
			}
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(raw, " "), "\t") || strings.HasPrefix(raw, "\t") {
			p.fail(no, 1, "", codeUnsupportedSyntax, "行頭にタブがあります", nil, "字下げは空白で書いてください")
			raw = strings.ReplaceAll(raw, "\t", "    ")
		}
		ind := leadingSpaces(raw)
		t := raw[ind:]

		// 箇条の続き(字下げした次の項目・前の項目の続きの行)
		if list != nil && ind >= 2 && !reBullet.MatchString(t) && !blankSinceList && para == nil {
			last := lastListEnt(list.items)
			last.text.appendLine(t, no, ind+1)
			continue
		}

		switch {
		case reCode.MatchString(t) && ind < 4:
			flush()
			m := reCode.FindStringSubmatch(t)
			fenceStr := m[1]
			if fenceStr[0] == '~' {
				p.fail(no, ind+1, "", codeUnsupportedSyntax, "~~~ のコードブロックは使えません", nil, "``` で囲んでください")
			}
			it := &mdItem{kind: mdCode, line: no, info: strings.TrimSpace(m[2]), codeLine: no + 1}
			closed := false
			for i++; i < len(lines); i++ {
				l := strings.TrimRight(lines[i], " \t")
				if strings.HasPrefix(strings.TrimLeft(l, " "), fenceStr) && strings.Trim(strings.TrimLeft(l, " "), fenceStr[:1]) == "" {
					closed = true
					break
				}
				it.code = append(it.code, lines[i])
			}
			if !closed {
				p.fail(no, ind+1, "", codeUnclosedBlock, fmt.Sprintf("コードブロック(%d 行目)が閉じていません", no), nil, "中身の終わりに "+fenceStr+" の行を置いてください")
			}
			add(it)

		case reHeading.MatchString(t) && ind < 4:
			flush()
			m := reHeading.FindStringSubmatch(t)
			level := len(m[1])
			if level >= 2 {
				if level == 2 {
					p.fail(no, ind+1, "", codeUnsupportedSyntax, "小見出し(##)は使えません(IR は小見出しを持たない)", nil, "スライドを分けるなら # を、まとまりの題なら囲みの見出しの文(:::名前 見出し)を、強調したいなら **…** を使ってください")
				} else {
					p.fail(no, ind+1, "", codeUnsupportedSyntax, fmt.Sprintf("見出しの段(%s)は使えません", m[1]), []string{"#"}, "スライドの題は # で書きます")
				}
				continue
			}
			if len(stack) > 0 {
				closeAllFences(no, "(次のスライドが始まりました)")
			}
			title := m[2]
			s := &mdSlide{line: no}
			if am := reAttrs.FindStringSubmatchIndex(title); am != nil {
				s.attrs = title[am[2]:am[3]]
				s.attrsCol = ind + len(m[1]) + 2 + utf8.RuneCountInString(title[:am[2]])
				title = title[:am[0]]
			}
			title = strings.TrimSpace(title)
			if title == "" {
				p.fail(no, ind+1, "", codeMisplaced, "スライドの題がありません", nil, "「# 題」のように # の後ろに題を書いてください")
			}
			col := ind + len(m[1]) + 1 + (len(t) - len(m[1]) - len(strings.TrimLeft(t[len(m[1]):], " \t")))
			s.title = lineText(title, no, col)
			slide = s
			doc.slides = append(doc.slides, s)

		case reFence.MatchString(t):
			flush()
			m := reFence.FindStringSubmatch(t)
			colons := len(m[1])
			rest := strings.TrimSpace(m[2])
			if rest == "" {
				// 閉じ
				if len(stack) == 0 {
					p.fail(no, ind+1, "", codeUnclosedBlock, "閉じる囲みがありません", nil, "対応する :::名前 の行があるか確かめてください")
					continue
				}
				top := stack[len(stack)-1]
				if top.colons != colons {
					p.fail(no, ind+1, "", codeUnclosedBlock,
						fmt.Sprintf("閉じのコロンの数(%d)が、開いた :::%s(%d 行目、%d 個)と合いません", colons, top.name, top.line, top.colons), nil,
						fmt.Sprintf("%s で閉じてください", strings.Repeat(":", top.colons)))
				}
				top.closed = true
				stack = stack[:len(stack)-1]
				add(&mdItem{kind: mdFence, line: top.line, fence: top})
				continue
			}
			name := rest
			label := ""
			if k := strings.IndexAny(rest, " \t{"); k >= 0 {
				name, label = rest[:k], rest[k:]
			}
			f := &mdFenceBlk{colons: colons, name: name, line: no}
			if am := reAttrs.FindStringSubmatchIndex(label); am != nil {
				f.attrs = label[am[2]:am[3]]
				f.attrsCol = ind + colons + 1 + utf8.RuneCountInString(m[2][:len(m[2])-len(rest)]) + utf8.RuneCountInString(name) + utf8.RuneCountInString(label[:am[2]])
				label = label[:am[0]]
			}
			labelTrim := strings.TrimSpace(label)
			lcol := ind + colons + 1 + utf8.RuneCountInString(name) + utf8.RuneCountInString(label) - utf8.RuneCountInString(strings.TrimLeft(label, " \t")) + 1
			f.label = lineText(labelTrim, no, lcol)
			if len(stack) > 0 {
				outer := stack[len(stack)-1]
				if colons >= outer.colons {
					p.fail(no, ind+1, "", codeUnclosedBlock,
						fmt.Sprintf("囲み :::%s の中に入れる囲みは、外側(%d 個)よりコロンを少なくしてください", outer.name, outer.colons), nil,
						fmt.Sprintf("外側の囲みを %s%s にしてください", strings.Repeat(":", colons+1), outer.name))
				}
				if len(stack) >= 3 {
					p.fail(no, ind+1, "", codeMisplaced, "囲みの入れ子は3段までです", nil, "")
				}
			}
			stack = append(stack, f)

		case reThematic.MatchString(t) && ind < 4:
			flush()
			p.fail(no, ind+1, "", codeUnsupportedSyntax, "水平線(---)は使えません。スライドの区切りでもありません", nil, "新しいスライドは「# 題」で始めてください")

		case reSetext.MatchString(t) && para != nil:
			p.fail(no, ind+1, "", codeUnsupportedSyntax, "=== の下線の見出しは使えません", nil, "スライドの題は「# 題」で書いてください")
			para = nil

		case reBullet.MatchString(t) && (ind < 4 || list != nil):
			m := reBullet.FindStringSubmatch(t)
			if m[1] == "*" || m[1] == "+" {
				p.fail(no, ind+1, "", codeUnsupportedSyntax, fmt.Sprintf("箇条の印 %s は使えません", m[1]), []string{"-", "1."}, "箇条は「- 」で書いてください")
			}
			if strings.TrimSpace(m[3]) == "" {
				p.fail(no, ind+1, "", codeMisplaced, "箇条の項目が空です", nil, "")
			}
			if para != nil {
				add(para)
				para = nil
			}
			if list == nil {
				list = &mdItem{kind: mdList, line: no}
			}
			blankSinceList = false
			ent := &mdListEnt{line: no, indent: ind}
			ent.text.appendLine(strings.TrimSpace(m[3]), no, ind+len(m[1])+2)
			p.placeListEnt(list, ent)

		case strings.HasPrefix(t, ">"):
			flush()
			hint := "他人の言葉なら :::quote 出どころ … :::、注記なら :::note … ::: で囲んでください"
			p.fail(no, ind+1, "", codeUnsupportedSyntax, "引用(>)は使えません", []string{"quote", "note"}, hint)

		case strings.HasPrefix(t, "|") && ind < 4:
			flush()
			tb := &mdTableBlk{}
			for ; i < len(lines); i++ {
				l := strings.TrimRight(lines[i], " \t")
				li := leadingSpaces(l)
				if !strings.HasPrefix(l[li:], "|") {
					break
				}
				tb.rows = append(tb.rows, mdTableLine{text: l[li:], line: i + 1, col: li + 1})
			}
			// 表のすぐ次の行の「: 説明」は表の説明の文(caption)。
			if i < len(lines) {
				l := strings.TrimRight(lines[i], " \t")
				if li := leadingSpaces(l); strings.HasPrefix(l[li:], ": ") {
					text := ""
					if len(l) > li+2 {
						text = l[li+2:]
					}
					// 行末の {…} は表の属性(total=last など)。
					if am := reAttrs.FindStringSubmatchIndex(text); am != nil {
						tb.attrs = text[am[2]:am[3]]
						tb.attrsCol = li + 3 + utf8.RuneCountInString(text[:am[2]])
						text = text[:am[0]]
					}
					lead := len(text) - len(strings.TrimLeft(text, " \t"))
					tb.caption = lineText(strings.TrimSpace(text), i+1, li+3+lead)
					tb.captionLine = i + 1
					i++
				}
			}
			i--
			add(&mdItem{kind: mdTable, line: no, table: tb})

		case strings.HasPrefix(t, "[^"):
			flush()
			p.fail(no, ind+1, "", codeUnsupportedSyntax, "脚注は使えません", nil, "補足は :::note … ::: で書いてください")

		case reHTMLLine.MatchString(t):
			flush()
			p.fail(no, ind+1, "", codeRawHTML, "生の HTML は使えません", nil, "見た目はテンプレートが決めます。意味を表す書き方(**強調** や :::note)を使ってください")

		default:
			if ind >= 4 && para == nil {
				flush()
				p.fail(no, ind+1, "", codeUnsupportedSyntax, "字下げしたコードは使えません", nil, "字下げを外してください")
				continue
			}
			if list != nil {
				if blankSinceList && ind >= 2 {
					p.fail(no, ind+1, "", codeUnsupportedSyntax, "箇条の項目の中に段落は書けません", nil, "空行を外して前の項目の続きにするか、改行したいなら行末に \\ を付けてください")
					continue
				}
				if !blankSinceList {
					// 字下げの無い続きの行(lazy continuation)
					lastListEnt(list.items).text.appendLine(t, no, ind+1)
					continue
				}
				add(list)
				list = nil
				blankSinceList = false
			}
			if para == nil {
				para = &mdItem{kind: mdPara, line: no}
			}
			para.text.appendLine(t, no, ind+1)
		}
	}
	flush()
	if len(stack) > 0 {
		closeAllFences(len(lines), "")
	}
	return doc
}

func lastListEnt(items []*mdListEnt) *mdListEnt {
	e := items[len(items)-1]
	for len(e.children) > 0 {
		e = e.children[len(e.children)-1]
	}
	return e
}

// placeListEnt は字下げで項目の段を決める。1段目より深ければ2段目。
// 3段目は誤り(仕様は2段まで)。
func (p *mdParser) placeListEnt(list *mdItem, ent *mdListEnt) {
	if len(list.items) == 0 || ent.indent <= list.items[0].indent {
		ent.indent = 0
		list.items = append(list.items, ent)
		return
	}
	parent := list.items[len(list.items)-1]
	if len(parent.children) > 0 && ent.indent > parent.children[0].indent {
		p.fail(ent.line, ent.indent+1, "", codeMisplaced, "箇条の入れ子は2段までです", nil, "")
		parent.children = append(parent.children, ent)
		return
	}
	if ent.indent < 2 {
		p.fail(ent.line, ent.indent+1, "", codeMisplaced, "入れ子の箇条は2つ以上の空白で字下げしてください", nil, "")
	}
	parent.children = append(parent.children, ent)
}

// --- 属性 ---

// parseAttrs は {variant=dense} の中身を読む。allowed にない名前は誤り。
func (p *mdParser) parseAttrs(s string, line, col int, path string, allowed []string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(s) {
		k, v, ok := strings.Cut(tok, "=")
		v = strings.Trim(v, `"'`)
		switch {
		case strings.HasPrefix(tok, "#") || strings.HasPrefix(tok, "."):
			p.fail(line, col, path, codeUnknownAttribute, fmt.Sprintf("属性 %s の書き方は使えません", tok), allowed, "変種は {variant=名前} で選びます")
			continue
		case !ok:
			p.fail(line, col, path, codeUnknownAttribute, fmt.Sprintf("属性 %s に値がありません", tok), allowed, "{名前=値} の形で書いてください")
			continue
		}
		known := false
		for _, a := range allowed {
			if a == k {
				known = true
			}
		}
		if !known {
			hint := "見た目はテンプレートが決めます。変えたいときはテンプレートの variant を選んでください"
			if k == "id" {
				p.fail(line, col, path, codePlannedSyntax, "属性 id はまだ使えません(予定)", allowed, "")
				continue
			}
			p.fail(line, col, path, codeUnknownAttribute, fmt.Sprintf("属性 %s はありません", k), allowed, hint)
			continue
		}
		if k == "variant" && !reName.MatchString(v) {
			p.fail(line, col, path, codeInvalidValue, fmt.Sprintf("変種の名前 %q の形が違います", v), nil, "英小文字・数字・ハイフンで書いてください(例 dense)")
			continue
		}
		out[k] = v
	}
	return out
}

// --- 文中の記法(インライン) ---

type inlineParser struct {
	p    *mdParser
	t    mdText
	i    int
	path string
}

func (x *inlineParser) posAt(i int) srcPos {
	if len(x.t.pos) == 0 {
		return srcPos{0, 0}
	}
	if i >= len(x.t.pos) {
		i = len(x.t.pos) - 1
	}
	return x.t.pos[i]
}

func (x *inlineParser) fail(i int, code, msg, hint string) {
	ps := x.posAt(i)
	x.p.fail(ps.line, ps.col, x.path, code, msg, nil, hint)
}

// parseInline は文を IR のインライン(文字列か、文字列と要素の並び)にする。
func (p *mdParser) parseInline(t mdText, path string) any {
	x := &inlineParser{p: p, t: t, path: path}
	parts, _ := x.parse(false)
	return normalizeInline(parts)
}

func normalizeInline(parts []any) any {
	var out []any
	for _, v := range parts {
		if s, ok := v.(string); ok && len(out) > 0 {
			if prev, ok := out[len(out)-1].(string); ok {
				out[len(out)-1] = prev + s
				continue
			}
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return ""
	}
	if len(out) == 1 {
		if s, ok := out[0].(string); ok {
			return s
		}
	}
	return out
}

func isASCIIPunct(r rune) bool {
	return r < 0x80 && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r)
}

// parse は inEm なら閉じの ** で戻る(closed=true)。
func (x *inlineParser) parse(inEm bool) (parts []any, closed bool) {
	var buf []rune
	flushText := func() {
		if len(buf) > 0 {
			parts = append(parts, string(buf))
			buf = nil
		}
	}
	r := x.t.r
	for x.i < len(r) {
		c := r[x.i]
		next := rune(0)
		if x.i+1 < len(r) {
			next = r[x.i+1]
		}
		switch {
		case c == brRune:
			flushText()
			parts = append(parts, obj("type", "br"))
			x.i++
		case c == '\\' && isASCIIPunct(next):
			buf = append(buf, next)
			x.i += 2
		case c == '`':
			n := 0
			for x.i+n < len(r) && r[x.i+n] == '`' {
				n++
			}
			end := -1
			for k := x.i + n; k+n <= len(r); k++ {
				if string(r[k:k+n]) == strings.Repeat("`", n) && (k+n == len(r) || r[k+n] != '`') && (k == 0 || r[k-1] != '`') {
					end = k
					break
				}
			}
			if end < 0 {
				x.fail(x.i, codeUnclosedBlock, "` が閉じていません", "コードの終わりにも ` を置いてください")
				buf = append(buf, r[x.i:x.i+n]...)
				x.i += n
				continue
			}
			code := string(r[x.i+n : end])
			if len(code) >= 2 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.TrimSpace(code) != "" {
				code = code[1 : len(code)-1]
			}
			flushText()
			parts = append(parts, obj("type", "code", "text", code))
			x.i = end + n
		case c == '*' && next == '*':
			if inEm {
				flushText()
				x.i += 2
				return parts, true
			}
			start := x.i
			x.i += 2
			inner, ok := x.parse(true)
			if !ok {
				x.fail(start, codeUnclosedBlock, "強調(**)が閉じていません", "強調の終わりにも ** を置いてください")
				buf = append(buf, '*', '*')
				flushText()
				parts = append(parts, inner...)
				continue
			}
			if len(inner) == 0 {
				x.fail(start, codeMisplaced, "強調(**)の中が空です", "")
				continue
			}
			flushText()
			parts = append(parts, obj("type", "em", "content", normalizeInline(inner)))
		case c == '*':
			x.fail(x.i, codeUnsupportedSyntax, "斜体(*…*)は使えません。強調は1種類です", "強調したいなら **…** を、文字の * なら \\* と書いてください")
			x.i++
		case c == '~' && next == '~':
			x.fail(x.i, codeUnsupportedSyntax, "取り消し線(~~)は使えません", "")
			x.i += 2
		case c == '!' && next == '[':
			x.fail(x.i, codeMisplaced, "画像は文の中には書けません", "画像だけの段落にしてください(1枚なら image、2〜6枚を並べると gallery)")
			x.i += 2
		case c == '[' && next == '^':
			x.fail(x.i, codeUnsupportedSyntax, "脚注は使えません", "補足は :::note … ::: で書いてください")
			x.i += 2
		case c == '[':
			if link, n := x.tryLink(); link != nil {
				flushText()
				parts = append(parts, link)
				x.i += n
				continue
			}
			buf = append(buf, c)
			x.i++
		case c == '<' && (next == '/' || next == '!' || next == '?' || (next < 0x80 && (next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z'))):
			end := x.i
			for end < len(r) && r[end] != '>' {
				end++
			}
			hint := "見た目はテンプレートが決めます。改行は行末の \\、強調は **…** で書いてください"
			if next != '/' && strings.Contains(string(r[x.i:min(end, len(r))]), "://") {
				hint = "リンクは [文](URL) で書いてください"
			}
			x.fail(x.i, codeRawHTML, "生の HTML は使えません", hint)
			x.i = min(end+1, len(r))
		default:
			buf = append(buf, c)
			x.i++
		}
	}
	flushText()
	return parts, false
}

// tryLink は x.i の [ から [文](URL) を読む。リンクでなければ nil。
func (x *inlineParser) tryLink() (*jobj, int) {
	r := x.t.r
	depth := 0
	close := -1
	for k := x.i; k < len(r); k++ {
		if r[k] == '\\' {
			k++
			continue
		}
		if r[k] == '[' {
			depth++
		}
		if r[k] == ']' {
			depth--
			if depth == 0 {
				close = k
				break
			}
		}
	}
	if close < 0 || close+1 >= len(r) || r[close+1] != '(' {
		return nil, 0
	}
	end := -1
	for k := close + 2; k < len(r); k++ {
		if r[k] == ')' {
			end = k
			break
		}
	}
	if end < 0 {
		return nil, 0
	}
	href := strings.TrimSpace(string(r[close+2 : end]))
	sub := &inlineParser{p: x.p, t: mdText{r: r[x.i+1 : close], pos: x.t.pos[x.i+1 : close]}, path: x.path}
	inner, _ := sub.parse(false)
	if len(inner) == 0 {
		x.fail(x.i, codeMisplaced, "リンクの文が空です", "[文](URL) の文を書いてください")
	}
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:\S+$`).MatchString(href) {
		x.fail(close+2, codeInvalidValue, fmt.Sprintf("リンク先 %q は URL ではありません", href), "https:// などから始まる URL を書いてください")
	}
	return obj("type", "link", "href", href, "content", normalizeInline(inner)), end + 1 - x.i
}

// --- 木 → IR ---

// MarkdownToIR は Boxloom Markdown を IR の JSON にする。誤りがあれば、
// 読めたところまでの IR と誤りの一覧を返す。file は誤りの場所に添える名前。
func MarkdownToIR(file string, src []byte) ([]byte, []Issue) {
	out, errs, _ := ParseMarkdownWithMap(src, file)
	return out, errs
}

// ParseMarkdownWithMap は MarkdownToIR と同じで、IR の場所 → Markdown の行の
// 対応も返す。
func ParseMarkdownWithMap(src []byte, file string) ([]byte, []Issue, SourceMap) {
	p := &mdParser{file: file}
	out, errs := p.parseMarkdown(src)
	return out, errs, SourceMap(p.lines)
}

func (p *mdParser) parseMarkdown(src []byte) ([]byte, []Issue) {
	doc := p.parseBlocks(string(src))
	root := p.toIR(doc)
	out, err := marshalIndent(root)
	if err != nil {
		p.fail(0, 0, "", codeSchema, err.Error(), nil, "")
		return nil, p.errs
	}
	if len(p.errs) == 0 {
		// できた IR を Go の読み手にも通す(Schema と同じ「知らないキーは断る」)。
		if _, err := ParseIR(out); err != nil {
			p.fail(1, 1, "", codeSchema, err.Error(), nil, "")
		}
	}
	sortErrors(p.errs)
	return out, p.errs
}

func sortErrors(errs []Issue) {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Line != errs[j].Line {
			return errs[i].Line < errs[j].Line
		}
		return errs[i].Column < errs[j].Column
	})
}

func marshalIndent(v any) ([]byte, error) {
	b, err := marshalNoEscape(v)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func (p *mdParser) toIR(doc *mdDoc) *jobj {
	root := obj("boxloom", "0.1")
	cover := true
	var meta *jobj
	if doc.fm != nil {
		meta, cover = p.frontMatter(doc.fm, root)
	} else {
		cover = false
	}
	if meta != nil {
		root.set("meta", meta)
	}
	var slides []any
	if cover && meta != nil {
		p.mark("/slides/0", doc.fm.line)
		c := obj("type", "cover")
		if n, ok := p.notesText(doc.coverNotes, "/slides/0/notes"); ok {
			c.set("notes", n)
		}
		slides = append(slides, c)
	} else if len(doc.coverNotes) > 0 {
		p.fail(doc.coverNotes[0].line, 1, "", codeMisplaced, "最初のスライド(# 題)より前の :::notes は表紙のノートですが、表紙がありません", nil, "スライドのノートなら「# 題」の行の後ろに置いてください")
	}
	for _, s := range doc.slides {
		path := fmt.Sprintf("/slides/%d", len(slides))
		p.mark(path, s.line)
		attrs := map[string]string{}
		if s.attrs != "" {
			attrs = p.parseAttrs(s.attrs, s.line, s.attrsCol, path, slideAttributeNames)
		}
		typ := p.slideType(attrs, s, path)
		sl := obj("type", typ)
		if v, ok := attrs["variant"]; ok {
			sl.set("variant", v)
		}
		body, notes := splitNotes(s.items)
		body, sources := splitSource(body)
		body, leads := splitLead(body)
		switch typ {
		case "content":
			sl.set("title", p.parseInline(s.title, path+"/title"))
			if lead, ok := p.leadInline(leads, path+"/lead"); ok {
				sl.set("lead", lead)
			}
			sl.set("body", p.blocks(body, path+"/body"))
		case "section":
			sl.set("title", p.parseInline(s.title, path+"/title"))
			p.noSlideBody(body, path, typ)
		case "statement":
			sl.set("content", p.parseInline(s.title, path+"/content"))
			p.noSlideBody(body, path, typ)
		}
		if typ != "content" {
			for _, f := range leads {
				p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf("リード文(:::lead)は本文のスライドにだけ書けます(このスライドは type=%s)", typ), nil, "言いたいことの文は、区切りなら題に、主張なら「# 文」に書いてください")
			}
		}
		if typ == "section" {
			for _, f := range sources {
				p.fail(f.line, 1, path, codeMisplaced, "区切りのスライド(type=section)には :::source を書けません", nil, "出典は数や引用を示す本文・主張のスライドに置いてください")
			}
		} else if src, ok := p.sourceInline(sources, path+"/source"); ok {
			sl.set("source", src)
		}
		if n, ok := p.notesText(notes, path+"/notes"); ok {
			sl.set("notes", n)
		}
		slides = append(slides, sl)
	}
	if len(slides) == 0 {
		line := 1
		if doc.fm != nil {
			line = doc.fm.line
		}
		p.fail(line, 1, "/slides", codeMisplaced, "スライドが1枚もありません", nil, "「# 題」の行でスライドを始めてください")
	}
	if slides == nil {
		slides = []any{}
	}
	root.set("slides", slides)
	return root
}

// slideAttributeNames は「# 題 {…}」に書ける属性。
var slideAttributeNames = []string{"type", "variant"}

// slideType は「# 題 {type=…}」のスライドの種類(省けば content)。表紙は文書の頭から
// 作るので type=cover は誤り。
func (p *mdParser) slideType(attrs map[string]string, s *mdSlide, path string) string {
	typ, ok := attrs["type"]
	if !ok {
		return "content"
	}
	names := slideTypeNames()
	for _, n := range names {
		if n == typ {
			return typ
		}
	}
	hint := ""
	if typ == "cover" {
		hint = "表紙は文書の頭(--- で囲んだ title など)から作ります"
	}
	p.fail(s.line, s.attrsCol, path+"/type", codeInvalidValue, fmt.Sprintf("スライドの種類 %q はありません", typ), names, hint)
	return "content"
}

// noSlideBody は本文を持たないスライド(区切り・主張)に本文を書いた誤りを出す。
// :::notes は splitNotes が先に外している。
func (p *mdParser) noSlideBody(items []*mdItem, path, typ string) {
	for _, it := range items {
		switch typ {
		case "section":
			p.fail(it.line, 1, path, codeMisplaced, "区切りのスライド(type=section)には本文を書けません。章の題は # の行に書きます",
				nil, "章の目次を見せたいなら、本文のスライドに箇条で書いてください(docs/patterns.md の R1)")
		default:
			p.fail(it.line, 1, path, codeMisplaced, "主張のスライド(type=statement)には本文を書けません。言いたい1文は # の行に書きます",
				nil, "根拠を並べたいなら、本文のスライド(題に結論、本文に箇条)にしてください。本文の中で1文を目立たせるなら :::takeaway(docs/patterns.md の R2)")
		}
	}
}

func (p *mdParser) frontMatter(n *yNode, root *jobj) (*jobj, bool) {
	cover := true
	p.mark("/meta", n.line)
	if n.kind == 'n' {
		p.fail(n.line, 1, "/meta", codeMissingKey, "文書の頭に title がありません", []string{"title"}, "")
		return nil, false
	}
	if n.kind != 'm' {
		p.fail(n.line, n.col, "/meta", codeInvalidValue, "文書の頭は「キー: 値」の対応表で書いてください", nil, "")
		return nil, false
	}
	allowed := frontMatterNames()
	meta := &jobj{}
	hasTitle := false
	for k, key := range n.keys {
		v := n.vals[k]
		line := n.keyLine[k]
		path := "/meta/" + key
		if key == "template" {
			p.mark("/template", line)
		} else {
			p.mark(path, line)
		}
		switch key {
		case "title", "subtitle", "author", "confidential", "disclaimer":
			if key == "title" {
				hasTitle = true
			}
			if v.kind != 's' || v.str == "" {
				p.fail(line, v.col, path, codeInvalidValue, key+" は1行の文字列で書いてください", nil, "")
				continue
			}
			meta.set(key, p.parseInline(scalarText(v), path))
		case "date":
			if v.kind != 's' || !reDate.MatchString(v.str) {
				p.fail(line, v.col, path, codeInvalidValue, "date は YYYY-MM-DD か YYYY-MM で書いてください", nil, "")
				continue
			}
			meta.set("date", v.str)
		case "template":
			if v.kind != 's' || !reName.MatchString(v.str) {
				p.fail(line, v.col, "/template", codeInvalidValue, "template はテンプレートの名前(英小文字・数字・ハイフン)で書いてください", nil, "")
				continue
			}
			root.set("template", v.str)
		case "cover":
			b, ok := yBool(v)
			if !ok {
				p.fail(line, v.col, "", codeInvalidValue, "cover は true か false で書いてください", []string{"true", "false"}, "")
				continue
			}
			cover = b
		default:
			p.fail(line, 1, "/meta", codeUnknownKey, fmt.Sprintf("文書の頭にキー %s はありません", key), allowed, "")
		}
	}
	if !hasTitle {
		p.fail(n.line, 1, "/meta", codeMissingKey, "文書の頭に title がありません", []string{"title"}, "")
		return nil, cover
	}
	// meta の中の順を IR の例と揃える(title, subtitle, author, date, confidential, disclaimer)。
	ordered := &jobj{}
	for _, k := range []string{"title", "subtitle", "author", "date", "confidential", "disclaimer"} {
		for i, kk := range meta.keys {
			if kk == k {
				ordered.set(k, meta.vals[i])
			}
		}
	}
	return ordered, cover
}

func yBool(v *yNode) (bool, bool) {
	if v.kind != 's' || v.quoted {
		return false, false
	}
	switch v.str {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// scalarText は YAML の値を、文中の記法として読むための mdText にする。
func scalarText(v *yNode) mdText {
	col := v.col
	if v.quoted {
		col++
	}
	return lineText(v.str, v.line, col)
}

// blocks はスライドの本文を IR のブロックにする。
func (p *mdParser) blocks(items []*mdItem, path string) []any {
	out := []any{}
	for _, it := range items {
		bp := fmt.Sprintf("%s/%d", path, len(out))
		p.mark(bp, it.line)
		switch it.kind {
		case mdPara:
			if b, ok := p.imageParagraph(it, bp); ok {
				if b != nil {
					out = append(out, b)
				}
				continue
			}
			out = append(out, obj("type", "paragraph", "content", p.parseInline(it.text.trimBR(), bp+"/content")))
		case mdTable:
			if b := p.tableBlock(it, bp); b != nil {
				out = append(out, b)
			}
		case mdList:
			out = append(out, obj("type", "points", "items", p.listItems(it.items, bp+"/items", true)))
		case mdCode:
			if b := p.codeBlock(it, bp); b != nil {
				out = append(out, b)
			}
		case mdFence:
			if b := p.fenceBlock(it.fence, bp); b != nil {
				out = append(out, b)
			}
		}
	}
	return out
}

func (p *mdParser) listItems(ents []*mdListEnt, path string, allowNest bool) []any {
	var out []any
	for k, e := range ents {
		ip := fmt.Sprintf("%s/%d", path, k)
		p.mark(ip, e.line)
		content := p.parseInline(e.text.trimBR(), ip)
		if len(e.children) == 0 {
			out = append(out, content)
			continue
		}
		if !allowNest {
			p.fail(e.children[0].line, e.children[0].indent+1, ip, codeMisplaced, "ここでは箇条を入れ子にできません", nil, "")
			out = append(out, content)
			continue
		}
		out = append(out, obj("content", p.parseInlineAt(e.text.trimBR(), ip+"/content"), "items", p.listItems(e.children, ip+"/items", false)))
	}
	return out
}

// parseInlineAt は誤りを重ねて出さないための別名(同じ文を2回読まない)。
func (p *mdParser) parseInlineAt(t mdText, path string) any {
	saved := len(p.errs)
	v := p.parseInline(t, path)
	p.errs = p.errs[:saved]
	return v
}

func fenceExpected() []string { return knownFences() }

// fenceBlock は本文に置いた囲みを IR のブロックにする。
func (p *mdParser) fenceBlock(f *mdFenceBlk, path string) any {
	switch {
	case plannedFences[f.name]:
		p.fail(f.line, 1, path, codePlannedSyntax, fmt.Sprintf("囲み :::%s はまだ使えません(予定)", f.name), fenceExpected(), "")
		return nil
	case f.name == "compare":
		return p.compareBlock(f, path)
	case f.name == "columns":
		return p.columnsBlock(f, path)
	case f.name == "column":
		p.fail(f.line, 1, path, codeMisplaced, "囲み :::column は ::::columns の中にだけ書けます", nil, "")
		return nil
	case f.name == "note":
		attrs := p.fenceAttrs(f, path, []string{"variant"})
		p.noLabel(f, path)
		para := p.onePara(f, path)
		if para == nil {
			return nil
		}
		b := obj("type", "note")
		if v, ok := attrs["variant"]; ok {
			b.set("variant", v)
		}
		b.set("content", p.parseInline(para.text.trimBR(), path+"/content"))
		return b
	case f.name == "takeaway":
		attrs := p.fenceAttrs(f, path, []string{"variant"})
		p.noLabel(f, path)
		para := p.onePara(f, path)
		if para == nil {
			return nil
		}
		b := obj("type", "takeaway")
		if v, ok := attrs["variant"]; ok {
			b.set("variant", v)
		}
		b.set("content", p.parseInline(para.text.trimBR(), path+"/content"))
		return b
	case f.name == "quote":
		return p.quoteBlock(f, path)
	case f.name == "stat":
		return p.statBlock(f, path)
	case f.name == "notes":
		p.fail(f.line, 1, path, codeMisplaced, "発表者ノート(:::notes)はスライドの一番外側にだけ書けます", nil, "ほかの囲みの外に出してください")
		return nil
	case f.name == "lead":
		p.fail(f.line, 1, path, codeMisplaced, "リード文(:::lead)はスライドの一番外側にだけ書けます", nil, "ほかの囲みの外に出してください")
		return nil
	case f.name == "source":
		p.fail(f.line, 1, path, codeMisplaced, "出典(:::source)はスライドの一番外側にだけ書けます", nil, "ほかの囲みの外に出してください。引用の出どころは :::quote の見出しの文に書きます")
		return nil
	case f.name == "value":
		p.fail(f.line, 1, path, codeMisplaced, "囲み :::value は :::stat の中にだけ書けます", nil, "")
		return nil
	case f.name == "option" || f.name == "pros" || f.name == "cons":
		where := ":::compare の中の :::option"
		if f.name == "option" {
			where = ":::compare"
		}
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf("囲み :::%s は %s の中にだけ書けます", f.name, where), nil, "")
		return nil
	}
	// 型の語彙の標準でない型の囲み(読み方は型が持つ)。
	if read, ok := blockTypes.fenceReader(f.name); ok {
		return read(p, f, path)
	}
	if d, ok := blockTypes.innerFence(f.name); ok {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf("囲み :::%s は :::%s の中にだけ書けます", f.name, strings.Join(d.Inside, "・:::")), nil, "")
		return nil
	}
	p.fail(f.line, 1, path, codeUnknownBlock, fmt.Sprintf("囲み :::%s はありません", f.name), fenceExpected(), "見た目を変えたいときはテンプレートの variant を選んでください")
	return nil
}

// onePara は囲みの中のただ1つの段落(中が空・段落でないもの・2つ目は誤り)。
func (p *mdParser) onePara(f *mdFenceBlk, path string) *mdItem {
	var para *mdItem
	for _, it := range f.items {
		if it.kind == mdPara && para == nil {
			para = it
			continue
		}
		p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中には段落を1つだけ書けます", f.name), nil, "改行したいなら行末に \\ を付けてください")
	}
	if para == nil {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中が空です", f.name), nil, "")
	}
	return para
}

// quoteBlock は :::quote 出どころ … ::: を IR の quote にする。見出しの文が
// 出どころ(必須)、中の段落が引用した言葉。
func (p *mdParser) quoteBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	ok := true
	if len(f.label.r) == 0 {
		p.fail(f.line, 1, path+"/source", codeMissingKey, ":::quote には出どころが要ります", nil,
			"「:::quote 利用者 A、2026 年 9 月」のように、囲みの名前の後ろに誰の・いつの言葉かを書いてください")
		ok = false
	}
	para := p.onePara(f, path)
	if para == nil || !ok {
		return nil
	}
	b := obj("type", "quote")
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("content", p.parseInline(para.text.trimBR(), path+"/content"))
	b.set("source", p.parseInline(f.label, path+"/source"))
	return b
}

// statBlock は ::::stat の中の :::value(1〜4個)を IR の stat にする。:::value の
// 見出しの文が数(書いたとおりの文字)、属性 unit が単位、中の段落が説明。
func (p *mdParser) statBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	p.noLabel(f, path)
	var items []any
	for _, it := range f.items {
		if it.kind != mdFence || it.fence.name != "value" {
			p.fail(it.line, 1, path, codeMisplaced, ":::stat の中には :::value だけを書けます", []string{"value"}, "数ごとに「:::value 12 {unit=台}」の囲みを書き、中に説明を書いてください")
			continue
		}
		vf := it.fence
		ip := fmt.Sprintf("%s/items/%d", path, len(items))
		p.mark(ip, vf.line)
		va := p.fenceAttrs(vf, ip, []string{"unit"})
		value := strings.TrimSpace(string(vf.label.r))
		if value == "" {
			p.fail(vf.line, 1, ip+"/value", codeMissingKey, ":::value には数が要ります", nil, "「:::value 12 {unit=台}」のように、囲みの名前の後ろに数を書いてください")
			continue
		}
		if !strings.ContainsAny(value, "0123456789０１２３４５６７８９") {
			p.fail(vf.line, vf.label.pos[0].col, ip+"/value", codeInvalidValue, fmt.Sprintf("%q に数字がありません。stat は数を見せるブロックです", value), nil, "数でない言葉を目立たせたいなら :::takeaway を使ってください")
			continue
		}
		para := p.onePara(vf, ip)
		if para == nil {
			continue
		}
		item := obj("value", value)
		if u, ok := va["unit"]; ok {
			if u == "" {
				p.fail(vf.line, vf.attrsCol, ip+"/unit", codeInvalidValue, "単位が空です", nil, "単位が無いなら unit を書かないでください")
				continue
			}
			item.set("unit", u)
		}
		item.set("label", p.parseInline(para.text.trimBR(), ip+"/label"))
		items = append(items, item)
	}
	if len(items) == 0 {
		p.fail(f.line, 1, path, codeMisplaced, ":::stat には :::value が1つ以上要ります", []string{"value"}, "")
		return nil
	}
	if len(items) > statMaxItems {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf(":::stat に置ける :::value は %d つまでです(いま %d つ)", statMaxItems, len(items)), nil,
			"5つ以上の数や推移は系列なので、表(table)かグラフ(chart)で見せます(docs/patterns.md の R3・R4)")
		return nil
	}
	b := obj("type", "stat")
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("items", items)
	return b
}

func (p *mdParser) fenceAttrs(f *mdFenceBlk, path string, allowed []string) map[string]string {
	if f.attrs == "" {
		return map[string]string{}
	}
	return p.parseAttrs(f.attrs, f.line, f.attrsCol, path, allowed)
}

func (p *mdParser) noLabel(f *mdFenceBlk, path string) {
	if len(f.label.r) > 0 {
		p.fail(f.line, f.label.pos[0].col, path, codeMisplaced, fmt.Sprintf("囲み :::%s には見出しの文を付けられません", f.name), nil, "")
	}
}

func (p *mdParser) compareBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	p.noLabel(f, path)
	var options []any
	for _, it := range f.items {
		if it.kind != mdFence || it.fence.name != "option" {
			p.fail(it.line, 1, path, codeMisplaced, ":::compare の中には :::option だけを書けます", []string{"option"}, "")
			continue
		}
		o := it.fence
		op := fmt.Sprintf("%s/options/%d", path, len(options))
		p.mark(op, o.line)
		p.fenceAttrs(o, op, nil)
		if len(o.label.r) == 0 {
			p.fail(o.line, 1, op, codeMisplaced, ":::option には選択肢の名前が要ります", nil, "「:::option 名前」のように書いてください")
			continue
		}
		opt := obj("label", p.parseInline(o.label, op+"/label"))
		seen := map[string]bool{}
		for _, sub := range o.items {
			if sub.kind != mdFence || (sub.fence.name != "pros" && sub.fence.name != "cons") {
				p.fail(sub.line, 1, op, codeMisplaced, ":::option の中には :::pros と :::cons だけを書けます", []string{"pros", "cons"}, "")
				continue
			}
			sf := sub.fence
			if seen[sf.name] {
				p.fail(sf.line, 1, op, codeMisplaced, fmt.Sprintf(":::%s が2つあります", sf.name), nil, "1つにまとめてください")
				continue
			}
			seen[sf.name] = true
			sp := op + "/" + sf.name
			p.fenceAttrs(sf, sp, nil)
			p.noLabel(sf, sp)
			var items []any
			for _, li := range sf.items {
				if li.kind != mdList {
					p.fail(li.line, 1, sp, codeMisplaced, fmt.Sprintf(":::%s の中には箇条(- …)だけを書けます", sf.name), nil, "")
					continue
				}
				items = append(items, p.listItems(li.items, sp, false)...)
			}
			if len(items) == 0 {
				p.fail(sf.line, 1, sp, codeMisplaced, fmt.Sprintf(":::%s の中に箇条がありません", sf.name), nil, "")
				continue
			}
			opt.set(sf.name, items)
		}
		if !seen["pros"] && !seen["cons"] {
			p.fail(o.line, 1, op, codeMisplaced, ":::option には :::pros か :::cons が要ります", []string{"pros", "cons"}, "")
		}
		options = append(options, opt)
	}
	if len(options) < 2 {
		p.fail(f.line, 1, path, codeMisplaced, ":::compare には :::option が2つ以上要ります", nil, "")
		return nil
	}
	b := obj("type", "compare")
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("options", options)
	return b
}

// mdSubsystems はコードブロックの言語名 → IR の data を作るもの。
var mdSubsystems = map[string]func(p *mdParser, n *yNode, path string) any{
	"gantt":    ganttDataFromYAML,
	"shapes":   shapesDataFromYAML,
	"process":  processDataFromYAML,
	"orgchart": orgChartDataFromYAML,
	"matrix":   matrixDataFromYAML,
	"funnel":   funnelDataFromYAML,
	"chart":    chartDataFromYAML,
}

func (p *mdParser) codeBlock(it *mdItem, path string) any {
	lang, rest := it.info, ""
	if k := strings.IndexAny(lang, " \t{"); k >= 0 {
		lang, rest = lang[:k], lang[k:]
	}
	conv, ok := mdSubsystems[lang]
	if !ok {
		var names []string
		for k := range mdSubsystems {
			names = append(names, k)
		}
		sort.Strings(names)
		msg := "コードのブロックはまだ使えません(予定。IR 第一版にコードのブロックが無い)"
		if lang == "" {
			msg = "言語名の無いコードブロックは使えません(IR 第一版にコードのブロックが無い)"
		}
		p.fail(it.line, 1, path, codePlannedSyntax, msg, names, "サブシステムを置くなら言語名にサブシステムの名前を書いてください(例 ```gantt)")
		return nil
	}
	b := obj("type", "subsystem", "subsystem", lang)
	if am := reAttrs.FindStringSubmatch(rest); am != nil {
		attrs := p.parseAttrs(am[1], it.line, 4+utf8.RuneCountInString(lang), path, []string{"variant"})
		if v, ok := attrs["variant"]; ok {
			b.set("variant", v)
		}
	} else if strings.TrimSpace(rest) != "" {
		p.fail(it.line, 4+utf8.RuneCountInString(lang), path, codeUnknownAttribute, "言語名の後ろには {variant=名前} だけを書けます", []string{"variant"}, "")
	}
	p.mark(path+"/data", it.line)
	node, yerrs := parseYAML(it.code, it.codeLine)
	for _, e := range yerrs {
		p.fail(e.line, e.col, path+"/data", codeYAML, e.msg, nil, e.hint)
	}
	if len(yerrs) > 0 {
		return nil
	}
	saved := len(p.errs)
	data := conv(p, node, path+"/data")
	if data == nil {
		return nil
	}
	if len(p.errs) == saved {
		// サブシステム自身にも読ませ、Schema では書けない誤り(id の重複・
		// target の行き先・日付の前後)をここで見つける。
		raw, _ := marshalNoEscape(data)
		// shapes は読ませない。範囲の外・無い id・知らない役割名は boxloom check が
		// 図形ごとの行と決まった code(shape-bounds など)で返す。
		if lang == "gantt" {
			if _, err := ganttFromIR(raw); err != nil {
				p.fail(it.line, 1, path+"/data", codeInvalidValue, err.Error(), nil, "")
			}
		}
	}
	b.set("data", data)
	return b
}
