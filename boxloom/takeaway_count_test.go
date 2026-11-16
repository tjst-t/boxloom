package boxloom

import (
	"strings"
	"testing"
)

// 1枚の本文に takeaway が2つあれば takeaway-count の警告が1件だけ出る(#165)。path は
// 2つ目の takeaway(.md なら その :::takeaway の行)。takeaway が1つのスライドには出ず、
// 誤りにはならない。例は架空の examples/md/warnings/takeaway-count.bx.md と、それを
// parse した examples/ir/check-warnings/takeaway-count.json。
func TestTakeawayCountWarning(t *testing.T) {
	for _, f := range []string{"../examples/md/warnings/takeaway-count.bx.md", "../examples/ir/check-warnings/takeaway-count.json"} {
		for _, tpl := range []string{"light", "dark", "report"} {
			errs, warns := CheckFile(f, tpl)
			if len(errs) > 0 {
				t.Errorf("%s %s: errors %+v", f, tpl, errs)
			}
			var got []Issue
			for _, w := range warns {
				if w.Code == codeTakeawayCount {
					got = append(got, w)
				}
			}
			if len(got) != 1 || got[0].Path != "/slides/1/body/2" || got[0].Hint == "" ||
				!strings.Contains(got[0].Message, "2 takeaways") {
				t.Errorf("%s %s: takeaway-count = %+v", f, tpl, got)
				continue
			}
			if strings.HasSuffix(f, ".md") && got[0].Line != 21 {
				t.Errorf("%s %s: line = %d, want 21", f, tpl, got[0].Line)
			}
		}
	}
}
