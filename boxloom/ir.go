// IR(意味の中間表現)を読む。形の正本は schema/ と docs/ir.md。
//
// ここでは Schema と同じく「知らないキーは断る」で読む(DisallowUnknownFields)。
// 断るときは IR の中の場所(slides[2].body[0] のような形)を添える。
// Schema の全部(文字列の形・enum など)をここで繰り返し検査はしない。細かい
// 検査は tools/validate_ir.py(Schema)と、これから作る boxloom check の仕事。
// ここで見るのは、組み立てるのに要ること(必須の欄・種類の名前)だけ。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IRDeck は文書の根。
type IRDeck struct {
	Schema   string    `json:"$schema,omitempty"`
	Boxloom  string    `json:"boxloom"`
	Template string    `json:"template,omitempty"`
	Meta     *IRMeta   `json:"meta,omitempty"`
	Slides   []IRSlide `json:"slides"`
}

type IRMeta struct {
	Title    Inline `json:"title"`
	Subtitle Inline `json:"subtitle,omitempty"`
	Author   Inline `json:"author,omitempty"`
	Date     string `json:"date,omitempty"`
	// Confidential は機密区分の短い文(表紙とフッター)。
	Confidential Inline `json:"confidential,omitempty"`
	// Disclaimer は文書全体の免責・但し書き(表紙に1回)。
	Disclaimer Inline `json:"disclaimer,omitempty"`
}

// IRSlide はスライド。Type で cover / content / section / statement を選ぶ。
type IRSlide struct {
	Type     string
	Variant  string
	Title    Inline // cover・content・section(章の題)
	Subtitle Inline // cover だけ
	Author   Inline // cover だけ
	Content  Inline // statement だけ(言いたいことの1文)
	Source   Inline // content・statement: 出典(本文の流れの外。本文の下・フッターの上)
	Lead     Inline // content だけ: リード文(題の下に置く、このスライドで言いたいことの文)
	Body     []IRBlock
	Notes    string
}

// IRBlock は本文のブロック。Type で種類を選び、種類ごとに使う欄が違う。
type IRBlock struct {
	Type    string
	Variant string

	Content Inline            // paragraph / note / takeaway / quote
	Source  Inline            // quote: 出どころ
	Items   []IRPointItem     // points
	Options []IRCompareOption // compare
	Stats   []IRStatItem      // stat

	Subsystem string          // subsystem: 名前(例 "gantt")
	Data      json.RawMessage // subsystem: 中身(サブシステムが読む)

	Columns []IRColumn // columns: 並列のまとまり(2〜3個)

	Header  []Inline   // table: 見出しの行
	Rows    [][]Inline // table: 本体の行(どの行もセルの数は見出しと同じ)
	Total   []Inline   // table: 合計の行(任意。セルの数は見出しと同じ)
	Caption Inline     // table・image: 説明の文(任意)

	Src    string    // image: 文書からの相対パス
	Alt    string    // image: 代わりの文
	Images []IRImage // gallery: 2〜6枚

	// Raw は標準でない型(型の語彙。typevocab.go)のブロックの JSON そのもの。
	// 組み方と代わりの組み方(Fallback)がこれを読む。標準の型では nil。
	Raw json.RawMessage
}

// IRColumn は並列のまとまりの1つ。Role は "" か "main"(主のまとまり)。
type IRColumn struct {
	Title Inline    `json:"title,omitempty"`
	Role  string    `json:"role,omitempty"`
	Body  []IRBlock `json:"-"`
}

// IRImage は画像1枚(gallery の1枚)。
type IRImage struct {
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	Caption Inline `json:"caption,omitempty"`
}

type IRPointItem struct {
	Content Inline        `json:"content"`
	Items   []IRPointItem `json:"items,omitempty"`
}

// IRStatItem は stat の1つの数。Value は書いたとおりの文字(計算はしない)。
type IRStatItem struct {
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
	Label Inline `json:"label"`
}

// statMaxItems は stat に置ける数の上限(Schema の maxItems と同じ)。
const statMaxItems = 4

