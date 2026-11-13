package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
)

// report の値を元の資料の詰め方に寄せる(Backlog #146)の試験。例は架空のもの。
//
//   - まとまりの題(対の側の題・列の題・topic の題)とその下の箇条の間・箇条の項目の間は
//     itemGap(2pt)
//   - フッターは footerText(8pt)、下の外周は pagePaddingBottom(20pt)。本文の箱はフッターと
//     重ならない
//   - dense の本文は 11pt
//   - リード文と箇条の行送りは 1.08 倍(段落は fonts.lineHeight の 1.2 倍のまま)

const reportTightenDeck = `---
title: 詰め方の試し
author: 試しの課
---

# 窓口の待ち時間を見直す

:::lead
夕方の窓口を増やし、書類の一覧を前もって送れば待ち時間は半分になる
:::

:::::pairs 分かったこと | 打ち手
::::subject 午前に混む
- 月曜と金曜の午前に来る人が全体の半分を占める
- 昼前は立って待つ人が出る
::::
::::response 予約の枠を広げる
- 午後の枠を倍にする
::::
:::subject 書類が足りない
- 来た人の三割が二度来ている
:::
:::response 一覧を前もって送る
- 要る書類の一覧を送る
:::
:::::

# 二つの館を比べる

:::::columns
:::column 東の館
- 返却の半分は閉館後に集まる
- 雨の日は箱の口から本が濡れる
:::
:::column 西の館
窓口の前の椅子は 12 脚で足りている
:::
:::::

# 割り振りの観点

:::::topics
:::topic 通いやすさ {label=A}
- 遠い人には近くの空き区画を先に案内する
- 車で来る人には駐車場に近い区画を回す
:::
:::topic 続けやすさ {label=B}
初めての人には相談役の区画の隣を割り振る
:::
:::::

# 密な一覧 {density=dense}

- 第1世代は保守が切れる前に移行先を決める
- 第2世代以降は続けるか移るかを選べる
`

// layoutFind は n の下で、文字が text を含む文字の葉を返す。
func layoutFind(n *LayoutNode, text string) *LayoutNode {
	if n.Text != nil && strings.Contains(*n.Text, text) {
		return n
	}
	for _, c := range n.Children {
		if f := layoutFind(c, text); f != nil {
			return f
		}
	}
	return nil
}

// layoutFindPath は path が suffix で終わる箱を返す。
func layoutFindPath(n *LayoutNode, suffix string) *LayoutNode {
	if strings.HasSuffix(n.Path, suffix) {
		return n
	}
	for _, c := range n.Children {
		if f := layoutFindPath(c, suffix); f != nil {
			return f
		}
	}
	return nil
}

// layoutPlaceholder は placeholder が ph の葉を返す。
func layoutPlaceholder(n *LayoutNode, ph string) *LayoutNode {
	if n.Placeholder == ph {
		return n
	}
	for _, c := range n.Children {
		if f := layoutPlaceholder(c, ph); f != nil {
			return f
		}
	}
	return nil
}

