package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
)

// pairsLayoutNodes は layout の木から、場所ごとに最初(一番外側)の箱を引く表を作る。
func pairsLayoutNodes(dump *LayoutDump) map[string]*LayoutNode {
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
	return seen
}

// 例の deck を dark・light で組むと、各対の2つの側の箱は同じ上端・同じ高さで、
// 対の行の高さは2つの側の中身の高い方と等しい。見出しは側の真上に来る。
// layout の箱が PPTX の図形と同じことは TestLayoutBoxesMatchPPTX が確かめる。
func TestPairsRowsAlign(t *testing.T) {
	for _, f := range []string{"../examples/md/pairs.bx.md", "../examples/ir/pairs.json"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, tmpl := range []string{"dark", "light"} {
			var pptx bytes.Buffer
			deck, err := ReadDeck(f, data)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Build(&pptx, deck, BuildOptions{Template: tmpl})
			if err != nil {
				t.Fatalf("%s@%s: %v", f, tmpl, err)
			}
			for _, w := range res.Warnings {
				if strings.Contains(w, "COMPONENT-FALLBACK") || strings.Contains(w, "LAYOUT-OVERFLOW") {
					t.Errorf("%s@%s: %s", f, tmpl, w)
				}
			}
			dump, err := Layout(f, data, BuildOptions{Template: tmpl})
			if err != nil {
				t.Fatal(err)
			}
			seen := pairsLayoutNodes(dump)
			block := seen["/slides/1/body/0"]
			if block == nil || block.Marks == nil || block.Marks.IRType != "pairs" || block.Marks.PairRows != 4 {
				t.Fatalf("%s@%s: block marks %+v", f, tmpl, block)
			}
			hs, hr := seen["/slides/1/body/0/headings/subject"], seen["/slides/1/body/0/headings/response"]
			if hs == nil || hr == nil || seen["/slides/1/body/0/headings"] == nil {
				t.Fatalf("%s@%s: no headings boxes", f, tmpl)
			}
			for k := 0; k < 4; k++ {
				pp := "/slides/1/body/0/pairs/" + string(rune('0'+k))
				row, s, r := seen[pp], seen[pp+"/subject"], seen[pp+"/response"]
				if row == nil || s == nil || r == nil || len(s.Children) != 1 || len(r.Children) != 1 {
					t.Fatalf("%s@%s: pair %d boxes missing", f, tmpl, k)
				}
				if !nearPt(s.Box.Y, r.Box.Y) || !nearPt(s.Box.Y, row.Box.Y) {
					t.Errorf("%s@%s: pair %d tops differ: subject %.2f response %.2f row %.2f", f, tmpl, k, s.Box.Y, r.Box.Y, row.Box.Y)
				}
				// 中身(段落・箇条・引用)の高さの高い方が、対の行と2つの側の高さ。
				tall := math.Max(s.Children[0].Box.Height, r.Children[0].Box.Height)
				if !nearPt(row.Box.Height, tall) || !nearPt(s.Box.Height, tall) || !nearPt(r.Box.Height, tall) {
					t.Errorf("%s@%s: pair %d heights: row %.2f subject %.2f response %.2f, contents %.2f / %.2f",
						f, tmpl, k, row.Box.Height, s.Box.Height, r.Box.Height, s.Children[0].Box.Height, r.Children[0].Box.Height)
				}
				// 中身は側の上端から置く。
				if !nearPt(s.Children[0].Box.Y, s.Box.Y) || !nearPt(r.Children[0].Box.Y, r.Box.Y) {
					t.Errorf("%s@%s: pair %d contents do not start at the top", f, tmpl, k)
				}
				// 見出しは側の真上(同じ x・同じ幅)。
				if !nearPt(s.Box.X, hs.Box.X) || !nearPt(s.Box.Width, hs.Box.Width) || !nearPt(r.Box.X, hr.Box.X) || !nearPt(r.Box.Width, hr.Box.Width) {
					t.Errorf("%s@%s: pair %d is not under the headings", f, tmpl, k)
				}
			}
		}
	}
}

// noPairsTemplate は pairs の組み方を持たない試験用のテンプレート(light の写しから外す)。
func noPairsTemplate(t *testing.T) *Template {
	t.Helper()
	tm := mustTemplate("light")
	delete(tm.components.components, pairsType)
	if tm.components.has(pairsType) || !mustTemplate("light").components.has(pairsType) {
		t.Fatal("template copies share the component table")
	}
	return tm
}

