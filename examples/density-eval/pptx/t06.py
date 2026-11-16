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

ACCENT = RGBColor(0x1F, 0x5F, 0x8B)
ACCENT_PALE = RGBColor(0xE4, 0xEE, 0xF5)
DECK = "席の自由化の試行"
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
    cover(prs, "席の自由化(フリーアドレス)を\n2階で3か月試す",
          "3フロアに広げる前に、140人の試行で席の足りる・足りないを確かめる",
          "総務の課長会議",
          "2階・対象140人・3か月の試行を始めるかどうか")

    # --- slide 2: current state ---
    s = page(prs, 2, "席の4割は毎日空いている",
             "社員420人に席400。平均の出社は58%(約240人)で、最も多い火曜でも76%(約320人)")
    text(s, M, 1.95, 7.4, 0.35, "曜日ごとの出社(人)  社員420人×出社率", size=13, color=SUB, bold=True)
    days = ["月", "火", "水", "木", "金"]
    rates = [64, 76, 71, 62, 41]
    ppl = [round(420 * r / 100) for r in rates]
    cats = [f"{d}({r}%)" for d, r in zip(days, rates)]
    bar_chart(s, M, 2.3, 7.4, 4.5, cats, ppl,
              [GRAY, ACCENT, GRAY, GRAY, GRAY], max_v=420, label_size=14, gap=55)
    # right panel
    px = 8.6
    pw = SW - M - px
    rect(s, px, 1.95, pw, 4.85, fill=PALE)
    text(s, px + 0.3, 2.1, pw - 0.6, 0.4, "席の数と出社の最大(火曜)", size=14, bold=True)
    rows = [
        ("今の席", "400", "火曜でも80席が空く"),
        ("案の席", "340", "席300+集中の席40"),
        ("火曜の出社", "319", "週のうち最も多い曜日"),
        ("他社の目安", "351", "出社の最大の1.1倍"),
    ]
    yy = 2.65
    for i, (k, v, n) in enumerate(rows):
        col = ACCENT if i == 1 else INK
        text(s, px + 0.3, yy, 1.5, 0.5, k, size=13, color=SUB, anchor=MSO_ANCHOR.MIDDLE)
        text(s, px + 1.75, yy, 1.2, 0.5, [P(R(v, size=24, bold=True, color=col), R(" 席" if i != 2 else " 人", size=12))],
             anchor=MSO_ANCHOR.MIDDLE)
        text(s, px + 0.3, yy + 0.48, pw - 0.6, 0.3, n, size=11, color=SUB)
        if i < len(rows) - 1:
            hline(s, px + 0.3, yy + 0.86, pw - 0.6, LINE)
        yy += 0.95
    text(s, px + 0.3, 6.45, pw - 0.6, 0.35,
         [P(R("案は目安より11席少ない", bold=True, color=WARN))], size=12)

    # --- slide 3: the plan ---
    s = page(prs, 3, "席を300に減らし、集中の席と小部屋を足す",
             "面積はそのまま使い方を変える。賃料は減らず、家具とシステムの費用がかかる")
    text(s, M, 1.95, 6.6, 0.35, "席の構成(3フロア合計)", size=13, color=SUB, bold=True)
    table(s, M, 2.35, 6.6, [
        ["", "今", "案", "差"],
        ["自由に座る席", "400", "300", "−100"],
        ["集中の席(1人用)", "—", "40", "+40"],
        ["打ち合わせの小部屋", "—", "8室", "+8室"],
        ["面積", "—", "同じ", "±0"],
    ], [2.7, 1.2, 1.2, 1.5], row_h=0.58, size=15,
        aligns=[PP_ALIGN.LEFT, PP_ALIGN.RIGHT, PP_ALIGN.RIGHT, PP_ALIGN.RIGHT],
        colors={(1, 2): ACCENT, (2, 2): ACCENT, (3, 2): ACCENT},
        bolds={(1, 2): True, (2, 2): True, (3, 2): True})
    text(s, M, 5.45, 6.6, 1.3, [
        P("社員420人に対し、座れる場所は 席300+集中の席40=340。", bullet="•"),
        P("減らした席の場所を、集中の席と小部屋に回す。", bullet="•"),
    ], size=14, gap=6)
    cx = 7.8
    cw = SW - M - cx
    text(s, cx, 1.95, cw, 0.35, "費用", size=13, color=SUB, bold=True)
    costs = [
        ("家具", "2,300", "万円", "初めの1回"),
        ("予約のシステム", "360", "万円/年", "毎年かかる"),
        ("賃料の削減", "なし", "", "面積が同じため"),
    ]
    yy = 2.35
    for i, (k, v, u, n) in enumerate(costs):
        rect(s, cx, yy, cw, 1.3, fill=PALE if i < 2 else WHITE, line_color=None if i < 2 else LINE)
        text(s, cx + 0.3, yy + 0.15, 2.4, 0.4, k, size=14, bold=True)
        text(s, cx + 0.3, yy + 0.6, 2.4, 0.4, n, size=12, color=SUB)
        text(s, cx + 2.4, yy + 0.2, cw - 2.7, 0.9,
             [P(R(v, size=32, bold=True, color=INK if i < 2 else WARN), R(" " + u if u else "", size=14))],
             align=PP_ALIGN.RIGHT, anchor=MSO_ANCHOR.MIDDLE)
        yy += 1.48

    # --- slide 4: trial ---
    s = page(prs, 4, "2階の140人で3か月試し、3つの物差しで測る",
             "試算では火曜に席が6人分足りない。足りるかどうかと2つの心配を、試行で確かめる")
    # trial summary strip
    items = [("場所", "2階だけ"), ("対象", "140人"), ("期間", "3か月"), ("2階の席", "100")]
    bw = (SW - 2 * M - 0.3 * 3) / 4
    for i, (k, v) in enumerate(items):
        x = M + i * (bw + 0.3)
        rect(s, x, 1.95, bw, 0.95, fill=ACCENT_PALE)
        text(s, x + 0.25, 1.95, 1.4, 0.95, k, size=13, color=SUB, anchor=MSO_ANCHOR.MIDDLE)
        text(s, x + 1.1, 1.95, bw - 1.3, 0.95, v, size=24, bold=True, color=ACCENT,
             anchor=MSO_ANCHOR.MIDDLE, align=PP_ALIGN.RIGHT)
    text(s, M, 3.15, 7.6, 0.35, "心配と、試行で確かめること", size=13, color=SUB, bold=True)
    table(s, M, 3.5, 7.6, [
        ["心配", "いまわかっていること", "試行で見ること"],
        ["火曜に席が足りるか", "76%×140人=106人\n2階の席100 → 6人分足りない", "火曜に座れなかった人数"],
        ["紙の書類の置き場所", "自分の席が無くなる", "置き場所の不足の声"],
        ["部署の会話が減る", "同じ部署が離れて座る", "満足度の調べで聞く"],
    ], [2.4, 2.9, 2.3], row_h=0.78, size=13,
        colors={(1, 1): WARN}, bolds={(1, 0): True, (2, 0): True, (3, 0): True})
    rx = 8.6
    rw = SW - M - rx
    text(s, rx, 3.15, rw, 0.35, "評価の物差し", size=13, color=SUB, bold=True)
    ms = ["席の空きを探す時間", "満足度", "会議室の予約の取りやすさ"]
    yy = 3.55
    for i, m in enumerate(ms):
        rect(s, rx, yy, 0.5, 0.5, fill=ACCENT, shape=MSO_SHAPE.OVAL)
        text(s, rx, yy, 0.5, 0.5, str(i + 1), size=15, bold=True, color=WHITE,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, margin=(0, 0))
        text(s, rx + 0.7, yy, rw - 0.7, 0.5, m, size=15, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        yy += 0.72
    text(s, rx, 5.8, rw, 0.95, [
        P("3か月の結果で、3フロアに広げるか・席の数を変えるかを決め直す。")
    ], size=13, color=SUB)

    prs.save(out)


if __name__ == "__main__":
    build(sys.argv[1])
