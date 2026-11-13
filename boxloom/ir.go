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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
	// Toc は表紙の直後に目次のスライドを置くか(既定 false)。目次のスライドは IR に
	// 書かず、組み立てで作る(level 1・2 の区切りと、組み立てたあとの頁番号)。
	Toc bool `json:"toc,omitempty"`
	// CoverImage は表紙の写真(飾り。代わりの文は持たない)。文書からの相対パスで、
	// 決まりは画像の src と同じ。テンプレートが表紙(と終わりのスライド)の全面に敷く。
	CoverImage string `json:"coverImage,omitempty"`
	// Closing は最後に終わりのスライドを置くか(既定 false)と、置くならその箱の中の文
	// (IR は true / false か文。文を書けば置く)。終わりのスライドは IR に書かず、
	// 組み立てで作る(closing.go)。
	Closing IRClosing `json:"closing,omitempty"`
}

// IRClosing は文書の頭の closing:真偽か文(#104)。文を書けば終わりのスライドを置き、
// その文をテンプレートが終わりのスライドに置く。
type IRClosing struct {
	On   bool   // 終わりのスライドを置くか
	Text Inline // 終わりのスライドの文(無ければ nil)
}

func (c *IRClosing) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch string(b) {
	case "true", "false":
		*c = IRClosing{On: string(b) == "true"}
		return nil
	}
	var in Inline
	if err := in.UnmarshalJSON(b); err != nil {
		return fmt.Errorf("closing must be true, false or an inline text: %w", err)
	}
	*c = IRClosing{On: true, Text: in}
	return nil
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
	// Marker は content だけ: スライドの隅の印(空白を含まない2字までの短い文。1・2・A・B)。
	// いまどのパターン・どの要因の話かを示す意味。描くかどうか・形はテンプレートが決める。
	Marker string
	// Density は content だけ: 本文の密度(""(normal と同じ)か "dense")。dense は
	// 「このページは参照用に詰めて読ませる」という意味で、本文の字の大きさ・間隔を
	// テンプレートの値の表の dense の組にする(題・リード・フッターは変えない)。
	Density string
	// Level は section だけ: 区切りの階層(1 章・2 節・3 節の中の項目)。書かなければ 0
	// (1 と同じ。sectionLevel で読む)。
	Level int
	Body  []IRBlock
	Notes string
	// fitShrink は IR に無い、組み立ての中だけの印(-fit。fit.go の fitDeck が写しに入れる):
	// 0 なら印なし、k なら本文を値の表の fit.shrink[k-1] の組で組む。
	fitShrink int
}

// IRBlock は本文のブロック。Type で種類を選び、種類ごとに使う欄が違う。
type IRBlock struct {
	Type    string
	Variant string

	Content Inline            // paragraph / note / takeaway / quote
	Source  Inline            // quote: 出どころ
	Items   []IRPointItem     // points
	Ordered bool              // points: 番号つき(順序に意味がある。番号の形はテンプレートの orderedStyle)
	Options []IRCompareOption // compare
	Stats   []IRStatItem      // stat

	Subsystem string          // subsystem: 名前(例 "gantt")
	Data      json.RawMessage // subsystem: 中身(サブシステムが読む)

	Columns []IRColumn // columns: 並列のまとまり(2〜3個)
	// Flow は columns の左のまとまりが右のまとまりにつながる(前提→結論、発見→対応)
	// という意味。印の形はテンプレートが決める。
	Flow bool

	Header []Inline        // table: 見出しの行
	Rows   [][]IRTableCell // table: 本体の行(どの行もセルの数は見出しと同じ)
	Total  []IRTableCell   // table: 合計の行(任意。セルの数は見出しと同じ)
	Groups []IRTableGroup  // table: 行の群(任意。先頭の列が群の列。本体の行のセルは見出しより1つ少ない)
	// GroupColumns は2列目以降の群の列(任意。左から。どの群も左の群の列の境を越えない。
	// 本体の行のセルは群の列を除く)。
	GroupColumns []IRTableGroupColumn // table
	Caption      Inline               // table・image: 説明の文(任意)
	// Continues は前のスライドの表の続き(table。見出しの行を書かず、最初の表の
	// 見出しの行・列の数・groups を引き継ぐ。tablechain.go)。
	Continues bool

	// chain は続きの表のひと続き(linkTableChains が入れる。続きの無い表では nil)。
	chain    *tableChain
	chainPos int // ひと続きの中の何枚目か(0 が最初の表)

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
	// Ordered は入れ子の箇条(Items)が番号つきか(親の箇条とは独立)。
	Ordered bool `json:"ordered,omitempty"`
}

