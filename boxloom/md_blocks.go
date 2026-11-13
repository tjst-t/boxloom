// Boxloom Markdown の表(GFM の表)・画像(![代わりの文](path))・並列のまとまり
// (::::columns と :::column)を IR にする(docs/content-syntax.md の 3.3・3.4)。

package boxloom

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
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
	attrs       string // その行の末尾の {…}(total=last・groups=first か列名・points=列名)
	attrsCol    int
}

// tableAttributeNames は表の説明の行の末尾 {…} に書ける属性。
var tableAttributeNames = []string{"total", "groups", "points", "continues"}

// mdTableShape は表の列の名前・群の列・箇条の列(続きの表 {continues} が引き継ぐ)。
type mdTableShape struct {
	names     []string
	groupCols []int // 群の列(左から。先頭の列は 0)
	points    map[int]bool
}

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
	attrs := map[string]string{}
	if tb.attrs != "" {
		attrs = p.parseAttrs(tb.attrs, tb.captionLine, tb.attrsCol, path, tableAttributeNames)
	}
	_, cont := attrs["continues"]
	// 続きの表は見出しの行を書かない:1行目が区切りの行で、2行目からが本体。
	// 見出しの列は直前の表(p.lastTable)から引き継ぐ。
	var header []mdText
	var prev *mdTableShape
	delimAt := 1
	if cont {
		prev = p.lastTable
		if !isDelimRow(rows[0]) {
			hint := "1行目を区切りの行(| --- | --- |)にし、本体の行を続けます。見出しの行は最初の表のものを Boxloom が繰り返します"
			if len(rows) >= 2 && isDelimRow(rows[1]) {
				p.fail(tb.rows[0].line, tb.rows[0].col, path+"/header", codeTableContinues, "続きの表({continues})には見出しの行を書きません", nil, hint)
			} else {
				p.fail(it.line, 1, path, codeMisplaced, "続きの表({continues})の1行目は区切りの行(| --- | --- |)にしてください", nil, hint)
			}
			return nil
		}
		delimAt = 0
		if prev != nil {
			for _, n := range prev.names {
				header = append(header, mdText{r: []rune(n)})
			}
		} else {
			// 前の表が無い(誤りは IR の検査が table-continues で言う)。列の数は区切りの行から。
			header = make([]mdText, len(rows[0]))
		}
	} else {
		if isDelimRow(rows[0]) {
			p.fail(it.line, 1, path, codeMisplaced, "表の1行目が区切りの行で、見出しの行がありません", nil,
				"前のスライドの表の続きなら、説明の行に {continues} と書きます(見出しの行は最初の表のものを繰り返します)。そうでなければ1行目に見出しの行を書きます")
			return nil
		}
		if len(rows) < 2 || !isDelimRow(rows[1]) {
			p.fail(it.line, 1, path, codeMisplaced, "表の2行目は区切りの行(| --- | --- |)にしてください", nil, "1行目が見出しの行、2行目が区切りの行、3行目からが本体です")
			return nil
		}
		header = rows[0]
	}
	for c, d := range rows[delimAt] {
		if strings.ContainsRune(string(d.r), ':') {
			ps := d.pos[0]
			p.fail(ps.line, ps.col, path, codeUnsupportedSyntax, "列の配置の指定(:--: など)は使えません。配置はテンプレートが決めます", nil,
				fmt.Sprintf("%d 列目の区切りを --- にしてください(数だけの列はテンプレートが右に揃えます)", c+1))
		}
	}
	if len(rows[delimAt]) != len(header) {
		dl := tb.rows[delimAt]
		if cont {
			p.fail(dl.line, dl.col, path, codeTableContinues, fmt.Sprintf("区切りの行のセルが %d 個で、前の表の列(%d 個)と合いません", len(rows[delimAt]), len(header)), nil,
				"続きの表の列の数は最初の表と同じにします")
		} else {
			p.fail(dl.line, dl.col, path, codeTableShape, fmt.Sprintf("区切りの行のセルが %d 個で、見出しの行(%d 個)と合いません", len(rows[delimAt]), len(header)), nil, "")
		}
	}
	bodyAt := delimAt + 1
	body := rows[bodyAt:]
	if len(body) == 0 {
		p.fail(it.line, 1, path, codeMisplaced, "表に本体の行がありません", nil, "区切りの行の下に行を書いてください")
		return nil
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
	var groupCols []int
	if v, ok := attrs["groups"]; ok {
		groupCols = p.tableGroupColumns(v, header, tb, path)
	}
	pointCols := p.tablePointColumns(attrs, header, groupCols, tb, path)
	if cont && prev != nil {
		// groups と points は最初の表に従う(書かなければ引き継ぎ、書くなら同じに)。
		if _, ok := attrs["groups"]; ok && !slices.Equal(groupCols, prev.groupCols) {
			p.fail(tb.captionLine, tb.attrsCol, path+"/groups", codeTableContinues, "続きの表の groups が前の表と違います", nil, "続きの表の groups は書かずに前の表から引き継ぎます")
		}
		if _, ok := attrs["points"]; ok && !maps.Equal(pointCols, prev.points) {
			p.fail(tb.captionLine, tb.attrsCol, path+"/points", codeTableContinues, "続きの表の points の列が前の表と違います", nil, "続きの表の points は書かずに前の表から引き継ぎます")
		}
		groupCols, pointCols = prev.groupCols, prev.points
	}
	shape := &mdTableShape{groupCols: groupCols, points: pointCols}
	for _, h := range header {
		shape.names = append(shape.names, string(h.r))
	}
	p.lastTable = shape
	hdr := []any{}
	if !cont {
		for c, cell := range header {
			hdr = append(hdr, p.parseInline(cell, fmt.Sprintf("%s/header/%d", path, c)))
		}
	}
	out := []any{}
	var totalCells []any
	// 群の列ごとの群(groupCols と同じ順)。
	type groupState struct {
		list []any
		cur  *jobj
		rows int
	}
	states := make([]groupState, len(groupCols))
	groupPath := func(k int) string {
		if groupCols[k] == 0 {
			return path + "/groups"
		}
		if groupCols[0] == 0 {
			return fmt.Sprintf("%s/groupColumns/%d/groups", path, k-1)
		}
		return fmt.Sprintf("%s/groupColumns/%d/groups", path, k)
	}
	closeGroup := func(st *groupState) {
		if st.cur != nil {
			st.cur.set("rows", st.rows)
			st.list = append(st.list, st.cur)
		}
	}
	isGroupCol := func(c int) bool { return slices.Contains(groupCols, c) }
	for r, row := range body {
		rp := fmt.Sprintf("%s/rows/%d", path, r)
		isTotal := total && r == len(body)-1
		if isTotal {
			rp = path + "/total"
		}
		tl := tb.rows[r+bodyAt]
		p.mark(rp, tl.line)
		if len(row) != len(header) {
			p.fail(tl.line, tl.col, rp, codeTableShape, fmt.Sprintf("行のセルが %d 個で、見出しの行(%d 個)と合いません", len(row), len(header)), nil, "どの行も見出しと同じ数のセルを書きます。空のセルは | | と書けます")
			continue
		}
		if len(groupCols) > 0 && !isTotal {
			// 群の列のセルが群の名前。空なら上の群の続き。左の群の列で群が始まった行では、
			// 右の群の列も群が始まる(左の群の境を越えて結合しない)。
			started, bad := false, false
			for k, c := range groupCols {
				st := &states[k]
				gp := groupPath(k)
				switch {
				case len(row[c].r) > 0:
					closeGroup(st)
					at := fmt.Sprintf("%s/%d", gp, len(st.list))
					p.mark(at, tl.line)
					st.cur = obj("label", p.parseInline(row[c], at+"/label"))
					st.rows = 0
					started = true
				case st.cur == nil:
					p.fail(tl.line, tl.col, gp+"/0/label", codeTableGroups, fmt.Sprintf("最初の行の群の名前(列 %q のセル)が空です", string(header[c].r)), nil, "群の列では、最初の行のセルに群の名前を書きます。空のセルは上の群の続きです")
					bad = true
				case started:
					p.fail(tl.line, tl.col, fmt.Sprintf("%s/%d/label", gp, len(st.list)+1), codeTableGroups, fmt.Sprintf("列 %q のセルが空ですが、左の群の列で新しい群が始まっています", string(header[c].r)), nil, "後ろの群の列は前の群の列の境を越えて結合しません。この行のセルにも群の名前を書きます")
					bad = true
				}
				st.rows++
			}
			if bad {
				continue
			}
		}
		cells := []any{}
		for c, cell := range row {
			if isGroupCol(c) && !isTotal {
				continue
			}
			cp := fmt.Sprintf("%s/%d", rp, len(cells))
			if pointCols[c] {
				cells = append(cells, p.tablePointsCell(cell, cp, string(header[c].r)))
				continue
			}
			cells = append(cells, p.parseInline(cell, cp))
		}
		if isTotal {
			totalCells = cells
			continue
		}
		out = append(out, cells)
	}
	var groupColumns []any
	var groups []any
	for k := range states {
		closeGroup(&states[k])
		if groupCols[k] == 0 {
			groups = states[k].list
			continue
		}
		groupColumns = append(groupColumns, obj("column", groupCols[k], "groups", states[k].list))
	}
	b := obj("type", "table")
	if cont {
		b.set("continues", true)
	} else {
		b.set("header", hdr)
	}
	b.set("rows", out)
	if groups != nil {
		b.set("groups", groups)
	}
	if groupColumns != nil {
		b.set("groupColumns", groupColumns)
	}
	if totalCells != nil {
		b.set("total", totalCells)
	}
	if len(tb.caption.r) > 0 {
		p.mark(path+"/caption", tb.captionLine)
		b.set("caption", p.parseInline(tb.caption, path+"/caption"))
	}
	return b
}

