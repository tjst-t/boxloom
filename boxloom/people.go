// people(人物の紹介):標準でない型(Backlog #120、docs/ir.md の10.3)。
//
// 1〜4人の写真・名前・役職・経歴を並べる。作者は人ごとに名前(必須)・役職・写真
// (代わりの文つき)・経歴(段落か箇条の1つ)を書くだけで、写真の形と大きさ・横に並べるか
// 縦に積むかはテンプレートが決める。
//
//   - 型の宣言(peopleTypeDecl):Schema は schema/types/people.schema.json の写し、IR を
//     読むときの形の検査、Markdown の囲み(::::people と :::person)とその読み方、代わりの
//     組み方(2〜3人は各人が1つのまとまりの columns。1人・4人とまとまりの中は、人ごとに
//     名前・役職・写真・経歴のブロックを順に積む)。
//   - 組み方(peopleComponent):同梱のテンプレート(dark・light・report)が持つ。写真は
//     値の表の personPhoto の正方形(形は personPhotoShape。丸は円で切り抜く)を左に置き、
//     右に名前(太字)・役職(薄い字)・経歴を積む。本文の幅を人数で割った1人の幅が
//     personMinWidth 以上なら横に1列、足りなければ2人ずつの段に積む。並列のまとまりの
//     中(1〜2人)は縦に積む。report は名前を基調色にする(report.go)。

package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// peopleType は IR の type。
const peopleType = "people"

// peopleMax は people に書ける人の数の上限。columnPeopleMax は並列のまとまりの中の上限。
const (
	peopleMax       = 4
	columnPeopleMax = 2
)

// personPhotoShapes は値の表の personPhotoShape に書ける形 → PPTX の形(a:prstGeom)。
var personPhotoShapes = map[string]string{
	"circle": "ellipse",
	"square": "rect",
}

// irPeople は people のブロックの中身(IRBlock.Raw を読んだもの)。
type irPeople struct {
	Variant string
	People  []irPerson
}

// irPerson は1人の紹介。Body は経歴(段落か箇条。無ければ nil)。
type irPerson struct {
	Name, Role Inline
	Image, Alt string
	Body       *IRBlock
}

// readPeople は people の IR を読んで形を見る(知らないキー・足りない欄・人の数・
// 写真の代わりの文・経歴の種類)。誤りの文の頭は IR の場所(people[1].alt: …)。
func readPeople(raw json.RawMessage) (*irPeople, error) {
	var v struct {
		Type    string            `json:"type"`
		Variant string            `json:"variant"`
		People  []json.RawMessage `json:"people"`
	}
	if err := strictUnmarshal(raw, &v); err != nil {
		return nil, err
	}
	if len(v.People) < 1 || len(v.People) > peopleMax {
		return nil, fmt.Errorf("people needs one to %d people (got %d)", peopleMax, len(v.People))
	}
	out := &irPeople{Variant: v.Variant}
	for i, r := range v.People {
		var pr struct {
			Name  Inline          `json:"name"`
			Role  Inline          `json:"role"`
			Image *string         `json:"image"`
			Alt   *string         `json:"alt"`
			Body  json.RawMessage `json:"body"`
		}
		if err := strictUnmarshal(r, &pr); err != nil {
			return nil, fmt.Errorf("people[%d]: %w", i, err)
		}
		if pr.Name == nil {
			return nil, fmt.Errorf("people[%d].name: missing (every person needs a name)", i)
		}
		p := irPerson{Name: pr.Name, Role: pr.Role}
		switch {
		case pr.Image != nil:
			if pr.Alt == nil || strings.TrimSpace(*pr.Alt) == "" {
				return nil, fmt.Errorf("people[%d].alt: a photo needs a non-empty \"alt\" (the text read aloud and shown when the image is missing)", i)
			}
			if err := checkImageSrc(*pr.Image); err != nil {
				return nil, fmt.Errorf("people[%d].image: %w", i, err)
			}
			p.Image, p.Alt = *pr.Image, *pr.Alt
		case pr.Alt != nil:
			return nil, fmt.Errorf("people[%d].alt: \"alt\" is the text of the photo; write \"image\" with it or remove it", i)
		}
		if len(pr.Body) > 0 {
			var b IRBlock
			if err := b.UnmarshalJSON(pr.Body); err != nil {
				return nil, fmt.Errorf("people[%d].body: %w", i, err)
			}
			if b.Type != "paragraph" && b.Type != "points" {
				return nil, fmt.Errorf("people[%d].body: the body of a person cannot be a %s block (allowed: paragraph, points)", i, b.Type)
			}
			p.Body = &b
		}
		out.People = append(out.People, p)
	}
	return out, nil
}

