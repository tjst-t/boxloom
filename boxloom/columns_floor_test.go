package boxloom

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 列の両側の図が下限(columnFloor)を持つとき(#132)。文はすべて架空。

const floorCycle = "```cycle\nsteps:\n" +
	"  - title: 地域の担い手を集めて話を聞く場をつくる\n  - title: 週末の拠点を開いて催しを続けていく\n" +
	"  - title: 小さな試みを重ねて手応えを確かめる\n  - title: 運営の決まりを皆で話し合って書く\n" +
	"  - title: 一年をふり返って次の担い手へ渡す\n```\n"

const floorPillars = "```pillars\nroof: 運営を10年続けられる形にする\npillars:\n" +
	"  - {title: 人, items: [担い手を3人以上]}\n  - {title: お金, items: [会費と町の補助]}\n" +
	"  - {title: 場所, items: [週末の拠点]}\n  - {title: 決まり, items: [運営の決まりを書く]}\n" +
	"  - {title: ふり返り, items: [年に一度の見直し]}\nfoundation: 住民どうしの信頼\n```\n"

// floorMatrix は cols 列・3行の matrix。
func floorMatrix(cols int) string {
	var b strings.Builder
	b.WriteString("```matrix\ncolumns: [")
	for j := 0; j < cols; j++ {
		if j > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "案 %c", 'A'+j)
	}
	b.WriteString("]\nrows:\n")
	sts := []string{"support", "conditional", "danger"}
	for i, lab := range []string{"費用", "期限", "担い手"} {
		fmt.Fprintf(&b, "  - label: %s\n    cells:\n", lab)
		for j := 0; j < cols; j++ {
			fmt.Fprintf(&b, "      - {status: %s, text: 架空の町で試した%sの見込み}\n", sts[(i+j)%3], lab)
		}
	}
	b.WriteString("```\n")
	return b.String()
}

// floorMD は左の列に段落と図 left、右の列 {role=main} に図 right の1枚。
func floorMD(tmpl, left, right string) string {
	return "---\ntitle: T\ntemplate: " + tmpl + "\n---\n\n# 輪と表\n\n::::columns\n:::column 進め方\n" +
		"段ごとに担い手を替えながら回す。\n\n" + left + ":::\n:::column 案の比べ {role=main}\n" + right + ":::\n::::\n"
}

type floorRun struct {
	warns  []Issue
	widths [2]float64 // まとまりの幅
	floors [2]float64 // まとまりの下限(columnMinWidth)
}

func runFloorDeck(t *testing.T, tmpl, src string) floorRun {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.bx.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	errs, warns, _ := CheckFileCapacity(path, "")
	if len(errs) > 0 {
		t.Fatalf("%s: errors %+v", tmpl, errs)
	}
	d, err := ReadDeck(path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate(tmpl)
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	restore := useFonts(tm.Fonts)
	defer restore()
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	}
	var row *Spec
	var find func(s *Spec)
	find = func(s *Spec) {
		if row == nil && s.columns != nil {
			row = s
		}
		for _, c := range s.Children {
			find(c)
		}
	}
	for _, b := range built {
		find(b.Root)
	}
	if row == nil || len(row.columns.cols) != 2 {
		t.Fatalf("%s: no 2-column row", tmpl)
	}
	r := floorRun{warns: warns}
	for i, c := range row.columns.cols {
		r.widths[i] = c.spec.Box.Width
		r.floors[i] = columnMinWidth(c.spec)
	}
	return r
}

// cycle と matrix・pillars と matrix を並べた例:どちらのまとまりも図の下限以上の幅に
// なり、図の文が横にはみ出さず、column-floor も出ない。主のまとまり(matrix)は
// 下限の上で広い方を取る。
func TestColumnFloorsBothKept(t *testing.T) {
	for _, tc := range []struct {
		name, left string
		cols       int
	}{
		{"cycle+matrix", floorCycle, 3},
		{"cycle+matrix4", floorCycle, 4},
		{"pillars+matrix", floorPillars, 3},
	} {
		r := runFloorDeck(t, "report", floorMD("report", tc.left, floorMatrix(tc.cols)))
		for i := range r.widths {
			if r.floors[i] <= 0 {
				t.Errorf("%s: columns[%d] declares no floor", tc.name, i)
			}
			if r.widths[i] < r.floors[i]-0.01 {
				t.Errorf("%s: columns[%d] is %.1fpt, below its floor %.1fpt", tc.name, i, r.widths[i], r.floors[i])
			}
		}
		for _, w := range r.warns {
			switch w.Code {
			case "column-floor", "figure-text-overflow", "subsystem-min":
				t.Errorf("%s: unexpected %s: %s", tc.name, w.Code, w.Message)
			}
		}
		t.Logf("%s: widths %v floors %v", tc.name, r.widths, r.floors)
	}
}

