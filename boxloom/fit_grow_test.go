package boxloom

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fitThinMD = "../examples/md/warnings/fill-ratio.bx.md"

// 薄いスライドは -fit で fit.grow を上って広がり、info の fit-applied・容量の fit が出て
// fill-ratio が上がる。fill-ratio の警告は組んだ後の値(light は目標に届いて消え、report は
// 届かないまま上がった値で残る)。
func TestFitGrowWidensThinSlide(t *testing.T) {
	file, data := readFitDeck(t, fitThinMD, nil)
	for _, c := range []struct {
		tmpl, step string
		reached    bool
	}{{"light", "fit.grow[1]", true}, {"dark", "fit.grow[1]", true}, {"report", "fit.grow[1]", false}} {
		plain := Check(file, data, CheckOptions{Template: c.tmpl})
		before := fitIssues(plain.Warnings, "fill-ratio")
		if len(before) != 1 || plain.Info != nil || plain.Capacity[0].Fit != "" {
			t.Fatalf("%s: without -fit the example should be underfilled with no info: %+v", c.tmpl, plain)
		}
		res := Check(file, data, CheckOptions{Template: c.tmpl, Fit: true})
		if len(res.Info) != 1 || res.Info[0].Code != codeFitApplied || res.Info[0].Path != "/slides/1" ||
			!strings.Contains(res.Info[0].Message, c.step+" step") || !strings.Contains(res.Info[0].Message, "underfilled") || res.Info[0].Line != 7 {
			t.Fatalf("%s: info = %+v", c.tmpl, res.Info)
		}
		if len(res.Capacity) != 1 || res.Capacity[0].Fit != c.step || res.Capacity[0].Remaining < 0 {
			t.Fatalf("%s: capacity = %+v", c.tmpl, res.Capacity)
		}
		if len(fitIssues(res.Warnings, codeLayoutOverflow)) != 0 {
			t.Fatalf("%s: grow overflowed: %+v", c.tmpl, res.Warnings)
		}
		after := fitIssues(res.Warnings, "fill-ratio")
		if c.reached != (len(after) == 0) {
			t.Fatalf("%s: fill-ratio after -fit = %+v", c.tmpl, after)
		}
		if !c.reached && after[0].Message <= before[0].Message[:len(after[0].Message)] {
			// "underfilled: NN.N% (< 65%)" の値は上がる(同じ桁なので文字列で比べられる)
			t.Fatalf("%s: fill-ratio did not rise: %q -> %q", c.tmpl, before[0].Message, after[0].Message)
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
		if len(br.Info) != 1 || !strings.HasPrefix(br.Info[0], "slide 2: [FIT-APPLIED INFO] the body was underfilled") {
			t.Fatalf("%s: build info %q", c.tmpl, br.Info)
		}
	}
}

// 広げた段の値は本文にだけ効く:本文の字は fit.grow の段の大きさ、題はそのまま。
func TestFitGrowUsesStepValues(t *testing.T) {
	file, data := readFitDeck(t, fitThinMD, nil)
	deck, err := ReadDeck(file, data)
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate("light")
	b, c, err := tm.fitSlide(deck, 1)
	if err != nil || c == nil || !c.grow || c.step != "fit.grow[1]" || !(c.ratio > c.fromRatio) {
		t.Fatalf("fitSlide = %v, %+v", err, c)
	}
	st := tm.Fit.Grow[1]
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
}

// 広げてはみ出すことは無い:目標をページ一杯(fillRatioMin 1)にしても、全ての例で
// 広げたスライドは layout-overflow を出さず、-fit なしより警告のはみ出しが増えない。
func TestFitGrowNeverOverflows(t *testing.T) {
	var files []string
	for _, pat := range []string{"../examples/md/*.bx.md", "../examples/md/warnings/fill-ratio.bx.md", "../examples/yaml/*.yaml"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	grown := 0
	for _, name := range []string{"light", "report"} {
		tm := mustTemplate(name)
		tm.Values.FillRatioMin = 1
		for _, f := range files {
			_, data := readFitDeck(t, f, nil)
			assets := os.DirFS(filepath.Dir(f))
			plain := Check(f, data, CheckOptions{Values: tm, Assets: assets})
			res := Check(f, data, CheckOptions{Values: tm, Assets: assets, Fit: true})
			overPlain := map[string]bool{}
			for _, w := range fitIssues(plain.Warnings, codeLayoutOverflow) {
				overPlain[fitSlidePath(w.Path)] = true
			}
			for _, c := range res.Capacity {
				if !strings.HasPrefix(c.Fit, "fit.grow") {
					continue
				}
				grown++
				if c.Remaining < -overflowEps {
					t.Errorf("%s %s %s: grown to %s but remaining %.1f", name, f, c.Path, c.Fit, c.Remaining)
				}
			}
			for _, w := range fitIssues(res.Warnings, codeLayoutOverflow) {
				if !overPlain[fitSlidePath(w.Path)] {
					t.Errorf("%s %s: -fit grow made a new overflow: %+v", name, f, w)
				}
			}
		}
	}
	if grown == 0 {
		t.Fatal("no slide was grown; the test does not test anything")
	}
}

// dense のスライドは広げない(作者が詰めると決めたもの)。fit.grow の無いテンプレートも広げない。
func TestFitGrowSkipsDenseAndNoLadder(t *testing.T) {
	file, data := readFitDeck(t, fitThinMD, func(s string) string {
		return strings.Replace(s, "# 移行の判断の進め方", "# 移行の判断の進め方 {density=dense}", 1)
	})
	if res := Check(file, data, CheckOptions{Fit: true}); res.Info != nil || res.Capacity[0].Fit != "" {
		t.Fatalf("dense: info %+v capacity %+v", res.Info, res.Capacity)
	}
	file, data = readFitDeck(t, fitThinMD, nil)
	tm := mustTemplate("light")
	tm.Fit.Grow = nil
	if res := Check(file, data, CheckOptions{Values: tm, Fit: true}); res.Info != nil || len(fitIssues(res.Warnings, "fill-ratio")) != 1 {
		t.Fatalf("no grow ladder: info %+v warnings %+v", res.Info, res.Warnings)
	}
}

// 同梱の fit.grow の各段は、#136 の名前も body と釣り合うように上げる:matrix の行の題と
// body の差は values と同じ(行の題が本文より小さく残らない)。表のセルの余白・pairs の
// 対の間・印も values より小さくない(梯子の検査)うえで、最初の段で上がる。
func TestFitGrowStepsKeepRowValuesWithBody(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		tm := mustTemplate(name)
		v := tm.Values
		if len(tm.Fit.Grow) == 0 {
			t.Fatalf("%s: no fit.grow", name)
		}
		for k, st := range tm.Fit.Grow {
			if st.MatrixRowTitle-st.Body != v.MatrixRowTitle-v.Body {
				t.Errorf("%s fit.grow[%d]: matrixRowTitle %v body %v; values have %v %v", name, k, st.MatrixRowTitle, st.Body, v.MatrixRowTitle, v.Body)
			}
		}
		g := tm.Fit.Grow[0]
		if !(g.TableCellPadX > v.TableCellPadX && g.TableCellPadY > v.TableCellPadY && g.PairsRowGap > v.PairsRowGap && g.PairMarkSize > v.PairMarkSize) {
			t.Errorf("%s fit.grow[0] does not raise the row values: %+v (values %+v)", name, g, fitBaseValues(v))
		}
		// 広げた段のテンプレートの matrix の行の題も body と同じだけ上がる。
		ft := tm.fitStepped(fitRef{grow: true, n: 1}).Values
		if ft.MatrixRowTitle-ft.Body != v.MatrixRowTitle-v.Body {
			t.Errorf("%s: fitStepped matrixRowTitle %v body %v", name, ft.MatrixRowTitle, ft.Body)
		}
	}
}

// fitSlidePath は IR の場所のスライドの部分("/slides/N")。
func fitSlidePath(p string) string {
	parts := strings.SplitN(p, "/", 4)
	if len(parts) < 3 {
		return p
	}
	return strings.Join(parts[:3], "/")
}
