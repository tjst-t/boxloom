package boxloom

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const chartsPath = "../examples/ir/charts.json"

// zipFiles は PPTX(や xlsx)の部品を名前 → 中身で返す。
func zipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		files[f.Name] = string(b)
	}
	return files
}

// 比べ方とデータの数から形が決まる(作者は形を書かない)。
func TestChartForm(t *testing.T) {
	months := `["1","2","3","4","5","6"]`
	nine := `["1","2","3","4","5","6","7","8","9"]`
	cases := []struct {
		data string
		want ChartKind
	}{
		{`{"compare":"item","categories":["a","b"],"series":[{"name":"s","values":[1,2]}]}`, ChartBar},
		{`{"compare":"item","stacked":true,"categories":["a","b"],"series":[{"name":"s","values":[1,2]},{"name":"t","values":[1,2]}]}`, ChartStackedBar},
		{`{"compare":"timeSeries","categories":` + months + `,"series":[{"name":"s","values":[1,2,3,4,5,6]}]}`, ChartColumn},
		{`{"compare":"timeSeries","categories":` + nine + `,"series":[{"name":"s","values":[1,2,3,4,5,6,7,8,9]}]}`, ChartLine},
		{`{"compare":"timeSeries","categories":` + months + `,"series":[{"name":"s","values":[1,2,3,4,5,6]},{"name":"t","values":[1,2,3,4,5,6]}]}`, ChartLine},
		{`{"compare":"timeSeries","stacked":true,"categories":` + months + `,"series":[{"name":"s","values":[1,2,3,4,5,6]},{"name":"t","values":[1,2,3,4,5,6]}]}`, ChartStackedColumn},
		{`{"compare":"component","categories":["a","b"],"series":[{"name":"s","values":[1,2]}]}`, ChartPie},
		{`{"compare":"frequency","categories":["a","b"],"series":[{"name":"s","values":[1,2]}]}`, ChartHistogram},
		{`{"compare":"correlation","xAxis":{"title":"x"},"yAxis":{"title":"y"},"series":[{"name":"s","points":[[1,2],[3,4]]}]}`, ChartScatter},
	}
	for _, c := range cases {
		sub := figureFromIR(t, "light", "chart", c.data).(*ChartSubsystem)
		if got := sub.Form(); got != c.want {
			t.Errorf("%s: form %s, want %s", c.data, got, c.want)
		}
	}
}

