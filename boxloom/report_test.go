package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
)

// reportPairsDeck は架空の対の1枚:左に2行ほどの「分かったこと」n、右にそれぞれ
// 3項目の打ち手。
func reportPairsDeck(n int) []byte {
	var b strings.Builder
	b.WriteString("---\ntitle: 読む資料の試し\n---\n\n# 窓口の待ち時間を見直す\n\n")
	b.WriteString(":::lead\n聞き取りと記録から分かったことごとに、打ち手を3つずつ決めた\n:::\n\n")
	b.WriteString(":::::pairs 分かったこと | 打ち手\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, ":::subject\n分かったこと %d:月曜と金曜の午前に来る人が全体の半分を占め、そのうち三割は書類の不足で二度来ている\n:::\n", i)
		fmt.Fprintf(&b, ":::response\n- 打ち手 %d-1:予約の枠を午後に広げる\n- 打ち手 %d-2:要る書類の一覧を前もって送る\n- 打ち手 %d-3:混む時間を案内に載せる\n:::\n", i, i, i)
	}
	b.WriteString(":::::\n")
	return []byte(b.String())
}

// report は pairs を自分の組み方で組む(component-fallback は出ない)。14pt の本文で
// 3つの対が1枚に収まる。
func TestReportPairs(t *testing.T) {
	res := Check("pairs.bx.md", reportPairsDeck(3), CheckOptions{Template: "report"})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	for _, w := range res.Warnings {
		switch w.Code {
		case "layout-overflow", "component-fallback", "title-wrap":
			t.Errorf("report: %+v", w)
		}
	}
}

// report の値は読む資料のサンプルを測った値(Backlog #83。人の決定)。
func TestReportMeasuredValues(t *testing.T) {
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	v := tm.Values
	for name, c := range map[string][2]float64{
		"pageWidth": {v.PageW, 960}, "pageHeight": {v.PageH, 540},
		"pagePadding": {v.PagePadding, 50}, "pagePaddingY": {v.PagePaddingY, 42},
		"leadText": {v.LeadText, 24}, "slideTitle": {v.SlideTitle, 16},
		"body": {v.Body, 14}, "subPoint": {v.SubPoint, 12}, "tableText": {v.TableText, 11},
		"small": {v.Small, 10}, "columnTitle": {v.ColumnTitle, 16},
		"fillRatioMin": {v.FillRatioMin, 0.65}, "spacingUnit": {tm.Scale.SpacingUnit, 2},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %v, want %v", name, c[0], c[1])
		}
	}
	for role, want := range map[string]string{
		"bg": "FFFFFF", "text_primary": "575757", "text_muted": "7F7F7F", "grid": "9A9A9A",
		"card": "F1F2F1", "accent": "29BA74", "support": "187955", "danger": "03522C", "conditional": "30C1D7",
	} {
		if got := tm.Theme[role]; got != want {
			t.Errorf("theme.%s = %s, want %s", role, got, want)
		}
	}
	if want := []float64{8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 32, 54}; fmt.Sprint(tm.Scale.TypeSizes) != fmt.Sprint(want) {
		t.Errorf("typeSizes = %v, want %v", tm.Scale.TypeSizes, want)
	}
}

// reportHeaderDeck は本文のスライド2枚:1枚目はリードあり、2枚目はリードなし。
const reportHeaderDeck = `---
title: 頭の試し
---

# 窓口の待ち時間

:::lead
午前の混雑は予約の枠を午後に広げれば半分にできる
:::

- 月曜と金曜の午前に来る人が全体の半分を占める

# 次の打ち手

- 要る書類の一覧を前もって送る
`

// runColor は文字の葉の色(塊があれば最初の塊の色)。
func runColor(s *Spec) string {
	if len(s.Runs) > 0 {
		return s.Runs[0].Color
	}
	return s.Color
}