// tableGroupColumns は {groups=first|列名,…} の群の列の番号(左から。first と先頭の列の
// 名前は 0)。知らない列名・同じ列を2度・群の列のほかに列が無いのは誤り(nil を返す)。
func (p *mdParser) tableGroupColumns(v string, header []mdText, tb *mdTableBlk, path string) []int {
	var names []string
	for _, h := range header {
		names = append(names, string(h.r))
	}
	var cols []int
	ok := true
	for _, name := range strings.Split(v, ",") {
		c := -1
		if name == "first" {
			c = 0
		} else {
			c = slices.Index(names, name)
		}
		switch {
		case c < 0:
			p.fail(tb.captionLine, tb.attrsCol, path+"/groups", codeInvalidValue, fmt.Sprintf("groups の列 %q は見出しの行にありません", name), append([]string{"first"}, names...),
				"{groups=first} は先頭の列、{groups=列名} はその列(見出しの行の文と完全に一致させる。複数なら {groups=列名1,列名2})を群の列にします")
			ok = false
		case slices.Contains(cols, c):
			p.fail(tb.captionLine, tb.attrsCol, path+"/groups", codeInvalidValue, fmt.Sprintf("groups の列 %q を2度書いています", name), nil, "群の列は1度ずつ書きます")
			ok = false
		default:
			cols = append(cols, c)
		}
	}
	if !ok {
		return nil
	}
	slices.Sort(cols)
	if len(header)-len(cols) < 1 {
		p.fail(tb.captionLine, tb.attrsCol, path+"/groups", codeMisplaced, "群の列のほかに列がありません", nil, "群の列のほかに、行の中身の列を書いてください")
		return nil
	}
	return cols
}

