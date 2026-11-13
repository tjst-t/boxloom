package boxloom

import (
	"testing"
)

// 狭い枠の3段の組織図(#134。再現の第6回の p33:2単位の組織図を幅約 420pt の列に置くと
// 横に並べた3段目が狭く、Min が枠より高くなった)は、3段目を親の下に字下げして縦に積む形に
// 切り替え、最小の高さを下げる。描いた図形は枠に収まり、箱は重ならず、文は収まる。
// 広い枠では横に並べる形のまま。
func TestOrgChartNarrowStacksMembers(t *testing.T) {
	data := `{"root": {"title": "推進の責任者", "detail": "架空 花子", "emphasis": true},
	  "units": [
	    {"title": "基盤のチーム", "detail": "架空 太郎", "members": [
	      {"title": "認証の基盤の整備", "detail": "架空 一郎"}, {"title": "データの移行の計画", "detail": "架空 二郎"},
	      {"title": "監視と運用の設計", "detail": "架空 三郎"}, {"title": "性能の試験と改善", "detail": "架空 四郎"}]},
	    {"title": "導入のチーム", "detail": "架空 次郎", "members": [
	      {"title": "現場への説明会の開催", "detail": "架空 五郎", "status": "conditional"}, {"title": "問い合わせ窓口の運営", "detail": "架空 六郎"},
	      {"title": "手順書と動画の整備", "detail": "架空 七郎"}, {"title": "利用の定着の支援", "detail": "架空 八郎"}]}],
	  "legend": {"conditional": "2名を増やす"}}`
	for _, tmpl := range []string{"light", "dark", "report"} {
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		sub := figureFromIR(t, tmpl, "orgchart", data).(*OrgChartSubsystem)
		const narrow = 420.0
		rowMin, _, _ := flexTotals(sub.geomRow(narrow).parts)
		g := sub.geom(narrow)
		if !g.stacked {
			t.Errorf("%s: %.0fpt should stack the members (row min %.1fpt)", tmpl, narrow, rowMin)
		}
		m := sub.Measure(Constraint{Width: narrow})
		if m.Min.Height >= rowMin {
			t.Errorf("%s: stacked min %.1fpt is not below the row form's %.1fpt", tmpl, m.Min.Height, rowMin)
		}
		t.Logf("%s: min %.1fpt (row form %.1fpt)", tmpl, m.Min.Height, rowMin)
		// 広い枠(本文の幅いっぱい)では横に並べる形のまま。
		if sub.geom(912).stacked {
			t.Errorf("%s: 912pt should keep the members in a row", tmpl)
		}
		for _, h := range []float64{m.Min.Height, m.Natural.Height, m.Max.Height + 40} {
			box := LBox{X: 30, Y: 20, Width: narrow, Height: h}
			pl := sub.place(box)
			all := []LBox{pl.root}
			all = append(all, pl.units...)
			for _, ms := range pl.members {
				all = append(all, ms...)
			}
			for i, a := range all {
				if !within(box, a) {
					t.Errorf("%s h=%.0f: box %d %+v outside %+v", tmpl, h, i, a, box)
				}
				for j := 0; j < i; j++ {
					if boxesOverlap(a, all[j]) {
						t.Errorf("%s h=%.0f: boxes %d and %d overlap", tmpl, h, i, j)
					}
				}
			}
			if ws := sub.Lint(box); len(ws) != 0 {
				t.Errorf("%s h=%.0f: warnings %v", tmpl, h, ws)
			}
			// 描いた図形(線も)が枠の中。3段目への線は箱の左の辺に繋がる。
			p := NewPPTXBuilder(960, 540)
			sub.Render(RenderContext{Out: p, Roles: tm.Theme}, box)
			glued := 0
			for _, s := range p.Shapes {
				var b LBox
				if s.kind == skLine || s.kind == skConnector {
					b = LBox{X: min(s.x1, s.x2), Y: min(s.y1, s.y2), Width: abs(s.x2 - s.x1), Height: abs(s.y2 - s.y1)}
					if s.kind == skConnector && s.stID == 0 && s.endID > 0 && s.endIdx == 1 {
						glued++
					}
				} else {
					b = s.box
				}
				if !within(box, b) {
					t.Errorf("%s h=%.0f: shape %d %+v outside %+v", tmpl, h, s.id, b, box)
				}
			}
			if glued != 8 {
				t.Errorf("%s h=%.0f: %d member lines glued to the left side, want 8", tmpl, h, glued)
			}
		}
		restore()
	}
}

// 列の中(幅約 420pt)の3段の組織図:縦に積む形の Min で枠を取り、subsystem-min の
// 警告(最小を割る)を出さない。
func TestOrgChartNarrowInColumn(t *testing.T) {
	src := "---\ntitle: 再現\n---\n\n# 体制\n\n::::columns\n:::column 体制\n```orgchart\n" +
		"root: {title: 推進の責任者, detail: 架空 花子}\nunits:\n" +
		"  - title: 基盤のチーム\n    detail: 架空 太郎\n    members:\n      - {title: 認証の基盤の整備, detail: 架空 一郎}\n      - {title: データの移行の計画, detail: 架空 二郎}\n      - {title: 監視と運用の設計, detail: 架空 三郎}\n      - {title: 性能の試験と改善, detail: 架空 四郎}\n" +
		"  - title: 導入のチーム\n    detail: 架空 次郎\n    members:\n      - {title: 現場への説明会の開催, detail: 架空 五郎}\n      - {title: 問い合わせ窓口の運営, detail: 架空 六郎}\n      - {title: 手順書と動画の整備, detail: 架空 七郎}\n      - {title: 利用の定着の支援, detail: 架空 八郎}\n" +
		"```\n:::\n:::column 補足\n体制の補足を短く書く。\n:::\n::::\n"
	for _, name := range []string{"dark", "light", "report"} {
		tm := mustTemplate(name)
		built := buildLaidOut(t, tm, "narrow.bx.md", []byte(src))
		var found bool
		for _, b := range built {
			if b.Body == nil {
				continue
			}
			var walk func(s *Spec)
			walk = func(s *Spec) {
				if s.Kind == KindSubsystem {
					if o, ok := s.Subsystem.(*OrgChartSubsystem); ok {
						found = true
						if ws := lintSubsystemMin(s); len(ws) != 0 {
							t.Errorf("%s: %v", name, ws)
						}
						if !o.geom(s.Box.Width).stacked && s.Box.Width < 480 {
							t.Errorf("%s: %.0fpt column keeps the members in a row", name, s.Box.Width)
						}
						t.Logf("%s: orgchart %.0f x %.0f", name, s.Box.Width, s.Box.Height)
					}
				}
				for _, c := range s.Children {
					walk(c)
				}
			}
			walk(b.Body)
		}
		if !found {
			t.Errorf("%s: no orgchart", name)
		}
	}
}
