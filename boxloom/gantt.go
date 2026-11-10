// Ganttチャート専用サブエンジン(box_engine.pyのcanvas葉ノードに差し込まれる、
// 座標スケール型の専用レンダラ)。外側のレイアウトエンジン(Taffy→自作Python→
// このGo版)を何度差し替えても、ここのロジックは一切変更していない ―
// それが二層アーキテクチャ(outer layout + inner canvas sub-engine)の狙い。
//
// 本物のPythonソース(layout_pptx_poc_v11.py の render_gantt()/build_chart())を
// 見ながら移植し直したバージョン。色は生の16進数ではなく、必ず意味役割名
// ("support"/"conditional"/"none"/"danger"/"risk"/"accent"/"on_conditional"等)
// 経由でThemeを引く ― build_chart()自体はThemeの存在を一切知らない。
package main

import (
	"fmt"
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
}

type Milestone struct {
	Date  time.Time
	Label string
}

type LegendItem struct {
	Role  string
	Label string
}

type Annotation struct {
	AnchorDate  time.Time
	AnchorYFrac float64
	Text        string
	Role        string
	Direction   string // "up" | "down"
	Emphasis    bool
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

// --- 定数(本物のGUTTER_*/LEGEND_H/...と1:1対応) ---

const (
	spaceMD = 16.0
	spaceXS = 4.0

	gutterGroupW = 70.0
	gutterSubW   = 46.0
	gutterLabelW = 118.0
	legendH      = 16.0
	milestoneH0  = 14.0
	axisLabelH   = 24.0
	groupGap     = 8.0
	minRowH      = 17.0

	annBoxW    = 176.0
	annBoxH    = 38.0
	annLaneGap = 8.0

	typeMicro = 9.0
	typeNano  = 8.0
)

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
}

// NewGanttSubsystem は登録簿の Factory。data は *GanttChart か GanttChart。
func NewGanttSubsystem(data any) (Subsystem, error) {
	switch v := data.(type) {
	case *GanttChart:
		return &GanttSubsystem{Chart: v}, nil
	case GanttChart:
		return &GanttSubsystem{Chart: &v}, nil
	}
	return nil, fmt.Errorf("gantt: expected *GanttChart, got %T", data)
}

func (g *GanttSubsystem) Name() string { return "gantt" }

func (g *GanttSubsystem) Render(ctx RenderContext, box LBox) {
	renderGantt(ctx.Out, box, g.Chart, ctx.Roles)
}

// 自己申告に使う大きさ。naturalRowH は行が詰まって見えない高さ、minRowH は
// 文字(typeMicro)とバーが重ならずに収まる下限。
const (
	naturalRowH   = 26.0
	naturalChartW = 560.0 // 日付軸の部分の自然な幅
	minChartW     = 240.0 // 日付軸の部分がこれより狭いと目盛りの文字が重なる
)

// Measure は Gantt の自然な大きさと最小の大きさを申告する。
//
// 高さ = 周りの固定の部品(凡例・注釈のレーン・節目・軸)+ 行の高さ×行数 +
// グループ間の隙間。行の高さは全グループで同じなので、グループ名(複数行)が
// 収まるだけの行の高さも下限に入れる。幅が分かっていれば幅はそのまま返す。
func (g *GanttSubsystem) Measure(c Constraint) Measurement {
	chart := g.Chart
	m := ganttMetricsFor(chart)
	totalRows, nGroups := 0, len(chart.Groups)
	labelRowH := 0.0 // グループ名を収めるのに要る、1行あたりの高さ
	for i := range chart.Groups {
		gr := &chart.Groups[i]
		totalRows += len(gr.Rows)
		if len(gr.Rows) == 0 {
			continue
		}
		lh := measureTextHeight(gr.Label, typeNano, gutterGroupW-10, true)
		if v := lh / float64(len(gr.Rows)); v > labelRowH {
			labelRowH = v
		}
	}
	gaps := groupGap * float64(max(0, nGroups-1))
	bodyFor := func(rowH float64) float64 {
		if rowH < labelRowH {
			rowH = labelRowH
		}
		return rowH*float64(totalRows) + gaps
	}

	gutterW := gutterGroupW + gutterSubW + gutterLabelW
	natW := 2*spaceMD + gutterW + naturalChartW
	if lw := 2*spaceMD + m.legendW; lw > natW {
		natW = lw
	}
	minW := 2*spaceMD + gutterW + minChartW
	if c.Width > 0 {
		natW = c.Width
	}
	return Measurement{
		Natural: Size{Width: natW, Height: m.chromeH + bodyFor(naturalRowH)},
		Min:     Size{Width: minW, Height: m.chromeH + bodyFor(minRowH)},
	}
}

