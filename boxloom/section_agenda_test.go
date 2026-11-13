package boxloom

import (
	"fmt"
	"strings"
	"testing"
)

// parseAgendaDeck は tocDeck の2つ目の項目(案内の見直し)の扉に agenda=true を付けた文書。
// 今の項目の番号も返す。
func parseAgendaDeck(t *testing.T) (*IRDeck, int) {
	t.Helper()
	md := strings.Replace(tocDeck, "# 案内の見直し {type=section level=3}", "# 案内の見直し {type=section level=3 agenda=true}", 1)
	d, _, err := ParseMarkdown("agenda.bx.md", []byte(md))
	if err != nil {
		t.Fatal(err)
	}
	for i := range d.Slides {
		if d.Slides[i].Agenda {
			return d, i
		}
	}
	t.Fatal("agenda is not in the IR")
	return nil, 0
}

// 同じ親の下の同じ level の区切りを数える(ほかの節・章の区切りは入れない)。
func TestSectionSiblings(t *testing.T) {
	d, i := parseAgendaDeck(t)
	kids := sectionChildren(d, sectionParent(d, i))
	if fmt.Sprint(sectionSiblings(d, i)) != fmt.Sprint(kids) || len(kids) != 2 || kids[1] != i {
		t.Fatalf("siblings %v, parent's children %v, i %d", sectionSiblings(d, i), kids, i)
	}
	if sectionSiblings(d, sectionParent(d, sectionParent(d, i))) != nil {
		t.Error("a level 1 section has siblings")
	}
}

// report の agenda の項目の扉は、親の節の扉と同じ frame と同じ一覧を描き、今の項目を
// 白の太字(題のプレースホルダー)、ほかを淡い色にする。題の場所は親の題。
func TestReportSectionAgenda(t *testing.T) {
	d, i := parseAgendaDeck(t)
	tm, err := LookupTemplate("report")
	if err != nil {
		t.Fatal(err)
	}
	b, err := tm.BuildSlide(d, i)
	if err != nil {
		t.Fatal(err)
	}
	parent := sectionParent(d, i)
	pb, err := tm.BuildSlide(d, parent)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := specTexts(b.Root), specTexts(pb.Root); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("agenda texts %v, parent door texts %v", got, want)
	}
	if b.Root.Background != tm.Theme["accent"] || b.Main == nil || b.Main.Background != tm.Theme["bg"] {
		t.Error("agenda door is not the frame")
	}
	cur := placeholderLeaf(b.Root, "title")
	if cur == nil || runsText(cur.Runs) != "案内の見直し" || !leafBold(cur) || runColor(cur) != tm.Theme["bg"] || cur.FontSize != 20 {
		t.Fatalf("current item %+v", cur)
	}
	other := leafWithPlain(b.Main, "夜間の返却")
	if other == nil || leafBold(other) || runColor(other) == runColor(cur) {
		t.Errorf("other item is not quieter: %+v", other)
	}
	head := leafWithPlain(b.Main, "返却の流れ")
	if head == nil || head.FontSize != 54 || head.Placeholder != "" {
		t.Errorf("heading %+v", head)
	}
	// agenda の無い項目の扉は簡素なまま。
	if prev := sectionSiblings(d, i)[0]; true {
		pb, err := tm.BuildSlide(d, prev)
		if err != nil {
			t.Fatal(err)
		}
		if got := specTexts(pb.Root); len(got) != 1 {
			t.Errorf("plain door texts %v", got)
		}
	}
}

// dark・light の agenda の扉は、題の下に同じ親の下の区切りだけの一覧を置き、今の区切りを
// 太字・アクセントの帯にする(ほかの章・節は並べない)。
func TestStandardSectionAgenda(t *testing.T) {
	d, i := parseAgendaDeck(t)
	for _, name := range []string{"dark", "light"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		b, err := tm.BuildSlide(d, i)
		if err != nil {
			t.Fatal(err)
		}
		if got := specTexts(b.Root); fmt.Sprint(got) != "[案内の見直し 夜間の返却 案内の見直し]" {
			t.Fatalf("%s: texts %v", name, got)
		}
		var items []*Spec
		findSpec(b.Main, func(s *Spec) bool {
			if s.Kind == KindText && s.Placeholder == "" {
				items = append(items, s)
			}
			return false
		})
		if len(items) != 2 || leafBold(items[0]) || !leafBold(items[1]) || runColor(items[1]) != tm.Theme["text_primary"] {
			t.Errorf("%s: list items %+v", name, items)
		}
		bar := findSpec(b.Main, func(s *Spec) bool {
			return s.Kind != KindText && s.Background == tm.Theme["accent"] && len(s.Children) == 0 && s != b.Main
		})
		if bar == nil {
			t.Errorf("%s: no accent bar", name)
		}
	}
}

// agenda は level 2・3 だけ(level 1 は check の invalid-value)。値は true・false、区切りだけ。
func TestSectionAgendaErrors(t *testing.T) {
	for _, c := range []struct {
		name, md, code, path string
	}{
		{"level 1", "---\ntitle: 試し\n---\n\n# 章 {type=section agenda=true}\n", "invalid-value", "/slides/1/agenda"},
		{"not bool", "---\ntitle: 試し\n---\n\n# 章 {type=section}\n\n# 節 {type=section level=2 agenda=yes}\n", "invalid-value", "/slides/2/agenda"},
		{"not a section", "---\ntitle: 試し\n---\n\n# 題 {agenda=true}\n\n- 文\n", "misplaced-content", "/slides/1"},
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
	ir := `{"boxloom":"0.1","slides":[{"type":"section","title":"章","agenda":true}]}`
	res := Check("x.json", []byte(ir), CheckOptions{})
	if len(res.Errors) == 0 || res.Errors[0].Code != "invalid-value" || res.Errors[0].Path != "/slides/0/agenda" {
		t.Errorf("IR level 1 agenda: %+v", res.Errors)
	}
	ok := "---\ntitle: 試し\n---\n\n# 章 {type=section}\n\n# 節 {type=section level=2 agenda=true}\n\n# 項目 {type=section level=3 agenda=true}\n"
	if res := Check("x.bx.md", []byte(ok), CheckOptions{}); len(res.Errors) > 0 {
		t.Errorf("valid agenda: %+v", res.Errors)
	}
}

// leafBold は文字の葉が太字か(塊の太字も見る)。
func leafBold(s *Spec) bool {
	if s.Bold {
		return true
	}
	for _, r := range s.Runs {
		if !r.Bold {
			return false
		}
	}
	return len(s.Runs) > 0
}
