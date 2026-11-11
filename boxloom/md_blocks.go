// Boxloom Markdown の表(GFM の表)・画像(![代わりの文](path))・並列のまとまり
// (::::columns と :::column)を IR にする(docs/content-syntax.md の 3.3・3.4)。

package boxloom

import (
	"fmt"
	"regexp"
	"strings"
)

// --- 表 ---

type mdTableLine struct {
	text      string
	line, col int
}

type mdTableBlk struct {
	rows        []mdTableLine
	caption     mdText // 表のすぐ次の行の「: 説明」
	captionLine int
	attrs       string // その行の末尾の {…}(total=last)
	attrsCol    int
}

// tableAttributeNames は表の説明の行の末尾 {…} に書ける属性。
var tableAttributeNames = []string{"total"}

var reTableDelim = regexp.MustCompile(`^:?-+:?$`)

// splitTableRow は「| a | b |」をセルに分ける(\| はセルの中の文字の |)。
// 行の終わりの | が無ければ ok=false。
func splitTableRow(tl mdTableLine) (cells []mdText, ok bool) {
	t := lineText(tl.text, tl.line, tl.col)
	r := t.r
	var start int = 1 // 先頭の | の次
	for k := 1; k < len(r); k++ {
		if r[k] == '\\' && k+1 < len(r) {
			k++
			continue
		}
		if r[k] != '|' {
			continue
		}
		cells = append(cells, unescapePipes(trimText(mdText{r: r[start:k], pos: t.pos[start:k]})))
		start = k + 1
	}
	return cells, start == len(r)
}

// unescapePipes はセルの中の \| を | にする(GFM と同じく、コードの中でも)。
func unescapePipes(t mdText) mdText {
	var out mdText
	for k := 0; k < len(t.r); k++ {
		if t.r[k] == '\\' && k+1 < len(t.r) && t.r[k+1] == '|' {
			continue
		}
		out.r = append(out.r, t.r[k])
		out.pos = append(out.pos, t.pos[k])
	}
	return out
}

// trimText は前後の空白を外す(場所は保つ)。
func trimText(t mdText) mdText {
	a, b := 0, len(t.r)
	for a < b && (t.r[a] == ' ' || t.r[a] == '\t') {
		a++
	}
	for b > a && (t.r[b-1] == ' ' || t.r[b-1] == '\t') {
		b--
	}
	return mdText{r: t.r[a:b], pos: t.pos[a:b]}
}

func (p *mdParser) tableBlock(it *mdItem, path string) any {
	tb := it.table
	var rows [][]mdText
	bad := false
	for _, tl := range tb.rows {
		cells, ok := splitTableRow(tl)
		if !ok {
			p.fail(tl.line, tl.col, path, codeUnsupportedSyntax, "表の行は | で終えてください", nil, "行の始まりと終わりに | を置きます(| a | b |)")
			bad = true
		}
		rows = append(rows, cells)
	}
	if bad {
		return nil
	}
	if len(rows) < 2 || !isDelimRow(rows[1]) {
		p.fail(it.line, 1, path, codeMisplaced, "表の2行目は区切りの行(| --- | --- |)にしてください", nil, "1行目が見出しの行、2行目が区切りの行、3行目からが本体です")
		return nil
	}
	header := rows[0]
	for c, d := range rows[1] {
		if strings.ContainsRune(string(d.r), ':') {
			ps := d.pos[0]
			p.fail(ps.line, ps.col, path, codeUnsupportedSyntax, "列の配置の指定(:--: など)は使えません。配置はテンプレートが決めます", nil,
				fmt.Sprintf("%d 列目の区切りを --- にしてください(数だけの列はテンプレートが右に揃えます)", c+1))
		}
	}
	if len(rows[1]) != len(header) {
		p.fail(tb.rows[1].line, tb.rows[1].col, path, codeTableShape, fmt.Sprintf("区切りの行のセルが %d 個で、見出しの行(%d 個)と合いません", len(rows[1]), len(header)), nil, "")
	}
	body := rows[2:]
	if len(body) == 0 {
		p.fail(it.line, 1, path, codeMisplaced, "表に本体の行がありません", nil, "区切りの行の下に行を書いてください")
		return nil
	}
	attrs := map[string]string{}
	if tb.attrs != "" {
		attrs = p.parseAttrs(tb.attrs, tb.captionLine, tb.attrsCol, path, tableAttributeNames)
	}
	total := false
	if v, ok := attrs["total"]; ok {
		if v != "last" {
			p.fail(tb.captionLine, tb.attrsCol, path+"/total", codeInvalidValue, fmt.Sprintf("total の値 %q はありません", v), []string{"last"}, "表の最後の行が合計の行なら {total=last} と書きます")
		} else if len(body) < 2 {
			p.fail(tb.captionLine, tb.attrsCol, path+"/total", codeMisplaced, "合計の行のほかに本体の行がありません", nil, "合計の行(最後の行)の上に、合計する行を書いてください")
		} else {
			total = true
		}
	}
	hdr := []any{}
	for c, cell := range header {
		hdr = append(hdr, p.parseInline(cell, fmt.Sprintf("%s/header/%d", path, c)))
	}
	out := []any{}
	var totalCells []any
	for r, row := range body {
		rp := fmt.Sprintf("%s/rows/%d", path, r)
		isTotal := total && r == len(body)-1
		if isTotal {
			rp = path + "/total"
		}
		tl := tb.rows[r+2]
		p.mark(rp, tl.line)
		if len(row) != len(header) {
			p.fail(tl.line, tl.col, rp, codeTableShape, fmt.Sprintf("行のセルが %d 個で、見出しの行(%d 個)と合いません", len(row), len(header)), nil, "どの行も見出しと同じ数のセルを書きます。空のセルは | | と書けます")
			continue
		}
		cells := []any{}
		for c, cell := range row {
			cells = append(cells, p.parseInline(cell, fmt.Sprintf("%s/%d", rp, c)))
		}
		if isTotal {
			totalCells = cells
			continue
		}
		out = append(out, cells)
	}
	b := obj("type", "table", "header", hdr, "rows", out)
	if totalCells != nil {
		b.set("total", totalCells)
	}
	if len(tb.caption.r) > 0 {
		p.mark(path+"/caption", tb.captionLine)
		b.set("caption", p.parseInline(tb.caption, path+"/caption"))
	}
	return b
}