// personBlocks は1人を標準の型のブロックに書き換える(代わりの組み方):名前(named なら
// 強調の段落)・役職(注)・写真(画像)・経歴。orig は元の人の場所(/people/K)、at は
// 書き換えたブロックの場所を作る関数で、場所の表 paths に写しを足す。
func personBlocks(p irPerson, named bool, orig string, at func(k int) string, paths fallbackPaths) []IRBlock {
	var out []IRBlock
	put := func(b IRBlock, to fallbackPath) {
		if paths != nil {
			paths[at(len(out))] = to
		}
		out = append(out, b)
	}
	if named {
		put(IRBlock{Type: "paragraph", Content: Inline{{Type: "em", Content: p.Name}}}, fallbackPath{To: orig + "/name"})
	}
	if p.Role != nil {
		put(IRBlock{Type: "note", Content: p.Role}, fallbackPath{To: orig + "/role"})
	}
	if p.Image != "" {
		put(IRBlock{Type: "image", Src: p.Image, Alt: p.Alt}, fallbackPath{To: orig + "/image"})
	}
	if p.Body != nil {
		put(*p.Body, fallbackPath{To: orig + "/body", Same: true})
	}
	return out
}

// peopleFallback は代わりの組み方。2〜3人は各人が1つのまとまりの columns(名前がまとまりの
// 題)。1人は名前・役職・写真・経歴のブロックを順に積み、4人は2人ずつの columns を2つ
// 積む(columns のまとまりは2〜3個)。
func peopleFallback(b *IRBlock) ([]*IRBlock, fallbackPaths, error) {
	p, err := readPeople(b.Raw)
	if err != nil {
		return nil, nil, err
	}
	n := len(p.People)
	if n == 1 {
		return peopleFallbackStacked(b)
	}
	columns := func(people []irPerson, base int, paths fallbackPaths) *IRBlock {
		cols := make([]IRColumn, len(people))
		for i, pr := range people {
			ci := i
			orig := fmt.Sprintf("/people/%d", base+i)
			if paths != nil {
				paths[fmt.Sprintf("/columns/%d", ci)] = fallbackPath{To: orig}
			}
			at := func(k int) string { return fmt.Sprintf("/columns/%d/body/%d", ci, k) }
			body := personBlocks(pr, false, orig, at, paths)
			if len(body) == 0 {
				// 名前だけの人:まとまりは中のブロックが要るので、名前を本文に置く。
				body = personBlocks(pr, true, orig, at, paths)
			} else {
				cols[ci].Title = pr.Name
			}
			cols[ci].Body = body
		}
		return &IRBlock{Type: "columns", Variant: p.Variant, Columns: cols}
	}
	if n <= 3 {
		paths := fallbackPaths{}
		return []*IRBlock{columns(p.People, 0, paths)}, paths, nil
	}
	return []*IRBlock{columns(p.People[:2], 0, nil), columns(p.People[2:], 2, nil)}, nil, nil
}

