package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// densityDeck は架空の倉庫の本文のスライド(本文・入れ子の箇条・表・図)。density は
// 呼ぶ側が {…} に入れる。
func densityDeck(attr string) string {
	return "---\ntitle: 架空の倉庫の報告\n---\n\n# 架空の在庫 " + attr + "\n\n:::lead\n架空の倉庫の在庫は月末に減る\n:::\n\n" +
		"架空の倉庫では、月末に出荷が集まる。\n\n- 出荷の山\n  - 月末の三日間\n\n" +
		"| 品目 | 数 |\n|---|---|\n| 箱 | 3 |\n\n" +
		"```process\nsteps:\n  - title: 受け入れ\n  - title: 出荷\n```\n"
}

// densitySizes は組んだスライドの字の大きさと間隔を集めたもの。
type densitySizes struct {
	title, lead, foot   float64
	body, sub, table    float64
	figure              float64
	blockGap, itemGap   float64
	bodyLeaves, subLeaf int
}

func measureDensitySizes(t *testing.T, tm *Template, attr string) densitySizes {
	t.Helper()
	d, issues, err := ParseMarkdown("x.bx.md", []byte(densityDeck(attr)))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	b, err := tm.BuildSlide(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	var z densitySizes
	z.title, z.lead = b.Title.FontSize, b.Lead.FontSize
	z.blockGap = b.Body.Gap
	var walk func(s *Spec, inList int)
	walk = func(s *Spec, inList int) {
		if s.List {
			inList++
			if z.itemGap == 0 {
				z.itemGap = s.Gap
			}
		}
		if s.Kind == KindText {
			switch {
			case inList >= 2:
				z.sub = s.FontSize
				z.subLeaf++
			case z.body == 0:
				z.body = s.FontSize
				z.bodyLeaves++
			}
		}
		switch x := s.Subsystem.(type) {
		case *tableLeaf:
			z.table = x.size
		case *ProcessSubsystem:
			z.figure = x.text().Title
		}
		for _, c := range s.Children {
			walk(c, inList)
		}
	}
	walk(b.Body, 0)
	// フッター:根の最後の子の文字。
	last := b.Root.Children[len(b.Root.Children)-1]
	var foot func(s *Spec)
	foot = func(s *Spec) {
		if s.Kind == KindText && z.foot == 0 {
			z.foot = s.FontSize
		}
		for _, c := range s.Children {
			foot(c)
		}
	}
	foot(last)
	return z
}

// dense のスライドでは本文・入れ子の箇条・表・図の字とブロック・項目の間が値の表の
// dense の組になり、題・リード・フッターは変わらない(3つのテンプレート)。
func TestDenseSlideUsesDenseValues(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		tm := mustTemplate(name)
		v, dv := tm.Values, tm.Dense
		normal := measureDensitySizes(t, tm, "")
		dense := measureDensitySizes(t, tm, "{density=dense}")
		explicit := measureDensitySizes(t, tm, "{density=normal}")
		if normal != explicit {
			t.Errorf("%s: density=normal differs from no density: %+v vs %+v", name, explicit, normal)
		}
		want := []struct {
			what      string
			got, want float64
		}{
			{"normal body", normal.body, v.Body}, {"normal subPoint", normal.sub, v.SubPoint},
			{"normal table", normal.table, v.TableText}, {"normal figure", normal.figure, v.Body},
			{"normal blockGap", normal.blockGap, v.BlockGap}, {"normal itemGap", normal.itemGap, v.ItemGap},
			{"dense body", dense.body, dv.Body}, {"dense subPoint", dense.sub, dv.SubPoint},
			{"dense table", dense.table, dv.TableText}, {"dense figure", dense.figure, dv.Body},
			{"dense blockGap", dense.blockGap, dv.BlockGap}, {"dense itemGap", dense.itemGap, dv.ItemGap},
			{"dense title", dense.title, normal.title}, {"dense lead", dense.lead, normal.lead},
			{"dense footer", dense.foot, normal.foot}, {"title", normal.title, v.SlideTitle},
		}
		for _, w := range want {
			if w.got != w.want || w.got == 0 {
				t.Errorf("%s: %s = %v, want %v", name, w.what, w.got, w.want)
			}
		}
		if dv.Body >= v.Body || dv.TableText >= v.TableText {
			t.Errorf("%s: dense values do not pack the body: %+v vs body %v table %v", name, dv, v.Body, v.TableText)
		}
	}
}