// matrix の下限は列の数を見る:Min の幅(240pt)では列の文が入らない多い列の
// matrix は、文が横に入る幅を下限に申告する(列が数 pt まで潰れない)。
func TestMatrixColumnFloorCountsColumns(t *testing.T) {
	restore := useFonts(mustTemplate("report").Fonts)
	defer restore()
	data := func(cols int) string {
		var b strings.Builder
		b.WriteString(`{"columns": [`)
		for j := 0; j < cols; j++ {
			if j > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `"案 %c"`, 'A'+j)
		}
		b.WriteString(`], "rows": [{"label": "費用", "cells": [`)
		for j := 0; j < cols; j++ {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"status": "support", "text": "架空の町の見込み"}`)
		}
		b.WriteString("]}]}")
		return b.String()
	}
	prev := 0.0
	for _, cols := range []int{2, 4, 6} {
		m := figureFromIR(t, "report", "matrix", data(cols)).(*MatrixSubsystem)
		f := m.columnMinWidth()
		if f < m.Measure(Constraint{}).Min.Width || f < prev {
			t.Errorf("%d columns: floor %.1f (Min %.1f, previous %.1f)", cols, f, m.Measure(Constraint{}).Min.Width, prev)
		}
		if l := m.Lint(LBox{Width: f, Height: m.Measure(Constraint{Width: f}).Max.Height}); len(l) > 0 {
			t.Errorf("%d columns: text does not fit at the floor %.1f: %v", cols, f, l)
		}
		prev = f
	}
	if prev <= 240 {
		t.Errorf("a 6-column matrix floor %.1f should exceed the Min width", prev)
	}
}

// 下限の和が本文の幅を超える例:まとまりは下限の比で配られ(どちらも同じ割合だけ
// 足りない)、下限を割ったまとまりごとに column-floor が出る(場所はまとまり)。
func TestColumnFloorWarnsWhenBothCannotFit(t *testing.T) {
	for _, tmpl := range []string{"report", "dark"} {
		r := runFloorDeck(t, tmpl, floorMD(tmpl, floorPillars, floorMatrix(6)))
		ratio := [2]float64{}
		for i := range r.widths {
			if r.widths[i] <= 0 || r.widths[i] >= r.floors[i] {
				t.Fatalf("%s: columns[%d] %.1fpt is not below its floor %.1fpt", tmpl, i, r.widths[i], r.floors[i])
			}
			ratio[i] = r.widths[i] / r.floors[i]
		}
		if math.Abs(ratio[0]-ratio[1]) > 1e-6 {
			t.Errorf("%s: widths %v are not in the ratio of the floors %v", tmpl, r.widths, r.floors)
		}
		got := map[string]bool{}
		for _, w := range r.warns {
			if w.Code == "column-floor" {
				got[w.Path] = true
				if !strings.Contains(w.Message, "short") {
					t.Errorf("%s: message %q does not say how much is short", tmpl, w.Message)
				}
			}
		}
		for i := 0; i < 2; i++ {
			if p := fmt.Sprintf("/slides/1/body/0/columns/%d", i); !got[p] {
				t.Errorf("%s: no column-floor at %s (got %v)", tmpl, p, got)
			}
		}
	}
}

// fakeFloor は下限だけを申告する図(applyColumnFloors の配り方の試験)。
type fakeFloor struct{ min float64 }

func (fakeFloor) Name() string                   { return "fakeFloor" }
func (fakeFloor) Measure(Constraint) Measurement { return Measurement{} }
func (fakeFloor) Render(RenderContext, LBox)     {}
func (f fakeFloor) columnMinWidth() float64      { return f.min }

// 3つのまとまり:下限に届かないまとまりを下限で止め、残りは grow の比で配る
// (主のまとまりは下限を守った上で、ほかの倍の幅)。
func TestApplyColumnFloorsKeepsMainWeight(t *testing.T) {
	col := func(min, grow float64) *Spec {
		return Col([]*Spec{SubsystemNode(fakeFloor{min}, WithGrow(1))}, WithGrow(grow), WithBasis(0))
	}
	kids := []*Spec{col(500, 1), col(0, 2), col(0, 1)}
	row := Row(kids, measureColumnGroups(&columnsInfo{}))
	computeLayout(row, 0, 0, 1000, 100)
	w := []float64{kids[0].Box.Width, kids[1].Box.Width, kids[2].Box.Width}
	want := []float64{500, 1000.0 / 3, 500.0 / 3}
	for i := range w {
		if math.Abs(w[i]-want[i]) > 1e-6 {
			t.Errorf("widths %v, want %v", w, want)
			break
		}
	}
}
