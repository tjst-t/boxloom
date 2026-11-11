package boxloom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 正しい例は誤り0件。警告は出てよい(例のデッキは本文が少ない)。
func TestCheckValidExamples(t *testing.T) {
	files, _ := filepath.Glob("../examples/ir/*.json")
	if len(files) == 0 {
		t.Fatal("no examples")
	}
	for _, f := range files {
		errs, _ := CheckFile(f, "")
		if len(errs) != 0 {
			t.Errorf("%s: unexpected errors: %+v", filepath.Base(f), errs)
		}
	}
}

// 組み立てまで通したときの警告は rule と場所つきで出る。
func TestCheckReportsBuildWarnings(t *testing.T) {
	_, warns := CheckFile(demoDeckPath, "")
	if len(warns) != 2 {
		t.Fatalf("want 2 warnings, got %+v", warns)
	}
	for _, w := range warns {
		if w.Code != "fill-ratio" || (w.Path != "/slides/1" && w.Path != "/slides/2") {
			t.Errorf("unexpected warning: %+v", w)
		}
	}
}

// check-invalid/ の例は、ファイルごとに決まった rule の誤りがちょうど1件出る。
func TestCheckInvalidSemanticExamples(t *testing.T) {
	want := map[string]struct{ rule, at string }{
		"date-order.json":               {"date-order", "/slides/2/body/0/data/groups/0/rows/1/segments/0"},
		"duplicate-id.json":             {"unique-id", "/slides/0/body/0/data/groups/1/rows/1/id"},
		"not-a-real-date.json":          {"date-valid", "/slides/2/body/0/data/milestones/0/at"},
		"outside-span.json":             {"within-span", "/slides/2/body/0/data/milestones/0/at"},
		"quote-empty-source.json":       {"empty-text", "/slides/6/body/0/source"},
		"shapes-out-of-frame.json":      {"shape-bounds", "/slides/1/body/1/data/items/6/w"},
		"shapes-unknown-connector.json": {"target-exists", "/slides/1/body/1/data/items/12/to"},
		"shapes-unknown-role.json":      {"role-exists", "/slides/1/body/1/data/items/7/fill"},
		"matrix-cells.json":             {"matrix-cells", "/slides/3/body/0/data/rows/1/cells"},
		"matrix-emphasis.json":          {"matrix-emphasis", "/slides/3/body/0/data/emphasis"},
		"chart-values.json":             {"chart-values", "/slides/2/body/0/data/series/0/values"},
		"chart-emphasis.json":           {"chart-emphasis", "/slides/5/body/0/data/emphasis/0"},
		"unknown-target.json":           {"target-exists", "/slides/2/body/0/data/annotations/0/target"},
		"unknown-template.json":         {"template-exists", "/template"},
		"unknown-variant.json":          {"variant-exists", "/slides/1/variant"},
		"table-shape.json":              {"table-shape", "/slides/1/body/0/rows/1"},
		"table-total-shape.json":        {"table-shape", "/slides/1/body/0/total"},
		"image-missing.json":            {"image-missing", "/slides/1/body/0/src"},
		"image-format.json":             {"image-format", "/slides/1/body/0/src"},
	}
	files, _ := filepath.Glob("../examples/ir/check-invalid/*.json")
	if len(files) != len(want) {
		t.Fatalf("found %d examples, want %d (update the table)", len(files), len(want))
	}
	for _, f := range files {
		name := filepath.Base(f)
		w, ok := want[name]
		if !ok {
			t.Errorf("%s: not in the table", name)
			continue
		}
		errs, _ := CheckFile(f, "")
		if len(errs) != 1 || errs[0].Code != w.rule || errs[0].Path != w.at {
			t.Errorf("%s: want one %s at %s, got %+v", name, w.rule, w.at, errs)
		}
	}
}

// Schema で断られる例(invalid/)も、check は全部断る。
func TestCheckRejectsSchemaInvalidExamples(t *testing.T) {
	files, _ := filepath.Glob("../examples/ir/invalid/*.json")
	for _, f := range files {
		errs, _ := CheckFile(f, "")
		if len(errs) == 0 {
			t.Errorf("%s: accepted", filepath.Base(f))
			continue
		}
		if errs[0].Path == "" || !strings.HasPrefix(errs[0].Path, "/") {
			t.Errorf("%s: error should carry a location: %+v", filepath.Base(f), errs[0])
		}
	}
}

// 誤りは最初の1件で止めず、全部集める。
func TestCheckCollectsAllErrors(t *testing.T) {
	src := `{"boxloom":"0.1","template":"light","slides":[
	  {"type":"content","title":"t","variant":"dense","body":[
	    {"type":"subsystem","subsystem":"gantt","data":{
	      "span":{"from":"2026-01","to":"2027-01"},
	      "groups":[
	        {"id":"a","label":"g","rows":[{"id":"a","label":"r","segments":[
	          {"from":"2026-06","to":"2026-03","status":"support"},
	          {"from":"2026-06","to":"2028-01","status":"none"}]}]}],
	      "annotations":[{"at":"2026-13","text":"x"},{"at":"2026-05","target":"zzz","text":"y"}]}}]}]}`
	errs, _ := CheckIR("x.json", []byte(src), "")
	rules := map[string]int{}
	for _, e := range errs {
		rules[e.Code]++
	}
	for _, r := range []string{"unique-id", "variant-exists", "date-order", "within-span", "target-exists"} {
		if rules[r] == 0 {
			t.Errorf("missing rule %s in %+v", r, errs)
		}
	}
}

