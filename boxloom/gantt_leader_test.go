package boxloom

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ganttLeaves は、組んで並べたデッキのすべての Gantt の葉(描く矩形つき)。
func ganttLeaves(t *testing.T, path, template string) (leaves []*GanttSubsystem, boxes []LBox) {
	t.Helper()
	d, err := readDeck(path)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate(template)
	tm.SetAssets(os.DirFS(filepath.Dir(path)))
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindSubsystem {
			if g, ok := s.Subsystem.(*GanttSubsystem); ok {
				leaves = append(leaves, g)
				boxes = append(boxes, subsystemDrawBox(s))
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		walk(b.Root)
	}
	return leaves, boxes
}

// segHitsRect は、線分を 0.1pt ごとに見て、矩形の内側(縁から inset だけ内)に
// 入る点があるか(splitAround とは別の求め方で確かめる)。
func segHitsRect(s lineSeg, r placedBox, inset float64) bool {
	n := int(math.Hypot(s.x2-s.x1, s.y2-s.y1)/0.1) + 1
	for k := 0; k <= n; k++ {
		u := float64(k) / float64(n)
		x, y := s.x1+u*(s.x2-s.x1), s.y1+u*(s.y2-s.y1)
		if x > r.x+inset && x < r.x+r.w-inset && y > r.y+inset && y < r.y+r.h-inset {
			return true
		}
	}
	return false
}

// 注釈の引き出し線は、目盛りと節目の文字を横切らない(#40)。線は点から箱まで
// 続いて見える(最初の線分は点から、最後の線分は箱の縁で終わる)。
func TestGanttLeaderAvoidsAxisLabels(t *testing.T) {
	cases := []struct {
		path      string
		wantSplit bool // 文字を避けて切った引き出し線があるはず(例が #40 を試している)
	}{
		{"../examples/md/gantt-leader.bx.md", true},
		{"../examples/ir/gantt-leader.json", true},
		{"../examples/md/subsystems-showcase.bx.md", true},
		{"../examples/md/gantt-full.bx.md", true},
		{"../examples/ir/gantt-full.json", true},
	}
	for _, c := range cases {
		for _, tmpl := range []string{"dark", "light"} {
			leaves, boxes := ganttLeaves(t, c.path, tmpl)
			if len(leaves) == 0 {
				t.Fatalf("%s: no gantt", c.path)
			}
			split := false
			for li, g := range leaves {
				gv := g.vals()
				f := ganttFrameFor(gv, g.Chart, boxes[li])
				labels := ganttLabelRects(gv, g.Chart, f)
				for ai, ag := range annotationGeoms(gv, g.Chart, f) {
					if len(ag.leader) == 0 {
						t.Errorf("%s %s: annotation %d has no leader", c.path, tmpl, ai)
						continue
					}
					if len(ag.leader) > 1 {
						split = true
					}
					for _, sg := range ag.leader {
						for ri, r := range labels {
							if segHitsRect(sg, r, -gv.LeaderGap+0.05) {
								t.Errorf("%s %s: annotation %d leader %+v crosses label %d %+v", c.path, tmpl, ai, sg, ri, r)
							}
						}
					}
					first, last := ag.leader[0], ag.leader[len(ag.leader)-1]
					if !nearPt(first.x1, ag.ax) || !nearPt(first.y1, ag.ay) {
						t.Errorf("%s %s: annotation %d leader starts at (%.1f,%.1f), dot at (%.1f,%.1f)", c.path, tmpl, ai, first.x1, first.y1, ag.ax, ag.ay)
					}
					b := ag.box
					if !nearPt(last.x2, b.x+b.w/2) || !(nearPt(last.y2, b.y) || nearPt(last.y2, b.y+b.h)) {
						t.Errorf("%s %s: annotation %d leader ends at (%.1f,%.1f), not on box %+v", c.path, tmpl, ai, last.x2, last.y2, b)
					}
				}
			}
			if split != c.wantSplit {
				t.Errorf("%s %s: split leader = %v, want %v", c.path, tmpl, split, c.wantSplit)
			}
		}
	}
}

// 文字と交わらない線はそのまま1本(従来の PPTX と同じ線)。交わる線は文字の
// 前後で切れ、切った間は文字(+隙間)の中だけ。
func TestSplitAround(t *testing.T) {
	s := lineSeg{10, 100, 10, 0}
	if got := splitAround(s, []placedBox{{50, 40, 20, 10}}, 2); len(got) != 1 || got[0] != s {
		t.Fatalf("no crossing: %+v", got)
	}
	got := splitAround(s, []placedBox{{0, 40, 20, 10}, {0, 60, 20, 10}}, 2)
	want := []lineSeg{{10, 100, 10, 72}, {10, 58, 10, 52}, {10, 38, 10, 0}}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if !nearPt(g.x1, w.x1) || !nearPt(g.y1, w.y1) || !nearPt(g.x2, w.x2) || !nearPt(g.y2, w.y2) {
			t.Fatalf("piece %d: got %+v, want %+v", i, g, w)
		}
	}
	// 1pt に満たない切れ端は描かない。
	if got := splitAround(s, []placedBox{{0, 40, 20, 10}, {0, 50.5, 20, 10}}, 0); len(got) != 2 {
		t.Fatalf("short piece kept: %+v", got)
	}
}
