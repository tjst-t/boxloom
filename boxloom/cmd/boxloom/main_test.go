package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

const demoDeckPath = "../../../examples/ir/demo-deck.json"

func TestCheckExitCodes(t *testing.T) {
	if c := run([]string{"check", demoDeckPath}, io.Discard, io.Discard); c != 0 {
		t.Errorf("valid deck: exit %d, want 0", c)
	}
	if c := run([]string{"check", "-strict", demoDeckPath}, io.Discard, io.Discard); c != 1 {
		t.Errorf("-strict with warnings: exit %d, want 1", c)
	}
	if c := run([]string{"check", "../../../examples/ir/check-invalid/date-order.json"}, io.Discard, io.Discard); c != 1 {
		t.Errorf("invalid deck: exit %d, want 1", c)
	}
	if c := run([]string{"check", "-format", "xml", demoDeckPath}, io.Discard, io.Discard); c != 2 {
		t.Errorf("bad format: exit %d, want 2", c)
	}
}

func TestSchemaUnknownTemplate(t *testing.T) {
	var errb bytes.Buffer
	if c := run([]string{"schema", "-template", "nope"}, io.Discard, &errb); c != 1 {
		t.Errorf("unknown template: exit %d, want 1", c)
	}
	if !bytes.HasPrefix(errb.Bytes(), []byte("boxloom schema: unknown template")) {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestBuildToFile(t *testing.T) {
	out := t.TempDir() + "/x.pptx"
	var stdout bytes.Buffer
	if c := run([]string{"build", "-o", out, "../../../examples/md/demo-deck.bx.md"}, &stdout, io.Discard); c != 0 {
		t.Fatalf("exit %d", c)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("saved: "+out+" (3 slides, template light)")) {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestDeckBaseName(t *testing.T) {
	if deckBaseName("a/demo-deck.bx.md") != "demo-deck" || deckBaseName("x.json") != "x" {
		t.Fatal("deckBaseName")
	}
}

// -template にファイルのパスを渡すと、その値の表で作る。誤ったファイルは
// 5章の形の誤り(終了コード 1)。
func TestTemplateFileFlag(t *testing.T) {
	const forest = "../../../examples/templates/forest.json"
	out := t.TempDir() + "/x.pptx"
	var stdout bytes.Buffer
	if c := run([]string{"build", "-template", forest, "-o", out, "../../../examples/md/demo-deck.bx.md"}, &stdout, io.Discard); c != 0 {
		t.Fatalf("build exit %d", c)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("template forest)")) {
		t.Errorf("stdout = %q", stdout.String())
	}
	if c := run([]string{"check", "-template", forest, demoDeckPath}, io.Discard, io.Discard); c != 0 {
		t.Errorf("check exit %d", c)
	}
	var sch bytes.Buffer
	if c := run([]string{"schema", "-template", forest}, &sch, io.Discard); c != 0 || !bytes.Contains(sch.Bytes(), []byte(`"name": "forest"`)) {
		t.Errorf("schema exit %d", c)
	}
	bad := t.TempDir() + "/bad.json"
	src, _ := os.ReadFile(forest)
	src = bytes.Replace(src, []byte(`"name": "forest"`), []byte(`"name": "bad"`), 1)
	src = bytes.Replace(src, []byte(`"blockGap": 20`), []byte(`"blockGap": 13`), 1)
	if err := os.WriteFile(bad, src, 0o644); err != nil {
		t.Fatal(err)
	}
	var co bytes.Buffer
	if c := run([]string{"check", "-format", "json", "-template", bad, demoDeckPath}, &co, io.Discard); c != 1 {
		t.Fatalf("bad template: exit %d", c)
	}
	var res struct {
		Errors []struct{ File, Path, Code string } `json:"errors"`
	}
	if err := json.Unmarshal(co.Bytes(), &res); err != nil || len(res.Errors) != 1 || res.Errors[0].Path != "/values/blockGap" || res.Errors[0].Code != "template-values" || res.Errors[0].File != bad {
		t.Fatalf("errors = %+v (%v)", res.Errors, err)
	}
	var be bytes.Buffer
	if c := run([]string{"build", "-template", bad, "-o", out, demoDeckPath}, io.Discard, &be); c != 1 || !bytes.Contains(be.Bytes(), []byte("/values/blockGap: template-values")) {
		t.Fatalf("build bad template: exit %d, stderr %q", c, be.String())
	}
}

// parse・check・build は YAML の deck(.yaml)を受け取る。parse の出力は
// 同じ内容の JSON の IR と同じ、build の PPTX は JSON から組んだものと同じ。
func TestYAMLDeckCommands(t *testing.T) {
	const y, j = "../../../examples/yaml/charts.yaml", "../../../examples/ir/charts.json"
	var ir bytes.Buffer
	if c := run([]string{"parse", y}, &ir, io.Discard); c != 0 {
		t.Fatalf("parse: exit %d", c)
	}
	want, err := os.ReadFile(j)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if json.Compact(&a, ir.Bytes()) != nil || json.Compact(&b, want) != nil || a.String() != b.String() {
		t.Errorf("parse output differs from %s", j)
	}
	if c := run([]string{"check", y}, io.Discard, io.Discard); c != 0 {
		t.Errorf("check: exit %d", c)
	}
	dir := t.TempDir()
	if c := run([]string{"build", "-o", dir + "/y.pptx", y}, io.Discard, io.Discard); c != 0 {
		t.Fatalf("build yaml: exit %d", c)
	}
	if c := run([]string{"build", "-o", dir + "/j.pptx", j}, io.Discard, io.Discard); c != 0 {
		t.Fatalf("build json: exit %d", c)
	}
	py, _ := os.ReadFile(dir + "/y.pptx")
	pj, _ := os.ReadFile(dir + "/j.pptx")
	if !bytes.Equal(py, pj) {
		t.Errorf("PPTX from YAML differs from PPTX from JSON")
	}
	if deckBaseName("a/charts.yaml") != "charts" || deckBaseName("a/x.yml") != "x" {
		t.Errorf("deckBaseName for YAML")
	}
}

func TestLayoutCommand(t *testing.T) {
	var stdout bytes.Buffer
	if c := run([]string{"layout", "-template", "dark", demoDeckPath}, &stdout, io.Discard); c != 0 {
		t.Fatalf("exit %d", c)
	}
	var out struct {
		Format   string `json:"format"`
		Template string `json:"template"`
		Slides   []struct {
			Path string `json:"path"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Format != "boxloom-layout/0.1" || out.Template != "dark" || len(out.Slides) == 0 || out.Slides[0].Path != "/slides/0" {
		t.Errorf("layout output = %+v", out)
	}
	if c := run([]string{"layout"}, io.Discard, io.Discard); c != 2 {
		t.Errorf("no deck: exit %d, want 2", c)
	}
	if c := run([]string{"layout", "-template", "nope", demoDeckPath}, io.Discard, io.Discard); c != 1 {
		t.Errorf("unknown template: exit %d, want 1", c)
	}
}

// `boxloom templates` は同梱したテンプレートの名前を1行に1つ書く(report を含む)。
func TestTemplatesCommand(t *testing.T) {
	var out strings.Builder
	if c := run([]string{"templates"}, &out, io.Discard); c != 0 {
		t.Fatalf("exit %d", c)
	}
	if out.String() != "dark\nlight\nreport\n" {
		t.Errorf("templates = %q", out.String())
	}
	if c := run([]string{"templates", "x"}, io.Discard, io.Discard); c != 2 {
		t.Errorf("extra argument: exit %d", c)
	}
}

// はみ出す文書の build は誤り(終了コード 1・pptx を作らない)で、hint を stderr に出す。
// -allow-overflow なら今までどおり作り、警告を出す。check は警告のままで、要約の行に
// build を止める件数を出す(#124)。
func TestBuildOverflow(t *testing.T) {
	const deck = "../../../examples/md/warnings/layout-overflow.bx.md"
	out := t.TempDir() + "/x.pptx"
	var stdout, stderr bytes.Buffer
	if c := run([]string{"build", "-o", out, deck}, &stdout, &stderr); c != 1 {
		t.Fatalf("overflow: exit %d, want 1", c)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("overflow: %s was written (err %v)", out, err)
	}
	if s := stderr.String(); !strings.Contains(s, "error: "+deck) || !strings.Contains(s, "layout-overflow") || !strings.Contains(s, "-allow-overflow") {
		t.Errorf("stderr = %q", s)
	}
	stdout.Reset()
	if c := run([]string{"build", "-allow-overflow", "-o", out, deck}, &stdout, io.Discard); c != 0 {
		t.Fatalf("-allow-overflow: exit %d", c)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("-allow-overflow: %v", err)
	}
	if !strings.Contains(stdout.String(), "[LAYOUT-OVERFLOW WARNING]") {
		t.Errorf("-allow-overflow: no warning in %q", stdout.String())
	}
	stdout.Reset()
	if c := run([]string{"check", deck}, &stdout, io.Discard); c != 0 {
		t.Errorf("check: exit %d, want 0 (overflow stays a warning)", c)
	}
	if !strings.Contains(stdout.String(), "layout-overflow warning(s) stop build") {
		t.Errorf("check summary = %q", stdout.String())
	}
	stdout.Reset()
	run([]string{"check", demoDeckPath}, &stdout, io.Discard)
	if strings.Contains(stdout.String(), "stop build") {
		t.Errorf("check of a deck that fits mentions stop build: %q", stdout.String())
	}
}
