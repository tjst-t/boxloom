package boxloom

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// スライドの地は全面の四角ではなくスライドの背景。マスターと同じ地なら何も書かず、
// 違う地(区切りの card_alt)ならそのスライドの p:bg に書く。
func TestSlideBackgroundIsNative(t *testing.T) {
	d, err := readDeck("../examples/ir/text-patterns.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"light", "dark"} {
		tm, _ := LookupTemplate(name)
		var buf bytes.Buffer
		if _, err := tm.RenderDeck(d, &buf); err != nil {
			t.Fatal(err)
		}
		files := zipFiles(t, buf.Bytes())
		full := fmt.Sprintf(`<a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/>`, ptToEMU(tm.Values.PageW), ptToEMU(tm.Values.PageH))
		cardAlt := hexColor(tm.Theme["card_alt"])
		bgRe := regexp.MustCompile(`<p:bg>.*?val="(\w+)"`)
		sections := 0
		for i, s := range d.Slides {
			x := files[fmt.Sprintf("ppt/slides/slide%d.xml", i+1)]
			if strings.Contains(x, full) {
				t.Errorf("%s slide %d: full-page shape", name, i+1)
			}
			m := bgRe.FindStringSubmatch(x)
			if s.Type == "section" {
				sections++
				if m == nil || m[1] != cardAlt {
					t.Errorf("%s slide %d: section lacks p:bg %s", name, i+1, cardAlt)
				}
			} else if m != nil {
				t.Errorf("%s slide %d: unexpected p:bg %s", name, i+1, m[1])
			}
		}
		if sections == 0 {
			t.Fatal("deck has no section slide")
		}
	}
}
