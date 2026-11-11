// chart サブシステム: 数の系列のグラフ(docs/patterns.md の R4)。
//
// 作者が書くのは「何を比べるか」(Zelazny の比較の5分類:item・timeSeries・
// component・frequency・correlation)と、項目・系列・値・単位・強調だけ。グラフの
// 形(横棒・縦棒・折れ線・円・ヒストグラム・散布図)はサブシステムが比較の種類と
// データの数から選び、色はテーマの役割名から引く。出すのは PowerPoint の本物の
// グラフ(c:chartSpace と、値を持つ埋め込みの xlsx。chartxml.go)なので、
// PowerPoint でグラフとして値を書き換えられる。
//
// 軸・目盛り・ラベルの置き場所はアプリが決める。Boxloom は枠の大きさ(Measure)と、
// 項目の名前が入りきらないこと(chart-labels)だけを、字体の寸法で測って申告する。
// 設計は docs/subsystem.md の12節、IR の data の形は schema/subsystems/chart.schema.json。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 比較の種類(Zelazny の5分類)。
const (
	CompareItem        = "item"        // 項目比較 → 横棒
	CompareTimeSeries  = "timeSeries"  // 時系列 → 縦棒(1系列で点が少ない)か折れ線
	CompareComponent   = "component"   // 構成比 → 円
	CompareFrequency   = "frequency"   // 度数分布 → ヒストグラム
	CompareCorrelation = "correlation" // 相関 → 散布図
)

var chartCompares = []string{CompareItem, CompareTimeSeries, CompareComponent, CompareFrequency, CompareCorrelation}

// chartPalette は系列の色の役割名(強調した系列、無ければ1つ目が accent)。
var chartPalette = []string{"accent", "text_muted", "none"}

// ChartValues は chart の寸法(pt。割合は書いたもの)。既定は subsystems/chart.json。
type ChartValues struct {
	NaturalWidth    float64 `json:"naturalWidth"`
	MinWidth        float64 `json:"minWidth"`
	MinPlotHeight   float64 `json:"minPlotHeight"` // 値の軸の長さ(縦の形)・円の直径の最小
	NaturalAspect   float64 `json:"naturalAspect"` // 縦の形・円・散布図の自然な高さ(幅に対する割合)
	MinBarSlot      float64 `json:"minBarSlot"`    // 横棒の1項目の高さ(最小・自然・上限)
	NaturalBarSlot  float64 `json:"naturalBarSlot"`
	MaxBarSlot      float64 `json:"maxBarSlot"`
	ColumnMaxPoints float64 `json:"columnMaxPoints"` // 時系列を縦棒で描く点の数の上限(越えたら折れ線)
	GapWidth        float64 `json:"gapWidth"`        // 棒の間(棒の幅に対する %)
	HistogramGap    float64 `json:"histogramGapWidth"`
	LineWidth       float64 `json:"lineWidth"`
	MarkerSize      float64 `json:"markerSize"`
	ScatterMarker   float64 `json:"scatterMarkerSize"`
	AxisLineWidth   float64 `json:"axisLineWidth"`
	GridLineWidth   float64 `json:"gridLineWidth"`
	PieOutlineWidth float64 `json:"pieOutlineWidth"`
	LabelMaxLines   float64 `json:"labelMaxLines"`    // 項目の名前を折り返してよい行の数
	BarLabelFrac    float64 `json:"barLabelFraction"` // 横棒の項目の名前に使える幅(枠の幅に対する割合)
	Padding         float64 `json:"padding" scale:"space"`
}

// ChartAxis は散布図の軸(何を測ったか)。
type ChartAxis struct {
	Title string `json:"title"`
	Unit  string `json:"unit"`
}

// ChartSeriesData は1つの系列。散布図は Points、ほかは Values。
type ChartSeriesData struct {
	Name     string
	Values   []float64
	Points   [][2]float64
	Emphasis bool
}

// ChartContent は IR の data を読んだもの(意味だけ)。
type ChartContent struct {
	Compare    string
	Categories []string
	Series     []ChartSeriesData
	Stacked    bool
	Unit       string
	Emphasis   []string // 目立たせる項目の名前(1系列のとき)
	XAxis      *ChartAxis
	YAxis      *ChartAxis
}

