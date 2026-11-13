// PPTX の本物の画像(p:pic)と本物の表(a:tbl)。
//
// 画像は ppt/media/imageN.png|jpeg に1つずつ置き(同じ中身は1つにまとめる)、
// スライドの関係(image)から r:embed で指す。[Content_Types].xml には、画像を
// 使う文書にだけ png・jpeg の Default を足す(画像の無い文書の出力は変わらない)。
//
// 表は p:graphicFrame の中の a:tbl。PowerPoint・LibreOffice で表として編集できる。
// 列の幅と行の高さは Boxloom が測った値を書く(行の高さは最小の高さとして扱われる)。
// セルの文字は折り返さずに書き、折り返しはアプリに任せる(表として編集したときに
// 文が流れ直るように)。測った幅には字体の余裕(widthMargin)が入っているので、
// ふつうは測った行の数と同じか少なく折り返される。

package boxloom

import (
	"fmt"
	"math"
	"strings"
)

// AddPicture は画像を box に置く。alt は代わりの文(p:cNvPr の descr)。
// 縦横比は呼ぶ側が合わせる(ここでは引き伸ばしの検査はしない)。
func (p *PPTXBuilder) AddPicture(box LBox, img *imageAsset, alt string) {
	p.Shapes = append(p.Shapes, shape{id: p.allocID(), kind: skPicture, box: box, image: img, text: alt})
}

// AddPictureFilled は画像を box いっぱいに置く:縦横比が box と違えば長い方の両端を
// 同じだけ切り(a:srcRect。引き伸ばさない)、形 prst("rect" | "ellipse")で切り抜く
// (人物の紹介の写真。people.go)。
func (p *PPTXBuilder) AddPictureFilled(box LBox, img *imageAsset, alt, prst string) {
	p.Shapes = append(p.Shapes, shape{id: p.allocID(), kind: skPicture, box: box, image: img, text: alt, prst: prst, picFill: true})
}

// TableCell は表の1つのセル。
type TableCell struct {
	Runs  []TextRun
	Align string // "left" | "right"
	// Items はセルの中の箇条(項目ごとの塊。先頭は印の塊 Mark)。nil でなければ
	// Runs の代わりに項目を段落にし、印は段落の箇条の印(a:buChar)で書く。
	Items [][]TextRun
	// Levels は Items の項目ごとの段(0 が1段目、1 が子の項目。nil なら全部 0)。
	// PPTX では段落の lvl と、親の印の幅の字下げ。
	Levels []int
	// RowSpan は縦に結合する行の数(2 以上で結合の始まり。a:tc の rowSpan)。
	// VMerge は上のセルに結合された続きのセル(a:tc の vMerge。文字は持たない)。
	RowSpan int
	VMerge  bool
	// Fill はこのセルの面(""なら行の面。行の群の名前のセルを濃い面にするなど)。
	Fill string
	// Vert はセルの文を縦書きにする(a:tcPr の vert="eaVert"。字は上から下へ、行は右から左へ)。
	Vert bool
}

// TableDraw は表の描き方(テンプレートが決めた見た目)。
type TableDraw struct {
	ColWidths  []float64
	RowHeights []float64
	Cells      [][]TableCell // 行ごと。1行目は見出しの行
	FontSize   float64
	PadX, PadY float64
	HeaderFill string // 見出しの行の塗り("" は塗らない)
	BodyFill   string // 本体の行の塗り
	RuleColor  string // 行の間の線
	RuleWidth  float64
	// HeaderRuleColor・HeaderRuleWidth は見出しの行の下の線(""なら行の間の線と同じ)。
	HeaderRuleColor string
	HeaderRuleWidth float64
	// TotalRow は最後の行が合計の行か(IR の table の total)。真なら a:tbl の
	// lastRow を立て、その行の上に TotalRuleColor の線を引く。
	TotalRow       bool
	TotalRuleColor string
}

// AddTable は本物の表を box の左上に置く(大きさは列の幅と行の高さの和)。
func (p *PPTXBuilder) AddTable(box LBox, t TableDraw) {
	p.Shapes = append(p.Shapes, shape{id: p.allocID(), kind: skTable, box: box, table: &t})
}

