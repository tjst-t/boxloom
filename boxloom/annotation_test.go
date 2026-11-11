package main

import (
	"strings"
	"testing"
	"time"
)

func autoAnn(at time.Time, text string) Annotation {
	return Annotation{AnchorDate: at, Text: text, Role: "danger", TargetGroup: -1, TargetRow: -1}
}

func annChart(anns ...Annotation) *GanttChart {
	from, to := d(2026, time.January, 1), d(2030, time.January, 1)
	return &GanttChart{
		Scale:       sqrtScale(from, to, nil),
		Groups:      []GanttGroup{{Label: "g", Rows: []GanttRow{{Label: "r"}, {Label: "r2"}}}},
		Annotations: anns,
	}
}

// 自動の注釈の箱は文に合わせる: 短い文は狭く、長い1行は上限の幅で折り返して高くなる。
func TestAnnotationBoxFitsText(t *testing.T) {
	sw, sh := annBoxSize(autoAnn(d(2027, 1, 1), "短い"))
	if !near(sw, annBoxMinW) {
		t.Fatalf("short text width = %v, want min %v", sw, annBoxMinW)
	}
	long := strings.Repeat("とても長い注釈の文", 8)
	lw, lh := annBoxSize(autoAnn(d(2027, 1, 1), long))
	if !near(lw, annBoxMaxW) || lh <= sh {
		t.Fatalf("long text box = %vx%v, want width %v and taller than %v", lw, lh, annBoxMaxW, sh)
	}
	mw, _ := annBoxSize(autoAnn(d(2027, 1, 1), "ここまでに全利用者を\nBoard B/C へ移行予定"))
	if mw <= annBoxMinW || mw >= annBoxMaxW {
		t.Fatalf("two-line text width = %v, want between min and max", mw)
	}
	// 手で置いた注釈は従来の固定の大きさ。
	m := autoAnn(d(2027, 1, 1), long)
	m.Direction = "up"
	if w, h := annBoxSize(m); w != annBoxW || h != annBoxH {
		t.Fatalf("manual annotation box = %vx%v, want fixed", w, h)
	}
}

// レーンの高さは、そのレーンの一番高い箱に合わせる(Measure と描画が同じ計画を使う)。
func TestLaneHeightFollowsTallestBox(t *testing.T) {
	long := strings.Repeat("長い文", 40)
	c := annChart(autoAnn(d(2027, 1, 1), "短い"), autoAnn(d(2029, 1, 1), long))
	plan := planAnnotations(c, spaceMD, 900-2*spaceMD)
	gm := ganttMetricsFor(c, 900)
	maxH := 0.0
	for _, pl := range plan {
		if !pl.up && pl.h > maxH {
			maxH = pl.h
		}
	}
	if !near(gm.annBottomH, maxH+annLaneGap) {
		t.Fatalf("bottom lane = %v, want %v", gm.annBottomH, maxH+annLaneGap)
	}
}

// 置き場所が見つからない注釈は、重ねて置き、警告になる。
func TestCrowdedAnnotationsWarn(t *testing.T) {
	var anns []Annotation
	for i := 0; i < 12; i++ {
		anns = append(anns, autoAnn(d(2028, time.January, 1), "同じ日付に集まった注釈"))
	}
	g := &GanttSubsystem{Chart: annChart(anns...)}
	w := g.Lint(LBox{Width: 500, Height: 400})
	if len(w) == 0 || !strings.Contains(w[0], "overlaps") {
		t.Fatalf("want overlap warning, got %v", w)
	}
	// 余裕があれば警告は出ない。
	g2 := &GanttSubsystem{Chart: annChart(anns[:2]...)}
	if w := g2.Lint(LBox{Width: 900, Height: 400}); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
}

// 警告はスライドの組み立ての検査(lintSubsystems)に乗る。
func TestLintSubsystemsIncludesSubsystemLint(t *testing.T) {
	var anns []Annotation
	for i := 0; i < 12; i++ {
		anns = append(anns, autoAnn(d(2028, time.January, 1), "注釈"))
	}
	node := SubsystemNode(&GanttSubsystem{Chart: annChart(anns...)})
	root := Col([]*Spec{node})
	computeLayout(root, 0, 0, 500, 500)
	found := false
	for _, w := range lintSubsystems(root) {
		if strings.Contains(w, "annotation") {
			found = true
		}
	}
	if !found {
		t.Fatal("lintSubsystems did not include the gantt annotation warning")
	}
}