// ChartSubsystem は ChartContent を本物のグラフで描くサブシステム。
type ChartSubsystem struct {
	Data *ChartContent
	V    *ChartValues // nil なら既定(subsystems/chart.json)
	Text *FigureText  // nil なら既定のテンプレート(light)の段。グラフの文字は Sub の段
}

func NewChartSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*ChartContent); ok {
		return &ChartSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("chart", data)
	if err != nil {
		return nil, err
	}
	d, err := chartFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &ChartSubsystem{Data: d}, nil
}

func (s *ChartSubsystem) Name() string { return "chart" }

func (s *ChartSubsystem) vals() *ChartValues {
	if s.V != nil {
		return s.V
	}
	return defaultChartValues()
}

func (s *ChartSubsystem) fontSize() float64 {
	if s.Text != nil {
		return s.Text.Sub
	}
	return defaultFigureText().Sub
}

// Form は比較の種類とデータの数から選んだグラフの形。
func (s *ChartSubsystem) Form() ChartKind {
	d := s.Data
	switch d.Compare {
	case CompareItem:
		if d.Stacked {
			return ChartStackedBar
		}
		return ChartBar
	case CompareTimeSeries:
		switch {
		case d.Stacked:
			return ChartStackedColumn
		case len(d.Series) == 1 && float64(len(d.Categories)) <= s.vals().ColumnMaxPoints:
			return ChartColumn
		}
		return ChartLine
	case CompareComponent:
		return ChartPie
	case CompareFrequency:
		return ChartHistogram
	}
	return ChartScatter
}

// --- 数の書式 ---

// chartDecimals は値の小数の桁の数の最大(2まで)。
func chartDecimals(vals []float64) int {
	dec := 0
	for _, v := range vals {
		s := strconv.FormatFloat(v, 'f', -1, 64)
		if _, frac, ok := strings.Cut(s, "."); ok {
			dec = max(dec, len(frac))
		}
	}
	return min(dec, 2)
}

// chartNumFmt は Excel の書式(3桁の区切り・小数の桁・後ろに単位)。
func chartNumFmt(dec int, unit string) string {
	f := "#,##0"
	if dec > 0 {
		f += "." + strings.Repeat("0", dec)
	}
	if unit != "" {
		f += `"` + unit + `"`
	}
	return f
}

// chartFormat は書式どおりに数を書いた文字(目盛りの幅を測るため)。
func chartFormat(v float64, dec int, unit string) string {
	return formatFunnelNumber(math.Round(v*math.Pow10(dec))/math.Pow10(dec)) + unit
}

func (s *ChartSubsystem) allValues() []float64 {
	var out []float64
	for _, se := range s.Data.Series {
		out = append(out, se.Values...)
		for _, p := range se.Points {
			out = append(out, p[1])
		}
	}
	return out
}

func (s *ChartSubsystem) xValues() []float64 {
	var out []float64
	for _, se := range s.Data.Series {
		for _, p := range se.Points {
			out = append(out, p[0])
		}
	}
	return out
}

// valueExtent は値の軸に出る一番大きな絶対値(積み上げなら項目ごとの和)。
func (s *ChartSubsystem) valueExtent() float64 {
	d := s.Data
	m := 0.0
	if d.Stacked {
		for i := range d.Categories {
			pos, neg := 0.0, 0.0
			for _, se := range d.Series {
				if i < len(se.Values) {
					if se.Values[i] >= 0 {
						pos += se.Values[i]
					} else {
						neg -= se.Values[i]
					}
				}
			}
			m = math.Max(m, math.Max(pos, neg))
		}
		return m
	}
	for _, v := range s.allValues() {
		m = math.Max(m, math.Abs(v))
	}
	return m
}

