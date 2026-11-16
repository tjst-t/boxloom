"""Shared helpers for t01.py .. t05.py (kept separate from common.py, which
belongs to the decks of the other topics).

Without seeing the result, every text box is checked at build time with a
conservative width estimate: if the text could need more lines than the box
holds, or a shape leaves the slide, the script stops with an error instead of
saving an overflowing deck.
"""
import math
import unicodedata

from pptx import Presentation
from pptx.chart.data import CategoryChartData
from pptx.dml.color import RGBColor
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION, XL_LEGEND_POSITION
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import MSO_ANCHOR, PP_ALIGN
from pptx.oxml.ns import qn
from pptx.util import Pt

FONT = "Noto Sans CJK JP"
W, H = 960, 540  # slide size in pt (16:9, 13.333 x 7.5 in)
MX = 48          # left/right margin
LS = 1.3         # exact line spacing as a multiple of the font size
BODY_BOTTOM = H - 36  # body content must end above this

INK = RGBColor(0x22, 0x2B, 0x36)
SUB = RGBColor(0x5A, 0x66, 0x73)
ACCENT = RGBColor(0x1F, 0x5F, 0x9E)
ACCENT_LIGHT = RGBColor(0xE6, 0xEF, 0xF8)
WARN = RGBColor(0xB5, 0x4A, 0x1C)
WARN_LIGHT = RGBColor(0xFB, 0xEE, 0xE6)
GRAY = RGBColor(0xB8, 0xC0, 0xC8)
GRAY_LIGHT = RGBColor(0xF2, 0xF4, 0xF6)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)


def char_w(c):
    """Width of one character in em (conservative)."""
    if c == " ":
        return 0.33
    if unicodedata.east_asian_width(c) in ("W", "F", "A"):
        return 1.0
    if c in "il.,:;|!'()[]":
        return 0.35
    return 0.62


def text_w(s, size):
    return sum(char_w(c) for c in s) * size


def lines_needed(s, size, width):
    """Lines for a paragraph wrapped at width. A nearly full last line counts
    as one more, since the renderer may break at a different place."""
    total = 0
    for part in s.split("\n"):
        r = text_w(part, size) / width
        total += max(1, math.ceil(r + 0.08))
    return total


class Overflow(Exception):
    pass


def _check(label, need, have):
    if need > have + 0.01:
        raise Overflow(f"{label}: needs {need:.1f}pt but box is {have:.1f}pt")


def _bounds(label, x, y, w, h):
    if x < -0.01 or y < -0.01 or x + w > W + 0.01 or y + h > H + 0.01:
        raise Overflow(f"{label} outside slide: {x},{y},{w},{h}")


def new_deck():
    prs = Presentation()
    prs.slide_width = Pt(W)
    prs.slide_height = Pt(H)
    return prs


def blank(prs):
    return prs.slides.add_slide(prs.slide_layouts[6])


def rect(slide, x, y, w, h, fill=None, line=None, shape=MSO_SHAPE.RECTANGLE, lw=1):
    _bounds("rect", x, y, w, h)
    s = slide.shapes.add_shape(shape, Pt(x), Pt(y), Pt(w), Pt(h))
    if fill is None:
        s.fill.background()
    else:
        s.fill.solid()
        s.fill.fore_color.rgb = fill
    if line is None:
        s.line.fill.background()
    else:
        s.line.color.rgb = line
        s.line.width = Pt(lw)
    s.shadow.inherit = False
    return s


def _style_run(r, size, bold, color):
    f = r.font
    f.name = FONT
    f.size = Pt(size)
    f.bold = bold
    f.color.rgb = color
    rpr = r._r.get_or_add_rPr()
    ea = rpr.find(qn("a:ea"))
    if ea is None:
        ea = rpr.makeelement(qn("a:ea"), {})
        rpr.append(ea)
    ea.set("typeface", FONT)


def _bullet(para, char, indent, color):
    ppr = para._p.get_or_add_pPr()
    ppr.set("marL", str(int(Pt(indent))))
    ppr.set("indent", str(-int(Pt(indent))))
    clr = ppr.makeelement(qn("a:buClr"), {})
    clr.append(clr.makeelement(qn("a:srgbClr"), {"val": str(color)}))
    font = ppr.makeelement(qn("a:buFont"), {"typeface": FONT})
    bu = ppr.makeelement(qn("a:buChar"), {"char": char})
    for el in (clr, font, bu):
        ppr.append(el)


