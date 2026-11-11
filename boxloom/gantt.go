// Ganttチャート専用サブエンジン(box_engine.pyのcanvas葉ノードに差し込まれる、
// 座標スケール型の専用レンダラ)。外側のレイアウトエンジン(Taffy→自作Python→
// このGo版)を何度差し替えても、ここのロジックは一切変更していない ―
// それが二層アーキテクチャ(outer layout + inner canvas sub-engine)の狙い。
//
// 本物のPythonソース(layout_pptx_poc_v11.py の render_gantt()/build_chart())を
// 見ながら移植し直したバージョン。色は生の16進数ではなく、必ず意味役割名
// ("support"/"conditional"/"none"/"danger"/"risk"/"accent"/"on_conditional"等)
// 経由でThemeを引く ― build_chart()自体はThemeの存在を一切知らない。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Theme は「意味役割名 → 実際の色(16進)」の対応表。render_gantt()はこの表の
// キー以外の文字列を一切知らない。build_chart()はtheme引数を受け取らない。
type Theme map[string]string

func (th Theme) c(role string) string { return th[role] }

// Color は Roles を満たす(テーマは役割名の解決器)。
func (th Theme) Color(role string) string { return th[role] }

// --- データモデル(本物のGanttSegment/GanttRow/GanttGroup/Annotation/GanttChart
//     ・TimeScaleと1:1対応) ---

type ctrlPoint struct {
	Date time.Time
	Frac float64
}

type TimeScale struct {
	Points []ctrlPoint
}

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func (ts *TimeScale) frac(dt time.Time) float64 {
	pts := ts.Points
	if !dt.After(pts[0].Date) {
		return pts[0].Frac
	}
	last := pts[len(pts)-1]
	if !dt.Before(last.Date) {
		return last.Frac
	}
	for i := 0; i < len(pts)-1; i++ {
		d0, f0 := pts[i].Date, pts[i].Frac
		d1, f1 := pts[i+1].Date, pts[i+1].Frac
		if !dt.Before(d0) && !dt.After(d1) {
			spanDays := d1.Sub(d0).Hours() / 24
			if spanDays == 0 {
				spanDays = 1
			}
			t := dt.Sub(d0).Hours() / 24 / spanDays
			return f0 + t*(f1-f0)
		}
	}
	return last.Frac
}

func (ts *TimeScale) X(dt time.Time, x0, width float64) float64 {
	return x0 + ts.frac(dt)*width
}

type GanttSegment struct {
	Start, End time.Time
	Role       string // 意味役割名。生の色は入らない。
	Label      string
	OpenEnd    bool
	Tentative  bool // 確定していない区間(破線の枠で描く)
}

type GanttRow struct {
	Label    string
	Segments []GanttSegment
	SubGroup string

	HasNote            bool
	NoteStart, NoteEnd time.Time
	NoteText           string
}

type GanttGroup struct {
	Label string // 複数行は"\n"で区切る(例: "第1世代\nBoard A\n120台\nStore X")
	Rows  []GanttRow

	HasRiskBand        bool
	RiskStart, RiskEnd time.Time
}

type Tick struct {
	Date      time.Time
	MainLabel string
	SubLabel  string
	Role      string
	// AutoDate なら MainLabel の代わりに Date を値の表の dateFormat で書く
	// (IR から作った Gantt)。
	AutoDate bool
}

type Milestone struct {
	Date  time.Time
	Label string
	// AutoDate なら Label の前に Date を値の表の dateFormat で書く(IR から作った Gantt)。
	AutoDate bool
}

// mainText は目盛りの上の段の文字。
func (t Tick) mainText(gv *GanttValues) string {
	if t.AutoDate {
		return formatGanttDate(t.Date, gv.DateFormat)
	}
	return t.MainLabel
}

// text は節目の文字(★ の後ろ)。
func (m Milestone) text(gv *GanttValues) string {
	if !m.AutoDate {
		return m.Label
	}
	d := formatGanttDate(m.Date, gv.DateFormat)
	if m.Label == "" {
		return d
	}
	return d + " " + m.Label
}

type LegendItem struct {
	Role  string
	Label string
}

// Annotation は日付への吹き出し。
//
// Direction を "up" / "down" にすると、手で決めた置き方(AnchorYFrac の高さ・
// そのレーン)になる(Go で直接組んだデモ用)。Direction が空なら、置き方は
// サブシステムが決める(IR から作るとき): 指す先は TargetGroup / TargetRow
// (-1 は「指さない」)で、上下のレーンと横のずらしは planAnnotations が
// 重なりを避けて選ぶ。
type Annotation struct {
	AnchorDate  time.Time
	AnchorYFrac float64
	Text        string
	Role        string
	Direction   string // "up" | "down" | ""(自動)
	Emphasis    bool

	TargetGroup, TargetRow int
}

type GanttChart struct {
	Scale       TimeScale
	Ticks       []Tick
	Groups      []GanttGroup
	Legend      []LegendItem
	Milestones  []Milestone
	Annotations []Annotation

	HasBreak    bool
	BreakBefore time.Time

	HasExtNote   bool
	ExtNoteStart time.Time
	ExtNoteYFrac float64
	ExtNoteLabel string
	ExtNoteRole  string
}

// 寸法は値の表(GanttValues、既定は subsystems/gantt.json)にある。

type placedBox struct{ x, y, w, h float64 }

