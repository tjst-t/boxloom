package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
)

// slideMarks は PPTX のスライドごとの、名前 boxloom-overflow の図形の数と文。
func slideMarks(t *testing.T, pptx []byte) (marks map[int]int, labels map[int]string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	marks, labels = map[int]int{}, map[int]string{}
	for _, f := range zr.File {
		var n int
		if _, err := fmt.Sscanf(f.Name, "ppt/slides/slide%d.xml", &n); err != nil || strings.Contains(f.Name, "_rels") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		s := string(b)
		if c := strings.Count(s, `name="`+overflowMarkName+`"`); c > 0 {
			marks[n] = c
			if i := strings.Index(s, "ここから "); i >= 0 {
				labels[n] = s[i : i+strings.Index(s[i:], "<")]
			}
		}
	}
	return marks, labels
}

// -allow-overflow で作ると、はみ出したスライドにだけ印(線と文の2つの図形)が付く。
// はみ出さない文書は AllowOverflow の有無でバイト列が変わらない(#141)。
func TestOverflowMark(t *testing.T) {
	for _, f := range []string{
		"../examples/md/warnings/layout-overflow.bx.md",
		"../examples/md/warnings/columns-overflow.bx.md",
		"../examples/md/warnings/table-overflow.bx.md",
		"../examples/ir/check-warnings/pairs-overflow.json",
	} {
		marked := 0
		for _, tmpl := range []string{"dark", "light", "report"} {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			deck, err := ReadDeck(f, data)
			if err != nil {
				t.Fatal(err)
			}
			if deck.Meta != nil && deck.Meta.Toc {
				t.Fatalf("%s: the test assumes no table of contents", f)
			}
			var buf bytes.Buffer
			res, err := Build(&buf, deck, BuildOptions{Template: tmpl, AllowOverflow: true})
			if err != nil {
				t.Fatalf("%s@%s: %v", f, tmpl, err)
			}
			want := map[int]bool{}
			for _, w := range res.Warnings {
				var n int
				if strings.Contains(w, "[LAYOUT-OVERFLOW WARNING]") {
					if _, err := fmt.Sscanf(w, "slide %d:", &n); err == nil {
						want[n] = true
					}
				}
			}
			marks, labels := slideMarks(t, buf.Bytes())
			var got, exp []int
			for n, c := range marks {
				got = append(got, n)
				if c != 2 {
					t.Errorf("%s@%s slide %d: %d marks, want 2 (line and label)", f, tmpl, n, c)
				}
				if !strings.HasSuffix(labels[n], " pt はみ出し") {
					t.Errorf("%s@%s slide %d: label %q", f, tmpl, n, labels[n])
				}
			}
			for n := range want {
				exp = append(exp, n)
			}
			sort.Ints(got)
			sort.Ints(exp)
			if fmt.Sprint(got) != fmt.Sprint(exp) {
				t.Errorf("%s@%s: marks on slides %v, overflow on slides %v", f, tmpl, got, exp)
			}
			marked += len(got)
		}
		if marked == 0 {
			t.Errorf("%s: no slide overflows in any template", f)
		}
	}
	for _, f := range []string{"../examples/md/blocks.bx.md", "../examples/ir/demo-deck.json"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		deck, err := ReadDeck(f, data)
		if err != nil {
			t.Fatal(err)
		}
		assets := os.DirFS("../examples/md")
		if strings.HasSuffix(f, ".json") {
			assets = os.DirFS("../examples/ir")
		}
		var a, b bytes.Buffer
		if _, err := Build(&a, deck, BuildOptions{Assets: assets}); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if _, err := Build(&b, deck, BuildOptions{Assets: assets, AllowOverflow: true}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("%s: AllowOverflow changed a deck that fits", f)
		}
		if m, _ := slideMarks(t, b.Bytes()); len(m) > 0 {
			t.Errorf("%s: marks %v on a deck that fits", f, m)
		}
	}
}
