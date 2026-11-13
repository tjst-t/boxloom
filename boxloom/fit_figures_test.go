package boxloom

import (
	"fmt"
	"strings"
	"testing"
)

// fitFigureDecks は図だけで本文の箱からはみ出すスライド(#139 の再現。matrix・orgchart・
// phases・引用つきの pairs)。
func fitFigureDecks() map[string]string {
	hdr := "---\ntitle: t\n---\n\n"
	var m strings.Builder
	m.WriteString("# 行列\n\n```matrix\ncolumns: [案 A, 案 B, 案 C]\nrows:\n")
	for i := 0; i < 11; i++ {
		fmt.Fprintf(&m, "  - label: 行%d\n    cells:\n      - {status: support, text: 文%dの長めの説明}\n      - {status: danger, text: 二つ目}\n      - {status: conditional, text: 三つ目の説明}\n", i, i)
	}
	m.WriteString("```\n")
	var o strings.Builder
	o.WriteString("# 組織\n\n```orgchart\nroot: {title: 長, detail: 花子}\nunits:\n")
	for i := 0; i < 7; i++ {
		fmt.Fprintf(&o, "  - title: 部%d\n    detail: 5名\n    members:\n", i)
		for j := 0; j < 5; j++ {
			fmt.Fprintf(&o, "      - {title: 係%dの担当, detail: [%d名, 兼務あり, 週に2日は別の係]}\n", j, j)
		}
	}
	o.WriteString("```\n")
	var p strings.Builder
	p.WriteString("# 段\n\n```phases\nsteps:\n")
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&p, "  - id: s%d\n    title: Step%d\n    period: 4月\n    items:\n", i, i)
		for j := 0; j < 6; j++ {
			fmt.Fprintf(&p, "      - title: 箱%d\n        detail: [一つ目の補足の行, 二つ目の補足]\n", j)
		}
	}
	p.WriteString("```\n")
	var q strings.Builder
	q.WriteString("# 対\n\n:::::pairs 分かったこと | 打ち手\n")
	for i := 0; i < 9; i++ {
		fmt.Fprintf(&q, "::::subject\n:::quote 利用者 %d、聞き取りにて\n返したのに延滞の知らせが来て驚いた\n:::\n::::\n:::response\n- 箱の中身は開館と同時に読み取る\n- 前の晩の返却日で記録する\n:::\n", i)
	}
	q.WriteString(":::::\n")
	return map[string]string{"matrix": hdr + m.String(), "orgchart": hdr + o.String(), "phases": hdr + p.String(), "pairs-quote": hdr + q.String()}
}

// figureLeaf は組んだ木の中の図の葉(report の結論・引用の印などの飾りは除く)。
func figureLeaf(s *Spec) Subsystem {
	if s.Kind == KindSubsystem {
		switch s.Subsystem.(type) {
		case *OrgChartSubsystem, *PhasesSubsystem, *MatrixSubsystem:
			return s.Subsystem
		}
	}
	for _, c := range s.Children {
		if f := figureLeaf(c); f != nil {
			return f
		}
	}
	return nil
}

// はみ出す matrix・orgchart・phases・引用つきの pairs は、-fit の段を下りるたびに要る高さが
// 下がる(図の Min も段ごとに下がる)。#139 の前は、図の寸法が段に無く Min が止まっていた。
func TestFitShrinkLowersFigureMin(t *testing.T) {
	decks := fitFigureDecks()
	for _, tn := range []string{"dark", "light", "report"} {
		tm := mustTemplate(tn)
		for _, name := range []string{"matrix", "orgchart", "phases", "pairs-quote"} {
			deck, err := ReadDeck(name+".bx.md", []byte(decks[name]))
			if err != nil {
				t.Fatal(err)
			}
			steps := []func(*IRSlide){func(*IRSlide) {}}
			for k := range tm.Fit.Shrink {
				r := fitRef{n: k + 1}
				steps = append(steps, func(s *IRSlide) { s.fit = r })
			}
			var rem, min []float64
			for _, set := range steps {
				dd := fitDeckCopy(deck, []int{1}, set)
				m, err := tm.measureFitStep(dd, 1)
				if err != nil || !m.ok {
					t.Fatalf("%s %s: %v %+v", tn, name, err, m)
				}
				rem = append(rem, m.remaining)
				b, err := tm.buildSlide(dd, 1)
				if err != nil {
					t.Fatal(err)
				}
				if f := figureLeaf(b.Root); f != nil {
					min = append(min, f.Measure(Constraint{Width: 860}).Min.Height)
				}
			}
			if rem[0] >= 0 {
				t.Fatalf("%s %s: the example should overflow without -fit (remaining %v)", tn, name, rem[0])
			}
			for k := 1; k < len(rem); k++ {
				if rem[k] <= rem[k-1]+0.5 {
					t.Errorf("%s %s: remaining %v does not grow at each shrink step", tn, name, rem)
				}
			}
			for k := 1; k < len(min); k++ {
				if min[k] >= min[k-1]-0.5 {
					t.Errorf("%s %s: figure min height %v does not go down at each shrink step", tn, name, min)
				}
			}
			if name != "pairs-quote" && len(min) != len(steps) {
				t.Errorf("%s %s: no figure leaf found", tn, name)
			}
		}
	}
}