func isDelimRow(cells []mdText) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !reTableDelim.MatchString(string(c.r)) {
			return false
		}
	}
	return true
}

// --- 画像 ---

type mdImage struct {
	alt     string
	altPos  srcPos
	src     string
	srcPos  srcPos
	caption mdText // ![…](path "説明") の "…"
}

// scanImages は段落が画像だけ(空白で区切った ![…](…) の並び)なら、その画像を返す。
func scanImages(t mdText) ([]mdImage, bool) {
	r := t.r
	var out []mdImage
	i := 0
	space := func(c rune) bool { return c == ' ' || c == '\t' || c == brRune }
	for {
		for i < len(r) && space(r[i]) {
			i++
		}
		if i == len(r) {
			break
		}
		if i+1 >= len(r) || r[i] != '!' || r[i+1] != '[' {
			return nil, false
		}
		start := i
		k := i + 2
		for k < len(r) && r[k] != ']' {
			k++
		}
		if k+1 >= len(r) || r[k+1] != '(' {
			return nil, false
		}
		im := mdImage{alt: strings.TrimSpace(string(r[i+2 : k])), altPos: t.pos[start]}
		k += 2
		for k < len(r) && r[k] == ' ' {
			k++
		}
		s0 := k
		for k < len(r) && r[k] != ')' && r[k] != ' ' {
			k++
		}
		im.src = string(r[s0:k])
		if s0 < len(t.pos) {
			im.srcPos = t.pos[s0]
		}
		for k < len(r) && r[k] == ' ' {
			k++
		}
		if k < len(r) && r[k] == '"' {
			c0 := k + 1
			k = c0
			for k < len(r) && r[k] != '"' {
				k++
			}
			if k >= len(r) {
				return nil, false
			}
			im.caption = trimText(mdText{r: r[c0:k], pos: t.pos[c0:k]})
			k++
			for k < len(r) && r[k] == ' ' {
				k++
			}
		}
		if k >= len(r) || r[k] != ')' {
			return nil, false
		}
		i = k + 1
		out = append(out, im)
	}
	return out, len(out) > 0
}

