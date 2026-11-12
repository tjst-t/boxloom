package boxloom

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
		{"missing scale", func(m map[string]any) { delete(m, "scale") }, `missing key "scale"`},
		{"zero spacing unit", func(m map[string]any) { sub(m, "scale")["spacingUnit"] = 0 }, `scale.spacingUnit: must be greater than 0`},
		{"empty type scale", func(m map[string]any) { sub(m, "scale")["typeSizes"] = []any{} }, `scale.typeSizes: must list at least one size`},
		{"type scale not increasing", func(m map[string]any) { sub(m, "scale")["typeSizes"] = []any{10, 14, 12} }, `scale.typeSizes[2]: sizes must be in increasing order`},
		{"spacing off scale", func(m map[string]any) { sub(m, "values")["itemGap"] = 6 }, `values.itemGap: 6 is not on the spacing scale (multiples of 4); nearest: 4 or 8`},
		{"type off scale", func(m map[string]any) { sub(m, "values")["subPoint"] = 12.5 }, `values.subPoint: 12.5 is not on the type scale [8 10 12 14 16 18 24 36]; nearest: 12 or 14`},
		{"type below scale", func(m map[string]any) { sub(m, "values")["small"] = 6 }, `nearest: 8`},
		{"gantt default off a narrowed scale", func(m map[string]any) { sub(m, "scale")["typeSizes"] = []any{10, 12, 14, 16, 18, 24, 36} }, `subsystems/gantt.json.smallText: 8 is not on the type scale`},
		{"gantt override off scale", func(m map[string]any) {
			var gv map[string]any
			raw, _ := valueFiles.ReadFile("subsystems/gantt.json")
			_ = json.Unmarshal(raw, &gv)
			gv["annotationPaddingY"] = 6
			m["subsystems"] = map[string]any{"gantt": gv}
		}, `subsystems.gantt.annotationPaddingY: 6 is not on the spacing scale`},
		{"gantt drawing size off scale", func(m map[string]any) {
			// #70 でコードから値の表へ移した寸法も段で見る(節目の箱の最小の幅)。
			var gv map[string]any
			raw, _ := valueFiles.ReadFile("subsystems/gantt.json")
			_ = json.Unmarshal(raw, &gv)
			gv["milestoneMinWidth"] = 150
			m["subsystems"] = map[string]any{"gantt": gv}
		}, `subsystems.gantt.milestoneMinWidth: 150 is not on the spacing scale (multiples of 4); nearest: 148 or 152`},
		{"shapes override off scale", func(m map[string]any) {
			var sv map[string]any
			raw, _ := valueFiles.ReadFile("subsystems/shapes.json")
			_ = json.Unmarshal(raw, &sv)
			sv["textInsetX"] = 5
			m["subsystems"] = map[string]any{"shapes": sv}
		}, `subsystems.shapes.textInsetX: 5 is not on the spacing scale`},
		{"missing marks", func(m map[string]any) { delete(m, "marks") }, `missing key "marks"`},
		{"null variants", func(m map[string]any) { m["variants"] = nil }, `"variants" must not be null`},
		{"wrong type", func(m map[string]any) { sub(m, "values")["body"] = "14" }, `cannot unmarshal string`},
		{"zero size", func(m map[string]any) { sub(m, "values")["itemGap"] = 0 }, `values.itemGap: must be greater than 0`},
		{"missing role", func(m map[string]any) { delete(sub(m, "theme"), "accent") }, `missing role "accent"`},
		{"unknown role", func(m map[string]any) { sub(m, "theme")["highlight"] = "FFFFFF" }, `unknown role "highlight"`},
		{"bad color", func(m map[string]any) { sub(m, "theme")["bg"] = "#fff" }, `theme.bg: "#fff" is not a color`},
		{"empty mark", func(m map[string]any) { sub(m, "marks")["pro"] = "" }, `marks.pro: must not be empty`},
		{"accent equals conditional", func(m map[string]any) { sub(m, "theme")["conditional"] = sub(m, "theme")["accent"] }, `theme.conditional: 0033CC is the same color as theme.accent`},
		{"none equals text_muted", func(m map[string]any) { sub(m, "theme")["none"] = sub(m, "theme")["text_muted"] }, `theme.text_muted: 5C6B85 is the same color as theme.none`},
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
	deck, err := readIRFile("../examples/ir/gantt-full.json")
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

// subsystems.shapes を書くと、そのテンプレートの shapes だけがその寸法になる。
// 字の段はテンプレートの values から引く。
func TestTemplateOverridesShapesValues(t *testing.T) {
	data := mutate(t, func(m map[string]any) {
		var sv map[string]any
		raw, _ := valueFiles.ReadFile("subsystems/shapes.json")
		_ = json.Unmarshal(raw, &sv)
		sv["defaultAspect"] = 1.2
		m["subsystems"] = map[string]any{"shapes": sv}
		m["values"].(map[string]any)["small"] = 8
	})
	tp, err := parseTemplateFile("light", data)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"items": [{"kind": "rect", "x": 0, "y": 0, "w": 0.5, "h": 0.5}]}`)
	custom, err := tp.Subsystems.Build("shapes", raw)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := mustTemplate("light").Subsystems.Build("shapes", raw)
	if err != nil {
		t.Fatal(err)
	}
	hc := custom.Measure(Constraint{Width: 600}).Natural.Height
	hp := plain.Measure(Constraint{Width: 600}).Natural.Height
	if hc <= hp {
		t.Errorf("natural height custom=%v plain=%v", hc, hp)
	}
	if got := custom.(*ShapesSubsystem).textSize("small"); got != 8 {
		t.Errorf("small = %v, want 8", got)
	}
	if defaultShapesValues().DefaultAspect != 2.4 {
		t.Errorf("default shapes values were modified")
	}
}

// 数と単位の間の印は空でよい(「71%」と詰める)。ほかの印は空なら誤り。
func TestStatUnitMarkMayBeEmpty(t *testing.T) {
	tm, err := parseTemplateFile("light", mutate(t, func(m map[string]any) { sub(m, "marks")["statUnit"] = "" }))
	if err != nil {
		t.Fatalf("empty statUnit mark rejected: %v", err)
	}
	if tm.Marks.Unit != "" {
		t.Errorf("Marks.Unit = %q, want empty", tm.Marks.Unit)
	}
}

// 同梱のテンプレートは、意味の違う役割名がどれも違う色で、図の補足と
// 表・画像の説明が注記(small)より大きい。
func TestBundledTemplatesDistinctRolesAndReadableDetail(t *testing.T) {
	for _, name := range TemplateNames {
		tm := mustTemplate(name)
		seen := map[string]string{}
		for _, r := range distinctRoles {
			if prev, ok := seen[tm.Theme[r]]; ok {
				t.Errorf("%s: %s and %s share %s", name, prev, r, tm.Theme[r])
			}
			seen[tm.Theme[r]] = r
		}
		v := tm.Values
		if v.FigureDetail <= v.Small || v.CaptionText <= v.Small {
			t.Errorf("%s: figureDetail %v / captionText %v should be above small %v", name, v.FigureDetail, v.CaptionText, v.Small)
		}
	}
}
