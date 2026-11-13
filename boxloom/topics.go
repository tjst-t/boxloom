// topics(印つきの塊の並び):標準でない型(docs/layers.md の8.1、Backlog #119)。
//
// いくつかの観点を、それぞれ題と説明で上から順に並べる。塊は題(必須)・短い印
// (label。A・B・1 のような2字まで。全ての塊に付けるか、全てに付けない)・中身
// (段落・箇条・引用の1〜3個)を持つ。対(pairs。左右の対応)でも列(columns。横の
// 並び)でもなく、縦に読む塊の並び。
//
//   - 型の宣言(topicsTypeDecl):Schema は schema/types/topics.schema.json の写し、
//     IR を読むときの形の検査、Markdown の囲み(::::topics と :::topic)とその読み方、
//     代わりの組み方(題を親、中身を子にした箇条)。
//   - 組み方(topicsComponent):同梱のテンプレート(dark・light)は題を太字にし、印を
//     題の頭に書く。report は左に丸い印(基調色)、右に基調色の題と中身、塊の間に罫線
//     (topicsStyled に見た目を渡す。report.go)。はみ出しは塊ごとに測る(印 measureTopicRows)。

package boxloom

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// topicsType は IR の type。
const topicsType = "topics"

// topicsMin・topicsMax は塊の数の下限と上限。
const (
	topicsMin = 2
	topicsMax = 6
)

// topicBodyTypes は塊の中に置けるブロックの種類。
var topicBodyTypes = []string{"paragraph", "points", "quote"}

// topicBodyMax は塊の中に置けるブロックの数の上限。
const topicBodyMax = 3

// topicLabelMaxRunes は塊の印の字の数の上限。
const topicLabelMaxRunes = 2

// irTopics は topics のブロックの中身(IRBlock.Raw を読んだもの)。
type irTopics struct {
	Variant string
	Topics  []irTopic
}

// irTopic は1つの塊。Label は印(無ければ "")。
type irTopic struct {
	Title Inline
	Label string
	Body  []IRBlock
}

// checkTopicLabel は塊の印が、空白を含まない1〜2字の文であることを確かめる。
func checkTopicLabel(l string) error {
	n := utf8.RuneCountInString(l)
	if n == 0 {
		return fmt.Errorf("label must not be empty")
	}
	if n > topicLabelMaxRunes {
		return fmt.Errorf("label %q is longer than %d characters", l, topicLabelMaxRunes)
	}
	if strings.IndexFunc(l, unicode.IsSpace) >= 0 {
		return fmt.Errorf("label %q contains a space", l)
	}
	return nil
}

// readTopics は topics の IR を読んで形を見る(知らないキー・足りない欄・数・置けない
// 種類・印の混在は誤り)。誤りの文の頭は IR の場所(topics[1].body[0]: …)。
func readTopics(raw json.RawMessage) (*irTopics, error) { return readTopicsMin(raw, topicsMin) }