func rectsOverlap(a, b placedBox, margin float64) bool {
	return !(a.x+a.w+margin < b.x || b.x+b.w+margin < a.x || a.y+a.h+margin < b.y || b.y+b.h+margin < a.y)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- サブシステムとしての Gantt(subsystem.go の Subsystem を満たす) ---

// GanttSubsystem は GanttChart(意味だけのデータ)を描くサブシステム。
type GanttSubsystem struct {
	Chart *GanttChart
	// V は寸法の値の表。nil なら既定(subsystems/gantt.json)。テンプレートは
	// templates/<名前>.json の subsystems.gantt で差し替えられる。
	V *GanttValues
}

func (g *GanttSubsystem) vals() *GanttValues {
	if g.V != nil {
		return g.V
	}
	return defaultGanttValues()
}

// NewGanttSubsystem は登録簿の Factory。data は *GanttChart か GanttChart。
func NewGanttSubsystem(data any) (Subsystem, error) {
	switch v := data.(type) {
	case *GanttChart:
		return &GanttSubsystem{Chart: v}, nil
	case GanttChart:
		return &GanttSubsystem{Chart: &v}, nil
	case json.RawMessage:
		// IR の data(schema/subsystems/gantt.schema.json の形)。
		chart, err := ganttFromIR(v)
		if err != nil {
			return nil, err
		}
		return &GanttSubsystem{Chart: chart}, nil
	}
	return nil, fmt.Errorf("gantt: expected *GanttChart or IR data, got %T", data)
}

func (g *GanttSubsystem) Name() string { return "gantt" }

func (g *GanttSubsystem) Render(ctx RenderContext, box LBox) {
	renderGantt(ctx.Out, box, g.Chart, ctx.Roles, g.vals())
}

// Measure は Gantt の自然な大きさと最小の大きさを申告する。
//
// 高さ = 周りの固定の部品(凡例・注釈のレーン・節目・軸)+ 行の高さ×行数 +
// グループ間の隙間。行の高さは全グループで同じなので、グループ名(複数行)が
// 収まるだけの行の高さも下限に入れる。幅が分かっていれば幅はそのまま返す。
func (g *GanttSubsystem) Measure(c Constraint) Measurement {
	gv := g.vals()
	chart := g.Chart
	gutterW := gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW
	natW := 2*gv.Padding + gutterW + gv.NaturalChartW
	if c.Width > 0 {
		natW = c.Width
	}
	m := ganttMetricsFor(gv, chart, natW)
	totalRows, nGroups := 0, len(chart.Groups)
	labelRowH := 0.0 // グループ名を収めるのに要る、1行あたりの高さ
	for i := range chart.Groups {
		gr := &chart.Groups[i]
		totalRows += len(gr.Rows)
		if len(gr.Rows) == 0 {
			continue
		}
		lh := measureTextHeight(gr.Label, gv.SmallText, gv.GutterGroupW-10, true)
		if v := lh / float64(len(gr.Rows)); v > labelRowH {
			labelRowH = v
		}
	}
	// 行の注記(破線の箱)の文は箱の中で折り返す。収まらない行だけ、収まる高さ
	// まで高くする(切り詰めない・縮めない。黙って潰さないため)。
	need := noteNeedGrid(gv, chart, natW)
	gaps := gv.GroupGap * float64(max(0, nGroups-1))
	bodyFor := func(rowH float64) float64 {
		if rowH < labelRowH {
			rowH = labelRowH
		}
		if !anyNeedAbove(need, rowH) {
			return rowH*float64(totalRows) + gaps
		}
		return ganttRows{base: rowH, need: need}.bodyHeight(gaps)
	}

	if lw := 2*gv.Padding + m.legendW; lw > natW && c.Width <= 0 {
		natW = lw
	}
	minW := 2*gv.Padding + gutterW + gv.MinChartW
	return Measurement{
		Natural: Size{Width: natW, Height: m.chromeH + bodyFor(gv.NaturalRowH)},
		Min:     Size{Width: minW, Height: m.chromeH + bodyFor(gv.MinRowH)},
		Max:     Size{Height: m.chromeH + bodyFor(gv.MaxRowH)},
	}
}

// ganttNoteNeed は、行の注記の文を破線の箱に収めるのに要る行の高さ。
type ganttNoteNeed struct {
	group, row int
	text       string
	rowH       float64 // 要る行の高さ(箱の上下の隙間 6pt を含む)
}

// ganttRows は行ごとの高さ。行の高さはふつう全行で同じ(base)だが、注記の文が
// base では収まらない行だけは、収まる高さ(need)まで高くする。
type ganttRows struct {
	base float64
	need [][]float64 // [グループ][行]。0 は「base でよい」
}

// h は行の高さ。
func (r ganttRows) h(g, i int) float64 { return math.Max(r.base, r.need[g][i]) }

// extra は base を越えた分(行 i より前の行の合計)。全部 base なら 0。
func (r ganttRows) extraBefore(g, i int) float64 {
	e := 0.0
	for j := 0; j < i; j++ {
		e += r.h(g, j) - r.base
	}
	return e
}

// top は行 i の上端(グループの上端からの距離)。
func (r ganttRows) top(g, i int) float64 { return float64(i)*r.base + r.extraBefore(g, i) }

// groupH はグループの高さ。
func (r ganttRows) groupH(g int) float64 {
	n := len(r.need[g])
	return float64(n)*r.base + r.extraBefore(g, n)
}

// ganttRowLayout は本体の高さ bodyH から行の高さを決める(renderGantt が使う)。
// 注記で高くする行が無ければ、全行が (bodyH − 隙間) ÷ 行数(minRowH 以上)。
func ganttRowLayout(gv *GanttValues, chart *GanttChart, boxWidth, bodyH float64) ganttRows {
	nGroups, totalRows := len(chart.Groups), 0
	for i := range chart.Groups {
		totalRows += len(chart.Groups[i].Rows)
	}
	avail := bodyH - gv.GroupGap*float64(nGroups-1)
	rowH := avail / float64(totalRows)
	if rowH < gv.MinRowH {
		rowH = gv.MinRowH
	}
	rows := ganttRows{base: rowH, need: noteNeedGrid(gv, chart, boxWidth)}
	if anyNeedAbove(rows.need, rowH) {
		rows = rowsFor(rows.need, avail, gv.MinRowH)
	}
	return rows
}

// anyNeedAbove は base より高くする行があるか。
func anyNeedAbove(need [][]float64, base float64) bool {
	for _, g := range need {
		for _, v := range g {
			if v > base {
				return true
			}
		}
	}
	return false
}

// noteNeedGrid は行ごとの注記の要る高さ(幅 boxWidth で描いたとき)。
func noteNeedGrid(gv *GanttValues, chart *GanttChart, boxWidth float64) [][]float64 {
	grid := make([][]float64, len(chart.Groups))
	for gi := range chart.Groups {
		grid[gi] = make([]float64, len(chart.Groups[gi].Rows))
	}
	for _, n := range ganttNoteNeeds(gv, chart, boxWidth) {
		grid[n.group][n.row] = n.rowH
	}
	// 区間の文が1行に収まらず帯の中で折り返す行も、収まる高さまで高くする。
	chartW := boxWidth - 2*gv.Padding - (gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW)
	for gi := range chart.Groups {
		for ri := range chart.Groups[gi].Rows {
			row := &chart.Groups[gi].Rows[ri]
			if row.HasNote {
				continue
			}
			for _, seg := range row.Segments {
				lp := planSegmentLabel(gv, chart, chartW, seg)
				if lp.mode == segLabelWrap && lp.rowH > grid[gi][ri] {
					grid[gi][ri] = lp.rowH
				}
			}
		}
	}
	return grid
}

// 区間の文の置き方。
const (
	segLabelNone   = iota // 文が無い
	segLabelFit           // 帯の中に1行で収まる(従来どおり)
	segLabelWrap          // 帯の中で折り返す(行を高くする)
	segLabelHidden        // 帯が狭すぎて置けない(描かずに警告する)
)

// ganttSegLabelMaxLines は、区間の文を帯の中で折り返してよい行数の上限。
// これを越えるほど帯が狭いときは、文を描かずに警告する(縦に細長い文の柱を
// 作らない)。
const ganttSegLabelMaxLines = 3

// segLabelPlan は区間の文の置き方の計画。
type segLabelPlan struct {
	mode        int
	textW, need float64 // 文に使える幅・文に要る幅(1行のとき)
	rowH        float64 // 折り返すときに要る行の高さ(帯の上下の隙間を含む)
}

// planSegmentLabel は、本体の幅 chartW で描いたときの区間の文の置き方を決める。
// Measure・Lint・描画が同じ計算を使う(申告と描画と警告がずれないように)。
func planSegmentLabel(gv *GanttValues, chart *GanttChart, chartW float64, seg GanttSegment) segLabelPlan {
	if seg.Label == "" {
		return segLabelPlan{mode: segLabelNone}
	}
	ww := chart.Scale.X(seg.End, 0, chartW) - chart.Scale.X(seg.Start, 0, chartW)
	textW := ww - 8
	lines := wrapText(seg.Label, gv.SmallText, math.Max(textW, 1), false)
	need := 0.0
	for _, para := range strings.Split(seg.Label, "\n") {
		need = math.Max(need, runWidthPt(para, gv.SmallText, false))
	}
	p := segLabelPlan{textW: textW, need: need}
	const eps = 1e-6
	if len(lines) == 1 && need <= textW+eps {
		p.mode = segLabelFit
		return p
	}
	fits := textW >= 2*gv.SmallText && len(lines) <= ganttSegLabelMaxLines
	for _, l := range lines {
		if runWidthPt(l, gv.SmallText, false) > textW+eps {
			fits = false // 折り返しても切れない語が帯より長い
		}
	}
	if !fits {
		p.mode = segLabelHidden
		return p
	}
	p.mode = segLabelWrap
	p.rowH = float64(len(lines))*gv.SmallText*lineHeightMult + ganttNoteBarInset
	return p
}

// rowsFor は行の高さの合計を avail(本体の高さ − グループ間の隙間)にする base を
// 決める。高くした行の分だけほかの行が低くなる。base は minRowH を下回らない
// (下回るときは本体からはみ出す。Measure の Min がそれを申告している)。
func rowsFor(need [][]float64, avail, minRowH float64) ganttRows {
	n := 0
	for _, g := range need {
		n += len(g)
	}
	r := ganttRows{need: need}
	if n == 0 {
		return r
	}
	base := avail / float64(n)
	for iter := 0; iter < n+1; iter++ {
		fixed, k := 0.0, 0
		for _, g := range need {
			for _, v := range g {
				if v > base {
					fixed += v
					k++
				}
			}
		}
		if k == 0 {
			break
		}
		if k == n {
			base = 0
			break
		}
		nb := (avail - fixed) / float64(n-k)
		if nb == base {
			break
		}
		base = nb
	}
	if base < minRowH {
		base = minRowH
	}
	r.base = base
	return r
}

// bodyHeight は base を決めたときの行の合計 + 隙間。
func (r ganttRows) bodyHeight(gaps float64) float64 {
	t := gaps
	for g := range r.need {
		t += r.groupH(g)
	}
	return t
}

// ganttNoteBarInset は行の高さと注記・区間の箱の高さの差(renderGantt の barH)。
const ganttNoteBarInset = 6

// ganttNoteNeeds は、幅 boxWidth で描いたときの各注記の要る行の高さ。箱の幅と
// 文の幅の取り方は renderGantt と同じ(Measure と描画がずれないように)。
func ganttNoteNeeds(gv *GanttValues, chart *GanttChart, boxWidth float64) []ganttNoteNeed {
	gutterW := gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW
	chartW := boxWidth - 2*gv.Padding - gutterW
	var out []ganttNoteNeed
	for gi := range chart.Groups {
		for ri := range chart.Groups[gi].Rows {
			row := &chart.Groups[gi].Rows[ri]
			if !row.HasNote || row.NoteText == "" {
				continue
			}
			ww := chart.Scale.X(row.NoteEnd, 0, chartW) - chart.Scale.X(row.NoteStart, 0, chartW)
			if ww < 1.0 {
				ww = 1.0
			}
			textW := ww - 10
			if textW < 1 {
				textW = 1
			}
			h := measureTextHeight(row.NoteText, gv.SmallText, textW, false)
			out = append(out, ganttNoteNeed{gi, ri, row.NoteText, h + ganttNoteBarInset})
		}
	}
	return out
}

// ganttMetrics は Measure と renderGantt が共有する、行の本体以外の寸法。
// 両方が同じ計算を使うので、申告した大きさと実際の描画がずれない。
type ganttMetrics struct {
	annTopH, annBottomH, milestoneH float64
	chromeH                         float64 // 行の本体以外の高さの合計(内側の余白を含む)
	legendW                         float64
}

// boxWidth はサブシステムに割り当てる幅(注釈をどちらのレーンに置くかが
// 横の重なりで変わるため)。
func ganttMetricsFor(gv *GanttValues, chart *GanttChart, boxWidth float64) ganttMetrics {
	var m ganttMetrics
	// レーンの高さは、そのレーンに置く箱のうち一番高いもの + 隙間。
	for _, pl := range planAnnotations(gv, chart, gv.Padding, boxWidth-2*gv.Padding) {
		if pl.up {
			m.annTopH = math.Max(m.annTopH, pl.h+gv.AnnLaneGap)
		} else {
			m.annBottomH = math.Max(m.annBottomH, pl.h+gv.AnnLaneGap)
		}
	}
	if len(chart.Milestones) > 0 {
		m.milestoneH = gv.MilestoneH + 4
	}
	// renderGantt の縦の並び: 余白 / 凡例 / 上の注釈 / 節目 / 軸の文字 / 軸 / 本体 / 下の注釈 / 余白
	m.chromeH = 2*gv.Padding + gv.LegendH + gv.LegendGap + m.annTopH + m.milestoneH + gv.AxisLabelH + 5 + 5 + m.annBottomH
	for _, li := range chart.Legend {
		m.legendW += 19 + 6 + runWidthPt(li.Label, gv.LabelText, false) + gv.Padding
	}
	return m
}

// renderGantt はbox(確定済みの絶対座標)の中にGanttチャート全体を描画する。
// 本物のrender_gantt()と同じ手順・同じピクセル計算式で移植している。
func renderGantt(p *PPTXBuilder, box LBox, chart *GanttChart, roles Roles, gv *GanttValues) {
	c := roles.Color
	f := ganttFrameFor(gv, chart, box)
	x0, y0, w := f.x0, f.y0, f.w

	// --- 凡例(実測した文字幅でカーソルを前進させる。本物と同じ) ---
	lx := x0
	for _, li := range chart.Legend {
		p.AddRectFull(LBox{X: lx, Y: y0 + 2, Width: 15, Height: 7}, c(li.Role), 2, c("text_muted"), "", 0.5)
		labelW := runWidthPt(li.Label, gv.LabelText, false)
		p.AddTextBoxNoWrap(LBox{X: lx + 19, Y: y0 - 2, Width: labelW + 4, Height: gv.LegendH}, li.Label, gv.LabelText, c("text_muted"), false, "left", "top")
		lx += 19 + 6 + labelW + gv.Padding
	}

	milestoneY, axisLabelY, axisY := f.milestoneY, f.axisLabelY, f.axisY
	chartX0, chartW := f.chartX0, f.chartW
	bodyTop, bodyH := f.bodyTop, f.bodyH

	sx := f.sx(chart)

	// --- ★マイルストーン ---
	for _, m := range chart.Milestones {
		mx := sx(m.Date)
		p.AddTextBoxNoWrap(LBox{X: mx - 74, Y: milestoneY, Width: 148, Height: gv.MilestoneH}, "★ "+m.text(gv), gv.SmallText, c("accent"), true, "center", "middle")
	}

	// --- 軸目盛り ---
	for _, t := range chart.Ticks {
		tx := sx(t.Date)
		p.AddTextBoxNoWrap(LBox{X: tx - 32, Y: axisLabelY, Width: 64, Height: 13}, t.mainText(gv), gv.LabelText, c(t.Role), true, "center", "top")
		if t.SubLabel != "" {
			p.AddTextBoxNoWrap(LBox{X: tx - 40, Y: axisLabelY + 12, Width: 80, Height: 12}, t.SubLabel, gv.SmallText, c("text_muted"), false, "center", "top")
		}
		p.AddDot(tx, axisY, 2.2, c(t.Role))
		p.AddLine(tx, bodyTop, tx, bodyTop+bodyH, 0.75, c("grid"), "dash")
	}
	p.AddLine(chartX0, axisY, x0+w, axisY, 1.25, c("text_muted"), "")

	// --- 区間圧縮のbreakマーク("//") ---
	if chart.HasBreak {
		pts := chart.Scale.Points
		bx := x0 + w
		for i := 0; i < len(pts)-1; i++ {
			if pts[i+1].Date.Equal(chart.BreakBefore) {
				bx = chartX0 + ((pts[i].Frac+pts[i+1].Frac)/2)*chartW
				break
			}
		}
		for _, dx := range []float64{-3.0, 3.0} {
			p.AddLine(bx+dx-3, axisY-7, bx+dx+3, axisY+7, 3.0, c("bg"), "")
			p.AddLine(bx+dx-3, axisY-7, bx+dx+3, axisY+7, 1.0, c("text_primary"), "")
		}
	}

	// --- 行本体 ---
	nGroups := len(chart.Groups)
	rows := f.rows
	rowH := rows.base

	subX := x0 + gv.GutterGroupW
	labelX := subX + gv.GutterSubW

	yCursor := bodyTop
	for gi := range chart.Groups {
		group := &chart.Groups[gi]
		groupTop := f.groupTops[gi]
		groupH := rowH*float64(len(group.Rows)) + rows.extraBefore(gi, len(group.Rows))

		if group.HasRiskBand {
			rb0, rb1 := sx(group.RiskStart), sx(group.RiskEnd)
			p.AddRect(LBox{X: rb0, Y: groupTop, Width: rb1 - rb0, Height: groupH}, c("risk"), 0)
		}

		p.AddRect(LBox{X: x0, Y: groupTop, Width: gv.GutterGroupW - 4, Height: groupH}, c("card_alt"), 6)
		p.AddTextBox(LBox{X: x0 + 3, Y: groupTop, Width: gv.GutterGroupW - 10, Height: groupH}, group.Label, gv.SmallText, c("text_primary"), true, "center", "middle")

		rowTops := make([]float64, len(group.Rows))
		for i := range group.Rows {
			rowTops[i] = groupTop + float64(i)*rowH + rows.extraBefore(gi, i)
		}

		// サブグループの縦結合セル
		i := 0
		for i < len(group.Rows) {
			sub := group.Rows[i].SubGroup
			j := i
			for j+1 < len(group.Rows) && group.Rows[j+1].SubGroup == sub {
				j++
			}
			if sub != "" {
				subTop := rowTops[i]
				subH := rowTops[j] + rows.h(gi, j) - subTop
				p.AddTextBox(LBox{X: subX + 2, Y: subTop, Width: gv.GutterSubW - 6, Height: subH}, sub, gv.SmallText, c("text_muted"), false, "center", "middle")
			}
			i = j + 1
		}

		for idx := range group.Rows {
			row := &group.Rows[idx]
			rowTop := rowTops[idx]
			thisH := rows.h(gi, idx)
			p.AddTextBoxNoWrap(LBox{X: labelX, Y: rowTop, Width: gv.GutterLabelW - 8, Height: thisH}, row.Label, gv.LabelText, c("text_muted"), false, "left", "middle")

			barH := thisH - ganttNoteBarInset
			barY := rowTop + (thisH-barH)/2

			if row.HasNote {
				nx1, nx2 := sx(row.NoteStart), sx(row.NoteEnd)
				ww := nx2 - nx1
				if ww < 1.0 {
					ww = 1.0
				}
				p.AddRectFull(LBox{X: nx1, Y: barY, Width: ww, Height: barH}, "", 3, c("text_muted"), "dash", 0.75)
				p.AddTextBox(LBox{X: nx1 + 5, Y: barY, Width: ww - 10, Height: barH}, row.NoteText, gv.SmallText, c("text_muted"), false, "left", "middle")
				continue
			}

			for _, seg := range row.Segments {
				bx1, bx2 := sx(seg.Start), sx(seg.End)
				ww := bx2 - bx1
				if ww < 1.0 {
					ww = 1.0
				}
				segColor := c(seg.Role)
				if seg.Tentative {
					p.AddRectFull(LBox{X: bx1, Y: barY, Width: ww, Height: barH}, "", 3, segColor, "dash", 1.25)
				} else {
					p.AddRect(LBox{X: bx1, Y: barY, Width: ww, Height: barH}, segColor, 3)
				}
				if lp := planSegmentLabel(gv, chart, chartW, seg); lp.mode == segLabelFit || lp.mode == segLabelWrap {
					labelColor := c("text_primary")
					if seg.Role == "conditional" && !seg.Tentative {
						labelColor = c("on_conditional")
					}
					if seg.Tentative {
						labelColor = segColor
					}
					lbox := LBox{X: bx1 + 5, Y: barY, Width: bx2 - bx1 - 8, Height: barH}
					if lp.mode == segLabelWrap {
						p.AddTextBox(lbox, seg.Label, gv.SmallText, labelColor, false, "left", "middle")
					} else {
						p.AddTextBoxNoWrap(lbox, seg.Label, gv.SmallText, labelColor, false, "left", "middle")
					}
				}
				if seg.OpenEnd {
					for _, dx := range []float64{5, 11, 17} {
						p.AddDot(bx2+dx, barY+barH/2, 1.4, segColor)
					}
				}
			}
		}

		yCursor += groupH
		if gi < nGroups-1 {
			p.AddLine(x0, yCursor+gv.GroupGap/2, x0+w, yCursor+gv.GroupGap/2, 0.75, c("grid"), "")
			yCursor += gv.GroupGap
		}
	}

	// --- 延伸ノート ---
	if chart.HasExtNote {
		ex0 := sx(chart.ExtNoteStart)
		ey := bodyTop + chart.ExtNoteYFrac*bodyH
		p.AddLine(ex0, ey, x0+w, ey, 1.25, c(chart.ExtNoteRole), "dash")
		p.AddTextBoxNoWrap(LBox{X: ex0, Y: ey - 15, Width: (x0 + w) - ex0, Height: 13}, chart.ExtNoteLabel, gv.SmallText, c(chart.ExtNoteRole), true, "left", "top")
	}

	// --- 注釈(置き場所と大きさは planAnnotations が決める) ---
	geoms := annotationGeoms(gv, chart, f)
	for i, ann := range chart.Annotations {
		lg := geoms[i]
		annColor := c(ann.Role)

		if ann.Emphasis {
			p.AddLine(lg.ax, bodyTop, lg.ax, bodyTop+bodyH, 2.0, annColor, "")
		}
		p.AddDot(lg.ax, lg.ay, 3.0, annColor)

		b := lg.box
		p.AddRectFull(LBox{X: b.x, Y: b.y, Width: b.w, Height: b.h}, c("card_alt"), 8, annColor, "", 1.25)
		if ann.Direction != "" {
			// 手で置いた注釈は従来どおりの余白。
			p.AddTextBox(LBox{X: b.x + 8, Y: b.y + 4, Width: b.w - 16, Height: b.h - 8}, ann.Text, gv.AnnText, c("text_primary"), false, "left", "top")
		} else {
			p.AddTextBox(LBox{X: b.x + gv.AnnPadX, Y: b.y + gv.AnnPadY, Width: b.w - 2*gv.AnnPadX, Height: b.h - 2*gv.AnnPadY}, ann.Text, gv.AnnText, c("text_primary"), false, "left", "top")
		}
		// 引き出し線は、目盛りと節目の文字を横切るところを空けて描く(文字に線を重ねない)。
		for _, sg := range lg.leader {
			p.AddLine(sg.x1, sg.y1, sg.x2, sg.y2, 1.0, annColor, "dash")
		}
	}
}

// ganttFrame は renderGantt の縦の並びと本体の行の位置(描くときの座標)。
// 描画と試験(引き出し線の検査)が同じ計算を使う。
type ganttFrame struct {
	x0, y0, w, h                       float64
	gm                                 ganttMetrics
	upY, milestoneY, axisLabelY, axisY float64
	chartX0, chartW                    float64
	bodyTop, bodyH, downY              float64
	rows                               ganttRows
	groupTops                          []float64
}

func ganttFrameFor(gv *GanttValues, chart *GanttChart, box LBox) ganttFrame {
	var f ganttFrame
	pad := gv.Padding
	f.x0, f.y0 = box.X+pad, box.Y+pad
	f.w, f.h = box.Width-2*pad, box.Height-2*pad
	f.gm = ganttMetricsFor(gv, chart, box.Width)
	f.upY = f.y0 + gv.LegendH + gv.LegendGap
	f.milestoneY = f.upY + f.gm.annTopH
	f.axisLabelY = f.milestoneY + f.gm.milestoneH
	f.axisY = f.axisLabelY + gv.AxisLabelH + 5
	gutterW := gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW
	f.chartX0 = f.x0 + gutterW
	f.chartW = f.w - gutterW
	f.bodyTop = f.axisY + 5
	f.bodyH = (f.y0 + f.h) - f.gm.annBottomH - f.bodyTop
	f.downY = f.bodyTop + f.bodyH + gv.AnnLaneGap
	f.rows = ganttRowLayout(gv, chart, box.Width, f.bodyH)
	f.groupTops = make([]float64, len(chart.Groups))
	yCursor := f.bodyTop
	for gi := range chart.Groups {
		f.groupTops[gi] = yCursor
		yCursor += f.rows.base*float64(len(chart.Groups[gi].Rows)) + f.rows.extraBefore(gi, len(chart.Groups[gi].Rows))
		if gi < len(chart.Groups)-1 {
			yCursor += gv.GroupGap
		}
	}
	return f
}

func (f ganttFrame) sx(chart *GanttChart) func(time.Time) float64 {
	return func(dt time.Time) float64 { return chart.Scale.X(dt, f.chartX0, f.chartW) }
}

// ganttLabelRects は、軸の目盛りの文字と節目の文字が実際に占める矩形(字の幅と
// 行の高さ。文字の箱そのものではない)。描くときの箱の置き方(中央揃え、目盛りは
// 上揃え、節目は上下の真ん中)から求める。上のレーンの注釈の引き出し線は必ず
// この帯を通るので、ここを避ける。本体の中の文字(区間の文・注記)は避けない
// (docs/subsystem.md の6節「引き出し線と文字」)。
func ganttLabelRects(gv *GanttValues, chart *GanttChart, f ganttFrame) []placedBox {
	sx := f.sx(chart)
	ink := func(cx, top, boxH float64, text string, size float64, bold bool, middle bool) placedBox {
		tw, n := 0.0, 0
		for _, line := range strings.Split(text, "\n") {
			tw = math.Max(tw, runWidthPt(line, size, bold))
			n++
		}
		th := float64(n) * size * lineHeightMult
		y := top
		if middle {
			y = top + (boxH-th)/2
		}
		return placedBox{cx - tw/2, y, tw, th}
	}
	var out []placedBox
	for _, m := range chart.Milestones {
		out = append(out, ink(sx(m.Date), f.milestoneY, gv.MilestoneH, "★ "+m.text(gv), gv.SmallText, true, true))
	}
	for _, t := range chart.Ticks {
		tx := sx(t.Date)
		if mt := t.mainText(gv); mt != "" {
			out = append(out, ink(tx, f.axisLabelY, 13, mt, gv.LabelText, true, false))
		}
		if t.SubLabel != "" {
			out = append(out, ink(tx, f.axisLabelY+12, 12, t.SubLabel, gv.SmallText, false, false))
		}
	}
	return out
}

// ganttLeaderGap は、引き出し線が文字を避けるときに文字との間に空ける隙間(pt)。
const ganttLeaderGap = 2.0

// lineSeg は線分。
type lineSeg struct{ x1, y1, x2, y2 float64 }

// annGeom は1つの注釈を描く座標: 日付の x・点の y・箱・引き出し線(文字を避けて
// 切った線分の列)。
type annGeom struct {
	ax, ay float64
	box    placedBox
	leader []lineSeg
}

// annotationGeoms は注釈を描く座標を決める(描画と試験が同じものを使う)。
func annotationGeoms(gv *GanttValues, chart *GanttChart, f ganttFrame) []annGeom {
	sx := f.sx(chart)
	plan := planAnnotations(gv, chart, f.x0, f.w)
	labels := ganttLabelRects(gv, chart, f)
	out := make([]annGeom, len(chart.Annotations))
	for i, ann := range chart.Annotations {
		pl := plan[i]
		ax := sx(ann.AnchorDate)
		ay := f.bodyTop + ann.AnchorYFrac*f.bodyH
		if ann.Direction == "" {
			ay = autoAnchorY(chart, ann, f.groupTops, f.rows, f.bodyTop, f.bodyH)
		}
		// 上のレーンは箱の下端を揃え(チャートに近い側)、下のレーンは上端を揃える。
		by := f.downY
		if pl.up {
			by = f.upY + (f.gm.annTopH - gv.AnnLaneGap) - pl.h
		}
		connY := by
		if by < ay {
			connY = by + pl.h
		}
		out[i] = annGeom{
			ax: ax, ay: ay,
			box:    placedBox{pl.bx, by, pl.w, pl.h},
			leader: splitAround(lineSeg{ax, ay, pl.bx + pl.w/2, connY}, labels, ganttLeaderGap),
		}
	}
	return out
}

// splitAround は線分 s から、矩形(gap だけ広げたもの)の中を通る部分を除いた
// 残りの線分を返す。どの矩形とも交わらなければ s をそのまま1本で返す。
// 1pt に満たない切れ端は描かない。
func splitAround(s lineSeg, rects []placedBox, gap float64) []lineSeg {
	type span struct{ t0, t1 float64 }
	var cut []span
	dx, dy := s.x2-s.x1, s.y2-s.y1
	for _, r := range rects {
		// Liang–Barsky: 線分のうち矩形の中にある t の範囲。
		t0, t1 := 0.0, 1.0
		ok := true
		for _, e := range [][2]float64{
			{-dx, s.x1 - (r.x - gap)}, {dx, (r.x + r.w + gap) - s.x1},
			{-dy, s.y1 - (r.y - gap)}, {dy, (r.y + r.h + gap) - s.y1},
		} {
			p, q := e[0], e[1]
			if p == 0 {
				if q < 0 {
					ok = false
				}
				continue
			}
			t := q / p
			if p < 0 {
				t0 = math.Max(t0, t)
			} else {
				t1 = math.Min(t1, t)
			}
		}
		if ok && t0 < t1 {
			cut = append(cut, span{t0, t1})
		}
	}
	if len(cut) == 0 {
		return []lineSeg{s}
	}
	sort.Slice(cut, func(i, j int) bool { return cut[i].t0 < cut[j].t0 })
	length := math.Hypot(dx, dy)
	var out []lineSeg
	at := func(t float64) (float64, float64) { return s.x1 + t*dx, s.y1 + t*dy }
	emit := func(a, b float64) {
		if (b-a)*length < 1 {
			return
		}
		x1, y1 := at(a)
		x2, y2 := at(b)
		out = append(out, lineSeg{x1, y1, x2, y2})
	}
	from := 0.0
	for _, c := range cut {
		if c.t0 > from {
			emit(from, c.t0)
		}
		from = math.Max(from, c.t1)
	}
	emit(from, 1)
	return out
}

// annPlacement は1つの注釈の置き場所と大きさ: 上のレーンか下のレーンか、
// 箱の左端と幅・高さ。overlap は、空いた場所が見つからず、ほかの箱に重ねて
// 置いたこと(警告の対象)。
type annPlacement struct {
	up      bool
	bx      float64
	w, h    float64
	overlap bool
}

// annBoxSize は注釈の箱の大きさ。手で置いた注釈は従来の固定の大きさ。
// 自動の注釈は、文の一番長い行が収まる幅(gv.AnnBoxMinW〜gv.AnnBoxMaxW)にし、
// 収まらなければ折り返して、その行数ぶんの高さにする。
func annBoxSize(gv *GanttValues, ann Annotation) (w, h float64) {
	if ann.Direction != "" {
		return gv.AnnBoxW, gv.AnnBoxH
	}
	textW := 0.0
	for _, line := range strings.Split(ann.Text, "\n") {
		textW = math.Max(textW, runWidthPt(line, gv.AnnText, false))
	}
	inner := clamp(textW, gv.AnnBoxMinW-2*gv.AnnPadX, gv.AnnBoxMaxW-2*gv.AnnPadX)
	h = measureTextHeight(ann.Text, gv.AnnText, inner, false) + 2*gv.AnnPadY
	return inner + 2*gv.AnnPadX, h
}

// planAnnotations は注釈の置き場所を決める。x0・w はチャートの内側の左端と幅
// (Measure と renderGantt で同じ幅を渡すので、申告した高さと描いた結果が
// ずれない)。重なりは同じレーンの中の横の重なりだけを見る(箱の高さが
// 違っても、同じレーンの箱は縦の帯を共有するため)。
//
//   - 手で決めた注釈(Direction が "up"/"down"): そのレーンで、日付の真上に
//     置き、ほかの箱と重なれば右へずらす(従来どおり)。
//   - 自動の注釈(Direction が空): 指す先がチャートの上半分なら上のレーン、
//     下半分(か真ん中)なら下のレーンを先に試す。日付の真上に置けなければ
//     もう一方のレーンの真上、それも駄目なら左右に少しずつずらして、最初に
//     空いた場所に置く。
//
// どちらも、空いた場所が見つからなければ最初の候補に重ねて置き、overlap を
// 立てる(GanttSubsystem.Lint が警告にする)。
func planAnnotations(gv *GanttValues, chart *GanttChart, x0, w float64) []annPlacement {
	gutterW := gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW
	chartX0, chartW := x0+gutterW, w-gutterW
	var placed []annPlacement
	free := func(up bool, bx, bw float64) bool {
		for _, pb := range placed {
			if pb.up == up && rectsOverlap(placedBox{bx, 0, bw, 1}, placedBox{pb.bx, 0, pb.w, 1}, 4.0) {
				return false
			}
		}
		return true
	}
	out := make([]annPlacement, len(chart.Annotations))
	for i, ann := range chart.Annotations {
		bw, bh := annBoxSize(gv, ann)
		step := bw + 8
		ax := chart.Scale.X(ann.AnchorDate, chartX0, chartW)
		center := clamp(ax-bw/2, x0, x0+w-bw)
		pl := annPlacement{w: bw, h: bh}
		if ann.Direction != "" {
			pl.up, pl.bx = ann.Direction == "up", center
			tries := 0
			for ; tries < 6 && !free(pl.up, pl.bx, bw); tries++ {
				pl.bx = clamp(pl.bx+step, x0, x0+w-bw)
			}
			pl.overlap = !free(pl.up, pl.bx, bw)
		} else {
			prefUp := annAnchorFrac(chart, ann) < 0.5
			pl.up, pl.bx = prefUp, center
			found := false
			for k := 0; k <= 6 && !found; k++ {
				offsets := []float64{0}
				if k > 0 {
					offsets = []float64{float64(k) * step / 2, -float64(k) * step / 2}
				}
				for _, off := range offsets {
					for _, up := range []bool{prefUp, !prefUp} {
						bx := clamp(center+off, x0, x0+w-bw)
						if free(up, bx, bw) {
							pl.up, pl.bx, found = up, bx, true
							break
						}
					}
					if found {
						break
					}
				}
			}
			pl.overlap = !found
		}
		placed = append(placed, pl)
		out[i] = pl
	}
	return out
}

// Lint は描く前の検査(SubsystemLinter)。割り当てられた矩形で注釈の置き場所を
// 計画し、重ねて置くしかなかった注釈を警告にする。
func (g *GanttSubsystem) Lint(box LBox) []string {
	gv := g.vals()
	var out []string
	plan := planAnnotations(gv, g.Chart, box.X+gv.Padding, box.Width-2*gv.Padding)
	for i, pl := range plan {
		if !pl.overlap {
			continue
		}
		first := strings.SplitN(g.Chart.Annotations[i].Text, "\n", 2)[0]
		out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] gantt: annotation %d (%q) has no free place and overlaps another annotation", i, first))
	}
	// 帯が狭すぎて区間の文を置けない(描かない)ものを警告にする。黙って消さない。
	chartW := box.Width - 2*gv.Padding - (gv.GutterGroupW + gv.GutterSubW + gv.GutterLabelW)
	for gi := range g.Chart.Groups {
		for ri := range g.Chart.Groups[gi].Rows {
			row := &g.Chart.Groups[gi].Rows[ri]
			if row.HasNote {
				continue
			}
			for si, seg := range row.Segments {
				lp := planSegmentLabel(gv, g.Chart, chartW, seg)
				if lp.mode != segLabelHidden {
					continue
				}
				first := strings.SplitN(seg.Label, "\n", 2)[0]
				out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] gantt: groups[%d].rows[%d].segments[%d] label (%q) is not drawn: the bar leaves %.1fpt for text, needs %.1fpt (up to %d lines)",
					gi, ri, si, first, math.Max(lp.textW, 0), lp.need, ganttSegLabelMaxLines))
			}
		}
	}
	return out
}

