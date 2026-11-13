package boxloom

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "testdata/outputs-golden.json を書き直す")

// TestOutputsGolden は examples/ の全 deck の build の PPTX(中の部品ごと)・
// check -format json・schema の出力を、記録した値(sha256)と比べる。
// 組み方の内側を替えても(部品の登録簿 #62 など)出力が1バイトも変わらないことを
// 確かめるための試験。出力を変えるのが目的の変更なら
// `go test -run TestOutputsGolden -update-golden` で書き直す。
func TestOutputsGolden(t *testing.T) {
	got := goldenOutputs(t)
	file := filepath.Join("testdata", "outputs-golden.json")
	if *updateGolden {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (write it with -update-golden)", err)
	}
	var want map[string]string
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	for k, w := range want {
		if g, ok := got[k]; !ok {
			t.Errorf("%s: missing", k)
		} else if g != w {
			t.Errorf("%s: output changed", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: new output (not in %s)", k, file)
		}
	}
}

func goldenOutputs(t *testing.T) map[string]string {
	t.Helper()
	var files []string
	for _, pat := range []string{"../examples/ir/*.json", "../examples/ir/check-*/*.json", "../examples/md/*.bx.md", "../examples/md/warnings/*.bx.md"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no examples")
	}
	sum := func(b []byte) string {
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	out := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(f, "../examples/")
		assets := os.DirFS(filepath.Dir(f))
		for _, tmpl := range []string{"", "dark", "light", "report"} {
			key := name + "@" + tmpl
			// check -format json と同じ書き方。
			r := Check(f, data, CheckOptions{Template: tmpl, Assets: assets})
			var cj bytes.Buffer
			enc := json.NewEncoder(&cj)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(r); err != nil {
				t.Fatal(err)
			}
			out["check "+key] = sum(cj.Bytes())

			deck, err := ReadDeck(f, data)
			if err != nil {
				out["build "+key] = "error: " + err.Error()
				continue
			}
			var pptx bytes.Buffer
			res, err := Build(&pptx, deck, BuildOptions{Template: tmpl, Assets: assets})
			if err != nil {
				out["build "+key] = "error: " + err.Error()
				continue
			}
			out["build "+key+" warnings"] = sum([]byte(strings.Join(res.Warnings, "\n")))
			zr, err := zip.NewReader(bytes.NewReader(pptx.Bytes()), int64(pptx.Len()))
			if err != nil {
				t.Fatal(err)
			}
			for _, zf := range zr.File {
				rc, err := zf.Open()
				if err != nil {
					t.Fatal(err)
				}
				var part bytes.Buffer
				if _, err := part.ReadFrom(rc); err != nil {
					t.Fatal(err)
				}
				rc.Close()
				out["build "+key+" "+zf.Name] = sum(part.Bytes())
			}
		}
	}
	for _, tmpl := range append([]string{""}, TemplateNames...) {
		b, err := Schema(tmpl)
		if err != nil {
			t.Fatal(err)
		}
		out["schema @"+tmpl] = sum(b)
	}
	return out
}
