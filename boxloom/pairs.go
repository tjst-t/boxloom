// pairs(対の並び):最初の標準でない型(docs/layers.md の8.1)。
//
// 「分かったこと ↔ 打ち手」のように、2つの側の見出しを1回だけ書き、その下に対を
// 並べる。側の名前は意味の名前で、subject はその対が何についてか(分かったこと・
// 課題・問い)、response はそれに応えるもの(打ち手・対策・答え)。どちらを左に
// 置くかはテンプレートが決める。対の2つの側は、それぞれ段落・箇条・引用のどれか1つか、
// 見出し(title。任意)と複数のブロック(body。段落・箇条・引用・画像・表の1〜4個)の
// まとまり(Backlog #87)。
//
//   - 型の宣言(pairsTypeDecl):Schema は schema/types/pairs.schema.json の写し、
//     IR を読むときの形の検査、Markdown の囲み(::::pairs と :::subject・:::response)
//     とその読み方、代わりの組み方(左右2つのまとまりの columns)。
//   - 組み方(pairsComponent):同梱のテンプレート(dark・light・report)が持つ。対ごとに
//     1つの行にし、2つの側を同じ上端から置いて、行の高さを高い方に合わせる。
//     はみ出しは対ごとに測る(測る単位の印 measurePairRows)。見出しと複数のブロックの側は
//     見出しと body を縦に積む。report は同じ組み方の見た目を差し替えたもの(pairsStyled。
//     A・B・C の印・側の間の印・対の間の罫線・見出しの帯)を自分で登録する(report.go)。

package boxloom

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// pairsType は IR の type。
const pairsType = "pairs"

// pairSideTypes は対の1つの側に置けるブロックの種類(1つのブロックの形)。
var pairSideTypes = []string{"paragraph", "points", "quote"}

// pairSideBodyTypes は見出しと複数のブロックの側(body)に置けるブロックの種類。
var pairSideBodyTypes = []string{"paragraph", "points", "quote", "image", "table"}

// pairSideBodyMax は側の body に置けるブロックの数の上限。
const pairSideBodyMax = 4

// irPairs は pairs のブロックの中身(IRBlock.Raw を読んだもの)。
type irPairs struct {
	Variant  string
	Subject  Inline // headings.subject
	Response Inline // headings.response
	Pairs    []irPair
}

// irPair は1つの対。Label は対の短い名前(無ければ "")。
type irPair struct {
	Label             string
	Subject, Response irPairSide
}

// pairLabelMaxRunes は対の名前(label)の字の数の上限。
const pairLabelMaxRunes = 12

// pairLabelMarkMax は report が対の名前を四角い印の中に書く字の数の上限(これより長い
// 名前は行の左の見出しの列に書く)。
const pairLabelMarkMax = 2

// checkPairLabel は対の名前が、見える字を含む1〜12字の文であることを確かめる。
func checkPairLabel(l string) error {
	if strings.TrimSpace(l) == "" {
		return fmt.Errorf("the label of a pair must not be empty")
	}
	if n := utf8.RuneCountInString(l); n > pairLabelMaxRunes {
		return fmt.Errorf("the label %q is longer than %d characters (got %d; it names the pair briefly)", l, pairLabelMaxRunes, n)
	}
	return nil
}

// pairLabelMixed は名前(label)のある・無いが最初の対と違う対の番号(無ければ -1)と、
// その誤りの文。1つの pairs の中では、全ての対に名前を書くか、どれにも書かない。
func pairLabelMixed(has []bool) (int, string) {
	word := func(b bool) string {
		if b {
			return "has"
		}
		return "has no"
	}
	for k := 1; k < len(has); k++ {
		if has[k] != has[0] {
			return k, fmt.Sprintf("write a label on every pair of the pairs or on none (pair 0 %s a label, pair %d %s)", word(has[0]), k, word(has[k]))
		}
	}
	return -1, ""
}

// labels は対の名前の並び(名前の無い pairs は nil)。
func (p *irPairs) labels() []string {
	if len(p.Pairs) == 0 || p.Pairs[0].Label == "" {
		return nil
	}
	out := make([]string, len(p.Pairs))
	for k, pr := range p.Pairs {
		out[k] = pr.Label
	}
	return out
}

// irPairSide は対の1つの側。1つのブロックの形(Grouped が false)なら Body はその
// ブロック1つ。まとまりの形({title?, body: [...]})なら Grouped が true で、Title は
// 見出し(無ければ nil)。
type irPairSide struct {
	Grouped bool
	Title   Inline
	Body    []IRBlock
}

// readPairSide は対の1つの側を読む。type のあるオブジェクトは1つのブロック、
// 無ければ見出しと複数のブロックのまとまり。at は誤りの文の頭(pairs[1].subject)。
func readPairSide(raw json.RawMessage, at string) (irPairSide, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return irPairSide{}, fmt.Errorf("%s: %w", at, err)
	}
	if _, ok := keys["type"]; ok {
		var b IRBlock
		if err := b.UnmarshalJSON(raw); err != nil {
			return irPairSide{}, fmt.Errorf("%s: %w", at, err)
		}
		if !contains(pairSideTypes, b.Type) {
			return irPairSide{}, fmt.Errorf("%s: a side of a pair cannot be a %s block (allowed: %s, or {title, body} with %s)", at, b.Type,
				strings.Join(pairSideTypes, ", "), strings.Join(pairSideBodyTypes, ", "))
		}
		return irPairSide{Body: []IRBlock{b}}, nil
	}
	var g struct {
		Title json.RawMessage   `json:"title"`
		Body  []json.RawMessage `json:"body"`
	}
	if err := strictUnmarshal(raw, &g); err != nil {
		return irPairSide{}, fmt.Errorf("%s: %w", at, err)
	}
	out := irPairSide{Grouped: true}
	if len(g.Title) > 0 {
		if err := out.Title.UnmarshalJSON(g.Title); err != nil {
			return irPairSide{}, fmt.Errorf("%s.title: %w", at, err)
		}
	}
	if len(g.Body) == 0 || len(g.Body) > pairSideBodyMax {
		return irPairSide{}, fmt.Errorf("%s.body: a side needs \"type\" (one block) or \"body\" with 1 to %d blocks (got %d)", at, pairSideBodyMax, len(g.Body))
	}
	for i, r := range g.Body {
		var b IRBlock
		if err := b.UnmarshalJSON(r); err != nil {
			return irPairSide{}, fmt.Errorf("%s.body[%d]: %w", at, i, err)
		}
		if !contains(pairSideBodyTypes, b.Type) {
			return irPairSide{}, fmt.Errorf("%s.body[%d]: the body of a side cannot have a %s block (allowed: %s)", at, i, b.Type, strings.Join(pairSideBodyTypes, ", "))
		}
		if b.Type == "table" && b.Continues {
			return irPairSide{}, fmt.Errorf("%s.body[%d].continues: a continuing table must be a block of the slide body, not inside pairs", at, i)
		}
		out.Body = append(out.Body, b)
	}
	return out, nil
}