// ganttMetrics は Measure と renderGantt が共有する、行の本体以外の寸法。
// 両方が同じ計算を使うので、申告した大きさと実際の描画がずれない。
type ganttMetrics struct {
	annTopH, annBottomH, milestoneH float64
	chromeH                         float64 // 行の本体以外の高さの合計(内側の余白を含む)
	legendW                         float64
}

func ganttMetricsFor(chart *GanttChart) ganttMetrics {
	var m ganttMetrics
	hasUp, hasDown := false, false
	for _, a := range chart.Annotations {
		if a.Direction == "up" {
			hasUp = true
		}
		if a.Direction == "down" {
			hasDown = true
		}
	}
	if hasUp {
		m.annTopH = annBoxH + annLaneGap
	}
	if hasDown {
		m.annBottomH = annBoxH + annLaneGap
	}
	if len(chart.Milestones) > 0 {
		m.milestoneH = milestoneH0 + 4
	}
	// renderGantt の縦の並び: 余白 / 凡例 / 上の注釈 / 節目 / 軸の文字 / 軸 / 本体 / 下の注釈 / 余白
	m.chromeH = 2*spaceMD + legendH + spaceXS + m.annTopH + m.milestoneH + axisLabelH + 5 + 5 + m.annBottomH
	for _, li := range chart.Legend {
		m.legendW += 19 + 6 + runWidthPt(li.Label, typeMicro, false) + spaceMD
	}
	return m
}