func leafWithPlain(s *Spec, text string) *Spec {
	if s.Kind == KindText && strings.Contains(s.Text+runsText(s.Runs), text) {
		return s
	}
	for _, c := range s.Children {
		if r := leafWithPlain(c, text); r != nil {
			return r
		}
	}
	return nil
}

// report の本文のスライドの頭:リードが上(24pt・基調色)、題が下(16pt・字の色)で、
// 題は title のプレースホルダー。リードの無いスライドは題が上の見出しの大きさ。
func TestReportContentHeader(t *testing.T) {
	deck, _, err := ParseMarkdown("h.bx.md", []byte(reportHeaderDeck))
	if err != nil {
		t.Fatal(err)
	}
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	accent, text := tm.Theme["accent"], tm.Theme["text_primary"]
	var b *BuiltSlide
	for _, s := range built {
		computeLayout(s.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	}
	b = built[1] // 1枚目は表紙
	lead := leafWithPlain(b.Root, "午前の混雑")
	if lead == nil || b.Title == nil {
		t.Fatalf("lead %v title %v", lead, b.Title)
	}
	if b.Title.Placeholder != "title" || !strings.Contains(b.Title.Text, "窓口の待ち時間") {
		t.Errorf("title leaf = %q (placeholder %q)", b.Title.Text, b.Title.Placeholder)
	}
	if lead.FontSize != 24 || runColor(lead) != accent {
		t.Errorf("lead: %vpt %s, want 24pt %s", lead.FontSize, runColor(lead), accent)
	}
	if b.Title.FontSize != 16 || runColor(b.Title) != text {
		t.Errorf("title: %vpt %s, want 16pt %s", b.Title.FontSize, runColor(b.Title), text)
	}
	if !(lead.Box.Y+lead.Box.Height <= b.Title.Box.Y) {
		t.Errorf("lead (y %.1f h %.1f) must be above the title (y %.1f)", lead.Box.Y, lead.Box.Height, b.Title.Box.Y)
	}
	if lead.Box.Y != tm.Values.PagePaddingY {
		t.Errorf("lead y = %.1f, want the top padding %.1f", lead.Box.Y, tm.Values.PagePaddingY)
	}
	// リードの無いスライドは題が上の見出しの位置と大きさ。
	nb := built[2]
	if nb.Title == nil || nb.Title.FontSize != 24 || runColor(nb.Title) != accent || nb.Title.Box.Y != tm.Values.PagePaddingY {
		t.Errorf("no-lead title = %+v", nb.Title)
	}

	// PPTX でも題は title のプレースホルダーで、リードの下にある。
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{Template: "report"}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var slide string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide2.xml" {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			slide = string(data)
		}
	}
	ph := strings.Index(slide, `<p:ph type="title"/>`)
	if ph < 0 || strings.Count(slide, `<p:ph type="title"/>`) != 1 {
		t.Fatal("content slide must have exactly one title placeholder")
	}
	end := strings.Index(slide[ph:], "</p:sp>")
	if !strings.Contains(slide[ph:ph+end], "窓口の待ち時間") || strings.Contains(slide[ph:ph+end], "午前の混雑") {
		t.Error("the title placeholder must hold the title, not the lead")
	}
}

// dark・light の頭は標準のまま(題が上・リードが下)。
func TestReportOverrideIsOnlyReport(t *testing.T) {
	deck, _, err := ParseMarkdown("h.bx.md", []byte(reportHeaderDeck))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dark", "light"} {
		tm, _ := LookupTemplate(name)
		built, err := tm.BuildDeck(deck)
		if err != nil {
			t.Fatal(err)
		}
		b := built[1]
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		lead := leafWithPlain(b.Root, "午前の混雑")
		if lead == nil || b.Title == nil || !(b.Title.Box.Y < lead.Box.Y) || b.Title.FontSize != tm.Values.SlideTitle {
			t.Errorf("%s: the standard header must keep the title above the lead", name)
		}
	}
}