def fill_tf(tf, paras, width, height, label, inset=0, anchor=MSO_ANCHOR.TOP):
    """paras: list of dicts {text|runs, size, bold, color, align, space_before,
    bullet}. runs = [(text, bold, color)]. Checks the height."""
    tf.word_wrap = True
    tf.auto_size = None
    vin = inset * 0.6
    tf.margin_left = tf.margin_right = Pt(inset)
    tf.margin_top = tf.margin_bottom = Pt(vin)
    tf.vertical_anchor = anchor
    inner_w = width - 2 * inset
    need = 2 * vin
    for i, p in enumerate(paras):
        size = p.get("size", 14)
        bullet = p.get("bullet")
        runs = p.get("runs") or [(p["text"], p.get("bold", False), p.get("color", INK))]
        txt = "".join(t for t, _, _ in runs)
        indent = text_w(bullet, size) + 2 if bullet else 0
        n = lines_needed(txt, size, inner_w - indent)
        sb = p.get("space_before", 0) if i > 0 else 0
        need += sb + n * size * LS
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        para.alignment = p.get("align", PP_ALIGN.LEFT)
        para.line_spacing = Pt(size * LS)
        if sb:
            para.space_before = Pt(sb)
        if bullet:
            _bullet(para, bullet, indent, p.get("bullet_color", ACCENT))
        for t, b, c in runs:
            r = para.add_run()
            r.text = t
            _style_run(r, size, b, c)
    _check(label, need, height)
    return need


def text(slide, x, y, w, h, paras, label="text", inset=0, anchor=MSO_ANCHOR.TOP):
    _bounds(label, x, y, w, h)
    if isinstance(paras, dict):
        paras = [paras]
    tb = slide.shapes.add_textbox(Pt(x), Pt(y), Pt(w), Pt(h))
    fill_tf(tb.text_frame, paras, w, h, label, inset, anchor)
    return tb


def card(slide, x, y, w, h, paras, fill, line=None, label="card", inset=10,
         anchor=MSO_ANCHOR.TOP):
    s = rect(slide, x, y, w, h, fill=fill, line=line)
    if isinstance(paras, dict):
        paras = [paras]
    fill_tf(s.text_frame, paras, w, h, label, inset, anchor)
    return s


def header(slide, title, message, page, total):
    """Title (1 line), message (up to 2 lines), page mark. Returns the y
    where the body may start."""
    rect(slide, 0, 22, 8, 40, fill=ACCENT)
    text(slide, MX, 22, W - 2 * MX - 70, 40,
         {"text": title, "size": 26, "bold": True, "color": INK}, label="title")
    text(slide, W - MX - 60, 30, 60, 22,
         {"text": f"{page} / {total}", "size": 11, "color": SUB, "align": PP_ALIGN.RIGHT},
         label="page")
    text(slide, MX, 68, W - 2 * MX, 46,
         {"text": message, "size": 16, "color": ACCENT, "bold": True}, label="message")
    rect(slide, MX, 120, W - 2 * MX, 1.2, fill=GRAY)
    return 132


def footer(slide, note):
    text(slide, MX, H - 30, W - 2 * MX, 18,
         {"text": note, "size": 10, "color": SUB}, label="footer")


