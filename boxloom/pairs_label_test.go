package boxloom

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// 対の名前(label。Backlog #118)の試験。例は架空の examples/md/pairs-label.bx.md
// (1枚目は2字までの名前、2枚目は3字以上の名前)と、それを parse した
// examples/ir/pairs-label.json。

const labelMD = "../examples/md/pairs-label.bx.md"
const labelIR = "../examples/ir/pairs-label.json"

// :::subject {label=…} は IR の対の label になる(空白は " で囲む)。IR の例は
// Markdown を parse したものと同じで、Go でも読める。
func TestPairsLabelMarkdown(t *testing.T) {
	md, err := os.ReadFile(labelMD)
	if err != nil {
		t.Fatal(err)
	}
	ir, issues := MarkdownToIR("p.bx.md", md)
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	want, err := os.ReadFile(labelIR)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir, want) {
		t.Errorf("%s differs from parsing %s", labelIR, labelMD)
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	for si, wl := range [][]string{{"1", "5", "7", "9"}, {"夕方 18 時以降", "週末の催し", "雨の日", "子ども連れ"}} {
		p, err := readPairs(d.Slides[si+1].Body[0].Raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.labels(); strings.Join(got, "|") != strings.Join(wl, "|") {
			t.Errorf("slide %d labels %q, want %q", si+1, got, wl)
		}
	}
	// 名前の無い pairs は labels が nil(今までどおり)。
	data, _ := os.ReadFile("../examples/ir/pairs.json")
	d, err = ParseIR(data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readPairs(d.Slides[1].Body[0].Raw)
	if err != nil || p.labels() != nil {
		t.Errorf("pairs without labels: %v %q", err, p.labels())
	}
}

func labelSrc(sub1, sub2 string) string {
	return "---\ntitle: T\n---\n\n# 題\n\n::::pairs A | B\n:::subject" + sub1 + "\nあ\n:::\n:::response\nい\n:::\n:::subject" + sub2 + "\nう\n:::\n:::response\nえ\n:::\n::::\n"
}

// Markdown の誤り:名前のある対と無い対が混ざる(pairs-label)・長すぎる・空・
// response に書く。
func TestPairsLabelMarkdownErrors(t *testing.T) {
	for _, c := range []struct {
		name, src, code, path string
		line                  int
	}{
		{"mixed (second missing)", labelSrc(" {label=1}", ""), "pairs-label", "/slides/1/body/0/pairs/1", 14},
		{"mixed (second has)", labelSrc("", " {label=1}"), "pairs-label", "/slides/1/body/0/pairs/1/label", 14},
		{"too long", labelSrc(" {label=あいうえおかきくけこさしす}", " {label=2}"), "invalid-value", "/slides/1/body/0/pairs/0/label", 8},
		{"blank", labelSrc(` {label=" "}`, " {label=2}"), "invalid-value", "/slides/1/body/0/pairs/0/label", 8},
		{"on response", strings.Replace(labelSrc("", ""), ":::response\nい", ":::response {label=1}\nい", 1), "unknown-attribute", "/slides/1/body/0/pairs/0/response", 11},
	} {
		_, issues := MarkdownToIR("t.bx.md", []byte(c.src))
		if len(issues) != 1 || issues[0].Code != c.code || issues[0].Path != c.path || issues[0].Line != c.line {
			t.Errorf("%s: issues %+v (want one %s at %s line %d)", c.name, issues, c.code, c.path, c.line)
		}
	}
	// 12字の名前と、空白を含む名前は通る。
	ir, issues := MarkdownToIR("t.bx.md", []byte(labelSrc(" {label=あいうえおかきくけこさし}", ` {label="第 2 の 対"}`)))
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	if !strings.Contains(string(ir), `"label": "第 2 の 対"`) {
		t.Errorf("quoted label: %s", ir)
	}
}

// IR の誤り:混ざれば check の pairs-label、長すぎる・空の名前は Schema と同じ誤り。
func TestPairsLabelIRErrors(t *testing.T) {
	side := `"subject": {"type": "paragraph", "content": "a"}, "response": {"type": "paragraph", "content": "b"}`
	block := func(l0, l1 string) string {
		return `{"type": "pairs", "headings": {"subject": "a", "response": "b"}, "pairs": [{` + l0 + side + `}, {` + l1 + side + `}]}`
	}
	for _, c := range []struct{ name, block, code, path string }{
		{"mixed", block(`"label": "1", `, ""), "pairs-label", "/slides/1/body/0/pairs/1"},
		{"mixed (second has)", block("", `"label": "1", `), "pairs-label", "/slides/1/body/0/pairs/1/label"},
		{"too long", block(`"label": "あいうえおかきくけこさしす", `, `"label": "1", `), "schema", "/slides/1/body/0/pairs/0/label"},
		{"blank", block(`"label": "  ", `, `"label": "1", `), "schema", "/slides/1/body/0/pairs/0/label"},
	} {
		for _, tmpl := range []string{"report", "light"} {
			res := Check("deck.json", pairDeck(tmpl, c.block), CheckOptions{})
			if len(res.Errors) != 1 || res.Errors[0].Code != c.code || res.Errors[0].Path != c.path {
				t.Errorf("%s@%s: errors %+v (want one %s at %s)", c.name, tmpl, res.Errors, c.code, c.path)
			}
		}
	}
	// Go の読み込みも、混ざったものを断る(check を通さずに組むとき)。
	if _, err := readPairs(json.RawMessage(block(`"label": "1", `, ""))); err == nil || !strings.Contains(err.Error(), "pairs[1]: write a label on every pair") {
		t.Errorf("readPairs mixed: %v", err)
	}
}

// Schema(boxloom schema の一覧)は対の label と、:::subject の属性 label を名乗る。
func TestPairsLabelSchema(t *testing.T) {
	out, err := Schema("report")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `"maxLength": 12`) || !strings.Contains(s, "対の短い名前") {
		t.Error("schema has no pair label")
	}
	d, _ := blockTypes.decl(pairsType)
	for _, b := range d.Markdown {
		if b.Name == "subject" && (len(b.Attributes) != 1 || b.Attributes[0] != "label") {
			t.Errorf("subject attributes %q", b.Attributes)
		}
	}
}

// labelBuild は例を tmpl で組んだ PPTX の n 枚目のスライドの XML と layout の木。
func labelBuild(t *testing.T, tmpl string, data []byte, n int) (string, map[string]*LayoutNode) {
	t.Helper()
	deck, err := ReadDeck("p.bx.md", data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{Template: tmpl}); err != nil {
		t.Fatal(err)
	}
	dump, err := Layout("p.bx.md", data, BuildOptions{Template: tmpl})
	if err != nil {
		t.Fatal(err)
	}
	return zipFiles(t, buf.Bytes())["ppt/slides/slide"+string(rune('0'+n))+".xml"], pairsLayoutNodes(dump)
}

// shapeOf は文 text を持つ図形の XML(無ければ "")。
func shapeOf(xml, text string) string {
	i := strings.Index(xml, `>`+text+`</a:t>`)
	if i < 0 {
		return ""
	}
	return xml[strings.LastIndex(xml[:i], "<p:sp>"):i]
}

// report:2字までの名前は A・B・C の代わりに四角い印(基調色の面に白い字)の中に、
// 3字以上の名前があれば印を置かず、行の左の見出しの列に基調色の太字で書く。
func TestReportPairsLabel(t *testing.T) {
	data, err := os.ReadFile(labelMD)
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate("report")
	accent, white := tm.Theme["accent"], tm.Theme["bg"]

	// 1枚目:印の中に 1・5・7・9。A・B は無い。
	xml, _ := labelBuild(t, "report", data, 2)
	for _, l := range []string{"1", "5", "7", "9"} {
		sh := shapeOf(xml, l)
		if sh == "" || !strings.Contains(sh, `prst="rect"`) || !strings.Contains(sh, `<a:srgbClr val="`+accent+`"/></a:solidFill>`) || !strings.Contains(sh, `val="`+white+`"`) {
			t.Errorf("label %s is not in a mark: %q", l, sh)
		}
	}
	for _, l := range []string{"A", "B"} {
		if shapeOf(xml, l) != "" {
			t.Errorf("letter mark %s drawn with labels", l)
		}
	}

	// 2枚目:見出しの列。印(四角の A・B…)は無く、名前は subject の左に、基調色の太字。
	xml, seen := labelBuild(t, "report", data, 3)
	for _, l := range []string{"A", "B"} {
		if shapeOf(xml, l) != "" {
			t.Errorf("letter mark %s drawn with long labels", l)
		}
	}
	hs := seen["/slides/2/body/0/headings/subject"]
	for k, l := range []string{"週末の催し", "雨の日", "子ども連れ"} {
		sh := shapeOf(xml, l)
		if sh == "" || !strings.Contains(sh, `b="1"`) || !strings.Contains(sh, `val="`+accent+`"`) || !strings.Contains(sh, "<a:noFill/>\n</p:spPr>") {
			t.Errorf("label %s is not an accent bold heading: %q", l, sh)
		}
		pp := "/slides/2/body/0/pairs/" + string(rune('1'+k))
		row, s := seen[pp], seen[pp+"/subject"]
		if row == nil || s == nil || len(row.Children) < 3 {
			t.Fatalf("pair %d boxes missing", k+1)
		}
		col := row.Children[0]
		if !nearPt(col.Box.X, row.Box.X) || col.Box.X+col.Box.Width > s.Box.X || !nearPt(col.Box.Y, s.Box.Y) {
			t.Errorf("label column %+v is not left of the subject %+v", col.Box, s.Box)
		}
		if !nearPt(col.Box.Width, 80) {
			t.Errorf("label column width %.2f, want 80", col.Box.Width)
		}
		// 見出しは側の真上。
		if !nearPt(hs.Box.X, s.Box.X) {
			t.Errorf("subject heading x %.2f, subject x %.2f", hs.Box.X, s.Box.X)
		}
	}
}

// dark・light は名前を subject の頭(中身の上)に、基調色の太字で書く。印・列は無い。
func TestPairsLabelStandard(t *testing.T) {
	src := "---\ntitle: T\n---\n\n# 題\n\n::::pairs 分かったこと | 打ち手\n:::subject {label=要因1}\n返却は夜に集まる\n:::\n:::response\n箱を増やす\n:::\n:::subject {label=要因2}\n雨で本が濡れる\n:::\n:::response\n口に庇を付ける\n:::\n::::\n"
	for _, tmpl := range []string{"dark", "light"} {
		xml, seen := labelBuild(t, tmpl, []byte(src), 2)
		accent := mustTemplate(tmpl).Theme["accent"]
		for k, l := range []string{"要因1", "要因2"} {
			sh := shapeOf(xml, l)
			if sh == "" || !strings.Contains(sh, `b="1"`) || !strings.Contains(sh, `val="`+accent+`"`) {
				t.Errorf("%s: label %s is not accent bold: %q", tmpl, l, sh)
			}
			s := seen["/slides/1/body/0/pairs/"+string(rune('0'+k))+"/subject"]
			if s == nil || len(s.Children) != 2 || s.Children[0].Box.Y >= s.Children[1].Box.Y || !nearPt(s.Children[0].Box.Y, s.Box.Y) {
				t.Errorf("%s: label is not at the head of subject %d", tmpl, k)
			}
		}
	}
	// 組み方を持たないテンプレートは、名前を subject のまとまりの強調の段落にする。
	deck, err := ReadDeck("p.bx.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, deck, BuildOptions{Values: noPairsTemplate(t)}); err != nil {
		t.Fatal(err)
	}
	if xml := zipFiles(t, buf.Bytes())["ppt/slides/slide2.xml"]; !strings.Contains(xml, ">要因1</a:t>") || !strings.Contains(xml, ">要因2</a:t>") {
		t.Error("fallback lost the labels")
	}
	res := Check("p.bx.md", []byte(src), CheckOptions{Values: noPairsTemplate(t)})
	if len(res.Errors) > 0 {
		t.Errorf("fallback errors %+v", res.Errors)
	}
}