// readPairs は pairs の IR を読んで形を見る(知らないキー・足りない欄・側に置けない
// 種類は誤り)。誤りの文の頭は IR の場所(pairs[1].subject: …)。
func readPairs(raw json.RawMessage) (*irPairs, error) {
	var v struct {
		Type     string            `json:"type"`
		Variant  string            `json:"variant"`
		Headings json.RawMessage   `json:"headings"`
		Pairs    []json.RawMessage `json:"pairs"`
	}
	if err := strictUnmarshal(raw, &v); err != nil {
		return nil, err
	}
	if len(v.Headings) == 0 {
		return nil, fmt.Errorf("pairs needs \"headings\" with \"subject\" and \"response\" (the heading of each side, written once)")
	}
	var heads map[string]json.RawMessage
	if err := json.Unmarshal(v.Headings, &heads); err != nil {
		return nil, fmt.Errorf("headings: %w", err)
	}
	keys := slices.Sorted(maps.Keys(heads))
	for _, k := range keys {
		if k != "subject" && k != "response" {
			return nil, fmt.Errorf("headings.%s: unknown key %q (the sides are named by meaning: subject, response)", k, k)
		}
	}
	out := &irPairs{Variant: v.Variant}
	for _, h := range []struct {
		name string
		dst  *Inline
	}{{"subject", &out.Subject}, {"response", &out.Response}} {
		r, ok := heads[h.name]
		if !ok {
			return nil, fmt.Errorf("headings.%s: missing (pairs needs the heading of each side: subject, response)", h.name)
		}
		if err := h.dst.UnmarshalJSON(r); err != nil {
			return nil, fmt.Errorf("headings.%s: %w", h.name, err)
		}
	}
	if len(v.Pairs) < 2 {
		return nil, fmt.Errorf("pairs needs at least two pairs (got %d)", len(v.Pairs))
	}
	var has []bool
	for i, raw := range v.Pairs {
		var p struct {
			Label    *string         `json:"label"`
			Subject  json.RawMessage `json:"subject"`
			Response json.RawMessage `json:"response"`
		}
		if err := strictUnmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("pairs[%d]: %w", i, err)
		}
		var pr irPair
		if p.Label != nil {
			if err := checkPairLabel(*p.Label); err != nil {
				return nil, fmt.Errorf("pairs[%d].label: %w", i, err)
			}
			pr.Label = *p.Label
		}
		has = append(has, p.Label != nil)
		for _, side := range []struct {
			name string
			raw  json.RawMessage
			dst  *irPairSide
		}{{"subject", p.Subject, &pr.Subject}, {"response", p.Response, &pr.Response}} {
			if len(side.raw) == 0 {
				return nil, fmt.Errorf("pairs[%d].%s: missing (a pair needs \"subject\" and \"response\")", i, side.name)
			}
			sd, err := readPairSide(side.raw, fmt.Sprintf("pairs[%d].%s", i, side.name))
			if err != nil {
				return nil, err
			}
			*side.dst = sd
		}
		out.Pairs = append(out.Pairs, pr)
	}
	if k, msg := pairLabelMixed(has); k >= 0 {
		at := fmt.Sprintf("pairs[%d]", k)
		if has[k] {
			at += ".label"
		}
		return nil, fmt.Errorf("%s: %s", at, msg)
	}
	return out, nil
}

// pairsFallback は代わりの組み方:2つのまとまりの columns。見出しはまとまりの題に、
// 各対の2つの側はそれぞれのまとまりに順に入れる(行は2つのまとまりでそろわなく
// なるが読める)。側の見出し(title)はまとまりの中の強調の段落にし、側の body の
// ブロックは順に入れる。まとまりには引用を置けないので、引用は言葉の段落と出どころの注に
// 分ける。場所の表は、まとまりを見出し(/headings/subject・/headings/response)に、
// まとまりの中のブロックを元の対の側(/pairs/K/subject など。引用の注は …/source)に写す。
// 対の名前(label)は subject のまとまりの中の強調の段落にする(/pairs/K/label に写す)。
func pairsFallback(b *IRBlock) ([]*IRBlock, fallbackPaths, error) {
	p, err := readPairs(b.Raw)
	if err != nil {
		return nil, nil, err
	}
	cols := []IRColumn{{Title: p.Subject}, {Title: p.Response}}
	paths := fallbackPaths{
		"/columns/0": {To: "/headings/subject"},
		"/columns/1": {To: "/headings/response"},
	}
	add := func(ci int, orig string, b IRBlock) {
		c := &cols[ci]
		at := func() string { return fmt.Sprintf("/columns/%d/body/%d", ci, len(c.Body)) }
		if b.Type == "quote" {
			paths[at()] = fallbackPath{To: orig}
			c.Body = append(c.Body, IRBlock{Type: "paragraph", Variant: b.Variant, Content: b.Content})
			paths[at()] = fallbackPath{To: orig + "/source"}
			c.Body = append(c.Body, IRBlock{Type: "note", Variant: b.Variant, Content: b.Source})
			return
		}
		paths[at()] = fallbackPath{To: orig, Same: true}
		c.Body = append(c.Body, b)
	}
	put := func(ci, k int, name string, side irPairSide) {
		orig := fmt.Sprintf("/pairs/%d/%s", k, name)
		// 対の名前は、subject のまとまりの中の強調の段落にする。
		if l := p.Pairs[k].Label; l != "" && name == "subject" {
			c := &cols[ci]
			paths[fmt.Sprintf("/columns/%d/body/%d", ci, len(c.Body))] = fallbackPath{To: fmt.Sprintf("/pairs/%d/label", k)}
			c.Body = append(c.Body, IRBlock{Type: "paragraph", Content: Inline{{Type: "em", Content: plainInline(l)}}})
		}
		if !side.Grouped {
			add(ci, orig, side.Body[0])
			return
		}
		// 側の見出しは、まとまりの中の強調の段落にする。
		if side.Title != nil {
			c := &cols[ci]
			paths[fmt.Sprintf("/columns/%d/body/%d", ci, len(c.Body))] = fallbackPath{To: orig + "/title"}
			c.Body = append(c.Body, IRBlock{Type: "paragraph", Content: Inline{{Type: "em", Content: side.Title}}})
		}
		for i, b := range side.Body {
			add(ci, fmt.Sprintf("%s/body/%d", orig, i), b)
		}
	}
	for k, pr := range p.Pairs {
		put(0, k, "subject", pr.Subject)
		put(1, k, "response", pr.Response)
	}
	return []*IRBlock{{Type: "columns", Variant: p.Variant, Columns: cols}}, paths, nil
}

