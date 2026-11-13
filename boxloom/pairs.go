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
	"slices"
	"strings"
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

// irPair は1つの対。
type irPair struct {
	Subject, Response irPairSide
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
	for i, raw := range v.Pairs {
		var p struct {
			Subject  json.RawMessage `json:"subject"`
			Response json.RawMessage `json:"response"`
		}
		if err := strictUnmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("pairs[%d]: %w", i, err)
		}
		var pr irPair
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
	return out, nil
}

// pairsFallback は代わりの組み方:2つのまとまりの columns。見出しはまとまりの題に、
// 各対の2つの側はそれぞれのまとまりに順に入れる(行は2つのまとまりでそろわなく
// なるが読める)。側の見出し(title)はまとまりの中の強調の段落にし、側の body の
// ブロックは順に入れる。まとまりには引用を置けないので、引用は言葉の段落と出どころの注に
// 分ける。場所の表は、まとまりを見出し(/headings/subject・/headings/response)に、
// まとまりの中のブロックを元の対の側(/pairs/K/subject など。引用の注は …/source)に写す。
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
			{Name: "subject", Label: "側の見出し(任意。短い文。例: 要因の名前)", Inside: []string{"pairs"}, Contains: []string{"段落", "箇条", ":::quote", "画像(1枚)", "表"}, Attributes: []string{},
				Meaning: "対の何についての側(分かったこと・課題・問い)。直後に :::response を書く。中は1〜4個のブロック(引用・表・画像を入れるときは囲みのコロンを1つ増やす)",
				IR:      "pairs.pairs[].subject(1つのブロックか {title, body})"},
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
	mark func(t *Template, k int) *Spec
	// between は2つの側の間の印の箱(nil なら置かない)。
	between func(t *Template) *Spec
	// rule は対の間の罫線の箱(nil なら引かない)。
	rule func(t *Template) *Spec
	// spacer は見出しの行で、印(mark・between)の幅だけ空ける箱を作る。
	spacer func(t *Template, mark *Spec) *Spec
}

// pairsComponent は pairs の組み方:見出しの行の下に、対ごとに1つの行を置く。
// 行の2つの側は同じ幅(grow 1・basis 0)で、交差軸は STRETCH なので上端がそろい、
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
	marks := func(k int) (*Spec, *Spec) {
		var m, bt *Spec
		if st.mark != nil {
			m = st.mark(t, k)
		}
		if st.between != nil {
			bt = st.between(t)
		}
		return m, bt
	}
	info := &pairsInfo{}
	info.headSubject = grow([]*Spec{t.text(p.Subject, "columnTitle", headRole, true)})
	info.headResponse = grow([]*Spec{t.text(p.Response, "columnTitle", headRole, true)})
	hm, hb := marks(0)
	if hm != nil {
		hm = st.spacer(t, hm)
	}
	if hb != nil {
		hb = st.spacer(t, hb)
	}
	info.head = row(hm, info.headSubject, hb, info.headResponse)
	side := func(k int, name string, sd *irPairSide) (*Spec, pairSideParts, error) {
		var parts pairSideParts
		var kids []*Spec
		if sd.Grouped && sd.Title != nil {
			if st.sideTitle != nil {
				parts.title = st.sideTitle(t, sd.Title)
			} else {
				parts.title = t.text(sd.Title, "body", "text_primary", true)
			}
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
			if sd.Grouped {
				parts.body = append(parts.body, s)
			}
			kids = append(kids, s)
		}
		if !sd.Grouped {
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
		pi.row = row(m, pi.subject, bt, pi.response)
		if k > 0 && st.rule != nil {
			pi.before = st.rule(t)
			rows = append(rows, pi.before)
		}
		info.rows = append(info.rows, pi)
		rows = append(rows, pi.row)
	}
	info.list = Col(rows, tokGap("blockGap"))
	return Col([]*Spec{info.head, info.list}, tokGap("itemGap"), measurePairRows(info)), nil
}

// registerBundledComponents は同梱のテンプレート(templates/*.json。いまは dark・light・report)が
// 持つ、標準でない型の組み方を r に足す。外の値の表だけのテンプレート(ParseTemplate)は
// 持たず、代わりの組み方で組む。テンプレートが自分の組み方をすでに登録した型
// (report.go)はそのままにする。
func registerBundledComponents(r *componentRegistry) {
	if !r.has(pairsType) {
		must(r.register(pairsType, componentFunc(pairsComponent)))
	}
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
		} else if cur == nil {
			p.fail(sf.line, 1, pp, codeMisplaced, ":::response の前に :::subject がありません", []string{"subject"}, "対は :::subject と :::response の1組です")
			continue
		}
		sp := pp + "/" + sf.name
		p.mark(sp, sf.line)
		p.fenceAttrs(sf, sp, nil)
		side := p.pairSide(sf, sp)
		if side == nil {
			ok = false
		} else {
			cur.set(sf.name, side)
		}
		if sf.name == "response" {
			if len(cur.vals) == 2 {
				pairs = append(pairs, cur)
			}
			cur = nil
		}
	}
	if cur != nil {
		p.fail(f.line, 1, path, codeMisplaced, "最後の :::subject に :::response がありません", []string{"response"}, "")
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
