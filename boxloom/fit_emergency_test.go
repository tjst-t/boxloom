package boxloom

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// 3つのテンプレートは非常用の段を持ち、どれも shrink の最後の段より本文と表の字が小さい。
func TestTemplatesHaveFitEmergency(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		f := mustTemplate(name).Fit
		if len(f.Emergency) == 0 {
			t.Fatalf("%s: no fit.emergency", name)
		}
		prev := f.Shrink[len(f.Shrink)-1]
		for i, e := range f.Emergency {
			if e.Body >= prev.Body || e.TableText >= prev.TableText || e.FigureDetail >= prev.FigureDetail {
				t.Errorf("%s: emergency[%d] body %v table %v figure %v does not go below %v %v %v", name, i, e.Body, e.TableText, e.FigureDetail, prev.Body, prev.TableText, prev.FigureDetail)
			}
			prev = e
		}
	}
}

// 値の表の fit.emergency:shrink の最後の段(無ければ dense)より大きい・段に無い・何も替えない
// 段は読み込みで誤り。書かなくてよい。
func TestFitEmergencyValuesChecked(t *testing.T) {
	fit := func(m map[string]any) map[string]any { return m["fit"].(map[string]any) }
	emerg := func(m map[string]any, step map[string]any) { fit(m)["emergency"] = []any{step} }
	for _, c := range []struct {
		name string
		edit func(m map[string]any)
		want string
	}{
		{"above shrink", func(m map[string]any) { emerg(m, map[string]any{"body": 12}) }, "fit.emergency[0].body: 12 is larger than fit.shrink[1].body (10)"},
		{"above previous", func(m map[string]any) {
			fit(m)["emergency"] = []any{map[string]any{"body": 8}, map[string]any{"body": 10}}
		}, "fit.emergency[1].body: 10 is larger than fit.emergency[0].body (8)"},
		{"above dense without shrink", func(m map[string]any) {
			delete(fit(m), "shrink")
			emerg(m, map[string]any{"body": 16})
		}, "fit.emergency[0].body: 16 is larger than dense.body (14)"},
		{"off type scale", func(m map[string]any) { emerg(m, map[string]any{"body": 9}) }, "fit.emergency[0].body: 9 is not on the type scale"},
		{"off space scale", func(m map[string]any) { emerg(m, map[string]any{"cardPadding": 6}) }, "fit.emergency[0].cardPadding: 6 is not on the spacing scale"},
		{"same as shrink", func(m map[string]any) { emerg(m, map[string]any{"body": 10}) }, "fit.emergency[0]: changes nothing from fit.shrink[1]"},
		{"unknown key", func(m map[string]any) { emerg(m, map[string]any{"slideTitle": 8}) }, `unknown field "slideTitle"`},
	} {
		_, err := parseTemplateFile("light", mutate(t, c.edit))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
	tm, err := parseTemplateFile("light", mutate(t, func(m map[string]any) { delete(fit(m), "emergency") }))
	if err != nil || len(tm.Fit.Emergency) != 0 {
		t.Fatalf("without emergency: %v %+v", err, tm.Fit.Emergency)
	}
}

// emergencyPhases は分けられない図(phases。段ごとに n 個の箱)。
func emergencyPhases(n int) string {
	var b strings.Builder
	b.WriteString("```phases\nsteps:\n")
	for s := 0; s < 3; s++ {
		fmt.Fprintf(&b, "  - title: 段 %d\n    items:\n", s+1)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "      - title: 項目 %d\n        detail: 沿線の地区で住民に聞き取りをして結果をまとめる\n", i+1)
		}
	}
	b.WriteString("```\n")
	return b.String()
}

// emergencyColumns は分けられない並列のまとまり(2列、各 n 項目の箇条)。
func emergencyColumns(n int) string {
	var b strings.Builder
	b.WriteString("::::columns\n")
	for c := 0; c < 2; c++ {
		fmt.Fprintf(&b, ":::column 観点 %d\n", c+1)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "- %s(%d)\n", splitSentence, i+1)
		}
		b.WriteString(":::\n")
	}
	b.WriteString("::::\n")
	return b.String()
}

func bodyCapacity(caps []SlideCapacity) SlideCapacity {
	for _, c := range caps {
		if c.Path == "/slides/2" {
			return c
		}
	}
	return SlideCapacity{}
}

