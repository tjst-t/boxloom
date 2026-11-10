// PPTXBuilder: python-pptx を使わず、OOXML(zip + XML)を直接組み立てる最小実装。
//
// 自由に使える成熟したGo製pptxライブラリが無い(unidoc/unioffice は商用ライセンス、
// 他のOSS候補はshape作成すら未実装)ため、Goの標準ライブラリ(archive/zip,
// encoding/xml相当の文字列組み立て)だけでOOXMLを手書きする。サポート範囲は
// このデモが実際に必要とする最小限(角丸/矩形の塗り、テキストボックス、直線/
// コネクタ、ノート)に絞っている。
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
)

type shapeKind int

const (
	skRect shapeKind = iota
	skRoundRect
	skOval
	skText
	skLine
)

type shape struct {
	id   int
	kind shapeKind

	box    LBox
	fill   string // "" = 塗りなし
	radius float64

	// 矩形の枠線(本物のrender_gantt()にある「破線の未確定区間」等で使う)。
	hasOutline     bool
	outlineColor   string
	outlineDash    string // "" | "dash"
	outlineWidthPt float64

	text     string
	fontSize float64
	bold     bool
	color    string
	align    string
	anchor   string
	runs     []TextRun // 空でなければ text・color・bold の代わりに使う
	wrap     bool      // true: このGo版の自前ロジックで幅に収まるよう折り返す。false: "\n"のみを境界として扱い、幅では折り返さない(本物のwrap=Falseに相当)。

	x1, y1, x2, y2 float64
	lineColor      string
	lineWidthPt    float64
	dash           string // "" | "dash"
}

// PPTXBuilder は1枚のスライドを表す。Canvas(box_engine.pyのcanvas葉ノード)の
// レンダリング関数はこれを受け取り、確定したboxの中に自由に図形を追加する。
type PPTXBuilder struct {
	WidthPt, HeightPt float64
	Shapes            []shape
	Notes             string
	nextID            int
}

func NewPPTXBuilder(widthPt, heightPt float64) *PPTXBuilder {
	return &PPTXBuilder{WidthPt: widthPt, HeightPt: heightPt, nextID: 2}
}

func (p *PPTXBuilder) allocID() int {
	id := p.nextID
	p.nextID++
	return id
}

// AddRect は塗り付きの矩形(radius>0なら角丸)を追加する。枠線なし。
func (p *PPTXBuilder) AddRect(box LBox, fill string, radius float64) {
	p.AddRectFull(box, fill, radius, "", "", 0)
}

// AddRectFull は塗り・角丸・枠線すべてを指定できる矩形(本物の_add_rect()の
// フル機能版に相当)。fill=""なら塗りなし。outlineColor=""なら枠線なし。
func (p *PPTXBuilder) AddRectFull(box LBox, fill string, radius float64, outlineColor, outlineDash string, outlineWidthPt float64) {
	k := skRect
	if radius > 0 {
		k = skRoundRect
	}
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: k, box: box, fill: fill, radius: radius,
		hasOutline: outlineColor != "", outlineColor: outlineColor, outlineDash: outlineDash, outlineWidthPt: outlineWidthPt,
	})
}

// AddDot は塗りつぶされた円(本物の_add_dot()に相当。軸目盛りの点やopen_endの
// 連続ドットに使う)を追加する。
func (p *PPTXBuilder) AddDot(cx, cy, r float64, color string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skOval,
		box: LBox{X: cx - r, Y: cy - r, Width: r * 2, Height: r * 2}, fill: color,
	})
}

// AddTextBox はテキストボックスを追加する(wrap=True相当)。テキストはこちら側で
// 幅に収まるよう事前に折り返し、PowerPoint/LibreOffice側の自動折り返しには
// 依存しない(box_engine.pyが計測した高さと実際の表示を一致させるため)。
func (p *PPTXBuilder) AddTextBox(box LBox, text string, fontSize float64, color string, bold bool, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: text, fontSize: fontSize, bold: bold, color: color, align: align, anchor: anchor, wrap: true,
	})
}

