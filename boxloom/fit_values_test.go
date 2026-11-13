package boxloom

import (
	"strings"
	"testing"
)

// 3つのテンプレートは収めの梯子(詰め2段・広げ2段)を持ち、詰めは dense から下へ、
// 広げは values から上へ並ぶ。report の表の字は 10pt を下回らない。
func TestTemplatesHaveFitLadders(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		tm := mustTemplate(name)
		f := tm.Fit
		if len(f.Shrink) != 2 || len(f.Grow) != 2 {
			t.Fatalf("%s: fit = %+v, want 2 shrink and 2 grow steps", name, f)
		}
		prev := fitBaseDense(tm.Dense, tm.Values)
		for i, s := range f.Shrink {
			if s.Body >= prev.Body {
				t.Errorf("%s: shrink[%d] body %v does not go below %v", name, i, s.Body, prev.Body)
			}
			prev = s
		}
		prev = fitBaseValues(tm.Values)
		for i, s := range f.Grow {
			if s.BlockGap <= prev.BlockGap {
				t.Errorf("%s: grow[%d] blockGap %v does not go above %v", name, i, s.BlockGap, prev.BlockGap)
			}
			prev = s
		}
		// 重ねた段は書かない名前を前の段から引き継ぐ。
		if g := f.Grow[1]; g.Body != f.Grow[0].Body {
			t.Errorf("%s: grow[1] body %v, want inherited %v", name, g.Body, f.Grow[0].Body)
		}
		v := f.Shrink[0].apply(tm.Values)
		if v.Body != f.Shrink[0].Body || v.CardPadding != f.Shrink[0].CardPadding || v.SlideTitle != tm.Values.SlideTitle {
			t.Errorf("%s: apply = %+v", name, v)
		}
	}
	if s := mustTemplate("report").Fit.Shrink; s[len(s)-1].TableText < 10 {
		t.Errorf("report: shrink floor table text %v is below 10", s[len(s)-1].TableText)
	}
}

// fit は書かなくてよい(梯子が空。詰めも広げもしない)。
func TestFitOptional(t *testing.T) {
	tm, err := parseTemplateFile("light", mutate(t, func(m map[string]any) { delete(m, "fit") }))
	if err != nil {
		t.Fatal(err)
	}
	if len(tm.Fit.Shrink) != 0 || len(tm.Fit.Grow) != 0 {
		t.Fatalf("fit = %+v", tm.Fit)
	}
	if _, err := parseTemplateFile("light", mutate(t, func(m map[string]any) {
		m["fit"] = map[string]any{"grow": []any{map[string]any{"body": 18}}}
	})); err != nil {
		t.Fatalf("grow only: %v", err)
	}
}

// 値の表の fit:段に無い値・順の逆転・知らないキー・何も替えない段は読み込みで誤り。
func TestFitValuesChecked(t *testing.T) {
	step := func(m map[string]any, ladder string, i int) map[string]any {
		return m["fit"].(map[string]any)[ladder].([]any)[i].(map[string]any)
	}
	for _, c := range []struct {
		name string
		edit func(m map[string]any)
		want string
	}{
		{"off type scale", func(m map[string]any) { step(m, "shrink", 0)["body"] = 13 }, "fit.shrink[0].body: 13 is not on the type scale"},
		{"off space scale", func(m map[string]any) { step(m, "grow", 1)["blockGap"] = 30 }, "fit.grow[1].blockGap: 30 is not on the spacing scale"},
		{"shrink above dense", func(m map[string]any) { step(m, "shrink", 0)["body"] = 16 }, "fit.shrink[0].body: 16 is larger than dense.body (14)"},
		{"shrink above previous", func(m map[string]any) { step(m, "shrink", 1)["body"] = 14 }, "fit.shrink[1].body: 14 is larger than fit.shrink[0].body (12)"},
		{"shrink padding above values", func(m map[string]any) { step(m, "shrink", 0)["cardPadding"] = 24 }, "fit.shrink[0].cardPadding: 24 is larger than values.cardPadding (20)"},
		{"grow below values", func(m map[string]any) { step(m, "grow", 0)["body"] = 14 }, "fit.grow[0].body: 14 is smaller than values.body (16)"},
		{"grow below previous", func(m map[string]any) { step(m, "grow", 1)["itemGap"] = 8 }, "fit.grow[1].itemGap: 8 is smaller than fit.grow[0].itemGap (12)"},
		{"zero", func(m map[string]any) { step(m, "shrink", 1)["itemGap"] = 0 }, "fit.shrink[1].itemGap: must be greater than 0"},
		{"unknown step key", func(m map[string]any) { step(m, "grow", 0)["slideTitle"] = 24 }, `unknown field "slideTitle"`},
		{"unknown ladder", func(m map[string]any) { m["fit"].(map[string]any)["steady"] = []any{} }, `unknown field "steady"`},
		{"null", func(m map[string]any) { step(m, "grow", 0)["body"] = nil }, `key "fit.grow[0].body" must not be null`},
		{"empty step", func(m map[string]any) { m["fit"].(map[string]any)["grow"] = []any{map[string]any{}} }, "fit.grow[0]: changes nothing from values"},
		{"same as previous", func(m map[string]any) {
			m["fit"].(map[string]any)["shrink"] = []any{map[string]any{"body": 14}}
		}, "fit.shrink[0]: changes nothing from dense"},
	} {
		_, err := parseTemplateFile("light", mutate(t, c.edit))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}
