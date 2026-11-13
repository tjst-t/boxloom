package boxloom

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

// columns の flow(左のまとまりが右につながる。Backlog #84)と、report の列の組み方
// (見出しの罫線・列の間の縦線・flow の丸い矢印)の試験。

const columnsFlowExample = "../examples/md/columns-flow.bx.md"

// flowSlides は例を tm で組み、並べて描いたスライドごとの (columns の行, 図形) を返す。
func flowSlides(t *testing.T, name string) (*Template, [][2]any) {
	t.Helper()
	data, err := os.ReadFile(columnsFlowExample)
	if err != nil {
		t.Fatal(err)
	}
	deck, issues, err := ParseMarkdown("columns-flow.bx.md", data)
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	tm, err := LookupTemplate(name)
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	var out [][2]any
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		p := NewPPTXBuilder(tm.Values.PageW, tm.Values.PageH)
		roles := newCheckedRoles(tm.Theme)
		renderSlide(RenderContext{Out: p, Roles: roles}, b.Root)
		if m := roles.Missing(); len(m) > 0 {
			t.Fatalf("%s: missing roles %v", name, m)
		}
		out = append(out, [2]any{findColumnsRow(b.Root), p.Shapes})
	}
	return tm, out
}

func findColumnsRow(s *Spec) *Spec {
	if s.columns != nil {
		return s
	}
	for _, c := range s.Children {
		if r := findColumnsRow(c); r != nil {
			return r
		}
	}
	return nil
}