// 例の IR を dark・light で組める(警告なし)。グラフは本物のグラフ(c:chartSpace)で、
// 値は埋め込みの xlsx にも入る。強調した項目・系列だけが accent の色になる。
// BOXLOOM_CHART_SHOTS にディレクトリを渡すと charts-{light,dark}.pptx をそこに書く。
func TestChartsExampleBuilds(t *testing.T) {
	d := mustReadIR(t, chartsPath)
	for _, name := range []string{"dark", "light"} {
		dir := os.Getenv("BOXLOOM_CHART_SHOTS")
		if dir == "" {
			dir = t.TempDir()
		}
		out := filepath.Join(dir, "charts-"+name+".pptx")
		warnings, err := renderDeckToFile(mustTemplate(name), d, out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warnings)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		files := zipFiles(t, raw)
		th := mustTemplate(name).Theme
		accent := `<a:srgbClr val="` + th.c("accent") + `"/>`
		for i := 1; i <= 7; i++ {
			for _, part := range []string{"ppt/charts/chart%d.xml", "ppt/charts/_rels/chart%d.xml.rels", "ppt/embeddings/Microsoft_Excel_Worksheet%d.xlsx"} {
				if _, ok := files[fmt.Sprintf(part, i)]; !ok {
					t.Errorf("%s: missing %s", name, fmt.Sprintf(part, i))
				}
			}
			if !strings.Contains(files[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1)], fmt.Sprintf("../charts/chart%d.xml", i)) {
				t.Errorf("%s: slide %d does not refer to chart %d", name, i+1, i)
			}
		}
		ch := func(i int) string { return files[fmt.Sprintf("ppt/charts/chart%d.xml", i)] }
		// 1: 項目の比較 → 横棒。上から書いた順(maxMin)、値の軸は下。強調した「印刷」(5番目)だけ accent。
		if c := ch(1); !strings.Contains(c, `<c:barDir val="bar"/>`) || !strings.Contains(c, `<c:orientation val="maxMin"/>`) ||
			!strings.Contains(c, `<c:dPt><c:idx val="4"/>`) || strings.Count(c, accent) != 1 || !strings.Contains(c, `formatCode="#,##0.0&quot;点&quot;"`) {
			t.Errorf("%s: chart 1 (item) is not a bar chart with one emphasized bar:\n%s", name, c)
		}
		// 2: 時系列(1系列・6点) → 縦棒。3: 9点・2系列 → 折れ線。
		if !strings.Contains(ch(2), `<c:barDir val="col"/>`) || !strings.Contains(ch(3), "<c:lineChart>") {
			t.Errorf("%s: time series should be a column (chart 2) and a line (chart 3)", name)
		}
		if !strings.Contains(ch(3), "<c:legend>") || strings.Contains(ch(2), "<c:legend>") {
			t.Errorf("%s: a legend is for two or more series only", name)
		}
		// 4: 積み上げ。5: 円(扇の境は背景の色)。6: ヒストグラム(棒の間を詰める)。7: 散布図(軸の題に単位)。
		if !strings.Contains(ch(4), `<c:grouping val="stacked"/>`) || !strings.Contains(ch(5), "<c:pieChart>") ||
			!strings.Contains(ch(5), `<a:srgbClr val="`+th.c("bg")+`"/>`) || !strings.Contains(ch(6), `<c:gapWidth val="6"/>`) {
			t.Errorf("%s: stacked, pie or histogram is wrong", name)
		}
		if c := ch(7); !strings.Contains(c, "<c:scatterChart>") || !strings.Contains(c, "誤りの数(件)") || !strings.Contains(c, "1件あたりの作業時間(分)") {
			t.Errorf("%s: scatter lacks axis titles", name)
		}
		// 埋め込みの表に値がある(PowerPoint の「データの編集」で開く)。
		xl := zipFiles(t, []byte(files["ppt/embeddings/Microsoft_Excel_Worksheet3.xlsx"]))
		if s := xl["xl/worksheets/sheet1.xml"]; !strings.Contains(s, "<t>新しい手順</t>") || !strings.Contains(s, `<c r="B10"><v>290</v></c>`) {
			t.Errorf("%s: workbook of chart 3 lacks the values: %s", name, s)
		}
		xl = zipFiles(t, []byte(files["ppt/embeddings/Microsoft_Excel_Worksheet7.xlsx"]))
		if s := xl["xl/worksheets/sheet1.xml"]; !strings.Contains(s, `<c r="A8"><v>7</v></c><c r="B8"><v>1</v></c>`) {
			t.Errorf("%s: workbook of the scatter lacks x/y columns: %s", name, s)
		}
	}
}

// 横棒は項目の数で高さが決まり(上限あり)、ほかは幅に比例した自然な高さで上限なし。
// 凡例・値のラベルの行の分だけ最小が増える。
func TestChartMeasure(t *testing.T) {
	bar := func(n int) *ChartSubsystem {
		cats, vals := []string{}, []string{}
		for i := 0; i < n; i++ {
			cats = append(cats, fmt.Sprintf(`"項目%d"`, i))
			vals = append(vals, "1")
		}
		return figureFromIR(t, "light", "chart", `{"compare":"item","categories":[`+strings.Join(cats, ",")+`],"series":[{"name":"s","values":[`+strings.Join(vals, ",")+`]}]}`).(*ChartSubsystem)
	}
	m3, m6 := bar(3).Measure(Constraint{Width: 600}), bar(6).Measure(Constraint{Width: 600})
	if !(m6.Min.Height > m3.Min.Height && m6.Natural.Height > m3.Natural.Height && m3.Max.Height > 0 && m3.Natural.Height <= m3.Max.Height) {
		t.Errorf("bar: 3 items %+v, 6 items %+v", m3, m6)
	}
	col := figureFromIR(t, "light", "chart", `{"compare":"timeSeries","categories":["1","2","3"],"series":[{"name":"s","values":[1,2,3]}]}`)
	two := figureFromIR(t, "light", "chart", `{"compare":"timeSeries","categories":["1","2","3"],"series":[{"name":"s","values":[1,2,3]},{"name":"t","values":[1,2,3]}]}`)
	mc, mt := col.Measure(Constraint{Width: 800}), two.Measure(Constraint{Width: 800})
	if mc.Max.Height != 0 || mc.Natural.Height != 800*defaultChartValues().NaturalAspect || mc.Natural.Height < mc.Min.Height {
		t.Errorf("column: %+v", mc)
	}
	if !(mc.Min.Height > defaultChartValues().MinPlotHeight) || mt.Min.Height <= mc.Min.Height-1e-9 {
		t.Errorf("column min %.1f, with legend %.1f", mc.Min.Height, mt.Min.Height)
	}
}

