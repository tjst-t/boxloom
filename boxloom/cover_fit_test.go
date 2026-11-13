package boxloom

import (
	"strings"
	"testing"
)

// report の表紙と終わりのスライドの基調色の箱は、中身が要るだけ伸びる(#104)。

// fitDeck は題・副題・日付・終わりの文を持つ文書を tm で組み、配置まで済ませる。
func fitDeck(t *testing.T, tm *Template, title, closing string) []*BuiltSlide {
	t.Helper()
	src := "---\ntitle: " + title + "\nsubtitle: 架空の地域の暮らしと移動の調べ(第二版)\ndate: 2026-11\nauthor: 架空の調べの会\n" + closing + "---\n\n# 朝に集まる\n\n- 朝の利用が多い\n"
	d, issues, err := ParseMarkdown("t.bx.md", []byte(src))
	if err != nil || len(issues) > 0 {
		t.Fatalf("parse: %v %v", err, issues)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	}
	return built
}

// accentBox は根の下の帯の左の基調色の箱。
func accentBox(t *testing.T, tm *Template, root *Spec) *Spec {
	t.Helper()
	box := root.Children[0].Children[0]
	if box.Background != tm.Theme["accent"] {
		t.Fatalf("the first box is not the accent box: %q", box.Background)
	}
	return box
}

// fitsIn は葉 s の矩形が箱 box の中にあり、文字の要る高さを持っているかを確かめる。
func fitsIn(t *testing.T, what string, s, box *Spec) {
	t.Helper()
	const eps = 0.01
	if need := heightForWidth(s, s.Box.Width); s.Box.Height < need-eps {
		t.Errorf("%s squeezed: %.1fpt < need %.1fpt", what, s.Box.Height, need)
	}
	b, r := box.Box, s.Box
	if r.X < b.X-eps || r.Y < b.Y-eps || r.X+r.Width > b.X+b.Width+eps || r.Y+r.Height > b.Y+b.Height+eps {
		t.Errorf("%s %+v is outside the box %+v", what, r, b)
	}
}

func TestReportCoverFitsLongTitle(t *testing.T) {
	tm, _ := LookupTemplate("report")
	W, H := tm.Values.PageW, tm.Values.PageH
	long := strings.Repeat("架空の街で朝に集まる人の流れと", 2) + "理由"
	built := fitDeck(t, tm, long, "closing: ご清聴ありがとうございました。問い合わせは架空の調べの会の窓口まで\n")
	cover := built[0].Root
	box := accentBox(t, tm, cover)
	var title, sub *Spec
	for _, c := range box.Children {
		switch c.Placeholder {
		case "ctrTitle":
			title = c
		case "subTitle":
			sub = c
		}
	}
	if title == nil || sub == nil {
		t.Fatal("no title or subtitle in the accent box")
	}
	if n := len(wrapRuns(title.Runs, title.FontSize, title.Box.Width)); n != 3 {
		t.Fatalf("the title should wrap to 3 lines (got %d)", n)
	}
	fitsIn(t, "title", title, box)
	fitsIn(t, "subtitle", sub, box)
	if title.Box.Y+title.Box.Height > sub.Box.Y+0.01 {
		t.Errorf("title %+v overlaps subtitle %+v", title.Box, sub.Box)
	}
	if box.Box.Width < W*reportCoverBoxW-0.01 || box.Box.Width > W*reportCoverBoxW+0.01 {
		t.Errorf("box width %.1f, want %.1f", box.Box.Width, W*reportCoverBoxW)
	}
	if box.Box.Height <= H*reportCoverBoxH+0.01 {
		t.Errorf("the box did not grow for the long title: %.1f", box.Box.Height)
	}
	// 日付・作者は箱の下で、要る高さを持つ。
	below := cover.Children[1]
	if below.Box.Y < box.Box.Y+box.Box.Height-0.01 {
		t.Errorf("the foot %+v starts inside the box %+v", below.Box, box.Box)
	}
	if need := heightForWidth(below, below.Box.Width); below.Box.Height < need-0.01 {
		t.Errorf("the foot is squeezed: %.1f < %.1f", below.Box.Height, need)
	}

	// 終わりのスライドの文は箱の中。
	closing := built[len(built)-1]
	if closing.SlideIndex != closingSlide {
		t.Fatalf("the last slide is not the closing slide: %d", closing.SlideIndex)
	}
	cbox := accentBox(t, tm, closing.Root)
	text := cbox.Children[len(cbox.Children)-1]
	if text.Kind != KindText || !strings.Contains(text.Text, "ご清聴ありがとうございました") {
		t.Fatalf("no closing text in the box: %+v", text)
	}
	fitsIn(t, "closing text", text, cbox)
}

