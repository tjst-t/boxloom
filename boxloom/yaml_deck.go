// YAML の deck(IR の写し。docs/content-syntax.md の入口の表、docs/ir.md)。
//
// YAML は JSON の IR と同じキー・同じ type で書く(写し方を独自にしない)。
// 読むのは yaml.go の部分集合(参照・タグ・複数文書・ブロックの文字列は誤り)。
// 値の読み方は JSON と同じ形だけを使う:
//
//   - 引用("…" と '…')は常に文字列
//   - 引用なしの true・false は真偽値、null と空の値は null
//   - 引用なしで JSON の数の形(-1・0.5・1e3)なら数
//   - ほかは文字列(2026-04 のような日付も文字列のまま。yes・on も文字列)
//
// 読んだ結果は JSON の IR にして、ParseIR と check にそのまま通す。誤りには
// IR の場所から引いた YAML の行と列を添える(Markdown の check と同じ考え)。

package boxloom

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// isYAMLPath は YAML の deck として読むファイルか(.yaml・.yml)。
func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// yamlPos は YAML の中の場所(行と列。1から)。
type yamlPos struct{ line, col int }

// YAMLSourceMap は IR の場所(JSON Pointer)→ YAML の行と列。
type YAMLSourceMap map[string]yamlPos

// Pos は path か、その一番近い親の行と列(無ければ 0, 0)。
func (m YAMLSourceMap) Pos(path string) (line, col int) {
	for {
		if p, ok := m[path]; ok {
			return p.line, p.col
		}
		k := strings.LastIndex(path, "/")
		if k < 0 {
			return 0, 0
		}
		path = path[:k]
	}
}

// Line は path の行(SourceMap と同じ使い方のため)。
func (m YAMLSourceMap) Line(path string) int {
	l, _ := m.Pos(path)
	return l
}

var yamlJSONNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

type yamlDeckReader struct {
	file string
	errs []Issue
	pos  YAMLSourceMap
}

func (r *yamlDeckReader) fail(line, col int, path, code, msg, hint string) {
	r.errs = append(r.errs, Issue{File: r.file, Line: line, Column: col, Path: path, Code: code, Message: msg, Hint: hint})
}

// value は yNode を JSON の値(*jobj・[]any・string・json.Number・bool・nil)にする。
func (r *yamlDeckReader) value(n *yNode, path string) any {
	switch n.kind {
	case 'm':
		o := &jobj{}
		for i, k := range n.keys {
			kp := path + "/" + escapePointer(k)
			col := n.vals[i].col
			if i < len(n.keyCol) {
				col = n.keyCol[i]
			}
			r.pos[kp] = yamlPos{n.keyLine[i], col}
			o.set(k, r.value(n.vals[i], kp))
		}
		return o
	case 'q':
		a := make([]any, 0, len(n.items))
		for i, it := range n.items {
			ip := path + "/" + strconv.Itoa(i)
			r.pos[ip] = yamlPos{it.line, it.col}
			a = append(a, r.value(it, ip))
		}
		return a
	case 's':
		if n.quoted {
			return n.str
		}
		switch n.str {
		case "true":
			return true
		case "false":
			return false
		case "null":
			return nil
		}
		if yamlJSONNumber.MatchString(n.str) {
			return json.Number(n.str)
		}
		return n.str
	}
	return nil
}

// YAMLToIR は YAML の deck(src)を IR の JSON にする。file は誤りの場所に
// 添える名前。YAML の部分集合の外の書き方と、できた IR が Go の読み込み
// (ParseIR。知らないキー・型の違い・足りない欄)を通らないものは、行と列
// つきの Issue で返す(そのときの JSON は使わないこと)。
func YAMLToIR(file string, src []byte) ([]byte, []Issue) {
	out, errs, pos := parseYAMLDeck(file, src)
	if len(errs) > 0 {
		return out, errs
	}
	if _, err := ParseIR(out); err != nil {
		return out, []Issue{yamlIRIssue(file, err, pos)}
	}
	return out, nil
}

// parseYAMLDeck は YAMLToIR に、IR の場所 → YAML の行と列の対応を足したもの。
// ParseIR は通さない(check は意味の検査を先に見るため)。
func parseYAMLDeck(file string, src []byte) ([]byte, []Issue, YAMLSourceMap) {
	r := &yamlDeckReader{file: file, pos: YAMLSourceMap{"": {1, 1}}}
	text := strings.TrimPrefix(string(src), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	node, yerrs := parseYAML(lines, 1)
	for _, e := range yerrs {
		r.fail(e.line, e.col, "", codeYAML, e.msg, e.hint)
	}
	if len(r.errs) > 0 {
		sortErrors(r.errs)
		return nil, r.errs, r.pos
	}
	if node.kind != 'm' {
		r.fail(node.line, node.col, "", codeSchema, "YAML の deck の一番外側は対応表(boxloom: \"0.1\" と slides: …)にしてください", "JSON の IR と同じキーで書きます(docs/ir.md)")
		return nil, r.errs, r.pos
	}
	out, err := marshalIndent(r.value(node, ""))
	if err != nil {
		r.fail(1, 1, "", codeSchema, err.Error(), "")
		return nil, r.errs, r.pos
	}
	return out, nil, r.pos
}

// yamlIRIssue は ParseIR の誤りを、YAML の行と列つきの Issue にする。
func yamlIRIssue(file string, err error, pos YAMLSourceMap) Issue {
	path := locationToPointer(err.Error())
	line, col := pos.Pos(path)
	return Issue{File: file, Line: line, Column: col, Path: path, Code: codeSchema, Message: err.Error(), Hint: yamlTypeHint(err.Error())}
}

// yamlTypeHint は型の違いの誤りに、YAML で起きやすい原因を添える。
func yamlTypeHint(msg string) string {
	if strings.Contains(msg, "cannot unmarshal number") || strings.Contains(msg, "cannot unmarshal bool") {
		return "YAML の引用なしの数・true・false は数と真偽値として読みます。文字列なら \"…\" で囲んでください"
	}
	return ""
}

// ParseYAMLDeck は YAML の deck(src)を読んで IR にする。誤りは行と列つきの
// Issue の一覧で返し、そのとき deck は nil(ParseMarkdown と同じ形)。
func ParseYAMLDeck(file string, src []byte) (*IRDeck, []Issue) {
	out, errs, pos := parseYAMLDeck(file, src)
	if len(errs) > 0 {
		return nil, errs
	}
	d, err := ParseIR(out)
	if err != nil {
		return nil, []Issue{yamlIRIssue(file, err, pos)}
	}
	return d, nil
}
