// 図のサブシステム(process・orgchart・matrix・funnel)に共通のもの。
//
// どれも「意味だけのデータ」(段階・組織・評価・数の減り方)を受け取り、置き場所・
// 大きさ・色はサブシステムが決める。shapes と違って作者は座標も役割名も書かない。
// 書けるのは状態(support・conditional・none・danger)と強調(emphasis)だけで、
// 色はそこからテーマの役割名を通して決まる。
//
//   - 強調(emphasis): 塗りが accent、文字が on_conditional。
//   - 状態(status):   塗りが card_alt、枠線が状態の色(役割名は状態の名前そのもの)。
//   - どちらも無い:     塗りが card_alt、枠線なし。
//
// 文字の大きさはテンプレートの values の段(body・subPoint・small)から引く。
// 縦の大きさは、部品ごとの「最小・自然・上限」を足し合わせて Measure で申告し、
// 割り当てられた高さに応じて最小〜上限の間で配る(flexSizes)。設計は
// docs/subsystem.md の11節。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

// figureStatuses は図の要素に書ける状態(Gantt と同じ意味の名前)。
var figureStatuses = []string{"support", "conditional", "none", "danger"}

// FigureText は図の文字の大きさ(pt)。テンプレートの values から入れる。
type FigureText struct {
	Title  float64 // 箱の題・見出し(values.body)
	Sub    float64 // 2段目の文字・セルの文字(values.subPoint)
	Detail float64 // 箱の補足・段階の下の補足・図の説明(values.figureDetail)
	Small  float64 // 凡例(values.small)
}

func figureTextFrom(v TemplateValues) FigureText {
	return FigureText{Title: v.Body, Sub: v.SubPoint, Detail: v.FigureDetail, Small: v.Small}
}

// defaultFigureText は既定のテンプレート(light)の段。
func defaultFigureText() FigureText { return figureTextFrom(mustTemplate("light").Values) }

// FigureNode は図の1つの箱(process の段階・組織図の人や組織・funnel の段)。
type FigureNode struct {
	Title    Inline
	Detail   Inline // 無ければ空
	Status   string // "" か figureStatuses のどれか
	Emphasis bool
}

// figureNodeIR は IR の箱の共通のキー。
type figureNodeIR struct {
	Title    Inline `json:"title"`
	Detail   Inline `json:"detail"`
	Status   string `json:"status"`
	Emphasis bool   `json:"emphasis"`
}

func (n figureNodeIR) node(at string) (FigureNode, error) {
	if len(n.Title) == 0 {
		return FigureNode{}, fmt.Errorf("%s.title: must not be empty", at)
	}
	if err := checkFigureStatus(at, n.Status); err != nil {
		return FigureNode{}, err
	}
	return FigureNode{Title: n.Title, Detail: n.Detail, Status: n.Status, Emphasis: n.Emphasis}, nil
}

func checkFigureStatus(at, s string) error {
	if s != "" && !contains(figureStatuses, s) {
		return fmt.Errorf("%s.status: unknown status %q (known: %s)", at, s, strings.Join(figureStatuses, ", "))
	}
	return nil
}

// figureStyle は箱の塗り・枠線・文字の役割名(枠線が無ければ outline は "")。
func figureStyle(status string, emphasis bool) (fill, outline, text string) {
	fill, text = "card_alt", "text_primary"
	if emphasis {
		fill, text = "accent", "on_conditional"
	}
	return fill, status, text
}

// --- 文字の大きさを測る ---

// runsBox は文を幅 maxW で折り返したときの幅(一番長い行)と高さ。
func runsBox(runs []TextRun, size, maxW float64) (w, h float64) {
	lines := wrapRuns(runs, size, math.Max(maxW, 0))
	for _, l := range lines {
		w = math.Max(w, naturalRunsWidth(l, size))
	}
	return w, float64(len(lines)) * size * lineHeightMult
}

// runsNaturalWidth は折り返さないときの幅(改行で分けた一番長い行)。
func runsNaturalWidth(runs []TextRun, size float64) float64 {
	w, _ := runsBox(runs, size, math.Inf(1))
	return w
}

