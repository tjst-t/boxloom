package boxloom

// footer-wrap(#171):フッターの欄は折り返さない1行。長い作者名なら check の警告が
// 文書に1件(作者の行を指す)、PPTX のフッターの文はどの枚でもフッターの枠(1行の高さ)
// から下にはみ出さない。3つのテンプレートで確かめる。

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func footerDoc(author string) string {
	return "---\ntitle: 例\nauthor: " + author + "\nconfidential: 社外秘\n---\n\n# 一枚目\n\n- 箇条\n\n# 二枚目\n\n- 箇条\n\n# 三枚目\n\n- 箇条\n"
}

// footerShapes は1枚のスライドの XML から、フッターの欄(ftr・sldNum の置き場)の図形を返す。
func footerShapes(x string) []string {
	var out []string
	for _, sp := range regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(x, -1) {
		if strings.Contains(sp, `type="ftr"`) || strings.Contains(sp, `type="sldNum"`) {
			out = append(out, sp)
		}
	}
	return out
}

func TestFooterWrap(t *testing.T) {
	// 15字の作者名(#161 の評価 V2)は警告なし、長すぎる作者名は警告が1件。
	short := "図書館の運営の見直しの作業部会"
	long := strings.Repeat("図書館の運営の見直しの作業部会と", 9)
	offRe := regexp.MustCompile(`<a:off x="(\d+)" y="(\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)
	for _, tmpl := range []string{"dark", "light", "report"} {
		for _, c := range []struct {
			author string
			want   int
		}{{short, 0}, {long, 1}} {
			src := footerDoc(c.author)
			_, warns, _ := checkCapacity("f.bx.md", []byte(src), tmpl, nil, nil)
			var got []Issue
			for _, w := range warns {
				if w.Code == "footer-wrap" {
					got = append(got, w)
				}
			}
			if len(got) != c.want {
				t.Fatalf("%s (author %d chars): want %d footer-wrap, got %+v", tmpl, len([]rune(c.author)), c.want, warns)
			}
			if c.want == 1 && (got[0].Path != "/meta/author" || got[0].Line != 3) {
				t.Errorf("%s: footer-wrap must point at the author line: %+v", tmpl, got[0])
			}

			deck, _, err := ParseMarkdown("f.bx.md", []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := Build(&buf, deck, BuildOptions{Template: tmpl, AllowOverflow: true}); err != nil {
				t.Fatal(err)
			}
			files := zipFiles(t, buf.Bytes())
			pageH := 0
			if m := regexp.MustCompile(`<p:sldSz cx="\d+" cy="(\d+)"`).FindStringSubmatch(files["ppt/presentation.xml"]); m != nil {
				pageH, _ = strconv.Atoi(m[1])
			}
			for n := 2; n <= 4; n++ {
				x := files["ppt/slides/slide"+strconv.Itoa(n)+".xml"]
				shapes := footerShapes(x)
				if len(shapes) < 2 {
					t.Fatalf("%s slide%d: footer shapes not found", tmpl, n)
				}
				for _, sp := range shapes {
					if !strings.Contains(sp, `wrap="none"`) {
						t.Errorf("%s slide%d: footer text must not wrap: %s", tmpl, n, sp)
					}
					if k := strings.Count(sp, "<a:p>"); k != 1 || strings.Contains(sp, "<a:br") {
						t.Errorf("%s slide%d: footer text must be one paragraph of one line: %s", tmpl, n, sp)
					}
					m := offRe.FindStringSubmatch(sp)
					if m == nil {
						t.Fatalf("no xfrm: %s", sp)
					}
					y, _ := strconv.Atoi(m[2])
					cy, _ := strconv.Atoi(m[4])
					size := regexp.MustCompile(`sz="(\d+)"`).FindStringSubmatch(sp)
					sz, _ := strconv.Atoi(size[1])
					// 枠は1行の高さ(字の大きさ×行送り程度)で、ページの中。
					if float64(cy) > float64(sz)/100*1.6*12700 || y+cy > pageH {
						t.Errorf("%s slide%d: footer box is not one line inside the page (y %d cy %d sz %d page %d)", tmpl, n, y, cy, sz, pageH)
					}
				}
			}
		}
	}
}

// TestFooterSlack(#180):report のフッターの作者の欄は、字の幅ちょうどの枠にしない。
// 字の幅ちょうどの枠は wrap="none" のプレースホルダーでも LibreOffice が2行に折る
// (短い名前でも)。枠は字の幅に値の表の footerSlackEm を足した幅。dark・light は
// footerSlackEm が 0 で、枠は字の幅のまま(出力を変えない)。
func TestFooterSlack(t *testing.T) {
	authors := []string{"総務", "経理部", "企画調整", "情報システ", "情報システム部", "図書館の運営の見直しの作業部会"}
	cxRe := regexp.MustCompile(`<a:ext cx="(\d+)"`)
	for _, name := range []string{"dark", "light", "report"} {
		tmpl, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		em := tmpl.Values.FooterSlackEm
		if name == "report" && em < 0.25 {
			t.Fatalf("report: footerSlackEm %v leaves too little room", em)
		}
		if name != "report" && em != 0 {
			t.Fatalf("%s: footerSlackEm must stay 0 (output unchanged), got %v", name, em)
		}
		for _, author := range authors {
			deck, _, err := ParseMarkdown("f.bx.md", []byte(footerDoc(author)))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := Build(&buf, deck, BuildOptions{Template: name}); err != nil {
				t.Fatal(err)
			}
			var ftr string
			for _, sp := range footerShapes(zipFiles(t, buf.Bytes())["ppt/slides/slide2.xml"]) {
				if strings.Contains(sp, `type="ftr"`) {
					ftr = sp
				}
			}
			m := cxRe.FindStringSubmatch(ftr)
			if m == nil || !strings.Contains(ftr, author) {
				t.Fatalf("%s %q: author footer not found: %s", name, author, ftr)
			}
			cx, _ := strconv.Atoi(m[1])
			restore := useFonts(tmpl.Fonts)
			ink := textInkWidth(TextNode(author, WithFontSize(tmpl.Values.FooterText)))
			restore()
			room := float64(cx)/12700 - ink
			want := em * tmpl.Values.FooterText
			if room < want-0.01 || room > want+0.01 {
				t.Errorf("%s %q: footer box is %.2fpt wider than the text, want %.2fpt", name, author, room, want)
			}
		}
	}
}