// tablePointColumns は {points=列名,…} の列(見出しの文と完全に一致する列)。
// 知らない列名・群の列は誤り(table-points)。
func (p *mdParser) tablePointColumns(attrs map[string]string, header []mdText, groupCols []int, tb *mdTableBlk, path string) map[int]bool {
	v, ok := attrs["points"]
	if !ok {
		return nil
	}
	var names []string
	for _, h := range header {
		names = append(names, string(h.r))
	}
	cols := map[int]bool{}
	for _, name := range strings.Split(v, ",") {
		c := -1
		for i, n := range names {
			if n == name {
				c = i
				break
			}
		}
		switch {
		case name == "":
			p.fail(tb.captionLine, tb.attrsCol, path+"/points", codeTablePoints, "points の列名が空です", names, "{points=列名} の列名は見出しの行の文と同じに書きます。複数なら {points=列名1,列名2}")
		case c < 0:
			p.fail(tb.captionLine, tb.attrsCol, path+"/points", codeTablePoints, fmt.Sprintf("points の列名 %q は見出しの行にありません", name), names, "列名は見出しの行の文と完全に一致させます")
		case slices.Contains(groupCols, c):
			p.fail(tb.captionLine, tb.attrsCol, path+"/points", codeTablePoints, fmt.Sprintf("列 %q は群の列で、箇条にできません", name), nil, "群の列({groups=…})でない列を指してください")
		default:
			cols[c] = true
		}
	}
	return cols
}