// runsMinWidth は一番長い語(折り返せない塊)の幅。
func runsMinWidth(runs []TextRun, size float64) float64 {
	w, _ := runsBox(runs, size, 0)
	return w
}

// --- 縦の配り方 ---

// flexPart は縦に積む1つの部品の大きさ(最小・自然・上限)。固定なら3つとも同じ。
type flexPart struct{ min, nat, max float64 }

func fixedPart(v float64) flexPart { return flexPart{v, v, v} }

func flexTotals(parts []flexPart) (min, nat, max float64) {
	for _, p := range parts {
		min += p.min
		nat += p.nat
		max += p.max
	}
	return
}

// flexSizes は合計 total を部品に配る。最小の和より小さければ全部最小(はみ出しは
// 最小の大きさの検査が警告する)、最小〜自然の間なら各部品の(自然−最小)に比例、
// 自然〜上限の間なら(上限−自然)に比例、上限より大きければ全部上限。
func flexSizes(parts []flexPart, total float64) []float64 {
	min, nat, max := flexTotals(parts)
	out := make([]float64, len(parts))
	for i, p := range parts {
		switch {
		case total <= min:
			out[i] = p.min
		case total <= nat:
			out[i] = p.min + (p.nat-p.min)*(total-min)/(nat-min)
		case total <= max:
			out[i] = p.nat + (p.max-p.nat)*(total-nat)/(max-nat)
		default:
			out[i] = p.max
		}
	}
	return out
}

// --- 描く ---

// drawFigureNode は箱を1つ描き、その図形の id を返す。文は題(titleSize)と
// 補足(detailSize、題の下の行)を1つの図形の中に書く(PowerPoint で1つの箱として
// 編集できるように)。
func drawFigureNode(ctx RenderContext, b LBox, n *FigureNode, titleSize, detailSize float64, fv figureBoxValues) int {
	fill, outline, text := figureStyle(n.Status, n.Emphasis)
	o := ShapeOpts{Prst: fv.prst, Fill: ctx.Roles.Color(fill), OutlineWidthPt: fv.outlineWidth,
		FontSize: titleSize, InsetX: fv.insetX, InsetY: fv.insetY, Radius: fv.radius}
	if outline != "" {
		o.Outline = ctx.Roles.Color(outline)
	}
	o.Runs = itemRuns(n.Title, text, ctx.Roles)
	if len(n.Detail) > 0 {
		detailColor := "text_muted"
		if n.Emphasis {
			detailColor = text
		}
		o.Paras = [][]TextRun{itemRuns(n.Detail, detailColor, ctx.Roles)}
		o.ParaSizes = []float64{detailSize}
	}
	return ctx.Out.AddShape(b, o)
}

// figureBoxValues は箱の描き方の寸法。
type figureBoxValues struct {
	prst                 string
	radius, outlineWidth float64
	insetX, insetY       float64
}

// nodeTextSize は箱の文(題+補足)を幅 w(箱の幅)で折り返したときの大きさ。
func nodeTextSize(n *FigureNode, titleSize, detailSize, w float64, fv figureBoxValues) (needW, needH float64) {
	inner := w - 2*fv.insetX
	tw, th := runsBox(itemRuns(n.Title, "", nil), titleSize, inner)
	needW, needH = tw, th
	if len(n.Detail) > 0 {
		dw, dh := runsBox(itemRuns(n.Detail, "", nil), detailSize, inner)
		needW, needH = math.Max(needW, dw), needH+dh
	}
	return needW + 2*fv.insetX, needH + 2*fv.insetY
}

// nodeNaturalWidth は箱の文を折り返さないときの箱の幅。
func nodeNaturalWidth(n *FigureNode, titleSize, detailSize float64, fv figureBoxValues) float64 {
	w := runsNaturalWidth(itemRuns(n.Title, "", nil), titleSize)
	if len(n.Detail) > 0 {
		w = math.Max(w, runsNaturalWidth(itemRuns(n.Detail, "", nil), detailSize))
	}
	return w + 2*fv.insetX
}

