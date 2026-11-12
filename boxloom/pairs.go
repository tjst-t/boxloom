// pairs(対の並び):最初の標準でない型(docs/layers.md の8.1)。
//
// 「分かったこと ↔ 打ち手」のように、2つの側の見出しを1回だけ書き、その下に対を
// 並べる。側の名前は意味の名前で、subject はその対が何についてか(分かったこと・
// 課題・問い)、response はそれに応えるもの(打ち手・対策・答え)。どちらを左に
// 置くかはテンプレートが決める。対の2つの側は、それぞれ段落・箇条・引用のどれか1つ。
//
//   - 型の宣言(pairsTypeDecl):Schema は schema/types/pairs.schema.json の写し、
//     IR を読むときの形の検査、Markdown の囲み(::::pairs と :::subject・:::response)
//     とその読み方、代わりの組み方(左右2つのまとまりの columns)。
//   - 組み方(pairsComponent):同梱のテンプレート(dark・light)が持つ。対ごとに
//     1つの行にし、2つの側を同じ上端から置いて、行の高さを高い方に合わせる。
//     はみ出しは対ごとに測る(測る単位の印 measurePairRows)。

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

// pairSideTypes は対の1つの側に置けるブロックの種類。
var pairSideTypes = []string{"paragraph", "points", "quote"}

// irPairs は pairs のブロックの中身(IRBlock.Raw を読んだもの)。
type irPairs struct {
	Variant  string
	Subject  Inline // headings.subject
	Response Inline // headings.response
	Pairs    []irPair
}

// irPair は1つの対。
type irPair struct {
	Subject, Response IRBlock
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
			dst  *IRBlock
		}{{"subject", p.Subject, &pr.Subject}, {"response", p.Response, &pr.Response}} {
			if len(side.raw) == 0 {
				return nil, fmt.Errorf("pairs[%d].%s: missing (a pair needs \"subject\" and \"response\")", i, side.name)
			}
			if err := side.dst.UnmarshalJSON(side.raw); err != nil {
				return nil, fmt.Errorf("pairs[%d].%s: %w", i, side.name, err)
			}
			if !contains(pairSideTypes, side.dst.Type) {
				return nil, fmt.Errorf("pairs[%d].%s: a side of a pair cannot be a %s block (allowed: %s)", i, side.name, side.dst.Type, strings.Join(pairSideTypes, ", "))
			}
		}
		out.Pairs = append(out.Pairs, pr)
	}
	return out, nil
}

// pairsFallback は代わりの組み方:2つのまとまりの columns。見出しはまとまりの題に、
// 各対の2つの側はそれぞれのまとまりに順に入れる(行は2つのまとまりでそろわなく
// なるが読める)。まとまりには引用を置けないので、引用は言葉の段落と出どころの注に
// 分ける。
func pairsFallback(b *IRBlock) ([]*IRBlock, error) {
	p, err := readPairs(b.Raw)
	if err != nil {
		return nil, err
	}
	subject := IRColumn{Title: p.Subject}
	response := IRColumn{Title: p.Response}
	put := func(c *IRColumn, side IRBlock) {
		if side.Type == "quote" {
			c.Body = append(c.Body,
				IRBlock{Type: "paragraph", Variant: side.Variant, Content: side.Content},
				IRBlock{Type: "note", Variant: side.Variant, Content: side.Source})
			return
		}
		c.Body = append(c.Body, side)
	}
	for _, pr := range p.Pairs {
		put(&subject, pr.Subject)
		put(&response, pr.Response)
	}
	return []*IRBlock{{Type: "columns", Variant: p.Variant, Columns: []IRColumn{subject, response}}}, nil
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
			{Name: "response", Inside: []string{"pairs"}, Contains: []string{"段落1つ", "箇条1つ", ":::quote 1つ"}, Attributes: []string{},
				Meaning: "対の応える側(打ち手・対策・答え)。直前の :::subject と対になる",
				IR:      "pairs.pairs[].response"},
			{Name: "subject", Inside: []string{"pairs"}, Contains: []string{"段落1つ", "箇条1つ", ":::quote 1つ"}, Attributes: []string{},
				Meaning: "対の何についての側(分かったこと・課題・問い)。直後に :::response を書く",
				IR:      "pairs.pairs[].subject"},
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
// 見える文字が要る)。
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
			part := innerPart{Path: fmt.Sprintf("/pairs/%d/%s", k, side), Map: sm, Block: true}
			if sm["type"] == "paragraph" {
				part.Texts = []string{"content"}
			}
			out = append(out, part)
		}
	}
	return out
}