// report の dense の組は本文 12・入れ子 11・表 10(読む資料のサンプルの密なページ)。
func TestReportDenseValues(t *testing.T) {
	dv := mustTemplate("report").Dense
	if dv.Body != 12 || dv.SubPoint != 11 || dv.TableText != 10 {
		t.Fatalf("report dense = %+v", dv)
	}
}

// 値の表の dense:欠けたら・段に無ければ・values より大きければ読み込みで誤り。
func TestDenseValuesChecked(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(m map[string]any)
		want string
	}{
		{"missing", func(m map[string]any) { delete(m, "dense") }, `missing key "dense"`},
		{"missing key", func(m map[string]any) { delete(m["dense"].(map[string]any), "tableText") }, `missing key "dense.tableText"`},
		{"off scale", func(m map[string]any) { m["dense"].(map[string]any)["body"] = 13 }, "dense.body: 13 is not on the type scale"},
		{"larger", func(m map[string]any) { m["dense"].(map[string]any)["body"] = 18 }, "dense.body: 18 is larger than values.body"},
		{"unknown", func(m map[string]any) { m["dense"].(map[string]any)["title"] = 12 }, `unknown field "title"`},
	} {
		_, err := parseTemplateFile("light", mutate(t, c.edit))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

// Markdown ⇔ IR:{density=dense} は IR の density になり、IR で書いたものも読める。
// 知らない値は invalid-value(expected に normal・dense)、本文のスライドのほかは誤り。
func TestDensityParse(t *testing.T) {
	b, _, _ := ParseMarkdownWithMap([]byte(densityDeck("{density=dense}")), "x.bx.md")
	if !strings.Contains(string(b), `"density": "dense"`) {
		t.Fatalf("IR has no density:\n%s", b)
	}
	d, err := ParseIR(b)
	if err != nil || !d.Slides[1].dense() {
		t.Fatalf("%v %+v", err, d)
	}
	for _, ok := range []string{
		`{"type":"content","title":"題","density":"dense","body":[]}`,
		`{"type":"content","title":"題","density":"normal","body":[]}`,
	} {
		if _, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[` + ok + `]}`)); err != nil {
			t.Errorf("rejected %s: %v", ok, err)
		}
	}
	for _, bad := range []string{
		`{"type":"content","title":"題","density":"tight","body":[]}`,
		`{"type":"content","title":"題","density":"","body":[]}`,
		`{"type":"section","title":"章","density":"dense"}`,
		`{"type":"statement","content":"文","density":"dense"}`,
	} {
		if _, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[` + bad + `]}`)); err == nil {
			t.Errorf("accepted: %s", bad)
		}
	}
	for _, c := range []struct {
		name, md, code, path string
		expected             []string
	}{
		{"unknown", "---\ntitle: 試し\n---\n\n# 題 {density=tight}\n\n- 文\n", "invalid-value", "/slides/1/density", densityNames},
		{"section", "---\ntitle: 試し\n---\n\n# 章 {type=section density=dense}\n", "misplaced-content", "/slides/1", nil},
		{"statement", "---\ntitle: 試し\n---\n\n# 言いたいこと {type=statement density=dense}\n", "misplaced-content", "/slides/1", nil},
	} {
		res := Check("x.bx.md", []byte(c.md), CheckOptions{})
		found := false
		for _, e := range res.Errors {
			if e.Code == c.code && e.Path == c.path && (c.expected == nil || fmt.Sprint(e.Expected) == fmt.Sprint(c.expected)) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: errors = %+v, want %s at %s", c.name, res.Errors, c.code, c.path)
		}
	}
}

// Schema の本文のスライドに density(normal・dense)がある。
func TestDensitySchema(t *testing.T) {
	out, err := Schema("report")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var find func(v any) map[string]any
	find = func(v any) map[string]any {
		switch x := v.(type) {
		case map[string]any:
			if cs, ok := x["contentSlide"].(map[string]any); ok {
				return cs
			}
			for _, c := range x {
				if r := find(c); r != nil {
					return r
				}
			}
		case []any:
			for _, c := range x {
				if r := find(c); r != nil {
					return r
				}
			}
		}
		return nil
	}
	cs := find(doc)
	if cs == nil {
		t.Fatal("contentSlide not found")
	}
	m, _ := cs["properties"].(map[string]any)["density"].(map[string]any)
	if m == nil || fmt.Sprint(m["enum"]) != "[normal dense]" {
		t.Fatalf("density = %+v", m)
	}
}

