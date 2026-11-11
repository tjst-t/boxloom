package boxloom

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TemplateError は外から渡したテンプレートの値の表(ParseTemplate)の誤り。
// 中身は誤りと警告の形(docs/content-syntax.md の5章)の1件で、path は値の表の
// 中の場所(JSON Pointer)、code は template-values。
type TemplateError struct{ Issue Issue }

func (e *TemplateError) Error() string { return e.Issue.String() }

// ParseTemplate は外のテンプレートの値の表(JSON。同梱の templates/<名前>.json
// と同じ形)を読み、同梱のものと同じ検査をする(知らないキー・欠けたキー・null・
// 0 以下・色の形・段から外れた値・寸法の無い字体は誤り)。file はファイルの名前で、
// その名前(拡張子 .json を除いた最後の部分)が値の表の name と同じでなければ
// ならない。誤りは *TemplateError で返す。ライブラリはファイルを読まないので、
// 中身は data で渡す。
//
// できた Template は BuildOptions・CheckOptions の Values と SchemaTemplate に渡す。
func ParseTemplate(file string, data []byte) (*Template, error) {
	name := strings.TrimSuffix(path.Base(strings.ReplaceAll(file, "\\", "/")), ".json")
	t, err := parseTemplateFile(name, data)
	if err != nil {
		return nil, &TemplateError{templateIssue(file, data, err.Error())}
	}
	return t, nil
}

// IsTemplateFile は -template の値がファイルのパスか(.json で終わるか、パスの
// 区切りを含む)。名前ならコマンドは同梱のテンプレートを引く。
func IsTemplateFile(v string) bool {
	return strings.HasSuffix(strings.ToLower(v), ".json") || strings.ContainsAny(v, "/\\")
}

var (
	tmplMissingRe = regexp.MustCompile(`^(?:missing key|key) "([^"]+)"`)
	tmplUnknownRe = regexp.MustCompile(`json: unknown field "([^"]+)"`)
	tmplGoFieldRe = regexp.MustCompile(`Go struct field [A-Za-z0-9_]+\.([A-Za-z0-9_.]+)`)
	tmplPrefixRe  = regexp.MustCompile(`^([a-z][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]+|\[[0-9]+\]|\["[^"]*"\])*):`)
	tmplRoleRe    = regexp.MustCompile(`^theme: (?:unknown|missing) role "([^"]+)"`)
	tmplPartRe    = regexp.MustCompile(`[A-Za-z0-9_]+|\[[0-9]+\]|\["[^"]*"\]`)
)

// templateIssue は値の表の誤りの文から、値の表の中の場所(JSON Pointer)と行を
// 引いて1件の誤りにする(引けなければ path は根)。
func templateIssue(file string, data []byte, msg string) Issue {
	dotted := ""
	switch {
	case tmplMissingRe.MatchString(msg):
		dotted = tmplMissingRe.FindStringSubmatch(msg)[1]
	case tmplUnknownRe.MatchString(msg):
		dotted = findKeyPath(data, tmplUnknownRe.FindStringSubmatch(msg)[1])
	case tmplGoFieldRe.MatchString(msg):
		dotted = tmplGoFieldRe.FindStringSubmatch(msg)[1]
	case tmplRoleRe.MatchString(msg):
		dotted = "theme." + tmplRoleRe.FindStringSubmatch(msg)[1]
		if strings.Contains(msg, "missing role") {
			dotted = "theme"
		}
	case strings.HasPrefix(msg, "name "):
		dotted = "name"
	case strings.HasPrefix(msg, "subsystems/"):
		dotted = "scale" // 既定の寸法がこの値の表の段に載らない
	case tmplPrefixRe.MatchString(msg) && !strings.HasPrefix(msg, "json:"):
		dotted = tmplPrefixRe.FindStringSubmatch(msg)[1]
	}
	var keys []string
	for _, p := range tmplPartRe.FindAllString(dotted, -1) {
		p = strings.TrimSuffix(strings.TrimPrefix(p, "["), "]")
		if uq, err := strconv.Unquote(p); err == nil {
			p = uq
		}
		keys = append(keys, p)
	}
	ptr := ""
	for _, k := range keys {
		ptr += "/" + escapePointer(k)
	}
	return Issue{File: file, Line: keyLine(data, keys), Path: ptr, Code: "template-values", Message: msg}
}

// findKeyPath は値の表の中でキー key を最初に持つ場所(点でつないだもの)を探す。
func findKeyPath(data []byte, key string) string {
	var doc any
	if json.Unmarshal(data, &doc) != nil {
		return key
	}
	var walk func(v any, at string) (string, bool)
	walk = func(v any, at string) (string, bool) {
		switch v := v.(type) {
		case map[string]any:
			if _, ok := v[key]; ok {
				return strings.TrimPrefix(at+"."+key, "."), true
			}
			for _, k := range sortedAnyKeys(v) {
				if p, ok := walk(v[k], at+"."+k); ok {
					return p, true
				}
			}
		case []any:
			for i, e := range v {
				if p, ok := walk(e, at+"["+strconv.Itoa(i)+"]"); ok {
					return p, true
				}
			}
		}
		return "", false
	}
	if p, ok := walk(doc, ""); ok {
		return p
	}
	return key
}

func sortedAnyKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// keyLine はキーの並び(配列の番号は飛ばす)を前から順に探し、最後のキーの
// 行を返す(見つからなければ 0。行は目安で、同じ名前のキーが前にあればずれうる)。
func keyLine(data []byte, keys []string) int {
	pos, found := 0, false
	for _, k := range keys {
		if _, err := strconv.Atoi(k); err == nil {
			continue
		}
		q, _ := json.Marshal(k)
		i := bytes.Index(data[pos:], append(q, ':'))
		if i < 0 {
			i = bytes.Index(data[pos:], q)
		}
		if i < 0 {
			break
		}
		pos, found = pos+i, true
	}
	if !found {
		return 0
	}
	return bytes.Count(data[:pos], []byte("\n")) + 1
}
