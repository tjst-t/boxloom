// ```gantt のコードブロックの中身(YAML の部分集合)を、IR の Gantt の data
// (schema/subsystems/gantt.schema.json)にする。
//
// キーは IR と同じ。Markdown の側だけの書き方は次の3つ。
//   - 範囲(span・risk)は「A .. B」と1つの文字列で書ける({from, to} でもよい)。
//   - 文の値(label・text など、IR でインラインのもの)は Boxloom Markdown の
//     文中の記法で読む(**強調**・`コード`・[リンク](URL))。YAML の列で書くと、
//     各行を意味のある改行(br)でつなぐ。
//   - 行の配列の値(グループの label)は、文字列か YAML の列。
//
// 知らないキー・型の違う値・知らない状態の名前は、行と列つきの誤りにする。
package main

import (
	"fmt"
	"strings"
)

type ganttConv func(p *mdParser, n *yNode, path string) any

type gField struct {
	name     string
	conv     ganttConv
	required bool
}

var ganttStatuses = []string{"support", "conditional", "none", "danger"}
var ganttLegendKeys = []string{"support", "conditional", "none", "danger", "risk"}

func ganttDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject(chartFields)(p, n, path)
}

var (
	chartFields     []gField
	tickFields      []gField
	milestoneFields []gField
	groupFields     []gField
	rowFields       []gField
	segmentFields   []gField
	rowNoteFields   []gField
	annFields       []gField
)

func init() {
	segmentFields = []gField{
		{"from", gDate, true}, {"to", gDate, true}, {"status", gEnum(ganttStatuses), true},
		{"label", gInline, false}, {"openEnd", gBool, false}, {"tentative", gBool, false},
	}
	rowNoteFields = []gField{{"from", gDate, true}, {"to", gDate, true}, {"text", gInline, true}}
	rowFields = []gField{
		{"id", gID, false}, {"label", gInline, true}, {"sub", gString, false},
		{"segments", gSeq(gObject(segmentFields), 0), false}, {"note", gObject(rowNoteFields), false},
	}
	groupFields = []gField{
		{"id", gID, false}, {"label", gLines, true}, {"risk", gRange, false},
		{"rows", gSeq(gRow, 1), true},
	}
	tickFields = []gField{{"at", gDate, true}, {"label", gInline, false}, {"status", gEnum(ganttStatuses), false}}
	milestoneFields = []gField{{"at", gDate, true}, {"label", gInline, true}}
	annFields = []gField{
		{"at", gDate, true}, {"target", gID, false}, {"text", gInline, true},
		{"status", gEnum(ganttStatuses), false}, {"emphasis", gBool, false},
	}
	chartFields = []gField{
		{"span", gRange, true}, {"axisBreakBefore", gDate, false},
		{"ticks", gSeq(gObject(tickFields), 0), false},
		{"milestones", gSeq(gObject(milestoneFields), 0), false},
		{"legend", gLegend, false},
		{"groups", gSeq(gObject(groupFields), 1), true},
		{"annotations", gSeq(gObject(annFields), 0), false},
	}
}

func gKind(n *yNode) string {
	switch n.kind {
	case 'm':
		return "対応表"
	case 'q':
		return "列"
	case 'n':
		return "空"
	}
	return "文字列"
}

func gFail(p *mdParser, n *yNode, path, code, msg string, expected []string, hint string) {
	p.fail(n.line, n.col, path, code, msg, expected, hint)
}

// gObject は決まったキーだけを持つ対応表。書いた順に出す。
func gObject(fields []gField) ganttConv {
	return func(p *mdParser, n *yNode, path string) any {
		if n.kind != 'm' {
			gFail(p, n, path, codeInvalidValue, fmt.Sprintf("ここには「キー: 値」の対応表を書いてください(%sが書かれています)", gKind(n)), nil, "")
			return nil
		}
		var names []string
		for _, f := range fields {
			names = append(names, f.name)
		}
		out := &jobj{}
		have := map[string]bool{}
		for k, key := range n.keys {
			var field *gField
			for i := range fields {
				if fields[i].name == key {
					field = &fields[i]
				}
			}
			kp := path + "/" + key
			p.mark(kp, n.keyLine[k])
			if field == nil {
				hint := ""
				switch key {
				case "color", "colour", "x", "y", "direction", "position", "above", "below", "size", "align":
					hint = "見た目と置き場所はサブシステムとテンプレートが決めます"
				}
				p.fail(n.keyLine[k], n.vals[k].col, path, codeUnknownKey, fmt.Sprintf("キー %s はありません", key), names, hint)
				continue
			}
			have[key] = true
			if v := field.conv(p, n.vals[k], kp); v != nil {
				out.set(key, v)
			}
		}
		for _, f := range fields {
			if f.required && !have[f.name] {
				gFail(p, n, path, codeMissingKey, fmt.Sprintf("キー %s がありません", f.name), []string{f.name}, "")
			}
		}
		return out
	}
}

// gRow は行。segments か note のどちらかが要る。
func gRow(p *mdParser, n *yNode, path string) any {
	v := gObject(rowFields)(p, n, path)
	if o, ok := v.(*jobj); ok {
		has := map[string]bool{}
		for _, k := range o.keys {
			has[k] = true
		}
		if !has["segments"] && !has["note"] {
			gFail(p, n, path, codeMissingKey, "行には segments か note が要ります", []string{"segments", "note"}, "")
		}
	}
	return v
}