// 梯子の最後の段でもはみ出し、分けられない図・まとまりは、-fit で非常用の段に下りて収まる:
// 警告の fit-emergency(hint つき)と容量の fit が出て、build は書く(本文は 8pt)。-fit なしは
// はみ出しのまま。
func TestFitEmergencyUnsplittable(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"phases", emergencyPhases(8)},
		{"columns", emergencyColumns(18)},
	} {
		md := splitDeckMD("light", c.body)
		plain := Check("e.bx.md", []byte(md), CheckOptions{})
		if len(fitIssues(plain.Warnings, codeLayoutOverflow)) == 0 || len(fitIssues(plain.Warnings, codeFitEmergency)) != 0 {
			t.Fatalf("%s: without -fit = %+v", c.name, plain.Warnings)
		}
		res := Check("e.bx.md", []byte(md), CheckOptions{Fit: true})
		em := fitIssues(res.Warnings, codeFitEmergency)
		if len(em) != 1 || em[0].Path != "/slides/2" || em[0].Hint == "" || !strings.Contains(em[0].Message, "fit.emergency[0] (body 8pt") ||
			len(fitIssues(res.Warnings, codeLayoutOverflow)) != 0 || len(res.Info) != 0 {
			t.Fatalf("%s: check -fit = warnings %+v info %+v", c.name, res.Warnings, res.Info)
		}
		if body := bodyCapacity(res.Capacity); body.Fit != "fit.emergency[0]" || body.Split != nil {
			t.Fatalf("%s: capacity = %+v", c.name, body)
		}
		if strict := Check("e.bx.md", []byte(md), CheckOptions{Fit: true, Strict: true}); strict.OK {
			t.Errorf("%s: -strict -fit is OK with a fit-emergency warning", c.name)
		}
		slides, b := buildSlidesXML(t, md, BuildOptions{Fit: true})
		found := false
		for _, w := range b.Warnings {
			found = found || strings.Contains(w, "[FIT-EMERGENCY WARNING]")
		}
		base, _ := buildSlidesXML(t, md, BuildOptions{AllowOverflow: true, noOverflowMark: true})
		if !found || len(b.Info) != 0 || len(slides) != len(base) ||
			strings.Count(slides[4], `sz="800"`) <= strings.Count(base[4], `sz="800"`) {
			t.Fatalf("%s: build -fit = %+v (%d slides, base %d)", c.name, b, len(slides), len(base))
		}
	}
}

// 1つの単位が梯子の最後の段では1枚に入らないが、非常用の段なら入るとき、分けた全部の枚を
// 非常用の段で組む(fit-split の知らせと fit-emergency の警告)。
func TestFitEmergencySplit(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&body, "- %s(%d)\n", splitSentence, i+1)
	}
	fmt.Fprintf(&body, "- %s\n", strings.Repeat(splitSentence+"。", 60))
	res := Check("e.bx.md", []byte(splitDeckMD("light", body.String())), CheckOptions{Fit: true})
	if len(fitIssues(res.Info, codeFitSplit)) != 1 || len(fitIssues(res.Warnings, codeFitEmergency)) != 1 || len(fitIssues(res.Warnings, codeLayoutOverflow)) != 0 {
		t.Fatalf("check -fit = warnings %+v info %+v", res.Warnings, res.Info)
	}
	n := 0
	for _, c := range res.Capacity {
		if c.Path == "/slides/2" {
			n++
			if c.Fit != "fit.emergency[0]" || c.Split == nil {
				t.Errorf("capacity = %+v", c)
			}
		}
	}
	if n < 2 {
		t.Fatalf("split into %d slides", n)
	}
}

// 非常用の段でも入らなければ元の組み方のまま:layout-overflow が残り、-fit の build は誤り、
// -allow-overflow なら作る。
func TestFitEmergencyStillOverflows(t *testing.T) {
	md := splitDeckMD("light", emergencyPhases(12))
	res := Check("e.bx.md", []byte(md), CheckOptions{Fit: true})
	if len(fitIssues(res.Warnings, codeLayoutOverflow)) == 0 || len(fitIssues(res.Warnings, codeFitEmergency)) != 0 || bodyCapacity(res.Capacity).Fit != "" {
		t.Fatalf("check -fit = %+v", res.Warnings)
	}
	d, _, err := ParseMarkdown("e.bx.md", []byte(md))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Fit: true}); !errors.Is(err, ErrLayoutOverflow) || buf.Len() != 0 {
		t.Fatalf("build -fit = %v", err)
	}
	if _, err := Build(&buf, d, BuildOptions{Fit: true, AllowOverflow: true}); err != nil || buf.Len() == 0 {
		t.Fatalf("build -fit -allow-overflow = %v", err)
	}
}
