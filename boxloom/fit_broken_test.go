package boxloom

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fitBrokenDeck は並列のまとまりに matrix を2つ並べた文書。評価の列が n 本なら、light では
// まとまりが図の下限を割り、評価の列の「p.135」が入らない(はみ出しは無い)。
func fitBrokenDeck(n int) []byte {
	cols := []string{"東の地区", "西の地区", "北の地区", "南の地区", "中の地区"}
	for k := len(cols); k < n; k++ {
		cols = append(cols, fmt.Sprintf("第%d地区", k+1))
	}
	cols = cols[:n]
	cells := strings.Repeat("      - {status: support, text: p.135}\n", n)
	mat := fmt.Sprintf("```matrix\ncolumns: [%s]\nrows:\n  - label: 設備\n    cells:\n%s```", strings.Join(cols, ", "), cells)
	return []byte(fmt.Sprintf("---\ntitle: t\n---\n\n# 評価\n\n::::columns\n:::column 左\n%s\n:::\n:::column 右\n%s\n:::\n::::\n", mat, mat))
}

func hasFitBroken(warns []Issue) bool {
	for _, w := range warns {
		if fitBrokenCodes[w.Code] {
			return true
		}
	}
	return false
}

// 本文の箱に収まっても図が崩れる(figure-text-overflow・subsystem-min・column-floor)
// スライドは、-fit が「収まった」とみなさず次の段へ下りる(#145)。
func TestFitBrokenFigureStepsDown(t *testing.T) {
	file, data := "broken.bx.md", fitBrokenDeck(4)
	plain := Check(file, data, CheckOptions{Template: "light"})
	if len(fitIssues(plain.Warnings, "figure-text-overflow")) == 0 || len(fitIssues(plain.Warnings, codeLayoutOverflow)) != 0 {
		t.Fatalf("without -fit the example should break a figure without overflowing: %+v", plain.Warnings)
	}
	res := Check(file, data, CheckOptions{Template: "light", Fit: true})
	if hasFitBroken(res.Warnings) || len(fitIssues(res.Warnings, codeLayoutOverflow)) != 0 {
		t.Fatalf("-fit should step down until the figures fit: %+v", res.Warnings)
	}
	if len(res.Info) != 1 || res.Info[0].Code != codeFitApplied || !strings.Contains(res.Info[0].Message, "a figure broke") ||
		!strings.Contains(res.Info[0].Message, "fit.shrink[0] step") {
		t.Fatalf("info = %+v", res.Info)
	}
	if len(res.Capacity) != 1 || res.Capacity[0].Fit != "fit.shrink[0]" {
		t.Fatalf("capacity = %+v", res.Capacity)
	}
	deck, err := ReadDeck(file, data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	b, err := Build(&buf, deck, BuildOptions{Template: "light", Fit: true})
	if err != nil || buf.Len() == 0 || len(b.Info) != 1 {
		t.Fatalf("Build -fit = %+v, %v", b, err)
	}
}

// 全部の段でも図が崩れるスライドは、-fit の build を誤りにする(-allow-overflow なら作る)。
// -fit なしでは今までどおり警告のまま作る(#145)。
// (評価の列が 5 本なら列ごとに分けて収まる(#152)ので、全幅の1列でも崩れる 12 本で試す。)
func TestFitBrokenEveryStepIsError(t *testing.T) {
	if res := Check("broken.bx.md", fitBrokenDeck(5), CheckOptions{Template: "light", Fit: true}); hasFitBroken(res.Warnings) ||
		len(fitIssues(res.Info, codeFitSplit)) != 1 || !strings.Contains(res.Info[0].Message, "split them by column") {
		t.Fatalf("5 columns should be split by column: warnings %+v, info %+v", res.Warnings, res.Info)
	}
	file, data := "broken.bx.md", fitBrokenDeck(12)
	res := Check(file, data, CheckOptions{Template: "light", Fit: true})
	if !hasFitBroken(res.Warnings) || len(res.Info) != 0 {
		t.Fatalf("the example should break at every step: warnings %+v, info %+v", res.Warnings, res.Info)
	}
	for _, w := range res.Warnings {
		if fitBrokenCodes[w.Code] && !strings.Contains(w.Hint, "-fit の最後の段") {
			t.Fatalf("hint of %s = %q", w.Code, w.Hint)
		}
	}
	deck, err := ReadDeck(file, data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, err = Build(&buf, deck, BuildOptions{Template: "light", Fit: true})
	var oe *OverflowError
	if !errors.As(err, &oe) || !errors.Is(err, ErrLayoutOverflow) || buf.Len() != 0 {
		t.Fatalf("Build -fit = %v (wrote %d bytes); want *OverflowError", err, buf.Len())
	}
	codes := map[string]bool{}
	for _, is := range oe.Issues {
		codes[is.Code] = true
	}
	if !codes["figure-text-overflow"] || !codes["column-floor"] || codes[codeLayoutOverflow] ||
		!strings.Contains(oe.Error(), "figure(s) break even at the last -fit step") {
		t.Fatalf("issues = %+v\n%v", oe.Issues, oe)
	}
	b, err := Build(&buf, deck, BuildOptions{Template: "light", Fit: true, AllowOverflow: true})
	if err != nil || buf.Len() == 0 || !strings.Contains(strings.Join(b.Warnings, "\n"), "text does not fit") {
		t.Fatalf("Build -fit -allow-overflow = %+v, %v", b, err)
	}
	buf.Reset()
	if b, err := Build(&buf, deck, BuildOptions{Template: "light"}); err != nil || buf.Len() == 0 || b.Info != nil {
		t.Fatalf("Build without -fit = %+v, %v; a broken figure stays a warning", b, err)
	}
}

// fitBrokenCode は警告の文を check の code で見分ける(頭の "slide N: " は有っても無くても)。
func TestFitBrokenCode(t *testing.T) {
	for w, want := range map[string]string{
		`[SUBSYSTEM WARNING] body[0]: matrix: rows[0].cells[1] ("p.135"): text does not fit (needs 49.5x18.9pt, has 42.7x237.6pt)`:                                                     "figure-text-overflow",
		"slide 3: [SUBSYSTEM WARNING] body[0]: matrix: needs at least 501.9pt width, got 424.0pt":                                                                                      "subsystem-min",
		"[COLUMN-FLOOR WARNING] body[0]: columns[1]: the figure in this column needs at least 501.9pt width, got 424.0pt (77.9pt short); the columns' figures do not fit side by side": "column-floor",
		`[SUBSYSTEM WARNING] body[0]: matrix: legend: status "danger" is used but has no legend text (write legend: {danger: …}); the reader cannot tell what the outline color means`: "",
		"[FILL-RATIO WARNING] underfilled: 40.0% (< 65%)": "",
	} {
		if got := fitBrokenCode(w); got != want {
			t.Errorf("fitBrokenCode(%q) = %q, want %q", w, got, want)
		}
	}
}