def table(slide, x, y, col_w, rows, size=12, head_fill=ACCENT, label="table",
          max_h=None, bold_first_col=True, highlight_rows=(), aligns=None,
          inset=6):
    """rows[0] is the header. Row heights are computed from the text and the
    total is checked against max_h. Returns the table height."""
    heights = []
    for row in rows:
        hmax = 0
        for ci, cell in enumerate(row):
            n = lines_needed(cell, size, col_w[ci] - 2 * inset)
            hmax = max(hmax, n * size * LS + 2 * 4)
        heights.append(hmax + 2)
    total = sum(heights)
    if max_h is not None:
        _check(label, total, max_h)
    _bounds(label, x, y, sum(col_w), total)
    gs = slide.shapes.add_table(len(rows), len(col_w), Pt(x), Pt(y),
                                Pt(sum(col_w)), Pt(total))
    tbl = gs.table
    tbl.first_row = True
    tbl.horz_banding = False
    for ci, cw in enumerate(col_w):
        tbl.columns[ci].width = Pt(cw)
    for ri, row in enumerate(rows):
        tbl.rows[ri].height = Pt(heights[ri])
        for ci, val in enumerate(row):
            c = tbl.cell(ri, ci)
            c.margin_left = c.margin_right = Pt(inset)
            c.margin_top = c.margin_bottom = Pt(4)
            c.vertical_anchor = MSO_ANCHOR.MIDDLE
            c.fill.solid()
            if ri == 0:
                c.fill.fore_color.rgb = head_fill
            elif ri in highlight_rows:
                c.fill.fore_color.rgb = ACCENT_LIGHT
            else:
                c.fill.fore_color.rgb = WHITE if ri % 2 else GRAY_LIGHT
            tf = c.text_frame
            tf.word_wrap = True
            p = tf.paragraphs[0]
            p.line_spacing = Pt(size * LS)
            if aligns and ri > 0:
                p.alignment = aligns[ci]
            r = p.add_run()
            r.text = val
            bold = ri == 0 or (bold_first_col and ci == 0) or ri in highlight_rows
            _style_run(r, size, bold, WHITE if ri == 0 else INK)
    return total


def bar_chart(slide, x, y, w, h, cats, series, horizontal=False, colors=None,
              number_format='#,##0', font_size=11, gap=60, legend=False,
              point_colors=None, max_label_chars=None, line=False):
    """series: [(name, values)]. Category labels are checked against
    max_label_chars (in full-width characters)."""
    _bounds("chart", x, y, w, h)
    if max_label_chars:
        for c in cats:
            if text_w(c, font_size) > max_label_chars * font_size:
                raise Overflow(f"chart label too long: {c}")
    cd = CategoryChartData()
    cd.categories = cats
    for name, vals in series:
        cd.add_series(name, vals)
    if line:
        kind = XL_CHART_TYPE.LINE_MARKERS
    else:
        kind = XL_CHART_TYPE.BAR_CLUSTERED if horizontal else XL_CHART_TYPE.COLUMN_CLUSTERED
    gf = slide.shapes.add_chart(kind, Pt(x), Pt(y), Pt(w), Pt(h), cd)
    ch = gf.chart
    ch.font.size = Pt(font_size)
    ch.font.name = FONT
    ch.font.color.rgb = INK
    ch.has_legend = legend
    if legend:
        ch.legend.position = XL_LEGEND_POSITION.TOP
        ch.legend.include_in_layout = False
    plot = ch.plots[0]
    if not line:
        plot.gap_width = gap
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.number_format = number_format
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.ABOVE if line else XL_LABEL_POSITION.OUTSIDE_END
    dl.font.size = Pt(font_size)
    va = ch.value_axis
    va.visible = False
    va.has_major_gridlines = False
    ca = ch.category_axis
    ca.tick_labels.font.size = Pt(font_size)
    ca.format.line.color.rgb = GRAY
    ca.has_major_gridlines = False
    if horizontal:
        ca.reverse_order = True
    for si, s in enumerate(plot.series):
        col = (colors or [ACCENT, GRAY])[si]
        if line:
            s.format.line.color.rgb = col
            s.format.line.width = Pt(2.5)
            s.smooth = False
        else:
            s.format.fill.solid()
            s.format.fill.fore_color.rgb = col
        if point_colors and si == 0 and not line:
            for pi, pc in point_colors.items():
                pt = s.points[pi]
                pt.format.fill.solid()
                pt.format.fill.fore_color.rgb = pc
    return ch


def cover(prs, title, subtitle, audience, date_note):
    s = blank(prs)
    rect(s, 0, 0, W, H, fill=ACCENT)
    rect(s, MX, 196, 6, 140, fill=WHITE)
    text(s, MX + 24, 190, W - 2 * MX - 24, 96,
         {"text": title, "size": 36, "bold": True, "color": WHITE}, label="cover-title")
    text(s, MX + 24, 290, W - 2 * MX - 24, 52,
         {"text": subtitle, "size": 18, "color": WHITE}, label="cover-sub")
    text(s, MX, H - 92, W - 2 * MX, 52,
         [{"text": audience, "size": 14, "color": WHITE},
          {"text": date_note, "size": 12, "color": ACCENT_LIGHT, "space_before": 4}],
         label="cover-foot")
    return s