// orphanRunes は、折り返した段落の最後の行がこの文字数(空白を除く)以下なら
// 泣き別れ(「欧州 AI 委員 / 会」の「会」)とみなす数。
const orphanRunes = 2

// nodeOrphans は箱の幅 w で折り返したとき、題と補足の段落のうち最後の行が
// 泣き別れになるものの数。
func nodeOrphans(n *FigureNode, titleSize, detailSize, w float64, fv figureBoxValues) int {
	inner := w - 2*fv.insetX
	count := func(in Inline, size float64) int {
		if len(in) == 0 {
			return 0
		}
		orphans := 0
		// 段落("\n" で分けたもの)ごとに、2行以上に折り返して最後の行が短いかを見る。
		var paras [][]string
		if plain := in.Plain(); !strings.Contains(plain, "\n") {
			var lines []string // 太字も文字ごとに測る
			for _, l := range wrapRuns(itemRuns(in, "", nil), size, math.Max(inner, 0)) {
				lines = append(lines, runsText(l))
			}
			paras = [][]string{lines}
		} else {
			for _, p := range strings.Split(plain, "\n") {
				paras = append(paras, wrapText(p, size, math.Max(inner, 0), false))
			}
		}
		for _, lines := range paras {
			if len(lines) < 2 {
				continue
			}
			last := []rune(strings.TrimSpace(lines[len(lines)-1]))
			k := 0
			for _, r := range last {
				if r != ' ' && r != '\t' {
					k++
				}
			}
			if k <= orphanRunes {
				orphans++
			}
		}
		return orphans
	}
	return count(n.Title, titleSize) + count(n.Detail, detailSize)
}

// nodeComfortWidth は箱の幅を w から limit までの間で広げ、泣き別れの段落が
// 一番少なくなる一番狭い幅を返す(1pt 刻み)。泣き別れが無ければ w のまま。
// 隣の箱と重ならない幅(limit)は呼ぶ側が決める。
func nodeComfortWidth(n *FigureNode, titleSize, detailSize, w, limit float64, fv figureBoxValues) float64 {
	best, bestN := w, nodeOrphans(n, titleSize, detailSize, w, fv)
	for x := math.Floor(w) + 1; bestN > 0 && x <= limit; x++ {
		if k := nodeOrphans(n, titleSize, detailSize, x, fv); k < bestN {
			best, bestN = x, k
		}
	}
	return best
}

// nodeMinWidth は箱の文の一番長い語が収まる箱の幅。
func nodeMinWidth(n *FigureNode, titleSize, detailSize float64, fv figureBoxValues) float64 {
	w := runsMinWidth(itemRuns(n.Title, "", nil), titleSize)
	if len(n.Detail) > 0 {
		w = math.Max(w, runsMinWidth(itemRuns(n.Detail, "", nil), detailSize))
	}
	return w + 2*fv.insetX
}

// overflowWarning は文が箱に収まらないときの警告の文。where は data の中の場所
// (steps[2] など。check はこれを JSON Pointer にする)。
func overflowWarning(kind, where string, label Inline, needW, needH, haveW, haveH float64) string {
	return fmt.Sprintf("[SUBSYSTEM WARNING] %s: %s (%q): text does not fit (needs %.1fx%.1fpt, has %.1fx%.1fpt)",
		kind, where, firstLine(label), needW, needH, math.Max(haveW, 0), math.Max(haveH, 0))
}

func firstLine(in Inline) string {
	return strings.SplitN(in.Plain(), "\n", 2)[0]
}

// textFits は要る大きさが使える大きさに収まるか(0.5pt の遊び)。
func textFits(needW, needH, haveW, haveH float64) bool {
	const eps = 0.5
	return needW <= haveW+eps && needH <= haveH+eps
}

// --- 値の表(subsystems/<名前>.json) ---

