package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// layoutText は layout の文字の葉の文。
func layoutText(n *LayoutNode) string {
	if n == nil {
		return ""
	}
	if n.Text != nil {
		return *n.Text
	}
	var b strings.Builder
	for _, r := range n.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// findSubsystemNode は n の下で名前 name のサブシステムの箱を探す。
func findSubsystemNode(n *LayoutNode, name string) *LayoutNode {
	if n == nil {
		return nil
	}
	if n.Subsystem != nil && n.Subsystem.Name == name {
		return n
	}
	for _, c := range n.Children {
		if f := findSubsystemNode(c, name); f != nil {
			return f
		}
	}
	return nil
}

// 例の deck を report・dark・light で組むと、塊は上から順に重ならずに並ぶ。report は
// 左に丸い印(label)を置いて題を基調色で書き、塊の間に罫線を引く。dark・light は
// 印を題の頭に書き、題を太字にする。
func TestTopicsBuild(t *testing.T) {
	titles := []string{"通いやすさ", "続けやすさ", "公平さ"}
	labels := []string{"A", "B", "C"}
	for _, f := range []string{"../examples/md/topics.bx.md", "../examples/ir/topics.json"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, tmpl := range []string{"report", "dark", "light"} {
			deck, err := ReadDeck(f, data)
			if err != nil {
				t.Fatal(err)
			}
			var pptx bytes.Buffer
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
			if block == nil || block.Marks == nil || block.Marks.IRType != "topics" || block.Marks.TopicRows != 3 {
				t.Fatalf("%s@%s: block marks %+v", f, tmpl, block)
			}
			// report は塊の間に罫線(塊3つ+罫線2本)、dark・light は塊だけ。
			wantKids := 3
			if tmpl == "report" {
				wantKids = 5
			}
			if len(block.Children) != wantKids {
				t.Errorf("%s@%s: %d children in the block, want %d", f, tmpl, len(block.Children), wantKids)
			}
			bottom := block.Box.Y
			for k := range titles {
				tp := fmt.Sprintf("/slides/1/body/0/topics/%d", k)
				row, title := seen[tp], seen[tp+"/title"]
				if row == nil || title == nil {
					t.Fatalf("%s@%s: topic %d boxes missing", f, tmpl, k)
				}
				if row.Box.Y < bottom-0.01 {
					t.Errorf("%s@%s: topic %d starts at %.2f above the previous bottom %.2f", f, tmpl, k, row.Box.Y, bottom)
				}
				bottom = row.Box.Y + row.Box.Height
				if seen[tp+"/body/0"] == nil {
					t.Errorf("%s@%s: topic %d body box missing", f, tmpl, k)
				}
				mark := findSubsystemNode(row, "topicLabel")
				got := layoutText(title)
				if tmpl == "report" {
					if got != titles[k] || mark == nil {
						t.Errorf("%s@%s: topic %d title %q, mark %v", f, tmpl, k, got, mark != nil)
					}
					if mark != nil && mark.Box.X+mark.Box.Width > title.Box.X+0.01 {
						t.Errorf("%s@%s: topic %d mark is not left of the title", f, tmpl, k)
					}
					continue
				}
				if want := labels[k] + " " + titles[k]; got != want || mark != nil {
					t.Errorf("%s@%s: topic %d title %q (want %q), mark %v", f, tmpl, k, got, want, mark != nil)
				}
				bold := title.Bold
				for _, r := range title.Runs {
					bold = bold || r.Bold
				}
				if !bold {
					t.Errorf("%s@%s: topic %d title is not bold", f, tmpl, k)
				}
			}
		}
	}
}

// Markdown の例を IR にすると examples/ir/topics.json と同じ。
func TestTopicsMarkdownIR(t *testing.T) {
	src, err := os.ReadFile("../examples/md/topics.bx.md")
	if err != nil {
		t.Fatal(err)
	}
	ir, issues := MarkdownToIR("topics.bx.md", src)
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	want, err := os.ReadFile("../examples/ir/topics.json")
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(ir, want) {
		t.Errorf("topics.bx.md differs from topics.json:\n%s", ir)
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readTopics(d.Slides[1].Body[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Topics) != 3 || p.Topics[2].Label != "C" || len(p.Topics[2].Body) != 2 || p.Topics[2].Body[1].Type != "quote" {
		t.Errorf("topics = %+v", p)
	}

	// 印の無い塊の並び。
	head := "---\ntitle: T\n---\n\n# 題\n\n"
	ir, issues = MarkdownToIR("t.bx.md", []byte(head+"::::topics\n:::topic 一\nあ\n:::\n:::topic 二\n- い\n- う\n:::\n::::\n"))
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	if d, err = ParseIR(ir); err != nil {
		t.Fatal(err)
	}
	if p, err = readTopics(d.Slides[1].Body[0].Raw); err != nil || p.Topics[0].Label != "" || p.Topics[1].Body[0].Type != "points" {
		t.Errorf("topics = %+v, %v", p, err)
	}
}

// Markdown の形の誤りは場所つきで返す。
func TestTopicsMarkdownErrors(t *testing.T) {
	head := "---\ntitle: T\n---\n\n# 題\n\n"
	topic := func(label, body string) string { return ":::topic 題 " + label + "\n" + body + "\n:::\n" }
	two := topic("{label=A}", "あ") + topic("{label=B}", "い")
	for _, c := range []struct {
		name, src, code, path string
	}{
		{"one topic", "::::topics\n" + topic("{label=A}", "あ") + "::::\n", codeMisplaced, "/slides/1/body/0/topics"},
		{"seven topics", "::::topics\n" + strings.Repeat(topic("", "あ"), 7) + "::::\n", codeMisplaced, "/slides/1/body/0/topics"},
		{"mixed labels", "::::topics\n" + topic("{label=A}", "あ") + topic("", "い") + "::::\n", codeInvalidValue, "/slides/1/body/0/topics"},
		{"long label", "::::topics\n" + topic("{label=ABC}", "あ") + topic("{label=B}", "い") + "::::\n", codeInvalidValue, "/slides/1/body/0/topics/0/label"},
		{"no title", "::::topics\n:::topic\nあ\n:::\n" + two + "::::\n", codeMissingKey, "/slides/1/body/0/topics/0/title"},
		{"empty topic", "::::topics\n:::topic 題\n:::\n" + two + "::::\n", codeMisplaced, "/slides/1/body/0/topics/0/body"},
		{"four blocks", "::::topics\n" + topic("{label=A}", "あ\n\nか\n\nさ\n\nた") + topic("{label=B}", "い") + "::::\n", codeMisplaced, "/slides/1/body/0/topics/0/body"},
		{"table", "::::topics\n" + topic("{label=A}", "| a | b |\n|---|---|\n| 1 | 2 |") + topic("{label=B}", "い") + "::::\n", codeMisplaced, "/slides/1/body/0/topics/0/body"},
		{"note", ":::::topics\n::::topic 題 {label=A}\n:::note\nあ\n:::\n::::\n" + topic("{label=B}", "い") + ":::::\n", codeMisplaced, "/slides/1/body/0/topics/0/body"},
		{"paragraph inside", "::::topics\n段落\n\n" + two + "::::\n", codeMisplaced, "/slides/1/body/0"},
		{"topic outside", topic("{label=A}", "あ"), codeMisplaced, "/slides/1/body/0"},
		{"unknown attribute", "::::topics {width=2}\n" + two + "::::\n", codeUnknownAttribute, "/slides/1/body/0"},
		{"unknown topic attribute", "::::topics\n" + topic("{label=A width=2}", "あ") + topic("{label=B}", "い") + "::::\n", codeUnknownAttribute, "/slides/1/body/0/topics/0"},
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
}

// IR の形の誤り(Schema と同じ決まりと、印の混在)は、Go でも場所つきで断る。
func TestTopicsIRErrors(t *testing.T) {
	tp := func(extra string) string {
		return `{"title": "題", ` + extra + `"body": [{"type": "paragraph", "content": "a"}]}`
	}
	block := func(topics ...string) string {
		return `{"type": "topics", "topics": [` + strings.Join(topics, ", ") + `]}`
	}
	six := slices.Repeat([]string{tp("")}, 6)
	for _, c := range []struct{ name, block, path string }{
		{"one topic", block(tp("")), "/slides/1/body/0"},
		{"seven topics", block(append(six, tp(""))...), "/slides/1/body/0"},
		{"mixed labels", block(tp(`"label": "A", `), tp("")), "/slides/1/body/0/topics/1/label"},
		{"long label", block(tp(`"label": "ABC", `), tp(`"label": "B", `)), "/slides/1/body/0/topics/0/label"},
		{"no title", block(`{"body": [{"type": "paragraph", "content": "a"}]}`, tp("")), "/slides/1/body/0/topics/0/title"},
		{"no body", block(`{"title": "題", "body": []}`, tp("")), "/slides/1/body/0/topics/0/body"},
		{"table", block(`{"title": "題", "body": [{"type": "table", "header": ["a"], "rows": [["1"]]}]}`, tp("")), "/slides/1/body/0/topics/0/body/0"},
		{"unknown key", block(tp(`"width": 2, `), tp("")), "/slides/1/body/0/topics/0/width"},
	} {
		res := Check("deck.json", pairDeck("", c.block), CheckOptions{})
		if len(res.Errors) != 1 || res.Errors[0].Path != c.path {
			t.Errorf("%s: errors %+v (want one at %s)", c.name, res.Errors, c.path)
		}
	}
	if res := Check("deck.json", pairDeck("", block(six...)), CheckOptions{}); len(res.Errors) > 0 {
		t.Errorf("six topics: %+v", res.Errors)
	}
}

// noTopicsTemplate は topics の組み方を持たない試験用のテンプレート(light の写しから外す)。
func noTopicsTemplate(t *testing.T) *Template {
	t.Helper()
	tm := mustTemplate("light")
	delete(tm.components.components, topicsType)
	if tm.components.has(topicsType) || !mustTemplate("light").components.has(topicsType) {
		t.Fatal("template copies share the component table")
	}
	return tm
}

// 組み方を持たないテンプレートは、警告 component-fallback を1件出して箇条で組む
// (題を親の項目、中身を子の項目)。それは書き換えた箇条をそのまま組んだものと同じ PPTX。
func TestTopicsFallback(t *testing.T) {
	data, err := os.ReadFile("../examples/ir/topics.json")
	if err != nil {
		t.Fatal(err)
	}
	res := Check("topics.json", data, CheckOptions{Values: noTopicsTemplate(t)})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	var fb []Issue
	for _, w := range res.Warnings {
		if w.Code == "component-fallback" {
			fb = append(fb, w)
		}
	}
	if len(fb) != 1 || fb[0].Path != "/slides/1/body/0" || !strings.Contains(fb[0].Message, `"topics"`) || !strings.Contains(fb[0].Message, "built as points") {
		t.Fatalf("component-fallback warnings = %+v", fb)
	}
	for _, name := range []string{"dark", "light", "report"} {
		for _, w := range Check("topics.json", data, CheckOptions{Template: name}).Warnings {
			if w.Code == "component-fallback" {
				t.Errorf("%s: %+v", name, w)
			}
		}
	}

	deck, err := ReadIR("topics.json", data)
	if err != nil {
		t.Fatal(err)
	}
	alts, _, err := topicsFallback(&deck.Slides[1].Body[0])
	if err != nil || len(alts) != 1 || alts[0].Type != "points" {
		t.Fatalf("fallback = %+v, %v", alts, err)
	}
	var got []string
	for _, it := range alts[0].Items {
		var kids []string
		for _, k := range it.Items {
			kids = append(kids, k.Content.Plain())
		}
		got = append(got, it.Content.Plain()+" > "+strings.Join(kids, " / "))
	}
	want := []string{
		"A 通いやすさ > 徒歩 15 分より遠い人には、近くの空き区画を先に案内する / 車で来る人には、駐車場に近い区画を回す",
		"B 続けやすさ > 初めての人には、相談役の区画の隣を割り振る。去年途中でやめた人の多くが、聞ける相手がいなかったと答えた",
		"C 公平さ > 抽選の結果と理由を、掲示板と便りの両方で知らせる / 外れた理由が分かれば来年も申し込める / 利用者 D、聞き取りにて",
	}
	if !slices.Equal(got, want) {
		t.Errorf("fallback items\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// 書き換えた箇条の IR(Schema の形)を組んだものと同じ PPTX。
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var items []any
	for _, it := range alts[0].Items {
		var kids []any
		for _, k := range it.Items {
			kids = append(kids, k.Content.Plain())
		}
		items = append(items, map[string]any{"content": it.Content.Plain(), "items": kids})
	}
	doc["slides"].([]any)[1].(map[string]any)["body"] = []any{map[string]any{"type": "points", "items": items}}
	fbJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if res := Check("fb.json", fbJSON, CheckOptions{Template: "light"}); len(res.Errors) > 0 {
		t.Fatalf("fallback IR fails check: %+v", res.Errors)
	}
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
	if !bytes.Equal(render(data, noTopicsTemplate(t)), render(fbJSON, mustTemplate("light"))) {
		t.Error("fallback build differs from building the rewritten points block")
	}
}

// はみ出しは塊ごとに測る:はみ出した塊ごとに layout-overflow が1件、場所はその塊。
// 数は容量(capacity)の塊の測り値と同じ。
func TestTopicsOverflowPerTopic(t *testing.T) {
	for _, f := range []string{"../examples/ir/check-warnings/topics-overflow.json", "../examples/md/warnings/topics-overflow.bx.md"} {
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
			if got := strings.Join(paths, " "); got != "/slides/1/body/1/topics/2 /slides/1/body/1/topics/3" {
				t.Fatalf("%s@%s: overflow paths %s", f, tmpl, got)
			}
			if strings.HasSuffix(f, ".md") && (warns[0].Line != 20 || warns[1].Line != 29) {
				t.Errorf("%s@%s: lines %d, %d", f, tmpl, warns[0].Line, warns[1].Line)
			}
			b := caps[0].Blocks[1]
			if b.Type != "topics" || len(b.Topics) != 4 {
				t.Fatalf("%s@%s: capacity %+v", f, tmpl, b)
			}
			for k, m := range b.Topics {
				if (m.Overflow > 0) != (k >= 2) {
					t.Errorf("%s@%s: topic %d overflow %.1f", f, tmpl, k, m.Overflow)
				}
				if k > 0 && m.Top < b.Topics[k-1].Bottom {
					t.Errorf("%s@%s: topic %d top %.1f above the previous bottom %.1f", f, tmpl, k, m.Top, b.Topics[k-1].Bottom)
				}
			}
			if b.Topics[3].Bottom != b.Bottom || b.Topics[0].Top != b.Top {
				t.Errorf("%s@%s: topics %+v, block %+v", f, tmpl, b.Topics, b)
			}
			if want := fmt.Sprintf("by %.1fpt (topic needs %.1fpt", b.Topics[2].Overflow, b.Topics[2].Need); !strings.Contains(warns[0].Message, want) {
				t.Errorf("%s@%s: %q does not contain %q", f, tmpl, warns[0].Message, want)
			}
			if strings.HasSuffix(f, ".md") && b.Topics[2].Line != 20 {
				t.Errorf("%s@%s: capacity line %d", f, tmpl, b.Topics[2].Line)
			}
		}
	}
}

// topics の題と中のブロックも、本文のブロックと同じ意味の検査(empty-text・
// variant-exists)を通る。場所は IR の場所で、組み方を持つテンプレートでも代わりの
// 組み方で組むテンプレートでも同じ。
func TestTopicsSemanticCheck(t *testing.T) {
	block := `{"type": "topics", "topics": [
	  {"title": "  ", "body": [{"type": "paragraph", "content": " "}]},
	  {"title": "題", "body": [{"type": "paragraph", "content": "a", "variant": "nope"}, {"type": "quote", "content": "言葉", "source": " "}]}]}`
	want := []string{
		"empty-text /slides/1/body/0/topics/0/title",
		"empty-text /slides/1/body/0/topics/0/body/0/content",
		"variant-exists /slides/1/body/0/topics/1/body/0/variant",
		"empty-text /slides/1/body/0/topics/1/body/1/source",
	}
	for _, c := range []struct {
		name string
		opts CheckOptions
	}{
		{"report", CheckOptions{Template: "report"}},
		{"dark", CheckOptions{Template: "dark"}},
		{"fallback", CheckOptions{Values: noTopicsTemplate(t)}},
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
