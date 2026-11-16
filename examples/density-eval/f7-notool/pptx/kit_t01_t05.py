"""t01〜t05 共通の部品(python-pptx で直に置く)。字の測りはしない。"""
from pptx import Presentation
from pptx.util import Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.chart.data import CategoryChartData
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION

FONT = "Noto Sans CJK JP"
INK = RGBColor(0x22, 0x2B, 0x36)
SUB = RGBColor(0x5A, 0x66, 0x73)
ACC = RGBColor(0x1F, 0x5F, 0xA8)
ACC_L = RGBColor(0xE6, 0xEE, 0xF8)
WARN = RGBColor(0xB5, 0x4A, 0x1F)
LINE = RGBColor(0xC8, 0xCF, 0xD8)
GREY = RGBColor(0xA9, 0xB3, 0xBE)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

W, H = 960, 540  # pt
M = 40  # 左右の余白


def new_deck():
    p = Presentation()
    p.slide_width = Pt(W)
    p.slide_height = Pt(H)
    return p


def _blank(prs):
    return prs.slides.add_slide(prs.slide_layouts[6])


def text(slide, x, y, w, h, paras, size=14, color=INK, bold=False, anchor=MSO_ANCHOR.TOP,
         align=PP_ALIGN.LEFT, fill=None, inset=0, line=None):
    """paras: 文字列か (文字列, {size,color,bold,bullet,level,space}) の並び。"""
    shp = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Pt(x), Pt(y), Pt(w), Pt(h)) if (fill or line) \
        else slide.shapes.add_textbox(Pt(x), Pt(y), Pt(w), Pt(h))
    if fill or line:
        shp.shadow.inherit = False
        if fill:
            shp.fill.solid(); shp.fill.fore_color.rgb = fill
        else:
            shp.fill.background()
        if line:
            shp.line.color.rgb = line; shp.line.width = Pt(0.75)
        else:
            shp.line.fill.background()
    tf = shp.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    for side in ("margin_left", "margin_right", "margin_top", "margin_bottom"):
        setattr(tf, side, Pt(inset))
    if isinstance(paras, str):
        paras = [paras]
    for i, item in enumerate(paras):
        if isinstance(item, str):
            s, o = item, {}
        elif isinstance(item, list):
            s, o = item, {}
        else:
            s, o = item
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        if o.get("space") is not None and i > 0:
            p.space_before = Pt(o["space"])
        runs = s if isinstance(s, list) else [(s, {})]
        for rs in runs:
            rt, ro = (rs, {}) if isinstance(rs, str) else rs
            r = p.add_run()
            r.text = rt
            f = r.font
            f.name = FONT
            f.size = Pt(ro.get("size", o.get("size", size)))
            f.bold = ro.get("bold", o.get("bold", bold))
            f.color.rgb = ro.get("color", o.get("color", color))
            rPr = r._r.get_or_add_rPr()
            ea = rPr.makeelement("{http://schemas.openxmlformats.org/drawingml/2006/main}ea", {"typeface": FONT})
            rPr.append(ea)
    return shp


def rect(slide, x, y, w, h, fill, shape=MSO_SHAPE.RECTANGLE):
    s = slide.shapes.add_shape(shape, Pt(x), Pt(y), Pt(w), Pt(h))
    s.shadow.inherit = False
    s.fill.solid(); s.fill.fore_color.rgb = fill
    s.line.fill.background()
    return s


def hline(slide, x, y, w, color=LINE, weight=0.75):
    c = slide.shapes.add_connector(1, Pt(x), Pt(y), Pt(x + w), Pt(y))
    c.line.color.rgb = color; c.line.width = Pt(weight)
    return c


def cover(prs, title, subtitle, audience, date="架空の資料"):
    s = _blank(prs)
    rect(s, 0, 0, 14, H, ACC)
    text(s, 70, 150, 820, 120, [(title, {"size": 36, "bold": True})], anchor=MSO_ANCHOR.BOTTOM)
    hline(s, 72, 290, 200, ACC, 2)
    text(s, 70, 305, 820, 70, [(subtitle, {"size": 18, "color": SUB})])
    text(s, 70, 450, 820, 40, [(f"{audience} ・ {date}", {"size": 12, "color": SUB})])
    return s


def page(prs, title, message, num, source=None):
    """本文のページ。題(短い)とその下の主張の文。本文の領域の上端を返す。"""
    s = _blank(prs)
    text(s, M, 26, W - 2 * M, 36, [(title, {"size": 24, "bold": True})], anchor=MSO_ANCHOR.TOP)
    text(s, M, 66, W - 2 * M, 44, [(message, {"size": 15, "color": ACC})])
    hline(s, M, 114, W - 2 * M, LINE, 1)
    foot = f"{num}"
    text(s, W - M - 60, H - 28, 60, 18, [(foot, {"size": 10, "color": SUB})], align=PP_ALIGN.RIGHT)
    if source:
        text(s, M, H - 28, W - 2 * M - 80, 18, [(source, {"size": 10, "color": SUB})])
    return s, 128


def bullets(slide, x, y, w, h, items, size=14, gap=6):
    """items: 文字列 か (文字列, 子の並び)。"""
    paras = []
    for it in items:
        head, kids = (it, []) if isinstance(it, str) else it
        paras.append(("・" + head if not isinstance(head, list) else [("・", {})] + head,
                      {"size": size, "space": gap}))
        for k in kids:
            paras.append(("　– " + k, {"size": size - 2, "color": SUB, "space": 2}))
    return text(slide, x, y, w, h, paras, size=size)


