package boxloom

// footer-wrap(#171):フッターの欄(作者・機密区分・頁番号)は幅で折り返さない1行
// (noWrap)に書く。欄の幅に1行で入りきらないとき(作者の名前が長く、頁番号の欄と
// 並べると足りない)、PPTX の文は隣の欄に重なるので、作者に短くするよう知らせる。
// 字を小さくしたり省略記号で切ったりはしない(はみ出しも自動で直さない)。

import (
	"fmt"
	"strings"
)

// footerWrapWarnings は根の下の折り返さない葉のうち、置いた幅に1行で入らないものの警告。
// 文の頭の "footer /meta/author: " で check は文書の頭のその値を指す(warningIssue)。
func footerWrapWarnings(root *Spec) []string {
	var out []string
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if len(out) > 0 {
			return // 1件だけ(作者を短くすれば並びの残りも入る)
		}
		if s.Kind == KindText && s.noWrap {
			need := textNaturalWidth(s)
			if need > s.Box.Width+keepLineEps {
				at := s.irPath
				if !strings.HasPrefix(at, "/meta/") {
					at = "/meta/author" // 頁番号の欄が押されたときも、縮める側は作者の欄
				}
				text := s.Text
				if len(s.Runs) > 0 {
					text = runsText(s.Runs)
				}
				out = append(out, fmt.Sprintf("[FOOTER-WRAP WARNING] footer %s: %q does not fit the footer on one line (needs %.1fpt, has %.1fpt at %.0fpt); shorten it",
					at, text, need, s.Box.Width, s.FontSize))
			}
			return
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}
