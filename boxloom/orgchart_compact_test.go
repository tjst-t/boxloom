package boxloom

import (
	"math"
	"testing"
)

// 再現の第9回の p33 に近い、孫3つずつの3段の組織図(#153)。
const orgCompactData = `{"root": {"title": "推進の責任者", "detail": "架空 花子", "emphasis": true},
  "units": [
    {"title": "基盤のチーム", "detail": "架空 太郎", "members": [
      {"title": "認証基盤", "detail": "架空 一郎"}, {"title": "移行計画", "detail": "架空 二郎"}, {"title": "監視運用", "detail": "架空 三郎"}]},
    {"title": "導入のチーム", "detail": "架空 次郎", "members": [
      {"title": "説明会", "detail": "架空 五郎"}, {"title": "窓口運営", "detail": "架空 六郎"}, {"title": "定着支援", "detail": "架空 八郎"}]}]}`

// 狭い列(340pt)の3段の組織図:字を下げずに横に並べる形より縦に積む形が低い幅で、
// 3段目の字を値の表の段で下げて横に並べる形のほうが低ければ、それを選ぶ(#153)。
// Measure の Min は選んだ形の Min で、描いた3段目の箱は下げた字、枠に収まり警告も無い。
// 広い枠(912pt)では字を下げない横に並べる形のまま。
func TestOrgChartCompactRow(t *testing.T) {
	const narrow = 340.0
	for _, tmpl := range []string{"light", "dark", "report"} {
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		sub := figureFromIR(t, tmpl, "orgchart", orgCompactData).(*OrgChartSubsystem)
		row0, st := orgMinHeight(sub.geomRow(narrow, 0)), orgMinHeight(sub.geomStacked(narrow))
		if !(st < row0-0.5) {
			t.Fatalf("%s: the case needs the stacked form (%.1f) below the plain row (%.1f)", tmpl, st, row0)
		}
		g := sub.geom(narrow)
		if g.stacked || g.memStep == 0 {
			t.Fatalf("%s: %.0fpt chose stacked=%v step=%d, want the compact row", tmpl, narrow, g.stacked, g.memStep)
		}
		tx := sub.text()
		if !(g.memSub < tx.Sub) || !onScale(g.memSub, tm.Scale.TypeSizes) || !onScale(g.memDetail, tm.Scale.TypeSizes) {
			t.Errorf("%s: member sizes %v/%v (from %v/%v) not lowered on the scale %v", tmpl, g.memSub, g.memDetail, tx.Sub, tx.Detail, tm.Scale.TypeSizes)
		}
		m := sub.Measure(Constraint{Width: narrow})
		if math.Abs(m.Min.Height-orgMinHeight(g)) > 0.01 || !(m.Min.Height < st-0.5) {
			t.Errorf("%s: Measure min %.1f, compact %.1f, stacked %.1f", tmpl, m.Min.Height, orgMinHeight(g), st)
		}
		t.Logf("%s: compact min %.1fpt at %v/%vpt (plain row %.1f, stacked %.1f)", tmpl, m.Min.Height, g.memSub, g.memDetail, row0, st)
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
							if s.fontSize != g.memSub {
								t.Errorf("%s: member drawn at %vpt, measured at %vpt", tmpl, s.fontSize, g.memSub)
							}
						}
					}
				}
			}
			if members != 6 {
				t.Errorf("%s h=%.0f: %d member boxes drawn, want 6", tmpl, h, members)
			}
		}
		if w := sub.geom(912); w.stacked || w.memStep != 0 {
			t.Errorf("%s: 912pt should keep the plain row (stacked=%v step=%d)", tmpl, w.stacked, w.memStep)
		}
		restore()
	}
}

func onScale(x float64, sizes []float64) bool {
	for _, s := range sizes {
		if math.Abs(s-x) < scaleEps {
			return true
		}
	}
	return false
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
