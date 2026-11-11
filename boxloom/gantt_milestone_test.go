package boxloom

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func ganttForMilestoneTest(t *testing.T, milestones string) *GanttSubsystem {
	t.Helper()
	data := `{"span":{"from":"2027-01","to":"2027-12"},"milestones":[` + milestones + `],"groups":[{"id":"g","label":["G"],"rows":[
	  {"label":"r","segments":[{"from":"2027-01","to":"2027-12","status":"support","label":"x"}]}]}]}`
	s, err := NewGanttSubsystem(json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}
	return s.(*GanttSubsystem)
}

func placedOverlap(a, b placedBox) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

// 近い節目は段をずらして、文字が重ならない。遠い節目は1段のまま(高さは前と同じ)。
func TestMilestoneRows(t *testing.T) {
	box := LBox{Width: 900, Height: 300}
	cases := []struct {
		name, ms string
		rows     int
		overlaps int
	}{
		{"far", `{"at":"2027-02","label":"開始"},{"at":"2027-10","label":"公開"}`, 1, 0},
		{"same month", `{"at":"2027-06","label":"外部監査"},{"at":"2027-06","label":"一般公開"}`, 2, 0},
		{"three close", `{"at":"2027-06","label":"外部監査"},{"at":"2027-06","label":"一般公開"},{"at":"2027-07","label":"契約の更新"}`, 2, 1},
	}
	for _, c := range cases {
		g := ganttForMilestoneTest(t, c.ms)
		gv := g.vals()
		f := ganttFrameFor(gv, g.Chart, box)
		mp := f.gm.milestones
		if mp.rows != c.rows || len(mp.overlap) != c.overlaps {
			t.Errorf("%s: rows %d overlaps %v, want %d and %d", c.name, mp.rows, mp.overlap, c.rows, c.overlaps)
		}
		if want := float64(c.rows)*gv.MilestoneH + 4; f.gm.milestoneH != want {
			t.Errorf("%s: milestone lane %v, want %v", c.name, f.gm.milestoneH, want)
		}
		// 節目の文字の矩形(描くときと同じ計算)どうしが、重なりを許したもの以外は重ならない。
		rects := ganttLabelRects(gv, g.Chart, f)[:len(g.Chart.Milestones)]
		allowed := map[[2]int]bool{}
		for _, ov := range mp.overlap {
			allowed[[2]int{ov[0], ov[1]}], allowed[[2]int{ov[1], ov[0]}] = true, true
		}
		for i := range rects {
			for j := i + 1; j < len(rects); j++ {
				if placedOverlap(rects[i], rects[j]) && !allowed[[2]int{i, j}] {
					t.Errorf("%s: milestones %d and %d overlap: %+v %+v", c.name, i, j, rects[i], rects[j])
				}
			}
		}
		warns := g.Lint(box)
		n := 0
		for _, w := range warns {
			if strings.Contains(w, ") overlaps milestones[") {
				n++
			}
		}
		if n != c.overlaps {
			t.Errorf("%s: Lint = %v", c.name, warns)
		}
	}
}

// 段を足した分、Measure の自然な高さが増える(はみ出しの計測と描画がずれない)。
func TestMilestoneRowsGrowNaturalHeight(t *testing.T) {
	one := ganttForMilestoneTest(t, `{"at":"2027-02","label":"開始"},{"at":"2027-10","label":"公開"}`)
	two := ganttForMilestoneTest(t, `{"at":"2027-06","label":"外部監査"},{"at":"2027-06","label":"一般公開"}`)
	c := Constraint{Width: 900}
	h1, h2 := one.Measure(c).Natural.Height, two.Measure(c).Natural.Height
	if d := h2 - h1; d < one.vals().MilestoneH-0.01 || d > one.vals().MilestoneH+0.01 {
		t.Fatalf("natural height grew by %v, want %v", d, one.vals().MilestoneH)
	}
}

// 例の IR/Markdown:誤り0件で milestone-overlap の警告が、重ねて置いた節目を指して出る。
func TestCheckMilestoneOverlapExample(t *testing.T) {
	const want = "/slides/0/body/0/data/milestones/2"
	for _, f := range []string{"../examples/ir/check-warnings/milestone-overlap.json", "../examples/md/warnings/milestone-overlap.bx.md"} {
		errs, warns := CheckFile(f, "")
		var got []Issue
		for _, w := range warns {
			if w.Code == "milestone-overlap" {
				got = append(got, w)
			}
		}
		if len(errs) != 0 || len(got) != 1 || got[0].Path != want || len(warns) != 1 {
			t.Fatalf("%s: errs %+v warns %+v", f, errs, warns)
		}
		if strings.HasSuffix(f, ".md") {
			src, _ := os.ReadFile(f)
			if l := lineOf(string(src), "契約の更新}"); got[0].Line != l {
				t.Fatalf("line %d, want %d", got[0].Line, l)
			}
		}
	}
}