// tickLabelWidth は値の軸の目盛りの文字の幅(一番長いもの)の見積もり。
// アプリは一番大きな値より1刻み上まで目盛りを振るので、桁が1つ増える分を見込む。
func (s *ChartSubsystem) tickLabelWidth(vals []float64, extent float64, unit string) float64 {
	dec := chartDecimals(vals)
	txt := chartFormat(extent*1.2, dec, unit)
	if strings.HasPrefix(txt, "-") {
		txt = txt[1:]
	}
	return runsNaturalWidth(plainRuns("-"+txt), s.fontSize())
}

func plainRuns(s string) []TextRun { return []TextRun{{Text: s}} }

// --- 大きさ ---

type chartGeom struct {
	min, nat, max float64 // 高さ
	slot          float64 // 1項目の幅(縦の形)か、項目の名前の列の幅(横棒)
	labelLines    int     // 項目の名前の行の数(一番多いもの)
}

func (s *ChartSubsystem) lineH() float64 { return s.fontSize() * lineHeightMult }

func (s *ChartSubsystem) legend() bool {
	return s.Form() != ChartPie && len(s.Data.Series) > 1
}

func (s *ChartSubsystem) dataLabels() bool {
	switch s.Form() {
	case ChartPie:
		return true
	case ChartBar, ChartColumn, ChartHistogram:
		return len(s.Data.Series) == 1
	}
	return false
}

// labelLinesAt は項目の名前を幅 w で折り返したときの一番多い行の数。
func (s *ChartSubsystem) labelLinesAt(w float64) int {
	n := 1
	for _, c := range s.Data.Categories {
		n = max(n, len(wrapRuns(plainRuns(c), s.fontSize(), math.Max(w, 0))))
	}
	return n
}

func (s *ChartSubsystem) maxLabelWidth() float64 {
	w := 0.0
	for _, c := range s.Data.Categories {
		w = math.Max(w, runsNaturalWidth(plainRuns(c), s.fontSize()))
	}
	return w
}

func (s *ChartSubsystem) geom(width float64) chartGeom {
	v, lh, d := s.vals(), s.lineH(), s.Data
	extra := 2 * v.Padding
	if s.legend() {
		extra += lh + v.Padding
	}
	var g chartGeom
	switch form := s.Form(); form {
	case ChartBar, ChartStackedBar:
		g.slot = math.Min(s.maxLabelWidth(), width*v.BarLabelFrac)
		g.labelLines = s.labelLinesAt(g.slot)
		k := 1.0
		if form == ChartBar {
			k = 1 + 0.5*float64(len(d.Series)-1) // 並べた棒は系列の数だけ太る
		}
		need := float64(g.labelLines)*lh + v.Padding/2
		n := float64(len(d.Categories))
		extra += lh // 下の目盛りの行
		g.min = n*math.Max(v.MinBarSlot*k, need) + extra
		g.nat = n*math.Max(v.NaturalBarSlot*k, need) + extra
		g.max = n*math.Max(v.MaxBarSlot*k, need) + extra
		return g
	case ChartPie:
		// 扇の外に項目の名前と値の2行が上下に出る。
		g.min = v.MinPlotHeight + 4*lh + extra
	case ChartScatter:
		// 下に目盛りの行と軸の題、左に軸の題。
		g.min = v.MinPlotHeight + 2*lh + extra
	default:
		yW := s.tickLabelWidth(s.allValues(), s.valueExtent(), d.Unit)
		plotW := width - 2*v.Padding - yW - v.Padding
		g.slot = plotW / float64(max(len(d.Categories), 1))
		g.labelLines = s.labelLinesAt(g.slot * 0.9)
		extra += float64(min(g.labelLines, int(v.LabelMaxLines)))*lh + v.Padding/2
		if s.dataLabels() {
			extra += lh // 一番高い棒の上の値
		}
		g.min = v.MinPlotHeight + extra
	}
	g.nat = math.Max(g.min, width*v.NaturalAspect)
	if s.Form() == ChartPie {
		// 円は幅いっぱいの直径(左右に扇の外の名前と値を残す)を自然な高さにする。
		// 細い枠(並列のまとまりの中)でも円が小さくなりすぎない。
		g.nat = math.Max(g.min, width-s.pieSide()+2*v.Padding+4*lh)
	}
	return g
}

