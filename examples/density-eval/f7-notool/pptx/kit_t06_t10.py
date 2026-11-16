"""t06〜t10 共通の補助: 16:9 の資料の型(表紙・本文の頭・文字枠・表・グラフ・カード)。
字を測らない。字の大きさと枠は固定の値で決める。"""
import sys
from pptx import Presentation
from pptx.util import Inches, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE
from pptx.chart.data import CategoryChartData
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION, XL_LEGEND_POSITION

FONT = "Noto Sans CJK JP"
NAVY = RGBColor(0x1F, 0x3A, 0x5F)
ACCENT = RGBColor(0xD9, 0x6C, 0x1F)
INK = RGBColor(0x26, 0x2B, 0x33)
MUTED = RGBColor(0x6B, 0x72, 0x80)
LIGHT = RGBColor(0xEE, 0xF2, 0xF7)
LINE = RGBColor(0xC8, 0xD0, 0xDA)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
GREY = RGBColor(0xA9, 0xB4, 0xC2)

W, H = 13.333, 7.5
M = 0.6  # 左右の外周


class Deck:
    def __init__(self, title):
        self.prs = Presentation()
        self.prs.slide_width = Inches(W)
        self.prs.slide_height = Inches(H)
        self.title = title
        self.page = 0

    def _blank(self):
        self.page += 1
        return self.prs.slides.add_slide(self.prs.slide_layouts[6])

    def save(self):
        self.prs.save(sys.argv[1])

    # ---- 表紙
    def cover(self, title, subtitle, audience, date_line):
        s = self._blank()
        rect(s, 0, 0, W, H, NAVY)
        rect(s, M, 2.55, 0.12, 2.0, ACCENT)
        text(s, M + 0.35, 2.4, W - 2 * M - 0.35, 1.3, [(title, 40, True, WHITE)])
        text(s, M + 0.35, 3.65, W - 2 * M - 0.35, 1.0, [(subtitle, 20, False, RGBColor(0xDD, 0xE5, 0xEF))])
        text(s, M + 0.35, 5.9, W - 2 * M - 0.35, 0.9,
             [(audience, 14, False, RGBColor(0xDD, 0xE5, 0xEF)), (date_line, 12, False, GREY)])
        return s

    # ---- 本文の頭(題=結論の1行)と下の頁番号
    def content(self, title, lead=None, source=None):
        s = self._blank()
        text(s, M, 0.35, W - 2 * M, 0.75, [(title, 26, True, NAVY)], anchor=MSO_ANCHOR.MIDDLE)
        line(s, M, 1.12, W - M, 1.12, NAVY, 1.5)
        if lead:
            text(s, M, 1.22, W - 2 * M, 0.55, [(lead, 15, False, INK)], anchor=MSO_ANCHOR.MIDDLE)
        foot = self.title + "    " + str(self.page)
        text(s, M, H - 0.42, W - 2 * M, 0.3, [(foot, 10, False, MUTED)], align=PP_ALIGN.RIGHT)
        if source:
            text(s, M, H - 0.42, 8.5, 0.3, [("出典:" + source, 10, False, MUTED)])
        return s


def rect(s, x, y, w, h, fill, line_color=None, shape=MSO_SHAPE.RECTANGLE):
    r = s.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h))
    r.shadow.inherit = False
    r.fill.solid()
    r.fill.fore_color.rgb = fill
    if line_color is None:
        r.line.fill.background()
    else:
        r.line.color.rgb = line_color
        r.line.width = Pt(1)
    return r


def line(s, x1, y1, x2, y2, color, width=1.0):
    c = s.shapes.add_connector(1, Inches(x1), Inches(y1), Inches(x2), Inches(y2))
    c.line.color.rgb = color
    c.line.width = Pt(width)
    return c


def _run(p, t, size, bold, color):
    r = p.add_run()
    r.text = t
    f = r.font
    f.name = FONT
    f.size = Pt(size)
    f.bold = bold
    f.color.rgb = color
    rPr = r._r.get_or_add_rPr()
    for tag in ("a:ea",):
        from pptx.oxml.ns import qn
        el = rPr.find(qn(tag))
        if el is None:
            el = rPr.makeelement(qn(tag), {})
            rPr.append(el)
        el.set("typeface", FONT)
    return r


