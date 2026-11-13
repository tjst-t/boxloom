package boxloom

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

// 人物の紹介(people、Backlog #120)の試験。人物・写真はすべて架空(写真は作った絵)。

const (
	peopleExampleMD = "../examples/md/people.bx.md"
	peopleExampleIR = "../examples/ir/people.json"
)

// Markdown の例を読むと IR の例と同じ IR になり、Go の読み込みも通る。
func TestPeopleMarkdownMatchesIR(t *testing.T) {
	md, err := os.ReadFile(peopleExampleMD)
	if err != nil {
		t.Fatal(err)
	}
	got, issues := MarkdownToIR("people.bx.md", md)
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	want, err := os.ReadFile(peopleExampleIR)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(got, want) {
		t.Fatalf("Markdown IR differs from %s:\n%s", peopleExampleIR, got)
	}
	d, err := ParseIR(got)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readPeople(d.Slides[2].Body[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	// 空白を含む属性の値は "…" で囲める。写真の無い人は image・alt を持たない。
	if len(p.People) != 4 || p.People[0].Role.Plain() != "大学 教授(交通工学)" || p.People[1].Image != "" || p.People[1].Role.Plain() != "商店会 会長" {
		t.Errorf("people = %+v", p.People)
	}
	if p.People[0].Body == nil || p.People[0].Body.Type != "paragraph" {
		t.Errorf("body = %+v", p.People[0].Body)
	}
}

// shapeText は図形の文字(塊をつないだもの)。
func shapeText(s shape) string {
	if len(s.runs) == 0 {
		return s.text
	}
	var b strings.Builder
	for _, r := range s.runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// overlaps は2つの矩形が(丸めの誤差より大きく)重なるか。
func overlaps(a, b LBox) bool {
	const eps = 0.5
	return a.X+a.Width > b.X+eps && b.X+b.Width > a.X+eps && a.Y+a.Height > b.Y+eps && b.Y+b.Height > a.Y+eps
}

func boxesOf(ss []shape) []LBox {
	var out []LBox
	for _, s := range ss {
		out = append(out, s.box)
	}
	return out
}

// peopleSlides は例を tm で組み、スライドごとの図形と組んだスライドを返す。
func peopleSlides(t *testing.T, tm *Template) ([][]shape, []*BuiltSlide) {
	t.Helper()
	md, err := os.ReadFile(peopleExampleMD)
	if err != nil {
		t.Fatal(err)
	}
	deck, issues, err := ParseMarkdown("people.bx.md", md)
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	tm.SetAssets(os.DirFS("../examples/md"))
	built, err := tm.BuildDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]shape
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
		p := NewPPTXBuilder(tm.Values.PageW, tm.Values.PageH)
		roles := newCheckedRoles(tm.Theme)
		renderSlide(RenderContext{Out: p, Roles: roles}, b.Root)
		if m := roles.Missing(); len(m) > 0 {
			t.Fatalf("%s: missing roles %v", tm.Name, m)
		}
		out = append(out, p.Shapes)
	}
	return out, built
}

// 3つのテンプレートで、写真は値の表の大きさと形の正方形で、文(名前・役職・経歴)と
// 重ならず、名前は写真の右に置く。3人は横に1列、4人は2人ずつの2段、並列のまとまりの
// 中の2人は縦に積み、まとまりの外に出ない。警告(代わりの組み方・はみ出し・最小)は無い。
func TestPeopleDrawn(t *testing.T) {
	md, err := os.ReadFile(peopleExampleMD)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report", "dark", "light"} {
		deck, err := ReadDeck("people.bx.md", md)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Build(io.Discard, deck, BuildOptions{Template: name, Assets: os.DirFS("../examples/md")})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, w := range res.Warnings {
			for _, bad := range []string{"COMPONENT-FALLBACK", "LAYOUT-OVERFLOW", "SUBSYSTEM", "SPARE"} {
				if strings.Contains(w, bad) {
					t.Errorf("%s: %s", name, w)
				}
			}
		}

		tm := mustTemplate(name)
		slides, built := peopleSlides(t, tm)
		size := tm.Values.PersonPhoto
		prst := personPhotoShapes[tm.Values.PersonPhotoShape]
		pics := make([][]shape, len(slides))
		for si := 1; si < len(slides); si++ {
			shapes := slides[si]
			for _, s := range shapes {
				if s.kind != skPicture {
					continue
				}
				pics[si] = append(pics[si], s)
				if !s.picFill || s.prst != prst || !nearPt(s.box.Width, size) || !nearPt(s.box.Height, size) {
					t.Errorf("%s slide %d: photo %+v (want %s %.0fpt square)", name, si, s.box, prst, size)
				}
				if s.box.X < 0 || s.box.Y < 0 || s.box.X+s.box.Width > tm.Values.PageW || s.box.Y+s.box.Height > tm.Values.PageH {
					t.Errorf("%s slide %d: photo outside the page %+v", name, si, s.box)
				}
				for _, o := range shapes {
					if o.kind == skText && strings.TrimSpace(shapeText(o)) != "" && overlaps(s.box, o.box) {
						t.Errorf("%s slide %d: photo %+v overlaps text %q %+v", name, si, s.box, shapeText(o), o.box)
					}
				}
				// 写真の代わりの文は p:pic の descr。
				if !strings.Contains(s.text, "作った絵") {
					t.Errorf("%s slide %d: alt %q", name, si, s.text)
				}
			}
		}
		// 名前は写真の右(同じ人の写真の右端より右、写真の高さの中)。
		nameRightOf := func(si int, person string, pic shape) {
			for _, o := range slides[si] {
				if o.kind == skText && shapeText(o) == person {
					if o.box.X < pic.box.X+pic.box.Width || o.box.Y < pic.box.Y-0.5 || o.box.Y > pic.box.Y+pic.box.Height {
						t.Errorf("%s slide %d: name %q at %+v, photo %+v", name, si, person, o.box, pic.box)
					}
					return
				}
			}
			t.Errorf("%s slide %d: no name %q", name, si, person)
		}
		// 3人:横に1列(同じ上端、左から順に)。
		if p := pics[1]; len(p) != 3 || !nearPt(p[0].box.Y, p[1].box.Y) || !nearPt(p[1].box.Y, p[2].box.Y) || !(p[0].box.X < p[1].box.X && p[1].box.X < p[2].box.X) {
			t.Errorf("%s: three people not in one row: %v", name, boxesOf(p))
		} else {
			nameRightOf(1, "森川 あおい", p[0])
			nameRightOf(1, "野々村 千尋", p[2])
		}
		// 4人(2人目は写真なし):2人ずつの2段。1人目と3人目は同じ左端、3人目と4人目は同じ段。
		if p := pics[2]; len(p) != 3 || !nearPt(p[0].box.X, p[1].box.X) || !(p[1].box.Y > p[0].box.Y+size) || !nearPt(p[1].box.Y, p[2].box.Y) || !(p[2].box.X > p[1].box.X) {
			t.Errorf("%s: four people not in two rows of two: %v", name, boxesOf(p))
		}
		// 並列のまとまりの中の2人:縦に積み、左のまとまりの中に収まる。
		p := pics[3]
		cols := findColumnsRow(built[3].Root)
		if cols == nil || len(p) != 2 {
			t.Fatalf("%s: column slide: columns %v, photos %d", name, cols != nil, len(p))
		}
		left := cols.columns.cols[0].spec.Box
		if !nearPt(p[0].box.X, p[1].box.X) || !(p[1].box.Y >= p[0].box.Y+size) {
			t.Errorf("%s: people in a column not stacked: %v", name, boxesOf(p))
		}
		for _, s := range p {
			if s.box.X < left.X-0.01 || s.box.X+s.box.Width > left.X+left.Width+0.01 {
				t.Errorf("%s: photo %+v outside the column %+v", name, s.box, left)
			}
		}
		for _, o := range slides[3] {
			if o.kind == skText && (shapeText(o) == "森川 あおい" || shapeText(o) == "調査の全体について") && o.box.X+o.box.Width > left.X+left.Width+0.01 {
				t.Errorf("%s: text %q %+v outside the column %+v", name, shapeText(o), o.box, left)
			}
		}
	}
}