// AddTextBoxNoWrap はwrap=False相当。幅による折り返しは行わない(“\n”のみ
// 段落境界として扱う)。本物のrender_gantt()で凡例・軸目盛り・行ラベル・
// セグメントラベル等、固定幅に収まる前提の短いラベルに使われているのと同じ。
func (p *PPTXBuilder) AddTextBoxNoWrap(box LBox, text string, fontSize float64, color string, bold bool, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: text, fontSize: fontSize, bold: bold, color: color, align: align, anchor: anchor, wrap: false,
	})
}

// AddRichTextBox は装飾つきの文字(wrap=True 相当)。
func (p *PPTXBuilder) AddRichTextBox(box LBox, runs []TextRun, fontSize float64, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: runsText(runs), runs: runs, fontSize: fontSize, align: align, anchor: anchor, wrap: true,
	})
}

// AddLine は直線/コネクタを追加する。dash は "" (実線) または "dash"。
func (p *PPTXBuilder) AddLine(x1, y1, x2, y2, widthPt float64, color, dash string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skLine,
		x1: x1, y1: y1, x2: x2, y2: y2, lineColor: color, lineWidthPt: widthPt, dash: dash,
	})
}

func (p *PPTXBuilder) SetNotes(text string) { p.Notes = text }

// --- XML組み立てヘルパー ---

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func ptToEMU(pt float64) int64 { return int64(pt * 12700) }

func hexColor(c string) string { return strings.TrimPrefix(strings.ToUpper(c), "#") }

func anchorAttr(a string) string {
	switch a {
	case "middle":
		return "ctr"
	case "bottom":
		return "b"
	default:
		return "t"
	}
}

func alignAttr(a string) string {
	switch a {
	case "center":
		return "ctr"
	case "right":
		return "r"
	default:
		return "l"
	}
}

func (p *PPTXBuilder) shapeXML(s *shape) string {
	switch s.kind {
	case skRect, skRoundRect:
		prst := "rect"
		adj := ""
		if s.kind == skRoundRect {
			prst = "roundRect"
			// 角丸の比率(短辺に対するradius)。avLstのgdは全体の縦横比に対する割合。
			minSide := s.box.Width
			if s.box.Height < minSide {
				minSide = s.box.Height
			}
			ratio := 0.0
			if minSide > 0 {
				ratio = s.radius / minSide
				if ratio > 0.5 {
					ratio = 0.5
				}
			}
			adj = fmt.Sprintf(`<a:avLst><a:gd name="adj" fmla="val %d"/></a:avLst>`, int(ratio*100000))
		}
		fillXML := "<a:noFill/>"
		if s.fill != "" {
			fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(s.fill))
		}
		lnXML := "<a:ln><a:noFill/></a:ln>"
		if s.hasOutline {
			dashXML := ""
			if s.outlineDash == "dash" {
				dashXML = `<a:prstDash val="dash"/>`
			}
			lnXML = fmt.Sprintf(`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>`,
				ptToEMU(s.outlineWidthPt), hexColor(s.outlineColor), dashXML)
		}
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="%s">%s</a:prstGeom>
%s
%s
</p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), prst, adj, fillXML, lnXML)

	case skOval:
		fillXML := "<a:noFill/>"
		if s.fill != "" {
			fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(s.fill))
		}
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Dot %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
%s
<a:ln><a:noFill/></a:ln>
</p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), fillXML)

	case skText:
		var lines [][]TextRun
		switch {
		case len(s.runs) > 0:
			lines = wrapRuns(s.runs, s.fontSize, s.box.Width)
		case s.wrap:
			for _, l := range wrapText(s.text, s.fontSize, s.box.Width, s.bold) {
				lines = append(lines, []TextRun{{Text: l, Color: s.color, Bold: s.bold}})
			}
		default:
			for _, l := range strings.Split(s.text, "\n") {
				lines = append(lines, []TextRun{{Text: l, Color: s.color, Bold: s.bold}})
			}
		}
		var paras strings.Builder
		szHundredths := int(s.fontSize * 100)
		for _, line := range lines {
			if len(line) == 0 {
				line = []TextRun{{Color: s.color, Bold: s.bold}}
			}
			paras.WriteString(fmt.Sprintf(`<a:p><a:pPr algn="%s"/>`, alignAttr(s.align)))
			for _, r := range line {
				boldAttr := ""
				if r.Bold {
					boldAttr = ` b="1"`
				}
				paras.WriteString(fmt.Sprintf(`<a:r><a:rPr lang="ja-JP" sz="%d"%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r>`,
					szHundredths, boldAttr, hexColor(r.Color), escapeXML(r.Text)))
			}
			paras.WriteString(`</a:p>`)
		}
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
<a:noFill/>
</p:spPr>
<p:txBody><a:bodyPr wrap="none" anchor="%s" lIns="0" tIns="0" rIns="0" bIns="0"><a:noAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), anchorAttr(s.anchor), paras.String())

	case skLine:
		dashXML := ""
		if s.dash == "dash" {
			dashXML = `<a:prstDash val="dash"/>`
		}
		x1, y1, x2, y2 := s.x1, s.y1, s.x2, s.y2
		x0, y0 := x1, y1
		if x2 < x0 {
			x0 = x2
		}
		if y2 < y0 {
			y0 = y2
		}
		w, h := x2-x1, y2-y1
		flipH, flipV := "", ""
		if w < 0 {
			flipH = ` flipH="1"`
			w = -w
		}
		if h < 0 {
			flipV = ` flipV="1"`
			h = -h
		}
		return fmt.Sprintf(`<p:cxnSp>
<p:nvCxnSpPr><p:cNvPr id="%d" name="Line %d"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>
<p:spPr>
<a:xfrm%s%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="line"><a:avLst/></a:prstGeom>
<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>
</p:spPr>
</p:cxnSp>
`, s.id, s.id, flipH, flipV, ptToEMU(x0), ptToEMU(y0), ptToEMU(w), ptToEMU(h), ptToEMU(s.lineWidthPt), hexColor(s.lineColor), dashXML)
	}
	return ""
}

