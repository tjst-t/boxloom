// 出典の無い本文のスライドの警告(source-missing、#166)。
//
// 数・数の表・グラフを見せる本文のスライドは、その数の出どころ(スライドの source)を
// 持つ(SKILL.md)。持たなければ警告を出す。内容の良し悪しは判定しないが、「出典の欄が
// 空か」は量で測れるので check が見る。資料の数が架空(例・練習・デモ)なら meta の
// fictional で切る。テンプレートでは切らない:出典が要るかは資料の意味で、見た目の方針
// ではないため。

package boxloom

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// codeSourceMissing は出典の無い本文のスライドの警告の code。
const codeSourceMissing = "source-missing"

// 表は数の表のときだけ出典が要る:本体か合計の行に、数だけのセル(右に揃える数の列と同じ
// numericCellRe)が1つでもあること。予定・リスク・比べの表のように文字の表は要らない。
// 「1月」「2026年」のような時点だけのセルは数に数えない(予定の表の時期の列)。
var dateCellRe = regexp.MustCompile(`^[0-9]+(年|月|日)$`)

func numberCell(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && numericCellRe.MatchString(s) && !dateCellRe.MatchString(s)
}

func irTableHasNumbers(b IRBlock) bool {
	for _, r := range append(append([][]IRTableCell{}, b.Rows...), b.Total) {
		for _, c := range r {
			if numberCell(c.Plain()) {
				return true
			}
		}
	}
	return false
}

// rawTableHasNumbers は JSON のままの表(標準でない型の中)の irTableHasNumbers。
// セルの中の箇条({points})は数のセルに数えない。
func rawTableHasNumbers(m map[string]any) bool {
	rows, _ := m["rows"].([]any)
	if t, ok := m["total"].([]any); ok {
		rows = append(rows, t)
	}
	for _, r := range rows {
		cells, _ := r.([]any)
		for _, c := range cells {
			if _, pts := c.(map[string]any); !pts && numberCell(inlinePlainAny(c)) {
				return true
			}
		}
	}
	return false
}

// sourceSubsystems は数を持つサブシステム(出典が要る図)。
var sourceSubsystems = map[string]bool{"chart": true, "funnel": true}

// sourceIssues は数・表・グラフのある本文のスライドで source が無いものに警告を出す
// (スライドに1件。path はスライド、文に最初に見つけたブロックの場所)。
func sourceIssues(file string, deck *IRDeck) []Issue {
	if deck == nil || (deck.Meta != nil && deck.Meta.Fictional) {
		return nil
	}
	var out []Issue
	for si, s := range deck.Slides {
		if s.Type != "content" || len(s.Source) > 0 {
			continue
		}
		sp := fmt.Sprintf("/slides/%d", si)
		at, kind := needsSource(s.Body, sp+"/body")
		if at == "" {
			continue
		}
		out = append(out, Issue{
			File: file, Path: sp, Code: codeSourceMissing,
			Message: fmt.Sprintf("the slide shows %s (%s) but has no source", kind, at),
			Hint:    "スライドに :::source で数の出どころ(調査・資料の名前、時点、n)を書いてください。資料の数が架空(例・練習)なら、文書の頭に fictional: true を書くと出ません",
		})
	}
	return out
}

// needsSource は本文のブロックのうち、出典が要る最初のブロックの場所と種類を返す
// (無ければ "")。並列のまとまりの中と、標準でない型(pairs・topics など)の中の
// ブロックも見る。
func needsSource(body []IRBlock, bp string) (string, string) {
	for i, b := range body {
		p := fmt.Sprintf("%s/%d", bp, i)
		switch {
		case b.Type == "table" && irTableHasNumbers(b):
			return p, "a table of numbers"
		case b.Type == "stat":
			return p, "numbers (stat)"
		case b.Type == "subsystem" && sourceSubsystems[b.Subsystem]:
			return p, "a " + b.Subsystem
		case b.Type == "columns":
			for ci, c := range b.Columns {
				if at, k := needsSource(c.Body, fmt.Sprintf("%s/columns/%d/body", p, ci)); at != "" {
					return at, k
				}
			}
		case len(b.Raw) > 0:
			var v any
			if json.Unmarshal(b.Raw, &v) == nil {
				if at, k := rawNeedsSource(v, p); at != "" {
					return at, k
				}
			}
		}
	}
	return "", ""
}

// rawNeedsSource は標準でない型のブロックの JSON の中を歩き、出典が要るブロックを探す。
func rawNeedsSource(v any, p string) (string, string) {
	found, kind := "", ""
	walkAll(v, p, func(ptr string, x any) {
		if found != "" || ptr == p {
			return
		}
		m, ok := x.(map[string]any)
		if !ok {
			return
		}
		switch t, _ := m["type"].(string); {
		case t == "table" && rawTableHasNumbers(m):
			found, kind = ptr, "a table of numbers"
		case t == "stat":
			found, kind = ptr, "numbers (stat)"
		case t == "subsystem":
			if n, _ := m["subsystem"].(string); sourceSubsystems[n] {
				found, kind = ptr, "a "+n
			}
		}
	})
	return found, kind
}

// insertBySlide は add(スライドの場所の警告)を、warns のうち後のスライドの最初の警告の
// 前に入れる(警告をスライドの順に読めるように。warns の中の順は変えない)。
func insertBySlide(warns, add []Issue) []Issue {
	for _, a := range add {
		at := len(warns)
		for i, w := range warns {
			if slideIndex(w.Path) > slideIndex(a.Path) {
				at = i
				break
			}
		}
		warns = append(warns[:at], append([]Issue{a}, warns[at:]...)...)
	}
	return warns
}

// slideIndex は /slides/S… の S(無ければ -1)。
func slideIndex(path string) int {
	rest, ok := strings.CutPrefix(path, "/slides/")
	if !ok {
		return -1
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return -1
	}
	return n
}
