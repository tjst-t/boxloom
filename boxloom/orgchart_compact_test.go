package boxloom

import (
	"math"
	"testing"
)

// 孫3つずつの3段の組織図(#153・#157)。名前はすべて架空。
const orgCompactData = `{"root": {"title": "推進の責任者", "detail": "架空 花子", "emphasis": true},
  "units": [
    {"title": "基盤のチーム", "detail": "架空 太郎", "members": [
      {"title": "認証基盤", "detail": "架空 一郎"}, {"title": "移行計画", "detail": "架空 二郎"}, {"title": "監視運用", "detail": "架空 三郎"}]},
    {"title": "導入のチーム", "detail": "架空 次郎", "members": [
      {"title": "説明会", "detail": "架空 五郎"}, {"title": "窓口運営", "detail": "架空 六郎"}, {"title": "定着支援", "detail": "架空 八郎"}]}]}`

// -fit なしでは、狭い枠(340pt)でも組織図の3段目の字は値の表の subPoint・figureDetail の
// まま(#157。#153 の字を下げて横に並べる形は外した)。幅が足りないときは縦に積む形(#134)を
// 選び、描いた図形は枠に収まり警告も無い。広い枠(912pt)では横に並べる形のまま。
func TestOrgChartCompactRow(t *testing.T) {
	const narrow = 340.0
	for _, tmpl := range []string{"light", "dark", "report"} {
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		sub := figureFromIR(t, tmpl, "orgchart", orgCompactData).(*OrgChartSubsystem)
		row, st := orgMinHeight(sub.geomRow(narrow)), orgMinHeight(sub.geomStacked(narrow))
		if !(st < row-0.5) {
			t.Fatalf("%s: the case needs the stacked form (%.1f) below the row (%.1f)", tmpl, st, row)
		}
		g := sub.geom(narrow)
		if !g.stacked {
			t.Fatalf("%s: %.0fpt kept the row, want the stacked form", tmpl, narrow)
		}
		tx := sub.text()
		if tx.Sub != tm.Values.SubPoint || tx.Detail != tm.Values.FigureDetail {
			t.Fatalf("%s: text %v/%v, want the values %v/%v", tmpl, tx.Sub, tx.Detail, tm.Values.SubPoint, tm.Values.FigureDetail)
		}
		m := sub.Measure(Constraint{Width: narrow})
		if math.Abs(m.Min.Height-st) > 0.01 {
			t.Errorf("%s: Measure min %.1f, stacked %.1f", tmpl, m.Min.Height, st)
		}
		for _, h := range []float64{m.Min.Height, m.Natural.Height} {
			box := LBox{X: 30, Y: 20, Width: narrow, Height: h}
			if ws := sub.Lint(box); len(ws) != 0 {
				t.Errorf("%s h=%.0f: warnings %v", tmpl, h, ws)
			}
			pl := sub.place(box)
			p := NewPPTXBuilder(960, 540)
			sub.Render(RenderContext{Out: p, Roles: tm.Theme}, box)
			members := 0
			for _, s := range p.Shapes {
				if s.kind != skLine && s.kind != skConnector && !within(box, s.box) {
					t.Errorf("%s h=%.0f: shape %+v outside %+v", tmpl, h, s.box, box)
				}
				for _, ms := range pl.members {
					for _, mb := range ms {
						if s.box == mb && s.kind != skLine && s.kind != skConnector {
							members++
							if s.fontSize != tm.Values.SubPoint {
								t.Errorf("%s: member drawn at %vpt, want subPoint %vpt", tmpl, s.fontSize, tm.Values.SubPoint)
							}
						}
					}
				}
			}
			if members != 6 {
				t.Errorf("%s h=%.0f: %d member boxes drawn, want 6", tmpl, h, members)
			}
		}
		if w := sub.geom(912); w.stacked {
			t.Errorf("%s: 912pt should keep the row", tmpl)
		}
		restore()
	}
}

// fit の段(shrink・emergency)は組織図の箱の高さの下限・段の間・3段目の箱の間・余白も
// 下げるので、同じ幅の組織図の Min は dense → shrink → emergency で下がる(#153)。
func TestOrgChartFitLowersMin(t *testing.T) {
	for _, tmpl := range []string{"light", "dark", "report"} {
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		steps := []TemplateValues{tm.Dense.apply(tm.Values)}
		names := []string{"dense"}
		for i, f := range tm.Fit.Shrink {
			steps, names = append(steps, f.apply(tm.Values)), append(names, "shrink"+string(rune('0'+i)))
		}
		for i, f := range tm.Fit.Emergency {
			steps, names = append(steps, f.apply(tm.Values)), append(names, "emergency"+string(rune('0'+i)))
		}
		prev, prevV := math.Inf(1), (*OrgChartValues)(nil)
		for i, v := range steps {
			sub := figureFromIR(t, tmpl, "orgchart", orgCompactData).(*OrgChartSubsystem)
			setTextSizes(sub, v, tm.Marks.Point)
			ov := sub.vals()
			if ov.MinBoxH != v.fig.OrgChartMinBoxH || ov.TextInsetX != v.fig.OrgChartTextInsetX {
				t.Errorf("%s %s: values not applied: %+v", tmpl, names[i], ov)
			}
			if prevV != nil && (ov.MinBoxH > prevV.MinBoxH || ov.MinLevelGap > prevV.MinLevelGap || ov.MemberGap > prevV.MemberGap ||
				ov.TextInsetX > prevV.TextInsetX || ov.TextInsetY > prevV.TextInsetY) {
				t.Errorf("%s %s: orgchart sizes grew from the step before", tmpl, names[i])
			}
			min := sub.Measure(Constraint{Width: 600}).Min.Height
			t.Logf("%s %s: min %.1fpt (box %v level %v member gap %v inset %v/%v)", tmpl, names[i], min,
				ov.MinBoxH, ov.MinLevelGap, ov.MemberGap, ov.TextInsetX, ov.TextInsetY)
			if !(min < prev-0.5) {
				t.Errorf("%s %s: min %.1fpt does not drop below the step before (%.1fpt)", tmpl, names[i], min, prev)
			}
			prev, prevV = min, ov
		}
		restore()
	}
}