// pairsTypeDecl は pairs が語彙に名乗るもの。
func pairsTypeDecl() typeDecl {
	schema, err := typeSchema(pairsType)
	if err != nil {
		panic(err)
	}
	return typeDecl{
		Schema: schema,
		Markdown: []BlockDecl{
			{Name: "pairs", Label: "2つの側の見出し(「分かったこと | 打ち手」のように | で分ける。文の中の | は \\|)", LabelRequired: true,
				Contains: []string{"subject", "response"}, MinChildren: 4, Attributes: []string{"variant"},
				Meaning: "対の並び(標準でない型)。2つの側の見出しを1回だけ書き、:::subject と :::response を交互に書いて対を並べる。対の2つの側は行がそろう。組み方を持たないテンプレートは ::::columns で組む",
				IR:      "pairs(headings が見出しの文、pairs[] が対)"},
			{Name: "response", Label: "側の見出し(任意。短い文)", Inside: []string{"pairs"}, Contains: []string{"段落", "箇条", ":::quote", "画像(1枚)", "表"}, Attributes: []string{},
				Meaning: "対の応える側(打ち手・対策・答え)。直前の :::subject と対になる。中は1〜4個のブロック(引用・表・画像を入れるときは囲みのコロンを1つ増やす)",
				IR:      "pairs.pairs[].response(1つのブロックか {title, body})"},
			{Name: "subject", Label: "側の見出し(任意。短い文。例: 要因の名前)", Inside: []string{"pairs"}, Contains: []string{"段落", "箇条", ":::quote", "画像(1枚)", "表"}, Attributes: []string{"label"},
				Meaning: "対の何についての側(分かったこと・課題・問い)。直後に :::response を書く。中は1〜4個のブロック(引用・表・画像を入れるときは囲みのコロンを1つ増やす)。{label=名前} でその対の短い名前(12字まで。空白を含むときは \" で囲む。全ての :::subject に書くか、どれにも書かない。描き方はテンプレート)",
				IR:      "pairs.pairs[].subject(1つのブロックか {title, body})。label は pairs.pairs[].label"},
		},
		Fallback: pairsFallback,
		Validate: func(b *IRBlock) error {
			_, err := readPairs(b.Raw)
			return err
		},
		readMarkdown: (*mdParser).pairsBlock,
		Inner:        pairsInner,
	}
}

// pairsInner は check に渡す pairs の中:2つの側の見出し(見える文字が要る)と、各対の
// subject・response(本文のブロックと同じ検査。段落の側はその1文が側の意味なので、
// 見える文字が要る)。見出しと複数のブロックの側は、側の見出しと body の各ブロック。
func pairsInner(m map[string]any) []innerPart {
	var out []innerPart
	if h, ok := m["headings"].(map[string]any); ok {
		out = append(out, innerPart{Path: "/headings", Map: h, Texts: []string{"subject", "response"}})
	}
	pairs, _ := m["pairs"].([]any)
	for k, p := range pairs {
		pm, _ := p.(map[string]any)
		for _, side := range []string{"subject", "response"} {
			sm, ok := pm[side].(map[string]any)
			if !ok {
				continue
			}
			sp := fmt.Sprintf("/pairs/%d/%s", k, side)
			blocks := []innerPart{{Path: sp, Map: sm, Block: true}}
			if _, ok := sm["type"]; !ok {
				// 見出しと複数のブロックの側:見出しは見える文字が要る(書いたとき)。
				blocks = nil
				if _, ok := sm["title"]; ok {
					out = append(out, innerPart{Path: sp, Map: sm, Texts: []string{"title"}})
				}
				body, _ := sm["body"].([]any)
				for i, b := range body {
					if bm, ok := b.(map[string]any); ok {
						blocks = append(blocks, innerPart{Path: fmt.Sprintf("%s/body/%d", sp, i), Map: bm, Block: true})
					}
				}
			}
			for _, part := range blocks {
				if part.Map["type"] == "paragraph" {
					part.Texts = []string{"content"}
				}
				out = append(out, part)
			}
		}
	}
	return out
}

// --- 組み方(同梱のテンプレート) ---

// pairsInfo は pairs の箱の中の部品(はみ出しを対ごとに測る・layout の場所を決める)。
type pairsInfo struct {
	head *Spec // 見出しの行
	// headSubject・headResponse は見出しの行の中の2つの側の見出し。
	headSubject, headResponse *Spec
	list                      *Spec // 対の行の列
	rows                      []pairRow
	// sideMin は側の幅の下限の割合(値の表の pairsSideMin)。
	sideMin float64
	// step は側の幅を探す刻み(pt。値の表の spacingUnit)。
	step float64
	// split・splitWidth は側の幅の比を決めた箱の幅(同じ幅なら決め直さない)。
	split      bool
	splitWidth float64
	// share は中身から決めた subject の側の幅の割合(pairSideShare)。
	share float64
}

// pairRow は1つの対の行と、その2つの側の箱。before は対の前に引いた罫線など、
// 列の中で行の直前に置いた箱(無ければ nil。測るときに上端をずらす)。
type pairRow struct {
	row, subject, response *Spec
	before                 *Spec
	subjectParts           pairSideParts
	responseParts          pairSideParts
}

// pairSideParts は見出しと複数のブロックの側の中の箱(layout の場所を決める)。
// 1つのブロックの形の側では空。
type pairSideParts struct {
	title *Spec
	body  []*Spec
}

