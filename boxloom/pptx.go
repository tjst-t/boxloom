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
	"math"
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
	skShape     // 図形(矩形・角丸・楕円)。枠線と中の文字を持てる(shapes サブシステム)
	skConnector // 矢印を付けられる線。図形の接続点に繋げられる(shapes サブシステム)
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

	// skShape
	prst   string  // rect | roundRect | ellipse
	insetX float64 // 文字の枠の内側の余白
	insetY float64

	// skConnector
	arrowEnd      bool // to の端に矢印(a:tailEnd)
	stID, stIdx   int  // 始点を繋ぐ図形の id と接続点の番号(id 0 は繋がない)
	endID, endIdx int  // 終点を繋ぐ図形の id と接続点の番号
}

// PPTXBuilder は1枚のスライドを表す。Canvas(box_engine.pyのcanvas葉ノード)の
// レンダリング関数はこれを受け取り、確定したboxの中に自由に図形を追加する。
type PPTXBuilder struct {
	WidthPt, HeightPt float64
	Shapes            []shape
	Notes             string
	// Fonts は書き出す字体(テーマの a:latin・a:ea)と行の送りの決め方。
	// 作ったときの activeFonts(測った字体)を写す。
	Fonts  *FontSet
	nextID int
}

func NewPPTXBuilder(widthPt, heightPt float64) *PPTXBuilder {
	return &PPTXBuilder{WidthPt: widthPt, HeightPt: heightPt, Fonts: activeFonts, nextID: 2}
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

// ShapeOpts は AddShape の指定。Fill・Outline が "" なら塗り・枠線なし。
type ShapeOpts struct {
	Prst           string // rect | roundRect | ellipse
	Fill           string
	Outline        string
	OutlineWidthPt float64
	Radius         float64 // roundRect の角
	Runs           []TextRun
	FontSize       float64
	InsetX, InsetY float64 // 文字の枠(楕円なら内接する矩形)の内側の余白
}

// AddShape は図形を足し、その id(コネクタの接続に使う)を返す。中の文字は
// 図形の文字の枠の幅で事前に折り返し、中央に置く。
func (p *PPTXBuilder) AddShape(box LBox, o ShapeOpts) int {
	id := p.allocID()
	p.Shapes = append(p.Shapes, shape{
		id: id, kind: skShape, box: box, prst: o.Prst, fill: o.Fill, radius: o.Radius,
		hasOutline: o.Outline != "", outlineColor: o.Outline, outlineWidthPt: o.OutlineWidthPt,
		runs: o.Runs, text: runsText(o.Runs), fontSize: o.FontSize, insetX: o.InsetX, insetY: o.InsetY,
		align: "center", anchor: "middle",
	})
	return id
}

// ConnectorEnd は線の端を図形の接続点に繋ぐ指定(ShapeID 0 は繋がない)。
type ConnectorEnd struct{ ShapeID, Site int }

// AddConnector は本物のコネクタ(p:cxnSp)を足す。st・end を渡すと a:stCxn・
// a:endCxn で図形に繋ぐので、PowerPoint で図形を動かすと線が付いてくる。
// arrowEnd なら終点に矢印(a:tailEnd)を付ける。
func (p *PPTXBuilder) AddConnector(x1, y1, x2, y2, widthPt float64, color string, arrowEnd bool, st, end ConnectorEnd) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skConnector,
		x1: x1, y1: y1, x2: x2, y2: y2, lineColor: color, lineWidthPt: widthPt,
		arrowEnd: arrowEnd, stID: st.ShapeID, stIdx: st.Site, endID: end.ShapeID, endIdx: end.Site,
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

// textLines は文字の図形を書く行に分ける(自前で折り返し、wrap="none" で
// 書くため)。折り返しはこのスライドを組んだときの字体の一式で測る。
func (p *PPTXBuilder) textLines(s *shape) [][]TextRun {
	if p.Fonts != nil {
		defer useFonts(p.Fonts)()
	}
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
	return lines
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
		lines := p.textLines(s)
		paras := p.parasXML(s, lines)
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
<a:noFill/>
</p:spPr>
<p:txBody><a:bodyPr wrap="none" anchor="%s" lIns="0" tIns="0" rIns="0" bIns="0"><a:noAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), anchorAttr(s.anchor), paras)

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

	case skShape:
		return p.shapeWithTextXML(s)

	case skConnector:
		return connectorXML(s)
	}
	return ""
}