// 段の図の寸法と補足の字は組んだ図に届く(fitStepped の登録簿が図の値の写しに当てる)。
// 段を通らない図は template の図の値のまま。
func TestFitStepFigureSizesReachFigures(t *testing.T) {
	decks := fitFigureDecks()
	for _, tn := range []string{"dark", "light", "report"} {
		tm := mustTemplate(tn)
		last := fitRef{n: len(tm.Fit.Shrink)}
		st := last.values(tm)
		leaf := func(name string, set func(*IRSlide)) Subsystem {
			deck, err := ReadDeck(name+".bx.md", []byte(decks[name]))
			if err != nil {
				t.Fatal(err)
			}
			b, err := tm.buildSlide(fitDeckCopy(deck, []int{1}, set), 1)
			if err != nil {
				t.Fatal(err)
			}
			return figureLeaf(b.Root)
		}
		atLast := func(s *IRSlide) { s.fit = last }
		o := leaf("orgchart", atLast).(*OrgChartSubsystem)
		if v := o.vals(); v.MinBoxH != st.OrgChartMinBoxH || v.MemberGap != st.OrgChartMemberGap || v.TextInsetY != st.OrgChartTextInsetY || v.MinLevelGap != st.OrgChartMinLevelGap {
			t.Errorf("%s orgchart: %+v, step %+v", tn, v, st)
		}
		if d := o.text().Detail; d != st.FigureDetail {
			t.Errorf("%s orgchart detail = %v, want %v", tn, d, st.FigureDetail)
		}
		ph := leaf("phases", atLast).(*PhasesSubsystem)
		if v := ph.vals(); v.ItemGap != st.PhasesItemGap || v.HeaderGap != st.PhasesHeaderGap || v.MinHeaderH != st.PhasesMinHeaderH || v.TextInsetY != st.PhasesTextInsetY {
			t.Errorf("%s phases: %+v, step %+v", tn, v, st)
		}
		m := leaf("matrix", atLast).(*MatrixSubsystem)
		if v := m.vals(); v.MinRowH != st.MatrixMinRowH || v.MarkSize != st.MatrixMarkSize || v.CellGap != st.MatrixCellGap || v.TextInsetY != st.MatrixTextInsetY {
			t.Errorf("%s matrix: %+v, step %+v", tn, v, st)
		}
		// 段を通らない図は template の図の値(report は subsystems.matrix の差し替え)。
		m0 := leaf("matrix", func(*IRSlide) {}).(*MatrixSubsystem)
		if v := m0.vals(); v.MinRowH != tm.Values.fig.MatrixMinRowH || v.MarkSize != tm.Values.fig.MatrixMarkSize || v.Padding <= 0 {
			t.Errorf("%s matrix without a step: %+v, fig %+v", tn, v, tm.Values.fig)
		}
	}
}

// dense・fit の図の名前も、元の図の値(このテンプレートの subsystems、書かなければ既定)
// より大きくはできない。
func TestDenseFigureSizesNotAboveFigure(t *testing.T) {
	_, err := parseTemplateFile("light", mutate(t, func(m map[string]any) {
		m["dense"].(map[string]any)["matrixMinRowHeight"] = 32
	}))
	if err == nil || !strings.Contains(err.Error(), "dense.matrixMinRowHeight: 32 is larger than subsystems.matrix.minRowHeight (28)") {
		t.Fatalf("err = %v", err)
	}
	_, err = parseTemplateFile("light", mutate(t, func(m map[string]any) {
		delete(m["dense"].(map[string]any), "orgchartMemberGap")
	}))
	if err == nil || !strings.Contains(err.Error(), "orgchartMemberGap") {
		t.Fatalf("missing dense key: err = %v", err)
	}
	_, err = parseTemplateFile("light", mutate(t, func(m map[string]any) {
		m["fit"].(map[string]any)["shrink"].([]any)[0].(map[string]any)["phasesItemGap"] = 12
	}))
	if err == nil || !strings.Contains(err.Error(), "fit.shrink[0].phasesItemGap: 12 is larger than dense.phasesItemGap (8)") {
		t.Fatalf("fit step above dense: err = %v", err)
	}
}
