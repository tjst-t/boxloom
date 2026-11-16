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

ACCENT = RGBColor(0x5B, 0x47, 0x8F)
ACCENT_PALE = RGBColor(0xEC, 0xE8, 0xF4)
DECK = "外来の待ち時間の短縮"
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
def chip(slide, x, y, tag, w=0.75, h=0.42, size=13):
    text(slide, x, y, w, h, tag, size=size, bold=True, color=WHITE, fill=ACCENT,
         align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, margin=(0, 0),
         shape=MSO_SHAPE.ROUNDED_RECTANGLE)


def build(out):
    prs = new_deck()
    cover(prs, "外来の待ち時間を短くする\n3つの手",
          "内科の待ち92分を、2年で60分以下に。検査・会計・待つ場所の3か所に手を打つ",
          "病院の運営会議",
          "待ち時間を短くする3つの手と、その費用")

    # --- slide 2 ---
    s = page(prs, 2, "待ちは平均68分。内科は92分で目標との差が最も大きい",
             "外来は1日平均820人。受付から診察までの待ち。目標は2年で内科60分以下、全体50分以下")
    text(s, M, 1.95, 7.6, 0.35, "受付から診察までの待ち(平均、分)", size=13, color=SUB, bold=True)
    rows = [("内科", 92, 60), ("整形外科", 74, None), ("小児科", 41, None), ("全体", 68, 50)]
    bx, bw, maxv = M + 1.7, 5.2, 100
    y0, rh = 2.45, 1.0
    for i, (lab, v, tgt) in enumerate(rows):
        yy = y0 + i * rh
        if i == 3:
            hline(s, M, yy - 0.08, 7.6, LINE)
        text(s, M, yy, 1.6, 0.6, lab, size=17, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        bl = bw * v / maxv
        c = ACCENT if lab in ("内科", "全体") else GRAY
        rect(s, bx, yy + 0.1, bl, 0.4, fill=c)
        text(s, bx + bl + 0.08, yy, 1.0, 0.6, f"{v}分", size=17, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        if tgt:
            tx = bx + bw * tgt / maxv
            vline(s, tx, yy - 0.02, 0.64, WARN, 2.25)
            text(s, tx - 0.8, yy + 0.6, 1.6, 0.3, f"目標 {tgt}分", size=11, bold=True, color=WARN,
                 align=PP_ALIGN.CENTER)
    rx = 8.7
    rw = SW - M - rx
    rect(s, rx, 1.95, rw, 4.85, fill=PALE)
    text(s, rx + 0.3, 2.1, rw - 0.6, 0.4, "患者の不満(調べ、回答640)", size=14, bold=True)
    stat_rows = [("1位", "待ち時間", "58%", ACCENT), ("2位", "待つ場所が混む", "21%", INK)]
    yy = 2.7
    for rank, lab, v, c in stat_rows:
        text(s, rx + 0.3, yy, rw - 0.6, 0.35, f"{rank} {lab}", size=14, color=SUB)
        text(s, rx + 0.3, yy + 0.3, rw - 0.6, 0.8, v, size=40, bold=True, color=c)
        yy += 1.35
    hline(s, rx + 0.3, 5.45, rw - 0.6, LINE)
    text(s, rx + 0.3, 5.55, rw - 0.6, 1.2, [
        "待ちの長さと、待つ間の混み方の両方が不満になっている。"
    ], size=13, color=SUB)

    # --- slide 3 ---
    s = page(prs, 3, "内科の待ちの中心は「診察の前」の52分",
             "予約の患者は54%いるが、予約の時刻どおりに始まったのは31%。検査と会計も長い")
    text(s, M, 1.95, 7.0, 0.35, "内科の待ちの内訳(分。重なりがあるので合計は92分にならない)", size=13,
         color=SUB, bold=True)
    text(s, 8.6, 1.95, 4.1, 0.35, "効く手", size=13, color=SUB, bold=True)
    parts = [("受付", 8, None), ("検査", 26, ("案1", "検査を先に受ける流れ")),
             ("診察の前", 52, ("案3", "呼び出しをスマホに")), ("会計", 18, ("案2", "自動精算機"))]
    bx, bw, maxv = M + 1.7, 5.0, 60
    y0, rh = 2.45, 0.85
    for i, (lab, v, fix) in enumerate(parts):
        yy = y0 + i * rh
        text(s, M, yy, 1.6, 0.6, lab, size=17, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        bl = bw * v / maxv
        rect(s, bx, yy + 0.1, bl, 0.4, fill=ACCENT if fix else GRAY)
        text(s, bx + bl + 0.08, yy, 1.0, 0.6, f"{v}分", size=17, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        if fix:
            chip(s, 8.6, yy + 0.09, fix[0])
            text(s, 9.5, yy, 3.2, 0.6, fix[1], size=15, anchor=MSO_ANCHOR.MIDDLE)
        hline(s, M, yy + rh - 0.12, SW - 2 * M, LINE, 0.75)
    # reservation
    yb = 5.95
    text(s, M, yb - 0.1, 3.0, 0.35, "予約が待ちを減らせていない", size=13, color=SUB, bold=True)
    items = [("予約の患者", "54%"), ("予約の時刻どおりに始まった", "31%")]
    for i, (k, v) in enumerate(items):
        x = M + i * 4.4
        text(s, x, yb + 0.25, 2.9, 0.6, k, size=14, anchor=MSO_ANCHOR.MIDDLE)
        text(s, x + 2.9, yb + 0.25, 1.2, 0.6, v, size=26, bold=True, color=WARN if i else INK,
             anchor=MSO_ANCHOR.MIDDLE)
    text(s, 9.4, yb + 0.15, SW - M - 9.4, 0.8, "検査の待ちが、予約の時刻を後ろへずらしている見込み。案1で確かめる。",
         size=12, color=SUB, anchor=MSO_ANCHOR.MIDDLE)

    # --- slide 4 ---
    s = page(prs, 4, "3つの手:検査を先に、会計を機械に、呼び出しをスマホに",
             "どれも内科の待ちの別の部分に効く。費用は精算機が最も大きい")
    table(s, M, 1.95, SW - 2 * M, [
        ["", "やること", "短くなる待ち", "費用"],
        ["案1", "検査を先に受ける流れ\n予約の前日に検査の時刻を知らせる", "検査 26分\n診察の前に検査が済む", "未試算"],
        ["案2", "会計の自動精算機を4台\n1台 600万円", "会計 18分", "2,400万円"],
        ["案3", "呼び出しの通知をスマートフォンに\n待つ場所を離れて待てる", "診察の前 52分の混み\n不満の2位(21%)", "年 240万円"],
    ], [0.9, 5.0, 3.6, 2.633], row_h=0.92, size=15,
        aligns=[PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.RIGHT],
        bolds={(1, 0): True, (2, 0): True, (3, 0): True, (2, 3): True, (3, 3): True},
        colors={(1, 0): ACCENT, (2, 0): ACCENT, (3, 0): ACCENT, (1, 3): SUB})
    ty = 5.85
    rect(s, M, ty, SW - 2 * M, 0.95, fill=ACCENT_PALE)
    text(s, M + 0.3, ty, 2.0, 0.95, "2年の目標", size=14, bold=True, color=ACCENT, anchor=MSO_ANCHOR.MIDDLE)
    text(s, M + 2.2, ty, 3.6, 0.95, [P(R("内科 ", size=15), R("92 → 60分以下", size=22, bold=True))],
         anchor=MSO_ANCHOR.MIDDLE)
    text(s, M + 6.0, ty, 3.6, 0.95, [P(R("全体 ", size=15), R("68 → 50分以下", size=22, bold=True))],
         anchor=MSO_ANCHOR.MIDDLE)
    text(s, M + 9.4, ty, SW - 2 * M - 9.6, 0.95, "職員:受付12人・会計6人の仕事の変わり方も合わせて見る",
         size=12, color=SUB, anchor=MSO_ANCHOR.MIDDLE)

    prs.save(out)


if __name__ == "__main__":
    build(sys.argv[1])
