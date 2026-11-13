package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"path"
	"regexp"
	"strings"
	"testing"
)

// 字体の実名(#111):LibreOffice はテーマの参照(+mn-ea)を和文で解決せず別の字体で
// 描く。出力の文字はどれも、継ぐ先(run の a:rPr → 表のセルの lstStyle → プレースホルダー
// なら置き場の lstStyle、そうでなければ presentation.xml の defaultTextStyle)をたどると
// テンプレートの字体の実名に行き着く。テーマの a:latin・a:ea はそのまま書く。
func TestTextResolvesToNamedFonts(t *testing.T) {
	decks := []string{
		"../examples/md/text-patterns.bx.md", // 表紙(ctrTitle・subTitle)・題・フッター
		"../examples/md/table-groups.bx.md",  // 表
		"../examples/md/charts.bx.md",        // グラフ
	}
	for _, name := range []string{"dark", "light", "report"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		lat, ea := tm.Fonts.Latin.Family, tm.Fonts.EastAsian.Family
		for _, deck := range decks {
			d, err := readDeck(deck)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := tm.RenderDeck(d, &buf); err != nil {
				t.Fatal(err)
			}
			parts := zipParts(t, buf.Bytes())
			label := name + " " + path.Base(deck)

			theme := parts["ppt/theme/theme1.xml"]
			if !strings.Contains(theme, `<a:minorFont><a:latin typeface="`+escapeXML(lat)+`"/><a:ea typeface="`+escapeXML(ea)+`"/>`) {
				t.Errorf("%s: theme does not name the fonts", label)
			}
			for n, x := range parts {
				if strings.HasSuffix(n, ".xml") && n != "ppt/theme/theme1.xml" && strings.Contains(x, `typeface="+m`) {
					t.Errorf("%s: %s refers to a theme font (+mn/+mj)", label, n)
				}
			}
			// presentation.xml の文字の既定:9つの段のすべてに実名。
			def := parts["ppt/presentation.xml"]
			for i := 1; i <= 9; i++ {
				lv := between(def, "<a:lvl"+itoa(i)+"pPr>", "</a:lvl"+itoa(i)+"pPr>")
				if fontOf(lv, "latin") != lat || fontOf(lv, "ea") != ea {
					t.Errorf("%s: defaultTextStyle lvl%d = %q", label, i, lv)
				}
			}

			runs := 0
			for n, x := range parts {
				if !regexp.MustCompile(`^ppt/slides/slide\d+\.xml$`).MatchString(n) {
					continue
				}
				layout := parts["ppt/slideLayouts/"+path.Base(regexp.MustCompile(`slideLayouts/(slideLayout\d+\.xml)`).FindString(parts["ppt/slides/_rels/"+path.Base(n)+".rels"]))]
				master := parts["ppt/slideMasters/slideMaster1.xml"]
				for _, sp := range regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>|<p:graphicFrame>.*?</p:graphicFrame>`).FindAllString(x, -1) {
					// 表のセルは、セルの lstStyle から継ぐ。
					for _, tc := range regexp.MustCompile(`(?s)<a:tc[ >].*?</a:tc>`).FindAllString(sp, -1) {
						lst := between(tc, "<a:lstStyle>", "</a:lstStyle>")
						for _, r := range runsOf(tc) {
							runs++
							checkRun(t, label, n, r, lst, lat, ea)
						}
					}
					if strings.HasPrefix(sp, "<p:graphicFrame>") {
						continue
					}
					inherit := between(def, "<a:lvl1pPr>", "</a:lvl1pPr>")
					if ph := regexp.MustCompile(`<p:ph type="(\w+)"`).FindStringSubmatch(sp); ph != nil {
						inherit = phLstStyle(layout, ph[1])
						if inherit == "" {
							inherit = phLstStyle(master, ph[1])
						}
						if inherit == "" {
							t.Errorf("%s %s: no placeholder %s in the layout or master", label, n, ph[1])
						}
					}
					for _, r := range runsOf(sp) {
						runs++
						checkRun(t, label, n, r, inherit, lat, ea)
					}
				}
			}
			if runs == 0 {
				t.Errorf("%s: no runs", label)
			}
			// グラフの文字は run の側に実名を書く(chartxml.go)。
			for n, x := range parts {
				if strings.HasPrefix(n, "ppt/charts/chart") && strings.HasSuffix(n, ".xml") && !strings.Contains(x, `<a:ea typeface="`+escapeXML(ea)+`"/>`) {
					t.Errorf("%s: %s does not name the east asian font", label, n)
				}
			}
		}
	}
}

// checkRun は run の字体を、run の a:rPr か継ぐ先(inherit)から引いて確かめる。
func checkRun(t *testing.T, label, part, r, inherit, lat, ea string) {
	t.Helper()
	for _, k := range []struct{ tag, want string }{{"latin", lat}, {"ea", ea}} {
		got := fontOf(r, k.tag)
		if got == "" {
			got = fontOf(inherit, k.tag)
		}
		if got != k.want {
			t.Errorf("%s %s: run %.80q resolves a:%s to %q, want %q", label, part, r, k.tag, got, k.want)
		}
	}
}

func runsOf(x string) []string {
	return regexp.MustCompile(`(?s)<a:r>.*?</a:r>`).FindAllString(x, -1)
}

// phLstStyle は置き場(type)の lstStyle の中身。
func phLstStyle(part, typ string) string {
	for _, sp := range regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(part, -1) {
		if strings.Contains(sp, `<p:ph type="`+typ+`"`) {
			return between(sp, "<a:lstStyle>", "</a:lstStyle>")
		}
	}
	return ""
}

func fontOf(x, tag string) string {
	m := regexp.MustCompile(`<a:` + tag + ` typeface="([^"]*)"`).FindStringSubmatch(x)
	if m == nil {
		return ""
	}
	return strings.ReplaceAll(m[1], "&amp;", "&")
}

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], b)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

func zipParts(t *testing.T, b []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		c, _ := io.ReadAll(r)
		r.Close()
		parts[f.Name] = string(c)
	}
	return parts
}
