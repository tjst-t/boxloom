package boxloom

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// tokenTemplates は試験に使うテンプレート(同梱の2つと、外の値の表だけのもの)。
func tokenTemplates(t *testing.T) []*Template {
	t.Helper()
	data, err := os.ReadFile("../examples/templates/forest.json")
	if err != nil {
		t.Fatal(err)
	}
	forest, err := ParseTemplate("forest.json", data)
	if err != nil {
		t.Fatal(err)
	}
	return []*Template{mustTemplate("dark"), mustTemplate("light"), forest}
}

// TestStandardComponentsWriteNamesOnly: 標準の部品と骨組み(と同梱の組み方の pairs)は、
// 組み立てたままの木(解決の前)に数・色・段の刻みを書かず、値の表の名前と役割名だけを書く。
func TestStandardComponentsWriteNamesOnly(t *testing.T) {
	var files []string
	for _, pat := range []string{"../examples/ir/*.json", "../examples/md/*.bx.md"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	slides := 0
	types := map[string]bool{}
	for _, tm := range tokenTemplates(t) {
		tt := newTokenTable(tm)
		isName := func(tok string) bool {
			_, a := tt.spaces[tok]
			_, b := tt.types[tok]
			_, c := tt.lengths[tok]
			_, d := tt.ems[tok]
			_, e := tm.Theme[tok]
			return a || b || c || d || e
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			deck, err := ReadDeck(f, data)
			if err != nil {
				continue // 誤りを確かめる例
			}
			tm.SetAssets(os.DirFS(filepath.Dir(f)))
			for i := range deck.Slides {
				b, err := tm.buildSlideTokens(deck, i)
				if err != nil {
					continue
				}
				slides++
				var walk func(s *Spec, at string)
				walk = func(s *Spec, at string) {
					if s.irType != "" {
						types[s.irType] = true
					}
					bad := func(what string, v any) {
						t.Errorf("%s %s slide %d %s: %s written as a value: %v", tm.Name, f, i, at, what, v)
					}
					if s.Gap != 0 {
						bad("gap", s.Gap)
					}
					if s.Padding != 0 || s.PaddingY != nil {
						bad("padding", s.Padding)
					}
					if s.HasRadius {
						bad("radius", s.Radius)
					}
					if s.Width != nil || s.Height != nil {
						bad("width/height", [2]*float64{s.Width, s.Height})
					}
					if s.Background != "" || (s.Color != "" && s.Color != defaultTextColor) {
						bad("color", s.Background+s.Color)
					}
					if s.Kind == KindText && (s.FontSize != 12.5 || s.tok.fontSize == "") {
						bad("fontSize", s.FontSize) // 12.5 は TextNode の既定(トークンで上書きされる)
					}
					for ri, r := range s.Runs {
						if r.Color != "" || r.Size != 0 {
							bad("run", ri)
						}
						for _, tok := range []string{r.role, r.sizeTok} {
							if tok != "" && !isName(tok) {
								t.Errorf("%s %s slide %d %s: run token %q is not a value-table name or role", tm.Name, f, i, at, tok)
							}
						}
					}
					k := s.tok
					for _, tok := range []string{k.gap, k.padX, k.padY, k.radius, k.width, k.height, k.fontSize, k.color, k.background} {
						if tok != "" && !isName(tok) {
							t.Errorf("%s %s slide %d %s: token %q is not a value-table name or role", tm.Name, f, i, at, tok)
						}
					}
					for ci, c := range s.Children {
						walk(c, at+"/"+strconv.Itoa(ci))
					}
				}
				walk(b.Root, "root")
				if err := resolveSpec(tm, b.Root); err != nil {
					t.Errorf("%s %s slide %d: %v", tm.Name, f, i, err)
				}
			}
		}
	}
	if slides < 50 {
		t.Fatalf("only %d slides were built", slides)
	}
	for _, typ := range []string{"paragraph", "note", "points", "compare", "stat", "takeaway", "quote", "columns", "table", "image", "subsystem", "pairs"} {
		if !types[typ] {
			t.Errorf("no example built a %q block", typ)
		}
	}
}

// TestResolveSpec: 名前・段の刻み・役割名は値の表から引き、段に無い刻み・知らない
// 名前・役割名・トークンの無い数と色は誤り。
func TestResolveSpec(t *testing.T) {
	tm := mustTemplate("dark") // spacingUnit 4, typeSizes [8 10 12 14 16 18 24 36]
	v := tm.Values
	text := func(opts ...SpecOpt) *Spec { return TextNode("x", append([]SpecOpt{tokSize("body")}, opts...)...) }

	ok := Col([]*Spec{
		text(tokColor("accent")),
		TextNode("y", tokSize(typeStep(24))),
		RichTextNode([]TextRun{{Text: "a", role: "text_muted", sizeTok: "statUnit"}}, tokSize("statValue")),
		Col(nil, tokSize("subPoint"), tokWidth("pointIndentEm"), tokHeight(spaceStep(3))),
	}, tokGap("blockGap"), tokPadXY(spaceStep(5), "cardPadding"), tokBackground("card"), tokRadius("cardRadius"))
	if err := resolveSpec(tm, ok); err != nil {
		t.Fatal(err)
	}
	if ok.Gap != v.BlockGap || ok.Padding != 20 || *ok.PaddingY != v.CardPadding || ok.Background != tm.Theme["card"] || ok.Radius != v.CardRadius || !ok.HasRadius {
		t.Errorf("box: gap %v padding %v/%v background %q radius %v", ok.Gap, ok.Padding, *ok.PaddingY, ok.Background, ok.Radius)
	}
	if c := ok.Children[0]; c.FontSize != v.Body || c.Color != tm.Theme["accent"] {
		t.Errorf("text: %v %q", c.FontSize, c.Color)
	}
	if c := ok.Children[1]; c.FontSize != 24 {
		t.Errorf("type(24): %v", c.FontSize)
	}
	if r := ok.Children[2].Runs[0]; r.Color != tm.Theme["text_muted"] || r.Size != v.StatUnit {
		t.Errorf("run: %q %v", r.Color, r.Size)
	}
	if c := ok.Children[3]; *c.Width != v.SubPoint*v.PointIndentEm || *c.Height != 12 {
		t.Errorf("em width %v height %v", *c.Width, *c.Height)
	}

	for _, c := range []struct {
		name string
		spec *Spec
		want string
	}{
		{"unknown space name", Col(nil, tokGap("blokGap")), `"blokGap" is neither a spacing name`},
		{"type name as gap", Col(nil, tokGap("body")), `"body" is neither a spacing name`},
		{"space step 0", Col(nil, tokGap("space(0)")), "whole number of at least 1"},
		{"space step fraction", Col(nil, tokPad("space(1.5)")), `"space(1.5)" is neither a spacing name`},
		{"type step off the scale", TextNode("x", tokSize("type(13)")), "not on the type scale"},
		{"unknown type name", TextNode("x", tokSize("huge")), `"huge" is neither a type size name`},
		{"unknown role", Col(nil, tokBackground("brand")), `"brand" is not a color role`},
		{"hex color", Col(nil, tokBackground("FF0000")), "hex colors cannot be written"},
		{"run hex", RichTextNode([]TextRun{{Text: "a", role: "#00ff00"}}, tokSize("body")), "hex colors cannot be written"},
		{"run size off the scale", RichTextNode([]TextRun{{Text: "a", sizeTok: "type(9)"}}, tokSize("body")), "not on the type scale"},
		{"em without font", Col(nil, tokWidth("pointIndentEm")), "has no fontSize token"},
		{"ratio is no length", Col(nil, tokWidth("fillRatioMin")), `"fillRatioMin" is neither a length name`},
		{"raw gap", Col(nil, WithGap(8)), "gap: 8 is written without a token"},
		{"raw width", Col(nil, WithWidth(3)), "width: 3 is written without a token"},
		{"raw background", Col(nil, WithBackground("12203D")), "background: 12203D is written without a token"},
		{"raw font size", TextNode("x", WithFontSize(14)), "a text leaf needs a type size token"},
		{"raw run color", RichTextNode([]TextRun{{Text: "a", Color: "FFFFFF"}}, tokSize("body")), "runs[0].color: FFFFFF is written without a token"},
		{"raw run size", RichTextNode([]TextRun{{Text: "a", Size: 9}}, tokSize("body")), "runs[0].size: 9 is written without a token"},
		{"nested", Col([]*Spec{Col(nil), Col(nil, tokGap("nope"))}), "at root/children/1: gap"},
	} {
		err := resolveSpec(tm, c.spec)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
}

// TestBuildSlideRejectsBadTokens: 組み方が段に無い刻みを書けば、組み立てが誤りになる
// (足す部品も #1301886 の段の検査を素通りできない)。
func TestBuildSlideRejectsBadTokens(t *testing.T) {
	tm := mustTemplate("light")
	if err := tm.components.override("paragraph", componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return Col([]*Spec{ctx.t.text(b.Content, "body", "text_primary", false)}, tokPad(spaceStep(1)), tokGap("space(2)"), tokRadius("type(14)")), nil
	})); err != nil {
		t.Fatal(err)
	}
	d := &IRDeck{Slides: []IRSlide{{Type: "content", Title: plainInline("T"), Body: []IRBlock{{Type: "paragraph", Content: plainInline("x")}}}}}
	_, err := tm.BuildDeck(d)
	if err == nil || !strings.Contains(err.Error(), `radius: "type(14)" is neither a length name`) {
		t.Fatalf("err %v", err)
	}
}
