package boxloom

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// markerDeck は架空の区切りの階層と、隅の印のある本文のスライドの文書。頁は
// 1 表紙・2 章・3 節・4 項目・5 本文(A)・6 節・7 本文(1)・8 本文(印なし)。
const markerDeck = `---
title: 架空の町の図書館の報告
---

# 利用の今 {type=section}

# 返却の流れ {type=section level=2}

# 夜間の返却 {type=section level=3}

# 返却箱の使われ方 {marker=A}

- 閉館後の返却が半分を占める

# 案内の見直し {type=section level=2}

# 案内の板 {marker=1}

:::lead
入口の案内を大きくすると迷う人が減る
:::

- 入口の案内を大きくする

# 予算

- 返却箱を二つ増やす
`

// marker が Markdown・IR で読める(本文のスライドだけ)。
func TestMarkerParse(t *testing.T) {
	d, issues, err := ParseMarkdown("m.bx.md", []byte(markerDeck))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	if d.Slides[4].Marker != "A" || d.Slides[6].Marker != "1" || d.Slides[7].Marker != "" {
		t.Fatalf("markers = %q %q %q", d.Slides[4].Marker, d.Slides[6].Marker, d.Slides[7].Marker)
	}
	b, _, _ := ParseMarkdownWithMap([]byte(markerDeck), "m.bx.md")
	if !strings.Contains(string(b), `"marker": "A"`) {
		t.Fatalf("IR has no marker:\n%s", b)
	}
	// IR で直接書いたものも読める(2字まで・全角も1字)。
	d2, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[{"type":"content","title":"題","marker":"甲乙","body":[]}]}`))
	if err != nil || d2.Slides[0].Marker != "甲乙" {
		t.Fatalf("%v %+v", err, d2)
	}
	for _, bad := range []string{
		`{"type":"content","title":"題","marker":"ABC","body":[]}`,
		`{"type":"content","title":"題","marker":"A B","body":[]}`,
		`{"type":"content","title":"題","marker":"","body":[]}`,
		`{"type":"section","title":"章","marker":"A"}`,
		`{"type":"statement","content":"文","marker":"A"}`,
	} {
		if _, err := ParseIR([]byte(`{"boxloom":"0.1","slides":[` + bad + `]}`)); err == nil {
			t.Errorf("accepted: %s", bad)
		}
	}
}

// Markdown の誤り:本文のスライドのほかに書く・長すぎる。
func TestMarkerErrors(t *testing.T) {
	for _, c := range []struct {
		name, md, code, path string
	}{
		{"section", "---\ntitle: 試し\n---\n\n# 章 {type=section marker=A}\n", "misplaced-content", "/slides/1"},
		{"statement", "---\ntitle: 試し\n---\n\n# 言いたいこと {type=statement marker=A}\n", "misplaced-content", "/slides/1"},
		{"too long", "---\ntitle: 試し\n---\n\n# 題 {marker=ABC}\n\n- 文\n", "invalid-value", "/slides/1/marker"},
		{"empty", "---\ntitle: 試し\n---\n\n# 題 {marker=}\n\n- 文\n", "invalid-value", "/slides/1/marker"},
	} {
		res := Check("x.bx.md", []byte(c.md), CheckOptions{})
		found := false
		for _, e := range res.Errors {
			if e.Code == c.code && e.Path == c.path {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: errors = %+v, want %s at %s", c.name, res.Errors, c.code, c.path)
		}
	}
}

// Schema(boxloom schema の IR)の本文のスライドに marker がある。
func TestMarkerSchema(t *testing.T) {
	out, err := Schema("report")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var find func(v any) map[string]any
	find = func(v any) map[string]any {
		switch x := v.(type) {
		case map[string]any:
			if cs, ok := x["contentSlide"].(map[string]any); ok {
				return cs
			}
			for _, c := range x {
				if r := find(c); r != nil {
					return r
				}
			}
		case []any:
			for _, c := range x {
				if r := find(c); r != nil {
					return r
				}
			}
		}
		return nil
	}
	cs := find(doc)
	if cs == nil {
		t.Fatal("contentSlide not found in the schema output")
	}
	m, _ := cs["properties"].(map[string]any)["marker"].(map[string]any)
	if m == nil || m["maxLength"] != float64(2) || m["pattern"] != `^\S{1,2}$` {
		t.Fatalf("marker = %+v", m)
	}
}

func TestBreadcrumb(t *testing.T) {
	d, _, err := ParseMarkdown("m.bx.md", []byte(markerDeck))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range map[int]string{4: "夜間の返却", 6: "案内の見直し", 7: "案内の見直し", 1: ""} {
		if got := breadcrumb(d, i).Plain(); got != want {
			t.Errorf("breadcrumb(%d) = %q, want %q", i, got, want)
		}
	}
}

// report は隅に三角(48pt・濃い基調色、右上が直角)と丸の中の印、その左に直前の
// いちばん深い区切りの題(10pt・薄い字)を描く。印の無いスライドには描かない。
func TestReportCornerMarker(t *testing.T) {
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		page        int
		mark, crumb string
	}{{5, "A", "夜間の返却"}, {7, "1", "案内の見直し"}} {
		slide := builtSlideXML(t, markerDeck, "report", c.page)
		tri := shapesWithGeom(slide, "rtTriangle")
		if len(tri) != 1 {
			t.Fatalf("page %d: %d triangles", c.page, len(tri))
		}
		if !strings.Contains(tri[0], `flipH="1" flipV="1"`) || fillOfXML(tri[0]) != tm.Theme[reportMarkerRole] {
			t.Errorf("page %d: triangle %s", c.page, tri[0])
		}
		want := `<a:off x="` + fmt.Sprint(ptToEMU(tm.Values.PageW-48)) + `" y="0"/><a:ext cx="` + fmt.Sprint(ptToEMU(48)) + `" cy="` + fmt.Sprint(ptToEMU(48)) + `"/>`
		if !strings.Contains(tri[0], want) {
			t.Errorf("page %d: triangle is not the 48pt corner: %s", c.page, tri[0])
		}
		found := false
		for _, e := range shapesWithGeom(slide, "ellipse") {
			if strings.Contains(e, "<a:t>"+c.mark+"</a:t>") && fillOfXML(e) == tm.Theme["bg"] {
				found = true
			}
		}
		if !found {
			t.Errorf("page %d: no circle with %q", c.page, c.mark)
		}
		i := strings.Index(slide, "<a:t>"+c.crumb+"</a:t>")
		if i < 0 {
			t.Fatalf("page %d: no breadcrumb %q", c.page, c.crumb)
		}
		sp := slide[strings.LastIndex(slide[:i], "<p:sp>"):i]
		if !strings.Contains(sp, `sz="1000"`) || !strings.Contains(sp, tm.Theme["text_muted"]) {
			t.Errorf("page %d: breadcrumb is not 10pt muted: %s", c.page, sp)
		}
	}
	if plain := builtSlideXML(t, markerDeck, "report", 8); len(shapesWithGeom(plain, "rtTriangle")) != 0 {
		t.Error("a slide without marker has a corner mark")
	}
}

// 印は頭の配置を変えない(上の余白に描く)。dark・light は印を描かない。
func TestMarkerLayoutAndStandardTemplates(t *testing.T) {
	d, _, err := ParseMarkdown("m.bx.md", []byte(markerDeck))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report", "dark", "light"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		with, err := tm.BuildSlide(d, 6)
		if err != nil {
			t.Fatal(err)
		}
		saved := d.Slides[6].Marker
		d.Slides[6].Marker = ""
		without, err := tm.BuildSlide(d, 6)
		d.Slides[6].Marker = saved
		if err != nil {
			t.Fatal(err)
		}
		computeLayout(with.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		computeLayout(without.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		for _, ph := range []string{"title"} {
			a, b := placeholderLeaf(with.Root, ph), placeholderLeaf(without.Root, ph)
			if a == nil || b == nil || a.Box != b.Box {
				t.Errorf("%s: %s moved with the marker", name, ph)
			}
		}
		if name == "report" {
			continue
		}
		if strings.Join(specTexts(with.Root), "|") != strings.Join(specTexts(without.Root), "|") {
			t.Errorf("%s: marker changes the slide", name)
		}
		if s := builtSlideXML(t, markerDeck, name, 7); len(shapesWithGeom(s, "rtTriangle")) != 0 || strings.Contains(s, "<a:t>1</a:t>") {
			t.Errorf("%s draws the marker", name)
		}
	}
}
