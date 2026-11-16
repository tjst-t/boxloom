import sys
from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE, MSO_CONNECTOR
from pptx.chart.data import CategoryChartData
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION
from pptx.oxml.ns import qn
from lxml import etree

# ---- common helpers ----
FONT = "Noto Sans CJK JP"
INK = RGBColor(0x1F, 0x29, 0x33)
SUB = RGBColor(0x55, 0x61, 0x6D)
MUTED = RGBColor(0x8A, 0x94, 0x9E)
LINE = RGBColor(0xD3, 0xD9, 0xDF)
PALE = RGBColor(0xF1, 0xF4, 0xF6)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
GRAY = RGBColor(0xB4, 0xBC, 0xC4)
WARN = RGBColor(0xC2, 0x41, 0x1C)
SW, SH = 13.333, 7.5
M = 0.6

ACCENT = RGBColor(0x2C, 0x4A, 0x8A)
ACCENT_PALE = RGBColor(0xE6, 0xEB, 0xF5)
DECK = "留学生の生活の支え"
TOTAL = 4


def nostyle(sh):
    st = sh._element.find(qn("p:style"))
    if st is not None:
        sh._element.remove(st)
    return sh


def font(f, size=None, color=None, bold=None):
    if size is not None:
        f.size = Pt(size)
    if bold is not None:
        f.bold = bold
    if color is not None:
        f.color.rgb = color
    f.name = FONT
    rPr = f._rPr
    latin = rPr.find(qn("a:latin"))
    if rPr.find(qn("a:ea")) is None:
        ea = etree.Element(qn("a:ea"))
        ea.set("typeface", FONT)
        latin.addnext(ea)


def para_fmt(p, align=None, space_before=0, space_after=0, line=None, bullet=None, level=0):
    if align is not None:
        p.alignment = align
    p.space_before = Pt(space_before)
    p.space_after = Pt(space_after)
    if line:
        p.line_spacing = line
    pPr = p._p.get_or_add_pPr()
    if bullet:
        ind = 0.22 + 0.22 * level
        pPr.set("marL", str(int(Inches(ind))))
        pPr.set("indent", str(int(-Inches(0.2))))
        for tag in ("a:buNone", "a:buChar", "a:buFont"):
            for el in pPr.findall(qn(tag)):
                pPr.remove(el)
        buf = etree.SubElement(pPr, qn("a:buFont"))
        buf.set("typeface", "Arial")
        bu = etree.SubElement(pPr, qn("a:buChar"))
        bu.set("char", bullet)
    else:
        etree.SubElement(pPr, qn("a:buNone"))


def text(slide, x, y, w, h, paras, size=14, color=INK, bold=False, align=PP_ALIGN.LEFT,
         anchor=MSO_ANCHOR.TOP, fill=None, line_color=None, margin=(0.04, 0.02),
         line=1.15, gap=0, shape=MSO_SHAPE.RECTANGLE):
    """paras: list of str | dict(runs=[(text, opts)], size, color, bold, bullet, level, gap)"""
    if fill is None and line_color is None:
        sh = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    else:
        sh = slide.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h))
        sh.shadow.inherit = False
        nostyle(sh)
        if fill is None:
            sh.fill.background()
        else:
            sh.fill.solid()
            sh.fill.fore_color.rgb = fill
        if line_color is None:
            sh.line.fill.background()
        else:
            sh.line.color.rgb = line_color
            sh.line.width = Pt(1)
    tf = sh.text_frame
    tf.word_wrap = True
    tf.auto_size = None
    tf.margin_left = tf.margin_right = Inches(margin[0])
    tf.margin_top = tf.margin_bottom = Inches(margin[1])
    tf.vertical_anchor = anchor
    if isinstance(paras, str):
        paras = [paras]
    for i, item in enumerate(paras):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        if isinstance(item, str):
            item = {"runs": [(item, {})]}
        psize = item.get("size", size)
        pcolor = item.get("color", color)
        pbold = item.get("bold", bold)
        para_fmt(p, align=item.get("align", align), space_before=(item.get("gap", gap) if i else 0),
                 line=item.get("line", line), bullet=item.get("bullet"), level=item.get("level", 0))
        for t, o in item["runs"]:
            r = p.add_run()
            r.text = t
            font(r.font, o.get("size", psize), o.get("color", pcolor), o.get("bold", pbold))
    return sh


def R(t, **o):
    return (t, o)