// pairsStyle は pairs の組み方の見た目の差し替え口(テンプレートが部品を差し替える
// ときの手。report.go)。零値は標準の組み方。
type pairsStyle struct {
	// headRole は側の見出し(列の題)の色の役割名("" なら text_primary)。
	headRole string
	// sideTitle は側の見出し(title)の箱(nil なら字の色の太字の本文)。
	sideTitle func(t *Template, title Inline) *Spec
	// mark は対の k 番目(0 から)の印の箱。subject の頭(左)に置く(nil なら置かない)。
	// label は印に書く対の名前(名前の無い pairs と、名前を見出しの列に書くときは "")。
	mark func(t *Template, k int, label string) *Spec
	// markLabelMax は対の名前を印(mark)の中に書く字の数の上限。全ての対の名前が
	// これ以下なら印の中に、1つでも長ければ見出しの列(labelColumn)に書く(0 なら
	// 印の中には書かない)。
	markLabelMax int
	// labelColumn は対の名前を書く、行の左の見出しの列の箱(幅をトークンで持つ。
	// nil なら名前は subject の頭に太字で書く)。
	labelColumn func(t *Template, label string) *Spec
	// between は2つの側の間の印の箱(nil なら置かない)。
	between func(t *Template) *Spec
	// rule は対の間の罫線の箱(nil なら引かない)。
	rule func(t *Template) *Spec
	// spacer は見出しの行で、印(mark・between)の幅だけ空ける箱を作る。
	spacer func(t *Template, mark *Spec) *Spec
	// rowGap は対の行の間(罫線を引くなら罫線の上と下それぞれ)の空きのトークン
	// ("" なら blockGap)。
	rowGap string
}

// pairsComponent は pairs の組み方:見出しの行の下に、対ごとに1つの行を置く。
// 行の2つの側は basis 0 で、幅の比(grow)は配置のときに箱の幅と中身から決める
// (splitPairs。組み立ての時点では箱はトークンのままで、幅も分からない)。
// 交差軸は STRETCH なので上端がそろい、
// 行の高さは高い方の側で決まる。見出しの行も同じ幅の割り方なので、側の題が側の
// 真上に来る。見出しと複数のブロックの側は、見出しと body を素直に縦に積む。
func pairsComponent(ctx componentContext, b *IRBlock) (*Spec, error) {
	return pairsStyled(ctx, b, pairsStyle{})
}

// pairsStyled は pairsComponent の見た目を st で差し替えたもの。
func pairsStyled(ctx componentContext, b *IRBlock, st pairsStyle) (*Spec, error) {
	t := ctx.t
	p, err := readPairs(b.Raw)
	if err != nil {
		return nil, err
	}
	headRole := st.headRole
	if headRole == "" {
		headRole = "text_primary"
	}
	grow := func(kids []*Spec, opts ...SpecOpt) *Spec {
		return Col(kids, append([]SpecOpt{WithGrow(1), WithBasis(0)}, opts...)...)
	}
	// row は対の行(見出しの行も同じ割り方):印・subject・間の印・response。
	row := func(mark, a, between, b *Spec) *Spec {
		kids := []*Spec{a, b}
		if between != nil {
			kids = []*Spec{a, between, b}
		}
		if mark != nil {
			kids = append([]*Spec{mark}, kids...)
		}
		return Row(kids, tokGap("columnGap"))
	}
	// 対の名前(label)の置き場所:印の中(短い名前)・行の左の見出しの列・subject の頭。
	labels := p.labels()
	inMark, inColumn := false, false
	if labels != nil {
		inMark = st.mark != nil && st.markLabelMax > 0
		for _, l := range labels {
			if utf8.RuneCountInString(l) > st.markLabelMax {
				inMark = false
			}
		}
		inColumn = !inMark && st.labelColumn != nil
	}
	marks := func(k int) (*Spec, *Spec) {
		var m, bt *Spec
		switch {
		case inColumn:
			m = st.labelColumn(t, labels[k])
		case inMark:
			m = st.mark(t, k, labels[k])
		case st.mark != nil:
			m = st.mark(t, k, "")
		}
		if st.between != nil {
			bt = st.between(t)
		}
		return m, bt
	}
	info := &pairsInfo{sideMin: t.Values.PairsSideMin, step: t.Scale.SpacingUnit}
	info.headSubject = grow([]*Spec{t.text(p.Subject, "columnTitle", headRole, true)})
	info.headResponse = grow([]*Spec{t.text(p.Response, "columnTitle", headRole, true)})
	hm, hb := marks(0)
	if hm != nil {
		hm = st.spacer(t, hm)
	}
	if hb != nil {
		hb = st.spacer(t, hb)
	}
	info.head = apply(row(hm, info.headSubject, hb, info.headResponse), []SpecOpt{noShrink()})
	side := func(k int, name string, sd *irPairSide) (*Spec, pairSideParts, error) {
		var parts pairSideParts
		var kids []*Spec
		if labels != nil && !inMark && !inColumn && name == "subject" {
			// 印も見出しの列も持たない組み方:対の名前を subject の頭に太字で。
			kids = append(kids, apply(t.text(plainInline(labels[k]), "body", "accent", true), []SpecOpt{noShrink()}))
		}
		if sd.Grouped && sd.Title != nil {
			if st.sideTitle != nil {
				parts.title = st.sideTitle(t, sd.Title)
			} else {
				parts.title = t.text(sd.Title, "body", "text_primary", true)
			}
			parts.title = apply(parts.title, []SpecOpt{noShrink()})
			kids = append(kids, parts.title)
		}
		for i := range sd.Body {
			where := fmt.Sprintf("pairs[%d]: %s", k, name)
			if sd.Grouped {
				where = fmt.Sprintf("pairs[%d]: %s: body[%d]", k, name, i)
			}
			s, err := t.buildBlockIn(ctx.within(where), &sd.Body[i])
			if err != nil {
				return nil, parts, fmt.Errorf("%s: %w", where, err)
			}
			// 側の中のブロックは縮めずに要る高さで積む(足りなければ layout-overflow)。
			// 画像など縮められるサブシステムは、Phase3a で先に縮む。
			s = apply(s, []SpecOpt{noShrink()})
			if sd.Grouped {
				parts.body = append(parts.body, s)
			}
			kids = append(kids, s)
		}
		if !sd.Grouped {
			if len(kids) > 1 {
				return grow(kids, tokGap("itemGap")), parts, nil // 名前と1つのブロック
			}
			return grow(kids), parts, nil
		}
		return grow(kids, tokGap("blockGap"), nestedSlack()), parts, nil
	}
	var rows []*Spec
	for k := range p.Pairs {
		pr := &p.Pairs[k]
		var pi pairRow
		var err error
		if pi.subject, pi.subjectParts, err = side(k, "subject", &pr.Subject); err != nil {
			return nil, err
		}
		if pi.response, pi.responseParts, err = side(k, "response", &pr.Response); err != nil {
			return nil, err
		}
		m, bt := marks(k)
		// 対の行も縮めない。足りないときは、中の縮められるサブシステムの分
		// (nestedSlack)だけを先に返し、残りははみ出させて layout-overflow で知らせる。
		pi.row = apply(row(m, pi.subject, bt, pi.response), []SpecOpt{noShrink(), nestedSlack()})
		if k > 0 && st.rule != nil {
			pi.before = apply(st.rule(t), []SpecOpt{noShrink()})
			rows = append(rows, pi.before)
		}
		info.rows = append(info.rows, pi)
		rows = append(rows, pi.row)
	}
	rowGap := st.rowGap
	if rowGap == "" {
		rowGap = "blockGap"
	}
	info.list = Col(rows, tokGap(rowGap), noShrink(), nestedSlack())
	return Col([]*Spec{info.head, info.list}, tokGap("itemGap"), measurePairRows(info), nestedSlack()), nil
}