// Save はこの1枚だけの pptx を書き出す。
func (p *PPTXBuilder) Save(path string) error {
	return SaveDeck(path, []*PPTXBuilder{p}, "")
}

// SaveDeck は複数枚のスライドを1つの pptx(zipコンテナ)に書き出す。大きさは
// 1枚目のものを使う。title は文書の題(空なら "boxloom")。
func SaveDeck(path string, slides []*PPTXBuilder, title string) error {
	if len(slides) == 0 {
		return fmt.Errorf("SaveDeck: no slides")
	}
	p := slides[0]
	n := len(slides)
	if title == "" {
		title = "boxloom"
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	hasNotes := false
	for _, sl := range slides {
		if sl.Notes != "" {
			hasNotes = true
		}
	}
	var slideTypes, notesTypes, sldIDs, slideRelsList strings.Builder
	for i, sl := range slides {
		slideTypes.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1))
		if sl.Notes != "" {
			notesTypes.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/notesSlides/notesSlide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>`, i+1))
		}
		sldIDs.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+2))
		slideRelsList.WriteString(fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+2, i+1))
	}

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` + slideTypes.String() + `
<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`
	if hasNotes {
		contentTypes += `
<Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"/>` + notesTypes.String()
	}
	contentTypes += `
</Types>`

	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

	coreXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>` + escapeXML(title) + `</dc:title><dc:creator>boxloom</dc:creator>
</cp:coreProperties>`

	appXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
<Application>boxloom</Application>
</Properties>`

	notesSz := fmt.Sprintf(`<p:notesSz cx="%d" cy="%d"/>`, ptToEMU(p.HeightPt), ptToEMU(p.WidthPt))
	presentation := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
<p:sldIdLst>%s</p:sldIdLst>
<p:sldSz cx="%d" cy="%d"/>
%s
</p:presentation>`, sldIDs.String(), ptToEMU(p.WidthPt), ptToEMU(p.HeightPt), notesSz)

	presRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` + slideRelsList.String() + fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`, n+2)
	if hasNotes {
		presRels += fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>`, n+3)
	}
	presRels += `
</Relationships>`

	theme := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="boxloom">
