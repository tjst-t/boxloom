package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

// 骨組みの7つの部品が標準の表にあり、どのテンプレートも写しを持つ。
func TestSkeletonRegistryNames(t *testing.T) {
	want := []string{"contentFrame", "contentHeader", "cover", "footer", "section", "source", "statement"}
	if got := standardSkeleton.names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("skeleton = %v, want %v", got, want)
	}
	for _, name := range TemplateNames {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := tm.skeleton.names(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: skeleton = %v, want %v", name, got, want)
		}
	}
}

// 知らない名前は override できない。テンプレートの差し替えは標準の表とほかの
// テンプレートに及ばない。
func TestSkeletonOverrideIsPerTemplate(t *testing.T) {
	tm, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.skeleton.override("contentHead", skeletonFunc(func(skeletonContext) (*Spec, error) { return nil, nil })); err == nil ||
		!strings.Contains(err.Error(), "unknown skeleton part") {
		t.Fatalf("override of an unknown name: err = %v", err)
	}
	if err := tm.skeleton.override(skelFooter, skeletonFunc(func(skeletonContext) (*Spec, error) { return nil, nil })); err != nil {
		t.Fatal(err)
	}
	other, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(other.skeleton.parts[skelFooter]).Pointer() != reflect.ValueOf(standardSkeleton.parts[skelFooter]).Pointer() {
		t.Fatal("override leaked into another template")
	}
}

const skeletonTestDeck = `{
  "boxloom": "0.1",
  "meta": {"title": "骨組みの試験", "author": "架空の研究会"},
  "slides": [
    {"type": "cover"},
    {"type": "content", "title": "夜間の返却を見直す", "lead": "返却の半分は閉館後に集まる",
     "source": "架空の利用者調査(2026)",
     "body": [{"type": "points", "items": ["返却箱を増やす", "案内を変える", "受付の時間を延ばす"]}]},
    {"type": "statement", "content": "返却は夜に集まる"}
  ]
}`

// leadFirstTemplate は contentHeader を差し替えて、リード文を題の上に置く試験の
// テンプレート。題の葉は標準と同じく title のプレースホルダーの印を付ける。
func leadFirstTemplate(t *testing.T) *Template {
	t.Helper()
	tm, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	err = tm.skeleton.override(skelContentHeader, skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
		t, s := ctx.t, ctx.slide
		title := t.text(s.Title, "slideTitle", "text_primary", true, WithPlaceholder("title"), fromIR("title"))
		if s.Lead == nil {
			return title, nil
		}
		return Col([]*Spec{t.leadText(s.Lead), title}, tokGap("headerGap")), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func findIRPath(s *Spec, p string) *Spec {
	if s.irPath == p {
		return s
	}
	for _, c := range s.Children {
		if f := findIRPath(c, p); f != nil {
			return f
		}
	}
	return nil
}

// contentHeader を差し替えると題とリード文の位置が変わり、題のプレースホルダーと
// 本文の容量の測りはそのまま保たれる。
func TestSkeletonContentHeaderOverride(t *testing.T) {
	deck, err := ReadDeck("deck.json", []byte(skeletonTestDeck))
	if err != nil {
		t.Fatal(err)
	}
	std, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	alt := leadFirstTemplate(t)

	type result struct {
		title, lead *Spec
		built       *BuiltSlide
		caps        []SlideCapacity
		warns       []string
		slideXML    string
	}
	run := func(tm *Template) result {
		t.Helper()
		_, warns, caps, err := tm.renderPages(deck)
		if err != nil {
			t.Fatal(err)
		}
		// renderPages が配置した木は返らないので、同じ組み方で組み直して配置する。
		b, err := tm.BuildSlide(deck, 1)
		if err != nil {
			t.Fatal(err)
		}
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		var buf bytes.Buffer
		if _, err := tm.RenderDeck(deck, &buf); err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		var xml string
		for _, f := range zr.File {
			if f.Name == "ppt/slides/slide2.xml" {
				rc, _ := f.Open()
				data, _ := io.ReadAll(rc)
				rc.Close()
				xml = string(data)
			}
		}
		return result{title: findIRPath(b.Root, "title"), lead: findIRPath(b.Root, "lead"), built: b, caps: caps, warns: warns, slideXML: xml}
	}
	a, b := run(std), run(alt)

	// 位置が変わる:標準は題が上、差し替えではリード文が上。
	if !(a.title.Box.Y < a.lead.Box.Y) {
		t.Errorf("standard: title y %.1f should be above lead y %.1f", a.title.Box.Y, a.lead.Box.Y)
	}
	if !(b.lead.Box.Y < b.title.Box.Y) {
		t.Errorf("override: lead y %.1f should be above title y %.1f", b.lead.Box.Y, b.title.Box.Y)
	}
	if a.title.Box.Y == b.title.Box.Y {
		t.Errorf("title did not move (y %.1f)", a.title.Box.Y)
	}

	// プレースホルダーの印が保たれる:題の葉は title の印を持ち、コアの題の測り
	// (BuiltSlide.Title)も同じ葉を指し、PPTX でも題のプレースホルダーに書かれる。
	if b.title.Placeholder != "title" || b.built.Title != b.title {
		t.Errorf("override: title leaf placeholder %q, BuiltSlide.Title is the leaf: %v", b.title.Placeholder, b.built.Title == b.title)
	}
	for name, r := range map[string]result{"standard": a, "override": b} {
		if n := strings.Count(r.slideXML, `<p:ph type="title"/>`); n != 1 {
			t.Errorf("%s: slide 2 has %d title placeholders, want 1", name, n)
		}
	}

	// 容量が同じに測れる:本文の箱は同じで、本文の領域・要る高さ・残りが同じ。
	if len(a.caps) != 1 || len(b.caps) != 1 {
		t.Fatalf("capacity: %d and %d slides, want 1", len(a.caps), len(b.caps))
	}
	ca, cb := a.caps[0], b.caps[0]
	if ca.BodyArea <= 0 || ca.BodyArea != cb.BodyArea || ca.Need != cb.Need || ca.Remaining != cb.Remaining {
		t.Errorf("capacity changed: standard %+v, override %+v", ca, cb)
	}
	if !reflect.DeepEqual(a.warns, b.warns) {
		t.Errorf("warnings changed:\n standard %v\n override %v", a.warns, b.warns)
	}
	if a.built.Body == nil || b.built.Body == nil || a.built.Body.Box != b.built.Body.Box {
		t.Errorf("body box moved")
	}
}

// 骨組みの印が無い・根の直接の子でない並べ方は誤り(容量を黙って測れなくしない)。
func TestSkeletonFrameMarksRequired(t *testing.T) {
	deck, err := ReadDeck("deck.json", []byte(skeletonTestDeck))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]skeletonFunc{
		"no body": func(ctx skeletonContext) (*Spec, error) {
			blocks, err := ctx.blocks()
			if err != nil {
				return nil, err
			}
			return Col(blocks, tokBackground("bg")), nil
		},
		"nested body": func(ctx skeletonContext) (*Spec, error) {
			blocks, err := ctx.blocks()
			if err != nil {
				return nil, err
			}
			return Col([]*Spec{Col([]*Spec{Col(blocks, markFrame(frameBody))})}, tokBackground("bg")), nil
		},
	}
	for name, f := range cases {
		tm, err := LookupTemplate("light")
		if err != nil {
			t.Fatal(err)
		}
		if err := tm.skeleton.override(skelContentFrame, f); err != nil {
			t.Fatal(err)
		}
		if _, err := tm.BuildSlide(deck, 1); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
