package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

const forestPath = "../examples/templates/forest.json"

// 外の値の表は同梱のものと同じく読めて、Build・Check・SchemaTemplate に渡せる。
func TestParseTemplateExternal(t *testing.T) {
	data := mustFile(t, forestPath)
	tm, err := ParseTemplate(forestPath, data)
	if err != nil {
		t.Fatal(err)
	}
	if tm.Name != "forest" || tm.Theme["accent"] != "2F6B3A" {
		t.Fatalf("name %q accent %q", tm.Name, tm.Theme["accent"])
	}
	deck, err := ReadDeck("demo-deck.bx.md", mustFile(t, "../examples/md/demo-deck.bx.md"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	res, err := Build(&buf, deck, BuildOptions{Template: "dark", Values: tm})
	if err != nil || res.Template != "forest" {
		t.Fatalf("Build: %v %+v", err, res)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	theme := ""
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide2.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			theme = string(b)
		}
	}
	if !strings.Contains(theme, "2F6B3A") {
		t.Errorf("slide2.xml does not use the forest accent")
	}
	r := Check("demo-deck.bx.md", mustFile(t, "../examples/md/demo-deck.bx.md"), CheckOptions{Values: tm})
	if len(r.Errors) != 0 {
		t.Fatalf("Check: %+v", r.Errors)
	}
	b, err := SchemaTemplate(tm)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Templates []struct{ Name string } `json:"templates"`
	}
	if err := json.Unmarshal(b, &out); err != nil || len(out.Templates) != 1 || out.Templates[0].Name != "forest" {
		t.Fatalf("schema templates = %+v (%v)", out.Templates, err)
	}
}

// 誤ったファイルは5章の形の誤り1件(値の表の中の場所と行)になる。
func TestParseTemplateErrors(t *testing.T) {
	base := string(mustFile(t, forestPath))
	cases := []struct {
		name, from, to, path string
	}{
		{"color", `"accent": "2F6B3A"`, `"accent": "green"`, "/theme/accent"},
		{"offscale", `"blockGap": 20`, `"blockGap": 13`, "/values/blockGap"},
		{"wrongtype", `"blockGap": 20`, `"blockGap": "16"`, "/values/blockGap"},
		{"unknown", `"latin": `, `"colour": "x", "latin": `, "/fonts/colour"},
		{"missing", `"pagePadding": 40,`, ``, "/values/pagePadding"},
		{"role", `"accent": "2F6B3A"`, `"accent": "2F6B3A", "brand": "112233"`, "/theme/brand"},
	}
	for _, c := range cases {
		src := strings.Replace(base, c.from, c.to, 1)
		if src == base {
			t.Fatalf("%s: replacement did not apply", c.name)
		}
		src = strings.Replace(src, `"name": "forest"`, `"name": "`+c.name+`"`, 1)
		_, err := ParseTemplate("dir/"+c.name+".json", []byte(src))
		var te *TemplateError
		if !errors.As(err, &te) {
			t.Fatalf("%s: err = %v", c.name, err)
		}
		is := te.Issue
		if is.File != "dir/"+c.name+".json" || is.Code != "template-values" || is.Path != c.path || is.Line == 0 {
			t.Errorf("%s: issue = %+v", c.name, is)
		}
	}
	// 値の表の name はファイルの名前と同じでなければならない。
	_, err := ParseTemplate("other.json", []byte(base))
	var te *TemplateError
	if !errors.As(err, &te) || te.Issue.Path != "/name" {
		t.Fatalf("name mismatch: %v", err)
	}
}

func TestIsTemplateFile(t *testing.T) {
	for v, want := range map[string]bool{"light": false, "dark": false, "./my.json": true, "my.json": true, "a/b": true, "": false} {
		if IsTemplateFile(v) != want {
			t.Errorf("IsTemplateFile(%q) != %v", v, want)
		}
	}
}