// 項目の名前がアプリの置く場所に入りきらないと chart-labels の警告(最初の1つを指し、
// 残りの数を添える)。check はその項目を指す。
func TestChartLabelsWarning(t *testing.T) {
	long := `{"compare":"timeSeries","categories":["Infrastructure-Operations-A","Infrastructure-Operations-B","c","d","e","f","g","h","i","j"],"series":[{"name":"s","values":[1,2,3,4,5,6,7,8,9,10]}]}`
	sub := figureFromIR(t, "light", "chart", long).(*ChartSubsystem)
	ws := sub.Lint(LBox{Width: 400, Height: 300})
	if len(ws) != 1 || !strings.Contains(ws[0], "categories[0]") || !strings.Contains(ws[0], "and 1 more") {
		t.Fatalf("warnings = %v", ws)
	}
	if ws := sub.Lint(LBox{Width: 4000, Height: 300}); len(ws) != 0 {
		t.Errorf("wide frame should fit: %v", ws)
	}
	is := warningIssue("x.json", "slide 2: [SUBSYSTEM WARNING] body[0]: "+strings.TrimPrefix(ws0(t, sub), "[SUBSYSTEM WARNING] "))
	if is.Code != "chart-labels" || is.Path != "/slides/1/body/0/data/categories/0" {
		t.Errorf("issue = %+v", is)
	}
	// 最小の大きさの警告は場所を持たないので、ブロックを指す。
	if is := warningIssue("x.json", "slide 2: [SUBSYSTEM WARNING] body[0]: chart: needs at least 156.4pt height, got 147.1pt"); is.Code != "subsystem-min" || is.Path != "/slides/1/body/0" {
		t.Errorf("subsystem-min issue = %+v", is)
	}
	// 日本語の短い名前は折り返して収まる。
	ok := figureFromIR(t, "light", "chart", `{"compare":"timeSeries","categories":["4月","5月","6月","7月","8月","9月"],"series":[{"name":"s","values":[1,2,3,4,5,6]}]}`).(*ChartSubsystem)
	if ws := ok.Lint(LBox{Width: 400, Height: 300}); len(ws) != 0 {
		t.Errorf("short labels: %v", ws)
	}
}

func ws0(t *testing.T, s *ChartSubsystem) string {
	t.Helper()
	ws := s.Lint(LBox{Width: 400, Height: 300})
	if len(ws) == 0 {
		t.Fatal("no warning")
	}
	return ws[0]
}