type IRCompareOption struct {
	Label Inline   `json:"label"`
	Pros  []Inline `json:"pros,omitempty"`
	Cons  []Inline `json:"cons,omitempty"`
}

// --- インライン ---

// Inline は文中の表現。文字列か、文字列とインライン要素の並び。
type Inline []InlinePart

// InlinePart は1つの部分。Type が "" ならただの文字列(Text)。
type InlinePart struct {
	Type    string // "" | em | code | link | br
	Text    string // ただの文字列・code の中身
	Href    string // link
	Content Inline // em・link の中身
}

func (in *Inline) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*in = Inline{{Text: s}}
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("inline must be a string or an array: %w", err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("inline array must not be empty")
	}
	out := make(Inline, 0, len(raw))
	for i, r := range raw {
		p, err := parseInlinePart(r)
		if err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
		out = append(out, p)
	}
	*in = out
	return nil
}

func parseInlinePart(r json.RawMessage) (InlinePart, error) {
	r = bytes.TrimSpace(r)
	if len(r) > 0 && r[0] == '"' {
		var s string
		err := json.Unmarshal(r, &s)
		return InlinePart{Text: s}, err
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(r, &head); err != nil {
		return InlinePart{}, err
	}
	switch head.Type {
	case "em":
		var v struct {
			Type    string `json:"type"`
			Content Inline `json:"content"`
		}
		if err := strictUnmarshal(r, &v); err != nil {
			return InlinePart{}, err
		}
		return InlinePart{Type: "em", Content: v.Content}, nil
	case "code":
		var v struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := strictUnmarshal(r, &v); err != nil {
			return InlinePart{}, err
		}
		return InlinePart{Type: "code", Text: v.Text}, nil
	case "link":
		var v struct {
			Type    string `json:"type"`
			Href    string `json:"href"`
			Content Inline `json:"content"`
		}
		if err := strictUnmarshal(r, &v); err != nil {
			return InlinePart{}, err
		}
		return InlinePart{Type: "link", Href: v.Href, Content: v.Content}, nil
	case "br":
		var v struct {
			Type string `json:"type"`
		}
		if err := strictUnmarshal(r, &v); err != nil {
			return InlinePart{}, err
		}
		return InlinePart{Type: "br"}, nil
	}
	return InlinePart{}, fmt.Errorf("unknown inline type %q (known: em, code, link, br)", head.Type)
}

// Plain は飾りを外した文字列。br は "\n" になる。
func (in Inline) Plain() string {
	var b strings.Builder
	for _, p := range in {
		switch p.Type {
		case "":
			b.WriteString(p.Text)
		case "code":
			b.WriteString(p.Text)
		case "br":
			b.WriteString("\n")
		default:
			b.WriteString(p.Content.Plain())
		}
	}
	return b.String()
}

// Lines は行の配列。1行なら文字列でよい。
type Lines []string

func (l *Lines) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = Lines{s}
		return nil
	}
	var a []string
	if err := json.Unmarshal(b, &a); err != nil {
		return fmt.Errorf("lines must be a string or an array of strings: %w", err)
	}
	*l = a
	return nil
}

// --- 箇条の1項目: インラインか {content, items} ---

func (p *IRPointItem) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		type plain IRPointItem
		var v plain
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Content == nil {
			return fmt.Errorf("point item object needs \"content\"")
		}
		*p = IRPointItem(v)
		return nil
	}
	var in Inline
	if err := in.UnmarshalJSON(b); err != nil {
		return err
	}
	*p = IRPointItem{Content: in}
	return nil
}

func (o *IRCompareOption) UnmarshalJSON(b []byte) error {
	type plain IRCompareOption
	var v plain
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	if v.Label == nil {
		return fmt.Errorf("compare option needs \"label\"")
	}
	if len(v.Pros) == 0 && len(v.Cons) == 0 {
		return fmt.Errorf("compare option needs \"pros\" or \"cons\"")
	}
	*o = IRCompareOption(v)
	return nil
}