// 短い題なら箱は従来の大きさ(幅 0.53・高さ 0.45)のまま。終わりの文が無ければ箱だけ。
func TestReportCoverShortTitleKeepsBox(t *testing.T) {
	tm, _ := LookupTemplate("report")
	W, H := tm.Values.PageW, tm.Values.PageH
	built := fitDeck(t, tm, "架空の街の調べ", "closing: true\n")
	for _, b := range []*BuiltSlide{built[0], built[len(built)-1]} {
		box := accentBox(t, tm, b.Root)
		if d := box.Box.Height - H*reportCoverBoxH; d > 0.01 || d < -0.01 {
			t.Errorf("slide %d: box height %.1f, want %.1f", b.SlideIndex, box.Box.Height, H*reportCoverBoxH)
		}
		if d := box.Box.Width - W*reportCoverBoxW; d > 0.01 || d < -0.01 {
			t.Errorf("slide %d: box width %.1f", b.SlideIndex, box.Box.Width)
		}
	}
	if cbox := accentBox(t, tm, built[len(built)-1].Root); len(cbox.Children) != 1 {
		t.Errorf("closing box without text has %d children", len(cbox.Children))
	}
}

// meta.closing は真偽か文。文は IR・Markdown のどちらからも読め、標準の終わりの
// スライド(light・dark)も文を置く。
func TestClosingText(t *testing.T) {
	d, err := ReadIR("t.json", []byte(coverDeckJSON("light", `, "closing": "ご清聴ありがとうございました"`)))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Meta.Closing.On || len(d.Meta.Closing.Text) != 1 || d.Meta.Closing.Text[0].Text != "ご清聴ありがとうございました" {
		t.Fatalf("closing = %+v", d.Meta.Closing)
	}
	if d, err := ReadIR("t.json", []byte(coverDeckJSON("light", `, "closing": false`))); err != nil || d.Meta.Closing.On {
		t.Fatalf("closing false: %v %+v", err, d)
	}
	if _, err := ReadIR("t.json", []byte(coverDeckJSON("light", `, "closing": 3`))); err == nil {
		t.Error("closing: 3 should be an error")
	}
	for _, name := range []string{"light", "dark"} {
		files := buildCover(t, name, `, "closing": "ご清聴ありがとうございました"`)
		if !strings.Contains(files["ppt/slides/slide3.xml"], "ご清聴ありがとうございました") {
			t.Errorf("%s: closing text is not on the closing slide", name)
		}
	}
	for _, c := range []struct {
		line string
		ok   bool
	}{
		{"closing: ご清聴ありがとうございました", true},
		{`closing: "yes"`, true},
		{"closing: yes", false},
		{"closing: Off", false},
	} {
		src := strings.Replace(coverDeckMD, "closing: true", c.line, 1)
		d, issues, _ := ParseMarkdown("t.bx.md", []byte(src))
		if c.ok != (len(issues) == 0) {
			t.Errorf("%s: issues %v", c.line, issues)
		}
		if c.ok && (d == nil || !d.Meta.Closing.On || d.Meta.Closing.Text == nil) {
			t.Errorf("%s: closing not read as text", c.line)
		}
	}
}