// renderGantt はbox(確定済みの絶対座標)の中にGanttチャート全体を描画する。
// 本物のrender_gantt()と同じ手順・同じピクセル計算式で移植している。
func renderGantt(p *PPTXBuilder, box LBox, chart *GanttChart, roles Roles) {
	c := roles.Color
	pad := spaceMD
	x0, y0 := box.X+pad, box.Y+pad
	w, h := box.Width-2*pad, box.Height-2*pad

	gm := ganttMetricsFor(chart)
	annTopH, annBottomH, milestoneH := gm.annTopH, gm.annBottomH, gm.milestoneH

	// --- 凡例(実測した文字幅でカーソルを前進させる。本物と同じ) ---
	lx := x0
	for _, li := range chart.Legend {
		p.AddRectFull(LBox{X: lx, Y: y0 + 2, Width: 15, Height: 7}, c(li.Role), 2, c("text_muted"), "", 0.5)
		labelW := runWidthPt(li.Label, typeMicro, false)
		p.AddTextBoxNoWrap(LBox{X: lx + 19, Y: y0 - 2, Width: labelW + 4, Height: legendH}, li.Label, typeMicro, c("text_muted"), false, "left", "top")
		lx += 19 + 6 + labelW + spaceMD
	}

	upY := y0 + legendH + spaceXS
	milestoneY := upY + annTopH
	axisLabelY := milestoneY + milestoneH
	axisY := axisLabelY + axisLabelH + 5
	gutterW := gutterGroupW + gutterSubW + gutterLabelW
	chartX0 := x0 + gutterW
	chartW := w - gutterW
	bodyTop := axisY + 5
	bodyH := (y0 + h) - annBottomH - bodyTop

	sx := func(dt time.Time) float64 { return chart.Scale.X(dt, chartX0, chartW) }

	// --- ★マイルストーン ---
	for _, m := range chart.Milestones {
		mx := sx(m.Date)
		p.AddTextBoxNoWrap(LBox{X: mx - 74, Y: milestoneY, Width: 148, Height: milestoneH0}, "★ "+m.Label, typeNano, c("accent"), true, "center", "middle")
	}

	// --- 軸目盛り ---
	for _, t := range chart.Ticks {
		tx := sx(t.Date)
		p.AddTextBoxNoWrap(LBox{X: tx - 32, Y: axisLabelY, Width: 64, Height: 13}, t.MainLabel, typeMicro, c(t.Role), true, "center", "top")
		if t.SubLabel != "" {
			p.AddTextBoxNoWrap(LBox{X: tx - 40, Y: axisLabelY + 12, Width: 80, Height: 12}, t.SubLabel, typeNano, c("text_muted"), false, "center", "top")
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
	totalRows := 0
	for i := range chart.Groups {
		totalRows += len(chart.Groups[i].Rows)
	}
	nGroups := len(chart.Groups)
	rowH := (bodyH - groupGap*float64(nGroups-1)) / float64(totalRows)
	if rowH < minRowH {
		rowH = minRowH
	}

	subX := x0 + gutterGroupW
	labelX := subX + gutterSubW

	yCursor := bodyTop
	for gi := range chart.Groups {
		group := &chart.Groups[gi]
		groupTop := yCursor
		groupH := rowH * float64(len(group.Rows))

		if group.HasRiskBand {
			rb0, rb1 := sx(group.RiskStart), sx(group.RiskEnd)
			p.AddRect(LBox{X: rb0, Y: groupTop, Width: rb1 - rb0, Height: groupH}, c("risk"), 0)
		}

		p.AddRect(LBox{X: x0, Y: groupTop, Width: gutterGroupW - 4, Height: groupH}, c("card_alt"), 6)
		p.AddTextBox(LBox{X: x0 + 3, Y: groupTop, Width: gutterGroupW - 10, Height: groupH}, group.Label, typeNano, c("text_primary"), true, "center", "middle")

		rowTops := make([]float64, len(group.Rows))
		for i := range group.Rows {
			rowTops[i] = groupTop + float64(i)*rowH
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
				subH := rowTops[j] + rowH - subTop
				p.AddTextBox(LBox{X: subX + 2, Y: subTop, Width: gutterSubW - 6, Height: subH}, sub, typeNano, c("text_muted"), false, "center", "middle")
			}
			i = j + 1
		}

		for idx := range group.Rows {
			row := &group.Rows[idx]
			rowTop := rowTops[idx]
			p.AddTextBoxNoWrap(LBox{X: labelX, Y: rowTop, Width: gutterLabelW - 8, Height: rowH}, row.Label, typeMicro, c("text_muted"), false, "left", "middle")

			barH := rowH - 6
			barY := rowTop + (rowH-barH)/2

			if row.HasNote {
				nx1, nx2 := sx(row.NoteStart), sx(row.NoteEnd)
				ww := nx2 - nx1
				if ww < 1.0 {
					ww = 1.0
				}
				p.AddRectFull(LBox{X: nx1, Y: barY, Width: ww, Height: barH}, "", 3, c("text_muted"), "dash", 0.75)
				p.AddTextBox(LBox{X: nx1 + 5, Y: barY, Width: ww - 10, Height: barH}, row.NoteText, typeNano, c("text_muted"), false, "left", "middle")
				continue
			}

			for _, seg := range row.Segments {
				bx1, bx2 := sx(seg.Start), sx(seg.End)
				ww := bx2 - bx1
				if ww < 1.0 {
					ww = 1.0
				}
				segColor := c(seg.Role)
				p.AddRect(LBox{X: bx1, Y: barY, Width: ww, Height: barH}, segColor, 3)
				if seg.Label != "" && (bx2-bx1) > 46 {
					labelColor := c("text_primary")
					if seg.Role == "conditional" {
						labelColor = c("on_conditional")
					}
					p.AddTextBoxNoWrap(LBox{X: bx1 + 5, Y: barY, Width: bx2 - bx1 - 8, Height: barH}, seg.Label, typeNano, labelColor, false, "left", "middle")
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
			p.AddLine(x0, yCursor+groupGap/2, x0+w, yCursor+groupGap/2, 0.75, c("grid"), "")
			yCursor += groupGap
		}
	}

	// --- 延伸ノート ---
	if chart.HasExtNote {
		ex0 := sx(chart.ExtNoteStart)
		ey := bodyTop + chart.ExtNoteYFrac*bodyH
		p.AddLine(ex0, ey, x0+w, ey, 1.25, c(chart.ExtNoteRole), "dash")
		p.AddTextBoxNoWrap(LBox{X: ex0, Y: ey - 15, Width: (x0 + w) - ex0, Height: 13}, chart.ExtNoteLabel, typeNano, c(chart.ExtNoteRole), true, "left", "top")
	}

	// --- 注釈(上下レーン、水平方向の重なりを動的に回避) ---
	downY := bodyTop + bodyH + annLaneGap
	var placed []placedBox
	for _, ann := range chart.Annotations {
		ax := sx(ann.AnchorDate)
		ay := bodyTop + ann.AnchorYFrac*bodyH
		annColor := c(ann.Role)

		if ann.Emphasis {
			p.AddLine(ax, bodyTop, ax, bodyTop+bodyH, 2.0, annColor, "")
		}
		p.AddDot(ax, ay, 3.0, annColor)

		bx := clamp(ax-annBoxW/2, x0, x0+w-annBoxW)
		by := downY
		if ann.Direction == "up" {
			by = upY
		}
		step := annBoxW + 8
		tries := 0
		for tries < 6 {
			conflict := false
			for _, pb := range placed {
				if rectsOverlap(placedBox{bx, by, annBoxW, annBoxH}, pb, 4.0) {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
			bx = clamp(bx+step, x0, x0+w-annBoxW)
			tries++
		}
		placed = append(placed, placedBox{bx, by, annBoxW, annBoxH})

		p.AddRectFull(LBox{X: bx, Y: by, Width: annBoxW, Height: annBoxH}, c("card_alt"), 8, annColor, "", 1.25)
		p.AddTextBox(LBox{X: bx + 8, Y: by + 4, Width: annBoxW - 16, Height: annBoxH - 8}, ann.Text, typeNano+0.5, c("text_primary"), false, "left", "top")
		connY := by
		if by < ay {
			connY = by + annBoxH
		}
		p.AddLine(ax, ay, bx+annBoxW/2, connY, 1.0, annColor, "dash")
	}
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
		{d(2026, time.September, 1), "26/9", "現在", "text_primary"},
		{d(2027, time.October, 1), "27/10", "Loom 3", "conditional"},
		{d(2028, time.August, 1), "28/8", "Loom 4(予)", "conditional"},
		{d(2029, time.December, 1), "29/12", "Board A・Store X", "danger"},
		{d(2031, time.February, 1), "31/2", "Board B(予)", "danger"},
		{d(2031, time.June, 1), "31/6", "Loom 5(予)", "conditional"},
		{d(2031, time.October, 1), "31/10", "OS 22", "text_primary"},
		{d(2034, time.November, 1), "34/11", "OS 25", "support"},
	}

	milestones := []Milestone{
		{d(2028, time.May, 1), "28/5 ライセンス更新"},
		{d(2031, time.May, 1), "31/5 ライセンス更新"},
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
					{d(2026, time.September, 1), d(2029, time.December, 1), none, "内部利用のみ、外部保守", false},
				}},
				{Label: "Loom 3", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2028, time.August, 1), support, "内部+外部利用", false},
					{d(2028, time.August, 1), d(2029, time.December, 1), cond, "延長要確認", false},
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
					{d(2026, time.September, 1), d(2031, time.June, 1), none, "内部利用のみ、外部保守", false},
				}},
				{Label: "(Loom 3)", SubGroup: "固定版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2029, time.December, 1), support, "内部+外部利用", false},
				}},
				{Label: "Loom 3/4/5", SubGroup: "定額版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2028, time.August, 1), support, "サポート対象", false},
					{d(2028, time.August, 1), d(2029, time.December, 1), cond, "延長要確認", false},
					{d(2029, time.December, 1), d(2031, time.June, 1), cond, "ベンダーのサポート可否は要協議", false},
				}},
				{Label: "Weft OS 22", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2034, time.November, 1), support, "", true},
				}},
			},
		},
		{
			Label: "第2世代\nBoard C\n80台\nStore Y",
			Rows: []GanttRow{
				{Label: "Loom 3→4→5", SubGroup: "定額版", Segments: []GanttSegment{
					{d(2026, time.September, 1), d(2027, time.October, 1), cond, "Loom 3", false},
					{d(2027, time.October, 1), d(2028, time.August, 1), cond, "Loom 4", false},
					{d(2028, time.August, 1), d(2034, time.November, 1), support, "Loom 5", true},
				}},
				{Label: "Weft OS 22", Segments: []GanttSegment{
					{d(2027, time.October, 1), d(2034, time.November, 1), support, "", true},
				}},
			},
		},
	}

	annotations := []Annotation{
		{d(2027, time.October, 1), 0.13, "外部向け第2世代はセキュリティ要件で\nLoom 4化が必須(デモ)", "conditional", "up", false},
		{d(2029, time.December, 1), 0.0, "ここまでに全利用者を\nBoard B/Cへ移行予定(デモ)", "danger", "down", true},
		{d(2031, time.February, 1), 0.55, "保守不可ならBoard C移行が要るが、\n増設/移行の期間が不足(デモ)", "conditional", "up", false},
		{d(2031, time.June, 1), 0.55, fmt.Sprintf("ベンダー:27/6までLoom対応\n開発元:27/8にLoom 5公開予定(デモ)"), "conditional", "down", false},
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
