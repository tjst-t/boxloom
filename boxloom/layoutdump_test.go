package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var xfrmRe = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)

// slideXfrms は PPTX のスライドごとの図形の矩形(pt。EMU / 12700)を書いた順に返す。
func slideXfrms(t *testing.T, pptx []byte) map[int][]LayoutBox {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pptx), int64(len(pptx)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[int][]LayoutBox{}
	for _, f := range zr.File {
		var n int
		if _, err := fmt.Sscanf(f.Name, "ppt/slides/slide%d.xml", &n); err != nil || !strings.HasSuffix(f.Name, ".xml") || strings.Contains(f.Name, "_rels") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[n-1] = out[n-1] // 図形の無いスライド(写真だけの終わりのスライド)も数える
		for _, m := range xfrmRe.FindAllStringSubmatch(string(b), -1) {
			v := make([]float64, 4)
			for i := range v {
				e, _ := strconv.ParseInt(m[i+1], 10, 64)
				v[i] = float64(e) / 12700
			}
			out[n-1] = append(out[n-1], LayoutBox{v[0], v[1], v[2], v[3]})
		}
	}
	return out
}

// expectedShapes は木を描く順に、PPTX で1つの図形になる箱(地のある箱・カード・
// 箇条・文字の葉)の矩形を返す。サブシステムの中は見ない(地は描いた矩形)。
func expectedShapes(n *LayoutNode, root bool, out *[]struct {
	what string
	box  LayoutBox
}) {
	add := func(what string, b LayoutBox) {
		*out = append(*out, struct {
			what string
			box  LayoutBox
		}{what, b})
	}
	if n.Marks != nil && n.Marks.Card {
		add("card "+n.Path, n.Box)
		return
	}
	if n.Background != "" && !root {
		if n.Subsystem != nil {
			add("background "+n.Path, n.Subsystem.Draw)
		} else {
			add("background "+n.Path, n.Box)
		}
	}
	if n.Marks != nil && n.Marks.List {
		add("list "+n.Path, n.Box)
		return
	}
	switch n.Kind {
	case "text":
		add(fmt.Sprintf("text %s %q", n.Path, *n.Text), n.Box)
		return
	case "subsystem":
		return
	}
	for _, c := range n.Children {
		expectedShapes(c, false, out)
	}
}

func sameBox(a, b LayoutBox) bool {
	const eps = 0.006 // 0.01pt への丸め + EMU の切り捨て
	return math.Abs(a.X-b.X) <= eps && math.Abs(a.Y-b.Y) <= eps &&
		math.Abs(a.Width-b.Width) <= eps && math.Abs(a.Height-b.Height) <= eps
}

// TestLayoutBoxesMatchPPTX は examples の全 deck で、layout の出力の文字の葉・カード・
// 箇条・地の box が、build の PPTX の図形の a:off / a:ext と(描く順に)一致することを
// 確かめる。
func TestLayoutBoxesMatchPPTX(t *testing.T) {
	var files []string
	for _, pat := range []string{"../examples/ir/*.json", "../examples/ir/check-warnings/*.json", "../examples/md/*.bx.md", "../examples/md/warnings/*.bx.md", "../examples/yaml/*.yaml"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no examples")
	}
	texts, cards := 0, 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		assets := os.DirFS(filepath.Dir(f))
		for _, tmpl := range []string{"", "dark", "light"} {
			dump, err := Layout(f, data, BuildOptions{Template: tmpl, Assets: assets, AllowOverflow: true})
			if err != nil {
				t.Fatalf("%s@%s: %v", f, tmpl, err)
			}
			if _, err := json.Marshal(dump); err != nil {
				t.Fatal(err)
			}
			deck, err := ReadDeck(f, data)
			if err != nil {
				t.Fatal(err)
			}
			var pptx bytes.Buffer
			if _, err := Build(&pptx, deck, BuildOptions{Template: tmpl, Assets: assets, AllowOverflow: true}); err != nil {
				t.Fatalf("%s@%s: %v", f, tmpl, err)
			}
			shapes := slideXfrms(t, pptx.Bytes())
			if len(shapes) != len(dump.Slides) {
				t.Fatalf("%s@%s: %d slides in PPTX, %d in layout", f, tmpl, len(shapes), len(dump.Slides))
			}
			for si, s := range dump.Slides {
				var want []struct {
					what string
					box  LayoutBox
				}
				expectedShapes(s.Root, true, &want)
				got := shapes[si]
				k := 0
				for _, w := range want {
					for k < len(got) && !sameBox(got[k], w.box) {
						k++
					}
					if k == len(got) {
						t.Errorf("%s@%s %s: %s %+v is not a shape in the PPTX (in drawing order)", f, tmpl, s.Path, w.what, w.box)
						break
					}
					k++
					switch {
					case strings.HasPrefix(w.what, "text "):
						texts++
					case strings.HasPrefix(w.what, "card "):
						cards++
					}
				}
			}
		}
	}
	if texts == 0 || cards == 0 {
		t.Fatalf("matched %d text leaves and %d cards; want some of each", texts, cards)
	}
	t.Logf("matched %d text leaves and %d cards", texts, cards)
}

