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

ACCENT = RGBColor(0x2D, 0x6A, 0x4F)
ACCENT_PALE = RGBColor(0xE3, 0xEF, 0xE8)
DECK = "今年の防災訓練のやり方"
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
def hbars(slide, x, y, w, rows, max_v, bar_x, bar_w, row_h=0.62, size=15, colors=None, unit="%"):
    """rows: (label, value). draws label | bar | value as shapes."""
    for i, (lab, v) in enumerate(rows):
        yy = y + i * row_h
        text(slide, x, yy, bar_x - x - 0.1, row_h, lab, size=size, anchor=MSO_ANCHOR.MIDDLE)
        bl = bar_w * v / max_v
        c = colors[i] if colors else ACCENT
        rect(slide, bar_x, yy + row_h * 0.18, bl, row_h * 0.64, fill=c)
        text(slide, bar_x + bl + 0.08, yy, 1.0, row_h, f"{v}{unit}", size=size, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)


def build(out):
    prs = new_deck()
    cover(prs, "防災訓練を、来られる日・\n来たくなる中身に変える",
          "参加は5年で310人から168人に。今年の訓練のやり方を決める",
          "自治会の役員会",
          "3つの案のうち、今年どれを行うか(6月に決定)")

    # --- slide 2 ---
    s = page(prs, 2, "参加は5年でほぼ半分、来る人の多くは60代以上",
             "去年の参加は168人で世帯(1,240)の9%。参加者の4人に3人が60代以上")
    text(s, M, 1.95, 4.2, 0.35, "訓練の参加者(人)", size=13, color=SUB, bold=True)
    bar_chart(s, M, 2.3, 4.2, 4.0, ["5年前", "去年"], [310, 168], [GRAY, ACCENT],
              max_v=360, label_size=18, size=14, gap=70)
    text(s, M, 6.3, 4.2, 0.5, "5年で142人(46%)減った", size=13, color=WARN, bold=True,
         align=PP_ALIGN.CENTER)
    vline(s, 5.25, 1.95, 4.85, LINE)
    text(s, 5.6, 1.95, 7.1, 0.35, "去年の参加者の年代", size=13, color=SUB, bold=True)
    hbars(s, 5.6, 2.35, 7.1, [("70代以上", 54), ("60代", 22), ("40〜50代", 15), ("30代以下", 9)],
          max_v=60, bar_x=7.2, bar_w=4.4, row_h=0.6,
          colors=[ACCENT, ACCENT, GRAY, GRAY])
    rect(s, 5.6, 4.95, SW - M - 5.6, 1.85, fill=PALE)
    text(s, 5.85, 5.05, 6.6, 0.4, "あわせて見ておきたいこと:要配慮の人の支え", size=14, bold=True)
    text(s, 5.85, 5.5, 2.6, 1.2, [P(R("39", size=34, bold=True, color=WARN), R(" / 86人", size=16))],
         anchor=MSO_ANCHOR.MIDDLE)
    text(s, 8.4, 5.5, 4.2, 1.2, [
        "名簿に登録した86人のうち、支援する人が決まっているのは39人(45%)。",
        P("訓練で支援する人を決める機会にもできる。", color=SUB, size=12),
    ], size=13, anchor=MSO_ANCHOR.MIDDLE, gap=4)

    # --- slide 3 ---
    s = page(prs, 3, "参加しない理由は「日が合わない」が最も多い",
             "回覧の調べ(回答214、複数回答)。3つの理由に、それぞれ別の手が要る")
    text(s, M, 1.95, 6.6, 0.35, "参加しない理由", size=13, color=SUB, bold=True)
    text(s, 8.2, 1.95, 4.5, 0.35, "効く案", size=13, color=SUB, bold=True)
    reasons = [("日が合わない", 46), ("知らなかった", 31), ("内容が毎年同じ", 24)]
    fixes = [
        [("案1", "平日の夜と休日の2回"), ("案3", "マンションごとに年3回")],
        [("—", "案には無い。8月の案内の出し方で手当てする")],
        [("案2", "子ども向けの企画")],
    ]
    y0 = 2.45
    rh = 1.35
    for i, (lab, v) in enumerate(reasons):
        yy = y0 + i * rh
        if i:
            hline(s, M, yy - 0.12, SW - 2 * M, LINE)
        text(s, M, yy, 2.4, 1.0, lab, size=17, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        bl = 3.6 * v / 50
        rect(s, 3.1, yy + 0.25, bl, 0.5, fill=ACCENT if i == 0 else GRAY)
        text(s, 3.1 + bl + 0.1, yy, 1.0, 1.0, f"{v}%", size=20, bold=True, anchor=MSO_ANCHOR.MIDDLE)
        fy = yy + (1.0 - 0.48 * len(fixes[i])) / 2
        for k, (tag, desc) in enumerate(fixes[i]):
            if tag != "—":
                text(s, 8.2, fy + k * 0.52, 0.75, 0.42, tag, size=13, bold=True, color=WHITE,
                     fill=ACCENT, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, margin=(0, 0),
                     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
                text(s, 9.1, fy + k * 0.52, 3.6, 0.42, desc, size=15, anchor=MSO_ANCHOR.MIDDLE)
            else:
                text(s, 8.2, fy, 4.5, 0.48, desc, size=14, color=WARN, anchor=MSO_ANCHOR.MIDDLE)
    text(s, M, 6.45, SW - 2 * M, 0.4,
         "「知らなかった」は3割。どの案を選んでも、8月の案内を届け方から見直す必要がある。",
         size=13, color=SUB)

    # --- slide 4 ---
    s = page(prs, 4, "3つの案の比較と、本番までの日程",
             "どの案も追加の費用は年6〜12万円。組み合わせて行うこともできる")
    table(s, M, 1.95, SW - 2 * M, [
        ["案", "やり方", "効く理由", "追加の費用"],
        ["案1", "平日の夜と休日の2回に分ける", "日が合わない(46%)", "+6万円"],
        ["案2", "子ども向けの企画\n消火器の体験・非常食の試食", "内容が毎年同じ(24%)\n若い世代を呼ぶ", "+12万円"],
        ["案3", "マンションごとの小さな訓練を年3回", "日が合わない(46%)\n近くで短く参加できる", "1回 2万円\n(3回で6万円)"],
    ], [0.9, 4.9, 3.9, 2.433], row_h=0.78, size=15,
        aligns=[PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.RIGHT],
        bolds={(1, 0): True, (2, 0): True, (3, 0): True, (1, 3): True, (2, 3): True, (3, 3): True},
        colors={(1, 0): ACCENT, (2, 0): ACCENT, (3, 0): ACCENT})
    text(s, M, 5.15, SW - 2 * M, 0.3, "避難場所の小学校の体育館は450人まで入る。参加が倍(336人)になっても収まる。",
         size=12, color=SUB)
    # timeline
    text(s, M, 5.6, 3, 0.35, "日程", size=13, color=SUB, bold=True)
    steps = [("6月", "やり方を決める", True), ("8月", "案内を配る", False), ("9月 第1日曜", "本番", False)]
    tw = (SW - 2 * M - 0.6) / 3
    for i, (when, what, now) in enumerate(steps):
        x = M + i * (tw + 0.3)
        sh = rect(s, x, 6.0, tw, 0.8, fill=ACCENT if now else ACCENT_PALE, shape=MSO_SHAPE.CHEVRON if i else MSO_SHAPE.PENTAGON)
        sh.adjustments[0] = 0.3
        text(s, x + 0.35, 6.0, tw - 0.6, 0.8,
             [P(R(when + "  ", bold=True, size=16), R(what, size=15))],
             color=WHITE if now else ACCENT, anchor=MSO_ANCHOR.MIDDLE)

    prs.save(out)


if __name__ == "__main__":
    build(sys.argv[1])
