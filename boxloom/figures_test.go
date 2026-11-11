package boxloom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const figuresPath = "../examples/ir/figures.json"

// figureFromIR は IR の data から、テンプレート name の描き方でサブシステムを作る。
func figureFromIR(t *testing.T, tmpl, kind, data string) Subsystem {
	t.Helper()
	sub, err := mustTemplate(tmpl).Subsystems.Build(kind, json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

// figureSlide はスライドの本文の領域(幅 912・高さ 414)に root を組んで描く。
// BOXLOOM_FIGURE_SHOTS にディレクトリを渡すと、目で見るための PPTX をそこに書く
// (soffice で PDF → pdftoppm で PNG にして見る)。
func figureSlide(t *testing.T, tmpl string, root *Spec, name string) *PPTXBuilder {
	t.Helper()
	tm := mustTemplate(tmpl)
	defer useFonts(tm.Fonts)()
	page := Col([]*Spec{root}, WithPadding(24), WithBackground(tm.Theme.c("bg")), WithAlignItems(AlignStretch))
	computeLayout(page, 0, 0, 960, 540)
	p := NewPPTXBuilder(960, 540)
	renderSpec(RenderContext{Out: p, Roles: tm.Theme}, page)
	if dir := os.Getenv("BOXLOOM_FIGURE_SHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, name+".pptx"))
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteDeck(f, []*PPTXBuilder{p}, name); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	return p
}

func within(in, b LBox) bool {
	const eps = 0.01
	return b.X >= in.X-eps && b.Y >= in.Y-eps && b.X+b.Width <= in.X+in.Width+eps && b.Y+b.Height <= in.Y+in.Height+eps
}

func boxesOverlap(a, b LBox) bool {
	const eps = 0.01
	return a.X+eps < b.X+b.Width && b.X+eps < a.X+a.Width && a.Y+eps < b.Y+b.Height && b.Y+eps < a.Y+a.Height
}

// 例の IR(Markdown の例と同じ)を dark・light で組める。警告は出ない。図の箱は
// 本物の図形、矢印・組織図の線は図形に繋がったコネクタ(組織図は曲がる線)。
func TestFiguresExampleBuilds(t *testing.T) {
	d := mustReadIR(t, figuresPath)
	for _, name := range []string{"dark", "light"} {
		out := filepath.Join(t.TempDir(), name+".pptx")
		warnings, err := renderDeckToFile(mustTemplate(name), d, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warnings)
		}
		process := slideXML(t, out, "ppt/slides/slide2.xml")
		if n := strings.Count(process, `prst="straightConnector1"`); n != 6 {
			t.Errorf("%s: process has %d straight connectors, want 6", name, n)
		}
		if n := strings.Count(process, `<a:tailEnd type="triangle"`); n != 6 {
			t.Errorf("%s: process has %d arrows, want 6", name, n)
		}
		org := slideXML(t, out, "ppt/slides/slide3.xml")
		// 根 → 3つの unit、unit → 5人。真下にあるものは直線になる。
		if n := strings.Count(org, "<p:cxnSp>"); n != 8 {
			t.Errorf("%s: orgchart has %d connectors, want 8", name, n)
		}
		if n := strings.Count(org, `prst="bentConnector3"`); n < 5 {
			t.Errorf("%s: orgchart has only %d elbow connectors", name, n)
		}
		if n := strings.Count(org, "<a:stCxn "); n != 8 || strings.Count(org, "<a:endCxn ") != 8 {
			t.Errorf("%s: every orgchart connector must be glued to both boxes", name)
		}
		matrix := slideXML(t, out, "ppt/slides/slide4.xml")
		for _, s := range []string{"満たす", "条件つき", "非対応", "満たさない"} {
			if !strings.Contains(matrix, ">"+s+"<") {
				t.Errorf("%s: matrix legend lacks %q", name, s)
			}
		}
		if !strings.Contains(matrix, `algn="l"`) || !strings.Contains(matrix, `algn="ctr"`) {
			t.Errorf("%s: matrix headers should be left (row) and centred (column)", name)
		}
	}
}

// 組織図は全幅でも、右の 60% の列でも、箱が枠の中に重ならずに収まり、文が
// 収まる(警告が出ない)。狭い列では文を折り返して箱が高くなる。
func TestOrgChartFitsFullAndNarrowColumns(t *testing.T) {
	data := `{"root": {"title": "責任者", "detail": "架空 太郎", "emphasis": true},
	  "units": [
	    {"title": "開発", "members": [{"title": "基盤", "detail": "4名"}, {"title": "画面", "detail": "3名"}]},
	    {"title": "運用", "members": [{"title": "導入支援", "detail": "2名", "status": "conditional"}, {"title": "監視", "detail": "2名"}]},
	    {"title": "営業", "members": [{"title": "提案", "detail": "2名"}, {"title": "契約", "detail": "1名"}]}]}`
	for _, tmpl := range []string{"light", "dark"} {
		full := figureFromIR(t, tmpl, "orgchart", data)
		fullLeaf := SubsystemNode(full, WithGrow(1))
		figureSlide(t, tmpl, fullLeaf, "orgchart-full-"+tmpl)

		narrow := figureFromIR(t, tmpl, "orgchart", data)
		narrowLeaf := SubsystemNode(narrow, WithGrow(3), WithBasis(0))
		text := RichTextNode([]TextRun{{Text: "左の 40% の列。組織図は右の 60% に置く。"}}, WithFontSize(14), WithGrow(2), WithBasis(0))
		row := Row([]*Spec{text, narrowLeaf}, WithGap(16), WithGrow(1), WithAlignItems(AlignStretch))
		figureSlide(t, tmpl, row, "orgchart-60-"+tmpl)
		if r := narrowLeaf.Box.Width / (fullLeaf.Box.Width - 16); r < 0.55 || r > 0.65 {
			t.Fatalf("narrow column is %.0f%% of the width", r*100)
		}
		for _, c := range []struct {
			name string
			leaf *Spec
		}{{"full", fullLeaf}, {"60%", narrowLeaf}} {
			sub := c.leaf.Subsystem.(*OrgChartSubsystem)
			box := subsystemDrawBox(c.leaf)
			if ws := sub.Lint(box); len(ws) != 0 {
				t.Errorf("%s %s: warnings %v", tmpl, c.name, ws)
			}
			if ws := lintSubsystemMin(c.leaf); len(ws) != 0 {
				t.Errorf("%s %s: %v", tmpl, c.name, ws)
			}
			pl := sub.place(box)
			all := []LBox{pl.root}
			all = append(all, pl.units...)
			for _, ms := range pl.members {
				all = append(all, ms...)
			}
			for i, a := range all {
				if !within(box, a) {
					t.Errorf("%s %s: box %d %+v outside %+v", tmpl, c.name, i, a, box)
				}
				for j := 0; j < i; j++ {
					if boxesOverlap(a, all[j]) {
						t.Errorf("%s %s: boxes %d and %d overlap", tmpl, c.name, i, j)
					}
				}
			}
		}
		// 狭い列のほうが3段目の箱は狭い。
		if n, f := narrow.(*OrgChartSubsystem).geom(narrowLeaf.Box.Width).memberW, full.(*OrgChartSubsystem).geom(fullLeaf.Box.Width).memberW; n >= f {
			t.Errorf("member width %.1f (60%%) should be less than %.1f (full)", n, f)
		}
	}
	// とても狭い枠では、文を折り返して箱が高くなる(自然な高さが増える)。
	sub := figureFromIR(t, "light", "orgchart", data)
	if a, b := sub.Measure(Constraint{Width: 900}).Natural.Height, sub.Measure(Constraint{Width: 300}).Natural.Height; b <= a {
		t.Errorf("natural height at 300pt (%.1f) should exceed the one at 900pt (%.1f)", b, a)
	}
}

// 段階が1行に並ばないときは、行の長さを揃えて折り返し、行の終わりと次の行の
// 始めを曲がるコネクタで繋ぐ。文は削らない。
func TestProcessWrapsToRows(t *testing.T) {
	var steps []string
	for _, s := range []string{"申請", "確認", "審査", "見積もり", "承認", "発注", "受け取り"} {
		steps = append(steps, `{"title": "`+s+`", "detail": "補足の文"}`)
	}
	data := `{"steps": [` + strings.Join(steps, ",") + `]}`
	sub := figureFromIR(t, "light", "process", data).(*ProcessSubsystem)
	if g := sub.geom(912); g.rows != 1 {
		t.Fatalf("full width: %d rows, want 1", g.rows)
	}
	g := sub.geom(500)
	if g.rows != 2 || g.perRow != 4 {
		t.Fatalf("500pt: rows=%d perRow=%d, want 2 rows of 4", g.rows, g.perRow)
	}
	leaf := SubsystemNode(sub, WithGrow(3), WithBasis(0))
	row := Row([]*Spec{TextNode("左の列", WithGrow(2), WithBasis(0)), leaf}, WithGap(16), WithGrow(1), WithAlignItems(AlignStretch))
	p := figureSlide(t, "light", row, "process-wrap")
	box := subsystemDrawBox(leaf)
	if ws := sub.Lint(box); len(ws) != 0 {
		t.Errorf("warnings %v", ws)
	}
	_, pl := sub.place(box)
	for i, b := range pl.boxes {
		if !within(box, b) || !within(box, pl.details[i]) {
			t.Errorf("step %d outside the frame", i)
		}
	}
	elbows := 0
	for _, s := range p.Shapes {
		if s.kind == skConnector && s.elbow {
			elbows++
			if s.stIdx != 2 || s.endIdx != 0 || s.elbowAdj <= 0 || s.elbowAdj >= 1 {
				t.Errorf("wrap connector: %+v", s)
			}
			// 曲がる高さは1行目の補足より下。
			bendY := s.y1 + (s.y2-s.y1)*s.elbowAdj
			if d := pl.details[3]; bendY < d.Y+d.Height {
				t.Errorf("bend at %.1f crosses the details of row 1 (bottom %.1f)", bendY, d.Y+d.Height)
			}
		}
	}
	if elbows != 1 {
		t.Errorf("%d wrap connectors, want 1", elbows)
	}
}

// 自己申告: 最小 ≤ 自然 ≤ 上限。幅が狭いほど(文を折り返すので)高くなる。
func TestFiguresMeasure(t *testing.T) {
	d := mustReadIR(t, figuresPath)
	for _, sl := range d.Slides {
		for _, b := range sl.Body {
			if b.Type != "subsystem" {
				continue
			}
			sub := figureFromIR(t, "light", b.Subsystem, string(b.Data))
			for _, w := range []float64{300, 600, 900} {
				m := sub.Measure(Constraint{Width: w})
				if !(m.Min.Height <= m.Natural.Height+1e-9 && m.Natural.Height <= m.Max.Height+1e-9) {
					t.Errorf("%s at %v: min %.1f nat %.1f max %.1f", b.Subsystem, w, m.Min.Height, m.Natural.Height, m.Max.Height)
				}
				if m.Natural.Width != w {
					t.Errorf("%s: natural width %.1f, want %v", b.Subsystem, m.Natural.Width, w)
				}
			}
			if a, c := sub.Measure(Constraint{Width: 300}).Min.Height, sub.Measure(Constraint{Width: 900}).Min.Height; a < c {
				t.Errorf("%s: min height at 300pt (%.1f) is less than at 900pt (%.1f)", b.Subsystem, a, c)
			}
		}
	}
}

// 箱に収まらない文は警告にする(黙って削らない)。check は
// figure-text-overflow とし、その要素を指す。
func TestFigureTextOverflowWarning(t *testing.T) {
	// 日本語は文字ごとに折り返せるので、収まらないのは折り返せない長い語。
	sub := figureFromIR(t, "light", "orgchart", `{"root": {"title": "責任者"}, "units": [{"title": "開発", "members": [{"title": "Infrastructure-Operations"}]}]}`)
	ws := sub.(SubsystemLinter).Lint(LBox{Width: 200, Height: 300})
	if len(ws) == 0 {
		t.Fatal("no warning for a frame that is too small")
	}
	is := warningIssue("x.json", "slide 2: "+strings.Replace(ws[len(ws)-1], "WARNING] ", "WARNING] body[1]: ", 1))
	if is.Code != "figure-text-overflow" || is.Path != "/slides/1/body/1/data/units/0/members/0" {
		t.Errorf("issue = %+v", is)
	}
	is = warningIssue("x.json", "slide 3: [SUBSYSTEM WARNING] body[0]: funnel: stages[2].value: 30 is larger than the previous stage (24); a funnel is for values that decrease")
	if is.Code != "funnel-increase" || is.Path != "/slides/2/body/0/data/stages/2/value" {
		t.Errorf("issue = %+v", is)
	}
}

// matrix: 凡例は使った状態を決まった順に、文は legend か既定。セルの数が列の数と
// 違えば読み込みが断り、check は matrix-cells を返す。
func TestMatrix(t *testing.T) {
	sub := figureFromIR(t, "light", "matrix", `{"columns": ["A", "B"], "rows": [
	  {"label": "x", "cells": [{"status": "danger", "text": "悪い"}, {"status": "support"}]}],
	  "legend": {"danger": "だめ", "conditional": "使わない"}}`).(*MatrixSubsystem)
	got := sub.legend()
	if len(got) != 2 || got[0] != (LegendItem{"support", "対応"}) || got[1] != (LegendItem{"danger", "だめ"}) {
		t.Errorf("legend = %+v", got)
	}
	for _, bad := range []struct{ data, want string }{
		{`{"columns": ["A", "B"], "rows": [{"label": "x", "cells": [{"status": "support"}]}]}`, "rows[0].cells: has 1 cells but there are 2 columns"},
		{`{"columns": ["A"], "rows": [{"label": "x", "cells": [{"text": "t"}]}]}`, "rows[0].cells[0].status: a cell needs a status"},
		{`{"columns": ["A"], "rows": [{"label": "x", "cells": [{"status": "accent"}]}]}`, `unknown status "accent"`},
		{`{"columns": ["A"], "rows": [{"label": "x", "cells": [{"status": "support"}]}], "legend": {"risk": "r"}}`, `legend: unknown key "risk"`},
	} {
		if _, err := matrixFromIR(json.RawMessage(bad.data)); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%s: err = %v, want %q", bad.data, err, bad.want)
		}
	}
	errs, _ := CheckFile("../examples/ir/check-invalid/matrix-cells.json", "")
	if len(errs) != 1 || errs[0].Code != "matrix-cells" || errs[0].Path != "/slides/3/body/0/data/rows/1/cells" {
		t.Errorf("errs = %+v", errs)
	}
}

// funnel: 幅は上から等しい刻みで狭まり(文が収まる幅より狭くしない)、上の段より
// 広くならない。数は3桁ごとに区切り、unit を付ける。数が増えると警告。
func TestFunnel(t *testing.T) {
	sub := figureFromIR(t, "light", "funnel", `{"unit": "件", "stages": [
	  {"label": "訪問", "value": 12000}, {"label": "登録", "value": 1500},
	  {"label": "とても長い名前の段階の名前", "value": "約 30"}, {"label": "有料", "value": 90}]}`).(*FunnelSubsystem)
	g := sub.geom(912)
	for i := 1; i < len(g.widths); i++ {
		if g.widths[i] > g.widths[i-1]+1e-9 {
			t.Errorf("widths %v increase at %d", g.widths, i)
		}
	}
	if got := sub.Data.Stages[0].Value; got != "12,000件" {
		t.Errorf("value = %q", got)
	}
	if got := sub.Data.Stages[2].Value; got != "約 30" {
		t.Errorf("text value = %q", got)
	}
	if ws := sub.Lint(LBox{Width: 912, Height: 300}); len(ws) != 0 {
		t.Errorf("warnings %v", ws)
	}
	inc := figureFromIR(t, "light", "funnel", `{"stages": [{"label": "a", "value": 10}, {"label": "b", "value": 20}]}`)
	ws := inc.(SubsystemLinter).Lint(LBox{Width: 600, Height: 200})
	if len(ws) != 1 || !strings.Contains(ws[0], "stages[1].value: 20 is larger than the previous stage (10)") {
		t.Errorf("warnings %v", ws)
	}
	if got := formatFunnelNumber(1234567.5); got != "1,234,567.5" {
		t.Errorf("format = %q", got)
	}
}

// 読み込みが断るもの(Schema と同じ形の誤り)。
func TestFiguresFromIRRejects(t *testing.T) {
	for _, c := range []struct{ kind, data, want string }{
		{"process", `{"steps": [{"title": "a"}]}`, "at least two steps"},
		{"process", `{"steps": [{"title": "a"}, {"title": "b", "status": "accent"}]}`, `steps[1].status: unknown status "accent"`},
		{"process", `{"steps": [{"title": "a"}, {"title": "b", "x": 1}]}`, `unknown field "x"`},
		{"orgchart", `{"root": {"title": "a"}, "units": []}`, "at least one unit"},
		{"orgchart", `{"root": {"title": "a"}, "units": [{"title": "b", "members": [{"detail": "c"}]}]}`, "units[0].members[0].title: must not be empty"},
		{"funnel", `{"stages": [{"label": "a", "value": 1}, {"label": "b"}]}`, "stages[1].value: a stage needs a value"},
		{"funnel", `{"stages": [{"label": "a", "value": 1}, {"label": "b", "value": true}]}`, "must be a number or a string"},
	} {
		_, err := DefaultSubsystems().Build(c.kind, json.RawMessage(c.data))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %s: err = %v, want %q", c.kind, c.data, err, c.want)
		}
	}
}