// readTopicsMin は readTopics の、塊の数の下限を min にしたもの(-fit で分けた続きの枚の
// 切れ端は1つでもよい。fit_split.go)。
func readTopicsMin(raw json.RawMessage, min int) (*irTopics, error) {
	var v struct {
		Type    string            `json:"type"`
		Variant string            `json:"variant"`
		Topics  []json.RawMessage `json:"topics"`
	}
	if err := strictUnmarshal(raw, &v); err != nil {
		return nil, err
	}
	if len(v.Topics) < min || len(v.Topics) > topicsMax {
		return nil, fmt.Errorf("topics needs %d to %d topics (got %d)", topicsMin, topicsMax, len(v.Topics))
	}
	out := &irTopics{Variant: v.Variant}
	labeled := 0
	for i, r := range v.Topics {
		var t struct {
			Title json.RawMessage   `json:"title"`
			Label *string           `json:"label"`
			Body  []json.RawMessage `json:"body"`
		}
		if err := strictUnmarshal(r, &t); err != nil {
			return nil, fmt.Errorf("topics[%d]: %w", i, err)
		}
		var tp irTopic
		if len(t.Title) == 0 {
			return nil, fmt.Errorf("topics[%d].title: missing (a topic needs a title)", i)
		}
		if err := tp.Title.UnmarshalJSON(t.Title); err != nil {
			return nil, fmt.Errorf("topics[%d].title: %w", i, err)
		}
		if t.Label != nil {
			if err := checkTopicLabel(*t.Label); err != nil {
				return nil, fmt.Errorf("topics[%d].label: %w", i, err)
			}
			tp.Label = *t.Label
			labeled++
		}
		if len(t.Body) == 0 || len(t.Body) > topicBodyMax {
			return nil, fmt.Errorf("topics[%d].body: a topic needs 1 to %d blocks (got %d)", i, topicBodyMax, len(t.Body))
		}
		for j, br := range t.Body {
			var b IRBlock
			if err := b.UnmarshalJSON(br); err != nil {
				return nil, fmt.Errorf("topics[%d].body[%d]: %w", i, j, err)
			}
			if !contains(topicBodyTypes, b.Type) {
				return nil, fmt.Errorf("topics[%d].body[%d]: a topic cannot have a %s block (allowed: %s)", i, j, b.Type, strings.Join(topicBodyTypes, ", "))
			}
			tp.Body = append(tp.Body, b)
		}
		out.Topics = append(out.Topics, tp)
	}
	if labeled > 0 && labeled < len(out.Topics) {
		for i, tp := range out.Topics {
			if tp.Label == "" {
				return nil, fmt.Errorf("topics[%d].label: missing (give every topic a label, or none: %d of %d topics have one)", i, labeled, len(out.Topics))
			}
		}
	}
	return out, nil
}

// topicHead は印つきの題(印を題の頭に書く。印が無ければ題そのもの)。
func topicHead(tp irTopic) Inline {
	if tp.Label == "" {
		return tp.Title
	}
	head := Inline{{Text: tp.Label + " "}}
	if len(tp.Title) > 0 && tp.Title[0].Type == "" {
		// 題の頭がただの文字列なら、1つの文字列にまとめる(同じ文を同じ塊で書く)。
		head[0].Text += tp.Title[0].Text
		return append(head, tp.Title[1:]...)
	}
	return append(head, tp.Title...)
}

// topicsFallback は代わりの組み方:1つの箇条。塊の題(印があれば頭に)を親の項目に、
// 中身を子の項目にする。段落はその文の項目、箇条はその項目(入れ子のまま)、引用は
// 言葉の項目と出どころの項目。場所の表は、親の項目を塊(/topics/K)、その文を題に写す。
func topicsFallback(b *IRBlock) ([]*IRBlock, fallbackPaths, error) {
	p, err := readTopicsMin(b.Raw, b.minParts(topicsMin))
	if err != nil {
		return nil, nil, err
	}
	paths := fallbackPaths{}
	var items []IRPointItem
	for k, tp := range p.Topics {
		var kids []IRPointItem
		ordered := false
		for _, blk := range tp.Body {
			switch blk.Type {
			case "paragraph":
				kids = append(kids, IRPointItem{Content: blk.Content})
			case "points":
				kids = append(kids, blk.Items...)
				ordered = len(tp.Body) == 1 && blk.Ordered
			case "quote":
				kids = append(kids, IRPointItem{Content: blk.Content})
				if len(blk.Source) > 0 {
					kids = append(kids, IRPointItem{Content: blk.Source})
				}
			}
		}
		items = append(items, IRPointItem{Content: topicHead(tp), Items: kids, Ordered: ordered})
		paths[fmt.Sprintf("/items/%d", k)] = fallbackPath{To: fmt.Sprintf("/topics/%d", k)}
		paths[fmt.Sprintf("/items/%d/content", k)] = fallbackPath{To: fmt.Sprintf("/topics/%d/title", k)}
	}
	return []*IRBlock{{Type: "points", Variant: p.Variant, Items: items}}, paths, nil
}