// 組み方を持たないテンプレートは、警告 component-fallback を1件出して columns で組む。
// 代わりの組み方の IR(columns)は Schema と同じ形の検査と check を通り、それを
// そのまま組んだものと同じ PPTX になる。
func TestPairsFallback(t *testing.T) {
	data, err := os.ReadFile("../examples/ir/pairs.json")
	if err != nil {
		t.Fatal(err)
	}
	tm := noPairsTemplate(t)
	res := Check("pairs.json", data, CheckOptions{Values: tm})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	var fb []Issue
	for _, w := range res.Warnings {
		if w.Code == "component-fallback" {
			fb = append(fb, w)
		}
	}
	if len(fb) != 1 || fb[0].Path != "/slides/1/body/0" || !strings.Contains(fb[0].Message, `"pairs"`) || !strings.Contains(fb[0].Message, "built as columns") {
		t.Fatalf("component-fallback warnings = %+v", fb)
	}
	// dark・light は組み方を持つので警告は出ない。
	for _, name := range []string{"dark", "light"} {
		for _, w := range Check("pairs.json", data, CheckOptions{Template: name}).Warnings {
			if w.Code == "component-fallback" {
				t.Errorf("%s: %+v", name, w)
			}
		}
	}

	// 代わりの組み方の IR を書き出し、IR として読み直して check を通す。
	deck, err := ReadIR("pairs.json", data)
	if err != nil {
		t.Fatal(err)
	}
	blk := &deck.Slides[1].Body[0]
	alts, err := pairsFallback(blk)
	if err != nil || len(alts) != 1 || alts[0].Type != "columns" {
		t.Fatalf("fallback = %+v, %v", alts, err)
	}
	cols := alts[0].Columns
	if len(cols) != 2 || cols[0].Title.Plain() != "分かったこと" || cols[1].Title.Plain() != "打ち手" {
		t.Fatalf("fallback columns = %+v", cols)
	}
	// 各対の側を順に入れる(引用は言葉の段落と出どころの注に分ける)。
	var types [2][]string
	for i, c := range cols {
		for _, b := range c.Body {
			types[i] = append(types[i], b.Type)
		}
	}
	if got := strings.Join(types[0], ","); got != "paragraph,paragraph,paragraph,note,paragraph" {
		t.Errorf("subject column = %s", got)
	}
	if got := strings.Join(types[1], ","); got != "points,paragraph,points,paragraph" {
		t.Errorf("response column = %s", got)
	}
	fbJSON := fallbackDeckJSON(t, data, alts[0])
	// 書き換えた IR は examples/ir/pairs-fallback.json と同じ。examples/ir の全 deck は
	// tools/validate_ir.py が Schema で検査する(CI の python)ので、代わりの IR が
	// Schema を通ることはそこで確かめる。
	const fbFile = "../examples/ir/pairs-fallback.json"
	if *updateGolden {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, fbJSON, "", "  "); err != nil {
			t.Fatal(err)
		}
		pretty.WriteString("\n")
		if err := os.WriteFile(fbFile, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fbFile)
	if err != nil {
		t.Fatalf("%v (write it with -update-golden)", err)
	}
	if !sameJSON(want, fbJSON) {
		t.Errorf("%s differs from the fallback of pairs.json (write it with -update-golden)", fbFile)
	}
	res = Check("pairs-fallback.json", want, CheckOptions{Template: "light"})
	if len(res.Errors) > 0 {
		t.Fatalf("fallback IR fails check: %+v", res.Errors)
	}
	for _, w := range res.Warnings {
		if w.Code == "component-fallback" || w.Code == "layout-overflow" {
			t.Errorf("fallback IR: %+v", w)
		}
	}

	// 組み方を持たないテンプレートの PPTX は、書き換えた columns をそのまま組んだものと同じ。
	render := func(b []byte, tm *Template) []byte {
		d, err := ParseIR(b)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := tm.RenderDeck(d, &buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	if !bytes.Equal(render(data, noPairsTemplate(t)), render(fbJSON, mustTemplate("light"))) {
		t.Error("fallback build differs from building the rewritten columns block")
	}
}

// fallbackDeckJSON は IR の deck の /slides/1/body/0 を b に替えた JSON を作る。
func fallbackDeckJSON(t *testing.T, data []byte, b *IRBlock) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	inl := func(in Inline) any { return in.Plain() }
	var block func(b IRBlock) map[string]any
	block = func(b IRBlock) map[string]any {
		out := map[string]any{"type": b.Type}
		switch b.Type {
		case "paragraph", "note":
			out["content"] = inl(b.Content)
		case "points":
			var items []any
			for _, it := range b.Items {
				items = append(items, inl(it.Content))
			}
			out["items"] = items
		default:
			t.Fatalf("unexpected type %s in the fallback", b.Type)
		}
		return out
	}
	var cols []any
	for _, c := range b.Columns {
		var body []any
		for _, x := range c.Body {
			body = append(body, block(x))
		}
		cols = append(cols, map[string]any{"title": inl(c.Title), "body": body})
	}
	slides := doc["slides"].([]any)
	slides[1].(map[string]any)["body"] = []any{map[string]any{"type": "columns", "columns": cols}}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// はみ出しは対ごとに測る:はみ出した対ごとに layout-overflow が1件、場所はその対。
// 数は容量(capacity)の対の測り値と同じ。
func TestPairsOverflowPerPair(t *testing.T) {
	for _, f := range []string{"../examples/ir/check-warnings/pairs-overflow.json", "../examples/md/warnings/pairs-overflow.bx.md"} {
		for _, tmpl := range []string{"", "dark"} {
			errs, warns, caps := CheckFileCapacity(f, tmpl)
			if len(errs) > 0 {
				t.Fatalf("%s: %+v", f, errs)
			}
			var paths []string
			for _, w := range warns {
				if w.Code != "layout-overflow" {
					t.Errorf("%s@%s: unexpected warning %+v", f, tmpl, w)
					continue
				}
				paths = append(paths, w.Path)
			}
			if got := strings.Join(paths, " "); got != "/slides/1/body/1/pairs/3 /slides/1/body/1/pairs/4" {
				t.Fatalf("%s@%s: overflow paths %s", f, tmpl, got)
			}
			if strings.HasSuffix(f, ".md") && (warns[0].Line != 31 || warns[1].Line != 43) {
				t.Errorf("%s@%s: lines %d, %d", f, tmpl, warns[0].Line, warns[1].Line)
			}
			b := caps[0].Blocks[1]
			if b.Type != "pairs" || len(b.Pairs) != 5 {
				t.Fatalf("%s@%s: capacity %+v", f, tmpl, b)
			}
			for k, p := range b.Pairs {
				if (p.Overflow > 0) != (k >= 3) {
					t.Errorf("%s@%s: pair %d overflow %.1f", f, tmpl, k, p.Overflow)
				}
				if p.Need != math.Max(p.Subject, p.Response) {
					t.Errorf("%s@%s: pair %d need %.1f, sides %.1f / %.1f", f, tmpl, k, p.Need, p.Subject, p.Response)
				}
			}
			if b.Pairs[3].Tallest != "response" || b.Pairs[4].Bottom != b.Bottom {
				t.Errorf("%s@%s: pairs %+v, block %+v", f, tmpl, b.Pairs, b)
			}
			// 警告の文の数は容量と同じ測り値(0.1pt に丸めたもの)。
			if want := fmt.Sprintf("by %.1fpt (pair needs %.1fpt", b.Pairs[3].Overflow, b.Pairs[3].Need); !strings.Contains(warns[0].Message, want) {
				t.Errorf("%s@%s: %q does not contain %q", f, tmpl, warns[0].Message, want)
			}
			if strings.HasSuffix(f, ".md") && b.Pairs[3].Line != 31 {
				t.Errorf("%s@%s: capacity line %d", f, tmpl, b.Pairs[3].Line)
			}
		}
	}
}

// Markdown の読み方:形の誤りは場所つきで返す。
func TestPairsMarkdownErrors(t *testing.T) {
	head := "---\ntitle: T\n---\n\n# 題\n\n"
	side := func(name, body string) string { return ":::" + name + "\n" + body + "\n:::\n" }
	good := side("subject", "あ") + side("response", "い") + side("subject", "う") + side("response", "- え")
	for _, c := range []struct {
		name, src, code, path string
	}{
		{"no headings", "::::pairs\n" + good + "::::\n", codeMissingKey, "/slides/1/body/0/headings"},
		{"one heading", "::::pairs 分かったこと\n" + good + "::::\n", codeMissingKey, "/slides/1/body/0/headings"},
		{"one pair", "::::pairs A | B\n" + side("subject", "あ") + side("response", "い") + "::::\n", codeMisplaced, "/slides/1/body/0"},
		{"response first", "::::pairs A | B\n" + side("response", "い") + good + "::::\n", codeMisplaced, "/slides/1/body/0/pairs/0"},
		{"subject twice", "::::pairs A | B\n" + side("subject", "あ") + good + "::::\n", codeMisplaced, "/slides/1/body/0/pairs/0"},
		{"paragraph inside", "::::pairs A | B\n段落\n\n" + good + "::::\n", codeMisplaced, "/slides/1/body/0"},
		{"two paragraphs", "::::pairs A | B\n" + side("subject", "あ\n\nか") + side("response", "い") + side("subject", "う") + side("response", "え") + "::::\n", codeMisplaced, "/slides/1/body/0/pairs/0/subject"},
		{"note in a side", ":::::pairs A | B\n::::subject\n:::note\nあ\n:::\n::::\n" + side("response", "い") + side("subject", "う") + side("response", "え") + ":::::\n", codeMisplaced, "/slides/1/body/0/pairs/0/subject"},
		{"subject outside", side("subject", "あ"), codeMisplaced, "/slides/1/body/0"},
		{"unknown attribute", "::::pairs A | B {width=2}\n" + good + "::::\n", codeUnknownAttribute, "/slides/1/body/0"},
	} {
		_, issues := MarkdownToIR("t.bx.md", []byte(head+c.src))
		if len(issues) == 0 {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if issues[0].Code != c.code || issues[0].Path != c.path {
			t.Errorf("%s: %+v (want %s at %s)", c.name, issues[0], c.code, c.path)
		}
	}

	// 見出しの文の中の \| は | の文字。引用は ::::subject の中に :::quote。
	src := head + ":::::pairs A \\| B | C\n::::subject\n:::quote 利用者 C\n言葉\n:::\n::::\n" + side("response", "い") + side("subject", "う") + side("response", "え") + ":::::\n"
	ir, issues := MarkdownToIR("t.bx.md", []byte(src))
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readPairs(d.Slides[1].Body[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject.Plain() != "A | B" || p.Response.Plain() != "C" || p.Pairs[0].Subject.Type != "quote" || p.Pairs[0].Subject.Source.Plain() != "利用者 C" {
		t.Errorf("pairs = %+v", p)
	}
}

// IR の形の誤り(Schema と同じ決まり)は、Go でも場所つきで断る。
func TestPairsIRErrors(t *testing.T) {
	pair := `{"subject": {"type": "paragraph", "content": "a"}, "response": {"type": "paragraph", "content": "b"}}`
	for _, c := range []struct{ name, block, path string }{
		{"no headings", `{"type": "pairs", "pairs": [` + pair + `,` + pair + `]}`, "/slides/1/body/0"},
		{"positional headings", `{"type": "pairs", "headings": {"left": "a", "right": "b"}, "pairs": [` + pair + `,` + pair + `]}`, "/slides/1/body/0/headings/left"},
		{"missing heading", `{"type": "pairs", "headings": {"subject": "a"}, "pairs": [` + pair + `,` + pair + `]}`, "/slides/1/body/0/headings/response"},
		{"one pair", `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [` + pair + `]}`, "/slides/1/body/0"},
		{"note side", `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [` + pair + `, {"subject": {"type": "note", "content": "a"}, "response": {"type": "paragraph", "content": "b"}}]}`, "/slides/1/body/0/pairs/1/subject"},
		{"missing side", `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [` + pair + `, {"subject": {"type": "paragraph", "content": "a"}}]}`, "/slides/1/body/0/pairs/1/response"},
		{"unknown key in a pair", `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [` + pair + `, {"subject": {"type": "paragraph", "content": "a"}, "response": {"type": "paragraph", "content": "b"}, "width": 2}]}`, "/slides/1/body/0/pairs/1/width"},
	} {
		res := Check("deck.json", pairDeck("", c.block), CheckOptions{})
		if len(res.Errors) != 1 || res.Errors[0].Path != c.path {
			t.Errorf("%s: errors %+v (want one at %s)", c.name, res.Errors, c.path)
		}
	}
}

// pairs の中のブロックと見出しも、本文のブロックと同じ意味の検査(empty-text・
// variant-exists)を通る。場所は IR の場所で、組み方を持つテンプレート(dark・light)
// でも代わりの組み方(columns)で組むテンプレートでも同じ。
func TestPairsSemanticCheck(t *testing.T) {
	block := `{"type": "pairs", "headings": {"subject": "  ", "response": "打ち手"}, "pairs": [
	  {"subject": {"type": "quote", "content": " ", "source": ""}, "response": {"type": "paragraph", "content": " "}},
	  {"subject": {"type": "paragraph", "content": "a", "variant": "nope"}, "response": {"type": "points", "items": ["x"]}}]}`
	want := []string{
		"empty-text /slides/1/body/0/headings/subject",
		"empty-text /slides/1/body/0/pairs/0/subject/content",
		"empty-text /slides/1/body/0/pairs/0/subject/source",
		"empty-text /slides/1/body/0/pairs/0/response/content",
		"variant-exists /slides/1/body/0/pairs/1/subject/variant",
	}
	for _, c := range []struct {
		name string
		opts CheckOptions
	}{
		{"light", CheckOptions{Template: "light"}},
		{"dark", CheckOptions{Template: "dark"}},
		{"fallback", CheckOptions{Values: noPairsTemplate(t)}},
	} {
		res := Check("deck.json", pairDeck("", block), c.opts)
		var got []string
		for _, e := range res.Errors {
			got = append(got, e.Code+" "+e.Path)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: errors\n%s\nwant\n%s", c.name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}