// annAnchorFrac は自動の注釈が指す先の、本体の中での縦の位置(0=上端、1=下端)。
// 行の数だけで決める(行の高さに依らないので、Measure でも同じ値になる)。
func annAnchorFrac(chart *GanttChart, ann Annotation) float64 {
	if ann.TargetGroup < 0 {
		return 0.5
	}
	total, before := 0, 0
	for gi := range chart.Groups {
		if gi < ann.TargetGroup {
			before += len(chart.Groups[gi].Rows)
		}
		total += len(chart.Groups[gi].Rows)
	}
	if total == 0 {
		return 0.5
	}
	if ann.TargetRow >= 0 {
		return (float64(before+ann.TargetRow) + 0.5) / float64(total)
	}
	return (float64(before) + float64(len(chart.Groups[ann.TargetGroup].Rows))/2) / float64(total)
}

// autoAnchorY は自動の注釈の点の高さ: 指したグループか行の縦の真ん中。
// 指す先が無ければ本体の真ん中。
func autoAnchorY(chart *GanttChart, ann Annotation, groupTops []float64, rows ganttRows, bodyTop, bodyH float64) float64 {
	if ann.TargetGroup < 0 || ann.TargetGroup >= len(groupTops) {
		return bodyTop + bodyH/2
	}
	g, rowH := ann.TargetGroup, rows.base
	top := groupTops[g]
	if ann.TargetRow >= 0 {
		i := ann.TargetRow
		return top + (float64(i)+0.5)*rowH + rows.extraBefore(g, i) + (rows.h(g, i)-rowH)/2
	}
	n := len(chart.Groups[g].Rows)
	return top + rowH*float64(n)/2 + rows.extraBefore(g, n)/2
}