// peopleFallbackStacked は人ごとに名前・役職・写真・経歴のブロックを順に積む代わりの
// 組み方(1人のときと、並列のまとまりの中。まとまりの中には columns を置けない)。
func peopleFallbackStacked(b *IRBlock) ([]*IRBlock, fallbackPaths, error) {
	p, err := readPeople(b.Raw)
	if err != nil {
		return nil, nil, err
	}
	var out []*IRBlock
	for i, pr := range p.People {
		for _, blk := range personBlocks(pr, true, fmt.Sprintf("/people/%d", i), nil, nil) {
			blk := blk
			if blk.Variant == "" {
				blk.Variant = p.Variant
			}
			out = append(out, &blk)
		}
	}
	return out, nil, nil
}

// peopleTypeDecl は people が語彙に名乗るもの。
func peopleTypeDecl() typeDecl {
	schema, err := typeSchema(peopleType)
	if err != nil {
		panic(err)
	}
	return typeDecl{
		Schema: schema,
		Markdown: []BlockDecl{
			{Name: "people", Contains: []string{"person"}, MinChildren: 1, Attributes: []string{"variant"},
				Meaning: "人物の紹介(標準でない型)。:::person を1〜4個書く(並列のまとまりの中は1〜2個)。写真の形・大きさ、横に並べるか縦に積むかはテンプレートが決める。組み方を持たないテンプレートは ::::columns で組む",
				IR:      "people(people[] が人)"},
			{Name: "person", Label: "名前", LabelRequired: true, Inside: []string{"people"}, Contains: []string{"段落1つ", "箇条1つ"},
				Attributes: []string{"role", "image", "alt"},
				Meaning:    "1人の紹介。名前は必須。{role=役職} は任意、{image=写真のパス alt=代わりの文} は写真(image を書いたら alt も必須。空白を含む値は \"…\" で囲む)。中は経歴の段落か箇条を1つ(無くてもよい)",
				IR:         "people.people[](name・role・image・alt・body)"},
		},
		Fallback:         peopleFallback,
		FallbackInColumn: peopleFallbackStacked,
		Validate: func(b *IRBlock) error {
			_, err := readPeople(b.Raw)
			return err
		},
		readMarkdown: (*mdParser).peopleBlock,
		Inner:        peopleInner,
	}
}

// peopleInner は check に渡す people の中:人ごとの名前・役職(見える文字が要る)、写真
// (image・alt。画像のブロックと同じ検査)、経歴(本文のブロックと同じ検査。段落はその
// 文が経歴の意味なので見える文字が要る)。
func peopleInner(m map[string]any) []innerPart {
	var out []innerPart
	people, _ := m["people"].([]any)
	for k, x := range people {
		pm, ok := x.(map[string]any)
		if !ok {
			continue
		}
		pp := fmt.Sprintf("/people/%d", k)
		texts := []string{"name"}
		if _, ok := pm["role"]; ok {
			texts = append(texts, "role")
		}
		_, img := pm["image"]
		out = append(out, innerPart{Path: pp, Map: pm, Texts: texts, Image: img})
		if bm, ok := pm["body"].(map[string]any); ok {
			part := innerPart{Path: pp + "/body", Map: bm, Block: true}
			if bm["type"] == "paragraph" {
				part.Texts = []string{"content"}
			}
			out = append(out, part)
		}
	}
	return out
}

// --- 組み方(同梱のテンプレート) ---

// peopleStyle は people の組み方の見た目の差し替え口(report.go)。零値は標準の組み方。
type peopleStyle struct {
	// nameRole は名前の字の色の役割名("" なら text_primary)。
	nameRole string
}

// peopleComponent は people の標準の組み方。
func peopleComponent(ctx componentContext, b *IRBlock) (*Spec, error) {
	return peopleStyled(ctx, b, peopleStyle{})
}

