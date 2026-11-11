package boxloom

// 根拠の語彙(#47):スライドの出典(source)・文書の機密区分と免責
// (meta.confidential・meta.disclaimer)・日付の書き方(marks.date)・表の合計の行
// (table の total)。

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

const evidenceDoc = `---
title: 例
author: 企画部
date: 2026-11-01
confidential: 社外秘
disclaimer: 数字はすべて架空です。
---

# 費用は人件費が大半を占める

| 費目 | 金額 |
| --- | --- |
| 人件費 | 9,100 |
| 設備 | 1,300 |
| 合計 | 10,400 |
: 見積もり {total=last}

:::source
社内の見積もり(2026 年 10 月)
:::

# 需要ははっきりある {type=statement}

:::source
38 社への聞き取り
:::
`

func TestMarkdownEvidenceVocabulary(t *testing.T) {
	v, errs := mdToAny(t, evidenceDoc)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	meta := getPath(v, "meta").(map[string]any)
	if meta["confidential"] != "社外秘" || meta["disclaimer"] != "数字はすべて架空です。" || meta["date"] != "2026-11-01" {
		t.Fatalf("meta: %v", meta)
	}
	slides := getPath(v, "slides").([]any)
	content := slides[1].(map[string]any)
	if content["source"] != "社内の見積もり(2026 年 10 月)" {
		t.Fatalf("content source: %v", content["source"])
	}
	tbl := content["body"].([]any)[0].(map[string]any)
	if rows := tbl["rows"].([]any); len(rows) != 2 {
		t.Fatalf("the total row must leave rows: %v", rows)
	}
	if tot, _ := json.Marshal(tbl["total"]); string(tot) != `["合計","10,400"]` {
		t.Fatalf("total: %s", tot)
	}
	if tbl["caption"] != "見積もり" {
		t.Fatalf("caption without the attributes: %v", tbl["caption"])
	}
	if st := slides[2].(map[string]any); st["source"] != "38 社への聞き取り" {
		t.Fatalf("statement source: %v", st)
	}
	// 作った IR は Go でも読める。
	b, _ := json.Marshal(v)
	d, err := ParseIR(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.Slides[1].Source.Plain() == "" || d.Slides[1].Body[0].Total == nil || d.Meta.Confidential.Plain() != "社外秘" {
		t.Fatalf("Go IR lost a field: %+v", d.Slides[1])
	}
}

func TestMarkdownEvidenceErrors(t *testing.T) {
	head := "---\ntitle: 例\n---\n\n"
	cases := []struct {
		name, src, code string
	}{
		{"section", head + "# 第1部 {type=section}\n\n:::source\nどこか\n:::\n", codeMisplaced},
		{"two", head + "# 題\n\n本文\n\n:::source\nA\n:::\n\n:::source\nB\n:::\n", codeMisplaced},
		{"inside", head + "# 題\n\n::::columns\n:::column\n本文\n:::\n:::column\n本文\n\n:::source\nA\n:::\n:::\n::::\n", codeMisplaced},
		{"before first slide", head + ":::source\nA\n:::\n\n# 題\n\n本文\n", codeMisplaced},
		{"label", head + "# 題\n\n本文\n\n:::source 見出し\nA\n:::\n", codeMisplaced},
		{"empty", head + "# 題\n\n本文\n\n:::source\n:::\n", codeMisplaced},
		{"total value", head + "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n| y | 2 |\n: {total=first}\n", codeInvalidValue},
		{"total alone", head + "# 題\n\n| a | b |\n| --- | --- |\n| 合計 | 1 |\n: {total=last}\n", codeMisplaced},
		{"table attr", head + "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n: 説明 {align=right}\n", codeUnknownAttribute},
		{"front matter", "---\ntitle: 例\nconfidential:\n---\n\n# 題\n\n本文\n", codeInvalidValue},
	}
	for _, c := range cases {
		_, errs := mdToAny(t, c.src)
		found := false
		for _, e := range errs {
			if e.Code == c.code {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want %s, got %+v", c.name, c.code, errs)
		}
	}
}

// 表の説明の行の属性だけ(説明の文なし)でも合計の行を書ける。
func TestMarkdownTotalWithoutCaption(t *testing.T) {
	v, errs := mdToAny(t, "# 題\n\n| a | b |\n| --- | --- |\n| x | 1 |\n| 計 | 1 |\n: {total=last}\n")
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	tbl := getPath(v, "slides").([]any)[0].(map[string]any)["body"].([]any)[0].(map[string]any)
	if _, ok := tbl["caption"]; ok || tbl["total"] == nil {
		t.Fatalf("want total and no caption: %v", tbl)
	}
}

func TestIRTotalShape(t *testing.T) {
	src := `{"boxloom":"0.1","slides":[{"type":"content","title":"t","body":[{"type":"table","header":["a","b"],"rows":[["x","1"]],"total":["計"]}]}]}`
	if _, err := ParseIR([]byte(src)); err == nil || !strings.Contains(err.Error(), "total") {
		t.Fatalf("want a total error, got %v", err)
	}
}

// 出典は本文の箱の外の枠なので、本文の領域(容量)はその分だけ狭くなる。
func TestSourceShrinksBodyArea(t *testing.T) {
	base := "# 題\n\n本文の段落\n"
	_, _, without := checkCapacity("a.bx.md", []byte(base), "light", nil, nil)
	_, _, with := checkCapacity("b.bx.md", []byte(base+"\n:::source\n出どころ\n:::\n"), "light", nil, nil)
	if len(without) != 1 || len(with) != 1 {
		t.Fatalf("capacity: %v %v", without, with)
	}
	tm, _ := LookupTemplate("light")
	want := tm.Values.SectionGap + tm.Values.Small*lineHeightMult
	if d := without[0].BodyArea - with[0].BodyArea; abs(d-want) > 0.2 {
		t.Fatalf("body area shrank by %.1f, want about %.1f", d, want)
	}
}

// 出典がはみ出しの測りに入る:本文がぎりぎり入るスライドに出典を足すと
// layout-overflow になる。
func TestSourceCountsForOverflow(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 題\n\n")
	for i := 0; i < 40; i++ {
		_, _, caps := checkCapacity("a.bx.md", []byte(b.String()+"- 項目\n"), "light", nil, nil)
		if len(caps) == 1 && caps[0].Remaining < caps[0].BodyLineHeight {
			break
		}
		b.WriteString("- 項目\n")
	}
	_, warns, _ := checkCapacity("a.bx.md", []byte(b.String()), "light", nil, nil)
	for _, w := range warns {
		if w.Code == "layout-overflow" {
			t.Fatalf("no overflow expected before the source: %+v", w)
		}
	}
	_, warns, _ = checkCapacity("a.bx.md", []byte(b.String()+"\n:::source\n出どころの1行目\\\n2行目\\\n3行目\n:::\n"), "light", nil, nil)
	found := false
	for _, w := range warns {
		if w.Code == "layout-overflow" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want layout-overflow once the source takes room: %+v", warns)
	}
}

func TestDateMarks(t *testing.T) {
	m := TemplateMarks{Date: "{YYYY}年{M}月{D}日", YearMonth: "{YYYY}/{MM}"}
	if got := m.formatDate("2026-03-05"); got != "2026年3月5日" {
		t.Errorf("date: %q", got)
	}
	if got := m.formatDate("2026-03"); got != "2026/03" {
		t.Errorf("year-month: %q", got)
	}
	for _, c := range []struct {
		f       string
		withDay bool
		ok      bool
	}{
		{"{YYYY}年{M}月{D}日", true, true},
		{"{YYYY}-{MM}-{DD}", true, true},
		{"{YYYY}年{M}月", true, false},      // 日が無い
		{"{YYYY}年{M}月{D}日", false, false}, // 年月の書き方に日
		{"{YY}/{M}/{D}", true, false},     // 知らない印
		{"{M}月{D}日", true, false},         // 年が無い
	} {
		err := checkDateMark("date", c.f, c.withDay)
		if (err == nil) != c.ok {
			t.Errorf("%q withDay=%v: err=%v", c.f, c.withDay, err)
		}
	}
}

// PPTX:表紙に日付・機密区分・免責、本文のスライドに出典とフッターの機密区分、
// 合計の行は lastRow と太字。
func TestEvidenceInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(evidenceDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		for _, f := range zr.File {
			if f.Name == name {
				rc, _ := f.Open()
				b, _ := io.ReadAll(rc)
				rc.Close()
				return string(b)
			}
		}
		t.Fatalf("no %s", name)
		return ""
	}
	cover := read("ppt/slides/slide1.xml")
	for _, s := range []string{"2026年11月1日", "社外秘", "数字はすべて架空です。"} {
		if !strings.Contains(cover, s) {
			t.Errorf("cover lacks %q", s)
		}
	}
	s2 := read("ppt/slides/slide2.xml")
	for _, s := range []string{"出典:", "社内の見積もり", "社外秘", `lastRow="1"`} {
		if !strings.Contains(s2, s) {
			t.Errorf("slide 2 lacks %q", s)
		}
	}
	if !strings.Contains(read("ppt/slides/slide3.xml"), "38 社への聞き取り") {
		t.Error("statement slide lacks its source")
	}
	// 合計の行のセルは太字。
	k := strings.Index(s2, "10,400")
	if k < 0 || !strings.Contains(s2[strings.LastIndex(s2[:k], "<a:rPr"):k], `b="1"`) {
		t.Error("the total row must be bold")
	}
}

// 例(examples/md/evidence.bx.md)は警告も誤りも出さない。
func TestEvidenceExampleClean(t *testing.T) {
	if _, err := os.Stat("../examples/md/evidence.bx.md"); err != nil {
		t.Skip(err)
	}
	for _, tmpl := range []string{"light", "dark"} {
		errs, warns := CheckFile("../examples/md/evidence.bx.md", tmpl)
		if len(errs)+len(warns) > 0 {
			t.Errorf("%s: %+v %+v", tmpl, errs, warns)
		}
	}
}