// reportColumnsDeck は列の見出しのある並列のまとまりと、対の並びの2枚。
const reportColumnsDeck = `---
title: 見出しの試し
---

# 二つの案

::::columns
:::column 今の窓口
- 午前に混む
:::
:::column 新しい窓口
- 予約で分ける
:::
::::

# 窓口の見直し

:::::pairs 分かったこと | 打ち手
:::subject
午前に混む
:::
:::response
- 予約の枠を広げる
:::
:::subject
書類が足りない
:::
:::response
- 一覧を前もって送る
:::
:::::
`

// report の列の見出し(まとまりの題・対の側の見出し)は 16pt・基調色。dark・light は
// 標準のまま字の色(text_primary)。
func TestReportColumnHeadings(t *testing.T) {
	deck, issues, err := ParseMarkdown("c.bx.md", []byte(reportColumnsDeck))
	if err != nil || len(issues) > 0 || deck == nil {
		t.Fatalf("%v %+v", err, issues)
	}
	for _, c := range []struct{ name, role string }{{"report", "accent"}, {"light", "text_primary"}, {"dark", "text_primary"}} {
		tm, err := LookupTemplate(c.name)
		if err != nil {
			t.Fatal(err)
		}
		built, err := tm.BuildDeck(deck)
		if err != nil {
			t.Fatal(err)
		}
		want := tm.Theme[c.role]
		for _, h := range []struct {
			slide int
			text  string
		}{{1, "今の窓口"}, {1, "新しい窓口"}, {2, "分かったこと"}, {2, "打ち手"}} {
			leaf := leafWithPlain(built[h.slide].Root, h.text)
			if leaf == nil {
				t.Fatalf("%s: heading %q not found", c.name, h.text)
			}
			if got := runColor(leaf); got != want || leaf.FontSize != tm.Values.ColumnTitle {
				t.Errorf("%s: heading %q = %vpt %s, want %vpt %s (%s)", c.name, h.text, leaf.FontSize, got, tm.Values.ColumnTitle, want, c.role)
			}
		}
		if c.name == "report" && (tm.Values.ColumnTitle != 16 || want != "29BA74") {
			t.Errorf("report: columnTitle %v / accent %s, want 16 / 29BA74", tm.Values.ColumnTitle, want)
		}
	}
}

// reportCalloutsDeck は行の群のある表・結論・引用を1枚に置く(文はすべて架空)。
const reportCalloutsDeck = `---
title: 表と結論と引用の試し
---

# 窓口の見直し

| 群 | 項目 | いま |
| --- | --- | --- |
| 手続き | 書類の確認 | 二度来る |
| | 予約 | 電話だけ |
| 体制 | 窓口 | 二つ |
: 架空の窓口の記録 {groups=first}

:::quote 窓口の担当、架空の聞き取り
予約が取れるようになって、午前の列が短くなった。
:::

:::takeaway
書類の一覧を前もって送ることから始める
:::
`

// builtSlideXML は deck を tmpl で組んだ PPTX の n 枚目のスライドの XML。
func builtSlideXML(t *testing.T, src, tmpl string, n int) string {
	t.Helper()
	deck, issues, err := ParseMarkdown("c.bx.md", []byte(src))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{Template: tmpl}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == fmt.Sprintf("ppt/slides/slide%d.xml", n) {
			r, _ := f.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			return string(data)
		}
	}
	t.Fatalf("slide %d not found", n)
	return ""
}

// tableCellsXML は表の行ごとのセル(a:tc)の XML。
func tableCellsXML(slide string) [][]string {
	var rows [][]string
	for _, tr := range strings.Split(slide, "<a:tr ")[1:] {
		tr = tr[:strings.Index(tr, "</a:tr>")]
		var cells []string
		idx := regexp.MustCompile(`<a:tc[ >]`).FindAllStringIndex(tr, -1)
		for i, m := range idx {
			end := len(tr)
			if i+1 < len(idx) {
				end = idx[i+1][0]
			}
			cells = append(cells, tr[m[0]+len("<a:tc"):end])
		}
		rows = append(rows, cells)
	}
	return rows
}