func (it *IRStatItem) UnmarshalJSON(b []byte) error {
	type plain IRStatItem
	var v plain
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	if v.Value == "" || v.Label == nil {
		return fmt.Errorf("stat item needs \"value\" and \"label\"")
	}
	if !strings.ContainsAny(v.Value, "0123456789０１２３４５６７８９") {
		return fmt.Errorf("stat item value %q has no digit (a stat shows a number)", v.Value)
	}
	*it = IRStatItem(v)
	return nil
}

func (m *IRMeta) UnmarshalJSON(b []byte) error {
	type plain IRMeta
	var v plain
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	*m = IRMeta(v)
	return nil
}

// --- スライドとブロック: type を先に見て、種類ごとの形で読み直す ---

func (s *IRSlide) UnmarshalJSON(b []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	switch head.Type {
	case "cover":
		var v struct {
			Type     string `json:"type"`
			Variant  string `json:"variant"`
			Title    Inline `json:"title"`
			Subtitle Inline `json:"subtitle"`
			Author   Inline `json:"author"`
			Notes    string `json:"notes"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		*s = IRSlide{Type: v.Type, Variant: v.Variant, Title: v.Title, Subtitle: v.Subtitle, Author: v.Author, Notes: v.Notes}
		return nil
	case "content":
		var v struct {
			Type    string            `json:"type"`
			Variant string            `json:"variant"`
			Title   Inline            `json:"title"`
			Lead    Inline            `json:"lead"`
			Body    []json.RawMessage `json:"body"`
			Source  Inline            `json:"source"`
			Notes   string            `json:"notes"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Title == nil {
			return fmt.Errorf("content slide needs \"title\"")
		}
		if v.Body == nil {
			return fmt.Errorf("content slide needs \"body\"")
		}
		out := IRSlide{Type: v.Type, Variant: v.Variant, Title: v.Title, Lead: v.Lead, Source: v.Source, Notes: v.Notes}
		for i, r := range v.Body {
			var blk IRBlock
			if err := blk.UnmarshalJSON(r); err != nil {
				return fmt.Errorf("body[%d]: %w", i, err)
			}
			out.Body = append(out.Body, blk)
		}
		*s = out
		return nil
	case "section":
		var v struct {
			Type    string `json:"type"`
			Variant string `json:"variant"`
			Title   Inline `json:"title"`
			Notes   string `json:"notes"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Title == nil {
			return fmt.Errorf("section slide needs \"title\"")
		}
		*s = IRSlide{Type: v.Type, Variant: v.Variant, Title: v.Title, Notes: v.Notes}
		return nil
	case "statement":
		var v struct {
			Type    string `json:"type"`
			Variant string `json:"variant"`
			Content Inline `json:"content"`
			Source  Inline `json:"source"`
			Notes   string `json:"notes"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Content == nil {
			return fmt.Errorf("statement slide needs \"content\"")
		}
		*s = IRSlide{Type: v.Type, Variant: v.Variant, Content: v.Content, Source: v.Source, Notes: v.Notes}
		return nil
	}
	return fmt.Errorf("unknown slide type %q (known: cover, content, section, statement)", head.Type)
}

func (k *IRBlock) UnmarshalJSON(b []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	switch head.Type {
	case "paragraph", "note", "takeaway":
		var v struct {
			Type    string `json:"type"`
			Variant string `json:"variant"`
			Content Inline `json:"content"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Content == nil {
			return fmt.Errorf("%s needs \"content\"", head.Type)
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Content: v.Content}
	case "points":
		var v struct {
			Type    string        `json:"type"`
			Variant string        `json:"variant"`
			Items   []IRPointItem `json:"items"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Items) == 0 {
			return fmt.Errorf("points needs at least one item")
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Items: v.Items}
	case "compare":
		var v struct {
			Type    string            `json:"type"`
			Variant string            `json:"variant"`
			Options []IRCompareOption `json:"options"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Options) < 2 {
			return fmt.Errorf("compare needs at least two options")
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Options: v.Options}
	case "stat":
		var v struct {
			Type    string       `json:"type"`
			Variant string       `json:"variant"`
			Items   []IRStatItem `json:"items"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Items) == 0 || len(v.Items) > statMaxItems {
			return fmt.Errorf("stat needs 1 to %d items (got %d)", statMaxItems, len(v.Items))
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Stats: v.Items}
	case "quote":
		var v struct {
			Type    string `json:"type"`
			Variant string `json:"variant"`
			Content Inline `json:"content"`
			Source  Inline `json:"source"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Content == nil || v.Source == nil {
			return fmt.Errorf("quote needs \"content\" and \"source\" (other people's words need their source)")
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Content: v.Content, Source: v.Source}
	case "subsystem":
		var v struct {
			Type      string          `json:"type"`
			Subsystem string          `json:"subsystem"`
			Variant   string          `json:"variant"`
			Data      json.RawMessage `json:"data"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Subsystem == "" || len(v.Data) == 0 {
			return fmt.Errorf("subsystem block needs \"subsystem\" and \"data\"")
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Subsystem: v.Subsystem, Data: v.Data}
	case "columns":
		var v struct {
			Type    string            `json:"type"`
			Variant string            `json:"variant"`
			Columns []json.RawMessage `json:"columns"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Columns) < 2 || len(v.Columns) > 3 {
			return fmt.Errorf("columns needs two or three columns (got %d)", len(v.Columns))
		}
		out := IRBlock{Type: v.Type, Variant: v.Variant}
		mains := 0
		for i, r := range v.Columns {
			var c IRColumn
			if err := c.UnmarshalJSON(r); err != nil {
				return fmt.Errorf("columns[%d]: %w", i, err)
			}
			if c.Role == "main" {
				mains++
			}
			out.Columns = append(out.Columns, c)
		}
		if mains > 1 {
			return fmt.Errorf("columns: at most one column can have role \"main\" (got %d)", mains)
		}
		*k = out
	case "table":
		var v struct {
			Type    string     `json:"type"`
			Variant string     `json:"variant"`
			Caption Inline     `json:"caption"`
			Header  []Inline   `json:"header"`
			Rows    [][]Inline `json:"rows"`
			Total   []Inline   `json:"total"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Header) == 0 || len(v.Rows) == 0 {
			return fmt.Errorf("table needs \"header\" and at least one row in \"rows\"")
		}
		for i, r := range v.Rows {
			if len(r) != len(v.Header) {
				return fmt.Errorf("rows[%d]: row has %d cells but the header has %d", i, len(r), len(v.Header))
			}
		}
		if v.Total != nil && len(v.Total) != len(v.Header) {
			return fmt.Errorf("total: row has %d cells but the header has %d", len(v.Total), len(v.Header))
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Caption: v.Caption, Header: v.Header, Rows: v.Rows, Total: v.Total}
	case "image":
		var v struct {
			Type    string `json:"type"`
			Variant string `json:"variant"`
			IRImage
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if err := v.IRImage.validate(); err != nil {
			return err
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Src: v.Src, Alt: v.Alt, Caption: v.Caption}
	case "gallery":
		var v struct {
			Type    string    `json:"type"`
			Variant string    `json:"variant"`
			Images  []IRImage `json:"images"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Images) < 2 || len(v.Images) > 6 {
			return fmt.Errorf("gallery needs two to six images (got %d)", len(v.Images))
		}
		for i := range v.Images {
			if err := v.Images[i].validate(); err != nil {
				return fmt.Errorf("images[%d]: %w", i, err)
			}
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Images: v.Images}
	default:
		if _, ok := blockTypes.decl(head.Type); ok && !blockTypes.isStandard(head.Type) {
			// 標準でない型:形の検査は Schema(schema/types/)の仕事。Go は variant だけ
			// 読み、中身は組み方と代わりの組み方に渡す。
			var v struct {
				Variant string `json:"variant"`
			}
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			*k = IRBlock{Type: head.Type, Variant: v.Variant, Raw: append(json.RawMessage{}, b...)}
			if d, _ := blockTypes.decl(head.Type); d.Validate != nil {
				return d.Validate(k)
			}
			return nil
		}
		return fmt.Errorf("unknown block type %q (known: %s)", head.Type, strings.Join(blockTypes.known(), ", "))
	}
	return nil
}

// columnBlockTypes はまとまり(column)の中に置けるブロックの種類。サブシステムは
// columnSubsystems のものだけ(グラフの横に読み取りを置く形)。
var columnBlockTypes = []string{"paragraph", "points", "note", "table", "image", "subsystem"}

// columnSubsystems はまとまりの中に置けるサブシステム。
var columnSubsystems = []string{"chart"}

func (c *IRColumn) UnmarshalJSON(b []byte) error {
	var v struct {
		Title Inline            `json:"title"`
		Role  string            `json:"role"`
		Body  []json.RawMessage `json:"body"`
	}
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	if v.Role != "" && v.Role != "main" {
		return fmt.Errorf("role: unknown role %q (known: main)", v.Role)
	}
	if len(v.Body) == 0 {
		return fmt.Errorf("column needs at least one block in \"body\"")
	}
	out := IRColumn{Title: v.Title, Role: v.Role}
	for i, r := range v.Body {
		var blk IRBlock
		if err := blk.UnmarshalJSON(r); err != nil {
			return fmt.Errorf("body[%d]: %w", i, err)
		}
		if !contains(columnBlockTypes, blk.Type) {
			return fmt.Errorf("body[%d]: a column cannot hold a %s block (allowed: %s)", i, blk.Type, strings.Join(columnBlockTypes, ", "))
		}
		if blk.Type == "subsystem" && !contains(columnSubsystems, blk.Subsystem) {
			return fmt.Errorf("body[%d]: a column cannot hold the %s subsystem (allowed: %s)", i, blk.Subsystem, strings.Join(columnSubsystems, ", "))
		}
		out.Body = append(out.Body, blk)
	}
	*c = out
	return nil
}

// validate は画像の欄の形を見る(ファイルがあるか・形式は check と組み立てが見る)。
func (m IRImage) validate() error {
	if m.Src == "" {
		return fmt.Errorf("image needs \"src\"")
	}
	if strings.TrimSpace(m.Alt) == "" {
		return fmt.Errorf("image needs a non-empty \"alt\" (the text read aloud and shown when the image is missing)")
	}
	if err := checkImageSrc(m.Src); err != nil {
		return fmt.Errorf("src: %w", err)
	}
	return nil
}

// --- 読み込みの入口 ---

// ReadIR は IR(JSON)を読む。知らないキー・足りない欄は断る。誤りは
// 「file: 場所: 文」の形(file が空なら場所から)。
func ReadIR(file string, data []byte) (*IRDeck, error) {
	d, err := ParseIR(data)
	if err != nil {
		if file == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return d, nil
}

// ParseIR は IR の JSON を読む。
func ParseIR(b []byte) (*IRDeck, error) {
	var root struct {
		Schema   string            `json:"$schema"`
		Boxloom  string            `json:"boxloom"`
		Template string            `json:"template"`
		Meta     *IRMeta           `json:"meta"`
		Slides   []json.RawMessage `json:"slides"`
	}
	if err := strictUnmarshal(b, &root); err != nil {
		return nil, err
	}
	if root.Boxloom != "0.1" {
		return nil, fmt.Errorf("boxloom: unsupported IR version %q (want \"0.1\")", root.Boxloom)
	}
	if len(root.Slides) == 0 {
		return nil, fmt.Errorf("slides: at least one slide is required")
	}
	d := &IRDeck{Schema: root.Schema, Boxloom: root.Boxloom, Template: root.Template, Meta: root.Meta}
	for i, r := range root.Slides {
		var s IRSlide
		if err := s.UnmarshalJSON(r); err != nil {
			return nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
		d.Slides = append(d.Slides, s)
	}
	return d, nil
}

// strictUnmarshal は知らないキーを断って読む。
func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after the value")
	}
	return nil
}

// --- 日付 ---

// parseIRDate は YYYY-MM か YYYY-MM-DD を読む(YYYY-MM はその月の1日)。
func parseIRDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("bad date %q (want YYYY-MM or YYYY-MM-DD)", s)
}