// tablePointsCell は箇条の列のセルを半角の ; で区切った箇条にする(\; は文の ;)。
// 「- 」で始まる項目は直前の項目の子(2段目。{text, children})。項目が1つなら
// ただの文字列のセル、空の項目・「- 」で始まる最初の項目は誤り(table-points)。
func (p *mdParser) tablePointsCell(cell mdText, path, col string) any {
	var parts []mdText
	start := 0
	for k := 0; k < len(cell.r); k++ {
		if cell.r[k] == '\\' && k+1 < len(cell.r) {
			k++
			continue
		}
		if cell.r[k] == ';' {
			parts = append(parts, trimText(mdText{r: cell.r[start:k], pos: cell.pos[start:k]}))
			start = k + 1
		}
	}
	parts = append(parts, trimText(mdText{r: cell.r[start:], pos: cell.pos[start:]}))
	// 「-」だけのセル(該当なしの印)はただの文字のセル。
	if len(parts) == 1 && (len(parts[0].r) == 1 || !isCellChild(parts[0])) {
		return p.parseInline(parts[0], path)
	}
	// 「- 」で始まる項目は直前の項目の子(2段目)。文字の - で始めるときは \-。
	type cellItem struct {
		text     any
		children []any
	}
	var items []*cellItem
	for i, it := range parts {
		line, c := 0, 0
		if len(it.pos) > 0 {
			line, c = it.pos[0].line, it.pos[0].col
		} else if len(cell.pos) > 0 {
			line, c = cell.pos[0].line, cell.pos[0].col
		}
		child := isCellChild(it)
		if child {
			it = trimText(mdText{r: it.r[1:], pos: it.pos[1:]})
		}
		if len(it.r) == 0 {
			p.fail(line, c, fmt.Sprintf("%s/points/%d", path, i), codeTablePoints, fmt.Sprintf("列 %q のセルの %d 番目の項目が空です", col, i+1), nil, "箇条の項目は半角の ; で区切ります(a;;b や末尾の ; は書きません)。文の ; は \\; と書きます")
			return nil
		}
		if child && len(items) == 0 {
			p.fail(line, c, fmt.Sprintf("%s/points/%d", path, i), codeTablePoints, fmt.Sprintf("列 %q のセルの最初の項目が「- 」で始まっています", col), nil, "「- 」で始まる項目は直前の項目の子(2段目)です。最初の項目は親の項目にします。文字の - で始めるときは \\- と書きます")
			return nil
		}
		if child {
			last := items[len(items)-1]
			last.children = append(last.children, p.parseInline(it, fmt.Sprintf("%s/points/%d/children/%d", path, len(items)-1, len(last.children))))
			continue
		}
		items = append(items, &cellItem{text: p.parseInline(it, fmt.Sprintf("%s/points/%d", path, len(items)))})
	}
	out := []any{}
	for _, it := range items {
		if it.children == nil {
			out = append(out, it.text)
			continue
		}
		out = append(out, obj("text", it.text, "children", it.children))
	}
	return obj("points", out)
}

// isCellChild はセルの箇条の項目(前後の空白を除いたもの)が「- 」で始まる子の項目か
// (「-」だけの項目は空の子の項目)。
func isCellChild(it mdText) bool {
	return len(it.r) >= 1 && it.r[0] == '-' && (len(it.r) == 1 || it.r[1] == ' ' || it.r[1] == '\t')
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
	attrs := p.fenceAttrs(f, path, []string{"variant", "flow"})
	flow := false
	if v, ok := attrs["flow"]; ok {
		switch v {
		case "true":
			flow = true
		case "false":
		default:
			p.fail(f.line, f.attrsCol, path+"/flow", codeInvalidValue, fmt.Sprintf("flow %q は true か false で書いてください", v), []string{"true", "false"},
				"左のまとまりが右のまとまりにつながる(前提→結論、発見→対応)ときに {flow=true} を付けます")
		}
	}
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
			ok := sub.kind == mdPara || sub.kind == mdList || sub.kind == mdTable || (sub.kind == mdFence && (sub.fence.name == "note" || sub.fence.name == pairsType || sub.fence.name == peopleType || sub.fence.name == topicsType)) ||
				(sub.kind == mdCode && contains(columnSubsystems, codeLang(sub.info)))
			if !ok {
				what := "コードブロック"
				if sub.kind == mdFence {
					what = ":::" + sub.fence.name
				} else if l := codeLang(sub.info); l != "" {
					what = "```" + l
				}
				p.fail(sub.line, 1, cp, codeMisplaced, fmt.Sprintf(":::column の中に %s は書けません", what), columnContains, "")
				continue
			}
			allowed = append(allowed, sub)
		}
		body := p.blocks(allowed, cp+"/body")
		for k, b := range body {
			if o, ok := b.(*jobj); ok && len(o.vals) > 0 && o.vals[0] == peopleType {
				for i, key := range o.keys {
					if people, _ := o.vals[i].([]any); key == "people" && len(people) > columnPeopleMax {
						bp := fmt.Sprintf("%s/body/%d", cp, k)
						p.fail(p.lines[bp], 1, bp+"/people", codeMisplaced,
							fmt.Sprintf(":::column の中の ::::people には :::person を%d個まで書けます(いま %d 個)", columnPeopleMax, len(people)), nil,
							"3人以上なら、::::people を本文に置いてください")
					}
				}
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
	if flow {
		b.set("flow", true)
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