// pairSideStep は値の表に spacingUnit が無いときの側の幅を探す刻み(pt)。
const pairSideStep = 2.0

// pairSideTie は高さの差がこれ以下の候補を同じ高さと見なす(pt。計算の誤差だけを
// 吸収する)。同じなら半々に近い方。
const pairSideTie = 0.01

// splitPairs は対の並びの箱(印 measurePairRows)の2つの側の幅の比(grow)を、箱の幅
// width から決めて全ての行(見出しの行も)に書く。同じ比にそろえるので側の列は縦に
// そろう。配置(computeLayout)と高さの見積もり(heightForWidth)の頭で呼ぶ。
func splitPairs(s *Spec, width float64) {
	info := s.pairs
	if !info.split || info.splitWidth != width {
		info.split, info.splitWidth = true, width
		info.share = pairSideShare(info, max0(width-2*s.padX()))
	}
	info.setShare(info.share)
}

// setShare は subject の側の幅の割合 share を全ての行(見出しの行も)に書く。
func (info *pairsInfo) setShare(share float64) {
	set := func(subject, response *Spec) {
		if subject != nil {
			subject.Grow = 2 * share
		}
		if response != nil {
			response.Grow = 2 * (1 - share)
		}
	}
	set(info.headSubject, info.headResponse)
	for _, r := range info.rows {
		set(r.subject, r.response)
	}
}

// layoutPairs は対の並びの箱を並べる(computeLayout から)。幅の比は中身から決め
// (splitPairs)、並べた後に、図(縦横比を保つ画像など)が側の幅を使い切らずに残した
// 幅を、どの行も高くならない分だけもう片側へ回して並べ直す(#113)。図は縦の高さで
// 縮んだ大きさのまま描かれるので、回した幅で隣の側の文が低くなる。
func layoutPairs(s *Spec, x, y, width, height float64) {
	splitPairs(s, width)
	layoutNode(s, x, y, width, height)
	if !hasFigure(s.pairs.list) {
		return
	}
	// 回せる幅を測ると(heightForWidth が行を並べ直すので)Box が崩れる。回しても
	// 回さなくても並べ直す。
	if share, ok := pairsGiveShare(s.pairs); ok {
		s.pairs.setShare(share)
	}
	layoutNode(s, x, y, width, height)
}

// pairsGiveShare は、並べた後の側の箱から、図の側の余りをもう片側へ回したときの
// subject の側の幅の割合を決める。回せるのは、その側の列の全ての行で、
// 図の側なら図の外の余り、文の側なら高さが行の高さを越えない分の
// うち一番小さい幅。図の余りを持つ側が無い・回せる幅が無いときは ok が false。
func pairsGiveShare(info *pairsInfo) (float64, bool) {
	type sidePair struct{ subject, response *Spec }
	var ps []sidePair
	if info.headSubject != nil && info.headResponse != nil {
		ps = append(ps, sidePair{info.headSubject, info.headResponse})
	}
	for _, r := range info.rows {
		ps = append(ps, sidePair{r.subject, r.response})
	}
	if len(ps) == 0 {
		return 0, false
	}
	sum := ps[0].subject.Box.Width + ps[0].response.Box.Width
	if sum <= 0 {
		return 0, false
	}
	// give は1つの列(subject か response)から回せる幅。
	give := func(pick func(sidePair) *Spec) (float64, bool) {
		g, figure := math.Inf(1), false
		for _, p := range ps {
			sd := pick(p)
			if sp, ok := sideFigureSpare(sd); ok {
				if sp > spareWidthMin {
					figure = true
				}
				g = math.Min(g, sp)
				continue
			}
			g = math.Min(g, heightSlack(sd))
		}
		if math.IsInf(g, 1) {
			g = 0
		}
		return g, figure
	}
	cur := ps[0].subject.Box.Width / sum
	gs, fs := give(func(p sidePair) *Spec { return p.subject })
	gr, fr := give(func(p sidePair) *Spec { return p.response })
	switch {
	case fs && gs > spareWidthMin && (!fr || gs >= gr):
		return cur - gs/sum, true
	case fr && gr > spareWidthMin:
		return cur + gr/sum, true
	}
	return 0, false
}

// sideFigureSpare は、図(印でないサブシステム)を持つ側の箱 sd を、中身の高さを
// 増やさず図も縮めずに狭められる幅(図の外の余りと、文の葉が折り返しを増やさずに
// 空けられる幅のうち小さい方)。図を持たない側は ok が false。
func sideFigureSpare(sd *Spec) (float64, bool) {
	if !hasFigure(sd) {
		return 0, false
	}
	g := boxGive(sd)
	if math.IsInf(g, 1) {
		g = 0
	}
	return g, true
}