// 意味の誤りは check が chart-values・chart-emphasis で返し、組み立ての読み込みも同じ
// ものを断る。
func TestChartProblems(t *testing.T) {
	cases := []struct {
		data, code, at string
	}{
		{`{"compare":"item","categories":["a","b"],"series":[{"name":"s","values":[1]}]}`, "chart-values", "/series/0/values"},
		{`{"compare":"component","categories":["a","b"],"series":[{"name":"s","values":[1,-1]}]}`, "chart-values", "/series/0/values/1"},
		{`{"compare":"item","stacked":true,"categories":["a","b"],"series":[{"name":"s","values":[1,2]}]}`, "chart-values", "/stacked"},
		{`{"compare":"correlation","xAxis":{"title":"x"},"series":[{"name":"s","points":[[1,2],[3,4]]}]}`, "chart-values", ""},
		{`{"compare":"item","categories":["a","b"],"emphasis":["c"],"series":[{"name":"s","values":[1,2]}]}`, "chart-emphasis", "/emphasis/0"},
		{`{"compare":"item","categories":["a","b"],"emphasis":["a"],"series":[{"name":"s","values":[1,2]},{"name":"t","values":[1,2]}]}`, "chart-emphasis", "/emphasis"},
		{`{"compare":"item","categories":["a","b"],"series":[{"name":"s","values":[1,2],"emphasis":true},{"name":"t","values":[1,2],"emphasis":true}]}`, "chart-emphasis", "/series/1/emphasis"},
	}
	for _, c := range cases {
		deck := `{"boxloom":"0.1","meta":{"title":"t"},"slides":[{"type":"content","title":"t","body":[{"type":"subsystem","subsystem":"chart","data":` + c.data + `}]}]}`
		errs, _ := CheckIR("x.json", []byte(deck), "")
		if len(errs) == 0 || errs[0].Code != c.code || errs[0].Path != "/slides/0/body/0/data"+c.at {
			t.Errorf("%s: errors %+v, want %s at %s", c.data, errs, c.code, c.at)
		}
		if _, err := mustTemplate("light").Subsystems.Build("chart", json.RawMessage(c.data)); err == nil {
			t.Errorf("%s: the loader accepted it", c.data)
		}
	}
}

// ```chart のコードブロックの形の誤りは、Markdown の行つきで断る。
func TestChartYAMLErrors(t *testing.T) {
	head := "---\ntitle: t\n---\n\n# 題\n\n"
	cases := []struct{ src, code, msg string }{
		{"```chart\ncompare: bar\nseries:\n  - {name: s, values: [1, 2]}\n```\n", codeInvalidValue, "比べ方 \"bar\" はありません"},
		{"```chart\ncompare: correlation\nseries:\n  - {name: s, points: [[1, 2, 3]]}\n```\n", codeInvalidValue, "点は [x, y] の2つの数"},
		{"```chart\ncompare: item\ncategories: [a, b]\nseries:\n  - {name: s, values: [1, \"2\"]}\n```\n", codeInvalidValue, "ここには数を書いてください"},
		{"```chart\ncompare: item\ncolor: accent\nseries:\n  - {name: s, values: [1]}\n```\n", codeUnknownKey, "キー color はありません"},
	}
	for _, c := range cases {
		_, errs := MarkdownToIR("t.bx.md", []byte(head+c.src))
		if len(errs) == 0 || errs[0].Code != c.code || !strings.Contains(errs[0].Message, c.msg) || errs[0].Line == 0 {
			t.Errorf("%q: %+v", c.src, errs)
		}
	}
	// 意味の誤り(値の数)は check が Markdown の行つきで返す。
	src := head + "```chart\ncompare: item\ncategories: [a, b, c]\nseries:\n  - name: s\n    values: [1, 2]\n```\n"
	res := Check("t.bx.md", []byte(src), CheckOptions{})
	if len(res.Errors) != 1 || res.Errors[0].Code != "chart-values" || res.Errors[0].Line != 12 {
		t.Errorf("check: %+v", res.Errors)
	}
}

// 値のラベルは折り返さない(#52)。アプリが決めるラベルの幅で「2,859 億ド/ル」の
// ように単位の途中で折れないよう、dLbls の文字の書式に wrap="none" を付ける。
// 目盛り・凡例の書式は今までどおり(アプリの折り返しに任せる)。
func TestChartDataLabelsDoNotWrap(t *testing.T) {
	cd := &ChartData{Kind: ChartBar, DataLabels: true, FontSize: 10, TextColor: "000000", NumFmt: `#,##0"億ドル"`}
	dl := cd.dLbls()
	if !strings.Contains(dl, `<c:txPr><a:bodyPr wrap="none"/>`) {
		t.Fatalf("data labels must not wrap: %s", dl)
	}
	if strings.Contains(cd.txPr(), "wrap=") {
		t.Fatalf("axis/legend text keeps the app's wrapping: %s", cd.txPr())
	}
}