<a:themeElements>
<a:clrScheme name="boxloom">
<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
<a:dk2><a:srgbClr val="0033CC"/></a:dk2>
<a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
<a:accent1><a:srgbClr val="0033CC"/></a:accent1>
<a:accent2><a:srgbClr val="4472C4"/></a:accent2>
<a:accent3><a:srgbClr val="70AD47"/></a:accent3>
<a:accent4><a:srgbClr val="FFC000"/></a:accent4>
<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5>
<a:accent6><a:srgbClr val="A5A5A5"/></a:accent6>
<a:hlink><a:srgbClr val="0563C1"/></a:hlink>
<a:folHlink><a:srgbClr val="954F72"/></a:folHlink>
</a:clrScheme>
<a:fontScheme name="boxloom">
<a:majorFont><a:latin typeface="Noto Sans CJK JP"/></a:majorFont>
<a:minorFont><a:latin typeface="Noto Sans CJK JP"/></a:minorFont>
</a:fontScheme>
<a:fmtScheme name="boxloom">
<a:fillStyleLst><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:fillStyleLst>
<a:lnStyleLst><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln></a:lnStyleLst>
<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
<a:bgFillStyleLst><a:solidFill><a:schemeClr val="lt1"/></a:solidFill><a:solidFill><a:schemeClr val="lt1"/></a:solidFill><a:solidFill><a:schemeClr val="lt1"/></a:solidFill></a:bgFillStyleLst>
</a:fmtScheme>
</a:themeElements>
</a:theme>`

	master := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>
<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree>
</p:cSld>
<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
</p:sldMaster>`

	masterRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

	layout := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">
<p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`

	layoutRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

	if err := write("[Content_Types].xml", contentTypes); err != nil {
		return err
	}
	if err := write("_rels/.rels", rootRels); err != nil {
		return err
	}
	if err := write("docProps/core.xml", coreXML); err != nil {
		return err
	}
	if err := write("docProps/app.xml", appXML); err != nil {
		return err
	}
	if err := write("ppt/presentation.xml", presentation); err != nil {
		return err
	}
	if err := write("ppt/_rels/presentation.xml.rels", presRels); err != nil {
		return err
	}
	if err := write("ppt/theme/theme1.xml", theme); err != nil {
		return err
	}
	if err := write("ppt/slideMasters/slideMaster1.xml", master); err != nil {
		return err
	}
	if err := write("ppt/slideMasters/_rels/slideMaster1.xml.rels", masterRels); err != nil {
		return err
	}
	if err := write("ppt/slideLayouts/slideLayout1.xml", layout); err != nil {
		return err
	}
	if err := write("ppt/slideLayouts/_rels/slideLayout1.xml.rels", layoutRels); err != nil {
		return err
	}
	for i, sl := range slides {
		if err := writeSlide(write, i+1, sl); err != nil {
			return err
		}
	}

	if hasNotes {
		notesMaster := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>
<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
</p:notesMaster>`
		notesMasterRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
		if err := write("ppt/notesMasters/notesMaster1.xml", notesMaster); err != nil {
			return err
		}
		if err := write("ppt/notesMasters/_rels/notesMaster1.xml.rels", notesMasterRels); err != nil {
			return err
		}
	}

	return zw.Close()
}

// writeSlide は num 枚目のスライド(と、あればノート)を書く。
func writeSlide(write func(name, content string) error, num int, p *PPTXBuilder) error {
	var shapesXML strings.Builder
	for i := range p.Shapes {
		shapesXML.WriteString(p.shapeXML(&p.Shapes[i]))
	}

	slide := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr/>
%s
</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`, shapesXML.String())

	slideRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`
	if p.Notes != "" {
		slideRels += fmt.Sprintf(`
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide%d.xml"/>`, num)
	}
	slideRels += `
</Relationships>`

	if err := write(fmt.Sprintf("ppt/slides/slide%d.xml", num), slide); err != nil {
		return err
	}
	if err := write(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", num), slideRels); err != nil {
		return err
	}
	if p.Notes == "" {
		return nil
	}
	notesSlide := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr/>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>%s</a:t></a:r></a:p></p:txBody>
</p:sp>
</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:notes>`, escapeXML(p.Notes))
	notesSlideRels := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="../notesMasters/notesMaster1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide%d.xml"/>
</Relationships>`, num)

	if err := write(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", num), notesSlide); err != nil {
		return err
	}
	return write(fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", num), notesSlideRels)
}