// --- 組み方(同梱のテンプレート) ---

// pairsInfo は pairs の箱の中の部品(はみ出しを対ごとに測る・layout の場所を決める)。
type pairsInfo struct {
	head *Spec // 見出しの行
	list *Spec // 対の行の列
	rows []pairRow
}

// pairRow は1つの対の行と、その2つの側の箱。
type pairRow struct {
	row, subject, response *Spec
}

// pairsComponent は pairs の組み方:見出しの行の下に、対ごとに1つの行を置く。
// 行の2つの側は同じ幅(grow 1・basis 0)で、交差軸は STRETCH なので上端がそろい、
// 行の高さは高い方の側で決まる。見出しの行も同じ幅の割り方なので、側の題が側の
// 真上に来る。
func pairsComponent(ctx componentContext, b *IRBlock) (*Spec, error) {
	t := ctx.t
	p, err := readPairs(b.Raw)
	if err != nil {
		return nil, err
	}
	side := func(s *Spec) *Spec { return Col([]*Spec{s}, WithGrow(1), WithBasis(0)) }
	row := func(a, b *Spec) *Spec { return Row([]*Spec{a, b}, tokGap("columnGap")) }
	info := &pairsInfo{}
	info.head = row(
		side(t.text(p.Subject, "columnTitle", "text_primary", true)),
		side(t.text(p.Response, "columnTitle", "text_primary", true)))
	var rows []*Spec
	for k := range p.Pairs {
		pr := &p.Pairs[k]
		s, err := t.buildBlockIn(ctx.within(fmt.Sprintf("pairs[%d]: subject", k)), &pr.Subject)
		if err != nil {
			return nil, fmt.Errorf("pairs[%d]: subject: %w", k, err)
		}
		r, err := t.buildBlockIn(ctx.within(fmt.Sprintf("pairs[%d]: response", k)), &pr.Response)
		if err != nil {
			return nil, fmt.Errorf("pairs[%d]: response: %w", k, err)
		}
		pi := pairRow{subject: side(s), response: side(r)}
		pi.row = row(pi.subject, pi.response)
		info.rows = append(info.rows, pi)
		rows = append(rows, pi.row)
	}
	info.list = Col(rows, tokGap("blockGap"))
	return Col([]*Spec{info.head, info.list}, tokGap("itemGap"), measurePairRows(info)), nil
}

// registerBundledComponents は同梱のテンプレート(templates/*.json。いまは dark・light)が
// 持つ、標準でない型の組み方を r に足す。外の値の表だけのテンプレート(ParseTemplate)は
// 持たず、代わりの組み方で組む。
func registerBundledComponents(r *componentRegistry) {
	must(r.register(pairsType, componentFunc(pairsComponent)))
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
// 箇条・:::quote のどれか1つ。
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
		p.noLabel(sf, sp)
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

// pairSide は :::subject・:::response の中のただ1つの段落・箇条・:::quote を IR の
// ブロックにする。
func (p *mdParser) pairSide(f *mdFenceBlk, path string) any {
	var out any
	n := 0
	for _, it := range f.items {
		n++
		if n > 1 {
			p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中には段落・箇条・:::quote を1つだけ書けます", f.name), nil,
				"文を足すなら同じ段落か箇条に入れるか、対を分けてください")
			continue
		}
		switch {
		case it.kind == mdPara:
			out = obj("type", "paragraph", "content", p.parseInline(it.text.trimBR(), path+"/content"))
		case it.kind == mdList:
			out = obj("type", "points", "items", p.listItems(it.items, path+"/items", true))
		case it.kind == mdFence && it.fence.name == "quote":
			out = p.quoteBlock(it.fence, path)
		default:
			what := "表"
			switch it.kind {
			case mdFence:
				what = ":::" + it.fence.name
			case mdCode:
				what = "コードブロック"
			}
			p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中に %s は書けません", f.name, what), []string{"段落", "箇条", "quote"}, "")
		}
	}
	if n == 0 {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf(":::%s の中が空です", f.name), nil, "")
	}
	if n != 1 {
		return nil
	}
	return out
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