// topicsTypeDecl は topics が語彙に名乗るもの。
func topicsTypeDecl() typeDecl {
	schema, err := typeSchema(topicsType)
	if err != nil {
		panic(err)
	}
	return typeDecl{
		Schema: schema,
		Markdown: []BlockDecl{
			{Name: "topic", Label: "塊の題(必須。観点の短い文)", LabelRequired: true, Inside: []string{"topics"},
				Contains: []string{"段落", "箇条", ":::quote"}, Attributes: []string{"label"},
				Meaning: "印つきの塊の1つ。題と中身(1〜3個のブロック。引用を入れるときは囲みのコロンを1つ増やす)。{label=A} は塊の短い印(空白を含まない2字まで)で、全ての塊に付けるか全てに付けない",
				IR:      "topics.topics[](title・label・body)"},
			{Name: "topics", Contains: []string{"topic"}, MinChildren: topicsMin, Attributes: []string{"variant"},
				Meaning: "印つきの塊の並び(標準でない型)。いくつかの観点を、それぞれ題と説明で上から順に並べる(2〜6個)。印の形・塊の間の罫線はテンプレートが決める。組み方を持たないテンプレートは箇条(題を親、中身を子)で組む",
				IR:      "topics"},
		},
		Fallback: topicsFallback,
		Validate: func(b *IRBlock) error {
			_, err := readTopics(b.Raw)
			return err
		},
		readMarkdown: (*mdParser).topicsBlock,
		Inner:        topicsInner,
	}
}

// topicsInner は check に渡す topics の中:各塊の題(見える文字が要る)と中のブロック
// (本文のブロックと同じ検査。段落は見える文字が要る)。
func topicsInner(m map[string]any) []innerPart {
	var out []innerPart
	topics, _ := m["topics"].([]any)
	for k, x := range topics {
		tm, ok := x.(map[string]any)
		if !ok {
			continue
		}
		tp := fmt.Sprintf("/topics/%d", k)
		out = append(out, innerPart{Path: tp, Map: tm, Texts: []string{"title"}})
		body, _ := tm["body"].([]any)
		for i, b := range body {
			bm, ok := b.(map[string]any)
			if !ok {
				continue
			}
			part := innerPart{Path: fmt.Sprintf("%s/body/%d", tp, i), Map: bm, Block: true}
			if bm["type"] == "paragraph" {
				part.Texts = []string{"content"}
			}
			out = append(out, part)
		}
	}
	return out
}

// --- 組み方(同梱のテンプレート) ---

// topicsInfo は topics の箱の中の部品(はみ出しを塊ごとに測る・layout の場所を決める)。
type topicsInfo struct {
	rows []topicRow
}

// topicRow は1つの塊の箱と、その中の題・中身の箱。
type topicRow struct {
	row, title *Spec
	body       []*Spec
}

// topicsStyle は topics の組み方の見た目の差し替え口(report.go)。零値は標準の組み方
// (題は字の色の太字で、印を題の頭に書く)。
type topicsStyle struct {
	// titleRole は題の色の役割名("" なら text_primary)。
	titleRole string
	// mark は塊の印の箱。左に置き、印は題の頭に書かない(nil なら題の頭に書く)。
	mark func(t *Template, label string) *Spec
	// rule は塊の間の罫線の箱(nil なら引かない)。
	rule func(t *Template) *Spec
	// rowGap は塊の間(罫線を引くなら罫線の上と下それぞれ)の空きのトークン
	// ("" なら blockGap)。
	rowGap string
}

// topicsComponent は topics の標準の組み方:塊を上から積み、塊の中は題の下に中身を積む。
func topicsComponent(ctx componentContext, b *IRBlock) (*Spec, error) {
	return topicsStyled(ctx, b, topicsStyle{})
}

// topicsStyled は topicsComponent の見た目を st で差し替えたもの。
func topicsStyled(ctx componentContext, b *IRBlock, st topicsStyle) (*Spec, error) {
	t := ctx.t
	p, err := readTopicsMin(b.Raw, b.minParts(topicsMin))
	if err != nil {
		return nil, err
	}
	role := st.titleRole
	if role == "" {
		role = "text_primary"
	}
	info := &topicsInfo{}
	var kids []*Spec
	for k, tp := range p.Topics {
		var tr topicRow
		head := topicHead(tp)
		if st.mark != nil {
			head = tp.Title
		}
		tr.title = apply(t.text(head, "body", role, true), []SpecOpt{noShrink()})
		for i := range tp.Body {
			where := fmt.Sprintf("topics[%d]: body[%d]", k, i)
			s, err := t.buildBlockIn(ctx.within(where), &tp.Body[i])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", where, err)
			}
			tr.body = append(tr.body, apply(s, []SpecOpt{noShrink()}))
		}
		content := Col([]*Spec{tr.title, Col(tr.body, tokGap("blockGap"), noShrink())}, tokGap("itemGap"), noShrink())
		tr.row = content
		if st.mark != nil && tp.Label != "" {
			content = apply(content, []SpecOpt{WithGrow(1), WithBasis(0)})
			tr.row = Row([]*Spec{st.mark(t, tp.Label), content}, tokGap("columnGap"))
		}
		tr.row = apply(tr.row, []SpecOpt{noShrink()})
		if k > 0 && st.rule != nil {
			kids = append(kids, apply(st.rule(t), []SpecOpt{noShrink()}))
		}
		info.rows = append(info.rows, tr)
		kids = append(kids, tr.row)
	}
	rowGap := st.rowGap
	if rowGap == "" {
		rowGap = "blockGap"
	}
	return Col(kids, tokGap(rowGap), measureTopicRows(info)), nil
}