// parasXML は折り返し済みの行を段落(a:p)にする。fonts.fixedLineSpacing なら
// 行の送りを測ったとおり(字の大きさ×lineHeightMult)に pt で書く。
func (p *PPTXBuilder) parasXML(s *shape, lines [][]TextRun) string {
	var paras strings.Builder
	szHundredths := int(s.fontSize * 100)
	for _, line := range lines {
		if len(line) == 0 {
			line = []TextRun{{Color: s.color, Bold: s.bold}}
		}
		if p.Fonts != nil && p.Fonts.FixedLineSpacing {
			paras.WriteString(fmt.Sprintf(`<a:p><a:pPr algn="%s"><a:lnSpc><a:spcPts val="%d"/></a:lnSpc></a:pPr>`,
				alignAttr(s.align), int(math.Round(s.fontSize*lineHeightMult*100))))
		} else {
			paras.WriteString(fmt.Sprintf(`<a:p><a:pPr algn="%s"/>`, alignAttr(s.align)))
		}
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
	return paras.String()
}

// shapeTextRect は図形の文字の枠(prstGeom の文字の矩形)。楕円は内接する矩形。
func shapeTextRect(prst string, box LBox) LBox {
	if prst != "ellipse" {
		return box
	}
	const k = 0.70710678 // cos 45°
	w, h := box.Width*k, box.Height*k
	return LBox{X: box.X + (box.Width-w)/2, Y: box.Y + (box.Height-h)/2, Width: w, Height: h}
}

// shapeWithTextXML は塗り・枠線・中の文字を持つ図形(skShape)。
func (p *PPTXBuilder) shapeWithTextXML(s *shape) string {
	adj := "<a:avLst/>"
	if s.prst == "roundRect" {
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
		lnXML = fmt.Sprintf(`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`,
			ptToEMU(s.outlineWidthPt), hexColor(s.outlineColor))
	}
	paras := "<a:p/>"
	if len(s.runs) > 0 {
		tr := shapeTextRect(s.prst, s.box)
		restore := func() {}
		if p.Fonts != nil {
			restore = useFonts(p.Fonts)
		}
		lines := wrapRuns(s.runs, s.fontSize, tr.Width-2*s.insetX)
		restore()
		paras = p.parasXML(s, lines)
	}
	return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="%s">%s</a:prstGeom>
%s
%s
</p:spPr>
<p:txBody><a:bodyPr wrap="none" anchor="%s" lIns="%d" tIns="%d" rIns="%d" bIns="%d"><a:noAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), s.prst, adj, fillXML, lnXML,
		anchorAttr(s.anchor), ptToEMU(s.insetX), ptToEMU(s.insetY), ptToEMU(s.insetX), ptToEMU(s.insetY), paras)
}

// connectorXML は本物のコネクタ(p:cxnSp)。図形に繋ぐなら a:stCxn・a:endCxn を書く。
func connectorXML(s *shape) string {
	x0, y0 := math.Min(s.x1, s.x2), math.Min(s.y1, s.y2)
	w, h := math.Abs(s.x2-s.x1), math.Abs(s.y2-s.y1)
	flip := ""
	if s.x2 < s.x1 {
		flip += ` flipH="1"`
	}
	if s.y2 < s.y1 {
		flip += ` flipV="1"`
	}
	cxn := ""
	if s.stID > 0 {
		cxn += fmt.Sprintf(`<a:stCxn id="%d" idx="%d"/>`, s.stID, s.stIdx)
	}
	if s.endID > 0 {
		cxn += fmt.Sprintf(`<a:endCxn id="%d" idx="%d"/>`, s.endID, s.endIdx)
	}
	tail := ""
	if s.arrowEnd {
		tail = `<a:tailEnd type="triangle" w="med" len="med"/>`
	}
	return fmt.Sprintf(`<p:cxnSp>
<p:nvCxnSpPr><p:cNvPr id="%d" name="Connector %d"/><p:cNvCxnSpPr>%s</p:cNvCxnSpPr><p:nvPr/></p:nvCxnSpPr>
<p:spPr>
<a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom>
<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>
</p:spPr>
</p:cxnSp>
`, s.id, s.id, cxn, flip, ptToEMU(x0), ptToEMU(y0), ptToEMU(w), ptToEMU(h), ptToEMU(s.lineWidthPt), hexColor(s.lineColor), tail)
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

	// テーマの字体:欧文(a:latin)と和文(a:ea)を分けて書く。a:ea が無いと
	// PowerPoint は和文を既定の字体で描き、測った字体と食い違う(docs/fonts.md)。
	fonts := p.Fonts
	if fonts == nil {
		fonts = defaultFontSet()
	}
	fontRefs := fmt.Sprintf(`<a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface=""/>`,
		escapeXML(fonts.Latin.Family), escapeXML(fonts.EastAsian.Family))

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
<a:majorFont>` + fontRefs + `</a:majorFont>
<a:minorFont>` + fontRefs + `</a:minorFont>
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
<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + notesMasterPlaceholders(p.WidthPt, p.HeightPt) + `</p:spTree></p:cSld>
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
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Slide Image Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr/>
<p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody>
</p:sp>
</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:notes>`, notesParagraphs(p.Notes))
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

// notesParagraphs はノートの文を改行ごとに1段落にする(空の行は空の段落)。
func notesParagraphs(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			b.WriteString(`<a:p><a:endParaRPr lang="ja-JP"/></a:p>`)
			continue
		}
		b.WriteString(`<a:p><a:r><a:t>` + escapeXML(line) + `</a:t></a:r></a:p>`)
	}
	return b.String()
}

// notesMasterPlaceholders はノートのページ(縦長。幅=スライドの高さ、高さ=スライドの幅)
// に、上にスライドの縮図・下にノートの文の置き場を置く。これが無いと LibreOffice の
// ノートのページに何も出ない。
func notesMasterPlaceholders(slideW, slideH float64) string {
	pageW, pageH := slideH, slideW
	imgW := pageW * 0.84
	imgH := imgW * slideH / slideW
	x := (pageW - imgW) / 2
	y := pageH * 0.07
	bodyY := y + imgH + pageH*0.04
	bodyH := pageH*0.95 - bodyY
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:sp>`+
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" sz="quarter" idx="1"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ja-JP"/></a:p></p:txBody></p:sp>`,
		ptToEMU(x), ptToEMU(y), ptToEMU(imgW), ptToEMU(imgH),
		ptToEMU(x), ptToEMU(bodyY), ptToEMU(imgW), ptToEMU(bodyH))
}
