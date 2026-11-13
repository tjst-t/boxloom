package boxloom

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// 再現(#138):長い行の題と箇条の説明を持つ matrix を、下限をわずかに割る幅に置く。
// 文はすべて架空。

var cliffRows = []struct {
	label  string
	detail []string
}{
	{"運営の費用をまかなう見込み", []string{"町の補助を三年は受けられる見込み", "会費は月に五百円を考えている"}},
	{"担い手を集める見込み", []string{"週末に来られる人を三人以上"}},
}

var cliffStatuses = []string{"support", "conditional", "danger"}

// cliffMatrix は cols 列の matrix の Markdown のコードブロック。
func cliffMatrix(cols int) string {
	var b strings.Builder
	b.WriteString("```matrix\ncolumns: [")
	for j := 0; j < cols; j++ {
		if j > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "案 %c", 'A'+j)
	}
	b.WriteString("]\nrows:\n")
	for i, r := range cliffRows {
		fmt.Fprintf(&b, "  - label: %s\n    detail: [%s]\n    cells:\n", r.label, strings.Join(r.detail, ", "))
		for j := 0; j < cols; j++ {
			fmt.Fprintf(&b, "      - {status: %s, text: 見込みあり}\n", cliffStatuses[(i+j)%3])
		}
	}
	b.WriteString("```\n")
	return b.String()
}

// cliffMatrixJSON は cliffMatrix と同じ内容の IR の data。
func cliffMatrixJSON(cols int) string {
	type cell struct {
		Status string `json:"status"`
		Text   string `json:"text"`
	}
	type row struct {
		Label  string   `json:"label"`
		Detail []string `json:"detail"`
		Cells  []cell   `json:"cells"`
	}
	var d struct {
		Columns []string `json:"columns"`
		Rows    []row    `json:"rows"`
	}
	for j := 0; j < cols; j++ {
		d.Columns = append(d.Columns, fmt.Sprintf("案 %c", 'A'+j))
	}
	for i, r := range cliffRows {
		rw := row{Label: r.label, Detail: r.detail}
		for j := 0; j < cols; j++ {
			rw.Cells = append(rw.Cells, cell{cliffStatuses[(i+j)%3], "見込みあり"})
		}
		d.Rows = append(d.Rows, rw)
	}
	b, _ := json.Marshal(d)
	return string(b)
}

// 行の題と評価の列が語の途中で折れない幅(floor の和)から Min の幅まで狭めても、
// 評価の列は最小(印と短い文が入る幅)を保ち、先に縮むのは行の題の列。文は横に
// はみ出さず、評価の列の幅は崖のように落ちない(1pt 狭めて 2pt 以上は変わらない)。
// 以前はこの間で自然の幅の比に切り替わり、評価の列が数 pt に崩れた。行の題の列が
// 一番長い語の幅を割って初めて両方が最小を割り、figure-text-overflow になる(#145)。
func TestMatrixNarrowKeepsRatingColumns(t *testing.T) {
	restore := useFonts(mustTemplate("report").Fonts)
	defer restore()
	for _, cols := range []int{3, 4} {
		m := figureFromIR(t, "report", "matrix", cliffMatrixJSON(cols)).(*MatrixSubsystem)
		v, mw, C := m.vals(), m.widths(), float64(cols)
		comfy := 2*v.Padding + C*v.CellGap + mw.headFloor + C*mw.colFloor
		min := m.Measure(Constraint{}).Min.Width
		if want := mw.minWidth(v, cols); min < want-0.01 || min < v.MinWidth {
			t.Errorf("%d columns: Min width %.1f does not match the split's minimum %.1f", cols, min, want)
		}
		if f := m.columnMinWidth(); f < min || f > min+1 {
			t.Errorf("%d columns: column floor %.1f is not the Min width %.1f", cols, f, min)
		}
		if comfy-min < 20 {
			t.Fatalf("%d columns: the example leaves no room between Min %.1f and %.1f", cols, min, comfy)
		}
		prev := m.geom(comfy + 20).colW
		for w := comfy + 20; w >= min; w -= 1 {
			g := m.geom(w)
			if g.colW < mw.colMin-0.01 {
				t.Errorf("%d columns at %.0fpt: rating column %.1fpt < its minimum %.1fpt", cols, w, g.colW, mw.colMin)
			}
			if g.headW < mw.headFloor-0.01 {
				t.Errorf("%d columns at %.0fpt: row title column %.1fpt < its minimum %.1fpt", cols, w, g.headW, mw.headFloor)
			}
			if prev-g.colW > 2 {
				t.Errorf("%d columns at %.0fpt: rating column drops from %.1f to %.1fpt (cliff)", cols, w, prev, g.colW)
			}
			prev = g.colW
			h := m.Measure(Constraint{Width: w}).Max.Height
			if l := m.Lint(LBox{Width: w, Height: h}); len(l) > 0 {
				t.Errorf("%d columns at %.0fpt (Min %.1f): %v", cols, w, min, l)
			}
		}
		// Min を割ると、行の題の列が一番長い語の幅(headMin)まで先に譲り、評価の列は
		// 最小を保つ(#145。題は字の境で折れるだけで、評価の列の短い語は1字ずつ折れない)。
		// 最小の大きさの警告は出る。
		w := min - 8
		if g := m.geom(w); g.colW < mw.colMin-0.01 {
			t.Errorf("%d columns at %.0fpt below Min %.1f: rating column %.1fpt < its minimum %.1fpt", cols, w, min, g.colW, mw.colMin)
		}
		h := m.Measure(Constraint{Width: w}).Max.Height
		if l := lintSubsystemMin(&Spec{Kind: KindSubsystem, Subsystem: m, Box: LBox{Width: w, Height: h}}); len(l) == 0 {
			t.Errorf("%d columns at %.0fpt below Min %.1f: no subsystem-min", cols, w, min)
		}
		// 行の題の列も語の幅を割って初めて、両方が最小を割り figure-text-overflow になる。
		w = 2*v.Padding + C*v.CellGap + mw.headMin + C*mw.colMin - 8
		h = m.Measure(Constraint{Width: w}).Max.Height
		if l := m.Lint(LBox{Width: w, Height: h}); len(l) == 0 {
			t.Errorf("%d columns at %.0fpt below the words' width: no figure-text-overflow", cols, w)
		}
	}
}

// 第7回の再現の形(元の p29):左の列に pillars、右の列に matrix。以前は matrix の
// 下限が評価の列の崩れる手前に置かれ、pillars の下限と合わせて本文の幅を 8pt
// 割ると評価の列が 4pt ほどに崩れた。今は matrix が評価の列の最小までゆずるので、
// どちらの図の文もはみ出さず、まとまりも下限を割らない。
func TestMatrixBesidePillarsBelowFloor(t *testing.T) {
	for _, cols := range []int{3, 4} {
		r := runFloorDeck(t, "report", floorMD("report", floorPillars, cliffMatrix(cols)))
		t.Logf("%d columns: widths %v floors %v", cols, r.widths, r.floors)
		for _, w := range r.warns {
			switch w.Code {
			case "figure-text-overflow", "column-floor", "subsystem-min":
				t.Errorf("%d columns: %s: %s", cols, w.Code, w.Message)
			}
		}
	}
}