def P(*runs, **o):
    rs = [r if isinstance(r, tuple) else (r, {}) for r in runs]
    d = {"runs": rs}
    d.update(o)
    return d


def rect(slide, x, y, w, h, fill=None, line_color=None, shape=MSO_SHAPE.RECTANGLE, width=1):
    sh = slide.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h))
    sh.shadow.inherit = False
    nostyle(sh)
    if fill is None:
        sh.fill.background()
    else:
        sh.fill.solid()
        sh.fill.fore_color.rgb = fill
    if line_color is None:
        sh.line.fill.background()
    else:
        sh.line.color.rgb = line_color
        sh.line.width = Pt(width)
    return sh


def hline(slide, x, y, w, color=LINE, width=1):
    c = slide.shapes.add_connector(MSO_CONNECTOR.STRAIGHT, Inches(x), Inches(y), Inches(x + w), Inches(y))
    c.line.color.rgb = color
    c.line.width = Pt(width)
    nostyle(c)
    return c


def vline(slide, x, y, h, color=LINE, width=1):
    c = slide.shapes.add_connector(MSO_CONNECTOR.STRAIGHT, Inches(x), Inches(y), Inches(x), Inches(y + h))
    c.line.color.rgb = color
    c.line.width = Pt(width)
    nostyle(c)
    return c


def new_deck():
    prs = Presentation()
    prs.slide_width = Inches(SW)
    prs.slide_height = Inches(SH)
    return prs


def blank(prs):
    return prs.slides.add_slide(prs.slide_layouts[6])


def cover(prs, title, subtitle, audience, ask):
    s = blank(prs)
    rect(s, 0, 0, 0.32, SH, fill=ACCENT)
    text(s, 1.1, 0.9, 8, 0.4, DECK + "|説明資料", size=14, color=ACCENT, bold=True)
    text(s, 1.1, 2.0, 11.2, 1.9, title, size=40, bold=True, line=1.1, anchor=MSO_ANCHOR.BOTTOM)
    text(s, 1.1, 4.0, 11.2, 0.6, subtitle, size=20, color=SUB)
    hline(s, 1.1, 5.15, 11.2, LINE)
    text(s, 1.1, 5.35, 2.0, 0.4, "聞き手", size=13, color=MUTED)
    text(s, 3.0, 5.35, 9.3, 0.4, audience, size=15)
    text(s, 1.1, 5.85, 2.0, 0.4, "決めること", size=13, color=MUTED)
    text(s, 3.0, 5.85, 9.3, 0.8, ask, size=15, bold=True)
    text(s, 1.1, 6.85, 8, 0.3, "数値はすべて架空のもの", size=10, color=MUTED)
    return s


def page(prs, n, title, lead):
    s = blank(prs)
    rect(s, M, 0.42, 0.09, 0.62, fill=ACCENT)
    text(s, M + 0.22, 0.36, SW - 2 * M - 0.22, 0.72, title, size=28, bold=True, anchor=MSO_ANCHOR.MIDDLE)
    text(s, M + 0.22, 1.1, SW - 2 * M - 0.22, 0.5, lead, size=16, color=SUB)
    hline(s, M, 1.72, SW - 2 * M, LINE)
    hline(s, M, 6.98, SW - 2 * M, LINE)
    text(s, M, 7.02, 8, 0.3, DECK + "|数値は架空", size=10, color=MUTED)
    text(s, SW - M - 2, 7.02, 2, 0.3, f"{n} / {TOTAL}", size=10, color=MUTED, align=PP_ALIGN.RIGHT)
    return s