// registerTopicsComponent は同梱のテンプレートの topics の組み方を r に足す
// (テンプレートが自分の組み方をすでに登録したものはそのまま)。
func registerTopicsComponent(r *componentRegistry) {
	if !r.has(topicsType) {
		must(r.register(topicsType, componentFunc(topicsComponent)))
	}
}

// --- はみ出しと容量 ---

// topicMeasure は塊の1つの測り値。Top・Bottom は本文の領域の上端から。
type topicMeasure struct {
	Index    int
	Need     float64
	Lines    int
	Top      float64
	Bottom   float64
	Overflow float64
}

// measureTopics は塊を1つずつ縮めずに測る(top はブロックの上端、width はブロックの
// 中の幅)。描いた後に呼ぶこと。
func measureTopics(block *Spec, info *topicsInfo, top, width, capacity float64) []topicMeasure {
	index := map[*Spec]int{}
	for k, r := range info.rows {
		index[r.row] = k
	}
	cursor := top + block.padY()
	var out []topicMeasure
	for i, c := range block.Children {
		if i > 0 {
			cursor += block.Gap
		}
		h, n := requiredMeasure(c, width)
		if k, ok := index[c]; ok {
			m := topicMeasure{Index: k, Need: h, Lines: n, Top: cursor, Bottom: cursor + h}
			m.Overflow = overflowOf(m.Top, m.Bottom, capacity)
			out = append(out, m)
		}
		cursor += h
	}
	return out
}

// TopicCapacity は容量の中の塊の1つ(Top・Bottom は本文の領域の上端から)。
type TopicCapacity struct {
	Line     int     `json:"line,omitempty"`
	Path     string  `json:"path"` // /slides/S/body/N/topics/K
	Need     float64 `json:"need"`
	Lines    int     `json:"lines"`
	Top      float64 `json:"top"`
	Bottom   float64 `json:"bottom"`
	Overflow float64 `json:"overflow"`
}

func topicCapacity(m topicMeasure, path string) TopicCapacity {
	return TopicCapacity{Path: fmt.Sprintf("%s/topics/%d", path, m.Index), Need: round1(m.Need), Lines: m.Lines,
		Top: round1(m.Top), Bottom: round1(m.Bottom), Overflow: round1(m.Overflow)}
}

// topicOverflowWarnings ははみ出した塊ごとの警告の文。1つも無ければ nil。
func topicOverflowWarnings(o BlockMeasure, capacity, need float64) []string {
	var out []string
	for _, m := range o.Topics {
		if m.Overflow <= 0 {
			continue
		}
		out = append(out, fmt.Sprintf("[LAYOUT-OVERFLOW WARNING] body[%d]: topics[%d]: topic %d (topics) overflows the body area by %.1fpt (topic needs %.1fpt; body area %.1fpt, all blocks need %.1fpt)",
			o.Index, m.Index, m.Index, m.Overflow, m.Need, capacity, need))
	}
	return out
}

// --- Markdown ---

