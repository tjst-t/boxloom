package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoDeckPath = "../examples/ir/demo-deck.json"
const ganttFullPath = "../examples/ir/gantt-full.json"

func mustReadIR(t *testing.T, path string) *IRDeck {
	t.Helper()
	d, err := ReadIR(path)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 例の IR が読め、種類と中身が取れる。
func TestParseDemoDeck(t *testing.T) {
	d := mustReadIR(t, demoDeckPath)
	if len(d.Slides) != 3 || d.Slides[0].Type != "cover" || d.Slides[2].Type != "content" {
		t.Fatalf("unexpected slides: %+v", d.Slides)
	}
	body := d.Slides[1].Body
	if body[0].Type != "points" || body[1].Type != "compare" || body[2].Type != "note" {
		t.Fatalf("unexpected blocks: %v %v %v", body[0].Type, body[1].Type, body[2].Type)
	}
	first := body[0].Items[0].Content
	if len(first) != 3 || first[1].Type != "em" || first[1].Content.Plain() != "2029年12月" {
		t.Fatalf("inline em not parsed: %+v", first)
	}
	if d.Slides[2].Body[0].Subsystem != "gantt" || len(d.Slides[2].Body[0].Data) == 0 {
		t.Fatal("subsystem block not parsed")
	}
}

// 知らないキーは、IR の中の場所を添えて断る。
func TestParseRejectsUnknownKeyWithLocation(t *testing.T) {
	src := `{"boxloom":"0.1","slides":[{"type":"cover"},{"type":"content","title":"t","body":[{"type":"note","content":"x","align":"right"}]}]}`
	_, err := ParseIR([]byte(src))
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "slides[1]") || !strings.Contains(err.Error(), "body[0]") || !strings.Contains(err.Error(), "align") {
		t.Fatalf("error should point at slides[1].body[0] and name the key: %v", err)
	}
}

// examples/ir/invalid/ の断られるべき例は、組み立てまでのどこかで全部断られる
// (Schema の検査 tools/validate_ir.py と同じ結論)。
func TestInvalidExamplesAreRejected(t *testing.T) {
	files, _ := filepath.Glob("../examples/ir/invalid/*.json")
	if len(files) == 0 {
		t.Fatal("no invalid examples found")
	}
	for _, f := range files {
		d, err := ReadIR(f)
		if err == nil {
			tm, terr := LookupTemplate(d.Template)
			if terr != nil {
				t.Fatal(terr)
			}
			_, err = tm.BuildDeck(d)
		}
		if err == nil {
			t.Errorf("%s: accepted, want rejection", filepath.Base(f))
		}
	}
}