// peoplePerRow は1つの段に横に並べる人の数。並列のまとまりの中は1(縦に積む)。本文では、
// 本文の幅(ページの幅から左右の外周を引いたもの)を人数で割った1人の幅が personMinWidth
// 以上なら全員を1段に、足りなければ2人ずつ(2人でも足りなければ1人ずつ)にする。
func (t *Template) peoplePerRow(n int, inColumn bool) int {
	if inColumn || n <= 1 {
		return 1
	}
	v := t.Values
	width := v.PageW - 2*v.PagePadding
	fits := func(k int) bool {
		return (width-float64(k-1)*v.ColumnGap)/float64(k) >= v.PersonMinWidth
	}
	for _, k := range []int{n, 2} {
		if k <= n && fits(k) {
			return k
		}
	}
	return 1
}

// peopleStyled は people を st の見た目で組む。1人は写真(あれば)を左に、右に名前・役職・
// 経歴を縦に積んだ行。人の行を peoplePerRow 人ずつの段に並べる(段の中の人は同じ幅。
// 足りない段は空きの箱で幅をそろえる)。
func peopleStyled(ctx componentContext, b *IRBlock, st peopleStyle) (*Spec, error) {
	t := ctx.t
	p, err := readPeople(b.Raw)
	if err != nil {
		return nil, err
	}
	nameRole := st.nameRole
	if nameRole == "" {
		nameRole = "text_primary"
	}
	persons := make([]*Spec, len(p.People))
	for i := range p.People {
		pr := &p.People[i]
		texts := []*Spec{t.text(pr.Name, "body", nameRole, true)}
		if pr.Role != nil {
			texts = append(texts, t.text(pr.Role, "small", "text_muted", false))
		}
		if pr.Body != nil {
			where := fmt.Sprintf("people[%d]: body", i)
			s, err := t.buildBlockIn(ctx.within(where), pr.Body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", where, err)
			}
			texts = append(texts, s)
		}
		text := Col(texts, tokGap("itemGap"), WithGrow(1), WithBasis(0))
		kids := []*Spec{text}
		if pr.Image != "" {
			img, err := t.images.get(pr.Image)
			if err != nil {
				return nil, fmt.Errorf("people[%d]: image: %w", i, err)
			}
			photo := &personPhoto{img: img, alt: pr.Alt, size: t.Values.PersonPhoto, prst: personPhotoShapes[t.Values.PersonPhotoShape]}
			kids = []*Spec{SubsystemNode(photo, noShrink()), text}
		}
		persons[i] = Row(kids, tokGap("cardPadding"))
	}
	per := t.peoplePerRow(len(persons), ctx.inColumn)
	if per == 1 {
		if len(persons) == 1 {
			return persons[0], nil
		}
		return Col(persons, tokGap("blockGap")), nil
	}
	var rows []*Spec
	for start := 0; start < len(persons); start += per {
		var kids []*Spec
		for k := start; k < start+per; k++ {
			if k < len(persons) {
				kids = append(kids, apply(persons[k], []SpecOpt{WithGrow(1), WithBasis(0)}))
			} else {
				kids = append(kids, Col(nil, WithGrow(1), WithBasis(0)))
			}
		}
		rows = append(rows, Row(kids, tokGap("columnGap")))
	}
	if len(rows) == 1 {
		return rows[0], nil
	}
	return Col(rows, tokGap("blockGap")), nil
}

// personPhoto は人物の紹介の写真の葉:一辺 size の正方形に、写真を縦横比を保って端を
// 切って敷き、形 prst(円か四角)で切り抜く。文に添える写真で大きさは値の表が決めるので、
// 横の余り(spare-width)には数えない(ornament)。
type personPhoto struct {
	img  *imageAsset
	alt  string
	size float64
	prst string
}

func (m *personPhoto) Name() string { return "personPhoto" }

// ornament:大きさの決まった写真で、枠に合わせて縮む図ではない。
func (m *personPhoto) ornament() {}

func (m *personPhoto) Measure(Constraint) Measurement {
	s := Size{Width: m.size, Height: m.size}
	return Measurement{Natural: s, Min: s, Max: s, FixedAspect: true}
}

