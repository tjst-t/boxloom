// boxloom check: JSON Schema では書けない IR の検査。
//
//	go run . check [-format text|json] [-template dark|light] [-strict] deck.json|deck.bx.md ...
//
// 誤りと警告は docs/content-syntax.md の5章の形(Issue)で出す:
//
//	{"ok": false,
//	 "errors":   [{"file": "deck.bx.md", "line": 40, "path": "/slides/2/body/0/data/...", "code": "date-order", "message": "..."}],
//	 "warnings": [{"file": "deck.json", "path": "/slides/1", "code": "fill-ratio", "message": "..."}]}
//
// IR(JSON)を検査したときは line・column を出さない。.md を検査したときは、
// 記法の誤りならパーサーの誤り(line・column つき)を、意味の誤りなら IR の
// 場所から引いた Markdown の行(line だけ)を添える。
//
// 誤りは最初の1件で止めず、全部集める。誤りが無ければ組み立て(build)まで通し、
// そのときの警告(本文の埋まり具合・サブシステムの最小の大きさ・注釈の重なり・
// テーマに無い役割名)も code つきで出す。終了コードは 0=誤りなし、1=誤りあり
// (-strict なら警告もこちら)、2=使い方の誤り。
//
// 形(キー・型・enum)の検査は Schema(tools/validate_ir.py)の仕事。ここは
// 「形は正しいが意味が通らない」もの(id の重なり・指す先の無い target・
// 日付の前後・範囲の外の日付・暦に無い日付・テンプレートに無い variant)を見る。
// Go の IR の読み込み(ir.go)が断るもの(知らないキーなど)も、場所つきの
// 誤りとして出す。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CheckResult は1回の check の結果。誤りと警告は Issue(docs/content-syntax.md
// の5章の形)。
type CheckResult struct {
	OK       bool    `json:"ok"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	tmpl := fs.String("template", "", "テンプレートの名前(省くと IR の template)")
	strict := fs.Bool("strict", false, "警告も誤りとして扱う(終了コード 1)")
	format := fs.String("format", "text", "出し方: text か json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || (*format != "text" && *format != "json") {
		fmt.Fprintln(os.Stderr, "usage: boxloom check [-format text|json] [-template dark|light] [-strict] deck.json|deck.bx.md ...")
		return 2
	}
	res := CheckResult{Errors: []Issue{}, Warnings: []Issue{}}
	for _, f := range fs.Args() {
		errs, warns := CheckFile(f, *tmpl)
		res.Errors = append(res.Errors, errs...)
		res.Warnings = append(res.Warnings, warns...)
	}
	res.OK = len(res.Errors) == 0 && (!*strict || len(res.Warnings) == 0)
	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		for _, e := range res.Errors {
			fmt.Println("error:", e.String())
		}
		for _, w := range res.Warnings {
			fmt.Println("warning:", w.String())
		}
		fmt.Printf("%d error(s), %d warning(s)\n", len(res.Errors), len(res.Warnings))
	}
	if !res.OK {
		return 1
	}
	return 0
}

// CheckFile は1つのファイルを検査する。templateOverride が空でなければ IR の
// template より優先する。.md は Boxloom Markdown として読み、記法の誤りが
// あればそれを返す。無ければできた IR を検査し、誤りと警告に Markdown の行を
// 添える(IR の場所から、その場所を作った Markdown の行を引く)。
func CheckFile(path, templateOverride string) (errs, warns []Issue) {
	b, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{File: path, Message: err.Error(), Code: "read"}}, nil
	}
	if !isMarkdownPath(path) {
		return CheckIR(path, b, templateOverride)
	}
	ir, mdErrs, lines := ParseMarkdownWithMap(b, path)
	if len(mdErrs) > 0 {
		return mdErrs, nil
	}
	errs, warns = CheckIR(path, ir, templateOverride)
	for _, list := range [][]Issue{errs, warns} {
		for i := range list {
			list[i].Line = lines.Line(list[i].Path)
		}
	}
	return errs, warns
}

// CheckIR はバイト列の IR を検査する(試験から直接呼べるように分けてある)。
func CheckIR(file string, b []byte, templateOverride string) (errs, warns []Issue) {
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return []Issue{{File: file, Path: "", Message: "not JSON: " + err.Error(), Code: "json"}}, nil
	}
	c := &checker{file: file}

	// テンプレート(variant の検査と組み立てに使う)。
	name, _ := getPath(doc, "template").(string)
	at := "/template"
	if templateOverride != "" {
		name, at = templateOverride, ""
	}
	tm, terr := LookupTemplate(name)
	if terr != nil {
		c.add(at, terr.Error(), "template-exists")
	}

	c.walkDoc(doc, tm)

	// Go の読み込みが断るもの(知らないキー・必須の欄など)。意味の検査と同じ
	// 誤りを二重に出さないよう、意味の誤りが無いときだけ見る。
	if len(c.errs) > 0 {
		return c.errs, nil
	}
	deck, perr := ParseIR(b)
	if perr != nil {
		c.add(locationToPointer(perr.Error()), perr.Error(), "schema")
		return c.errs, nil
	}
	warnings, berr := tm.RenderDeck(deck, os.DevNull)
	if berr != nil {
		c.add(locationToPointer(berr.Error()), berr.Error(), "build")
		return c.errs, nil
	}
	for _, w := range warnings {
		warns = append(warns, warningIssue(file, w))
	}
	return c.errs, warns
}

type checker struct {
	file string
	errs []Issue
}

func (c *checker) add(at, msg, rule string) {
	c.errs = append(c.errs, Issue{File: c.file, Path: at, Message: msg, Code: rule})
}

// --- 文書の歩き方 ---

func (c *checker) walkDoc(doc any, tm *Template) {
	// id は文書全体で一意(Schema の id の説明どおり)。
	ids := map[string]string{} // id → 最初に出た場所
	walkAll(doc, "", func(ptr string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		id, ok := m["id"].(string)
		if !ok {
			return
		}
		if first, dup := ids[id]; dup {
			c.add(ptr+"/id", fmt.Sprintf("id %q is already used at %s", id, first), "unique-id")
			return
		}
		ids[id] = ptr
	})

	slides, _ := getPath(doc, "slides").([]any)
	for si, s := range slides {
		sp := fmt.Sprintf("/slides/%d", si)
		sm, _ := s.(map[string]any)
		c.checkVariant(sm, sp, tm)
		body, _ := sm["body"].([]any)
		for bi, blk := range body {
			bp := fmt.Sprintf("%s/body/%d", sp, bi)
			bm, _ := blk.(map[string]any)
			c.checkVariant(bm, bp, tm)
			if bm["type"] == "subsystem" && bm["subsystem"] == "gantt" {
				if data, ok := bm["data"].(map[string]any); ok {
					c.checkGantt(data, bp+"/data", ids)
				}
			}
		}
	}

	// 日付の範囲(from/to)は、どこにあっても前後を見る。
	walkAll(doc, "", func(ptr string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		fs, ok1 := m["from"].(string)
		ts, ok2 := m["to"].(string)
		if !ok1 || !ok2 {
			return
		}
		f, e1 := parseIRDate(fs)
		t, e2 := parseIRDate(ts)
		if e1 != nil || e2 != nil {
			return // 日付そのものの誤りは checkDates が出す
		}
		if !f.Before(t) {
			c.add(ptr, fmt.Sprintf("from (%s) must be before to (%s)", fs, ts), "date-order")
		}
	})
	c.checkDates(doc)
}

func (c *checker) checkVariant(m map[string]any, ptr string, tm *Template) {
	v, ok := m["variant"].(string)
	if !ok || tm == nil {
		return
	}
	if !tm.Variants[v] {
		known := make([]string, 0, len(tm.Variants))
		for k := range tm.Variants {
			known = append(known, k)
		}
		sort.Strings(known)
		c.add(ptr+"/variant", fmt.Sprintf("template %q has no variant %q (known: %v)", tm.Name, v, known), "variant-exists")
	}
}

// checkDates は日付の形をした文字列が、暦の上で実在する日かを見る
// (Schema の正規表現は 2026-02-30 を通してしまう)。
var irDateRe = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])(-(0[1-9]|[12][0-9]|3[01]))?$`)