// cellPart は a:tc の XML の中の、文(a:txBody)と書式(a:tcPr)の部分。
func cellPart(tc, tag string) string {
	i := strings.Index(tc, "<a:"+tag)
	j := strings.Index(tc, "</a:"+tag+">")
	if i < 0 || j < 0 {
		return ""
	}
	return tc[i:j]
}

// shapesWithGeom は prst の図形(p:sp)の XML。
func shapesWithGeom(slide, prst string) []string {
	var out []string
	for _, sp := range strings.Split(slide, "<p:sp>")[1:] {
		if strings.Contains(sp, `<a:prstGeom prst="`+prst+`"`) {
			out = append(out, sp[:strings.Index(sp, "</p:sp>")])
		}
	}
	return out
}

func fillOfXML(x string) string {
	const k = `<a:solidFill><a:srgbClr val="`
	i := strings.Index(x, k)
	if i < 0 {
		return ""
	}
	return x[i+len(k) : i+len(k)+6]
}

// report の表(#88):見出しの行は面なし・基調色の字・基調色の罫線、行の間は灰色の
// 罫線、群の名前のセルは濃い基調色の面に白い字。
func TestReportTableLook(t *testing.T) {
	slide := builtSlideXML(t, reportCalloutsDeck, "report", 2)
	rows := tableCellsXML(slide)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	for c, tc := range rows[0] {
		pr, body := cellPart(tc, "tcPr"), cellPart(tc, "txBody")
		if !strings.Contains(pr, "</a:lnB><a:noFill/>") && !strings.HasSuffix(pr, "<a:noFill/>") {
			t.Errorf("header cell %d must not be filled: %s", c, pr)
		}
		if fillOfXML(body) != "29BA74" {
			t.Errorf("header cell %d text = %s, want 29BA74", c, fillOfXML(body))
		}
		lnB := pr[strings.Index(pr, "<a:lnB"):]
		if fillOfXML(lnB) != "29BA74" || !strings.Contains(lnB, `w="19050"`) {
			t.Errorf("header cell %d rule = %s, want 29BA74 1.5pt", c, lnB[:strings.Index(lnB, "</a:lnB>")])
		}
	}
	// 本体の行:面なし、行の間は灰色の罫線。
	for r := 1; r < len(rows); r++ {
		for c, tc := range rows[r] {
			pr := cellPart(tc, "tcPr")
			lnB := pr[strings.Index(pr, "<a:lnB"):strings.Index(pr, "</a:lnB>")]
			if c == 0 {
				continue // 群の名前のセル(下で見る)
			}
			if !strings.HasSuffix(pr, "<a:noFill/>") {
				t.Errorf("body cell %d,%d must not be filled: %s", r, c, pr)
			}
			if fillOfXML(lnB) != "9A9A9A" {
				t.Errorf("body cell %d,%d rule = %s, want 9A9A9A", r, c, lnB)
			}
		}
	}
	// 群の名前のセル(結合の始まりと続き)は濃い基調色の面、名前は白い字。
	for _, r := range []int{1, 2, 3} {
		pr := cellPart(rows[r][0], "tcPr")
		if got := fillOfXML(pr[strings.LastIndex(pr, "</a:lnB>"):]); got != "187955" {
			t.Errorf("group cell row %d fill = %q, want 187955", r, got)
		}
	}
	if !strings.HasPrefix(rows[1][0], ` rowSpan="2"`) || !strings.HasPrefix(rows[2][0], ` vMerge="1"`) {
		t.Errorf("rows 1-2: the group cell must be merged: %.30s / %.30s", rows[1][0], rows[2][0])
	}
	for _, r := range []int{1, 3} {
		if got := fillOfXML(cellPart(rows[r][0], "txBody")); got != "FFFFFF" {
			t.Errorf("group name row %d text = %s, want FFFFFF", r, got)
		}
	}
}