// 写真は箱いっぱいに置き、縦横比が違えば長い方の端を切る(縦長の絵は上下を切る)。
// 形は値の表の personPhotoShape(丸は ellipse で切り抜く)。
func TestPersonPhotoXML(t *testing.T) {
	tall := &imageAsset{Format: "png", W: 240, H: 300}
	s := &shape{id: 2, kind: skPicture, box: LBox{Width: 64, Height: 64}, image: tall, text: "絵", prst: "ellipse", picFill: true, rid: "rId9"}
	x := pictureXML(s)
	if !strings.Contains(x, `<a:srcRect t="10000" b="10000"/>`) || !strings.Contains(x, `prst="ellipse"`) {
		t.Errorf("picture = %s", x)
	}
	// ふつうの画像は今のまま(切らず、四角)。
	plain := &shape{id: 2, kind: skPicture, box: LBox{Width: 64, Height: 80}, image: tall, text: "絵", rid: "rId9"}
	if x := pictureXML(plain); strings.Contains(x, "srcRect") || !strings.Contains(x, `prst="rect"`) {
		t.Errorf("plain picture = %s", x)
	}
}

// 属性の値は "…" で囲めば空白を含められる。
func TestAttrFieldsQuoted(t *testing.T) {
	got := attrFields(`role="大学 教授" image=a.png  alt='作った 絵'`)
	want := []string{`role="大学 教授"`, `image=a.png`, `alt='作った 絵'`}
	if !slices.Equal(got, want) {
		t.Errorf("attrFields = %q", got)
	}
}

// 本文の幅を人数で割った1人の幅が personMinWidth に足りなければ2人ずつの段にする。
// 並列のまとまりの中は縦に積む。
func TestPeoplePerRow(t *testing.T) {
	for _, name := range []string{"report", "dark", "light"} {
		tm := mustTemplate(name)
		for n, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 2} {
			if got := tm.peoplePerRow(n, false); got != want {
				t.Errorf("%s: %d people per row = %d, want %d", name, n, got, want)
			}
			if got := tm.peoplePerRow(n, true); got != 1 {
				t.Errorf("%s: %d people in a column per row = %d", name, n, got)
			}
		}
	}
}