func gSeq(item ganttConv, minItems int) ganttConv {
	return func(p *mdParser, n *yNode, path string) any {
		if n.kind != 'q' {
			gFail(p, n, path, codeInvalidValue, fmt.Sprintf("ここには列(- …)を書いてください(%sが書かれています)", gKind(n)), nil, "")
			return nil
		}
		if len(n.items) < minItems {
			gFail(p, n, path, codeInvalidValue, fmt.Sprintf("項目が %d つ以上要ります", minItems), nil, "")
		}
		out := []any{}
		for k, it := range n.items {
			p.mark(fmt.Sprintf("%s/%d", path, k), it.line)
			if v := item(p, it, fmt.Sprintf("%s/%d", path, k)); v != nil {
				out = append(out, v)
			}
		}
		return out
	}
}

func gScalar(p *mdParser, n *yNode, path, what string) (string, bool) {
	if n.kind != 's' {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("ここには%sを書いてください(%sが書かれています)", what, gKind(n)), nil, "")
		return "", false
	}
	return n.str, true
}

func gString(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "文字列")
	if !ok {
		return nil
	}
	if s == "" {
		gFail(p, n, path, codeInvalidValue, "空の文字列は書けません", nil, "")
		return nil
	}
	return s
}

func gDate(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "日付")
	if !ok {
		return nil
	}
	if !reDate.MatchString(s) {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("日付 %q の形が違います", s), nil, "YYYY-MM か YYYY-MM-DD で書いてください(例 2029-12)")
		return nil
	}
	return s
}

func gID(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "id")
	if !ok {
		return nil
	}
	if !reID.MatchString(s) {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("id %q の形が違います", s), nil, "英字で始め、英数字・_・- で書いてください")
		return nil
	}
	return s
}

func gBool(p *mdParser, n *yNode, path string) any {
	b, ok := yBool(n)
	if !ok {
		gFail(p, n, path, codeInvalidValue, "ここには true か false を書いてください", []string{"true", "false"}, "yes・no・on・off は真偽値になりません")
		return nil
	}
	return b
}

func gEnum(values []string) ganttConv {
	return func(p *mdParser, n *yNode, path string) any {
		s, ok := gScalar(p, n, path, "状態の名前")
		if !ok {
			return nil
		}
		for _, v := range values {
			if v == s {
				return s
			}
		}
		hint := "状態は意味の名前で書きます。色はテンプレートが決めます"
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("状態 %q はありません", s), values, hint)
		return nil
	}
}

// gRange は「A .. B」か {from, to}。
func gRange(p *mdParser, n *yNode, path string) any {
	if n.kind == 'm' {
		return gObject([]gField{{"from", gDate, true}, {"to", gDate, true}})(p, n, path)
	}
	s, ok := gScalar(p, n, path, "範囲")
	if !ok {
		return nil
	}
	a, b, found := strings.Cut(s, "..")
	if !found {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("範囲 %q の形が違います", s), nil, "「2026-09 .. 2034-11」のように .. でつないでください")
		return nil
	}
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if !reDate.MatchString(a) || !reDate.MatchString(b) {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("範囲 %q の日付の形が違います", s), nil, "YYYY-MM か YYYY-MM-DD で書いてください")
		return nil
	}
	return obj("from", a, "to", b)
}

// gInline は文の値。列なら行を br でつなぐ。
func gInline(p *mdParser, n *yNode, path string) any {
	switch n.kind {
	case 's':
		if n.str == "" {
			gFail(p, n, path, codeInvalidValue, "空の文は書けません", nil, "")
			return nil
		}
		return p.parseInline(scalarText(n), path)
	case 'q':
		var t mdText
		for k, it := range n.items {
			if it.kind != 's' {
				gFail(p, it, path, codeInvalidValue, "文の列の項目は文字列にしてください", nil, "")
				return nil
			}
			if k > 0 {
				t.r = append(t.r, brRune)
				t.pos = append(t.pos, srcPos{it.line, it.col})
			}
			st := scalarText(it)
			t.r = append(t.r, st.r...)
			t.pos = append(t.pos, st.pos...)
		}
		if len(t.r) == 0 {
			gFail(p, n, path, codeInvalidValue, "空の文は書けません", nil, "")
			return nil
		}
		return p.parseInline(t, path)
	}
	gFail(p, n, path, codeInvalidValue, fmt.Sprintf("ここには文を書いてください(%sが書かれています)", gKind(n)), nil, "")
	return nil
}

// gLines は行の配列(文字列か、文字列の列)。
func gLines(p *mdParser, n *yNode, path string) any {
	switch n.kind {
	case 's':
		return gString(p, n, path)
	case 'q':
		var out []any
		for k, it := range n.items {
			if v := gString(p, it, fmt.Sprintf("%s/%d", path, k)); v != nil {
				out = append(out, v)
			}
		}
		if len(out) == 0 {
			gFail(p, n, path, codeInvalidValue, "行が1つも無い", nil, "")
			return nil
		}
		return out
	}
	gFail(p, n, path, codeInvalidValue, "ここには文字列か文字列の列を書いてください", nil, "")
	return nil
}

// gLegend は凡例: キーは状態の名前か risk、値は文。書いた順を保つ。
func gLegend(p *mdParser, n *yNode, path string) any {
	if n.kind != 'm' {
		gFail(p, n, path, codeInvalidValue, "凡例は「状態の名前: 説明」の対応表で書いてください", nil, "")
		return nil
	}
	out := &jobj{}
	for k, key := range n.keys {
		ok := false
		for _, v := range ganttLegendKeys {
			if v == key {
				ok = true
			}
		}
		if !ok {
			p.fail(n.keyLine[k], n.vals[k].col, path, codeUnknownKey, fmt.Sprintf("凡例のキー %s はありません", key), ganttLegendKeys, "")
			continue
		}
		if v := gInline(p, n.vals[k], path+"/"+key); v != nil {
			out.set(key, v)
		}
	}
	return out
}
