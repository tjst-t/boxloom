package boxloom

import (
	"strings"
	"testing"
)

// 図の中の収まり(#106)。文はすべて架空。

const fitCycleData = `{"steps": [{"title": "地域の担い手を育てる"}, {"title": "週末の拠点を開く"}, {"title": "小さく試す"},
  {"title": "運営の決まりを作る"}, {"title": "ふり返りを共有する"}, {"title": "次の年へ渡す"}]}`

// cycle: 長い題は、扇の中の枠が折れない塊(和文の4字)より狭くならず、1〜2字ずつに
// 折れない。題が自然の輪の扇に入らなければ、自然の大きさを大きく申告する。小さい輪で
// 扇に入らない題は警告になる。
func TestCycleLongTitlesDoNotBreakPerChar(t *testing.T) {
	for _, tm := range []string{"report", "light", "dark"} {
		sub := figureFromIR(t, tm, "cycle", fitCycleData).(*CycleSubsystem)
		v, tx := sub.vals(), sub.text()
		m := sub.Measure(Constraint{})
		if base, _ := sub.size(v.NaturalDiameter/2, sub.labelSizes(v.LabelMaxWidth)); tm == "light" && !(m.Natural.Width > base) {
			// light は自然の輪の扇に長い題が入らない。
			t.Errorf("%s: natural size %.1f did not grow for long titles", tm, m.Natural.Width)
		}
		box := LBox{Width: m.Natural.Width, Height: m.Natural.Height}
		pl := sub.place(box)
		for i := range sub.Data.Steps {
			runs := sub.titleRuns(&sub.Data.Steps[i], "", nil)
			tb := pl.titles[i]
			if tb.Width < runsChunkWidth(runs, tx.Sub)-1e-6 {
				t.Errorf("%s: steps[%d] title box %.1f is narrower than a 4-char chunk", tm, i, tb.Width)
			}
			lines := wrapRuns(runs, tx.Sub, tb.Width)
			for k, l := range lines[:len(lines)-1] {
				if n := len([]rune(runsText(l))); n < tableCJKChunk {
					t.Errorf("%s: steps[%d] line %d %q has %d chars", tm, i, k, runsText(l), n)
				}
			}
			if r := pl.titleRoom[i]; r.Width > 0 {
				t.Errorf("%s: steps[%d] title does not fit the sector at the natural size", tm, i)
			}
		}
		if ws := sub.Lint(box); len(ws) != 0 {
			t.Errorf("%s: natural size warns %v", tm, ws)
		}
		// 最小の輪では入らない題があり、警告になる(題は塊の幅のまま)。
		small := LBox{Width: m.Min.Width, Height: m.Min.Height}
		ws := sub.Lint(small)
		if len(ws) == 0 || !strings.Contains(strings.Join(ws, "\n"), "cycle: steps[") {
			t.Errorf("%s: min size gives no title warning: %v", tm, ws)
		}
		for i, tb := range sub.place(small).titles {
			if tb.Width < runsChunkWidth(sub.titleRuns(&sub.Data.Steps[i], "", nil), tx.Sub)-1e-6 {
				t.Errorf("%s: min size: steps[%d] title box %.1f is narrower than a chunk", tm, i, tb.Width)
			}
		}
	}
}

