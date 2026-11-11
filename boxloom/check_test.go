package main

import (
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
		if w.Rule != "fill-ratio" || (w.At != "/slides/1" && w.At != "/slides/2") {
			t.Errorf("unexpected warning: %+v", w)
		}
	}
}

// check-invalid/ の例は、ファイルごとに決まった rule の誤りがちょうど1件出る。
func TestCheckInvalidSemanticExamples(t *testing.T) {
	want := map[string]struct{ rule, at string }{
		"date-order.json":       {"date-order", "/slides/2/body/0/data/groups/0/rows/1/segments/0"},
		"duplicate-id.json":     {"unique-id", "/slides/0/body/0/data/groups/1/rows/1/id"},
		"not-a-real-date.json":  {"date-valid", "/slides/2/body/0/data/milestones/0/at"},
		"outside-span.json":     {"within-span", "/slides/2/body/0/data/milestones/0/at"},
		"unknown-target.json":   {"target-exists", "/slides/2/body/0/data/annotations/0/target"},
		"unknown-template.json": {"template-exists", "/template"},
		"unknown-variant.json":  {"variant-exists", "/slides/1/variant"},
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
		if len(errs) != 1 || errs[0].Rule != w.rule || errs[0].At != w.at {
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
		if errs[0].At == "" || !strings.HasPrefix(errs[0].At, "/") {
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
		rules[e.Rule]++
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

func TestRunCheckExitCodes(t *testing.T) {
	old := os.Stdout
	null, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	os.Stdout = null
	defer func() { os.Stdout = old; null.Close() }()
	if c := runCheck([]string{demoDeckPath}); c != 0 {
		t.Errorf("valid deck: exit %d, want 0", c)
	}
	if c := runCheck([]string{"-strict", demoDeckPath}); c != 1 {
		t.Errorf("-strict with warnings: exit %d, want 1", c)
	}
	if c := runCheck([]string{"../examples/ir/check-invalid/date-order.json"}); c != 1 {
		t.Errorf("invalid deck: exit %d, want 1", c)
	}
}