// loadFigureValues は subsystems/<name>.json を v に読む(知らないキー・欠けたキーは
// 誤り、数は 0 より大きい)。
func loadFigureValues(name string, v any) {
	data, err := valueFiles.ReadFile("subsystems/" + name + ".json")
	if err == nil {
		err = decodeStrict(data, v)
	}
	if err == nil {
		err = checkPositive(name, derefValue(v))
	}
	if err != nil {
		panic(fmt.Errorf("subsystems/%s.json: %w", name, err))
	}
}

func derefValue(v any) any {
	switch x := v.(type) {
	case *ProcessValues:
		return *x
	case *OrgChartValues:
		return *x
	case *MatrixValues:
		return *x
	case *FunnelValues:
		return *x
	case *ChartValues:
		return *x
	}
	return v
}

var (
	figureDefaultsOnce sync.Once
	processDefaults    ProcessValues
	orgDefaults        OrgChartValues
	matrixDefaults     MatrixValues
	funnelDefaults     FunnelValues
	chartDefaults      ChartValues
)

func loadFigureDefaults() {
	figureDefaultsOnce.Do(func() {
		loadFigureValues("process", &processDefaults)
		loadFigureValues("orgchart", &orgDefaults)
		loadFigureValues("matrix", &matrixDefaults)
		loadFigureValues("funnel", &funnelDefaults)
		loadFigureValues("chart", &chartDefaults)
	})
}

func defaultProcessValues() *ProcessValues {
	loadFigureDefaults()
	v := processDefaults
	return &v
}

func defaultOrgChartValues() *OrgChartValues {
	loadFigureDefaults()
	v := orgDefaults
	return &v
}

func defaultMatrixValues() *MatrixValues {
	loadFigureDefaults()
	v := matrixDefaults
	return &v
}

func defaultFunnelValues() *FunnelValues {
	loadFigureDefaults()
	v := funnelDefaults
	return &v
}

func defaultChartValues() *ChartValues {
	loadFigureDefaults()
	v := chartDefaults
	return &v
}

// --- 登録 ---

// registerFigures は図のサブシステムを既定の表に足す。
func registerFigures(r *SubsystemRegistry) {
	must(r.Register("process", NewProcessSubsystem))
	must(r.Declare("process", processDecl()))
	must(r.Register("orgchart", NewOrgChartSubsystem))
	must(r.Declare("orgchart", orgChartDecl()))
	must(r.Register("matrix", NewMatrixSubsystem))
	must(r.Declare("matrix", matrixDecl()))
	must(r.Register("funnel", NewFunnelSubsystem))
	must(r.Declare("funnel", funnelDecl()))
	must(r.Register("chart", NewChartSubsystem))
	must(r.Declare("chart", chartDecl()))
}

// figureKinds は図のサブシステム(と chart)の名前。
var figureKinds = []string{"process", "orgchart", "matrix", "funnel", "chart"}

