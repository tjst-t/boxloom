package boxloom

import (
	"fmt"
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
		// 最小の大きさ(Min)では題が読める(扇に入るか輪の外。#112)。
		if ws := sub.Lint(LBox{Width: m.Min.Width, Height: sub.Measure(Constraint{Width: m.Min.Width}).Natural.Height}); len(ws) != 0 {
			t.Errorf("%s: min size warns %v", tm, ws)
		}
		// それより小さい最小の輪では入らない題があり、警告になる(題は塊の幅のまま)。
		sw, sh := sub.size(v.MinDiameter/2, sub.labelSizes(v.LabelMinWidth))
		small := LBox{Width: sw, Height: sh}
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

// phases: 枠が最小より低くても、箱は文の要る高さより低くしない(#107)。箱どうし・
// 箱の文の矩形どうしは重ならず、山形・群の帯は枠の中。足りない分は図の下へはみ出し、
// layout-overflow・subsystem-min で知らせる。最小の幅は箱の一番長い語を割らない幅を含む。
func TestPhasesBoxesNeverCrushed(t *testing.T) {
	var items []string
	for i := 0; i < 5; i++ {
		items = append(items, `{"title": "地域の拠点の運営", "detail": ["週末の催しを開く", "担い手を三人育てる", "費用の見通しを立てる"]}`)
	}
	sub := figureFromIR(t, "report", "phases", `{"steps": [
	  {"title": "調べる", "id": "a", "items": [`+strings.Join(items, ",")+`]},
	  {"title": "試す", "id": "b", "items": [{"title": "小さく試す"}]},
	  {"title": "広げる", "id": "c", "items": [{"title": "Infrastructure-Operations"}]}],
	  "groups": [{"title": "一年目", "from": "a", "to": "b"}]}`).(*PhasesSubsystem)
	v := sub.vals()
	m := sub.Measure(Constraint{Width: 860})
	for _, h := range []float64{m.Min.Height * 0.5, m.Min.Height * 0.8, m.Min.Height, m.Natural.Height} {
		box := LBox{X: 50, Y: 120, Width: 860, Height: h}
		_, pl := sub.place(box)
		var heads []LBox
		heads = append(heads, pl.bands...)
		heads = append(heads, pl.chevrons...)
		for _, b := range heads {
			if !within(box, b) {
				t.Errorf("h=%.1f (min %.1f): %+v is outside the frame %+v", h, m.Min.Height, b, box)
			}
		}
		num := 0
		for i, col := range pl.items {
			var texts []LBox
			for k, b := range col {
				num++
				_, needH := sub.itemNeed(num, &sub.Data.Steps[i].Items[k], b.Width)
				if b.Height+1e-6 < needH {
					t.Errorf("h=%.1f: steps[%d].items[%d] is %.1fpt, its text needs %.1fpt", h, i, k, b.Height, needH)
				}
				// 文の矩形(内側の余白の中、文の要る高さ)
				texts = append(texts, LBox{X: b.X + v.TextInsetX, Y: b.Y + v.TextInsetY, Width: b.Width - 2*v.TextInsetX, Height: needH - 2*v.TextInsetY})
			}
			for a := range texts {
				for b := a + 1; b < len(texts); b++ {
					if boxesOverlap(texts[a], texts[b]) {
						t.Errorf("h=%.1f: steps[%d] text of items[%d] overlaps items[%d]", h, i, a, b)
					}
				}
			}
		}
		if ws := sub.Lint(box); len(ws) != 0 {
			t.Errorf("h=%.1f: boxes keep their text, but Lint warns %v", h, ws)
		}
		if ws := lintSubsystemMin(&Spec{Kind: KindSubsystem, Subsystem: sub, Box: box}); (h < m.Min.Height-1) != (len(ws) > 0) {
			t.Errorf("h=%.1f (min %.1f): subsystem-min warnings %v", h, m.Min.Height, ws)
		}
	}
	// 最小の幅: どの箱も一番長い語を割らずに置ける。
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

// phases の箱の中身が本文に収まらないとき、check は警告を1件以上出し、箱の文は
// 収まったまま(figure-text-overflow は出さない、#107)。
func TestPhasesTooTallWarns(t *testing.T) {
	var b strings.Builder
	b.WriteString("---\ntitle: 架空の計画\ntemplate: report\n---\n\n# 進め方\n\n```phases\nsteps:\n")
	for i, st := range []string{"調べる", "試す", "広げる"} {
		fmt.Fprintf(&b, "  - title: %s\n    items:\n", st)
		for k := 0; k < 6-2*i; k++ {
			b.WriteString("      - title: 地域の拠点の運営\n        detail:\n          - 週末の催しを開く\n          - 担い手を三人育てる\n          - 費用の見通しを立てる\n")
		}
	}
	b.WriteString("```\n")
	res := Check("plan.bx.md", []byte(b.String()), CheckOptions{})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %v", res.Errors)
	}
	var codes []string
	for _, w := range res.Warnings {
		codes = append(codes, w.Code)
		if w.Code == "figure-text-overflow" {
			t.Errorf("box text crushed: %v", w)
		}
	}
	if !strings.Contains(strings.Join(codes, " "), "layout-overflow") && !strings.Contains(strings.Join(codes, " "), "subsystem-min") {
		t.Errorf("no layout-overflow / subsystem-min warning: %v", res.Warnings)
	}
}

// phases: 枠が最小より低いとき、凡例は図の頭に移り、枠の中に残る。群の帯・山形・箱と
// 重ならない(#114)。枠が足りていれば凡例は箱の列の下のまま。
func TestPhasesLegendStaysInFrame(t *testing.T) {
	var items []string
	for i := 0; i < 4; i++ {
		items = append(items, `{"title": "地域の拠点の運営", "status": "conditional", "detail": ["週末の催しを開く", "担い手を三人育てる"]}`)
	}
	for _, tm := range []string{"report", "light", "dark"} {
		sub := figureFromIR(t, tm, "phases", `{"steps": [
		  {"title": "調べる", "id": "a", "items": [`+strings.Join(items, ",")+`]},
		  {"title": "試す", "id": "b", "items": [{"title": "小さく試す", "status": "danger"}]},
		  {"title": "広げる", "id": "c", "items": [{"title": "仲間を増やす"}]}],
		  "groups": [{"title": "一年目", "from": "a", "to": "b"}],
		  "legend": {"conditional": "条件つきで進める", "danger": "遅れの見込み"}}`).(*PhasesSubsystem)
		m := sub.Measure(Constraint{Width: 860})
		for _, h := range []float64{m.Min.Height * 0.87, m.Min.Height * 0.6, m.Min.Height, m.Natural.Height} {
			box := LBox{X: 50, Y: 120, Width: 860, Height: h}
			_, pl := sub.place(box)
			if pl.legend.Height <= 0 {
				t.Fatalf("%s: no legend", tm)
			}
			if !within(box, pl.legend) {
				t.Errorf("%s h=%.1f (min %.1f): legend %+v is outside the frame %+v", tm, h, m.Min.Height, pl.legend, box)
			}
			others := append(append([]LBox{}, pl.bands...), pl.chevrons...)
			for _, col := range pl.items {
				others = append(others, col...)
			}
			for _, b := range others {
				if boxesOverlap(pl.legend, b) {
					t.Errorf("%s h=%.1f: legend %+v overlaps %+v", tm, h, pl.legend, b)
				}
			}
			if top := pl.legend.Y < pl.chevrons[0].Y; top != (h < m.Min.Height-1) {
				t.Errorf("%s h=%.1f (min %.1f): legend at top = %v", tm, h, m.Min.Height, top)
			}
		}
	}
}