// hasFigure は s の下に図(印でないサブシステムの葉)があるか。
func hasFigure(s *Spec) bool {
	if s.Kind == KindSubsystem && s.Subsystem != nil {
		_, orn := s.Subsystem.(ornament)
		return !orn
	}
	for _, c := range s.Children {
		if hasFigure(c) {
			return true
		}
	}
	return false
}

// boxGive は、並べた後の箱 s を、中身が自分の箱の高さを越えず図も縮めずに狭められる
// 幅(制約の無い箱は +Inf)。図は描く矩形の外の余り、文は高さが箱を越えない分、
// 図を含まない行・列は中身を積んだ高さが箱を越えない分、図を含む列は子の最小、
// 図を含む行は伸びる子(幅の変化を受ける子)の最小。
func boxGive(s *Spec) float64 {
	switch {
	case s.Width != nil:
		return 0
	case s.Kind == KindSubsystem:
		if s.Subsystem == nil {
			return math.Inf(1)
		}
		if _, orn := s.Subsystem.(ornament); orn {
			return 0
		}
		d := subsystemDrawBox(s)
		return max0(s.Box.Width - d.Width)
	case s.Kind == KindText || !hasFigure(s):
		if len(s.Children) == 0 && s.Kind != KindText {
			return math.Inf(1)
		}
		return heightSlack(s)
	}
	g := math.Inf(1)
	for _, c := range s.Children {
		if s.Kind == KindRow && c.Grow == 0 {
			continue
		}
		g = math.Min(g, boxGive(c))
	}
	return g
}

// heightSlack は、箱 s を、縮めずに積んだ高さ(heightForWidth)が並べた後の箱の高さを
// 越えずに狭められる幅。
func heightSlack(s *Spec) float64 {
	limit := s.Box.Width - 1
	if limit <= 0 {
		return 0
	}
	fits := func(d float64) bool { return heightForWidth(s, s.Box.Width-d) <= s.Box.Height+overflowEps }
	if fits(limit) {
		return limit
	}
	if !fits(0) {
		return 0
	}
	lo, hi := 0.0, limit
	for hi-lo > 0.25 {
		mid := (lo + hi) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// pairSideShare は subject の側の幅の割合(2つの側の幅の合計に対する。0〜1)。中身を
// 縮めずに積んだ高さ(heightForWidth)の、全ての行の高い方の側の合計が一番小さくなる
// 割合を pairsSideMin〜1 − pairsSideMin の中から選ぶ(同じなら半々に近い方)。短い側
// (論点・名前)は自分が折り返さない幅まで狭くなり、長い側が広くなる。両側が同じくらい
// なら半々のまま。width は対の箱の中の幅。
//
// 候補(#115):最初の行の側の幅の合計を spacingUnit(pt)の刻みで割った全ての幅と、
// 下限・上限・半々・5% 刻みの割合。740pt を report(spacingUnit 2pt・下限 0.2)で割ると約 220 の
// 候補(と 5% 刻みの十数個)で、1つの候補は行ごとに2回 heightForWidth を測る(見出しの行を含めて n 行なら約 460n 回)。
// 探し直すのは箱の幅が変わったときだけ(splitPairs)。
func pairSideShare(info *pairsInfo, width float64) float64 {
	if info.sideMin <= 0 || info.sideMin >= 0.5 {
		return 0.5
	}
	total, base := pairsNeed(info, width)
	if total == nil {
		return 0.5
	}
	lo, hi := info.sideMin, 1-info.sideMin
	shares := []float64{0.5, lo, hi}
	// 5% 刻みの割合(前の候補)も残す。pt の刻みの間に落ちても、前より高くならない。
	for k := 1; k < 20; k++ {
		if sh := float64(k) / 20; sh > lo && sh < hi {
			shares = append(shares, sh)
		}
	}
	if base > 0 {
		step := info.step
		if !(step > 0) {
			step = pairSideStep
		}
		d := step / base
		for sh := lo + d; sh < hi-1e-9; sh += d {
			shares = append(shares, sh)
		}
	}
	hs := make([]float64, len(shares))
	bestH := math.Inf(1)
	for i, sh := range shares {
		hs[i] = total(sh)
		bestH = math.Min(bestH, hs[i])
	}
	// 一番低い高さから pairSideTie 以内の候補のうち、半々に一番近いもの。
	best, bestD := 0.5, math.Inf(1)
	for i, sh := range shares {
		if d := math.Abs(sh - 0.5); hs[i] <= bestH+pairSideTie && d < bestD {
			best, bestD = sh, d
		}
	}
	return best
}

// pairsNeed は、subject の側の幅の割合 share で割ったときの対の並びの要る高さ(全ての行
// (見出しの行も)の、高い方の側を縮めずに積んだ高さの合計)を返す関数と、最初の行の側の
// 幅の合計(pt)。width は対の箱の中の幅。行が無ければ nil。
func pairsNeed(info *pairsInfo, width float64) (func(share float64) float64, float64) {
	type pair struct{ subject, response, row *Spec }
	var ps []pair
	if info.head != nil {
		ps = append(ps, pair{info.headSubject, info.headResponse, info.head})
	}
	for _, r := range info.rows {
		ps = append(ps, pair{r.subject, r.response, r.row})
	}
	if len(ps) == 0 {
		return nil, 0
	}
	// 側の幅の合計:行の幅から、側でない子(印)と隙間を引いたもの。
	sides := func(row *Spec) float64 {
		w := width - 2*row.padX() - row.Gap*float64(max(0, len(row.Children)-1))
		for _, c := range row.Children {
			if c.Grow == 0 {
				w -= naturalSize(c, 'w')
			}
		}
		return max0(w)
	}
	avails := make([]float64, len(ps))
	for i, p := range ps {
		avails[i] = sides(p.row)
	}
	total := func(share float64) float64 {
		h := 0.0
		for i, p := range ps {
			a := heightForWidth(p.subject, avails[i]*share)
			b := heightForWidth(p.response, avails[i]*(1-share))
			h += math.Max(a, b)
		}
		return h
	}
	return total, avails[0]
}

// registerBundledComponents は同梱のテンプレート(templates/*.json。いまは dark・light・report)が
// 持つ、標準でない型の組み方を r に足す。外の値の表だけのテンプレート(ParseTemplate)は
// 持たず、代わりの組み方で組む。テンプレートが自分の組み方をすでに登録した型
// (report.go)はそのままにする。
func registerBundledComponents(r *componentRegistry) {
	if !r.has(pairsType) {
		must(r.register(pairsType, componentFunc(pairsComponent)))
	}
	registerTopicsComponent(r)
}

// --- はみ出しと容量 ---

// pairMeasure は対の1つの測り値。Top・Bottom は本文の領域の上端から。
type pairMeasure struct {
	Index        int
	Need         float64 // 行の高さ(高い方の側の高さ)
	SubjectNeed  float64 // subject の側を縮めずに積んだ高さ
	ResponseNeed float64
	Lines        int // 高い方の側の文字の行の数
	Top          float64
	Bottom       float64
	Overflow     float64
}

// measurePairs は対を1つずつ縮めずに測る(top はブロックの上端、width はブロックの
// 中の幅)。requiredMeasure が行を並べるので、描いた後に呼ぶこと。
func measurePairs(block *Spec, info *pairsInfo, top, width, capacity float64) []pairMeasure {
	cursor := top
	if info.head != nil {
		h, _ := requiredMeasure(info.head, width)
		cursor += h + block.Gap
	}
	var out []pairMeasure
	for k, r := range info.rows {
		if k > 0 {
			cursor += info.list.Gap
		}
		if r.before != nil {
			bh, _ := requiredMeasure(r.before, width)
			cursor += bh + info.list.Gap
		}
		h, n := requiredMeasure(r.row, width)
		sh, _ := requiredMeasure(r.subject, r.subject.Box.Width)
		rh, _ := requiredMeasure(r.response, r.response.Box.Width)
		pm := pairMeasure{Index: k, Need: h, SubjectNeed: sh, ResponseNeed: rh, Lines: n, Top: cursor, Bottom: cursor + h}
		pm.Overflow = overflowOf(pm.Top, pm.Bottom, capacity)
		out = append(out, pm)
		cursor += h
	}
	return out
}

// PairCapacity は容量の中の対の1つ(Top・Bottom は本文の領域の上端から)。対の高さを
// 決めるのは高い方の側で(tallest)、低い方に文を足しても対の高さは増えない。
type PairCapacity struct {
	Line     int     `json:"line,omitempty"`
	Path     string  `json:"path"` // /slides/S/body/N/pairs/K
	Need     float64 `json:"need"`
	Lines    int     `json:"lines"`
	Top      float64 `json:"top"`
	Bottom   float64 `json:"bottom"`
	Overflow float64 `json:"overflow"`
	// Subject・Response は側ごとの高さ。
	Subject  float64 `json:"subject"`
	Response float64 `json:"response"`
	// Tallest は対の高さを決める側("subject"・"response")。
	Tallest string `json:"tallest"`
}

func pairCapacity(m pairMeasure, path string) PairCapacity {
	tall := "subject"
	if m.ResponseNeed > m.SubjectNeed+overflowEps {
		tall = "response"
	}
	return PairCapacity{Path: fmt.Sprintf("%s/pairs/%d", path, m.Index), Need: round1(m.Need), Lines: m.Lines,
		Top: round1(m.Top), Bottom: round1(m.Bottom), Overflow: round1(m.Overflow),
		Subject: round1(m.SubjectNeed), Response: round1(m.ResponseNeed), Tallest: tall}
}

// pairOverflowWarnings ははみ出した対ごとの警告の文。1つも無ければ nil(見出しの
// 行だけがはみ出したときは、ブロックの警告にする)。
func pairOverflowWarnings(o BlockMeasure, capacity, need float64) []string {
	var out []string
	for _, p := range o.Pairs {
		if p.Overflow <= 0 {
			continue
		}
		out = append(out, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] body[%d]: pairs[%d]: pair %d (pairs) overflows the body area by %.1fpt (pair needs %.1fpt: subject %.1fpt, response %.1fpt; body area %.1fpt, all blocks need %.1fpt)",
			o.Index, p.Index, p.Index, p.Overflow, p.Need, p.SubjectNeed, p.ResponseNeed, capacity, need))
	}
	return out
}