func demoGanttChart(t *testing.T, path string, slide int) *GanttChart {
	t.Helper()
	d := mustReadIR(t, path)
	chart, err := ganttFromIR(d.Slides[slide].Body[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	return chart
}

// IR の Gantt から GanttChart を組む: 凡例の順・両端の目盛り・状態・注釈の指す先。
func TestGanttFromIR(t *testing.T) {
	c := demoGanttChart(t, demoDeckPath, 2)
	if got := []string{c.Legend[0].Role, c.Legend[1].Role, c.Legend[2].Role}; strings.Join(got, ",") != "support,none,risk" {
		t.Fatalf("legend order = %v, want written order", got)
	}
	var labels []string
	for _, tk := range c.Ticks {
		labels = append(labels, tk.MainLabel)
	}
	if strings.Join(labels, ",") != "26/9,29/12,34/11" {
		t.Fatalf("ticks = %v, want span ends added", labels)
	}
	if c.Ticks[1].Role != "danger" || c.Ticks[1].SubLabel != "Board A・Store X" {
		t.Fatalf("tick = %+v", c.Ticks[1])
	}
	if c.Groups[0].Label != "第1世代\nBoard A\n120台\nStore X" || !c.Groups[0].HasRiskBand {
		t.Fatalf("group = %+v", c.Groups[0])
	}
	a := c.Annotations[0]
	if a.Direction != "" || a.TargetGroup != 0 || a.TargetRow != -1 || a.Text != "ここまでに全利用者を\nBoard B/C へ移行予定" {
		t.Fatalf("annotation = %+v", a)
	}
	if c.Milestones[0].Label != "28/5 ライセンス更新" {
		t.Fatalf("milestone = %q", c.Milestones[0].Label)
	}
}

func TestGanttFromIRRejects(t *testing.T) {
	cases := map[string]string{
		"unknown target": `{"span":{"from":"2026-09","to":"2030-01"},"groups":[{"label":"g","rows":[{"label":"r","segments":[]}]}],"annotations":[{"at":"2027-01","target":"nope","text":"x"}]}`,
		"bad status":     `{"span":{"from":"2026-09","to":"2030-01"},"groups":[{"label":"g","rows":[{"label":"r","segments":[{"from":"2026-09","to":"2027-01","status":"text_primary"}]}]}]}`,
		"position key":   `{"span":{"from":"2026-09","to":"2030-01"},"groups":[{"label":"g","rows":[{"label":"r","segments":[]}]}],"annotations":[{"at":"2027-01","text":"x","direction":"up"}]}`,
		"duplicate id":   `{"span":{"from":"2026-09","to":"2030-01"},"groups":[{"id":"a","label":"g","rows":[{"id":"a","label":"r","segments":[]}]}]}`,
		"span order":     `{"span":{"from":"2030-01","to":"2026-09"},"groups":[{"label":"g","rows":[{"label":"r","segments":[]}]}]}`,
	}
	for name, src := range cases {
		if _, err := ganttFromIR([]byte(src)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// 自動の注釈は、同じレーンの中で重ならない。指す先が上半分なら上のレーンを先に使う。
func TestAutoAnnotationsDoNotOverlap(t *testing.T) {
	c := demoGanttChart(t, ganttFullPath, 0)
	x0, w := 40.0, 880.0
	plan := planAnnotations(c, x0, w)
	for i := range plan {
		if plan[i].bx < x0-1e-9 || plan[i].bx+annBoxW > x0+w+1e-9 {
			t.Errorf("annotation %d outside the chart: bx=%v", i, plan[i].bx)
		}
		for j := i + 1; j < len(plan); j++ {
			if plan[i].up == plan[j].up && rectsOverlap(placedBox{plan[i].bx, 0, annBoxW, annBoxH}, placedBox{plan[j].bx, 0, annBoxW, annBoxH}, 4) {
				t.Errorf("annotations %d and %d overlap in the same lane", i, j)
			}
		}
	}
	// 0 番目は第1世代(上半分)を指すので上のレーン。
	if !plan[0].up {
		t.Errorf("annotation 0 points at the top group; want the upper lane")
	}
}

// 申告した高さと、描くときの置き方は同じ計画から来る(上下のレーンの有無がずれない)。
func TestGanttMeasureMatchesPlan(t *testing.T) {
	c := demoGanttChart(t, demoDeckPath, 2)
	g := &GanttSubsystem{Chart: c}
	m := g.Measure(Constraint{Width: 912})
	gm := ganttMetricsFor(c, 912)
	if gm.annTopH != 0 || gm.annBottomH == 0 {
		t.Fatalf("one annotation at the middle should use only the lower lane: %+v", gm)
	}
	if m.Max.Height <= m.Natural.Height || m.Natural.Height <= m.Min.Height {
		t.Fatalf("want min < natural < max, got %+v", m)
	}
}

// サブシステムの上限(Max)で grow が止まり、余りはほかの伸びる子へ回る。
func TestGrowStopsAtSubsystemMax(t *testing.T) {
	sub := SubsystemNode(maxSub{}, WithGrow(1))
	other := Col(nil, WithGrow(1))
	root := Col([]*Spec{sub, other})
	computeLayout(root, 0, 0, 100, 500)
	if !near(sub.Box.Height, 150) {
		t.Fatalf("sub height = %v, want 150 (its max)", sub.Box.Height)
	}
	if !near(other.Box.Height, 350) {
		t.Fatalf("other height = %v, want 350 (the rest)", other.Box.Height)
	}
}

type maxSub struct{}

func (maxSub) Name() string { return "max" }
func (maxSub) Measure(c Constraint) Measurement {
	return Measurement{Natural: Size{c.Width, 100}, Min: Size{0, 50}, Max: Size{0, 150}}
}
func (maxSub) Render(RenderContext, LBox) {}

// 装飾の無い文字では、wrapRuns は wrapText と同じところで折り返す。
func TestWrapRunsMatchesWrapText(t *testing.T) {
	text := "Loom 4 は Store X 非対応のため、第1世代は移行先になり得ない\n2行目"
	for _, w := range []float64{60, 120, 200, 400} {
		plain := wrapText(text, 14, w, false)
		rich := wrapRuns([]TextRun{{Text: text}}, 14, w)
		if len(plain) != len(rich) {
			t.Fatalf("width %v: %d lines vs %d", w, len(plain), len(rich))
		}
		for i := range plain {
			if runsText(rich[i]) != plain[i] {
				t.Fatalf("width %v line %d: %q vs %q", w, i, runsText(rich[i]), plain[i])
			}
		}
	}
}

// 列の中の行で文字が折り返すと、行の高さは折り返した分まで伸びる。
func TestRowInColumnGrowsWithWrappedText(t *testing.T) {
	long := strings.Repeat("長い文字列", 10)
	a := TextNode(long, WithFontSize(10), WithGrow(1), WithBasis(0))
	b := TextNode("短い", WithFontSize(10), WithGrow(1), WithBasis(0))
	row := Row([]*Spec{a, b})
	after := TextNode("next", WithFontSize(10))
	root := Col([]*Spec{row, after})
	computeLayout(root, 0, 0, 200, 400)
	want := measureTextHeight(long, 10, 100, false)
	if !near(row.Box.Height, want) {
		t.Fatalf("row height = %v, want %v (wrapped)", row.Box.Height, want)
	}
	if after.Box.Y < row.Box.Y+row.Box.Height-1e-9 {
		t.Fatal("next block overlaps the wrapped row")
	}
}

// IR から pptx を書くと、スライドの数だけ slideN.xml がある。
func TestRenderDeckWritesAllSlides(t *testing.T) {
	d := mustReadIR(t, demoDeckPath)
	tm, err := LookupTemplate(d.Template)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "deck.pptx")
	if _, err := tm.RenderDeck(d, out); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	slides := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slides++
		}
	}
	if slides != 3 {
		t.Fatalf("slides in pptx = %d, want 3", slides)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}
