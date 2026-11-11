package main

import (
	"bytes"
	"io"
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