// flow は IR・Markdown・Schema で読める。書かなければ false で、IR にも出ない。
func TestColumnsFlowReads(t *testing.T) {
	data, err := os.ReadFile(columnsFlowExample)
	if err != nil {
		t.Fatal(err)
	}
	js, issues := MarkdownToIR("columns-flow.bx.md", data)
	if len(issues) > 0 {
		t.Fatalf("%+v", issues)
	}
	var ir struct {
		Slides []struct {
			Body []map[string]any `json:"body"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(js, &ir); err != nil {
		t.Fatal(err)
	}
	var flows []any
	for _, s := range ir.Slides {
		for _, b := range s.Body {
			if b["type"] == "columns" {
				flows = append(flows, b["flow"])
			}
		}
	}
	if len(flows) != 3 || flows[0] != true || flows[1] != true || flows[2] != nil {
		t.Errorf("flow in the IR = %v, want [true true <nil>]", flows)
	}
	deck, err := ReadIR("columns-flow.json", js)
	if err != nil {
		t.Fatal(err)
	}
	if !deck.Slides[1].Body[0].Flow || deck.Slides[3].Body[0].Flow {
		t.Errorf("ReadIR: flow = %v / %v, want true / false", deck.Slides[1].Body[0].Flow, deck.Slides[3].Body[0].Flow)
	}

	// {flow=false} は書かないのと同じ。ほかの値は誤り。
	const md = "---\ntitle: t\n---\n\n# 見出し\n\n::::columns {flow=%s}\n:::column\nA\n:::\n:::column\nB\n:::\n::::\n"
	if js, issues := MarkdownToIR("f.bx.md", []byte(strings.Replace(md, "%s", "false", 1))); len(issues) > 0 || strings.Contains(string(js), `"flow"`) {
		t.Errorf("flow=false: %+v %s", issues, js)
	}
	_, issues = MarkdownToIR("f.bx.md", []byte(strings.Replace(md, "%s", "yes", 1)))
	if len(issues) != 1 || issues[0].Code != codeInvalidValue || !strings.HasSuffix(issues[0].Path, "/flow") {
		t.Errorf("flow=yes: issues %+v, want one invalid-value at …/flow", issues)
	}

	// IR の flow は真偽だけ。
	bad := `{"type":"columns","flow":"yes","columns":[{"body":[{"type":"paragraph","content":"A"}]},{"body":[{"type":"paragraph","content":"B"}]}]}`
	var blk IRBlock
	if err := json.Unmarshal([]byte(bad), &blk); err == nil {
		t.Error(`IR flow "yes" must be an error`)
	}

	// Schema(schema/ と埋め込みの写し)に flow がある。
	for _, f := range []string{"../schema/core.schema.json", "schemas/core.schema.json"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var sc struct {
			Defs map[string]struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(b, &sc); err != nil {
			t.Fatal(err)
		}
		if sc.Defs["columns"].Properties["flow"].Type != "boolean" {
			t.Errorf("%s: columns.flow is not a boolean", f)
		}
	}
	// schema コマンドの囲みの一覧にも属性 flow がある。
	for _, d := range defaultBlocks() {
		if d.Name == "columns" && !strings.Contains(strings.Join(d.Attributes, ","), "flow") {
			t.Errorf("schema: ::::columns does not list the attribute flow")
		}
	}
}

// 標準の組み方(dark・light):flow のときだけ、隙間の縦の真ん中に基調色の小さい矢印。
func TestColumnsFlowStandardArrow(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		tm, slides := flowSlides(t, name)
		accent := tm.Theme["accent"]
		for si, want := range map[int]int{1: 1, 2: 2, 3: 0} {
			row := slides[si][0].(*Spec)
			var arrows []shape
			for _, s := range slides[si][1].([]shape) {
				if s.kind == skShape && s.prst == "rightArrow" {
					arrows = append(arrows, s)
				}
				if s.kind == skLine {
					t.Errorf("%s slide %d: the standard columns draw no rules", name, si+1)
				}
			}
			if len(arrows) != want {
				t.Fatalf("%s slide %d: %d arrows, want %d", name, si+1, len(arrows), want)
			}
			cols := row.columns.cols
			for i, a := range arrows {
				left, right := cols[i].spec.Box, cols[i+1].spec.Box
				gapL, gapR := left.X+left.Width, right.X
				if !nearPt(right.X-gapL, tm.Values.ColumnGap) {
					t.Errorf("%s: gap %.2f, want columnGap %v", name, right.X-gapL, tm.Values.ColumnGap)
				}
				if a.box.X < gapL || a.box.X+a.box.Width > gapR || !nearPt(a.box.X+a.box.Width/2, (gapL+gapR)/2) ||
					!nearPt(a.box.Y+a.box.Height/2, row.Box.Y+row.Box.Height/2) || a.fill != accent {
					t.Errorf("%s slide %d: arrow %+v not centred in the gap %.1f..%.1f (row %+v) in %s", name, si+1, a.box, gapL, gapR, row.Box, accent)
				}
			}
		}
	}
}

// report:見出しの下の罫線(grid・0.75pt)、列の間の真ん中の縦線(grid)、flow のときは
// 縦線の真ん中に直径 24pt の基調色の丸と地の色の山形(>)。
func TestReportColumnRules(t *testing.T) {
	tm, slides := flowSlides(t, "report")
	grid, accent, bg := tm.Theme["grid"], tm.Theme["accent"], tm.Theme["bg"]
	for si, flow := range map[int]bool{1: true, 2: true, 3: false} {
		row := slides[si][0].(*Spec)
		shapes := slides[si][1].([]shape)
		cols := row.columns.cols
		var heads, dividers, chevrons []shape
		var circles []shape
		for _, s := range shapes {
			switch {
			case s.kind == skLine && nearPt(s.y1, s.y2) && s.lineColor == grid:
				heads = append(heads, s)
			case s.kind == skLine && nearPt(s.x1, s.x2) && s.lineColor == grid:
				dividers = append(dividers, s)
			case s.kind == skLine && s.lineColor == bg:
				chevrons = append(chevrons, s)
			case s.kind == skShape && s.prst == "ellipse":
				circles = append(circles, s)
			}
		}
		if len(heads) != len(cols) || len(dividers) != len(cols)-1 {
			t.Fatalf("slide %d: %d head rules, %d dividers; want %d, %d", si+1, len(heads), len(dividers), len(cols), len(cols)-1)
		}
		for i, h := range heads {
			c := cols[i]
			title := c.title.Children[0].Box
			if h.lineWidthPt != 0.75 || !nearPt(h.x1, c.spec.Box.X) || !nearPt(h.x2, c.spec.Box.X+c.spec.Box.Width) ||
				!(h.y1 > title.Y+title.Height) || !(h.y1 < c.body.Box.Y) {
				t.Errorf("slide %d: head rule %d (%.1f,%.1f)-(%.1f,%.1f) w %.2f not under the heading %+v and above the body %+v",
					si+1, i, h.x1, h.y1, h.x2, h.y2, h.lineWidthPt, title, c.body.Box)
			}
			if leaf := c.title.Children[0]; runColor(leaf) != accent || leaf.FontSize != 16 {
				t.Errorf("slide %d: heading %d = %vpt %s, want 16pt %s", si+1, i, leaf.FontSize, runColor(leaf), accent)
			}
		}
		mids := make([]float64, len(dividers))
		for i, d := range dividers {
			gapL, gapR := cols[i].spec.Box.X+cols[i].spec.Box.Width, cols[i+1].spec.Box.X
			mids[i] = (gapL + gapR) / 2
			if d.lineWidthPt != 0.75 || !nearPt(d.x1, mids[i]) || !nearPt(math.Min(d.y1, d.y2), row.Box.Y) || !nearPt(math.Max(d.y1, d.y2), row.Box.Y+row.Box.Height) {
				t.Errorf("slide %d: divider %d at x %.1f (%.1f..%.1f), want the middle of the gap %.1f over the row %+v", si+1, i, d.x1, d.y1, d.y2, mids[i], row.Box)
			}
		}
		if !flow {
			if len(circles) != 0 || len(chevrons) != 0 {
				t.Errorf("slide %d: no flow, but %d circles and %d chevron strokes", si+1, len(circles), len(chevrons))
			}
			continue
		}
		if len(circles) != len(dividers) || len(chevrons) != 2*len(dividers) {
			t.Fatalf("slide %d: %d circles, %d chevron strokes; want %d, %d", si+1, len(circles), len(chevrons), len(dividers), 2*len(dividers))
		}
		midY := row.Box.Y + row.Box.Height/2
		for i, c := range circles {
			gap := cols[i+1].spec.Box.X - (cols[i].spec.Box.X + cols[i].spec.Box.Width)
			if !nearPt(c.box.Width, 24) || !nearPt(c.box.Height, 24) || c.box.Width > gap+0.01 || c.fill != accent ||
				!nearPt(c.box.X+c.box.Width/2, mids[i]) || !nearPt(c.box.Y+c.box.Height/2, midY) {
				t.Errorf("slide %d: circle %d %+v (%s), want ⌀24 %s centred at (%.1f, %.1f) within the gap %.1f", si+1, i, c.box, c.fill, accent, mids[i], midY, gap)
			}
			// 山形(>):2本の線が丸の中で右の頂点に集まる。
			a, b := chevrons[2*i], chevrons[2*i+1]
			inside := func(x, y float64) bool { return math.Hypot(x-mids[i], y-midY) < 12 }
			if !nearPt(a.x2, b.x1) || !nearPt(a.y2, b.y1) || !(a.x2 > a.x1) || !(b.x2 < b.x1) || !nearPt(a.y2, midY) ||
				!inside(a.x1, a.y1) || !inside(b.x2, b.y2) {
				t.Errorf("slide %d: chevron %d (%.1f,%.1f)-(%.1f,%.1f)-(%.1f,%.1f) is not a > inside the circle", si+1, i, a.x1, a.y1, a.x2, a.y2, b.x2, b.y2)
			}
		}
	}
}
