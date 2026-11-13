package boxloom

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// noShrinkDeck は題・リード文・出典・フッター(作者)を持つ本文のスライドの deck。
// items の数だけ箇条を書く(多ければ本文がページから大きくはみ出す)。文は架空。
func noShrinkDeck(items int) string {
	var b strings.Builder
	b.WriteString("---\ntitle: 縮めない頭の試験\nauthor: 架空の調査室\n---\n\n# 窓口の待ち時間\n\n")
	b.WriteString(":::lead\n問い合わせの多くは最初の窓口で片づかず、別の窓口へ回されて待ち時間が延びている。\n:::\n\n")
	for i := 0; i < items; i++ {
		fmt.Fprintf(&b, "- 架空の窓口 %d では、受付から回答までに平均で数日かかり、その間に同じ問い合わせが繰り返し届く\n", i+1)
	}
	b.WriteString("\n:::source\n架空の窓口の記録(架空の年度、架空の件数)\n:::\n")
	return b.String()
}

// 本文がはみ出しても、頭(題・リード文)・出典・フッターは縮まず、位置と大きさは
// はみ出さないときと同じ。はみ出しは layout-overflow の警告で知らせる(#96)。
func TestOverflowKeepsHeaderAndFooter(t *testing.T) {
	short, long := noShrinkDeck(2), noShrinkDeck(40)
	for _, name := range []string{"dark", "light", "report"} {
		t.Run(name, func(t *testing.T) {
			tm, err := LookupTemplate(name)
			if err != nil {
				t.Fatal(err)
			}
			slide := func(src string) *BuiltSlide {
				d, _, err := ParseMarkdown("h.bx.md", []byte(src))
				if err != nil {
					t.Fatal(err)
				}
				built, err := tm.BuildDeck(d)
				if err != nil {
					t.Fatal(err)
				}
				b := built[len(built)-1]
				computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
				return b
			}
			a, b := slide(short), slide(long)
			if len(a.Root.Children) != len(b.Root.Children) || len(a.Root.Children) != 4 {
				t.Fatalf("root children %d and %d, want head・body・source・footer", len(a.Root.Children), len(b.Root.Children))
			}
			if a.Body.Box.Height < heightForWidth(a.Body, a.Body.Box.Width) {
				t.Fatalf("the short deck must fit (body %.1f < need %.1f)", a.Body.Box.Height, heightForWidth(a.Body, a.Body.Box.Width))
			}
			if need := heightForWidth(b.Body, b.Body.Box.Width); need < b.Body.Box.Height+200 {
				t.Fatalf("the long deck must overflow by far (body %.1f, need %.1f)", b.Body.Box.Height, need)
			}
			// 本文の箱以外の根の子(頭・出典・フッター)と、その中の葉の矩形が同じ。
			for i := range a.Root.Children {
				if a.Root.Children[i] == a.Body {
					continue
				}
				var ra, rb []LBox
				collectBoxes(a.Root.Children[i], &ra)
				collectBoxes(b.Root.Children[i], &rb)
				if len(ra) != len(rb) {
					t.Fatalf("child %d: %d and %d boxes", i, len(ra), len(rb))
				}
				for k := range ra {
					if !sameLBox(ra[k], rb[k]) {
						t.Errorf("child %d box %d: %+v without overflow, %+v with overflow", i, k, ra[k], rb[k])
					}
				}
			}
			// 題・リード文の葉も、自然の高さのまま(縮めていない)。
			lead := leafWithPlain(b.Root, "問い合わせの多くは")
			if lead == nil || b.Title == nil {
				t.Fatal("no lead or title leaf")
			}
			for _, s := range []*Spec{lead, b.Title} {
				if want := textHeightFor(s, s.Box.Width); math.Abs(s.Box.Height-want) > 0.01 {
					t.Errorf("%q: height %.2f, natural %.2f", s.Text+runsText(s.Runs), s.Box.Height, want)
				}
			}

			// layout-overflow の警告は出る(はみ出さない deck には出ない)。
			for _, c := range []struct {
				src  string
				want bool
			}{{short, false}, {long, true}} {
				res := Check("h.bx.md", []byte(c.src), CheckOptions{Template: name})
				got := false
				for _, w := range res.Warnings {
					if w.Code == "layout-overflow" {
						got = true
					}
				}
				if got != c.want {
					t.Errorf("layout-overflow warned = %v, want %v (%+v)", got, c.want, res.Warnings)
				}
			}
		})
	}
}

func collectBoxes(s *Spec, out *[]LBox) {
	*out = append(*out, s.Box)
	for _, c := range s.Children {
		collectBoxes(c, out)
	}
}

func sameLBox(a, b LBox) bool {
	return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6 &&
		math.Abs(a.Width-b.Width) < 1e-6 && math.Abs(a.Height-b.Height) < 1e-6
}

// noShrink の印の無い木は従来どおり比で縮める。印の付いた子は basis のまま。
// (頭は縮めずに要る高さの無い basis だけの箱。本文の箱は骨組みの枠で、下限を持たない)
func TestNoShrinkLayout(t *testing.T) {
	head := func() *Spec { return Col(nil, WithBasis(20)) }
	mk := func(opts ...SpecOpt) (*Spec, *Spec, *Spec) {
		h := apply(head(), opts)
		body := Col([]*Spec{Col(nil, WithHeight(200))}, markFrame(frameBody))
		root := Col([]*Spec{h, body}, WithPadding(0))
		computeLayout(root, 0, 0, 100, 110)
		return root, h, body
	}
	_, h, body := mk()
	if math.Abs(h.Box.Height-10) > 1e-9 || math.Abs(body.Box.Height-100) > 1e-9 {
		t.Errorf("without the mark: head %.2f body %.2f, want 10 and 100", h.Box.Height, body.Box.Height)
	}
	_, h, body = mk(noShrink())
	if math.Abs(h.Box.Height-20) > 1e-9 || math.Abs(body.Box.Height-90) > 1e-9 || math.Abs(body.Box.Y-20) > 1e-9 {
		t.Errorf("with the mark: head %.2f body %.2f at %.2f, want 20 and 90 at 20", h.Box.Height, body.Box.Height, body.Box.Y)
	}
}
