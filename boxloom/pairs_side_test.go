package boxloom

import (
	"bytes"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
)

// 対の側の見出しと複数のブロック(Backlog #87)の試験。例は架空の
// examples/md/pairs-side-body.bx.md(左の側は見出し+箇条、右の側は箇条+引用)と、
// それを parse した examples/ir/pairs-side-body.json。

const sideBodyMD = "../examples/md/pairs-side-body.bx.md"
const sideBodyIR = "../examples/ir/pairs-side-body.json"

// Markdown の :::subject 見出し と、中の複数のブロックは IR の {title, body} になる。
// IR の例は Markdown を parse したものと同じで、Go でも読める。
func TestPairsSideBodyMarkdown(t *testing.T) {
	md, err := os.ReadFile(sideBodyMD)
	if err != nil {
		t.Fatal(err)
	}
	ir, issues := MarkdownToIR("p.bx.md", md)
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	want, err := os.ReadFile(sideBodyIR)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir, want) {
		t.Errorf("%s differs from parsing %s", sideBodyIR, sideBodyMD)
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readPairs(d.Slides[1].Body[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	s0, r0, r1 := p.Pairs[0].Subject, p.Pairs[0].Response, p.Pairs[1].Response
	if !s0.Grouped || s0.Title.Plain() != "家賃と広さが合わない" || len(s0.Body) != 1 || s0.Body[0].Type != "points" {
		t.Errorf("subject 0 = %+v", s0)
	}
	if !r0.Grouped || r0.Title != nil || len(r0.Body) != 2 || r0.Body[0].Type != "points" || r0.Body[1].Type != "quote" {
		t.Errorf("response 0 = %+v", r0)
	}
	// 見出しの無い1つのブロックの側は、今の形のまま。
	if r1.Grouped || len(r1.Body) != 1 || r1.Body[0].Type != "points" {
		t.Errorf("response 1 = %+v", r1)
	}

	// 画像と表も側に置ける(表と画像の数も body の数に入る)。
	src := "---\ntitle: T\n---\n\n# 題\n\n::::pairs A | B\n:::subject 見出し\n| 列 | 値 |\n|---|---|\n| あ | 1 |\n\n![地図](map.png)\n:::\n:::response\nい\n:::\n:::subject\nう\n:::\n:::response\nえ\n:::\n::::\n"
	ir, issues = MarkdownToIR("t.bx.md", []byte(src))
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	d, err = ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	p, err = readPairs(d.Slides[1].Body[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	if b := p.Pairs[0].Subject.Body; len(b) != 2 || b[0].Type != "table" || b[1].Type != "image" {
		t.Errorf("subject with a table and an image = %+v", b)
	}
}

// IR の形の誤り:body が空・5個以上・置けない種類・続きの表・知らないキー。
func TestPairsSideBodyIRErrors(t *testing.T) {
	pair := `{"subject": {"type": "paragraph", "content": "a"}, "response": {"type": "paragraph", "content": "b"}}`
	with := func(side string) string {
		return `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [` + pair + `, {"subject": ` + side + `, "response": {"type": "paragraph", "content": "b"}}]}`
	}
	p := `{"type": "paragraph", "content": "x"}`
	for _, c := range []struct{ name, side, path string }{
		{"empty body", `{"title": "t", "body": []}`, "/slides/1/body/0/pairs/1/subject/body"},
		{"no body", `{"title": "t"}`, "/slides/1/body/0/pairs/1/subject/body"},
		{"five blocks", `{"body": [` + strings.Repeat(p+",", 4) + p + `]}`, "/slides/1/body/0/pairs/1/subject/body"},
		{"note in body", `{"body": [{"type": "note", "content": "x"}]}`, "/slides/1/body/0/pairs/1/subject/body/0"},
		{"continuing table", `{"body": [{"type": "table", "continues": true, "rows": [["a"]]}]}`, "/slides/1/body/0/pairs/1/subject/body/0"},
		{"unknown key", `{"body": [` + p + `], "width": 2}`, "/slides/1/body/0/pairs/1/subject/width"},
	} {
		res := Check("deck.json", pairDeck("", with(c.side)), CheckOptions{})
		if len(res.Errors) == 0 || !strings.HasPrefix(res.Errors[0].Path, c.path) {
			t.Errorf("%s: errors %+v (want one at %s)", c.name, res.Errors, c.path)
		}
	}
	// 見出しの無い {body} の形も通る。
	res := Check("deck.json", pairDeck("", with(`{"body": [`+p+`, `+p+`]}`)), CheckOptions{})
	if len(res.Errors) > 0 {
		t.Errorf("body without title: %+v", res.Errors)
	}
}

// 側の見出しと body のブロックも、本文のブロックと同じ意味の検査を通る(場所は IR の場所)。
func TestPairsSideBodySemanticCheck(t *testing.T) {
	block := `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [
	  {"subject": {"title": " ", "body": [{"type": "paragraph", "content": " "}, {"type": "quote", "content": "q", "source": " "}]}, "response": {"type": "paragraph", "content": "b"}},
	  {"subject": {"type": "paragraph", "content": "a"}, "response": {"body": [{"type": "points", "items": ["x"], "variant": "nope"}]}}]}`
	want := []string{
		"empty-text /slides/1/body/0/pairs/0/subject/title",
		"empty-text /slides/1/body/0/pairs/0/subject/body/0/content",
		"empty-text /slides/1/body/0/pairs/0/subject/body/1/source",
		"variant-exists /slides/1/body/0/pairs/1/response/body/0/variant",
	}
	for _, c := range []struct {
		name string
		opts CheckOptions
	}{{"light", CheckOptions{Template: "light"}}, {"report", CheckOptions{Template: "report"}}, {"fallback", CheckOptions{Values: noPairsTemplate(t)}}} {
		var got []string
		for _, e := range Check("deck.json", pairDeck("", block), c.opts).Errors {
			got = append(got, e.Code+" "+e.Path)
		}
		slices.Sort(got)
		w := slices.Clone(want)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s: errors\n%s\nwant\n%s", c.name, strings.Join(got, "\n"), strings.Join(w, "\n"))
		}
	}
}

// dark・light は側の見出しと body を縦に積む:見出し → body[0] → body[1] の順に上から、
// 同じ左端。側の高さは中身を積んだ高さで、対の行は高い方の側に合わせる。
func TestPairsSideBodyStacks(t *testing.T) {
	data, err := os.ReadFile(sideBodyMD)
	if err != nil {
		t.Fatal(err)
	}
	for _, tmpl := range []string{"dark", "light", "report"} {
		res := Check("p.bx.md", data, CheckOptions{Template: tmpl})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %+v", tmpl, res.Errors)
		}
		for _, w := range res.Warnings {
			if w.Code == "fill-ratio" {
				continue // 詰まった report では例の本文が少なく見える(積み方の試験とは別の話)
			}
			t.Errorf("%s: unexpected warning %+v", tmpl, w)
		}
		dump, err := Layout("p.bx.md", data, BuildOptions{Template: tmpl})
		if err != nil {
			t.Fatal(err)
		}
		seen := pairsLayoutNodes(dump)
		const pp = "/slides/1/body/0/pairs/"
		for _, side := range []struct {
			path  string
			parts []string
		}{
			{pp + "0/subject", []string{"title", "body/0"}},
			{pp + "0/response", []string{"body/0", "body/1"}},
			{pp + "1/subject", []string{"title", "body/0"}},
		} {
			s := seen[side.path]
			if s == nil {
				t.Fatalf("%s: no box %s", tmpl, side.path)
			}
			prev := s.Box.Y - 0.01
			for _, part := range side.parts {
				n := seen[side.path+"/"+part]
				if n == nil {
					t.Fatalf("%s: no box %s/%s", tmpl, side.path, part)
				}
				if n.Box.Y < prev || !nearPt(n.Box.X, s.Box.X) {
					t.Errorf("%s: %s/%s at (%.1f, %.1f) is not stacked under the previous part (side x %.1f)", tmpl, side.path, part, n.Box.X, n.Box.Y, s.Box.X)
				}
				prev = n.Box.Y + n.Box.Height
			}
			if last := seen[side.path+"/"+side.parts[len(side.parts)-1]]; !nearPt(last.Box.Y+last.Box.Height, s.Box.Y+s.Box.Height) && s.Box.Y+s.Box.Height < last.Box.Y+last.Box.Height {
				t.Errorf("%s: %s does not hold its last block", tmpl, side.path)
			}
		}
		// 容量の側の高さは、側の見出しと body を積んだ高さ(layout の側の中身)。
		_, _, caps := checkCaps(t, data, tmpl)
		b := caps[0].Blocks[0]
		if b.Type != "pairs" || len(b.Pairs) != 2 {
			t.Fatalf("%s: capacity %+v", tmpl, b)
		}
		for k, pc := range b.Pairs {
			row := seen[pp+string(rune('0'+k))]
			if math.Abs(pc.Need-row.Box.Height) > 0.11 || math.Abs(pc.Need-math.Max(pc.Subject, pc.Response)) > 0.11 {
				t.Errorf("%s: pair %d capacity need %.1f (subject %.1f, response %.1f), row %.1f", tmpl, k, pc.Need, pc.Subject, pc.Response, row.Box.Height)
			}
			// 見出しと複数のブロックの側の高さは、見出しと body を積んだ高さ。
			sub := seen[pp+string(rune('0'+k))+"/subject"]
			last := seen[pp+string(rune('0'+k))+"/subject/body/0"]
			if math.Abs(pc.Subject-(last.Box.Y+last.Box.Height-sub.Box.Y)) > 0.11 {
				t.Errorf("%s: pair %d subject capacity %.1f, stacked %.1f", tmpl, k, pc.Subject, last.Box.Y+last.Box.Height-sub.Box.Y)
			}
		}
	}
}

// checkCaps は Check の容量を返す。
func checkCaps(t *testing.T, data []byte, tmpl string) ([]Issue, []Issue, []SlideCapacity) {
	t.Helper()
	res := Check("p.bx.md", data, CheckOptions{Template: tmpl})
	return res.Errors, res.Warnings, res.Capacity
}

// はみ出しは側の body まで測る:見出しと複数のブロックの側が長い対は、その対の
// layout-overflow になり、数は容量の対の測り値と同じ。
func TestPairsSideBodyOverflow(t *testing.T) {
	var b strings.Builder
	b.WriteString("---\ntitle: T\n---\n\n# はみ出す対\n\n:::::pairs 特筆すべき点 | 聞き取りの声\n")
	for k := 0; k < 3; k++ {
		b.WriteString(":::subject 要因の名前\n- 一つ目の点\n- 二つ目の点\n:::\n::::response\n")
		for i := 0; i < 3; i++ {
			b.WriteString("- 聞き取りの声を長めに書いて、側の高さを伸ばすための文をここに置く\n- 二つ目の声\n\n")
		}
		b.WriteString(":::quote 利用者 C\n言葉\n:::\n::::\n")
	}
	b.WriteString(":::::\n")
	for _, tmpl := range []string{"light", "report"} {
		res := Check("o.bx.md", []byte(b.String()), CheckOptions{Template: tmpl})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %+v", tmpl, res.Errors)
		}
		var paths []string
		for _, w := range res.Warnings {
			if w.Code == "layout-overflow" {
				paths = append(paths, w.Path)
			}
		}
		if len(paths) == 0 || !strings.HasPrefix(paths[0], "/slides/1/body/0/pairs/") {
			t.Fatalf("%s: overflow paths %v (warnings %+v)", tmpl, paths, res.Warnings)
		}
		bc := res.Capacity[0].Blocks[0]
		last := bc.Pairs[len(bc.Pairs)-1]
		if last.Overflow <= 0 || last.Tallest != "response" || last.Response <= last.Subject {
			t.Errorf("%s: last pair %+v", tmpl, last)
		}
	}
}

// 代わりの組み方:側の見出しはまとまりの中の強調の段落、body のブロックは順に入る。
// 場所の表は見出しを …/title に、body のブロックを …/body/I に写す。
func TestPairsSideBodyFallback(t *testing.T) {
	data, err := os.ReadFile(sideBodyIR)
	if err != nil {
		t.Fatal(err)
	}
	deck, err := ReadIR("p.json", data)
	if err != nil {
		t.Fatal(err)
	}
	alts, paths, err := pairsFallback(&deck.Slides[1].Body[0])
	if err != nil || len(alts) != 1 {
		t.Fatalf("%+v %v", alts, err)
	}
	var types [2][]string
	for i, c := range alts[0].Columns {
		for _, b := range c.Body {
			types[i] = append(types[i], b.Type)
		}
	}
	if got := strings.Join(types[0], ","); got != "paragraph,points,paragraph,points" {
		t.Errorf("subject column = %s", got)
	}
	if got := strings.Join(types[1], ","); got != "points,paragraph,note,points" {
		t.Errorf("response column = %s", got)
	}
	title := alts[0].Columns[0].Body[0].Content
	if len(title) != 1 || title[0].Type != "em" || title[0].Content.Plain() != "家賃と広さが合わない" {
		t.Errorf("side title in the fallback = %+v", title)
	}
	for at, want := range map[string]fallbackPath{
		"/columns/0/body/0": {To: "/pairs/0/subject/title"},
		"/columns/0/body/1": {To: "/pairs/0/subject/body/0", Same: true},
		"/columns/1/body/1": {To: "/pairs/0/response/body/1"},
		"/columns/1/body/2": {To: "/pairs/0/response/body/1/source"},
		"/columns/1/body/3": {To: "/pairs/1/response", Same: true},
	} {
		if paths[at] != want {
			t.Errorf("path %s = %+v, want %+v", at, paths[at], want)
		}
	}
	res := Check("p.json", data, CheckOptions{Values: noPairsTemplate(t)})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	n := 0
	for _, w := range res.Warnings {
		if w.Code == "component-fallback" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("component-fallback warnings: %d", n)
	}
}

// report の pairs:対ごとに A・B・C の四角い印(基調色・白い字)を subject の頭に、側の
// 間に丸い山形の印を置き、対の間に罫線(grid)を引く。側の見出しは帯を置かず基調色の字だけ。
// dark・light にはどれも無い。
func TestReportPairsMarks(t *testing.T) {
	md, err := os.ReadFile(sideBodyMD)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		data []byte
		n    int
	}{{"side-body", md, 2}, {"three pairs", reportPairsDeck(3), 3}} {
		for _, tmpl := range []string{"report", "light", "dark"} {
			deck, err := ReadDeck("p.bx.md", c.data)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := Build(&buf, deck, BuildOptions{Template: tmpl}); err != nil {
				t.Fatal(err)
			}
			xml := zipFiles(t, buf.Bytes())["ppt/slides/slide2.xml"]
			tm := mustTemplate(tmpl)
			accent, grid, card, white := tm.Theme["accent"], tm.Theme["grid"], tm.Theme["card"], tm.Theme["bg"]
			chevrons := strings.Count(xml, `prst="chevron"`)
			// 引用の人の印(頭の円と肩)と結論の丸い矢印(円と矢印)の円は数えない(#88)。
			ellipses := strings.Count(xml, `prst="ellipse"`) - strings.Count(xml, `prst="round2SameRect"`) - strings.Count(xml, `prst="rightArrow"`)
			gridRects := strings.Count(xml, `<a:prstGeom prst="rect"></a:prstGeom>
<a:solidFill><a:srgbClr val="`+grid+`"/>`)
			if tmpl != "report" {
				if chevrons != 0 || ellipses != 0 {
					t.Errorf("%s@%s: marks drawn (%d chevrons)", c.name, tmpl, chevrons)
				}
				continue
			}
			if chevrons != c.n || ellipses != c.n {
				t.Errorf("%s: %d chevrons, %d circles, want %d", c.name, chevrons, ellipses, c.n)
			}
			for k := 0; k < c.n; k++ {
				letter := string(rune('A' + k))
				want := `<a:solidFill><a:srgbClr val="` + accent + `"/></a:solidFill>`
				i := strings.Index(xml, `>`+letter+`</a:t>`)
				if i < 0 {
					t.Errorf("%s: no mark %s", c.name, letter)
					continue
				}
				head := xml[strings.LastIndex(xml[:i], "<p:sp>"):i]
				if !strings.Contains(head, want) || !strings.Contains(head, `val="`+white+`"`) {
					t.Errorf("%s: mark %s is not white on the accent color", c.name, letter)
				}
			}
			if gridRects != c.n-1 {
				t.Errorf("%s: %d rules between pairs, want %d", c.name, gridRects, c.n-1)
			}
			// 側の見出しは帯(面)を置かず、基調色の字だけ(#103)。
			if c.name == "side-body" {
				for _, title := range []string{"家賃と広さが合わない", "持ち主と借り手が出会わない"} {
					i := strings.Index(xml, `>`+title+`</a:t>`)
					if i < 0 {
						t.Errorf("%s: no side title %q", c.name, title)
						continue
					}
					head := xml[strings.LastIndex(xml[:i], "<p:sp>"):i]
					if strings.Contains(head, `val="`+card+`"`) {
						t.Errorf("%s: side title %q has a band", c.name, title)
					}
					if !strings.Contains(head, `val="`+accent+`"`) {
						t.Errorf("%s: side title %q is not in the accent color", c.name, title)
					}
				}
			}
		}
	}
	if pairLetter(0) != "A" || pairLetter(25) != "Z" || pairLetter(26) != "AA" {
		t.Error("pairLetter")
	}
}
