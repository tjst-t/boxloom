package boxloom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pieYAML = "```chart\ncompare: component\nunit: \"%\"\ncategories: [操作, 契約, 障害, その他]\nseries:\n  - name: 内訳\n    values: [46, 27, 18, 9]\n```\n"

func pieSub(t *testing.T) *ChartSubsystem {
	t.Helper()
	sub, err := NewChartSubsystem(json.RawMessage(`{"compare":"component","unit":"%","categories":["操作","契約","障害","その他"],"series":[{"name":"内訳","values":[46,27,18,9]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	return sub.(*ChartSubsystem)
}

// 円グラフは高さで直径が決まるので、その高さで使う幅を横の上限として申告する(#53)。
// 図の地(カード)も円の幅に縮み、余りは地の外に残る。
func TestPieDeclaresWidthForHeight(t *testing.T) {
	s := pieSub(t)
	w := s.Measure(Constraint{Height: 300}).Max.Width
	if w <= 0 || w >= 900 {
		t.Fatalf("pie max width at height 300 = %.1f, want a positive width well below the body width", w)
	}
	if w2 := s.Measure(Constraint{Height: 200}).Max.Width; w2 >= w {
		t.Errorf("a lower box must give a narrower pie: %.1f (h=200) vs %.1f (h=300)", w2, w)
	}
	node := SubsystemNode(s, WithBackground("FFFFFF"), WithRadius(4))
	node.Box = LBox{X: 10, Y: 20, Width: 900, Height: 300}
	p := NewPPTXBuilder(960, 540)
	renderSpec(RenderContext{Out: p, Roles: newCheckedRoles(mustTemplate("light").Theme)}, node)
	if len(p.Shapes) == 0 || p.Shapes[0].box.Width > w+0.01 || p.Shapes[0].box.X != 10 {
		t.Fatalf("card must follow the pie (x 10, width <= %.1f): %+v", w, p.Shapes[0].box)
	}
}

func checkMD(t *testing.T, src string) (errs, warns []Issue, caps []SlideCapacity) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.bx.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return CheckFileCapacity(path, "light")
}

// グラフは並列のまとまり(columns)の中に置ける。円だけの本文は spare-width になり、
// まとまりの中でまとまりの幅を使い切らないときは、そのブロックを指して警告する。
func TestChartInColumns(t *testing.T) {
	alone := "# 円だけ\n\n" + pieYAML
	_, warns, caps := checkMD(t, alone)
	if !hasWarn(warns, "spare-width", "/slides/0/body/0") {
		t.Errorf("a lone pie must warn spare-width: %+v", warns)
	}
	if caps[0].Blocks[0].RemainingWidth <= 0 {
		t.Errorf("a lone pie must report remainingWidth: %+v", caps[0].Blocks[0])
	}

	cols := func(chartMain bool) string {
		role := ""
		textRole := " {role=main}"
		if chartMain {
			role, textRole = " {role=main}", ""
		}
		return "# 円と読み取り\n\n::::columns\n:::column" + role + "\n" + pieYAML + ":::\n:::column 読み取り" + textRole +
			"\n- 操作の質問が 46% で最も多い\n- 契約と障害を合わせても半分に届かない\n- 操作の手引きを直すのが先\n:::\n::::\n"
	}
	errs, warns, caps := checkMD(t, cols(true))
	if len(errs) > 0 {
		t.Fatalf("errors: %+v", errs)
	}
	p := "/slides/0/body/0/columns/0/body/0"
	if !hasWarn(warns, "spare-width", p) {
		t.Errorf("a pie in the wide (main) column must warn spare-width at %s: %+v", p, warns)
	}
	for _, w := range warns {
		if w.Code == "spare-width" && !strings.Contains(w.Hint, "role=main") {
			t.Errorf("column hint must name role=main: %q", w.Hint)
		}
	}
	if rw := caps[0].Blocks[0].Columns[0].Blocks[0].RemainingWidth; rw <= 0 {
		t.Errorf("column capacity must report remainingWidth, got %.1f", rw)
	}
	_, warns, _ = checkMD(t, cols(false))
	if hasWarn(warns, "spare-width", "") {
		t.Errorf("a pie in the narrow column fills it: %+v", warns)
	}
}

func hasWarn(ws []Issue, code, path string) bool {
	for _, w := range ws {
		if w.Code == code && (path == "" || w.Path == path) {
			return true
		}
	}
	return false
}

// まとまりの中に置けるサブシステムは chart だけ(Markdown・IR の両方)。
func TestOnlyChartInColumns(t *testing.T) {
	src := "# A\n\n::::columns\n:::column\n```process\nsteps:\n  - label: a\n  - label: b\n```\n:::\n:::column\nb\n:::\n::::\n"
	_, errs := mdToAny(t, src)
	if len(errs) == 0 || errs[0].Code != codeMisplaced {
		t.Fatalf("process in a column must be misplaced: %+v", errs)
	}
	ir := `{"boxloom":"0.1","slides":[{"type":"content","title":"A","body":[{"type":"columns","columns":[` +
		`{"body":[{"type":"subsystem","subsystem":"gantt","data":{}}]},{"body":[{"type":"paragraph","content":"b"}]}]}]}]}`
	if _, err := ReadIR("t.json", []byte(ir)); err == nil || !strings.Contains(err.Error(), "cannot hold the gantt subsystem") {
		t.Fatalf("gantt in a column must be refused: %v", err)
	}
}