def table(slide, x, y, w, rows, col_w, row_h=0.42, size=13, header_fill=None, header_color=INK,
          aligns=None, bold_first_col=False, fills=None, colors=None, bolds=None):
    nr, nc = len(rows), len(rows[0])
    gs = slide.shapes.add_table(nr, nc, Inches(x), Inches(y), Inches(w), Inches(row_h * nr))
    tbl = gs.table
    tblPr = tbl._tbl.tblPr
    for k in ("firstRow", "bandRow"):
        tblPr.set(k, "0")
    style = tblPr.find(qn("a:tableStyleId"))
    if style is None:
        style = etree.SubElement(tblPr, qn("a:tableStyleId"))
    style.text = "{5940675A-B579-460E-94D1-54222C63F5DA}"  # No Style, Table Grid
    for j, cw in enumerate(col_w):
        tbl.columns[j].width = Inches(cw)
    for i in range(nr):
        tbl.rows[i].height = Inches(row_h)
        for j in range(nc):
            c = tbl.cell(i, j)
            c.margin_left = c.margin_right = Inches(0.1)
            c.margin_top = c.margin_bottom = Inches(0.04)
            c.vertical_anchor = MSO_ANCHOR.MIDDLE
            f = None
            if i == 0 and header_fill is not None:
                f = header_fill
            if fills and fills.get((i, j)) is not None:
                f = fills[(i, j)]
            if f is None:
                c.fill.background()
            else:
                c.fill.solid()
                c.fill.fore_color.rgb = f
            tf = c.text_frame
            tf.word_wrap = True
            val = rows[i][j]
            lines = val.split("\n") if isinstance(val, str) else val
            for k, ln in enumerate(lines):
                p = tf.paragraphs[0] if k == 0 else tf.add_paragraph()
                para_fmt(p, align=(aligns[j] if aligns else PP_ALIGN.LEFT), line=1.1)
                r = p.add_run()
                r.text = ln
                b = (i == 0) or (bold_first_col and j == 0)
                if bolds and (i, j) in bolds:
                    b = bolds[(i, j)]
                col = header_color if i == 0 else INK
                if colors and (i, j) in colors:
                    col = colors[(i, j)]
                sz = size if k == 0 else size - 2
                if k > 0 and i > 0:
                    col = SUB if not (colors and (i, j) in colors) else col
                font(r.font, sz, col, b)
    # borders: only horizontal lines
    for i in range(nr):
        for j in range(nc):
            tcPr = tbl.cell(i, j)._tc.get_or_add_tcPr()
            for tag, show in (("a:lnL", False), ("a:lnR", False), ("a:lnT", i == 0), ("a:lnB", True)):
                ln = etree.SubElement(tcPr, qn(tag))
                if show:
                    ln.set("w", str(int(Pt(1.25 if (i == 0 or i == nr - 1) else 0.75))))
                    sf = etree.SubElement(ln, qn("a:solidFill"))
                    clr = etree.SubElement(sf, qn("a:srgbClr"))
                    clr.set("val", "1F2933" if (i == 0 and tag == "a:lnT") or i == nr - 1 or i == 0 else "D3D9DF")
                else:
                    ln.set("w", "0")
                    etree.SubElement(ln, qn("a:noFill"))
            # move ln elements before fill (schema order: lnL lnR lnT lnB ... fill)
            fill_el = None
            for el in list(tcPr):
                if el.tag in (qn("a:solidFill"), qn("a:noFill")):
                    fill_el = el
            if fill_el is not None:
                tcPr.remove(fill_el)
                tcPr.append(fill_el)
    return gs


def bar_chart(slide, x, y, w, h, cats, values, colors, horizontal=False, fmt='0', size=12,
              label_size=13, gap=60, max_v=None, series_name="値"):
    cd = CategoryChartData()
    cd.categories = cats
    cd.add_series(series_name, values)
    kind = XL_CHART_TYPE.BAR_CLUSTERED if horizontal else XL_CHART_TYPE.COLUMN_CLUSTERED
    gf = slide.shapes.add_chart(kind, Inches(x), Inches(y), Inches(w), Inches(h), cd)
    ch = gf.chart
    ch.has_legend = False
    ch.has_title = False
    font(ch.font, size, SUB)
    plot = ch.plots[0]
    plot.gap_width = gap
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.number_format = fmt
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    font(dl.font, label_size, INK, True)
    ser = plot.series[0]
    for i, c in enumerate(colors):
        pt = ser.points[i]
        pt.format.fill.solid()
        pt.format.fill.fore_color.rgb = c
    va = ch.value_axis
    va.visible = False
    va.has_major_gridlines = False
    va.minimum_scale = 0
    if max_v:
        va.maximum_scale = max_v
    ca = ch.category_axis
    ca.format.line.color.rgb = LINE
    ca.has_major_gridlines = False
    font(ca.tick_labels.font, size, INK)
    if horizontal:
        ca.reverse_order = True
    return gf


def stat(slide, x, y, w, label, value, unit="", note=None, color=None, vsize=30):
    text(slide, x, y, w, 0.32, label, size=12, color=SUB)
    text(slide, x, y + 0.3, w, 0.62, [P(R(value, size=vsize, bold=True, color=color or INK),
                                           R(unit, size=14, color=INK))], anchor=MSO_ANCHOR.BOTTOM)
    if note:
        text(slide, x, y + 0.95, w, 0.5, note, size=11, color=SUB)


