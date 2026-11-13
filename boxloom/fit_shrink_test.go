package boxloom

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

const fitOverflowMD = "../examples/md/warnings/layout-overflow.bx.md"

func readFitDeck(t *testing.T, path string, edit func(string) string) (string, []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		data = []byte(edit(string(data)))
	}
	return path, data
}

// reportFitMore は例の deck の最後に箇条を4つ足す。report は箇条の行送りと項目の間を
// 詰め、本文が下へ広い(#146)ので、例のままでは normal で収まる。
func reportFitMore(md string) string {
	return strings.Replace(md, ":::note", "- 追加の論点の1つ目を書く\n- 追加の論点の2つ目を書く\n- 追加の論点の3つ目を書く\n- 追加の論点の4つ目を書く\n\n:::note", 1)
}

func fitIssues(warns []Issue, code string) []Issue {
	var out []Issue
	for _, w := range warns {
		if w.Code == code {
			out = append(out, w)
		}
	}
	return out
}

// はみ出すスライドは -fit で梯子を下りて収まり、info の fit-applied と容量の fit が出る。
// normal のスライドは dense から(report は dense で、light は fit.shrink[0] で収まる)。
func TestFitShrinkPacksOverflowingSlide(t *testing.T) {
	for _, c := range []struct {
		tmpl, step string
		edit       func(string) string
	}{{"light", "fit.shrink[0]", nil}, {"report", fitStepDense, reportFitMore}} {
		file, data := readFitDeck(t, fitOverflowMD, c.edit)
		plain := Check(file, data, CheckOptions{Template: c.tmpl})
		if len(fitIssues(plain.Warnings, codeLayoutOverflow)) == 0 || plain.Info != nil {
			t.Fatalf("%s: without -fit the example should overflow and have no info: %+v", c.tmpl, plain)
		}
		res := Check(file, data, CheckOptions{Template: c.tmpl, Fit: true, Strict: true})
		if !res.OK || len(res.Warnings) != 0 {
			t.Fatalf("%s: -fit check = ok %v, warnings %+v", c.tmpl, res.OK, res.Warnings)
		}
		if len(res.Info) != 1 || res.Info[0].Code != codeFitApplied || res.Info[0].Path != "/slides/1" ||
			!strings.Contains(res.Info[0].Message, c.step+" step") || !strings.Contains(res.Info[0].Message, "remaining") || res.Info[0].Line != 7 {
			t.Fatalf("%s: info = %+v", c.tmpl, res.Info)
		}
		if len(res.Capacity) != 1 || res.Capacity[0].Fit != c.step || res.Capacity[0].Remaining < -overflowEps {
			t.Fatalf("%s: capacity = %+v", c.tmpl, res.Capacity)
		}

		deck, err := ReadDeck(file, data)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		br, err := Build(&buf, deck, BuildOptions{Template: c.tmpl, Fit: true})
		if err != nil || buf.Len() == 0 {
			t.Fatalf("%s: -fit build: %v", c.tmpl, err)
		}
		if len(br.Info) != 1 || !strings.HasPrefix(br.Info[0], "slide 2: [FIT-APPLIED INFO]") || hasOverflowWarning(br.Warnings) {
			t.Fatalf("%s: build info %q warnings %q", c.tmpl, br.Info, br.Warnings)
		}
	}
}

// 詰めた段の値は本文にだけ効く:fit.shrink[0] の本文の字と blockGap、題はそのまま。
func TestFitShrinkUsesStepValues(t *testing.T) {
	file, data := readFitDeck(t, fitOverflowMD, nil)
	deck, err := ReadDeck(file, data)
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate("light")
	b, c, err := tm.fitSlide(deck, 1)
	if err != nil || c == nil || c.step != "fit.shrink[0]" {
		t.Fatalf("fitSlide = %v, %+v", err, c)
	}
	st := tm.Fit.Shrink[0]
	if b.Body.Gap != st.BlockGap || c.body != st.Body {
		t.Fatalf("body gap %v body %v; want %v %v", b.Body.Gap, c.body, st.BlockGap, st.Body)
	}
	plain, err := tm.BuildSlide(deck, 1)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title.FontSize != plain.Title.FontSize {
		t.Fatalf("title %v; want %v", b.Title.FontSize, plain.Title.FontSize)
	}
	var sizes []float64
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindText {
			sizes = append(sizes, s.FontSize)
		}
		for _, k := range s.Children {
			walk(k)
		}
	}
	walk(b.Body)
	found := false
	for _, z := range sizes {
		if z > tm.Values.Body {
			t.Fatalf("body text %vpt is larger than values.body %v", z, tm.Values.Body)
		}
		found = found || z == st.Body
	}
	if !found {
		t.Fatalf("no body text at %vpt: %v", st.Body, sizes)
	}
}

// dense のスライドは dense の次(fit.shrink[0])から下りる。
func TestFitShrinkDenseStartsBelowDense(t *testing.T) {
	file, data := readFitDeck(t, fitOverflowMD, func(s string) string {
		return strings.Replace(s, "# 1枚に詰め込みすぎた論点", "# 1枚に詰め込みすぎた論点 {density=dense}", 1)
	})
	if len(fitIssues(Check(file, data, CheckOptions{}).Warnings, codeLayoutOverflow)) == 0 {
		t.Fatal("the dense example should still overflow without -fit")
	}
	res := Check(file, data, CheckOptions{Fit: true})
	if len(res.Capacity) != 1 || res.Capacity[0].Fit != "fit.shrink[0]" || res.Capacity[0].Density != densityDense {
		t.Fatalf("capacity = %+v, info %+v", res.Capacity, res.Info)
	}
}

// 最後の段でも収まらなければ layout-overflow のまま(build は誤り)。fit-applied は出さない。
func TestFitShrinkLastStepStillOverflows(t *testing.T) {
	file, data := readFitDeck(t, "../examples/md/warnings/table-overflow.bx.md", nil)
	res := Check(file, data, CheckOptions{Fit: true})
	if len(fitIssues(res.Warnings, codeLayoutOverflow)) == 0 || res.Info != nil || res.Capacity[0].Fit != "" {
		t.Fatalf("check -fit = warnings %+v info %+v", res.Warnings, res.Info)
	}
	deck, err := ReadDeck(file, data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{Fit: true}); !errors.Is(err, ErrLayoutOverflow) || buf.Len() != 0 {
		t.Fatalf("build -fit = %v, %d bytes", err, buf.Len())
	}
}

// はみ出しも薄いスライドも無いデッキは -fit でも1バイトも変わらない(薄いスライドは
// fit.grow で広がる。fit_grow_test.go)。
func TestFitShrinkNoChangeWhenFits(t *testing.T) {
	for _, f := range []string{"../examples/ir/charts.json"} {
		file, data := readFitDeck(t, f, nil)
		deck, err := ReadDeck(file, data)
		if err != nil {
			t.Fatal(err)
		}
		var a, b bytes.Buffer
		ra, err := Build(&a, deck, BuildOptions{AllowOverflow: true})
		if err != nil {
			t.Fatal(err)
		}
		rb, err := Build(&b, deck, BuildOptions{AllowOverflow: true, Fit: true})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) || rb.Info != nil || len(ra.Warnings) != len(rb.Warnings) {
			t.Fatalf("%s: -fit changed a deck that fits (info %q)", f, rb.Info)
		}
	}
}