// imageParagraph は画像だけの段落を image(1枚)か gallery(2〜6枚)にする。
// 画像だけの段落でなければ ok=false(ふつうの段落として読む)。
func (p *mdParser) imageParagraph(it *mdItem, path string) (any, bool) {
	imgs, ok := scanImages(it.text)
	if !ok {
		return nil, false
	}
	if len(imgs) > 6 {
		p.fail(it.line, 1, path, codeMisplaced, fmt.Sprintf("1つの段落に画像が %d 枚あります。並べられるのは6枚までです", len(imgs)), nil, "空行で段落を分けるか、スライドを分けてください")
		return nil, true
	}
	one := func(im mdImage, ip string) *jobj {
		o := &jobj{}
		if err := checkImageSrc(im.src); err != nil {
			p.fail(im.srcPos.line, im.srcPos.col, ip+"/src", codeImagePath, err.Error(), nil, "文書のファイルからの相対パスで書いてください(例 images/plan.png)")
		}
		if im.alt == "" {
			p.fail(im.altPos.line, im.altPos.col, ip+"/alt", codeImageAlt, "画像の代わりの文(![代わりの文](path) の […])がありません", nil, "画像が何を示すかを1文で書いてください(読み上げと、画像が無いときに使います)")
		}
		o.set("src", im.src)
		o.set("alt", im.alt)
		if len(im.caption.r) > 0 {
			o.set("caption", p.parseInline(im.caption, ip+"/caption"))
		}
		return o
	}
	if len(imgs) == 1 {
		b := obj("type", "image")
		o := one(imgs[0], path)
		for i, k := range o.keys {
			b.set(k, o.vals[i])
		}
		return b, true
	}
	var list []any
	for i, im := range imgs {
		ip := fmt.Sprintf("%s/images/%d", path, i)
		p.mark(ip, im.altPos.line)
		list = append(list, one(im, ip))
	}
	return obj("type", "gallery", "images", list), true
}

// --- 並列のまとまり ---

func (p *mdParser) columnsBlock(f *mdFenceBlk, path string) any {
	attrs := p.fenceAttrs(f, path, []string{"variant"})
	p.noLabel(f, path)
	var cols []any
	mains := 0
	for _, it := range f.items {
		if it.kind != mdFence || it.fence.name != "column" {
			p.fail(it.line, 1, path, codeMisplaced, "::::columns の中には :::column だけを書けます", []string{"column"}, "")
			continue
		}
		c := it.fence
		cp := fmt.Sprintf("%s/columns/%d", path, len(cols))
		p.mark(cp, c.line)
		ca := p.fenceAttrs(c, cp, []string{"role"})
		col := &jobj{}
		if len(c.label.r) > 0 {
			col.set("title", p.parseInline(c.label, cp+"/title"))
		}
		if role, ok := ca["role"]; ok {
			if role != "main" {
				p.fail(c.line, c.attrsCol, cp+"/role", codeInvalidValue, fmt.Sprintf("role %q はありません", role), []string{"main"}, "主のまとまり(ほかがそれを補う)に role=main を付けます。幅はテンプレートが決めます")
			} else {
				mains++
				if mains > 1 {
					p.fail(c.line, c.attrsCol, cp+"/role", codeInvalidValue, "role=main を付けられるのは1つの ::::columns に1つだけです", nil, "")
				}
				col.set("role", "main")
			}
		}
		var allowed []*mdItem
		for _, sub := range c.items {
			ok := sub.kind == mdPara || sub.kind == mdList || sub.kind == mdTable || (sub.kind == mdFence && sub.fence.name == "note") ||
				(sub.kind == mdCode && contains(columnSubsystems, codeLang(sub.info)))
			if !ok {
				what := "コードブロック"
				if sub.kind == mdFence {
					what = ":::" + sub.fence.name
				} else if l := codeLang(sub.info); l != "" {
					what = "```" + l
				}
				p.fail(sub.line, 1, cp, codeMisplaced, fmt.Sprintf(":::column の中に %s は書けません", what), []string{"段落", "箇条", "表", "画像", "note", "```chart"}, "")
				continue
			}
			allowed = append(allowed, sub)
		}
		body := p.blocks(allowed, cp+"/body")
		for k, b := range body {
			if o, ok := b.(*jobj); ok && len(o.vals) > 0 && o.vals[0] == "gallery" {
				bp := fmt.Sprintf("%s/body/%d", cp, k)
				p.fail(p.lines[bp], 1, bp, codeMisplaced, ":::column の中には画像を1枚ずつしか置けません(画像の並びは本文に置きます)", nil, "")
			}
		}
		if len(body) == 0 {
			p.fail(c.line, 1, cp, codeMisplaced, ":::column の中が空です", nil, "")
		}
		col.set("body", body)
		cols = append(cols, col)
	}
	if len(cols) < 2 || len(cols) > 3 {
		p.fail(f.line, 1, path, codeMisplaced, fmt.Sprintf("::::columns には :::column を2つか3つ書きます(いま %d 個)", len(cols)), nil, "4つ以上の並列で、それぞれ1行なら箇条(- …)にしてください")
		return nil
	}
	b := obj("type", "columns")
	if v, ok := attrs["variant"]; ok {
		b.set("variant", v)
	}
	b.set("columns", cols)
	return b
}

// codeLang はコードブロックの情報の行の言語名(属性 {…} の前まで)。
func codeLang(info string) string {
	if k := strings.IndexAny(info, " \t{"); k >= 0 {
		return info[:k]
	}
	return info
}