// templateFigures はテンプレートの values と subsystems の差し替えを、図の
// サブシステムの Factory に入れる(parseTemplateFile から呼ぶ)。
func templateFigures(t *Template, f *templateFile) error {
	text := figureTextFrom(f.Values)
	var pv *ProcessValues
	var ov *OrgChartValues
	var mv *MatrixValues
	var fv *FunnelValues
	var cv *ChartValues
	if s := f.Subsystems; s != nil {
		pv, ov, mv, fv, cv = s.Process, s.OrgChart, s.Matrix, s.Funnel, s.Chart
	}
	check := func(name string, override any, def any) error {
		if override == nil || isNilPtr(override) {
			return checkOnScale("subsystems/"+name+".json", derefValue(def), f.Scale)
		}
		if err := checkPositive("subsystems."+name, derefValue(override)); err != nil {
			return err
		}
		return checkOnScale("subsystems."+name, derefValue(override), f.Scale)
	}
	for _, c := range []struct {
		name     string
		override any
		def      any
	}{
		{"process", pv, defaultProcessValues()},
		{"orgchart", ov, defaultOrgChartValues()},
		{"matrix", mv, defaultMatrixValues()},
		{"funnel", fv, defaultFunnelValues()},
		{"chart", cv, defaultChartValues()},
	} {
		if err := check(c.name, c.override, c.def); err != nil {
			return err
		}
	}
	must(t.Subsystems.Override("process", func(data any) (Subsystem, error) {
		s, err := NewProcessSubsystem(data)
		if err != nil {
			return nil, err
		}
		s.(*ProcessSubsystem).V, s.(*ProcessSubsystem).Text = pv, &text
		return s, nil
	}))
	must(t.Subsystems.Override("orgchart", func(data any) (Subsystem, error) {
		s, err := NewOrgChartSubsystem(data)
		if err != nil {
			return nil, err
		}
		s.(*OrgChartSubsystem).V, s.(*OrgChartSubsystem).Text = ov, &text
		return s, nil
	}))
	must(t.Subsystems.Override("matrix", func(data any) (Subsystem, error) {
		s, err := NewMatrixSubsystem(data)
		if err != nil {
			return nil, err
		}
		s.(*MatrixSubsystem).V, s.(*MatrixSubsystem).Text = mv, &text
		return s, nil
	}))
	must(t.Subsystems.Override("funnel", func(data any) (Subsystem, error) {
		s, err := NewFunnelSubsystem(data)
		if err != nil {
			return nil, err
		}
		s.(*FunnelSubsystem).V, s.(*FunnelSubsystem).Text = fv, &text
		return s, nil
	}))
	must(t.Subsystems.Override("chart", func(data any) (Subsystem, error) {
		s, err := NewChartSubsystem(data)
		if err != nil {
			return nil, err
		}
		s.(*ChartSubsystem).V, s.(*ChartSubsystem).Text = cv, &text
		return s, nil
	}))
	return nil
}

func isNilPtr(v any) bool {
	switch x := v.(type) {
	case *ProcessValues:
		return x == nil
	case *OrgChartValues:
		return x == nil
	case *MatrixValues:
		return x == nil
	case *FunnelValues:
		return x == nil
	case *ChartValues:
		return x == nil
	}
	return false
}

// rawFigureData は Factory の data(IR の data か、既に読んだもの)を JSON にする。
func rawFigureData(kind string, data any) (json.RawMessage, error) {
	if r, ok := data.(json.RawMessage); ok {
		return r, nil
	}
	return nil, fmt.Errorf("%s: expected IR data, got %T", kind, data)
}

// figureDecl は図のサブシステムが名乗るものの共通の部分。
func figureDecl(schema string, extra ...string) SubsystemDecl {
	return SubsystemDecl{
		Schema: embeddedSchema(schema),
		Markdown: append([]string{
			"中身は YAML で、キーは IR の data と同じ。知らないキーは誤り",
			"置き場所・大きさ・色は書けない(サブシステムが決める)。状態は status に意味の名前(support・conditional・none・danger)、目立たせるものは emphasis: true",
			"文の値は本文と同じ文中の記法で読む。YAML の列で書くと各行を意味のある改行でつなぐ",
		}, extra...),
	}
}

// --- 状態の凡例(process・orgchart・funnel) ---
//
// 箱の枠線の色(状態)の意味は作者の言葉でしか書けないので、data の legend
// (状態の名前 → 文)に書いた状態だけを凡例に出す。使った状態に文が無ければ
// 警告 figure-legend(読み手は色の意味が分からない)。matrix は評価の既定の文が
// あるので別(matrix.go)。

// figureLegendValues は凡例の寸法(各サブシステムの値の表から入れる)。
type figureLegendValues struct {
	gap, itemGap     float64 // 図と凡例の間・項目の間
	swatchW, swatchH float64 // 見本の箱
	swatchGap        float64 // 見本と文の間
	outlineW, radius float64
}

// figureLegendIR は IR の legend を読む(キーは状態の名前だけ)。
func figureLegendIR(kind string, l orderedLegend) ([]LegendItem, error) {
	for _, li := range l {
		if !contains(figureStatuses, li.Role) {
			return nil, fmt.Errorf("%s: legend: unknown key %q (known: %s)", kind, li.Role, strings.Join(figureStatuses, ", "))
		}
	}
	return []LegendItem(l), nil
}