def heading(slide, x, y, w, label, color=ACC):
    text(slide, x, y, w, 20, [(label, {"size": 13, "bold": True, "color": color})])
    hline(slide, x, y + 22, w, color, 1.25)


def table(slide, x, y, w, rows, col_w=None, size=12, row_h=26, head_fill=ACC, hl_rows=(), hl_fill=ACC_L,
          align_right_from=1, head_h=None):
    nr, nc = len(rows), len(rows[0])
    gt = slide.shapes.add_table(nr, nc, Pt(x), Pt(y), Pt(w), Pt(row_h * nr))
    tbl = gt.table
    tblPr = tbl._tbl.tblPr
    # 既定の表の様式を外す
    for k in ("bandRow", "firstRow"):
        tblPr.set(k, "0")
    if col_w:
        tot = sum(col_w)
        for i, cw in enumerate(col_w):
            tbl.columns[i].width = Pt(w * cw / tot)
    for r in range(nr):
        tbl.rows[r].height = Pt(head_h if (r == 0 and head_h) else row_h)
        for c in range(nc):
            cell = tbl.cell(r, c)
            v = rows[r][c]
            cell.margin_left = cell.margin_right = Pt(6)
            cell.margin_top = cell.margin_bottom = Pt(3)
            cell.vertical_anchor = MSO_ANCHOR.MIDDLE
            cell.fill.solid()
            if r == 0:
                cell.fill.fore_color.rgb = head_fill
            elif r in hl_rows:
                cell.fill.fore_color.rgb = hl_fill
            else:
                cell.fill.fore_color.rgb = WHITE if r % 2 else RGBColor(0xF4, 0xF6, 0xF9)
            tf = cell.text_frame
            tf.word_wrap = True
            p = tf.paragraphs[0]
            p.alignment = PP_ALIGN.RIGHT if (c >= align_right_from and r > 0 and _numlike(v)) else PP_ALIGN.LEFT
            run = p.add_run(); run.text = str(v)
            f = run.font; f.name = FONT; f.size = Pt(size)
            f.bold = (r == 0) or (r in hl_rows and c == 0)
            f.color.rgb = WHITE if r == 0 else INK
    return gt


def _numlike(v):
    s = str(v).replace(",", "").replace("%", "").replace("+", "").replace("−", "").replace("-", "")
    s = s.replace("万円", "").replace("千人", "").replace("件", "").replace("人", "").replace("分", "").replace(".", "")
    return s.strip().isdigit()


def bar_chart(slide, x, y, w, h, cats, series, horizontal=True, colors=None, number_format='#,##0',
              size=11, highlight=None, legend=False, gap=60, vmin=None):
    """series: [(名前, 値の並び)]。highlight: 強調する項目の番号の集合(系列1つのとき)。"""
    cd = CategoryChartData()
    cd.categories = cats
    for name, vals in series:
        cd.add_series(name, vals)
    kind = XL_CHART_TYPE.BAR_CLUSTERED if horizontal else XL_CHART_TYPE.COLUMN_CLUSTERED
    gf = slide.shapes.add_chart(kind, Pt(x), Pt(y), Pt(w), Pt(h), cd)
    ch = gf.chart
    ch.has_legend = legend
    if legend:
        from pptx.enum.chart import XL_LEGEND_POSITION
        ch.legend.position = XL_LEGEND_POSITION.TOP
        ch.legend.include_in_layout = False
        ch.legend.font.size = Pt(size); ch.legend.font.name = FONT
    ch.font.size = Pt(size); ch.font.name = FONT
    va = ch.value_axis
    va.visible = False
    if vmin is not None:
        va.minimum_scale = vmin
    va.has_major_gridlines = False
    ca = ch.category_axis
    ca.tick_labels.font.size = Pt(size)
    ca.format.line.color.rgb = LINE
    ca.has_major_gridlines = False
    if horizontal:
        ca.reverse_order = True
    plot = ch.plots[0]
    plot.gap_width = gap
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.number_format = number_format; dl.number_format_is_linked = False
    dl.font.size = Pt(size); dl.position = XL_LABEL_POSITION.OUTSIDE_END
    cols = colors or [ACC, GREY, WARN]
    for i, s in enumerate(plot.series):
        s.format.fill.solid(); s.format.fill.fore_color.rgb = cols[i % len(cols)]
        if highlight is not None and len(series) == 1:
            for j in range(len(cats)):
                pt = s.points[j]
                pt.format.fill.solid()
                pt.format.fill.fore_color.rgb = ACC if j in highlight else GREY
    return gf


def card(slide, x, y, w, h, label, big, unit, note, color=ACC, fill=ACC_L):
    rect(slide, x, y, w, h, fill)
    rect(slide, x, y, 5, h, color)
    text(slide, x + 16, y + 10, w - 26, 20, [(label, {"size": 12, "color": SUB, "bold": True})])
    text(slide, x + 16, y + 32, w - 26, 44,
         [[(big, {"size": 30, "bold": True, "color": color}), (" " + unit, {"size": 14, "color": color})]])
    text(slide, x + 16, y + 80, w - 26, h - 88, [(note, {"size": 12, "color": INK})])


def timeline(slide, x, y, w, steps, size=12, color=ACC):
    """steps: [(時期, 中身)]。横の線に丸と文字。"""
    n = len(steps)
    hline(slide, x, y + 10, w, color, 2)
    step = w / n
    for i, (when, what) in enumerate(steps):
        cx = x + step * i
        rect(slide, cx, y + 3, 14, 14, color, MSO_SHAPE.OVAL)
        text(slide, cx, y + 24, step - 12, 20, [(when, {"size": size + 1, "bold": True, "color": color})])
        text(slide, cx, y + 46, step - 12, 60, [(what, {"size": size})])