// pointsDeck は架空の箇条を n 個持つ本文のスライド。
func pointsDeck(n int, attr string) string {
	var sb strings.Builder
	sb.WriteString("---\ntitle: 架空の町の報告\n---\n\n# 架空の要望 " + attr + "\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "- 架空の町の住民から寄せられた要望の%d番目は、駅前の案内を見やすくすることである\n", i+1)
	}
	return sb.String()
}

func overflowHints(res *CheckResult) []string {
	var out []string
	for _, w := range res.Warnings {
		if w.Code == "layout-overflow" {
			out = append(out, w.Hint)
		}
	}
	return out
}

// はみ出した normal のスライドの hint:dense で組み直して測り、収まるなら
// {density=dense} の案、収まらないなら「足りない」。容量にも dense の残りが出る。
func TestOverflowHintSuggestsDense(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		opts := CheckOptions{Template: name}
		// normal で初めてはみ出す箇条の数(dense は字が小さいので、ここは収まる)。
		first := 0
		for n := 3; n <= 60; n++ {
			if len(overflowHints(Check("x.bx.md", []byte(pointsDeck(n, "")), opts))) > 0 {
				first = n
				break
			}
		}
		if first == 0 {
			t.Fatalf("%s: no overflow up to 60 points", name)
		}
		res := Check("x.bx.md", []byte(pointsDeck(first, "")), opts)
		hints := overflowHints(res)
		if len(hints) == 0 || !strings.Contains(hints[0], "{density=dense} で収まります") {
			t.Errorf("%s: %d points: hints = %q", name, first, hints)
		}
		c := res.Capacity[0]
		if c.Remaining >= 0 || c.DenseRemaining == nil || *c.DenseRemaining < 0 || c.Density != "" {
			t.Errorf("%s: capacity = %+v", name, c)
		}
		// 書いたとおり dense にすると収まり、残りは容量の denseRemaining と同じ。
		resDense := Check("x.bx.md", []byte(pointsDeck(first, "{density=dense}")), opts)
		if h := overflowHints(resDense); len(h) > 0 {
			t.Errorf("%s: dense still overflows: %q", name, h)
		}
		dc := resDense.Capacity[0]
		if dc.Density != "dense" || dc.DenseRemaining != nil || dc.Remaining != *c.DenseRemaining {
			t.Errorf("%s: dense capacity = %+v (normal's denseRemaining %v)", name, dc, *c.DenseRemaining)
		}

		// 多すぎれば dense でも足りない。
		many := Check("x.bx.md", []byte(pointsDeck(first*2, "")), opts)
		mc := many.Capacity[0]
		if mc.DenseRemaining == nil || *mc.DenseRemaining >= 0 {
			t.Fatalf("%s: capacity = %+v", name, mc)
		}
		want := fmt.Sprintf("dense にしても %.1fpt 足りません", -*mc.DenseRemaining)
		if h := overflowHints(many); len(h) == 0 || !strings.HasPrefix(h[0], want) {
			t.Errorf("%s: hints = %q, want prefix %q", name, h, want)
		}
		// dense のスライドがはみ出したら、分ける・短くする。
		manyDense := Check("x.bx.md", []byte(pointsDeck(first*2, "{density=dense}")), opts)
		if h := overflowHints(manyDense); len(h) == 0 || !strings.HasPrefix(h[0], "このスライドは dense です。") {
			t.Errorf("%s: dense hints = %q", name, h)
		}
	}
}

// 配置の段の木(layout)に本文の箱の印 dense が出て、読み直して組むと build と同じ PPTX。
func TestDenseLayoutRoundTrip(t *testing.T) {
	src := []byte(densityDeck("{density=dense}"))
	deck, err := ReadDeck("x.bx.md", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, tmpl := range []string{"dark", "light", "report"} {
		dump, err := Layout("x.bx.md", src, BuildOptions{Template: tmpl})
		if err != nil {
			t.Fatal(err)
		}
		tree, err := json.Marshal(dump)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(tree), `"dense":true`) {
			t.Fatalf("%s: layout tree has no dense mark", tmpl)
		}
		var want, got bytes.Buffer
		if _, err := Build(&want, deck, BuildOptions{Template: tmpl}); err != nil {
			t.Fatal(err)
		}
		if _, err := BuildLayout(&got, "tree.json", tree, LayoutReadOptions{Deck: deck}); err != nil {
			t.Fatalf("%s: read back: %v", tmpl, err)
		}
		we, ge := zipEntries(t, want.Bytes()), zipEntries(t, got.Bytes())
		for name, w := range we {
			if !bytes.Equal(w, ge[name]) {
				t.Errorf("%s: %s differs after reading the layout tree back", tmpl, name)
			}
		}
	}
}