def text(s, x, y, w, h, paras, align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, space=0, inset=0.04):
    """paras: [(text, size, bold, color)] または [[(run..),(run..)], ...] の段落の並び。"""
    tb = s.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    tf = tb.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    for side in ("margin_left", "margin_right", "margin_top", "margin_bottom"):
        setattr(tf, side, Inches(inset))
    for i, para in enumerate(paras):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        if i > 0 and space:
            p.space_before = Pt(space)
        runs = para if isinstance(para, list) else [para]
        for t, size, bold, color in runs:
            _run(p, t, size, bold, color)
    return tb


def bullets(s, x, y, w, h, items, size=15, color=INK, space=6, mark="・"):
    """items: 文字列か (文字列, 子の並び)。子は一段下げて小さく。"""
    paras = []
    for it in items:
        if isinstance(it, tuple):
            head, kids = it
        else:
            head, kids = it, []
        paras.append([(mark + head, size, False, color)])
        for k in kids:
            paras.append([("    – " + k, size - 2, False, MUTED)])
    return text(s, x, y, w, h, paras, space=space)


def card(s, x, y, w, h, head, body, size=14, head_size=16, fill=LIGHT, bar=NAVY):
    rect(s, x, y, w, h, fill)
    rect(s, x, y, 0.08, h, bar)
    text(s, x + 0.2, y + 0.12, w - 0.3, 0.45, [(head, head_size, True, bar)])
    paras = body if isinstance(body, list) else [body]
    text(s, x + 0.2, y + 0.6, w - 0.3, h - 0.7, [[(b, size, False, INK)] for b in paras], space=4)


def stat(s, x, y, w, number, label, color=NAVY, nsize=36, lsize=13):
    text(s, x, y, w, 0.75, [(number, nsize, True, color)])
    text(s, x, y + 0.75, w, 0.55, [(label, lsize, False, MUTED)])


def table(s, x, y, w, h, rows, col_w, size=13, head_fill=NAVY, emph_rows=(), emph_col=None, align_right=()):
    nr, nc = len(rows), len(rows[0])
    gt = s.shapes.add_table(nr, nc, Inches(x), Inches(y), Inches(w), Inches(h))
    tbl = gt.table
    for j, cw in enumerate(col_w):
        tbl.columns[j].width = Inches(cw)
    for i in range(nr):
        tbl.rows[i].height = Inches(h / nr)
        for j in range(nc):
            c = tbl.cell(i, j)
            c.margin_left = c.margin_right = Inches(0.08)
            c.margin_top = c.margin_bottom = Inches(0.03)
            c.vertical_anchor = MSO_ANCHOR.MIDDLE
            tf = c.text_frame
            tf.word_wrap = True
            p = tf.paragraphs[0]
            head = i == 0
            emph = (i in emph_rows) or (emph_col is not None and j == emph_col and i > 0)
            color = WHITE if head else (ACCENT if emph else INK)
            _run(p, str(rows[i][j]), size, head or emph, color)
            if j in align_right and not head:
                p.alignment = PP_ALIGN.RIGHT
            c.fill.solid()
            c.fill.fore_color.rgb = head_fill if head else (WHITE if i % 2 else LIGHT)
    return tbl


def bar_chart(s, x, y, w, h, cats, values, fmt='0"%"', horizontal=False, highlight=(), size=12,
              color=NAVY, hcolor=ACCENT, maxv=None, series="値"):
    cd = CategoryChartData()
    cd.categories = cats
    cd.add_series(series, values)
    kind = XL_CHART_TYPE.BAR_CLUSTERED if horizontal else XL_CHART_TYPE.COLUMN_CLUSTERED
    gf = s.shapes.add_chart(kind, Inches(x), Inches(y), Inches(w), Inches(h), cd)
    ch = gf.chart
    ch.has_legend = False
    ch.font.size = Pt(size)
    ch.font.name = FONT
    plot = ch.plots[0]
    plot.gap_width = 60
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.number_format = fmt
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    dl.font.size = Pt(size)
    ser = plot.series[0]
    ser.format.fill.solid()
    ser.format.fill.fore_color.rgb = color
    for i in highlight:
        pt = ser.points[i]
        pt.format.fill.solid()
        pt.format.fill.fore_color.rgb = hcolor
    va = ch.value_axis
    va.visible = False
    va.has_major_gridlines = False
    if maxv is not None:
        va.maximum_scale = maxv
    va.minimum_scale = 0
    ca = ch.category_axis
    ca.tick_labels.font.size = Pt(size)
    ca.format.line.color.rgb = LINE
    if horizontal:
        ca.reverse_order = True
    return ch
