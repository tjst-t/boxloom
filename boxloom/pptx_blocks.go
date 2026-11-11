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

// TableCell は表の1つのセル。
type TableCell struct {
	Runs  []TextRun
	Align string // "left" | "right"
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
	return fmt.Sprintf(`<p:pic>
<p:nvPicPr><p:cNvPr id="%d" name="Picture %d" descr="%s"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>
<p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>
<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
</p:pic>
`, s.id, s.id, escapeXML(s.text), s.rid, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height))
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
	noLine := func(tag string) string { return fmt.Sprintf(`<a:%s w="0"><a:noFill/></a:%s>`, tag, tag) }
	rule := func(tag string) string {
		if t.RuleColor == "" {
			return noLine(tag)
		}
		return fmt.Sprintf(`<a:%s w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:%s>`, tag, ptToEMU(t.RuleWidth), hexColor(t.RuleColor), tag)
	}
	for r, row := range t.Cells {
		fmt.Fprintf(&b, `<a:tr h="%d">`, ptToEMU(t.RowHeights[r]))
		fill := t.BodyFill
		if r == 0 {
			fill = t.HeaderFill
		}
		fillXML := "<a:noFill/>"
		if fill != "" {
			fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(fill))
		}
		// 線は行の下だけ(見出しの下と行の間。最後の行の下にも引いて表の終わりを示す)。
		lnT := noLine("lnT")
		lnB := rule("lnB")
		if t.TotalRow && t.TotalRuleColor != "" {
			// 合計の行の上の線は濃い色(本体の行と見分ける)。同じ線を上の行の下の線
			// としても書く(隣り合うセルの線はアプリによってどちらかが勝つため)。
			dark := func(tag string) string {
				return fmt.Sprintf(`<a:%s w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:%s>`, tag, ptToEMU(t.RuleWidth), hexColor(t.TotalRuleColor), tag)
			}
			switch r {
			case len(t.Cells) - 1:
				lnT = dark("lnT")
			case len(t.Cells) - 2:
				lnB = dark("lnB")
			}
		}
		for _, c := range row {
			fmt.Fprintf(&b, `<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>%s</a:txBody><a:tcPr marL="%d" marR="%d" marT="%d" marB="%d">%s%s%s%s%s</a:tcPr></a:tc>`,
				p.cellParaXML(c, t.FontSize), ptToEMU(t.PadX), ptToEMU(t.PadX), ptToEMU(t.PadY), ptToEMU(t.PadY),
				noLine("lnL"), noLine("lnR"), lnT, lnB, fillXML)
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString("</a:tbl></a:graphicData></a:graphic>\n</p:graphicFrame>\n")
	return b.String()
}

// cellParaXML はセルの文を1つの段落にする。意味のある改行(br)は a:br。
func (p *PPTXBuilder) cellParaXML(c TableCell, size float64) string {
	var b strings.Builder
	sz := int(size * 100)
	if p.Fonts != nil && p.Fonts.FixedLineSpacing {
		fmt.Fprintf(&b, `<a:p><a:pPr algn="%s"><a:lnSpc><a:spcPts val="%d"/></a:lnSpc></a:pPr>`, alignAttr(c.Align), int(math.Round(size*lineHeightMult*100)))
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
	b.WriteString(end + `</a:p>`)
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
	for _, sl := range slides {
		ridByName := map[string]string{}
		for i := range sl.Shapes {
			s := &sl.Shapes[i]
			if s.kind != skPicture || s.image == nil {
				continue
			}
			name, ok := byData[string(s.image.Data)]
			if !ok {
				name = fmt.Sprintf("image%d.%s", len(parts)+1, s.image.Format)
				byData[string(s.image.Data)] = name
				parts = append(parts, mediaPart{name: name, data: s.image.Data})
			}
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