// -template は IR の template より優先する。
func TestCheckTemplateOverride(t *testing.T) {
	errs, _ := CheckFile("../examples/ir/check-invalid/unknown-template.json", "dark")
	if len(errs) != 0 {
		t.Fatalf("override should win: %+v", errs)
	}
}

func TestLocationToPointer(t *testing.T) {
	cases := map[string]string{
		"slides[2]: body[0]: gantt: groups[0].rows[1].segments[0].status: unknown status": "/slides/2/body/0/data/groups/0/rows/1/segments/0/status",
		`slides[1]: body[2]: json: unknown field "align"`:                                 "/slides/1/body/2/align",
		"boxloom: unsupported IR version":                                                 "",
	}
	for in, want := range cases {
		if got := locationToPointer(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestCheckOK(t *testing.T) {
	read := func(p string) []byte {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	demo := read(demoDeckPath)
	if r := Check(demoDeckPath, demo, CheckOptions{}); !r.OK || len(r.Warnings) == 0 || len(r.Capacity) == 0 {
		t.Errorf("valid deck: ok %v, %d warning(s), %d capacity", r.OK, len(r.Warnings), len(r.Capacity))
	}
	if r := Check(demoDeckPath, demo, CheckOptions{Strict: true}); r.OK {
		t.Errorf("strict with warnings: ok")
	}
	bad := "../examples/ir/check-invalid/date-order.json"
	if r := Check(bad, read(bad), CheckOptions{}); r.OK || len(r.Errors) == 0 || r.Capacity == nil {
		t.Errorf("invalid deck: %+v", r)
	}
}

// 注釈の重なりの警告は、最小の大きさの警告と別の rule になる。
func TestWarningIssueRules(t *testing.T) {
	cases := map[string]string{
		`slide 3: [SUBSYSTEM WARNING] gantt: needs at least 400.0pt height, got 300.0pt`:                                                            "subsystem-min",
		`slide 3: [SUBSYSTEM WARNING] gantt: annotation 9 ("x") has no free place and overlaps another annotation`:                                  "annotation-overlap",
		`slide 3: [SUBSYSTEM WARNING] gantt: milestones[2] ("a") overlaps milestones[0] ("b"): the dates are too close to fit the labels in 2 rows`: "milestone-overlap",
		`slide 2: [FILL-RATIO WARNING] underfilled: 50.8% (< 65%)`:                                                                                  "fill-ratio",
	}
	for w, want := range cases {
		if got := warningIssue("f", w).Code; got != want {
			t.Errorf("%q: rule = %q, want %q", w, got, want)
		}
	}
}

// .md を検査すると、意味の誤りにも IR の場所から引いた Markdown の行が付く。
func TestCheckMarkdownAddsLines(t *testing.T) {
	src, err := os.ReadFile("../examples/md/gantt-full.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(src), "  - {at: 2028-05, label: ライセンス更新}", "  - {at: 2036-05, label: ライセンス更新}", 1)
	path := filepath.Join(t.TempDir(), "bad.bx.md")
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	errs, _ := CheckFile(path, "")
	if len(errs) != 1 || errs[0].Code != "within-span" {
		t.Fatalf("errs = %+v, want one within-span", errs)
	}
	want := 0
	for i, l := range strings.Split(s, "\n") {
		if strings.Contains(l, "2036-05") {
			want = i + 1
		}
	}
	if errs[0].Line != want || errs[0].Path != "/slides/0/body/0/data/milestones/0/at" {
		t.Fatalf("line/path = %d %s, want %d", errs[0].Line, errs[0].Path, want)
	}
}

// .md の記法の誤りは、パーサーの誤り(行と列つき)がそのまま出る。
func TestCheckMarkdownSyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raw.bx.md")
	os.WriteFile(path, []byte("---\ntitle: x\n---\n\n# A\n\n<b>raw</b>\n"), 0o644)
	errs, _ := CheckFile(path, "")
	if len(errs) != 1 || errs[0].Code != "raw-html" || errs[0].Line != 7 || errs[0].Column != 1 {
		t.Fatalf("errs = %+v", errs)
	}
}

// IR(JSON)を直接検査したときは line・column を出さず、path と code は必ず出す。
func TestCheckJSONOmitsLineAndColumn(t *testing.T) {
	errs, _ := CheckFile("../examples/ir/check-invalid/date-order.json", "")
	b, _ := json.Marshal(errs[0])
	var m map[string]any
	json.Unmarshal(b, &m)
	if _, ok := m["line"]; ok {
		t.Errorf("line should be omitted: %s", b)
	}
	if _, ok := m["column"]; ok {
		t.Errorf("column should be omitted: %s", b)
	}
	if m["path"] == nil || m["code"] != "date-order" {
		t.Errorf("path/code missing: %s", b)
	}
}