func (m *personPhoto) Render(ctx RenderContext, box LBox) {
	s := math.Min(m.size, math.Min(box.Width, box.Height))
	if s <= 0 {
		return
	}
	ctx.Out.AddPictureFilled(LBox{X: box.X, Y: box.Y, Width: s, Height: s}, m.img, m.alt, m.prst)
}

// --- Markdown ---

// peopleBlock は ::::people … :::: を IR の people にする。中には :::person 名前
// {role=… image=… alt=…} を1〜4個書き、中身は経歴の段落か箇条を1つ(無くてもよい)。
func (p *mdParser) peopleBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	p.noLabel(f, path)
	ok := true
	var people []any
	for _, it := range f.items {
		if it.kind != mdFence || it.fence.name != "person" {
			p.fail(it.line, 1, path, codeMisplaced, "::::people の中には :::person だけを書けます", []string{"person"}, "")
			ok = false
			continue
		}
		pf := it.fence
		pp := fmt.Sprintf("%s/people/%d", path, len(people))
		p.mark(pp, pf.line)
		pa := p.fenceAttrs(pf, pp, []string{"role", "image", "alt"})
		if len(pf.label.r) == 0 {
			p.fail(pf.line, 1, pp+"/name", codeMissingKey, ":::person には名前が要ります", nil, "「:::person 山田 花子 {role=研究員}」のように、囲みの名前の後ろに書いてください")
			ok = false
			continue
		}
		person := obj("name", p.parseInline(pf.label, pp+"/name"))
		if r, has := pa["role"]; has {
			person.set("role", r)
		}
		img, hasImg := pa["image"]
		alt, hasAlt := pa["alt"]
		switch {
		case hasImg && strings.TrimSpace(alt) == "":
			p.fail(pf.line, pf.attrsCol, pp+"/alt", codeMissingKey, "写真(image)には代わりの文(alt)が要ります", nil,
				"{image=photos/a.png alt=\"写真が何を示すか\"} のように書いてください(読み上げと、写真が無いときに使います)")
			ok = false
		case hasAlt && !hasImg:
			p.fail(pf.line, pf.attrsCol, pp+"/alt", codeMisplaced, "alt は写真(image)の代わりの文です。image と一緒に書きます", nil, "")
			ok = false
		case hasImg:
			if err := checkImageSrc(img); err != nil {
				p.fail(pf.line, pf.attrsCol, pp+"/image", codeInvalidValue, err.Error(), nil, "")
				ok = false
			}
			person.set("image", img)
			person.set("alt", alt)
		}
		var body *mdItem
		for _, sub := range pf.items {
			if (sub.kind != mdPara && sub.kind != mdList) || body != nil {
				p.fail(sub.line, 1, pp+"/body", codeMisplaced, ":::person の中には経歴の段落か箇条を1つだけ書けます", []string{"段落", "箇条"},
					"文を足すなら同じ段落か箇条に入れてください")
				ok = false
				continue
			}
			body = sub
		}
		if body != nil {
			bp := pp + "/body"
			p.mark(bp, body.line)
			if body.kind == mdPara {
				person.set("body", obj("type", "paragraph", "content", p.parseInline(body.text.trimBR(), bp+"/content")))
			} else {
				pts := obj("type", "points", "items", p.listItems(body.items, bp+"/items", 0, pointMaxDepth))
				if p.listOrdered(body.items, bp+"/items") {
					pts.set("ordered", true)
				}
				person.set("body", pts)
			}
		}
		people = append(people, person)
	}
	if ok && (len(people) < 1 || len(people) > peopleMax) {
		p.fail(f.line, 1, path+"/people", codeMisplaced, fmt.Sprintf("::::people には :::person を1〜%d個書きます(いま %d 個)", peopleMax, len(people)), nil,
			"5人以上なら、スライドを分けてください")
		return nil
	}
	if !ok {
		return nil
	}
	b := obj("type", peopleType)
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("people", people)
	return b
}