// report の結論は丸い矢印(基調色の円と白い矢印)、引用は薄い面の吹き出し(角丸と尾)と
// 人の印(基調色の頭と肩)。dark・light は標準のまま(どの図形も出ない)。
func TestReportCallouts(t *testing.T) {
	slide := builtSlideXML(t, reportCalloutsDeck, "report", 2)
	ellipses := shapesWithGeom(slide, "ellipse")
	arrows := shapesWithGeom(slide, "rightArrow")
	shoulders := shapesWithGeom(slide, "round2SameRect")
	tails := shapesWithGeom(slide, "rtTriangle")
	if len(ellipses) != 2 || len(arrows) != 1 || len(shoulders) != 1 || len(tails) != 1 {
		t.Fatalf("ellipse %d rightArrow %d round2SameRect %d rtTriangle %d, want 2 1 1 1",
			len(ellipses), len(arrows), len(shoulders), len(tails))
	}
	for _, e := range ellipses {
		if fillOfXML(e) != "29BA74" {
			t.Errorf("circle/head fill = %s, want 29BA74", fillOfXML(e))
		}
	}
	if fillOfXML(arrows[0]) != "FFFFFF" || fillOfXML(shoulders[0]) != "29BA74" || fillOfXML(tails[0]) != "F1F2F1" {
		t.Errorf("arrow %s shoulders %s tail %s, want FFFFFF 29BA74 F1F2F1",
			fillOfXML(arrows[0]), fillOfXML(shoulders[0]), fillOfXML(tails[0]))
	}
	// 結論の円は直径 calloutMark(24pt)。
	if !strings.Contains(ellipses[len(ellipses)-1], `cx="304800" cy="304800"`) && !strings.Contains(ellipses[0], `cx="304800" cy="304800"`) {
		t.Error("the takeaway circle must be 24pt across")
	}
	// 引用の文は薄い面の角丸の吹き出しの中、結論の文は基調色の太字で地の面を持たない。
	var bubble string
	for _, sp := range shapesWithGeom(slide, "roundRect") {
		if strings.Contains(sp, "午前の列が短くなった") {
			bubble = sp
		}
	}
	if bubble == "" || fillOfXML(bubble) != "F1F2F1" {
		t.Errorf("the quote must sit in a F1F2F1 bubble: %q", bubble)
	}
	for _, sp := range strings.Split(slide, "<p:sp>")[1:] {
		if strings.Contains(sp, "書類の一覧を前もって送る") {
			sp = sp[:strings.Index(sp, "</p:sp>")]
			if strings.Contains(sp[:strings.Index(sp, "</p:spPr>")], "<a:solidFill>") || !strings.Contains(sp, `b="1"`) || !strings.Contains(sp, `val="29BA74"`) {
				t.Errorf("takeaway text must be bold accent without a fill: %s", sp)
			}
		}
	}
	// 結論・引用の印は図ではない(spare-width を出さない)。
	res := Check("c.bx.md", []byte(reportCalloutsDeck), CheckOptions{Template: "report"})
	for _, w := range res.Warnings {
		if w.Code == "spare-width" || w.Code == "layout-overflow" {
			t.Errorf("report: %+v", w)
		}
	}
	for _, name := range []string{"dark", "light"} {
		s := builtSlideXML(t, reportCalloutsDeck, name, 2)
		for _, prst := range []string{"rightArrow", "round2SameRect", "rtTriangle", "ellipse"} {
			if n := len(shapesWithGeom(s, prst)); n != 0 {
				t.Errorf("%s: %d %s shapes, want none (standard takeaway/quote)", name, n, prst)
			}
		}
		rows := tableCellsXML(s)
		if got := fillOfXML(cellPart(rows[0][0], "tcPr")[strings.Index(cellPart(rows[0][0], "tcPr"), "</a:lnB>"):]); got != strings.ToUpper(mustTemplate(name).Theme["card_alt"]) {
			t.Errorf("%s: header fill = %s, want card_alt", name, got)
		}
	}
}
