// IR(意味の中間表現)を読む。形の正本は schema/ と docs/ir.md。
//
// ここでは Schema と同じく「知らないキーは断る」で読む(DisallowUnknownFields)。
// 断るときは IR の中の場所(slides[2].body[0] のような形)を添える。
// Schema の全部(文字列の形・enum など)をここで繰り返し検査はしない。細かい
// 検査は tools/validate_ir.py(Schema)と、これから作る boxloom check の仕事。
// ここで見るのは、組み立てるのに要ること(必須の欄・種類の名前)だけ。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
}

// IRSlide はスライド。Type で cover / content を選ぶ。
type IRSlide struct {
	Type     string
	Variant  string
	Title    Inline
	Subtitle Inline // cover だけ
	Author   Inline // cover だけ
	Body     []IRBlock
	Notes    string
}

// IRBlock は本文のブロック。Type で種類を選び、種類ごとに使う欄が違う。
type IRBlock struct {
	Type    string
	Variant string

	Content Inline            // paragraph / note
	Items   []IRPointItem     // points
	Options []IRCompareOption // compare

	Subsystem string          // subsystem: 名前(例 "gantt")
	Data      json.RawMessage // subsystem: 中身(サブシステムが読む)
}

type IRPointItem struct {
	Content Inline        `json:"content"`
	Items   []IRPointItem `json:"items,omitempty"`
}

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
			Body    []json.RawMessage `json:"body"`
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
		out := IRSlide{Type: v.Type, Variant: v.Variant, Title: v.Title, Notes: v.Notes}
		for i, r := range v.Body {
			var blk IRBlock
			if err := blk.UnmarshalJSON(r); err != nil {
				return fmt.Errorf("body[%d]: %w", i, err)
			}
			out.Body = append(out.Body, blk)
		}
		*s = out
		return nil
	}
	return fmt.Errorf("unknown slide type %q (known: cover, content)", head.Type)
}

func (k *IRBlock) UnmarshalJSON(b []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	switch head.Type {
	case "paragraph", "note":
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
	default:
		return fmt.Errorf("unknown block type %q (known: paragraph, points, compare, note, subsystem)", head.Type)
	}
	return nil
}

// --- 読み込みの入口 ---

// ReadIR はファイルから IR を読む。
func ReadIR(path string) (*IRDeck, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := ParseIR(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
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
