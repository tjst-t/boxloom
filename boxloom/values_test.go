package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// 同梱したテンプレートと Gantt の値の表は、どれも厳しい読み込みを通る。
func TestBundledValueFilesLoad(t *testing.T) {
	names := TemplateNames
	if strings.Join(names, ",") != "dark,light" {
		t.Fatalf("bundled templates = %v", names)
	}
	for _, n := range names {
		if _, err := loadTemplate(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if gv := defaultGanttValues(); gv.NaturalRowH != 26 || gv.MaxRowH != 40 || gv.MinRowH != 17 {
		t.Errorf("gantt defaults = %+v", gv)
	}
	if _, err := LookupTemplate("nope"); err == nil || !strings.Contains(err.Error(), "known: dark, light") {
		t.Errorf("unknown template error = %v", err)
	}
	if tp, err := LookupTemplate(""); err != nil || tp.Name != "light" {
		t.Errorf("empty name = %v, %v", tp, err)
	}
}

// mutate は同梱の light.json を読み、f で書き換えて、もう一度 JSON にする。
func mutate(t *testing.T, f func(m map[string]any)) []byte {
	t.Helper()
	data, err := valueFiles.ReadFile("templates/light.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sub(m map[string]any, k string) map[string]any { return m[k].(map[string]any) }

func TestTemplateFileStrict(t *testing.T) {
	cases := []struct {
		name string
		f    func(m map[string]any)
		want string
	}{
		{"unknown top key", func(m map[string]any) { m["colour"] = 1 }, `unknown field "colour"`},
		{"unknown value key", func(m map[string]any) { sub(m, "values")["bodyy"] = 14 }, `unknown field "bodyy"`},
		{"missing value", func(m map[string]any) { delete(sub(m, "values"), "body") }, `missing key "values.body"`},
		{"missing marks", func(m map[string]any) { delete(m, "marks") }, `missing key "marks"`},
		{"null variants", func(m map[string]any) { m["variants"] = nil }, `"variants" must not be null`},
		{"wrong type", func(m map[string]any) { sub(m, "values")["body"] = "14" }, `cannot unmarshal string`},
		{"zero size", func(m map[string]any) { sub(m, "values")["itemGap"] = 0 }, `values.itemGap: must be greater than 0`},
		{"missing role", func(m map[string]any) { delete(sub(m, "theme"), "accent") }, `missing role "accent"`},
		{"unknown role", func(m map[string]any) { sub(m, "theme")["highlight"] = "FFFFFF" }, `unknown role "highlight"`},
		{"bad color", func(m map[string]any) { sub(m, "theme")["bg"] = "#fff" }, `theme.bg: "#fff" is not a color`},
		{"empty mark", func(m map[string]any) { sub(m, "marks")["pro"] = "" }, `marks.pro: must not be empty`},
		{"name mismatch", func(m map[string]any) { m["name"] = "dark" }, `does not match the file name`},
		{"partial gantt", func(m map[string]any) {
			m["subsystems"] = map[string]any{"gantt": map[string]any{"padding": 16}}
		}, `missing key "subsystems.gantt.legendHeight"`},
		{"unknown subsystem", func(m map[string]any) {
			m["subsystems"] = map[string]any{"table": map[string]any{}}
		}, `unknown field "table"`},
	}
	for _, c := range cases {
		_, err := parseTemplateFile("light", mutate(t, c.f))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
	if _, err := parseTemplateFile("light", append(mutate(t, func(map[string]any) {}), []byte(" {}")...)); err == nil {
		t.Errorf("trailing data was accepted")
	}
}

// テンプレートが subsystems.gantt を書くと、そのテンプレートで作る Gantt だけが
// その寸法で測られる(既定の表は変わらない)。
func TestTemplateOverridesGanttValues(t *testing.T) {
	data := mutate(t, func(m map[string]any) {
		var gv map[string]any
		raw, _ := valueFiles.ReadFile("subsystems/gantt.json")
		_ = json.Unmarshal(raw, &gv)
		gv["naturalRowHeight"] = 34
		m["subsystems"] = map[string]any{"gantt": gv}
	})
	tp, err := parseTemplateFile("light", data)
	if err != nil {
		t.Fatal(err)
	}
	deck, err := ReadIR("../examples/ir/gantt-full.json")
	if err != nil {
		t.Fatal(err)
	}
	raw := deck.Slides[0].Body[0].Data
	custom, err := tp.Subsystems.Build("gantt", raw)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DefaultSubsystems().Build("gantt", raw)
	if err != nil {
		t.Fatal(err)
	}
	hc := custom.Measure(Constraint{Width: 900}).Natural.Height
	hp := plain.Measure(Constraint{Width: 900}).Natural.Height
	if hc < hp+8 { // 行の高さが 8pt 増えたので、少なくとも 1 行ぶん高くなる
		t.Errorf("natural height custom=%v plain=%v", hc, hp)
	}
	if defaultGanttValues().NaturalRowH != 26 {
		t.Errorf("default gantt values were modified")
	}
}
