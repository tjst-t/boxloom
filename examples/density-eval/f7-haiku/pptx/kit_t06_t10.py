"""t06〜t10 の python-pptx 共通の補助(担当 2 人目の作り手が書いた。画像は作らない)。

配置の計算は文字数から行数を見積もるだけ。はみ出しの見積もりは WARN に溜めて save() で表示する。
"""
import math

from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE

FONT = "Noto Sans CJK JP"
NAVY = RGBColor(0x1F, 0x3A, 0x5F)
ACC = RGBColor(0x2E, 0x7D, 0x9A)
TXT = RGBColor(0x33, 0x33, 0x33)
SUB = RGBColor(0x66, 0x66, 0x66)
PALE = RGBColor(0xEE, 0xF3, 0xF7)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
WARN = []


def _lines(t, size, width_in):
    width_pt = width_in * 72
    em = sum(0.55 if c.isascii() else 1.0 for c in t) * size
    return max(1, math.ceil(em * 1.05 / width_pt))


def new_prs():
    prs = Presentation()
    prs.slide_width = Inches(13.333)
    prs.slide_height = Inches(7.5)
    return prs


def rect(slide, x, y, w, h, color):
    shp = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(x), Inches(y), Inches(w), Inches(h))
    shp.fill.solid()
    shp.fill.fore_color.rgb = color
    shp.line.fill.background()
    return shp


def text(slide, x, y, w, h, paras, size=16, color=TXT, bold=False):
    """paras: 文字列、または (文字列, {size, bold, color, gap}) の並び。"""
    tb = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    tf = tb.text_frame
    tf.word_wrap = True
    tf.margin_left = tf.margin_right = Inches(0.05)
    tf.margin_top = tf.margin_bottom = Inches(0.03)
    est = 0.0
    for i, p in enumerate(paras):
        t, o = (p, {}) if isinstance(p, str) else p
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        s = o.get("size", size)
        r = para.add_run()
        r.text = t
        f = r.font
        f.name = FONT
        f.size = Pt(s)
        f.bold = o.get("bold", bold)
        f.color.rgb = o.get("color", color)
        gap = o.get("gap", 0 if i == 0 else 6)
        para.space_before = Pt(gap)
        est += _lines(t, s, w - 0.1) * s * 1.3 + gap
    if est > h * 72 + 2:
        WARN.append("text overflow est %.0fpt > %.0fpt: %s" % (est, h * 72, str(paras[0])[:24]))
    return tb


def table(slide, x, y, col_w, rows, size=14, header=True):
    nr, nc = len(rows), len(rows[0])
    shp = slide.shapes.add_table(nr, nc, Inches(x), Inches(y), Inches(sum(col_w)), Inches(0.4 * nr))
    tbl = shp.table
    for j, w in enumerate(col_w):
        tbl.columns[j].width = Inches(w)
    total = 0.0
    for i, row in enumerate(rows):
        rh = 0.0
        for j, val in enumerate(row):
            c = tbl.cell(i, j)
            c.margin_left = c.margin_right = Inches(0.06)
            c.margin_top = c.margin_bottom = Inches(0.03)
            is_head = header and i == 0
            c.fill.solid()
            c.fill.fore_color.rgb = NAVY if is_head else (PALE if i % 2 == 0 else WHITE)
            tf = c.text_frame
            tf.word_wrap = True
            p = tf.paragraphs[0]
            r = p.add_run()
            r.text = val
            r.font.name = FONT
            r.font.size = Pt(size)
            r.font.bold = is_head
            r.font.color.rgb = WHITE if is_head else TXT
            lines = _lines(val, size, col_w[j] - 0.12)
            rh = max(rh, lines * size * 1.3 + 8.6)
        tbl.rows[i].height = Pt(rh)
        total += rh
    if y * 72 + total > 6.9 * 72:
        WARN.append("table bottom %.2fin over 6.9in: %s" % ((y * 72 + total) / 72, rows[0][0]))
    return total / 72


def cover(prs, title, sub, note):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, NAVY)
    text(s, 0.9, 2.2, 11.5, 1.9, [(title, {"gap": 0})], size=40, color=WHITE, bold=True)
    text(s, 0.9, 4.2, 11.5, 0.8, [sub], size=20, color=WHITE)
    text(s, 0.9, 6.5, 11.5, 0.5, [note], size=12, color=WHITE)
    return s


def body(prs, title, lead, footer, n, total=4):
    s = prs.slides.add_slide(prs.slide_layouts[5])
    t = s.shapes.title
    t.left, t.top, t.width, t.height = Inches(0.6), Inches(0.35), Inches(12.1), Inches(0.8)
    tf = t.text_frame
    tf.word_wrap = True
    tf.text = title
    for p in tf.paragraphs:
        for r in p.runs:
            r.font.name = FONT
            r.font.size = Pt(28)
            r.font.bold = True
            r.font.color.rgb = NAVY
    if _lines(title, 28, 12.0) > 1:
        WARN.append("title wraps: %s" % title[:24])
    rect(s, 0, 0, 13.333, 0.12, ACC)
    text(s, 0.6, 1.2, 12.1, 0.5, [lead], size=16)
    text(s, 0.6, 6.95, 12.1, 0.35, ["%s    %d/%d" % (footer, n, total)], size=10, color=SUB)
    return s


def save(prs, path):
    prs.save(path)
    for w in WARN:
        print("WARN", w)
    print("saved", path, "slides", len(prs.slides))
