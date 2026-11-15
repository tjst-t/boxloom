package boxloom

import (
	"os"
	"testing"
)

// 評価の資料(examples/eval、#161)の正しい版は、誤りも警告も出さない。
// 誤った版(*-faulty.bx.md)の判定は docs/evaluation.md にある。
func TestEvalExamplesClean(t *testing.T) {
	for _, name := range []string{"proposal", "report", "brief"} {
		path := "../examples/eval/" + name + ".bx.md"
		if _, err := os.Stat(path); err != nil {
			t.Skip(err)
		}
		errs, warns := CheckFile(path, "")
		if len(errs)+len(warns) > 0 {
			t.Errorf("%s: %+v %+v", name, errs, warns)
		}
	}
}