func nearTight(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func reportTightenLayout(t *testing.T, tmpl string) *LayoutDump {
	t.Helper()
	dump, err := Layout("t.bx.md", []byte(reportTightenDeck), BuildOptions{Template: tmpl})
	if err != nil {
		t.Fatal(err)
	}
	return dump
}

// 新しい値:題と箇条の間・項目の間・フッター・dense の本文・リードと箇条の行送り。
func TestReportTightenValues(t *testing.T) {
	v := mustTemplate("report").Values
	if v.ItemGap != 2 || v.PagePaddingBottom != 20 || v.FooterText != 8 || v.LeadLineHeight != 1.08 || v.ListLineHeight != 1.08 {
		t.Fatalf("report values: itemGap %v pagePaddingBottom %v footerText %v leadLineHeight %v listLineHeight %v",
			v.ItemGap, v.PagePaddingBottom, v.FooterText, v.LeadLineHeight, v.ListLineHeight)
	}
	if d := mustTemplate("report").Dense; d.Body != 11 || d.ItemGap != 2 {
		t.Fatalf("report dense: body %v itemGap %v", d.Body, d.ItemGap)
	}
	for _, name := range []string{"dark", "light"} {
		v := mustTemplate(name).Values
		lh := mustTemplate(name).Fonts.lineHeight()
		// dark・light は前と同じ組み方になる値(下の外周は上と同じ、フッターは small、行送りは本文と同じ)。
		if v.PagePaddingBottom != v.PagePaddingY || v.FooterText != v.Small || v.LeadLineHeight != lh || v.ListLineHeight != lh {
			t.Errorf("%s: new values change the output: %+v", name, v)
		}
	}
}

func TestReportTightenLayout(t *testing.T) {
	dump := reportTightenLayout(t, "report")
	tm := mustTemplate("report")
	v := tm.Values
	gapBelow := func(a, b *LayoutNode) float64 { return b.Box.Y - (a.Box.Y + a.Box.Height) }

	// 1枚目:対の側の題 → 箇条、箇条の項目の間。リードの行送り。
	s1 := dump.Slides[1].Root
	title, it1, it2 := layoutFind(s1, "午前に混む"), layoutFind(s1, "月曜と金曜"), layoutFind(s1, "昼前は")
	if title == nil || it1 == nil || it2 == nil {
		t.Fatal("pairs leaves not found")
	}
	if g := gapBelow(title, it1); !nearTight(g, v.ItemGap) {
		t.Errorf("pairs: side title → list = %.2fpt, want %v", g, v.ItemGap)
	}
	if g := gapBelow(it1, it2); !nearTight(g, v.ItemGap) {
		t.Errorf("pairs: item gap = %.2fpt, want %v", g, v.ItemGap)
	}
	if it1.LineHeight != 1.08 || it1.Tokens["lineHeight"] != "listLineHeight" || !nearTight(it1.Box.Height, v.Body*1.08) {
		t.Errorf("list item: lineHeight %v token %q height %.2f, want 1.08 / listLineHeight / %.2f", it1.LineHeight, it1.Tokens["lineHeight"], it1.Box.Height, v.Body*1.08)
	}
	lead := layoutFind(s1, "夕方の窓口")
	if lead == nil || lead.LineHeight != 1.08 || !nearTight(math.Mod(lead.Box.Height, v.LeadText*1.08), 0) {
		t.Errorf("lead: %+v, want line height 1.08 (rows of %.2fpt)", lead, v.LeadText*1.08)
	}

	// 2枚目:列の題 → (罫線) → 箇条。題と罫線、罫線と箇条の間はどちらも itemGap。
	s2 := dump.Slides[2].Root
	ct, ci := layoutFind(s2, "東の館"), layoutFind(s2, "返却の半分")
	if ct == nil || ci == nil {
		t.Fatal("columns leaves not found")
	}
	if g, want := gapBelow(ct, ci), 2*v.ItemGap; !nearTight(g, want) {
		t.Errorf("columns: title → list = %.2fpt, want %.2f (itemGap, the 0-high rule, itemGap)", g, want)
	}
	// 段落は fonts.lineHeight のまま。
	if para := layoutFind(s2, "窓口の前の椅子"); para == nil || para.LineHeight != 0 || !nearTight(para.Box.Height, v.Body*tm.Fonts.lineHeight()) {
		t.Errorf("paragraph: %+v, want the font line height %v", para, tm.Fonts.lineHeight())
	}

	// 3枚目:topic の題 → 箇条。
	s3 := dump.Slides[3].Root
	tt, ti := layoutFind(s3, "通いやすさ"), layoutFind(s3, "遠い人には")
	if tt == nil || ti == nil {
		t.Fatal("topics leaves not found")
	}
	if g := gapBelow(tt, ti); !nearTight(g, v.ItemGap) {
		t.Errorf("topics: title → list = %.2fpt, want %v", g, v.ItemGap)
	}

	// 4枚目:dense の本文は 11pt。
	if d := layoutFind(dump.Slides[4].Root, "第1世代"); d == nil || d.FontSize != 11 || !nearTight(d.Box.Height, 11*1.08) {
		t.Errorf("dense list item: %+v, want 11pt with rows of %.2f", d, 11*1.08)
	}
}

// 本文の箱はフッター(作者・頁番号)と重ならない。フッターは下の外周の上に置く。
func TestReportTightenBodyAboveFooter(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		dump := reportTightenLayout(t, tmpl)
		v := mustTemplate(tmpl).Values
		for _, s := range dump.Slides[1:] {
			body := layoutFindPath(s.Root, "/body")
			num := layoutPlaceholder(s.Root, "sldNum")
			ftr := layoutPlaceholder(s.Root, "ftr")
			if body == nil || num == nil || ftr == nil {
				t.Fatalf("%s %s: body %v page number %v footer %v", tmpl, s.Path, body, num, ftr)
			}
			bottom := body.Box.Y + body.Box.Height
			for _, f := range []*LayoutNode{num, ftr} {
				if f.FontSize != v.FooterText {
					t.Errorf("%s %s: footer text %vpt, want %v", tmpl, s.Path, f.FontSize, v.FooterText)
				}
				if bottom > f.Box.Y-v.SectionGap+0.01 {
					t.Errorf("%s %s: body ends at %.2f, the footer starts at %.2f (gap %v)", tmpl, s.Path, bottom, f.Box.Y, v.SectionGap)
				}
				if end := f.Box.Y + f.Box.Height; !nearTight(end, v.PageH-v.PagePaddingBottom) {
					t.Errorf("%s %s: footer ends at %.2f, want %.2f", tmpl, s.Path, end, v.PageH-v.PagePaddingBottom)
				}
			}
			// 本文の中の葉も本文の箱の中(はみ出していない例)。
			for _, l := range layoutLeaves(body, nil) {
				if l.Box.Y+l.Box.Height > bottom+0.01 {
					t.Errorf("%s %s: %s ends below the body box", tmpl, s.Path, l.Path)
				}
			}
		}
	}
	// report は本文が前(下の外周 42・フッター 10pt)より下へ広い。
	dump := reportTightenLayout(t, "report")
	body := layoutFindPath(dump.Slides[2].Root, "/body")
	if end, before := body.Box.Y+body.Box.Height, 540.0-42-10*1.2-12; end < before+20 {
		t.Errorf("report: body ends at %.2f, want at least 20pt below the old %.2f", end, before)
	}
}

// PPTX の行送り:箇条とリードは 1.08 倍、段落は 1.2 倍。
func TestReportTightenPPTXLineSpacing(t *testing.T) {
	d, issues, err := ParseMarkdown("t.bx.md", []byte(reportTightenDeck))
	if err != nil || d == nil {
		t.Fatal(err, issues)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Template: "report"}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	slide := func(name string) string {
		for _, f := range zr.File {
			if f.Name == name {
				rc, _ := f.Open()
				b, _ := io.ReadAll(rc)
				rc.Close()
				return string(b)
			}
		}
		t.Fatalf("%s not found", name)
		return ""
	}
	s2, s3 := slide("ppt/slides/slide2.xml"), slide("ppt/slides/slide3.xml")
	for _, want := range []string{`<a:spcPts val="2592"/>`, `<a:spcPts val="1512"/>`} { // 24×1.08・14×1.08
		if !strings.Contains(s2, want) {
			t.Errorf("slide 2: no %s", want)
		}
	}
	if !strings.Contains(s3, `<a:spcPts val="1680"/>`) { // 段落 14×1.2
		t.Errorf("slide 3: the paragraph lost the font line spacing")
	}
}
