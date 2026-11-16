package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// 桁区切りの「,」を挟んだ数は1つのトークン(#185)。数でない「,」は1文字のまま。
func TestTokenizeKeepsGroupedNumbers(t *testing.T) {
	cases := map[string][]string{
		"枠の1,000万円":    {"枠", "の", "1,000", "万", "円"},
		"1,234,567.8円": {"1,234,567.8", "円"},
		"A,B と 1, 2":   {"A", ",", "B", " ", "と", " ", "1", ",", " ", "2"},
		"3,":           {"3", ","},
		",000":         {",", "000"},
		"10%,20%":      {"10%", ",", "20%"},
	}
	for in, want := range cases {
		if got := tokenizeLine(in); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("tokenizeLine(%q) = %q, want %q", in, got, want)
		}
	}
}

// どの幅で折っても、題(wrapText)・本文・リード(Phrase)の行は数の途中で切れない。
func TestWrapNeverSplitsGroupedNumber(t *testing.T) {
	restore := useFonts(mustTemplate("report").Fonts)
	defer restore()
	text := "施策1と3は満額、バディは新入生と希望者に絞って180万円にし、枠の1,000万円に収める。合計は1,234,567.8円。"
	full := naturalRunsWidth([]TextRun{{Text: text}}, 18)
	for w := 40.0; w < full+20; w += 3 {
		for _, phrase := range []bool{false, true} {
			var lines []string
			for _, l := range wrapRuns([]TextRun{{Text: text, Phrase: phrase}}, 18, w) {
				lines = append(lines, runsText(l))
			}
			checkNumbersWhole(t, lines, w)
		}
		checkNumbersWhole(t, wrapText(text, 18, w, true), w)
	}
}

func checkNumbersWhole(t *testing.T, lines []string, w float64) {
	t.Helper()
	for i := 1; i < len(lines); i++ {
		prev, next := lines[i-1], lines[i]
		if prev == "" || next == "" {
			continue
		}
		last, first := prev[len(prev)-1], next[0]
		if (last == ',' || last == '.' || last >= '0' && last <= '9') && first >= '0' && first <= '9' {
			t.Fatalf("width %.0f: number split between %q and %q", w, prev, next)
		}
	}
}

// eval F6 の形:リード文の数「1,000」の「,」の後ろに強制の改行(a:br)を入れない。
func TestLeadNoBreakInsideNumber(t *testing.T) {
	doc := `---
title: 架空の資料
template: report
fictional: true
---

# 予算の案

:::lead
施策1と3は満額、バディは新入生と希望者に絞って180万円にし、枠の1,000万円に収める。
:::

- 本文
`
	ir, errs := MarkdownToIR("t.bx.md", []byte(doc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("report")
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		s := string(b)
		if strings.Contains(s, "1,</a:t>") || strings.Contains(s, "<a:t>000万円") {
			t.Errorf("%s: the lead is broken inside 1,000", f.Name)
		}
		found = found || strings.Contains(s, "1,000万円")
	}
	if !found {
		t.Error("the lead with 1,000万円 was not written in one run")
	}
}
