package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
	src = bytes.Replace(src, []byte(`"blockGap": 16`), []byte(`"blockGap": 13`), 1)
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
