package boxloom

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

// Build ははみ出し(layout-overflow)があれば書かずに *OverflowError を返し、その
// Issue は check の layout-overflow の警告と同じ(path・message・hint)。AllowOverflow
// なら作り、警告は Warnings に残る(#124)。
func TestBuildOverflowError(t *testing.T) {
	for _, f := range []string{
		"../examples/md/warnings/layout-overflow.bx.md",
		"../examples/md/warnings/columns-overflow.bx.md",
		"../examples/md/warnings/table-overflow.bx.md",
		"../examples/ir/check-warnings/pairs-overflow.json",
	} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		deck, err := ReadDeck(f, data)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		res, err := Build(&buf, deck, BuildOptions{})
		var oe *OverflowError
		if !errors.As(err, &oe) || !errors.Is(err, ErrLayoutOverflow) || res != nil {
			t.Fatalf("%s: Build = %v, %v; want *OverflowError", f, res, err)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: wrote %d bytes", f, buf.Len())
		}
		var want []Issue
		for _, w := range Check(f, data, CheckOptions{}).Warnings {
			if w.Code == "layout-overflow" {
				want = append(want, w)
			}
		}
		if len(want) == 0 || len(oe.Issues) != len(want) {
			t.Fatalf("%s: %d issues, check has %d", f, len(oe.Issues), len(want))
		}
		for i, is := range oe.Issues {
			w := want[i]
			if is.Path != w.Path || is.Message != w.Message || is.Hint != w.Hint || is.Code != w.Code {
				t.Errorf("%s: issue %d = %+v, check %+v", f, i, is, w)
			}
		}
		res, err = Build(&buf, deck, BuildOptions{AllowOverflow: true})
		if err != nil || buf.Len() == 0 || !hasOverflowWarning(res.Warnings) {
			t.Errorf("%s: AllowOverflow: %v, %d bytes", f, err, buf.Len())
		}
	}
}