// TestLayoutPaths は IR の場所と印が木の形から付くことを確かめる。
func TestLayoutPaths(t *testing.T) {
	data, err := os.ReadFile("../examples/md/blocks.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	dump, err := Layout("blocks.bx.md", data, BuildOptions{Assets: os.DirFS("../examples/md")})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]*LayoutNode{}
	var walk func(n *LayoutNode)
	walk = func(n *LayoutNode) {
		if _, ok := seen[n.Path]; !ok {
			seen[n.Path] = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, s := range dump.Slides {
		walk(s.Root)
	}
	for _, p := range []string{"/meta/title", "/slides/1/title", "/slides/1/body", "/slides/1/body/0",
		"/slides/1/body/0/columns/0", "/slides/1/body/0/columns/0/title", "/slides/1/body/0/columns/0/body/0"} {
		if seen[p] == nil {
			t.Errorf("no box with path %s", p)
		}
	}
	if n := seen["/slides/1/body/0"]; n != nil && (n.Marks == nil || n.Marks.IRType != "columns" || n.Marks.ColumnGroups != 3) {
		t.Errorf("/slides/1/body/0 marks = %+v; want irType columns, 3 groups", n.Marks)
	}
	if n := seen["/slides/1/title"]; n != nil && (n.Line == 0 || n.Tokens["fontSize"] != "slideTitle" || n.Placeholder != "title") {
		t.Errorf("/slides/1/title = line %d tokens %v placeholder %q", n.Line, n.Tokens, n.Placeholder)
	}
}

// TestLayoutTokens は layout の出力が部品の書いたトークン(値の表の名前・役割名)と
// 解決した値を一緒に出すことを確かめる。
func TestLayoutTokens(t *testing.T) {
	data := []byte("---\ntitle: T\ntemplate: dark\n---\n\n# 題\n\n- 親の項目\n  - 子の項目\n- 次の項目\n\n:::::compare\n::::option A\n:::pros\n- 良い\n:::\n::::\n::::option B\n:::cons\n- 悪い\n:::\n::::\n:::::\n")
	dump, err := Layout("tokens.bx.md", data, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate("dark")
	var root, card, indent, mark *LayoutNode
	var markRun *LayoutRun
	var walk func(n *LayoutNode)
	walk = func(n *LayoutNode) {
		switch {
		case root == nil && n.Tokens["padding"] == "pagePadding":
			root = n
		case card == nil && n.Marks != nil && n.Marks.Card && n.Tokens["radius"] == "cardRadius":
			card = n
		case indent == nil && n.Tokens["width"] == "pointIndentEm":
			indent = n
		}
		for i, r := range n.Runs {
			if markRun == nil && r.Mark && r.Role == "accent" {
				mark, markRun = n, &n.Runs[i]
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, s := range dump.Slides {
		walk(s.Root)
	}
	v := tm.Values
	if root == nil || root.Tokens["paddingY"] != "pagePaddingY" || root.Tokens["gap"] != "sectionGap" || root.Tokens["background"] != "bg" ||
		root.Padding.X != v.PagePadding || root.Padding.Y != v.PagePaddingY || root.Background != tm.Theme["bg"] {
		t.Errorf("slide root: %+v", root)
	}
	if card == nil || card.Tokens["padding"] != "cardPadding" || card.Tokens["background"] != "card" || *card.Radius != v.CardRadius {
		t.Errorf("card: %+v", card)
	}
	if indent == nil || indent.Tokens["fontSize"] != "body" || *indent.Width != v.Body*v.PointIndentEm {
		t.Errorf("point indent: %+v", indent)
	}
	if markRun == nil || markRun.Color != tm.Theme["accent"] || mark.Tokens["fontSize"] == "" {
		t.Errorf("point mark run: %+v", markRun)
	}
}