// pointMaxDepth は箇条の入れ子の段の数の上限(Schema の pointItem・pointItem2・pointItem3)。
const pointMaxDepth = 3

// pointDepth は箇条の段の数(入れ子の無い箇条は 1)。
func pointDepth(items []IRPointItem) int {
	d := 0
	for _, it := range items {
		d = max(d, 1+pointDepth(it.Items))
	}
	return d
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
		if v.Ordered && len(v.Items) == 0 {
			return fmt.Errorf("point item \"ordered\" numbers the nested \"items\", but the item has none")
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
			Marker  *string           `json:"marker"`
			Density *string           `json:"density"`
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
		if v.Marker != nil {
			if err := checkMarker(*v.Marker); err != nil {
				return fmt.Errorf("marker: %w", err)
			}
			out.Marker = *v.Marker
		}
		if v.Density != nil {
			if !isDensity(*v.Density) {
				return fmt.Errorf("density: %q is not a density (known: %v)", *v.Density, densityNames)
			}
			if *v.Density == densityDense {
				out.Density = densityDense
			}
		}
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
			Level   int    `json:"level"`
			Notes   string `json:"notes"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Title == nil {
			return fmt.Errorf("section slide needs \"title\"")
		}
		if v.Level != 0 && (v.Level < 1 || v.Level > 3) {
			return fmt.Errorf("level: section level must be 1, 2 or 3 (got %d)", v.Level)
		}
		*s = IRSlide{Type: v.Type, Variant: v.Variant, Title: v.Title, Level: v.Level, Notes: v.Notes}
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
			Ordered bool          `json:"ordered"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Items) == 0 {
			return fmt.Errorf("points needs at least one item")
		}
		if d := pointDepth(v.Items); d > pointMaxDepth {
			return fmt.Errorf("points nest at most %d levels (got %d)", pointMaxDepth, d)
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Items: v.Items, Ordered: v.Ordered}
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
			Flow    bool              `json:"flow"`
			Columns []json.RawMessage `json:"columns"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if len(v.Columns) < 2 || len(v.Columns) > 3 {
			return fmt.Errorf("columns needs two or three columns (got %d)", len(v.Columns))
		}
		out := IRBlock{Type: v.Type, Variant: v.Variant, Flow: v.Flow}
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
			Type         string               `json:"type"`
			Variant      string               `json:"variant"`
			Caption      Inline               `json:"caption"`
			Header       []Inline             `json:"header"`
			Rows         [][]IRTableCell      `json:"rows"`
			Total        []IRTableCell        `json:"total"`
			Groups       []IRTableGroup       `json:"groups"`
			GroupColumns []IRTableGroupColumn `json:"groupColumns"`
			// Continues は書かなければ false。false と書くのは誤り(続きでない表は書かない)。
			Continues *bool `json:"continues"`
		}
		if err := strictUnmarshal(b, &v); err != nil {
			return err
		}
		if v.Continues != nil && !*v.Continues {
			return fmt.Errorf("continues: write continues only as true (omit it for a table that does not continue)")
		}
		cont := v.Continues != nil
		if cont {
			// 続きの表は見出しの行を書かない(最初の表の見出しの行を繰り返す)。
			// 列の数の検査は linkTableChains が最初の表の見出しの行で行う。
			if v.Header != nil {
				return fmt.Errorf("header: a continuing table (continues: true) has no header; the header of the first table is repeated")
			}
			if len(v.Rows) == 0 {
				return fmt.Errorf("table needs at least one row in \"rows\"")
			}
		} else if len(v.Header) == 0 || len(v.Rows) == 0 {
			return fmt.Errorf("table needs \"header\" and at least one row in \"rows\"")
		}
		if !cont {
			if err := validateTableShape(v.Header, v.Rows, v.Total, v.Groups, v.GroupColumns); err != nil {
				return err
			}
		}
		*k = IRBlock{Type: v.Type, Variant: v.Variant, Caption: v.Caption, Header: v.Header, Rows: v.Rows, Total: v.Total, Groups: v.Groups, GroupColumns: v.GroupColumns, Continues: cont}
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
// columnSubsystems のものだけ(グラフの横に読み取り、図の横に段ごとの説明・柱ごとの説明を置く形)。
// 対の並び(pairs)と画像の並び(gallery)も置ける(#117。対の横に抽出した一覧、写真の
// 並びの横に説明)。人物の紹介(people)は columnPeopleMax 人まで(縦に積む。#120)。
// 印つきの塊の並び(topics)も置ける(縦に積むだけ。#129。概要の横に話題の塊)。
// columns・compare は置けない。
var columnBlockTypes = []string{"paragraph", "points", "note", "table", "image", "gallery", "pairs", "subsystem", peopleType, topicsType}

// columnSubsystems はまとまりの中に置けるサブシステム。どれもまとまりの幅で
// 測れる(幅→高さ)図で、gantt・process・funnel・shapes のように本文の幅を
// 前提にするものは置けない。
var columnSubsystems = []string{"chart", "cycle", "phases", "matrix", "orgchart", "pillars"}

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
		if blk.Type == peopleType {
			if p, err := readPeople(blk.Raw); err == nil && len(p.People) > columnPeopleMax {
				return fmt.Errorf("body[%d]: people inside a column can have at most %d people (got %d; put more people in a people block of the slide body)", i, columnPeopleMax, len(p.People))
			}
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
	if err := linkTableChains(d); err != nil {
		return nil, err
	}
	return d, nil
}

// strictUnmarshal は知らないキーを断って読む。
func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if fe := findFieldError(b, v); fe != nil {
			return fe
		}
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after the value")
	}
	return nil
}

// irFieldError は対応表の1つの欄(Key)の値が読めなかった誤り。誤りの文は
// 「key: 元の文」で、場所(JSON Pointer)は errPointer が誤りの鎖から組む。
type irFieldError struct {
	Key string
	Err error
}

func (e *irFieldError) Error() string { return e.Key + ": " + e.Err.Error() }
func (e *irFieldError) Unwrap() error { return e.Err }

// findFieldError は strictUnmarshal が断った対応表(b)を欄ごとに読み直し、
// 文書の順で最初に読めない欄を irFieldError にして返す。知らないキー
// (encoding/json の文が場所を持つ)と、欄ごとでは誤りにならないときは nil。
func findFieldError(b []byte, v any) error {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		key, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil
		}
		one, err := json.Marshal(map[string]json.RawMessage{key: raw})
		if err != nil {
			return nil
		}
		d := json.NewDecoder(bytes.NewReader(one))
		d.DisallowUnknownFields()
		if err := d.Decode(reflect.New(t.Elem()).Interface()); err != nil {
			if strings.Contains(err.Error(), "json: unknown field") {
				return nil
			}
			return &irFieldError{Key: key, Err: err}
		}
	}
	return nil
}

// errPointer は ParseIR の誤りの場所を JSON Pointer にする。文の頭の場所
// (locationToPointer)に、誤りの鎖の irFieldError の欄の名前をつなぐ。
func errPointer(err error) string {
	msg := err.Error()
	var fe *irFieldError
	if !errors.As(err, &fe) || !strings.HasSuffix(msg, fe.Error()) {
		return locationToPointer(msg)
	}
	return locationToPointer(strings.TrimSuffix(msg, fe.Error())) + "/" + escapePointer(fe.Key) + errPointer(fe.Err)
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

// IRTableGroup は表の行の群1つ(IR の table の groups)。群の名前と、その群が
// まとめる本体の行の数。群は上から順に並ぶ。
type IRTableGroup struct {
	Label Inline `json:"label"`
	Rows  int    `json:"rows"`
}

// IRTableGroupColumn は2列目以降の群の列1つ(IR の table の groupColumns)。
// Column は見出しの行の列の番号(0 から。先頭の列は groups で書くので 1 以上)。
type IRTableGroupColumn struct {
	Column int            `json:"column"`
	Groups []IRTableGroup `json:"groups"`
}

// validateGroupList は1つの群の列の群(名前が空でなく、行の数の和が本体の行の数)を見る。
// at は誤りの文の頭("groups" や "groupColumns[0].groups")。
func validateGroupList(gs []IRTableGroup, rows int, at string) error {
	if len(gs) == 0 {
		return fmt.Errorf("%s: needs at least one group", at)
	}
	sum := 0
	for i, g := range gs {
		if g.Rows < 1 {
			return fmt.Errorf("%s[%d]: rows must be at least 1", at, i)
		}
		if strings.TrimSpace(g.Label.Plain()) == "" {
			return fmt.Errorf("%s[%d]: label is empty", at, i)
		}
		sum += g.Rows
	}
	if sum != rows {
		return fmt.Errorf("%s: the groups cover %d rows but the table has %d", at, sum, rows)
	}
	return nil
}

// groupStarts は群の始まりの行の番号の集まり。
func groupStarts(gs []IRTableGroup) map[int]bool {
	out := map[int]bool{}
	r := 0
	for _, g := range gs {
		out[r] = true
		r += g.Rows
	}
	return out
}

// validateTableGroups は groups と groupColumns の形を見る(群の列のほかに列があり、
// 名前が空でなく、行の数の和が本体の行の数、群の列は左から並び、どの群も左の群の列の
// 境を越えない)。
func validateTableGroups(gs []IRTableGroup, gcs []IRTableGroupColumn, cols, rows int) error {
	if gs == nil && gcs == nil {
		return nil
	}
	if gcs != nil && len(gcs) == 0 {
		return fmt.Errorf("groupColumns: needs at least one column")
	}
	n := len(gcs)
	if gs != nil {
		n++
	}
	if cols-n < 1 {
		if gcs == nil {
			return fmt.Errorf("groups: the table needs a column besides the group column")
		}
		return fmt.Errorf("groupColumns: the table needs a column besides the group columns")
	}
	var bounds map[int]bool
	if gs != nil {
		if err := validateGroupList(gs, rows, "groups"); err != nil {
			return err
		}
		bounds = groupStarts(gs)
	}
	prev := 0
	for k, gc := range gcs {
		at := fmt.Sprintf("groupColumns[%d]", k)
		if gc.Column < 1 || gc.Column >= cols {
			return fmt.Errorf("%s.column: %d is not a column after the first (1 to %d; the first column is written as groups)", at, gc.Column, cols-1)
		}
		if gc.Column <= prev && k > 0 {
			return fmt.Errorf("%s.column: the group columns go from left to right without repeats (%d after %d)", at, gc.Column, prev)
		}
		prev = gc.Column
		if err := validateGroupList(gc.Groups, rows, at+".groups"); err != nil {
			return err
		}
		starts := groupStarts(gc.Groups)
		if r, ok := firstCrossing(bounds, starts); ok {
			return fmt.Errorf("%s.groups: %s", at, msgGroupCrosses(r))
		}
		bounds = starts
	}
	return nil
}

// firstCrossing は左の群の列の境(bounds)のうち、starts に無いいちばん上の行。
func firstCrossing(bounds, starts map[int]bool) (int, bool) {
	best := -1
	for r := range bounds {
		if !starts[r] && (best < 0 || r < best) {
			best = r
		}
	}
	return best, best >= 0
}

// msgGroupCrosses は群が左の群の列の境(本体の行 r から始まる群)を越える誤りの文。
func msgGroupCrosses(r int) string {
	return fmt.Sprintf("a group crosses the boundary of a group in a column to its left at body row %d (a group must stay inside one group of every group column to its left; start a new group there)", r)
}

// tableGroupColumnCount は群の列の数(先頭の列の groups と groupColumns)。
func tableGroupColumnCount(gs []IRTableGroup, gcs []IRTableGroupColumn) int {
	n := len(gcs)
	if gs != nil {
		n++
	}
	return n
}

// validateTableShape は表の行の群と、本体・合計の行のセルの数を見る(header は
// 最初の表の見出しの行)。
func validateTableShape(header []Inline, rows [][]IRTableCell, total []IRTableCell, groups []IRTableGroup, groupCols []IRTableGroupColumn) error {
	if err := validateTableGroups(groups, groupCols, len(header), len(rows)); err != nil {
		return err
	}
	ng := tableGroupColumnCount(groups, groupCols)
	want := len(header) - ng
	for i, r := range rows {
		if len(r) != want {
			return fmt.Errorf("rows[%d]: row has %d cells but %s", i, len(r), tableRowWant(len(header), ng))
		}
	}
	if total != nil && len(total) != len(header) {
		return fmt.Errorf("total: row has %d cells but the header has %d", len(total), len(header))
	}
	return nil
}

// tableRowWant は本体の行のセルの数の決まりを文で言う(groupCols は群の列の数)。
func tableRowWant(header int, groupCols int) string {
	switch {
	case groupCols == 1:
		return fmt.Sprintf("the header has %d (with groups, a row has one cell per column except the group column: %d)", header, header-1)
	case groupCols > 1:
		return fmt.Sprintf("the header has %d (with groups, a row has one cell per column except the %d group columns: %d)", header, groupCols, header-groupCols)
	}
	return fmt.Sprintf("the header has %d", header)
}

// IRTableCell は表の本体・合計の行のセル1つ。文(インライン)か、セルの中の箇条
// ({"points": [...]}、段落は合わせて2つ以上)。Points が nil なら Text のセル。
type IRTableCell struct {
	Text   Inline
	Points []IRCellPoint
}

// IRCellPoint はセルの中の箇条の1項目。文(インライン)か {text, children}
// (Children は2段目の項目。セルの中の箇条は2段まで)。
type IRCellPoint struct {
	Text     Inline
	Children []Inline
}

func (p *IRCellPoint) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return p.Text.UnmarshalJSON(b)
	}
	var v struct {
		Text     Inline   `json:"text"`
		Children []Inline `json:"children"`
	}
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	if v.Text == nil || len(v.Children) == 0 {
		return fmt.Errorf("a cell list item object needs \"text\" and at least one item in \"children\"")
	}
	*p = IRCellPoint{Text: v.Text, Children: v.Children}
	return nil
}

// cellParaCount はセルの中の箇条の段落の数(項目とその子の項目の和)。
func cellParaCount(items []IRCellPoint) int {
	n := 0
	for _, it := range items {
		n += 1 + len(it.Children)
	}
	return n
}

func (c *IRTableCell) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return c.Text.UnmarshalJSON(b)
	}
	var v struct {
		Points []IRCellPoint `json:"points"`
	}
	if err := strictUnmarshal(b, &v); err != nil {
		return err
	}
	if cellParaCount(v.Points) < 2 {
		return fmt.Errorf("points: a cell list needs at least 2 items (write a single item as a plain cell)")
	}
	for i, p := range v.Points {
		if strings.TrimSpace(p.Text.Plain()) == "" {
			return fmt.Errorf("points[%d]: the item is empty", i)
		}
		for j, ch := range p.Children {
			if strings.TrimSpace(ch.Plain()) == "" {
				return fmt.Errorf("points[%d].children[%d]: the item is empty", i, j)
			}
		}
	}
	*c = IRTableCell{Points: v.Points}
	return nil
}

// Plain は飾りを外した文字列(箇条の項目は改行でつなぐ)。
func (c IRTableCell) Plain() string {
	if c.Points == nil {
		return c.Text.Plain()
	}
	var parts []string
	for _, p := range c.Points {
		parts = append(parts, p.Text.Plain())
		for _, ch := range p.Children {
			parts = append(parts, ch.Plain())
		}
	}
	return strings.Join(parts, "\n")
}

// 本文の密度(content の density)。
const (
	densityNormal = "normal"
	densityDense  = "dense"
)

// densityNames は density に書ける値(省けば normal)。
var densityNames = []string{densityNormal, densityDense}

func isDensity(v string) bool { return v == densityNormal || v == densityDense }

// dense は本文を値の表の dense の組で組むスライドか。
func (s *IRSlide) dense() bool { return s.Density == densityDense }

// markerMaxRunes はスライドの隅の印(marker)の字の数の上限。
const markerMaxRunes = 2

// checkMarker はスライドの隅の印が、空白を含まない1〜2字の文であることを確かめる。
func checkMarker(m string) error {
	n := utf8.RuneCountInString(m)
	if n == 0 {
		return fmt.Errorf("marker must not be empty")
	}
	if n > markerMaxRunes {
		return fmt.Errorf("marker %q is longer than %d characters", m, markerMaxRunes)
	}
	if strings.IndexFunc(m, unicode.IsSpace) >= 0 {
		return fmt.Errorf("marker %q contains a space", m)
	}
	return nil
}