# ---- deck ----
def chip(slide, x, y, tag, w=0.95, h=0.42, size=13, fill=None):
    text(slide, x, y, w, h, tag, size=size, bold=True, color=WHITE, fill=fill or ACCENT,
         align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, margin=(0, 0),
         shape=MSO_SHAPE.ROUNDED_RECTANGLE)


def build(out):
    prs = new_deck()
    cover(prs, "留学生の生活の支え\n来年度の3つの施策と予算",
          "住まい・手続き・孤立に応える3つの施策。合計は予算の枠を270万円超える",
          "大学の学生支援の委員会",
          "来年度に行う施策と、年1,000万円の枠の中での予算の割り振り")

    # --- slide 2 ---
    s = page(prs, 2, "困りごとの1位は住まい探し、2位は日本語の手続き",
             "留学生380人(学部140・大学院240、32の国と地域)への調べ。回答211、複数回答")
    text(s, M, 1.95, 7.6, 0.35, "困っていること(回答者に占める割合)", size=13, color=SUB, bold=True)
    probs = [("住まい探し", 48, "施策1"), ("日本語の手続き", 44, "施策3"), ("アルバイト探し", 31, None),
             ("孤立", 27, "施策2"), ("病院", 22, None)]
    bx, bw, maxv = M + 2.1, 3.9, 50
    y0, rh = 2.4, 0.86
    for i, (lab, v, f) in enumerate(probs):
        yy = y0 + i * rh
        text(s, M, yy, 2.0, 0.6, lab, size=16, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        bl = bw * v / maxv
        rect(s, bx, yy + 0.1, bl, 0.4, fill=ACCENT if f else GRAY)
        text(s, bx + bl + 0.08, yy, 0.9, 0.6, f"{v}%", size=16, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        if f:
            chip(s, M + 7.0 - 0.3, yy + 0.09, f, w=0.9, size=12)
        if i < len(probs) - 1:
            hline(s, M, yy + rh - 0.13, 7.6, LINE, 0.75)
    text(s, M, 6.65 - 0.2, 7.6, 0.35, "アルバイト探しと病院には、今回の施策では直接は応えない。", size=12, color=SUB)
    rx = 8.7
    rw = SW - M - rx
    rect(s, rx, 1.95, rw, 4.85, fill=PALE)
    text(s, rx + 0.3, 2.1, rw - 0.6, 0.4, "住まいとお金の実情", size=14, bold=True)
    facts = [
        ("大学の寮", "120室", "空き待ち 64人"),
        ("民間の部屋で保証人がいない", "37%", "施策1で大学が代わりになる"),
        ("去年の退学・休学", "14人", "理由の1位は経済(6人)"),
    ]
    yy = 2.6
    for i, (k, v, n) in enumerate(facts):
        text(s, rx + 0.3, yy, rw - 0.6, 0.32, k, size=13, color=SUB)
        text(s, rx + 0.3, yy + 0.3, rw - 0.6, 0.62, v, size=28, bold=True,
             color=WARN if i == 2 else INK)
        text(s, rx + 0.3, yy + 0.9, rw - 0.6, 0.3, n, size=12, color=SUB)
        yy += 1.38

    # --- slide 3 ---
    s = page(prs, 3, "3つの施策は、上位の困りごとにそれぞれ応える",
             "住まいは保証人の代わり、孤立は日本人学生の相棒、手続きは付き添いの窓口で支える")
    table(s, M, 1.95, SW - 2 * M, [
        ["", "やること", "応える困りごと", "年の費用"],
        ["施策1", "大学が保証人の代わりになる制度\n年300万円を積み立てる",
         "住まい探し 48%\n保証人がいない 37%", "300万円"],
        ["施策2", "日本人学生の相棒(バディ)を1人ずつ\n謝礼を払う", "孤立 27%", "450万円"],
        ["施策3", "手続きの付き添いの窓口\n職員1名を置く", "日本語の手続き 44%", "520万円"],
        ["合計", "", "", "1,270万円"],
    ], [1.1, 5.1, 3.6, 2.333], row_h=0.88, size=15,
        aligns=[PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.RIGHT],
        bolds={(1, 0): True, (2, 0): True, (3, 0): True, (4, 0): True, (1, 3): True, (2, 3): True,
               (3, 3): True, (4, 3): True},
        colors={(1, 0): ACCENT, (2, 0): ACCENT, (3, 0): ACCENT, (4, 3): WARN},
        fills={(4, j): PALE for j in range(4)})
    text(s, M, 6.45, SW - 2 * M, 0.4,
         "予算の枠は年1,000万円。3つとも今の規模で行うと270万円足りない(次のページ)。",
         size=14, color=WARN, bold=True)

    # --- slide 4 ---
    s = page(prs, 4, "3つの合計は1,270万円。枠に収める形を決める",
             "枠は年1,000万円。2つに絞るか、3つとも規模を縮めて270万円を減らすか")
    text(s, M, 1.95, 7.5, 0.35, "年の費用(万円)と予算の枠", size=13, color=SUB, bold=True)
    bx0, bx1 = M + 1.9, M + 7.3
    maxv = 1300
    scale = (bx1 - bx0) / maxv
    combos = [
        ("3つとも", [("施策1", 300), ("施策2", 450), ("施策3", 520)]),
        ("施策1+3", [("施策1", 300), ("施策3", 520)]),
        ("施策2+3", [("施策2", 450), ("施策3", 520)]),
        ("施策1+2", [("施策1", 300), ("施策2", 450)]),
    ]
    shades = {"施策1": ACCENT, "施策2": RGBColor(0x6E, 0x8C, 0xC4), "施策3": RGBColor(0xA9, 0xBD, 0xE0)}
    y0, rh = 2.55, 0.9
    lx = bx0 + 1000 * scale
    vline(s, lx, y0 - 0.2, rh * 4 + 0.1, WARN, 2)
    text(s, lx - 0.9, y0 - 0.48, 1.8, 0.3, "枠 1,000万円", size=12, bold=True, color=WARN, align=PP_ALIGN.CENTER)
    for i, (lab, parts) in enumerate(combos):
        yy = y0 + i * rh
        text(s, M, yy, 1.8, 0.6, lab, size=15, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        x = bx0
        tot = 0
        for name, v in parts:
            w = v * scale
            text(s, x, yy + 0.06, w, 0.48, f"{name.replace('施策', '')}:{v}", size=12, bold=True,
                 color=WHITE if name != "施策3" else INK, fill=shades[name], align=PP_ALIGN.CENTER,
                 anchor=MSO_ANCHOR.MIDDLE, margin=(0, 0), line_color=WHITE)
            x += w
            tot += v
        over = tot > 1000
        text(s, bx0 + maxv * scale + 0.1, yy, 1.0, 0.6, f"{tot:,}", size=16, bold=True, color=WARN if over else INK,
             anchor=MSO_ANCHOR.MIDDLE)
    text(s, M, 6.25, 7.5, 0.6, "棒の中の数字は施策の番号と費用(万円)。2つに絞ればどの組み合わせも枠に入る。",
         size=12, color=SUB)
    # right: decide + schedule
    rx = 8.75
    rw = SW - M - rx
    rect(s, rx, 1.95, rw, 1.95, fill=ACCENT_PALE)
    text(s, rx + 0.3, 2.05, rw - 0.6, 0.4, "決めていただくこと", size=14, bold=True, color=ACCENT)
    text(s, rx + 0.3, 2.5, rw - 0.6, 1.8, [
        P("3つとも行うか、2つに絞るか", bullet="•"),
        P("3つとも行うなら、どの施策を270万円縮めるか", bullet="•"),
    ], size=14, gap=8)
    text(s, rx, 4.35, rw, 0.35, "日程", size=13, color=SUB, bold=True)
    steps = [("2月", "決定", True), ("4月", "開始", False), ("10月", "中間の調べ", False)]
    yy = 4.75
    for i, (w_, what, now) in enumerate(steps):
        rect(s, rx + 0.05, yy + 0.1, 0.3, 0.3, fill=ACCENT if now else WHITE, line_color=ACCENT,
             shape=MSO_SHAPE.OVAL, width=2)
        if i < len(steps) - 1:
            vline(s, rx + 0.2, yy + 0.4, 0.3, ACCENT, 2)
        text(s, rx + 0.55, yy, 1.0, 0.5, w_, size=16, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        text(s, rx + 1.55, yy, rw - 1.6, 0.5, what, size=15, anchor=MSO_ANCHOR.MIDDLE)
        yy += 0.6

    prs.save(out)


if __name__ == "__main__":
    build(sys.argv[1])