// Markdown の誤りは行と列つき。知らないキー・状態の名前・値の型。
func TestFiguresYAMLErrors(t *testing.T) {
	head := "---\ntitle: t\n---\n\n# 題\n\n"
	for _, c := range []struct{ yaml, code, msg string }{
		{"```process\nsteps:\n  - {title: a, color: accent}\n  - {title: b}\n", codeUnknownKey, "キー color はありません"},
		{"```process\nsteps:\n  - {title: a, status: accent}\n  - {title: b}\n", codeInvalidValue, `状態 "accent" はありません`},
		{"```orgchart\nroot: {title: a}\n", codeMissingKey, "キー units がありません"},
		{"```matrix\ncolumns: [A]\nrows:\n  - {label: x, cells: [{text: t}]}\n", codeMissingKey, "キー status がありません"},
		{"```matrix\ncolumns: [A]\nrows:\n  - {label: x, cells: [{status: support}]}\nlegend: {risk: r}\n", codeUnknownKey, "凡例のキー risk はありません"},
		{"```funnel\nstages:\n  - {label: a, value: 1}\n  - {label: b, value: [1]}\n", codeInvalidValue, "ここには値を書いてください"},
	} {
		_, errs := MarkdownToIR("t.bx.md", []byte(head+c.yaml+"```\n"))
		if len(errs) == 0 || errs[0].Code != c.code || !strings.Contains(errs[0].Message, c.msg) || errs[0].Line == 0 {
			t.Errorf("%q: errs = %+v", c.yaml, errs)
		}
	}
	// 数の値は数のまま、引用した数は文字列になる。
	ir, errs := MarkdownToIR("t.bx.md", []byte(head+"```funnel\nstages:\n  - {label: a, value: 400}\n  - {label: b, value: \"180\"}\n```\n"))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if !regexp.MustCompile(`"value":400\b`).Match(compactJSON(t, ir)) || !strings.Contains(string(compactJSON(t, ir)), `"value":"180"`) {
		t.Errorf("ir = %s", ir)
	}
}

func compactJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	out, _ := marshalNoEscape(v)
	return out
}

// boxloom schema は4つの図を名乗る(Markdown のコードブロックと IR の Schema)。
func TestSchemaDeclaresFigures(t *testing.T) {
	out, err := BuildSchemaOutput([]string{"light"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]SubsystemSchema{}
	for _, s := range out.Templates[0].Subsystems {
		names[s.Name] = s
	}
	for _, k := range figureKinds {
		s, ok := names[k]
		if !ok || s.Fence != "```"+k || len(s.Markdown) == 0 {
			t.Errorf("%s: %+v", k, s)
		}
		if !strings.Contains(string(out.IRSchema), `"$id":"https://boxloom.invalid/schema/ir/0.1/subsystems/`+k+`.schema.json"`) {
			t.Errorf("irSchema does not bundle %s", k)
		}
	}
}

// テンプレートの段から外れた寸法は読み込みで誤り。
func TestFigureValuesOnScale(t *testing.T) {
	for _, k := range figureKinds {
		raw, _ := valueFiles.ReadFile("subsystems/" + k + ".json")
		var v map[string]any
		json.Unmarshal(raw, &v)
		v["padding"] = 15.0
		tf, _ := valueFiles.ReadFile("templates/light.json")
		var m map[string]any
		json.Unmarshal(tf, &m)
		m["subsystems"] = map[string]any{k: v}
		b, _ := json.Marshal(m)
		_, err := parseTemplateFile("light", b)
		if err == nil || !strings.Contains(err.Error(), "subsystems."+k+".padding: 15 is not on the spacing scale") {
			t.Errorf("%s: err = %v", k, err)
		}
	}
}