// --- Markdown ---

// pairsBlock は ::::pairs 見出し | 見出し … :::: を IR の pairs にする。中には
// :::subject と :::response を交互に書き、1組が1つの対になる。側の中身は段落・
// 箇条・:::quote のどれか1つか、見出しの文(:::subject 見出し)と複数のブロック(pairSide)。
func (p *mdParser) pairsBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	ok := true
	heads := splitPipes(f.label)
	if len(f.label.r) == 0 || len(heads) != 2 || len(heads[0].r) == 0 || len(heads[1].r) == 0 {
		col := 1
		if len(f.label.r) > 0 {
			col = f.label.pos[0].col
		}
		p.fail(f.line, col, path+"/headings", codeMissingKey, "::::pairs には2つの側の見出しを | で分けて書きます", nil,
			"「::::pairs 分かったこと | 打ち手」のように、囲みの名前の後ろに書いてください(文の中の | は \\|)")
		ok = false
	}
	var pairs []any
	var cur *jobj // subject を読んで response を待っている対
	// labelAt は対の名前({label=…})を書いた :::subject の行(書かなければ 0)。対ごと。
	var labelAt []int
	var curLabel int
	for _, it := range f.items {
		if it.kind != mdFence || (it.fence.name != "subject" && it.fence.name != "response") {
			p.fail(it.line, 1, path, codeMisplaced, "::::pairs の中には :::subject と :::response だけを書けます", []string{"subject", "response"}, "")
			continue
		}
		sf := it.fence
		k := len(pairs)
		pp := fmt.Sprintf("%s/pairs/%d", path, k)
		if sf.name == "subject" {
			if cur != nil {
				p.fail(sf.line, 1, pp, codeMisplaced, ":::subject の後には :::response を書きます", []string{"response"}, "対は :::subject と :::response の1組です")
				cur = nil
			}
			p.mark(pp, sf.line)
			cur = &jobj{}
			curLabel = 0
			attrs := p.fenceAttrs(sf, pp+"/subject", []string{"label"})
			if l, has := attrs["label"]; has {
				if err := checkPairLabel(l); err != nil {
					p.fail(sf.line, sf.attrsCol, pp+"/label", codeInvalidValue,
						fmt.Sprintf("対の名前 %q は書けません(見える字を含む %d 字まで)", l, pairLabelMaxRunes), nil,
						"{label=要因1} や {label=\"住まい の 課題\"} のように短く書きます(空白を含むときは \" で囲みます)")
					ok = false
				} else {
					cur.set("label", l)
					curLabel = sf.line
				}
			}
		} else if cur == nil {
			p.fail(sf.line, 1, pp, codeMisplaced, ":::response の前に :::subject がありません", []string{"subject"}, "対は :::subject と :::response の1組です")
			continue
		}
		sp := pp + "/" + sf.name
		p.mark(sp, sf.line)
		if sf.name == "response" {
			p.fenceAttrs(sf, sp, nil)
		}
		side := p.pairSide(sf, sp)
		if side == nil {
			ok = false
		} else {
			cur.set(sf.name, side)
		}
		if sf.name == "response" {
			if slices.Contains(cur.keys, "subject") && slices.Contains(cur.keys, "response") {
				pairs = append(pairs, cur)
				labelAt = append(labelAt, curLabel)
			}
			cur = nil
		}
	}
	if cur != nil {
		p.fail(f.line, 1, path, codeMisplaced, "最後の :::subject に :::response がありません", []string{"response"}, "")
		ok = false
	}
	// 対の名前は、全ての対に書くか、どれにも書かない。
	has := make([]bool, len(labelAt))
	for k, l := range labelAt {
		has[k] = l > 0
	}
	if k, _ := pairLabelMixed(has); k >= 0 && ok {
		at, line := fmt.Sprintf("%s/pairs/%d", path, k), p.lines[fmt.Sprintf("%s/pairs/%d", path, k)]
		if has[k] {
			at, line = at+"/label", labelAt[k]
		}
		hint := fmt.Sprintf("1つ目の対には名前があり、%d つ目の対にはありません", k+1)
		if !has[0] {
			hint = fmt.Sprintf("1つ目の対には名前が無く、%d つ目の対にはあります", k+1)
		}
		p.fail(line, 1, at, codePairsLabel, "対の名前({label=…})は、全ての :::subject に書くか、どれにも書きません", nil, hint)
		ok = false
	}
	if len(pairs) < 2 && ok {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf("::::pairs には対(:::subject と :::response)を2つ以上書きます(いま %d 個)", len(pairs)), nil,
			"対が1つなら、段落か ::::columns にしてください")
		return nil
	}
	if !ok {
		return nil
	}
	b := obj("type", pairsType)
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("headings", obj("subject", p.parseInline(heads[0], path+"/headings/subject"),
		"response", p.parseInline(heads[1], path+"/headings/response")))
	b.set("pairs", pairs)
	return b
}