func (c *checker) checkDates(doc any) {
	walkAll(doc, "", func(ptr string, v any) {
		s, ok := v.(string)
		if !ok || !irDateRe.MatchString(s) {
			return
		}
		if _, err := parseIRDate(s); err != nil {
			c.add(ptr, fmt.Sprintf("%q is not a real date", s), "date-valid")
		}
	})
}

// checkGantt は Gantt の data の意味を見る: target の指す先と、範囲(span)の
// 外にはみ出す日付。
func (c *checker) checkGantt(data map[string]any, ptr string, docIDs map[string]string) {
	// このチャートの中の id(グループと行)。
	local := map[string]bool{}
	groups, _ := data["groups"].([]any)
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		if id, ok := gm["id"].(string); ok {
			local[id] = true
		}
		rows, _ := gm["rows"].([]any)
		for _, r := range rows {
			rm, _ := r.(map[string]any)
			if id, ok := rm["id"].(string); ok {
				local[id] = true
			}
		}
	}
	anns, _ := data["annotations"].([]any)
	for i, a := range anns {
		am, _ := a.(map[string]any)
		tgt, ok := am["target"].(string)
		if !ok || local[tgt] {
			continue
		}
		ap := fmt.Sprintf("%s/annotations/%d/target", ptr, i)
		if where, elsewhere := docIDs[tgt]; elsewhere {
			c.add(ap, fmt.Sprintf("target %q is not a group or row of this chart (it is at %s)", tgt, where), "target-exists")
		} else {
			c.add(ap, fmt.Sprintf("no group or row has id %q", tgt), "target-exists")
		}
	}

	span, _ := data["span"].(map[string]any)
	sf, e1 := dateAt(span, "from")
	st, e2 := dateAt(span, "to")
	if e1 != nil || e2 != nil || !sf.Before(st) {
		return // 範囲そのものが壊れていれば、はみ出しは測れない(ほかの規則が出す)
	}
	within := func(p, s string) {
		t, err := parseIRDate(s)
		if err != nil {
			return
		}
		if t.Before(sf) || t.After(st) {
			c.add(p, fmt.Sprintf("%s is outside the chart span %s .. %s", s, fmtYM(sf), fmtYM(st)), "within-span")
		}
	}
	if s, ok := data["axisBreakBefore"].(string); ok {
		within(ptr+"/axisBreakBefore", s)
	}
	for _, key := range []string{"ticks", "milestones", "annotations"} {
		arr, _ := data[key].([]any)
		for i, x := range arr {
			xm, _ := x.(map[string]any)
			if s, ok := xm["at"].(string); ok {
				within(fmt.Sprintf("%s/%s/%d/at", ptr, key, i), s)
			}
		}
	}
	rangeWithin := func(p string, m map[string]any) {
		for _, k := range []string{"from", "to"} {
			if s, ok := m[k].(string); ok {
				within(p+"/"+k, s)
			}
		}
	}
	for gi, g := range groups {
		gm, _ := g.(map[string]any)
		gp := fmt.Sprintf("%s/groups/%d", ptr, gi)
		if r, ok := gm["risk"].(map[string]any); ok {
			rangeWithin(gp+"/risk", r)
		}
		rows, _ := gm["rows"].([]any)
		for ri, r := range rows {
			rm, _ := r.(map[string]any)
			rp := fmt.Sprintf("%s/rows/%d", gp, ri)
			if n, ok := rm["note"].(map[string]any); ok {
				rangeWithin(rp+"/note", n)
			}
			segs, _ := rm["segments"].([]any)
			for si, s := range segs {
				sm, _ := s.(map[string]any)
				rangeWithin(fmt.Sprintf("%s/segments/%d", rp, si), sm)
			}
		}
	}
}