// Measure: 横棒は項目の数で高さが決まる(最小・自然・上限)。ほかは幅に比例した
// 高さを自然とし、値の軸(円は直径)の最小に名前・凡例の行を足したものを最小にする。
// 上限は持たない(グラフは大きくしても読みやすさを損なわない)。
func (s *ChartSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	w := v.NaturalWidth
	if c.Width > 0 {
		w = c.Width
	}
	g := s.geom(w)
	m := Measurement{
		Natural: Size{Width: w, Height: g.nat},
		Min:     Size{Width: v.MinWidth, Height: g.min},
	}
	if f := s.Form(); f == ChartBar || f == ChartStackedBar {
		m.Max = Size{Height: g.max}
	}
	if s.Form() == ChartPie && c.Height > 0 {
		// 円は高さで直径が決まり、横には扇の外の名前と値の分しか要らない。
		// その幅を横の上限にする(余りは切り詰めて Place の側に残し、spare-width で知らせる)。
		m.Max = Size{Width: s.pieWidth(c.Height)}
	}
	return m
}

// pieWidth は高さ h の枠で円グラフが使う幅:直径(高さから上下の名前と値の2行ずつと
// 余白を引いたもの)+左右の扇の外の名前・値の幅+余白。円より広い枠は横が空くだけなので、
// これを横の上限として申告する。
func (s *ChartSubsystem) pieWidth(h float64) float64 {
	v, lh := s.vals(), s.lineH()
	d := math.Max(h-2*v.Padding-4*lh, v.MinPlotHeight)
	return math.Max(d+s.pieSide(), v.MinWidth)
}

// pieSide は円の左右に要る幅の合計:扇の外の名前・値(一番広いもの)と引き出しの
// 隙間を左右に1つずつ+内側の余白。
func (s *ChartSubsystem) pieSide() float64 {
	v, lh, fs := s.vals(), s.lineH(), s.fontSize()
	dec := chartDecimals(s.allValues())
	lw := 0.0
	for i, c := range s.Data.Categories {
		lw = math.Max(lw, runsNaturalWidth(plainRuns(c), fs))
		for _, se := range s.Data.Series {
			if i < len(se.Values) {
				lw = math.Max(lw, runsNaturalWidth(plainRuns(chartFormat(se.Values[i], dec, s.Data.Unit)), fs))
			}
		}
	}
	return 2*(lw+lh) + 2*v.Padding
}

// --- 描く ---

// seriesColors は系列ごとの色の役割名。強調した系列(無ければ1つ目)が accent。
func (s *ChartSubsystem) seriesColors() []string {
	d := s.Data
	out := make([]string, len(d.Series))
	first := 0
	for i, se := range d.Series {
		if se.Emphasis {
			first = i
		}
	}
	k := 1
	for i := range d.Series {
		if i == first {
			out[i] = chartPalette[0]
			continue
		}
		out[i] = chartPalette[min(k, len(chartPalette)-1)]
		k++
	}
	return out
}

func (s *ChartSubsystem) emphasized(cat string) bool { return contains(s.Data.Emphasis, cat) }