// pictureXML は p:pic。rid は画像の関係の Id。
func pictureXML(s *shape) string {
	prst, crop := "rect", ""
	if s.picFill {
		crop = coverCropXML(s.image, s.box.Width, s.box.Height)
		if s.prst != "" {
			prst = s.prst
		}
	}
	return fmt.Sprintf(`<p:pic>
<p:nvPicPr><p:cNvPr id="%d" name="Picture %d" descr="%s"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>
<p:blipFill><a:blip r:embed="%s"/>%s<a:stretch><a:fillRect/></a:stretch></p:blipFill>
<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="%s"><a:avLst/></a:prstGeom></p:spPr>
</p:pic>
`, s.id, s.id, escapeXML(s.text), s.rid, crop, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), prst)
}

// tableXML は p:graphicFrame の中の a:tbl。
func (p *PPTXBuilder) tableXML(s *shape) string {
	t := s.table
	var b strings.Builder
	w, h := 0.0, 0.0
	for _, cw := range t.ColWidths {
		w += cw
	}
	for _, rh := range t.RowHeights {
		h += rh
	}
	fmt.Fprintf(&b, `<p:graphicFrame>
<p:nvGraphicFramePr><p:cNvPr id="%d" name="Table %d"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>
<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1"%s/><a:tblGrid>`,
		s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(w), ptToEMU(h), lastRowAttr(t.TotalRow))
	for _, cw := range t.ColWidths {
		fmt.Fprintf(&b, `<a:gridCol w="%d"/>`, ptToEMU(cw))
	}
	b.WriteString(`</a:tblGrid>`)
	// セルの段落の既定に字体の実名を書く。LibreOffice は表のセルに presentation.xml の
	// defaultTextStyle を当てず、テーマの参照(+mn-ea)も和文で解決しない(docs/fonts.md の8章)。
	fonts := p.Fonts
	if fonts == nil {
		fonts = defaultFontSet()
	}
	cellLst := `<a:lstStyle><a:lvl1pPr><a:defRPr>` + namedFontRPr(fonts) + `</a:defRPr></a:lvl1pPr></a:lstStyle>`
	noLine := func(tag string) string { return fmt.Sprintf(`<a:%s w="0"><a:noFill/></a:%s>`, tag, tag) }
	rule := func(tag string) string {
		if t.RuleColor == "" {
			return noLine(tag)
		}
		return fmt.Sprintf(`<a:%s w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:%s>`, tag, ptToEMU(t.RuleWidth), hexColor(t.RuleColor), tag)
	}
	dark := func(tag string) string {
		return fmt.Sprintf(`<a:%s w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:%s>`, tag, ptToEMU(t.RuleWidth), hexColor(t.TotalRuleColor), tag)
	}
	totalRule := t.TotalRow && t.TotalRuleColor != ""
	// rowLnB は行 r の下の線。線は行の下だけ(見出しの下と行の間。最後の行の下にも
	// 引いて表の終わりを示す)。合計の行の上の線は濃い色(本体の行と見分ける)で、同じ
	// 線を上の行の下の線としても書く(隣り合うセルの線はアプリによってどちらかが勝つため)。
	rowLnB := func(r int) string {
		if totalRule && r == len(t.Cells)-2 {
			return dark("lnB")
		}
		if r == 0 && t.HeaderRuleColor != "" {
			return fmt.Sprintf(`<a:lnB w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:lnB>`, ptToEMU(t.HeaderRuleWidth), hexColor(t.HeaderRuleColor))
		}
		return rule("lnB")
	}
	for r, row := range t.Cells {
		fmt.Fprintf(&b, `<a:tr h="%d">`, ptToEMU(t.RowHeights[r]))
		fill := t.BodyFill
		if r == 0 {
			fill = t.HeaderFill
		}
		fillOf := func(fill string) string {
			if fill == "" {
				return "<a:noFill/>"
			}
			return fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(fill))
		}
		rowFill := fillOf(fill)
		lnT := noLine("lnT")
		if totalRule && r == len(t.Cells)-1 {
			lnT = dark("lnT")
		}
		for ci, c := range row {
			// 縦に結合したセル(行の群の名前)は、結合の始まりが rowSpan、続きが vMerge。
			// 結合の中の行の境目には線を引かず、結合の終わりの行の下の線が群の境目になる。
			span, lnB := "", rowLnB(r)
			switch {
			case c.RowSpan > 1:
				span = fmt.Sprintf(` rowSpan="%d"`, c.RowSpan)
				lnB = rowLnB(r + c.RowSpan - 1)
			case c.VMerge:
				span = ` vMerge="1"`
				if r+1 < len(t.Cells) && ci < len(t.Cells[r+1]) && t.Cells[r+1][ci].VMerge {
					lnB = noLine("lnB")
				}
			}
			fillXML := rowFill
			if c.Fill != "" {
				fillXML = fillOf(c.Fill)
			}
			// 縦書きは表のセルでは a:tcPr の vert(a:bodyPr の vert は表のセルでは読まれない)。
			vert := ""
			if c.Vert {
				vert = ` vert="eaVert"`
			}
			fmt.Fprintf(&b, `<a:tc%s><a:txBody><a:bodyPr/>%s%s</a:txBody><a:tcPr marL="%d" marR="%d" marT="%d" marB="%d"%s>%s%s%s%s%s</a:tcPr></a:tc>`,
				span, cellLst, p.cellParaXML(c, t.FontSize), ptToEMU(t.PadX), ptToEMU(t.PadX), ptToEMU(t.PadY), ptToEMU(t.PadY), vert,
				noLine("lnL"), noLine("lnR"), lnT, lnB, fillXML)
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString("</a:tbl></a:graphicData></a:graphic>\n</p:graphicFrame>\n")
	return b.String()
}

// cellParaXML はセルの文を1つの段落にする。意味のある改行(br)は a:br。
func (p *PPTXBuilder) cellParaXML(c TableCell, size float64) string {
	if c.Items != nil {
		return p.cellListXML(c, size)
	}
	var b strings.Builder
	sz := int(size * 100)
	if p.Fonts != nil && p.Fonts.FixedLineSpacing {
		fmt.Fprintf(&b, `<a:p><a:pPr algn="%s"><a:lnSpc><a:spcPts val="%d"/></a:lnSpc></a:pPr>`, alignAttr(c.Align), int(math.Round(size*p.Fonts.lineHeight()*100)))
	} else {
		fmt.Fprintf(&b, `<a:p><a:pPr algn="%s"/>`, alignAttr(c.Align))
	}
	rpr := func(r TextRun) string {
		bold := ""
		if r.Bold {
			bold = ` b="1"`
		}
		return fmt.Sprintf(`<a:rPr lang="ja-JP" sz="%d"%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr>`, sz, bold, hexColor(r.Color))
	}
	var lastRun TextRun
	for _, r := range c.Runs {
		lastRun = r
		for i, part := range strings.Split(r.Text, "\n") {
			if i > 0 {
				b.WriteString(`<a:br>` + rpr(r) + `</a:br>`)
			}
			if part != "" {
				b.WriteString(`<a:r>` + rpr(r) + `<a:t>` + escapeXML(part) + `</a:t></a:r>`)
			}
		}
	}
	end := strings.Replace(rpr(lastRun), "<a:rPr", "<a:endParaRPr", 1)
	end = strings.Replace(end, "</a:rPr>", "</a:endParaRPr>", 1)
	if len(c.Runs) == 0 {
		// 文字を持たないセル(縦に結合した続きのセル)は色を書かない。
		end = fmt.Sprintf(`<a:endParaRPr lang="ja-JP" sz="%d"/>`, sz)
	}
	b.WriteString(end + `</a:p>`)
	return b.String()
}

// cellListXML はセルの中の箇条を項目ごとの段落にする。印は段落の箇条の印
// (a:buChar)で、本文は印の幅だけ字下げする(本文の箇条と同じ書き方)。
// 折り返しはアプリに任せる(セルの文は折り返す)。
func (p *PPTXBuilder) cellListXML(c TableCell, size float64) string {
	restore := func() {}
	if p.Fonts != nil {
		restore = useFonts(p.Fonts)
	}
	defer restore()
	var b strings.Builder
	for i, it := range c.Items {
		mark, rest := splitMark(it)
		var props, indent, lvl string
		if p.Fonts != nil && p.Fonts.FixedLineSpacing {
			props = fmt.Sprintf(`<a:lnSpc><a:spcPts val="%d"/></a:lnSpc>`, int(math.Round(size*p.Fonts.lineHeight()*100)))
		}
		if mark != nil {
			var bullet string
			bullet, indent = p.bulletXML(mark, size, c.indent(i, size))
			props += bullet
		}
		if i < len(c.Levels) && c.Levels[i] > 0 {
			lvl = fmt.Sprintf(` lvl="%d"`, c.Levels[i])
		}
		fmt.Fprintf(&b, `<a:p><a:pPr%s%s algn="%s">%s</a:pPr>`, indent, lvl, alignAttr(c.Align), props)
		for _, r := range rest {
			for i, part := range strings.Split(r.Text, "\n") {
				if i > 0 {
					fmt.Fprintf(&b, `<a:br><a:rPr lang="ja-JP" sz="%d"/></a:br>`, int(size*100))
				}
				if part != "" {
					q := r
					q.Text = part
					b.WriteString(runXML(q, size))
				}
			}
		}
		b.WriteString(`</a:p>`)
	}
	return b.String()
}

// --- 画像の部品(ppt/media) ---

// mediaPart は文書に入れる画像の部品1つ。
type mediaPart struct {
	name string // imageN.png など(ppt/media/ の下)
	data []byte
}

// assignMedia は全スライドの画像に部品の名前と関係の Id を割り当て、入れる部品の
// 一覧を返す。同じ中身の画像は1つの部品にまとめる。rid は各スライドで rId10 から
// (レイアウト rId1・ノート rId2 と重ならないように)。
func assignMedia(slides []*PPTXBuilder) []mediaPart {
	var parts []mediaPart
	byData := map[string]string{}
	add := func(img *imageAsset) string {
		name, ok := byData[string(img.Data)]
		if !ok {
			name = fmt.Sprintf("image%d.%s", len(parts)+1, img.Format)
			byData[string(img.Data)] = name
			parts = append(parts, mediaPart{name: name, data: img.Data})
		}
		return name
	}
	for _, sl := range slides {
		ridByName := map[string]string{}
		if sl.BackgroundImage != nil {
			sl.bgMedia = add(sl.BackgroundImage)
			sl.bgRID = fmt.Sprintf("rId%d", 10+len(ridByName))
			ridByName[sl.bgMedia] = sl.bgRID
		}
		for i := range sl.Shapes {
			s := &sl.Shapes[i]
			if s.kind != skPicture || s.image == nil {
				continue
			}
			name := add(s.image)
			s.media = name
			rid, ok := ridByName[name]
			if !ok {
				rid = fmt.Sprintf("rId%d", 10+len(ridByName))
				ridByName[name] = rid
			}
			s.rid = rid
		}
	}
	return parts
}

// mediaContentTypes は使った画像の形式の Default(使わなければ空)。
func mediaContentTypes(parts []mediaPart) string {
	var png, jpeg bool
	for _, m := range parts {
		png = png || strings.HasSuffix(m.name, ".png")
		jpeg = jpeg || strings.HasSuffix(m.name, ".jpeg")
	}
	out := ""
	if png {
		out += "\n" + `<Default Extension="png" ContentType="image/png"/>`
	}
	if jpeg {
		out += "\n" + `<Default Extension="jpeg" ContentType="image/jpeg"/>`
	}
	return out
}

// slideImageRels はスライドの画像の関係(同じ画像は1つ)。
func slideImageRels(p *PPTXBuilder) string {
	var b strings.Builder
	seen := map[string]bool{}
	if p.BackgroundImage != nil && p.bgRID != "" {
		seen[p.bgRID] = true
		fmt.Fprintf(&b, "\n"+`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, p.bgRID, p.bgMedia)
	}
	for _, s := range p.Shapes {
		if s.kind != skPicture || s.rid == "" || seen[s.rid] {
			continue
		}
		seen[s.rid] = true
		fmt.Fprintf(&b, "\n"+`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, s.rid, s.media)
	}
	return b.String()
}

func lastRowAttr(on bool) string {
	if on {
		return ` lastRow="1"`
	}
	return ""
}

// coverCropXML は全面に敷く写真の a:srcRect:写真の縦横比がページと違えば、長い方の
// 両端を同じだけ切ってページを覆う(引き伸ばさない)。同じなら何も書かない。
func coverCropXML(img *imageAsset, pageW, pageH float64) string {
	if img == nil || img.W <= 0 || img.H <= 0 || pageW <= 0 || pageH <= 0 {
		return ""
	}
	page, pic := pageW/pageH, img.aspect()
	// 切る割合(片側。1/1000 パーセント = 100000 が全体)。
	switch {
	case pic > page*1.0001:
		c := int(math.Round((1 - page/pic) / 2 * 100000))
		return fmt.Sprintf(`<a:srcRect l="%d" r="%d"/>`, c, c)
	case pic < page/1.0001:
		c := int(math.Round((1 - pic/page) / 2 * 100000))
		return fmt.Sprintf(`<a:srcRect t="%d" b="%d"/>`, c, c)
	}
	return ""
}