func dateAt(m map[string]any, k string) (time.Time, error) {
	s, ok := m[k].(string)
	if !ok {
		return time.Time{}, fmt.Errorf("missing %s", k)
	}
	return parseIRDate(s)
}

func fmtYM(t time.Time) string {
	if t.Day() == 1 {
		return t.Format("2006-01")
	}
	return t.Format("2006-01-02")
}

// --- 道具 ---

// walkAll は JSON の木を深さ優先で歩き、すべての値に JSON Pointer を添えて f を呼ぶ。
// object のキーは名前の順に歩く(出力の順を決まったものにするため)。
func walkAll(v any, ptr string, f func(ptr string, v any)) {
	f(ptr, v)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkAll(x[k], ptr+"/"+escapePointer(k), f)
		}
	case []any:
		for i, e := range x {
			walkAll(e, ptr+"/"+strconv.Itoa(i), f)
		}
	}
}

func escapePointer(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

func getPath(v any, key string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

// locationToPointer は ir.go / gantt_ir.go / template.go の誤りの文の頭にある場所
// (例 "slides[2]: body[0]: gantt: groups[0].rows[1].segments[0].status: ...")を
// JSON Pointer(/slides/2/body/0/data/groups/0/rows/1/segments/0/status)に変える。
// 場所として読めない部分に来たらそこで止める。
var locSegRe = regexp.MustCompile(`^[A-Za-z]+(\[[0-9]+\])?(\.[A-Za-z]+(\[[0-9]+\])?)*$`)
var locPartRe = regexp.MustCompile(`([A-Za-z]+)|\[([0-9]+)\]`)

func locationToPointer(msg string) string {
	ptr := ""
	for _, seg := range strings.Split(msg, ": ") {
		if seg == "gantt" {
			ptr += "/data"
			continue
		}
		if !locSegRe.MatchString(seg) || !strings.ContainsAny(seg, "[.") {
			break
		}
		for _, m := range locPartRe.FindAllStringSubmatch(seg, -1) {
			if m[1] != "" {
				ptr += "/" + m[1]
			} else {
				ptr += "/" + m[2]
			}
		}
	}
	// encoding/json の「知らないキー」は、そのキーまで指す。
	if m := unknownFieldRe.FindStringSubmatch(msg); m != nil {
		ptr += "/" + escapePointer(m[1])
	}
	return ptr
}

var unknownFieldRe = regexp.MustCompile(`json: unknown field "([^"]+)"`)

// warningIssue は RenderDeck の警告の文("slide 2: [FILL-RATIO WARNING] ...")を
// 1件の警告にする。rule は角括弧の中の名前から作る(FILL-RATIO → fill-ratio)。
// 新しい種類の警告が増えても、この形で書かれていればそのまま rule になる。
var warnRe = regexp.MustCompile(`^slide ([0-9]+): \[([A-Z-]+) WARNING\] (.*)$`)

var warnRuleNames = map[string]string{"SUBSYSTEM": "subsystem-min", "ROLE": "theme-role"}

func warningIssue(file, w string) Issue {
	m := warnRe.FindStringSubmatch(w)
	if m == nil {
		return Issue{File: file, Message: w, Code: "warning"}
	}
	n, _ := strconv.Atoi(m[1])
	rule := warnRuleNames[m[2]]
	if m[2] == "SUBSYSTEM" && strings.Contains(m[3], "overlaps another annotation") {
		// Gantt の注釈の置き場所が無かった(lintSubsystems が拾う SubsystemLinter の警告)
		rule = "annotation-overlap"
	}
	if rule == "" {
		rule = strings.ToLower(m[2])
	}
	return Issue{File: file, Path: fmt.Sprintf("/slides/%d", n-1), Message: m[3], Code: rule}
}