// phases: 枠が最小より低くても、箱・山形・群の帯は図の矩形の外に描かない(はみ出しは
// layout-overflow と箱の警告で知らせる)。最小の幅は箱の一番長い語を割らない幅を含む。
func TestPhasesBoxesStayInFrame(t *testing.T) {
	var items []string
	for i := 0; i < 5; i++ {
		items = append(items, `{"title": "地域の拠点の運営", "detail": ["週末の催しを開く", "担い手を三人育てる", "費用の見通しを立てる"]}`)
	}
	sub := figureFromIR(t, "report", "phases", `{"steps": [
	  {"title": "調べる", "id": "a", "items": [`+strings.Join(items, ",")+`]},
	  {"title": "試す", "id": "b", "items": [{"title": "小さく試す"}]},
	  {"title": "広げる", "id": "c", "items": [{"title": "Infrastructure-Operations"}]}],
	  "groups": [{"title": "一年目", "from": "a", "to": "b"}]}`).(*PhasesSubsystem)
	m := sub.Measure(Constraint{Width: 860})
	for _, h := range []float64{m.Min.Height * 0.5, m.Min.Height * 0.8, m.Min.Height, m.Natural.Height} {
		box := LBox{X: 50, Y: 120, Width: 860, Height: h}
		_, pl := sub.place(box)
		var all []LBox
		all = append(all, pl.bands...)
		all = append(all, pl.chevrons...)
		for _, col := range pl.items {
			all = append(all, col...)
		}
		for _, b := range all {
			if !within(box, b) {
				t.Errorf("h=%.1f (min %.1f): %+v is outside the frame %+v", h, m.Min.Height, b, box)
			}
		}
		if h < m.Min.Height-1 && len(sub.Lint(box)) == 0 {
			t.Errorf("h=%.1f: a frame below the minimum gives no warning", h)
		}
	}
	// 最小の幅: どの箱も一番長い語を割らずに置ける。
	v := sub.vals()
	n := float64(len(sub.Data.Steps))
	colW := (m.Min.Width - 2*v.Padding - (n-1)*v.ColumnGap) / n
	need, _ := sub.itemNeed(7, &sub.Data.Steps[2].Items[0], colW)
	if colW+1e-6 < need {
		t.Errorf("min width %.1f gives columns of %.1f, the long word needs %.1f", m.Min.Width, colW, need)
	}
}

// matrix: 行の題の列は題と説明の自然な幅から取る(印だけの列は印の幅で足りる)。
// 前の決め方(内側の幅の 0.35 まで)より広く、題は1行に収まる。
func TestMatrixRowHeadTakesNaturalWidth(t *testing.T) {
	sub := figureFromIR(t, "report", "matrix", `{"columns": ["案 A", "案 B", "案 C", "案 D"], "rows": [
	  {"label": "地域の担い手を育てる仕組み", "detail": ["週末に開く拠点の運営を任せられる人を育てる", "講座の修了者を次の講師にする"],
	   "cells": [{"status": "support"}, {"status": "conditional"}, {"status": "none"}, {"status": "support"}]},
	  {"label": "費用の見通し", "detail": "初年度の費用と三年目からの運営費を分けて見積もる",
	   "cells": [{"status": "none"}, {"status": "support"}, {"status": "conditional"}, {"status": "support"}]}]}`).(*MatrixSubsystem)
	v, tx := sub.vals(), sub.text()
	const width = 860.0
	g := sub.geom(width)
	old := g.inner * 0.35
	if !(g.headW > old+1) {
		t.Errorf("row head column %.1f is not wider than before (%.1f)", g.headW, old)
	}
	if g.headW > g.inner*v.RowHeaderFrac+1e-6 {
		t.Errorf("row head column %.1f exceeds maxRowHeaderFraction", g.headW)
	}
	if g.colW < v.MarkSize+2*v.TextInsetX {
		t.Errorf("columns %.1f are narrower than the mark", g.colW)
	}
	for i := range sub.Data.Rows {
		label, detail := sub.rowHeadParas(&sub.Data.Rows[i], "", "", nil)
		if n := len(wrapRuns(label, tx.Title, g.headW-2*v.TextInsetX)); n != 1 {
			t.Errorf("rows[%d] label wraps into %d lines", i, n)
		}
		for _, p := range detail {
			if n := len(wrapRuns(p, tx.Detail, g.headW-2*v.TextInsetX)); n != 1 {
				t.Errorf("rows[%d] detail %q wraps into %d lines", i, runsText(p), n)
			}
		}
	}
	if ws := sub.Lint(LBox{Width: width, Height: sub.Measure(Constraint{Width: width}).Natural.Height}); len(ws) != 0 {
		t.Errorf("warnings %v", ws)
	}
}