// ChartData は描くもの(色は解決済み)。
func (s *ChartSubsystem) chartData(roles Roles, fonts *FontSet) *ChartData {
	v, d := s.vals(), s.Data
	form := s.Form()
	cd := &ChartData{
		Kind: form, Categories: d.Categories,
		FontSize: s.fontSize(), TextColor: roles.Color("text_muted"),
		GridColor: roles.Color("grid"), AxisColor: roles.Color("grid"),
		Legend: s.legend(), DataLabels: s.dataLabels(),
		GapWidth: v.GapWidth, LineWidth: v.LineWidth, MarkerSize: v.MarkerSize,
		AxisLineWidth: v.AxisLineWidth, GridLineWidth: v.GridLineWidth,
		PieOutline: roles.Color("bg"), PieOutlineWidth: v.PieOutlineWidth,
	}
	if fonts != nil && fonts.Latin != nil && fonts.EastAsian != nil {
		cd.Latin, cd.EA = fonts.Latin.Family, fonts.EastAsian.Family
	}
	// 値の書式(目盛りと値のラベル)に単位を付ける。散布図の単位は軸の題に書く。
	cd.NumFmt = chartNumFmt(chartDecimals(s.allValues()), d.Unit)
	switch form {
	case ChartHistogram:
		cd.GapWidth = v.HistogramGap
	case ChartScatter:
		cd.MarkerSize = v.ScatterMarker
		cd.XTitle, cd.YTitle = axisTitle(d.XAxis), axisTitle(d.YAxis)
		cd.XNumFmt = chartNumFmt(chartDecimals(s.xValues()), "")
	}
	colors := s.seriesColors()
	for i, se := range d.Series {
		cs := ChartSeries{Name: se.Name, Values: se.Values, Color: roles.Color(colors[i])}
		for _, p := range se.Points {
			cs.XValues = append(cs.XValues, p[0])
			cs.Values = append(cs.Values, p[1])
		}
		switch {
		case form == ChartPie:
			// 強調した扇(無ければ1つ目)が accent、ほかは none と text_muted を交互に。
			cs.PointColors = map[int]string{}
			anyEm := len(d.Emphasis) > 0
			k := 0
			for ci, c := range d.Categories {
				if (anyEm && s.emphasized(c)) || (!anyEm && ci == 0) {
					cs.PointColors[ci] = roles.Color("accent")
					continue
				}
				cs.PointColors[ci] = roles.Color([]string{"none", "text_muted"}[k%2])
				k++
			}
		case len(d.Emphasis) > 0:
			// 1系列で項目を強調:強調した項目が accent、ほかは none。
			cs.Color = roles.Color("none")
			cs.PointColors = map[int]string{}
			for ci, c := range d.Categories {
				if s.emphasized(c) {
					cs.PointColors[ci] = roles.Color("accent")
				}
			}
		}
		cd.Series = append(cd.Series, cs)
	}
	return cd
}

func axisTitle(a *ChartAxis) string {
	if a == nil {
		return ""
	}
	if a.Unit != "" {
		return a.Title + "(" + a.Unit + ")"
	}
	return a.Title
}

func (s *ChartSubsystem) Render(ctx RenderContext, box LBox) {
	ctx.Out.AddChart(box, s.chartData(ctx.Roles, ctx.Out.Fonts))
}

