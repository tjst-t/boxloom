package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// zipEntries は PPTX の中身(名前 → バイト列)。
func zipEntries(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = data
	}
	return out
}

func layoutExampleFiles(t *testing.T) []string {
	var files []string
	for _, pat := range []string{"../examples/ir/*.json", "../examples/ir/check-warnings/*.json", "../examples/md/*.bx.md", "../examples/md/warnings/*.bx.md", "../examples/yaml/*.yaml"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no examples")
	}
	return files
}

// TestLayoutRoundTrip は examples の全 deck で、layout の出力(JSON)を読み直して組むと
// build と同じ PPTX になることを確かめる(サブシステムの中身・ノート・題は Deck から)。
// Deck を渡さないときも読めて、サブシステムの無いスライドは同じになる。
func TestLayoutRoundTrip(t *testing.T) {
	for _, f := range layoutExampleFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		assets := os.DirFS(filepath.Dir(f))
		deck, err := ReadDeck(f, data)
		if err != nil {
			t.Fatal(err)
		}
		for _, tmpl := range []string{"", "dark", "light"} {
			dump, err := Layout(f, data, BuildOptions{Template: tmpl, Assets: assets, AllowOverflow: true})
			if err != nil {
				t.Fatalf("%s@%s: %v", f, tmpl, err)
			}
			src, err := json.Marshal(dump)
			if err != nil {
				t.Fatal(err)
			}
			var want, got, stub bytes.Buffer
			if _, err := Build(&want, deck, BuildOptions{Template: tmpl, Assets: assets, AllowOverflow: true, noOverflowMark: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := BuildLayout(&got, "tree.json", src, LayoutReadOptions{Deck: deck, Assets: assets}); err != nil {
				t.Fatalf("%s@%s: read back: %v", f, tmpl, err)
			}
			we, ge := zipEntries(t, want.Bytes()), zipEntries(t, got.Bytes())
			if len(we) != len(ge) {
				t.Errorf("%s@%s: %d parts, read back %d", f, tmpl, len(we), len(ge))
			}
			for name, w := range we {
				if !bytes.Equal(w, ge[name]) {
					t.Errorf("%s@%s: %s differs after reading the layout tree back", f, tmpl, name)
				}
			}
			// Deck なし:サブシステムは申告だけの何も描かない葉。木は読めて、
			// サブシステムを持たないスライドは同じ図形になる。
			if _, err := BuildLayout(&stub, "tree.json", src, LayoutReadOptions{}); err != nil {
				t.Fatalf("%s@%s: read back without deck: %v", f, tmpl, err)
			}
			se := zipEntries(t, stub.Bytes())
			for i, s := range dump.Slides {
				// Deck も Assets も無ければ地の写真(meta.coverImage)は読めない。
				if hasSubsystemNode(s.Root) || s.BackgroundImage != "" {
					continue
				}
				name := "ppt/slides/slide" + itoa(i+1) + ".xml"
				if !bytes.Equal(we[name], se[name]) {
					t.Errorf("%s@%s: %s differs when read back without the deck", f, tmpl, name)
				}
			}
		}
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func hasSubsystemNode(n *LayoutNode) bool {
	if n.Kind == "subsystem" {
		return true
	}
	for _, c := range n.Children {
		if hasSubsystemNode(c) {
			return true
		}
	}
	return false
}

// readLayoutIssues は木を JSON の文字列のまま書き換えて読み、誤りを返す。
func readLayoutIssues(t *testing.T, edit func(d *LayoutDump)) []Issue {
	t.Helper()
	data, err := os.ReadFile("../examples/md/blocks.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	dump, err := Layout("blocks.bx.md", data, BuildOptions{Assets: os.DirFS("../examples/md")})
	if err != nil {
		t.Fatal(err)
	}
	edit(dump)
	src, _ := json.Marshal(dump)
	_, err = BuildLayout(io.Discard, "tree.json", src, LayoutReadOptions{})
	var issues MarkdownErrors
	if !errors.As(err, &issues) {
		t.Fatalf("err = %v; want issues", err)
	}
	return issues
}

// findNode は木の中で pred に当たる最初の箱と、その JSON の場所を返す。
func findNode(d *LayoutDump, pred func(*LayoutNode) bool) (*LayoutNode, string) {
	var walk func(n *LayoutNode, at string) (*LayoutNode, string)
	walk = func(n *LayoutNode, at string) (*LayoutNode, string) {
		if pred(n) {
			return n, at
		}
		for i, c := range n.Children {
			if m, p := walk(c, at+"/children/"+itoa(i)); m != nil {
				return m, p
			}
		}
		return nil, ""
	}
	for i, s := range d.Slides {
		if m, p := walk(s.Root, "/slides/"+itoa(i)+"/root"); m != nil {
			return m, p
		}
	}
	return nil, ""
}

// TestLayoutReadChecksTokens は段から外れた余白・字の大きさ、知らない役割名・値の表の
// 名前、トークンの無い数・色、トークンと食い違う控えの値が誤りになることを確かめる。
func TestLayoutReadChecksTokens(t *testing.T) {
	gapBox := func(n *LayoutNode) bool { return n.Tokens["gap"] != "" && n.Gap != 0 }
	textLeaf := func(n *LayoutNode) bool { return n.Kind == "text" && n.Tokens["color"] != "" && len(n.Runs) == 0 }
	runLeaf := func(n *LayoutNode) bool { return len(n.Runs) > 0 && n.Runs[0].Role != "" }
	boxNode := func(n *LayoutNode) bool { return n.Kind == "row" || n.Kind == "col" }
	roleRunLeaf := func(n *LayoutNode) bool { return runLeaf(n) && n.Tokens["color"] == "" }
	growNode := func(n *LayoutNode) bool { return n.Grow > 0 }
	cases := []struct {
		name string
		pred func(*LayoutNode) bool
		edit func(n *LayoutNode)
		path string // 場所の末尾
		code string
	}{
		{"gap off the scale", gapBox, func(n *LayoutNode) { n.Tokens["gap"] = "space(0)"; n.Gap = 0 }, "/tokens/gap", "layout-token"},
		{"gap unknown name", gapBox, func(n *LayoutNode) { n.Tokens["gap"] = "hugeGap"; n.Gap = 0 }, "/tokens/gap", "layout-token"},
		{"gap without token", gapBox, func(n *LayoutNode) { delete(n.Tokens, "gap"); n.Gap = 7 }, "/gap", "layout-raw"},
		{"gap mismatch", gapBox, func(n *LayoutNode) { n.Gap += 1 }, "/gap", "layout-mismatch"},
		{"font size off the scale", textLeaf, func(n *LayoutNode) { n.Tokens["fontSize"] = "type(13.3)"; n.FontSize = 0 }, "/tokens/fontSize", "layout-token"},
		{"font size unknown name", textLeaf, func(n *LayoutNode) { n.Tokens["fontSize"] = "giant"; n.FontSize = 0 }, "/tokens/fontSize", "layout-token"},
		{"font size without token", textLeaf, func(n *LayoutNode) { delete(n.Tokens, "fontSize") }, "/fontSize", "layout-raw"},
		{"unknown role", textLeaf, func(n *LayoutNode) { n.Tokens["color"] = "brand_pink"; n.Color = "" }, "/tokens/color", "layout-token"},
		{"hex color token", textLeaf, func(n *LayoutNode) { n.Tokens["color"] = "#FF00FF"; n.Color = "" }, "/tokens/color", "layout-token"},
		{"color without role", textLeaf, func(n *LayoutNode) { delete(n.Tokens, "color"); n.Color = "FF00FF" }, "/color", "layout-raw"},
		{"run unknown role", runLeaf, func(n *LayoutNode) { n.Runs[0].Role = "brand_pink"; n.Runs[0].Color = "" }, "/runs/0/role", "layout-token"},
		{"run color without role", runLeaf, func(n *LayoutNode) { n.Runs[0].Role = "" }, "/runs/0/color", "layout-raw"},
		{"row/col color without role", boxNode, func(n *LayoutNode) { delete(n.Tokens, "color"); n.Color = "FF00FF" }, "/color", "layout-raw"},
		{"negative grow", growNode, func(n *LayoutNode) { n.Grow = -1 }, "/grow", "layout-node"},
		{"negative basis", boxNode, func(n *LayoutNode) { b := -5.0; n.Basis = &b }, "/basis", "layout-node"},
		{"text leaf without color", textLeaf, func(n *LayoutNode) { delete(n.Tokens, "color"); n.Color = "" }, "/tokens", "layout-raw"},
		{"runs without every role", roleRunLeaf, func(n *LayoutNode) { n.Runs[0].Role = ""; n.Runs[0].Color = "" }, "/tokens", "layout-raw"},
		{"unknown token key", gapBox, func(n *LayoutNode) { n.Tokens["margin"] = "blockGap" }, "/tokens/margin", "layout-token"},
	}
	for _, c := range cases {
		var at string
		issues := readLayoutIssues(t, func(d *LayoutDump) {
			n, p := findNode(d, c.pred)
			if n == nil {
				t.Fatalf("%s: no box", c.name)
			}
			at = p
			c.edit(n)
		})
		found := false
		for _, is := range issues {
			if is.Path == at+c.path && is.Code == c.code {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: issues %v; want %s at %s%s", c.name, issues, c.code, at, c.path)
		}
	}
}

// TestLayoutReadIgnoresBox は座標(box)を読まないことを確かめる(書き換えても同じ PPTX)。
func TestLayoutReadIgnoresBox(t *testing.T) {
	data, err := os.ReadFile("../examples/md/text-patterns.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	dump, err := Layout("t.bx.md", data, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(dump)
	dump.Slides[0].Root.Box = LayoutBox{1, 2, 3, 4}
	b, _ := json.Marshal(dump)
	var pa, pb bytes.Buffer
	if _, err := BuildLayout(&pa, "a.json", a, LayoutReadOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildLayout(&pb, "b.json", b, LayoutReadOptions{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pa.Bytes(), pb.Bytes()) {
		t.Error("the box changed the PPTX")
	}
}

// TestLayoutReadNotInSchema は読み直す口が作者向けの一覧(boxloom schema)に現れない
// ことを確かめる。
func TestLayoutReadNotInSchema(t *testing.T) {
	b, err := Schema("")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"boxloom-layout", "layout-build", "LayoutDump"} {
		if strings.Contains(string(b), w) {
			t.Errorf("schema output mentions %q", w)
		}
	}
}
