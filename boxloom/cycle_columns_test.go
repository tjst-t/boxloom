package boxloom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 列の中の cycle(#112)。文はすべて架空。

const cycleColumnSteps = `[
  {"title": "地域の担い手を集めて話を聞く場をつくる"}, {"title": "週末の拠点を開いて催しを続けていく"},
  {"title": "小さな試みを重ねて手応えを確かめる"}, {"title": "運営の決まりを皆で話し合って書く"},
  {"title": "一年をふり返って次の担い手へ渡す"}]`

func cycleColumnsMD(tmpl string, extra int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: T\ntemplate: %s\n---\n\n# 輪と説明\n\n::::columns\n:::column\n```cycle\nsteps:\n", tmpl)
	for _, s := range []string{"地域の担い手を集めて話を聞く場をつくる", "週末の拠点を開いて催しを続けていく",
		"小さな試みを重ねて手応えを確かめる", "運営の決まりを皆で話し合って書く", "一年をふり返って次の担い手へ渡す"} {
		fmt.Fprintf(&b, "  - title: %s\n", s)
	}
	b.WriteString("```\n:::\n:::column 段ごとの要点 {role=main}\n- 住民に話を聞いて、次に何をするかを決める\n" +
		"- 担い手を決めて、週末の拠点を任せる\n- 催しを続けて、手応えを確かめる\n:::\n")
	for i := 0; i < extra; i++ {
		fmt.Fprintf(&b, ":::column 補足%d\n- 架空の町の例\n:::\n", i+1)
	}
	b.WriteString("::::\n")
	return b.String()
}

// findSubsystem は配置の段の木から名前 name のサブシステムの葉を探す。
func findSubsystem(n *LayoutNode, name string) *LayoutNode {
	if n == nil {
		return nil
	}
	if n.Subsystem != nil && n.Subsystem.Name == name {
		return n
	}
	for _, c := range n.Children {
		if f := findSubsystem(c, name); f != nil {
			return f
		}
	}
	return nil
}

// columns の狭い列に 20 字前後の題5つの cycle、横に role=main の箇条:題は扇に
// 入らないので輪の外に置く。列の幅は題を外に置くのに要る幅(Min)まで広がり、題どうし・
// 題と輪(番号の丸を含む正方形)は重ならず、警告も出ない。列がさらに狭くて外にも
// 入らなければ、警告(figure-text-overflow か subsystem-min)が出る。
func TestCycleTitlesOutsideInNarrowColumn(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		for _, extra := range []int{0, 1} {
			name := fmt.Sprintf("%s+%d", tmpl, extra)
			src := []byte(cycleColumnsMD(tmpl, extra))
			path := filepath.Join(t.TempDir(), "c.bx.md")
			if err := os.WriteFile(path, src, 0o644); err != nil {
				t.Fatal(err)
			}
			errs, warns, _ := CheckFileCapacity(path, tmpl)
			if len(errs) > 0 {
				t.Fatalf("%s: errors %+v", name, errs)
			}
			warned := false
			for _, w := range warns {
				if w.Code == "figure-text-overflow" || w.Code == "subsystem-min" {
					warned = true
				}
			}
			dump, err := Layout(path, src, BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			leaf := findSubsystem(dump.Slides[1].Root, "cycle")
			if leaf == nil {
				t.Fatalf("%s: no cycle in the layout", name)
			}
			d := leaf.Subsystem.Draw
			box := LBox{X: d.X, Y: d.Y, Width: d.Width, Height: d.Height}
			sub := figureFromIR(t, tmpl, "cycle", `{"steps": `+cycleColumnSteps+`}`).(*CycleSubsystem)
			restore := useFonts(mustTemplate(tmpl).Fonts)
			pl := sub.place(box)
			lint := sub.Lint(box)
			restore()
			overlap := cycleTitlesOverlap(sub, pl)
			if extra == 0 {
				// 2つの列:列が広がり、題は輪の外に重ならずに入る。
				if !pl.outside {
					t.Errorf("%s: titles must go outside the ring (box %+v)", name, box)
				}
				if overlap != "" || warned || len(lint) > 0 {
					t.Errorf("%s: box %+v overlap=%q warns=%+v lint=%v", name, box, overlap, warns, lint)
				}
				continue
			}
			t.Logf("%s: outside=%v warned=%v overlap=%q box=%+v", name, pl.outside, warned, overlap, box)
			if overlap != "" && !warned {
				t.Errorf("%s: titles overlap without a warning: %s", name, overlap)
			}
		}
	}
}

// cycleTitlesOverlap は置いた題どうし・題と輪が重なるかを書く(重ならなければ "")。
// 題を扇の中に置いたときは、扇に入らなかった題があれば重なりとみなす。
func cycleTitlesOverlap(sub *CycleSubsystem, pl *cyclePlaced) string {
	if !pl.outside {
		for i, r := range pl.titleRoom {
			if r.Width > 0 || r.Height > 0 {
				return fmt.Sprintf("steps[%d] does not fit its sector", i)
			}
		}
		return ""
	}
	e := pl.r + sub.vals().NumberDiameter/2
	ring := LBox{X: pl.cx - e, Y: pl.cy - e, Width: 2 * e, Height: 2 * e}
	for i, l := range pl.labels {
		if boxesOverlap(l.box, ring) {
			return fmt.Sprintf("steps[%d] %+v overlaps the ring %+v", i, l.box, ring)
		}
		for j := i + 1; j < len(pl.labels); j++ {
			if boxesOverlap(l.box, pl.labels[j].box) {
				return fmt.Sprintf("steps[%d] and steps[%d] overlap", i, j)
			}
		}
	}
	return ""
}

// どの幅でも:題が扇に入らなければ輪の外に置き、外にも入らなければ警告が出る
// (重なったまま黙らない)。Min の幅からは警告が出ない。
func TestCycleTitlesAnyWidth(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		sub := figureFromIR(t, tmpl, "cycle", `{"steps": `+cycleColumnSteps+`}`).(*CycleSubsystem)
		restore := useFonts(mustTemplate(tmpl).Fonts)
		m := sub.Measure(Constraint{})
		for w := 160.0; w <= 760; w += 30 {
			for _, h := range []float64{sub.Measure(Constraint{Width: w}).Natural.Height, 420} {
				box := LBox{X: 40, Y: 80, Width: w, Height: h}
				pl := sub.place(box)
				lint := sub.Lint(box)
				overlap := cycleTitlesOverlap(sub, pl)
				if overlap != "" && len(lint) == 0 {
					t.Errorf("%s %.0fx%.0f: %s without a warning", tmpl, w, h, overlap)
				}
				if w >= m.Min.Width && h >= sub.Measure(Constraint{Width: w}).Natural.Height && (overlap != "" || len(lint) > 0) {
					t.Errorf("%s %.0fx%.0f (min width %.1f): overlap=%q lint=%v", tmpl, w, h, m.Min.Width, overlap, lint)
				}
			}
		}
		restore()
	}
}