// Lint は項目の名前が、アプリが置く場所に入りきらないこと(chart-labels)を警告にする。
// 入りきらないとアプリは名前を斜めにする・間引く・何行にも折るので、読めなくなる。
func (s *ChartSubsystem) Lint(box LBox) []string {
	v, d := s.vals(), s.Data
	form := s.Form()
	if form == ChartPie || form == ChartScatter {
		return nil
	}
	g := s.geom(box.Width)
	fs := s.fontSize()
	var bad []int
	var needW float64
	haveW := g.slot * 0.9
	if form == ChartBar || form == ChartStackedBar {
		haveW = g.slot
	}
	for i, c := range d.Categories {
		runs := plainRuns(c)
		lines := len(wrapRuns(runs, fs, math.Max(haveW, 0)))
		if mw := runsMinWidth(runs, fs); mw > haveW+0.5 || float64(lines) > v.LabelMaxLines {
			if len(bad) == 0 {
				needW = math.Max(mw, runsNaturalWidth(runs, fs)/v.LabelMaxLines)
			}
			bad = append(bad, i)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	more := ""
	if len(bad) > 1 {
		more = fmt.Sprintf(" and %d more", len(bad)-1)
	}
	return []string{fmt.Sprintf("[SUBSYSTEM WARNING] chart: categories[%d] (%q)%s: label does not fit its place on the category axis (needs %.1fpt within %d lines, has %.1fpt); the app will tilt, skip or wrap the labels",
		bad[0], d.Categories[bad[0]], more, needW, int(v.LabelMaxLines), math.Max(haveW, 0))}
}

// --- IR から読む ---

type chartSeriesIR struct {
	Name     string       `json:"name"`
	Values   []float64    `json:"values"`
	Points   [][2]float64 `json:"points"`
	Emphasis bool         `json:"emphasis"`
}

type chartIR struct {
	Compare    string          `json:"compare"`
	Categories []string        `json:"categories"`
	Series     []chartSeriesIR `json:"series"`
	Stacked    *bool           `json:"stacked"`
	Unit       *string         `json:"unit"`
	Emphasis   []string        `json:"emphasis"`
	XAxis      *ChartAxis      `json:"xAxis"`
	YAxis      *ChartAxis      `json:"yAxis"`
}

// chartProblem は data の中の意味の誤り。at は data の中の JSON Pointer。
type chartProblem struct {
	at, msg, code string
}

// chartProblems は形(Schema)は正しいが意味の通らないものを全部挙げる。check と
// 読み込み(chartFromIR)が同じものを使う。code は chart-values か chart-emphasis。
func chartProblems(in *chartIR) []chartProblem {
	var out []chartProblem
	add := func(at, code, f string, a ...any) { out = append(out, chartProblem{at, fmt.Sprintf(f, a...), code}) }
	corr := in.Compare == CompareCorrelation
	nCat := len(in.Categories)
	if !contains(chartCompares, in.Compare) {
		add("/compare", "chart-values", "unknown compare %q (known: %s)", in.Compare, strings.Join(chartCompares, ", "))
		return out
	}
	if corr {
		for _, k := range []struct {
			present bool
			key     string
		}{{nCat > 0, "categories"}, {in.Unit != nil, "unit"}, {len(in.Emphasis) > 0, "emphasis"}, {in.Stacked != nil, "stacked"}} {
			if k.present {
				add("/"+k.key, "chart-values", "%s is not used with compare correlation (the axes come from xAxis and yAxis)", k.key)
			}
		}
		if in.XAxis == nil || in.YAxis == nil {
			add("", "chart-values", "compare correlation needs xAxis and yAxis (what each axis measures)")
		}
	} else {
		if nCat < 2 {
			add("/categories", "chart-values", "compare %s needs at least two categories", in.Compare)
		}
		if in.XAxis != nil || in.YAxis != nil {
			add("", "chart-values", "xAxis and yAxis are only for compare correlation")
		}
	}
	if len(in.Series) == 0 {
		add("/series", "chart-values", "a chart needs at least one series")
	}
	if len(in.Series) > len(chartPalette) {
		add("/series", "chart-values", "a chart has at most %d series (got %d); split the chart or move the rest to a table", len(chartPalette), len(in.Series))
	}
	for i, se := range in.Series {
		at := fmt.Sprintf("/series/%d", i)
		if strings.TrimSpace(se.Name) == "" {
			add(at+"/name", "chart-values", "a series needs a name")
		}
		if corr {
			if se.Values != nil {
				add(at+"/values", "chart-values", "compare correlation takes points [[x, y], …], not values")
			}
			if len(se.Points) < 2 {
				add(at+"/points", "chart-values", "a correlation series needs at least two points")
			}
			continue
		}
		if se.Points != nil {
			add(at+"/points", "chart-values", "points are only for compare correlation; write values (one per category)")
		}
		if len(se.Values) != nCat {
			add(at+"/values", "chart-values", "series has %d values but there are %d categories (one value per category)", len(se.Values), nCat)
		}
	}
	stacked := in.Stacked != nil && *in.Stacked
	switch in.Compare {
	case CompareComponent, CompareFrequency:
		if len(in.Series) > 1 {
			add("/series", "chart-values", "compare %s takes exactly one series (got %d)", in.Compare, len(in.Series))
		}
		if stacked {
			add("/stacked", "chart-values", "stacked is only for compare item and timeSeries")
		}
	case CompareItem, CompareTimeSeries:
		if stacked && len(in.Series) < 2 {
			add("/stacked", "chart-values", "stacked needs at least two series (the parts of each total)")
		}
	}
	if in.Compare == CompareComponent {
		if nCat > 6 {
			add("/categories", "chart-values", "compare component has at most 6 parts (got %d); merge the small ones into one part", nCat)
		}
		if len(in.Series) == 1 {
			sum := 0.0
			for k, x := range in.Series[0].Values {
				if x < 0 {
					add(fmt.Sprintf("/series/0/values/%d", k), "chart-values", "a part of a whole cannot be negative (%v)", x)
				}
				sum += x
			}
			if sum <= 0 && len(in.Series[0].Values) > 0 {
				add("/series/0/values", "chart-values", "the parts add up to %v; a component chart needs a positive total", sum)
			}
		}
	}
	if in.Unit != nil && strings.ContainsAny(*in.Unit, `"\`) {
		add("/unit", "chart-values", "unit cannot contain \" or \\")
	}
	// 強調
	if len(in.Emphasis) > 0 && len(in.Series) > 1 {
		add("/emphasis", "chart-emphasis", "emphasis names categories and needs a single series; with several series, mark one series with emphasis: true")
	}
	seen := map[string]bool{}
	for k, e := range in.Emphasis {
		if !contains(in.Categories, e) && !corr {
			add(fmt.Sprintf("/emphasis/%d", k), "chart-emphasis", "%q is not one of the categories (%s)", e, strings.Join(in.Categories, ", "))
		}
		if seen[e] {
			add(fmt.Sprintf("/emphasis/%d", k), "chart-emphasis", "%q is emphasized twice", e)
		}
		seen[e] = true
	}
	nEm := 0
	for i, se := range in.Series {
		if se.Emphasis {
			nEm++
			if nEm == 2 {
				add(fmt.Sprintf("/series/%d/emphasis", i), "chart-emphasis", "at most one series can be emphasized")
			}
			if len(in.Emphasis) > 0 {
				add(fmt.Sprintf("/series/%d/emphasis", i), "chart-emphasis", "emphasize either categories or a series, not both")
			}
		}
	}
	return out
}

func chartFromIR(raw json.RawMessage) (*ChartContent, error) {
	var in chartIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("chart: %w", err)
	}
	if ps := chartProblems(&in); len(ps) > 0 {
		return nil, fmt.Errorf("chart: %s: %s", pointerToLocation(ps[0].at), ps[0].msg)
	}
	d := &ChartContent{Compare: in.Compare, Categories: in.Categories, Emphasis: in.Emphasis, XAxis: in.XAxis, YAxis: in.YAxis}
	if in.Stacked != nil {
		d.Stacked = *in.Stacked
	}
	if in.Unit != nil {
		d.Unit = *in.Unit
	}
	for _, se := range in.Series {
		d.Series = append(d.Series, ChartSeriesData{Name: se.Name, Values: se.Values, Points: se.Points, Emphasis: se.Emphasis})
	}
	return d, nil
}

// pointerToLocation は data の中の JSON Pointer(/series/0/values)を、誤りの文の
// 場所の形(series[0].values)にする。check の locationToPointer が元に戻す。
func pointerToLocation(ptr string) string {
	var b strings.Builder
	for _, seg := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		if seg == "" {
			continue
		}
		if _, err := strconv.Atoi(seg); err == nil {
			b.WriteString("[" + seg + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(seg)
	}
	if b.Len() == 0 {
		return "data"
	}
	return b.String()
}

func chartDecl() SubsystemDecl {
	return SubsystemDecl{
		Schema: embeddedSchema("subsystems/chart.schema.json"),
		Markdown: []string{
			"中身は YAML で、キーは IR の data と同じ。知らないキーは誤り",
			"compare に比べ方を書く:item(項目の比較)・timeSeries(時系列)・component(構成比)・frequency(度数分布)・correlation(相関)。グラフの形・色・大きさは書けない(サブシステムが比べ方とデータの数から選ぶ)",
			"categories に項目の名前、series に系列(name と、項目ごとに1つの values)。correlation だけは categories を書かず、xAxis・yAxis と系列ごとの points: [[x, y], …]",
			"値は引用しない数。unit は値の後ろに付く単位(例 件・%)。% は割合を 46 のように百分率の数で書く",
			"目立たせる項目は emphasis: [項目の名前](1系列のとき)。系列が複数なら目立たせる系列に emphasis: true(1つまで)",
			"stacked: true は系列が合計の内訳のとき(item・timeSeries で系列が2つ以上)",
		},
	}
}
