package main

import (
	"fmt"
	"strings"
)

// 発表者ノート(:::notes)。
//
// スライドの本文の一番外側に :::notes … ::: を1つ置くと、その中身がスライドの
// notes(IR では文字列)になる。最初のスライド(# 題)より前に置いた :::notes は
// 表紙のノートになる(表紙があるときだけ)。中に書けるのは段落と箇条だけ。
// 文中の記法(**…**・`…`・[文](URL)・行末の \)は本文と同じく検査したうえで、
// 文字だけにする(ノートは見た目を持たないため):強調とコードは中の文字、
// リンクは「文 (URL)」、意味のある改行は改行。段落と箇条の項目は1行ずつ
// (PPTX では1段落ずつ)になり、入れ子の項目は前に空白2つを付ける。

// splitNotes はスライドの一番外側の項目から :::notes を取り出す。
func splitNotes(items []*mdItem) (body []*mdItem, notes []*mdFenceBlk) {
	for _, it := range items {
		if it.kind == mdFence && it.fence.name == "notes" {
			notes = append(notes, it.fence)
			continue
		}
		body = append(body, it)
	}
	return body, notes
}

// notesText は :::notes の並び(ふつうは1つ)を IR の notes の文字列にする。
// 無ければ ok=false。
func (p *mdParser) notesText(fs []*mdFenceBlk, path string) (string, bool) {
	if len(fs) == 0 {
		return "", false
	}
	p.mark(path, fs[0].line)
	for _, f := range fs[1:] {
		p.fail(f.line, 1, path, codeMisplaced, "発表者ノート(:::notes)は1枚に1つだけ書けます", nil, "1つの :::notes にまとめてください")
	}
	f := fs[0]
	p.fenceAttrs(f, path, nil)
	p.noLabel(f, path)
	var lines []string
	for _, it := range f.items {
		switch it.kind {
		case mdPara:
			lines = append(lines, notesPlain(p.parseInline(it.text.trimBR(), path)))
		case mdList:
			var walk func(ents []*mdListEnt, prefix string)
			walk = func(ents []*mdListEnt, prefix string) {
				for _, e := range ents {
					t := notesPlain(p.parseInline(e.text.trimBR(), path))
					lines = append(lines, prefix+"- "+strings.ReplaceAll(t, "\n", "\n"+prefix+"  "))
					walk(e.children, prefix+"  ")
				}
			}
			walk(it.items, "")
		default:
			what := "囲み"
			if it.kind == mdCode {
				what = "コードブロック"
			}
			p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::notes の中に%sは書けません", what), nil, "発表者ノートには段落と箇条だけを書けます")
		}
	}
	if len(lines) == 0 {
		p.fail(f.line, 1, path, codeMisplaced, ":::notes の中が空です", nil, "")
		return "", false
	}
	return strings.Join(lines, "\n"), true
}

// notesPlain は IR の文中の値(文字列か、文字列と em・code・link・br の並び)を
// 文字だけにする。
func notesPlain(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, e := range x {
			b.WriteString(notesPlain(e))
		}
		return b.String()
	case *jobj:
		get := func(k string) any {
			for i, kk := range x.keys {
				if kk == k {
					return x.vals[i]
				}
			}
			return nil
		}
		switch get("type") {
		case "br":
			return "\n"
		case "code":
			s, _ := get("text").(string)
			return s
		case "em":
			return notesPlain(get("content"))
		case "link":
			text := notesPlain(get("content"))
			href, _ := get("href").(string)
			if href != "" && href != text {
				return text + " (" + href + ")"
			}
			return text
		}
	}
	return ""
}