// topicsBlock は ::::topics … :::: を IR の topics にする。中には :::topic 題 {label=A} を
// 2〜6個書き、中身は段落・箇条・:::quote の1〜3個。
func (p *mdParser) topicsBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	p.noLabel(f, path)
	ok := true
	var topics []any
	var labeled, unlabeled []int // 印のある塊・無い塊の行
	for _, it := range f.items {
		if it.kind != mdFence || it.fence.name != "topic" {
			p.fail(it.line, 1, path, codeMisplaced, "::::topics の中には :::topic だけを書けます", []string{"topic"}, "")
			ok = false
			continue
		}
		tf := it.fence
		tp := fmt.Sprintf("%s/topics/%d", path, len(topics))
		p.mark(tp, tf.line)
		ta := p.fenceAttrs(tf, tp, []string{"label"})
		if len(tf.label.r) == 0 {
			p.fail(tf.line, 1, tp+"/title", codeMissingKey, ":::topic には塊の題が要ります", nil, "「:::topic 題の文」のように、囲みの名前の後ろに書いてください")
			ok = false
			topics = append(topics, nil)
			continue
		}
		topic := obj("title", p.parseInline(tf.label, tp+"/title"))
		if l, has := ta["label"]; has {
			if err := checkTopicLabel(l); err != nil {
				p.fail(tf.line, tf.attrsCol, tp+"/label", codeInvalidValue, fmt.Sprintf("塊の印 %q は書けません(空白を含まない2字まで)", l), nil, "{label=A} や {label=1} のように短く書きます")
				ok = false
			}
			topic.set("label", l)
			labeled = append(labeled, tf.line)
		} else {
			unlabeled = append(unlabeled, tf.line)
		}
		body := p.topicBody(tf, tp+"/body")
		if body == nil {
			ok = false
		} else {
			topic.set("body", body)
		}
		topics = append(topics, topic)
	}
	if len(labeled) > 0 && len(unlabeled) > 0 {
		p.fail(unlabeled[0], 1, path+"/topics", codeInvalidValue,
			fmt.Sprintf("塊の印(label)は全ての :::topic に付けるか、全てに付けません(いま %d 個中 %d 個に付いています)", len(labeled)+len(unlabeled), len(labeled)), nil,
			"印の無い :::topic に {label=…} を足すか、全ての印を外してください")
		ok = false
	}
	if n := len(topics); ok && (n < topicsMin || n > topicsMax) {
		p.fail(f.line, 1, path+"/topics", codeMisplaced, fmt.Sprintf("::::topics には :::topic を %d〜%d 個書きます(いま %d 個)", topicsMin, topicsMax, n), nil,
			"塊が1つなら段落と箇条に、7つ以上ならスライドを分けてください")
		return nil
	}
	if !ok {
		return nil
	}
	b := obj("type", topicsType)
	if v, has := attrs["variant"]; has {
		b.set("variant", v)
	}
	b.set("topics", topics)
	return b
}

// topicBody は :::topic の中を IR の body(段落・箇条・:::quote の1〜3個)にする。
// 誤りがあれば nil。
func (p *mdParser) topicBody(f *mdFenceBlk, path string) []any {
	var allowed []*mdItem
	bad := false
	for _, it := range f.items {
		if it.kind == mdPara || it.kind == mdList || (it.kind == mdFence && it.fence.name == "quote") {
			if it.kind == mdPara {
				if _, img := scanImages(it.text); img {
					p.fail(it.line, 1, path, codeMisplaced, ":::topic の中に画像は置けません", []string{"段落", "箇条", "quote"}, "")
					bad = true
					continue
				}
			}
			allowed = append(allowed, it)
			continue
		}
		what := "コードブロック"
		switch {
		case it.kind == mdFence:
			what = ":::" + it.fence.name
		case it.kind == mdTable:
			what = "表"
		}
		p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf(":::topic の中に %s は書けません", what), []string{"段落", "箇条", "quote"}, "")
		bad = true
	}
	if len(f.items) == 0 {
		p.fail(f.line, 1, path, codeMisplaced, ":::topic の中が空です", nil, "題の説明を段落か箇条で書いてください")
		return nil
	}
	if len(allowed) > topicBodyMax {
		p.fail(allowed[topicBodyMax].line, 1, path, codeMisplaced,
			fmt.Sprintf(":::topic の中にはブロックを %d 個まで書けます(いま %d 個)", topicBodyMax, len(allowed)), nil,
			"文を足すなら同じ段落か箇条に入れるか、塊を分けてください")
		bad = true
	}
	if bad {
		return nil
	}
	body := p.blocks(allowed, path)
	if len(body) != len(allowed) {
		return nil // 中のブロックの誤りは p.blocks が場所つきで返した
	}
	return body
}