// figureLegendItems は描く項目(使った状態のうち文のあるもの、状態の順)と、
// 使ったのに文の無い状態。
func figureLegendItems(used map[string]bool, legend []LegendItem) (items []LegendItem, missing []string) {
	label := map[string]string{}
	for _, li := range legend {
		label[li.Role] = li.Label
	}
	for _, st := range figureStatuses {
		if !used[st] {
			continue
		}
		if l, ok := label[st]; ok && l != "" {
			items = append(items, LegendItem{Role: st, Label: l})
		} else {
			missing = append(missing, st)
		}
	}
	return items, missing
}

// figureLegendLayout は凡例を幅 avail に並べたもの(越えたら次の行へ)。
type figureLegendLayout struct {
	items []LegendItem
	rows  [][]int
	h     float64 // 凡例の高さ(項目が無ければ 0)
}

func (lv figureLegendValues) itemW(li LegendItem, size float64) float64 {
	return lv.swatchW + lv.swatchGap + runWidthPt(li.Label, size, false)
}

func layoutFigureLegend(items []LegendItem, avail, size float64, lv figureLegendValues) figureLegendLayout {
	l := figureLegendLayout{items: items}
	x := 0.0
	for i, li := range items {
		w := lv.itemW(li, size)
		if len(l.rows) == 0 || (x > 0 && x+w > avail) {
			l.rows = append(l.rows, nil)
			x = 0
		}
		l.rows[len(l.rows)-1] = append(l.rows[len(l.rows)-1], i)
		x += w + lv.itemGap
	}
	l.h = float64(len(l.rows)) * size * lineHeightMult
	return l
}

// part は凡例が縦に取る大きさ(図の下の余白の手前に足す)。無ければ nil。
func (l figureLegendLayout) part(lv figureLegendValues) []flexPart {
	if l.h == 0 {
		return nil
	}
	return []flexPart{fixedPart(lv.gap + l.h)}
}

// noWrapSlack は、折り返さない1行の文字の箱を、測った文字の幅より広げる分(pt)。
// 決まりの数:文字は左揃えなので箱の右の余りは見えない(見た目の寸法ではない)。
// アプリの字の幅が測った値より少し広くても、箱の端で文字が切れたり次の行へ
// 落ちたりしないためのゆとり(Gantt の凡例・図の凡例・funnel の転換率)。
const noWrapSlack = 4.0

// drawFigureLegend は凡例を (x, y) から描く。見本は箱と同じ形(地の塗りに状態の
// 色の枠線)。
func drawFigureLegend(ctx RenderContext, x0, y0 float64, l figureLegendLayout, size float64, lv figureLegendValues) {
	lineH := size * lineHeightMult
	for k, row := range l.rows {
		x, y := x0, y0+float64(k)*lineH
		for _, i := range row {
			li := l.items[i]
			ctx.Out.AddShape(LBox{X: x, Y: y + (lineH-lv.swatchH)/2, Width: lv.swatchW, Height: lv.swatchH},
				ShapeOpts{Prst: "roundRect", Fill: ctx.Roles.Color("card_alt"), Outline: ctx.Roles.Color(li.Role),
					OutlineWidthPt: lv.outlineW, Radius: lv.radius})
			lx := x + lv.swatchW + lv.swatchGap
			w := runWidthPt(li.Label, size, false)
			ctx.Out.AddTextBoxNoWrap(LBox{X: lx, Y: y, Width: w + noWrapSlack, Height: lineH}, li.Label, size, ctx.Roles.Color("text_muted"), false, "left", "top")
			x += lv.itemW(li, size) + lv.itemGap
		}
	}
}

// legendMissingWarnings は使ったのに凡例の文が無い状態の警告(figure-legend)。
func legendMissingWarnings(kind string, missing []string) []string {
	var out []string
	for _, st := range missing {
		out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] %s: legend: status %q is used but has no legend text (write legend: {%s: …}); the reader cannot tell what the outline color means",
			kind, st, st))
	}
	return out
}
