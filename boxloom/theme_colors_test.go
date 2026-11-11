package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// zipPart は pptx の中の1つの部品を読む。
func zipPart(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	t.Fatalf("%s not found", name)
	return ""
}

// テーマの配色(theme1.xml)とマスターの clrMap が、テンプレートの値の表から
// 作られる(#43)。地が暗いテーマはマスターの clrMap が bg1=dk1 を向く。
func TestThemeColorsFromValueTable(t *testing.T) {
	src, err := os.ReadFile("../examples/md/demo-deck.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	deck, _, err := ParseMarkdown("demo-deck.bx.md", src)
	if err != nil {
		t.Fatal(err)
	}
	forestData, err := os.ReadFile("../examples/templates/forest.json")
	if err != nil {
		t.Fatal(err)
	}
	forest, err := ParseTemplate("forest.json", forestData)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		opt  BuildOptions
		tmpl *Template
		dark bool
	}{
		{"light", BuildOptions{Template: "light"}, mustTemplate("light"), false},
		{"dark", BuildOptions{Template: "dark"}, mustTemplate("dark"), true},
		{"forest", BuildOptions{Values: forest}, forest, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if _, err := Build(&buf, deck, c.opt); err != nil {
				t.Fatal(err)
			}
			th := zipPart(t, buf.Bytes(), "ppt/theme/theme1.xml")
			master := zipPart(t, buf.Bytes(), "ppt/slideMasters/slideMaster1.xml")
			col := func(r string) string { return hexColor(c.tmpl.Theme[r]) }
			if !strings.Contains(th, `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="`+c.tmpl.Name+`">`) ||
				!strings.Contains(th, `<a:clrScheme name="`+c.tmpl.Name+`">`) {
				t.Errorf("theme name is not the template name %q", c.tmpl.Name)
			}
			bgSlot, textSlot := "lt1", "dk1"
			if c.dark {
				bgSlot, textSlot = "dk1", "lt1"
			}
			for slot, role := range map[string]string{bgSlot: "bg", textSlot: "text_primary", "accent1": "accent", "accent3": "support", "accent4": "danger"} {
				want := `<a:` + slot + `><a:srgbClr val="` + col(role) + `"/></a:` + slot + `>`
				if !strings.Contains(th, want) {
					t.Errorf("theme1.xml lacks %s", want)
				}
			}
			if strings.Contains(th, "sysClr") {
				t.Error("theme1.xml still uses system colors")
			}
			wantMap := `bg1="lt1" tx1="dk1"`
			if c.dark {
				wantMap = `bg1="dk1" tx1="lt1"`
			}
			if !strings.Contains(master, `<p:clrMap `+wantMap) {
				t.Errorf("master clrMap: want %s", wantMap)
			}
			if !strings.Contains(master, `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="`+col("bg")+`"/>`) {
				t.Error("master background is not the template's bg")
			}
		})
	}
}
