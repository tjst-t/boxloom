package boxloom

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"testing"
)

func mustFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func buildBytes(t *testing.T, d *IRDeck, opt BuildOptions) ([]byte, *BuildResult) {
	t.Helper()
	var buf bytes.Buffer
	res, err := Build(&buf, d, opt)
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), res
}

// 公開した入口だけで Markdown → IR → PPTX が通る。
func TestPublicAPIFlow(t *testing.T) {
	const md = "../examples/md/demo-deck.bx.md"
	deck, issues, err := ParseMarkdown(md, mustFile(t, md))
	if err != nil || len(issues) > 0 {
		t.Fatalf("ParseMarkdown: %v %v", issues, err)
	}
	pptx, res := buildBytes(t, deck, BuildOptions{Template: "dark"})
	if res.Template != "dark" || res.Slides != 3 || len(res.Warnings) == 0 {
		t.Errorf("result = %+v", res)
	}
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range zr.File {
		found = found || f.Name == "ppt/presentation.xml"
	}
	if !found {
		t.Error("no ppt/presentation.xml")
	}

	// 同じ文書の IR(JSON)からも同じものができる。
	const ir = "../examples/ir/demo-deck.json"
	d2, err := ReadDeck(ir, mustFile(t, ir))
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := buildBytes(t, d2, BuildOptions{Template: "dark"}); !bytes.Equal(again, pptx) {
		t.Error("Markdown and IR builds differ")
	}
}

func TestPublicAPIErrors(t *testing.T) {
	_, issues, err := ParseMarkdown("x.bx.md", []byte("# 題\n\n:::nope\n文\n:::\n"))
	if err != nil || len(issues) == 0 || issues[0].Line == 0 {
		t.Errorf("issues %v, err %v", issues, err)
	}
	var me MarkdownErrors
	if _, err := ReadDeck("x.bx.md", []byte("# 題\n\n:::nope\n文\n:::\n")); !errors.As(err, &me) {
		t.Errorf("ReadDeck: %v, want MarkdownErrors", err)
	}
	if _, err := ReadIR("x.json", []byte(`{"boxloom":"0.1","slides":[],"nope":1}`)); err == nil {
		t.Error("ReadIR accepted an unknown key")
	}
	d, err := ReadIR("", mustFile(t, demoDeckPath))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Template: "nope"}); !errors.Is(err, ErrUnknownTemplate) || buf.Len() != 0 {
		t.Errorf("Build with unknown template: %v (%d bytes)", err, buf.Len())
	}
	if got := Templates(); len(got) < 2 || got[0] != "dark" {
		t.Errorf("Templates() = %v", got)
	}
}

// Build がテンプレートの字体に替えても、終わったら戻す(デモは既定の字体で測る)。
func TestBuildRestoresFonts(t *testing.T) {
	var before, after bytes.Buffer
	if _, err := WriteDemo(&before, "light"); err != nil {
		t.Fatal(err)
	}
	d, err := ReadIR("", mustFile(t, demoDeckPath))
	if err != nil {
		t.Fatal(err)
	}
	buildBytes(t, d, BuildOptions{Template: "dark"})
	Check(demoDeckPath, mustFile(t, demoDeckPath), CheckOptions{})
	if _, err := WriteDemo(&after, "light"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Error("demo changed after Build")
	}
}
