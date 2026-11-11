package boxloom

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 別のモジュール(examples/libuse。replace でこのモジュールを指す)から
// import して、Markdown から PPTX を書けるか。ネットワークは使わない。
func TestExternalModule(t *testing.T) {
	if testing.Short() {
		t.Skip("builds another module")
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not found")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "libuse")
	env := append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOTOOLCHAIN=local", "GOWORK=off")
	build := exec.Command(gobin, "build", "-o", bin, ".")
	build.Dir = "examples/libuse"
	build.Env = env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build examples/libuse: %v\n%s", err, out)
	}

	src, err := filepath.Abs("../examples/md/demo-deck.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	pptx := filepath.Join(dir, "demo-deck.pptx")
	run := exec.Command(bin, "-template", "dark", src, pptx)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("libuse: %v\n%s", err, out)
	}
	zr, err := zip.OpenReader(pptx)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "ppt/presentation.xml" {
			return
		}
	}
	t.Error("no ppt/presentation.xml in the PPTX")
}