// pairSide は :::subject・:::response の中を IR の側にする。見出しの文(:::subject 見出し)が
// 無く、中がただ1つの段落・箇条・:::quote なら1つのブロックの形。見出しの文があるか、
// 中に複数のブロック・画像・表があれば、見出しと複数のブロックの形({title?, body})。
// body に置けるのは段落・箇条・:::quote・画像・表の1〜4個。
func (p *mdParser) pairSide(f *mdFenceBlk, path string) any {
	var allowed []*mdItem
	bad := false
	for _, it := range f.items {
		ok := it.kind == mdPara || it.kind == mdList || it.kind == mdTable || (it.kind == mdFence && it.fence.name == "quote")
		if !ok {
			what := "コードブロック"
			if it.kind == mdFence {
				what = ":::" + it.fence.name
			}
			p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中に %s は書けません", f.name, what), []string{"段落", "箇条", "quote", "画像", "表"}, "")
			bad = true
			continue
		}
		allowed = append(allowed, it)
	}
	if len(f.items) == 0 {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中が空です", f.name), nil, "")
		return nil
	}
	if len(allowed) > pairSideBodyMax {
		p.fail(allowed[pairSideBodyMax].line, 1, path+"/body", codeMisplaced,
			fmt.Sprintf(":::%s の中にはブロックを %d 個まで書けます(いま %d 個)", f.name, pairSideBodyMax, len(allowed)), nil,
			"文を足すなら同じ段落か箇条に入れるか、対を分けてください")
		bad = true
	}
	if bad {
		return nil
	}
	if len(f.label.r) == 0 && len(allowed) == 1 {
		it := allowed[0]
		switch {
		case it.kind == mdPara:
			if _, img := scanImages(it.text); !img {
				return obj("type", "paragraph", "content", p.parseInline(it.text.trimBR(), path+"/content"))
			}
		case it.kind == mdList:
			pts := obj("type", "points", "items", p.listItems(it.items, path+"/items", true))
			if p.listOrdered(it.items, path+"/items") {
				pts.set("ordered", true)
			}
			return pts
		case it.kind == mdFence:
			return p.quoteBlock(it.fence, path)
		}
	}
	side := &jobj{}
	if len(f.label.r) > 0 {
		side.set("title", p.parseInline(f.label, path+"/title"))
	}
	body := p.blocks(allowed, path+"/body")
	for k, b := range body {
		if o, ok := b.(*jobj); ok && len(o.vals) > 0 && o.vals[0] == "gallery" {
			bp := fmt.Sprintf("%s/body/%d", path, k)
			p.fail(p.lines[bp], 1, bp, codeMisplaced, fmt.Sprintf(":::%s の中には画像を1枚ずつしか置けません", f.name), nil, "")
			return nil
		}
		if o, ok := b.(*jobj); ok && slices.Contains(o.keys, "continues") {
			bp := fmt.Sprintf("%s/body/%d", path, k)
			p.fail(p.lines[bp], 1, bp+"/continues", codeMisplaced, "続きの表(continues)は対の側の中に置けません", nil, "続きの表は本文に置きます")
			return nil
		}
	}
	if len(body) != len(allowed) {
		return nil // 中のブロックの誤りは p.blocks が場所つきで返した
	}
	side.set("body", body)
	return side
}

// splitPipes は見出しの文を | で分ける(\| は文の中の |)。前後の空白は外す。
func splitPipes(t mdText) []mdText {
	var out []mdText
	start := 0
	for k := 0; k < len(t.r); k++ {
		if t.r[k] == '\\' && k+1 < len(t.r) {
			k++
			continue
		}
		if t.r[k] == '|' {
			out = append(out, unescapePipes(trimText(mdText{r: t.r[start:k], pos: t.pos[start:k]})))
			start = k + 1
		}
	}
	return append(out, unescapePipes(trimText(mdText{r: t.r[start:], pos: t.pos[start:]})))
}