// buildChart はv11(本物)のbuild_chart()と同じ内容を再現したもの。
// 色フィールドはすべて意味役割名で、build_chart()はtheme引数を一切持たない。
func buildChart() *GanttChart {
	scale := TimeScale{Points: []ctrlPoint{
		{d(2026, time.September, 1), 0.00},
		{d(2027, time.October, 1), 0.15},
		{d(2028, time.August, 1), 0.29},
		{d(2029, time.December, 1), 0.47},
		{d(2031, time.February, 1), 0.60},
		{d(2031, time.June, 1), 0.68},
		{d(2031, time.October, 1), 0.76},
		{d(2034, time.November, 1), 1.00},
	}}

	ticks := []Tick{
		{d(2026, time.September, 1), "26/9", "現在", "text_primary", false},
		{d(2027, time.October, 1), "27/10", "Loom 3", "conditional", false},
		{d(2028, time.August, 1), "28/8", "Loom 4(予)", "conditional", false},
		{d(2029, time.December, 1), "29/12", "Board A・Store X", "danger", false},
		{d(2031, time.February, 1), "31/2", "Board B(予)", "danger", false},
		{d(2031, time.June, 1), "31/6", "Loom 5(予)", "conditional", false},
		{d(2031, time.October, 1), "31/10", "OS 22", "text_primary", false},
		{d(2034, time.November, 1), "34/11", "OS 25", "support", false},
	}

	milestones := []Milestone{
		{d(2028, time.May, 1), "28/5 ライセンス更新", false},
		{d(2031, time.May, 1), "31/5 ライセンス更新", false},
	}

	const support, cond, none = "support", "conditional", "none"

	groups := []GanttGroup{
		{
			Label:       "第1世代\nBoard A\n120台\nStore X",
			HasRiskBand: true,
			RiskStart:   d(2029, time.December, 1),
			RiskEnd:     d(2031, time.February, 1),
			Rows: []GanttRow{
				{Label: "Loom 2", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2029, time.December, 1), none, "内部利用のみ、外部保守", false, false},
				}},
				{Label: "Loom 3", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2028, time.August, 1), support, "内部+外部利用", false, false},
					{d(2028, time.August, 1), d(2029, time.December, 1), cond, "延長要確認", false, false},
				}},
				{Label: "Loom 3/4/5", SubGroup: "定額版",
					HasNote: true, NoteStart: d(2027, time.October, 1), NoteEnd: d(2029, time.December, 1),
					NoteText: "Store XはLoom 4非対応のため移行先になり得ない"},
			},
		},
		{
			Label: "第2世代\nBoard B\n240台\nStore Y",
			Rows: []GanttRow{
				{Label: "Loom 2/3", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2031, time.June, 1), none, "内部利用のみ、外部保守", false, false},
				}},
				{Label: "(Loom 3)", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2029, time.December, 1), support, "内部+外部利用", false, false},
				}},
				{Label: "Loom 3/4/5", SubGroup: "定額版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2028, time.August, 1), support, "サポート対象", false, false},
					{d(2028, time.August, 1), d(2029, time.December, 1), cond, "延長要確認", false, false},
					{d(2029, time.December, 1), d(2031, time.June, 1), cond, "ベンダーのサポート可否は要協議", false, false},
				}},
				{Label: "Weft OS 22", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2034, time.November, 1), support, "", true, false},
				}},
			},
		},
		{
			Label: "第2世代\nBoard C\n80台\nStore Y",
			Rows: []GanttRow{
				{Label: "Loom 3→4→5", SubGroup: "定額版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2027, time.October, 1), cond, "Loom 3", false, false},
					{d(2027, time.October, 1), d(2028, time.August, 1), cond, "Loom 4", false, false},
					{d(2028, time.August, 1), d(2034, time.November, 1), support, "Loom 5", true, false},
				}},
				{Label: "Weft OS 22", Segments: []GanttSegment{
					{d(2027, time.October, 1), d(2034, time.November, 1), support, "", true, false},
				}},
			},
		},
	}

	annotations := []Annotation{
		{d(2027, time.October, 1), 0.13, "外部向け第2世代はセキュリティ要件で\nLoom 4化が必須(デモ)", "conditional", "up", false, -1, -1},
		{d(2029, time.December, 1), 0.0, "ここまでに全利用者を\nBoard B/Cへ移行予定(デモ)", "danger", "down", true, -1, -1},
		{d(2031, time.February, 1), 0.55, "保守不可ならBoard C移行が要るが、\n増設/移行の期間が不足(デモ)", "conditional", "up", false, -1, -1},
		{d(2031, time.June, 1), 0.55, fmt.Sprintf("ベンダー:27/6までLoom対応\n開発元:27/8にLoom 5公開予定(デモ)"), "conditional", "down", false, -1, -1},
	}

	return &GanttChart{
		Scale: scale, Ticks: ticks, Groups: groups,
		Legend: []LegendItem{
			{support, "ランタイム サポート有"},
			{none, "ランタイム サポート無"},
			{"risk", "ハードウェア サポート無"},
		},
		Milestones: milestones, Annotations: annotations,
		HasBreak: true, BreakBefore: d(2034, time.November, 1),
		HasExtNote: true, ExtNoteStart: d(2031, time.February, 1), ExtNoteYFrac: 0.27,
		ExtNoteLabel: "Board B保守延長予定(デモ)", ExtNoteRole: "danger",
	}
}
