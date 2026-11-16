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

ACCENT = RGBColor(0x0E, 0x6F, 0x7C)
ACCENT_PALE = RGBColor(0xE0, 0xF0, 0xF2)
DECK = "浄水器の発売の計画"
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
def build(out):
    prs = new_deck()
    cover(prs, "家庭用の浄水器(蛇口に付ける型)\n発売の時期と価格",
          "本体は7,980円、発売は7月を軸に。弁の調達が遅れたときは9月",
          "経営会議",
          "本体の価格(3案から1つ)と、発売の時期")

    # --- slide 2: price ---
    s = page(prs, 2, "本体7,980円が、買いたい人も粗利の見込みも最も多い",
             "調べ(回答1,500人、30〜60代)の「買いたい」をもとにした試算。実際に買う割合はこれより低い")
    text(s, M, 1.95, 4.6, 0.35, "本体の価格ごとの「買いたい」", size=13, color=SUB, bold=True)
    bar_chart(s, M, 2.3, 4.6, 3.3, ["7,980円", "9,980円", "12,800円"], [0.34, 0.22, 0.09],
              [ACCENT, GRAY, GRAY], fmt='0%', max_v=0.4, label_size=18, size=13, gap=60)
    tx = 5.6
    tw = SW - M - tx
    text(s, tx, 1.95, tw, 0.35, "回答1,000人あたりの本体の粗利(試算)", size=13, color=SUB, bold=True)
    table(s, tx, 2.35, tw, [
        ["本体の価格", "1台の粗利", "買いたい", "粗利の見込み"],
        ["7,980円", "4,880円", "340人", "166万円"],
        ["9,980円", "6,880円", "220人", "151万円"],
        ["12,800円", "9,700円", "90人", "87万円"],
    ], [1.75, 1.75, 1.45, 2.183], row_h=0.6, size=15,
        aligns=[PP_ALIGN.LEFT, PP_ALIGN.RIGHT, PP_ALIGN.RIGHT, PP_ALIGN.RIGHT],
        bolds={(1, 0): True, (1, 3): True}, colors={(1, 0): ACCENT, (1, 3): ACCENT},
        fills={(1, j): ACCENT_PALE for j in range(4)})
    text(s, tx, 4.85, tw, 0.7, [
        P("1台の粗利=本体の価格−原価3,100円。台数が多いほど、後のカートリッジの売上も増える。")
    ], size=12, color=SUB)
    # price line
    text(s, M, 5.65, 6, 0.3, "競合3製品との位置(本体の価格)", size=13, color=SUB, bold=True)
    lx0, lx1 = M + 0.4, SW - M - 0.4
    ly = 6.45
    pmin, pmax = 6000, 14000

    def px(v):
        return lx0 + (lx1 - lx0) * (v - pmin) / (pmax - pmin)

    hline(s, lx0, ly, lx1 - lx0, GRAY, 2)
    for v in (6480, 8900, 13200):
        rect(s, px(v) - 0.09, ly - 0.09, 0.18, 0.18, fill=GRAY, shape=MSO_SHAPE.OVAL)
        text(s, px(v) - 0.8, ly + 0.1, 1.6, 0.3, f"競合 {v:,}円", size=11, color=SUB, align=PP_ALIGN.CENTER)
    for v, main in ((7980, True), (9980, False), (12800, False)):
        c = ACCENT if main else MUTED
        rect(s, px(v) - 0.11, ly - 0.11, 0.22, 0.22, fill=c, shape=MSO_SHAPE.DIAMOND)
        text(s, px(v) - 0.8, ly - 0.45, 1.6, 0.3, f"案 {v:,}円", size=12, color=c, bold=True,
             align=PP_ALIGN.CENTER)

    # --- slide 3: plan & cartridge ---
    s = page(prs, 3, "3年で18.5万台。利益の柱はカートリッジの継続",
             "カートリッジは3か月ごとの交換で、1個の粗利は1,560円。1年続けば本体の粗利を上回る")
    text(s, M, 1.95, 5.2, 0.35, "販売の計画(千台/年)", size=13, color=SUB, bold=True)
    bar_chart(s, M, 2.3, 5.2, 4.5, ["1年目", "2年目", "3年目"], [40, 65, 80],
              [ACCENT, ACCENT, ACCENT], max_v=95, label_size=18, size=14, gap=70)
    cx = 6.4
    cw = SW - M - cx
    text(s, cx, 1.95, cw, 0.35, "1台あたりの粗利(本体7,980円のとき、試算)", size=13, color=SUB, bold=True)
    rows = [
        ("本体", "4,880円", "7,980円−原価3,100円", GRAY),
        ("カートリッジ 1年分", "6,240円", "(1,980円−原価420円)× 年4個", ACCENT),
    ]
    yy = 2.4
    for lab, v, note, c in rows:
        rect(s, cx, yy, 0.12, 1.0, fill=c)
        text(s, cx + 0.3, yy, 3.0, 0.5, lab, size=15, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        text(s, cx + 0.3, yy + 0.5, 3.6, 0.5, note, size=12, color=SUB, anchor=MSO_ANCHOR.MIDDLE)
        text(s, cx + 3.6, yy, cw - 3.6, 1.0, v, size=28, bold=True, align=PP_ALIGN.RIGHT,
             anchor=MSO_ANCHOR.MIDDLE, color=c if c != GRAY else INK)
        yy += 1.25
    rect(s, cx, 5.0, cw, 1.8, fill=PALE)
    text(s, cx + 0.3, 5.1, cw - 0.6, 1.6, [
        P("カートリッジを1年後も買い続ける人は55%の計画。", bullet="•"),
        P("競合のカートリッジは1,650〜2,400円。1,980円は中ほど。", bullet="•"),
        P("本体の台数が多いほど、継続の売上の土台が広がる。", bullet="•"),
    ], size=14, gap=6, anchor=MSO_ANCHOR.MIDDLE)

    # --- slide 4: schedule & risk ---
    s = page(prs, 4, "7月の発売には、弁の調達に余裕がない",
             "夏の需要に合わせて7月に出す。弁は取引先が1社だけで納期12週。遅れたら9月に出す")
    months = ["3月", "4月", "5月", "6月", "7月", "8月", "9月"]
    gx0 = M + 2.2
    gx1 = SW - M
    mw = (gx1 - gx0) / len(months)
    gy = 1.95
    for i, mth in enumerate(months):
        x = gx0 + i * mw
        rect(s, x, gy, mw, 0.42, fill=PALE if i % 2 == 0 else WHITE)
        text(s, x, gy, mw, 0.42, mth, size=13, color=SUB, bold=True, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.MIDDLE)
        vline(s, x, gy + 0.42, 2.55, LINE, 0.75)
    vline(s, gx1, gy + 0.42, 2.55, LINE, 0.75)
    lanes = [
        ("量産の試作", 0, 1, ACCENT_PALE, ACCENT, "試作"),
        ("品質の確認", 2, 1, ACCENT_PALE, ACCENT, "確認"),
        ("発売(予定)", 4, 1, ACCENT, WHITE, "7月 発売"),
        ("弁の作り直し", 2.5, 2.8, WHITE, WARN, "直しが出たら 12週 → 8月に届く"),
        ("遅れたときの発売", 6, 1, WHITE, WARN, "9月 発売"),
    ]
    ly = gy + 0.5
    for lab, st, ln, f, tc, inner in lanes:
        warn = tc == WARN
        text(s, M, ly, 2.1, 0.46, lab, size=14, bold=True, anchor=MSO_ANCHOR.MIDDLE,
             color=WARN if warn else INK)
        b = text(s, gx0 + st * mw + 0.06, ly + 0.04, ln * mw - 0.12, 0.38, inner, size=12, bold=True,
                 color=tc, fill=f, line_color=WARN if warn else None, align=PP_ALIGN.CENTER,
                 anchor=MSO_ANCHOR.MIDDLE, margin=(0.02, 0), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
        if warn:
            b.line.dash_style = 4
        ly += 0.48
    # risk box
    ry = 5.05
    rect(s, M, ry, SW - 2 * M, 1.75, fill=WHITE, line_color=WARN, width=1.5)
    text(s, M + 0.3, ry + 0.15, 3.4, 0.4, "懸念:弁の調達", size=15, bold=True, color=WARN)
    stats = [("取引先", "1社だけ"), ("納期", "12週")]
    for i, (k, v) in enumerate(stats):
        text(s, M + 0.3, ry + 0.65 + i * 0.5, 1.0, 0.45, k, size=13, color=SUB, anchor=MSO_ANCHOR.MIDDLE)
        text(s, M + 1.3, ry + 0.65 + i * 0.5, 2.2, 0.45, v, size=18, bold=True, anchor=MSO_ANCHOR.MIDDLE)
    vline(s, M + 3.8, ry + 0.25, 1.25, LINE)
    text(s, M + 4.1, ry + 0.2, SW - 2 * M - 4.4, 1.4, [
        P("品質の確認(5月)で弁に直しが出ると、作り直しに12週かかり7月に間に合わない。", bullet="•"),
        P("その場合は9月の発売になり、夏の需要を逃す。", bullet="•"),
        P("決めること:価格とあわせて、9月に切り替える判断の時期(5月の品質の確認の後)。", bullet="•",
          runs=[R("決めること:", bold=True, color=ACCENT),
                R("価格とあわせて、9月に切り替える判断をいつするか(5月の品質の確認の後)。")]),
    ], size=14, gap=6, anchor=MSO_ANCHOR.MIDDLE)

    prs.save(out)


if __name__ == "__main__":
    build(sys.argv[1])
