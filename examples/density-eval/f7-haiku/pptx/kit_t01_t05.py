"""t01〜t05 の共通の補助(python-pptx で直に書く)。作り手が書く中身は tNN.py にある。

配置は手で決める。字の幅は Noto Sans CJK JP の目安(和字 1em、欧数 0.55em)で数え、
枠に入らないときは WARN を出す(画像は見ない。PPTX を開かずに確かめるための数)。
"""
from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE
from pptx.oxml.ns import qn
from pptx.oxml.xmlchemy import OxmlElement

FONT = "Noto Sans CJK JP"
NAVY = RGBColor(0x1F, 0x3A, 0x5F)
INK = RGBColor(0x26, 0x2B, 0x33)
GRAY = RGBColor(0x5A, 0x61, 0x6B)
LIGHT = RGBColor(0xEC, 0xF1, 0xF7)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

LM = 0.6          # 左右の余白(in)
CW = 12.133       # 本文の幅(in)
BOTTOM = 6.75    # 本文の下の限り(in)。下の脚注は 6.95 から
WARN = []


def P(t, size=16, bold=False, color=INK, sa=6, align=None):
    return dict(t=t, size=size, bold=bold, color=color, sa=sa, align=align)


def _cw(ch, size):
    return size if ord(ch) >= 0x2E80 else size * 0.55


def _lines(s, size, width_pt):
    n = 0
    for part in s.split("\n"):
        n += 1
        cur = 0.0
        for ch in part:
            w = _cw(ch, size)
            if cur + w > width_pt and cur > 0:
                n += 1
                cur = w
            else:
                cur += w
    return n


def _need(paras, w_in, margin_in, mtb_in):
    width_pt = (w_in - 2 * margin_in) * 72
    h = 0.0
    for p in paras:
        h += _lines(p["t"], p["size"], width_pt) * p["size"] * 1.2 + p["sa"]
    return h + 2 * mtb_in * 72


def _check(label, paras, w_in, h_in, margin_in=0.05, mtb_in=0.03):
    need = _need(paras, w_in, margin_in, mtb_in)
    if need > h_in * 72 + 0.5:
        WARN.append("%s: 必要 %.0fpt > 枠 %.0fpt" % (label, need, h_in * 72))


def style_run(r, size, bold=False, color=INK):
    f = r.font
    f.name = FONT
    f.size = Pt(size)
    f.bold = bold
    f.color.rgb = color
    rPr = r._r.get_or_add_rPr()
    for tag in ("a:ea", "a:cs"):
        el = rPr.find(qn(tag))
        if el is None:
            el = OxmlElement(tag)
            rPr.append(el)
        el.set("typeface", FONT)


def fill_tf(tf, paras, anchor=MSO_ANCHOR.TOP, margin_in=0.05, mtb_in=0.03):
    tf.word_wrap = True
    tf.margin_left = tf.margin_right = Inches(margin_in)
    tf.margin_top = tf.margin_bottom = Inches(mtb_in)
    tf.vertical_anchor = anchor
    for i, p in enumerate(paras):
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        para.space_after = Pt(p["sa"])
        if p["align"] is not None:
            para.alignment = p["align"]
        r = para.add_run()
        r.text = p["t"]
        style_run(r, p["size"], p["bold"], p["color"])


def box(slide, x, y, w, h, paras, label, fill=None, anchor=MSO_ANCHOR.TOP,
        shape=MSO_SHAPE.RECTANGLE, margin_in=0.05, mtb_in=0.03):
    """文の箱。fill を渡すと地のある図形になる。"""
    _check(label, paras, w, h, margin_in, mtb_in)
    if fill is None:
        shp = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    else:
        shp = slide.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h))
        shp.fill.solid()
        shp.fill.fore_color.rgb = fill
        shp.line.fill.background()
        shp.shadow.inherit = False
    fill_tf(shp.text_frame, paras, anchor, margin_in, mtb_in)
    return shp


def subhead(slide, x, y, w, text, size=18):
    box(slide, x, y, w, 0.42, [P(text, size, True, NAVY, 0)], "小見出し:" + text)


def bullets(slide, x, y, w, h, items, label, size=16):
    paras = []
    for it in items:
        if isinstance(it, tuple):  # (字, 段) の段 2 は字下げ
            t, lvl = it
            paras.append(P(("　　・" if lvl else "・") + t, size - (2 if lvl else 0), sa=6))
        else:
            paras.append(P("・" + it, size, sa=6))
    box(slide, x, y, w, h, paras, label)


def stat(slide, x, y, w, h, value, label, vsize=30, lsize=14):
    box(slide, x, y, w, h,
        [P(value, vsize, True, NAVY, 2), P(label, lsize, False, GRAY, 0)],
        "数字カード:" + value, fill=LIGHT, anchor=MSO_ANCHOR.MIDDLE,
        margin_in=0.14, mtb_in=0.08)


def steps(slide, x, y, w, h, items, label, size=15):
    n = len(items)
    gap = 0.08
    each = (w - gap * (n - 1)) / n
    for i, (when, what) in enumerate(items):
        box(slide, x + i * (each + gap), y, each, h,
            [P(when, size + 3, True, WHITE, 3), P(what, size, False, WHITE, 0)],
            "工程:" + when, fill=NAVY, anchor=MSO_ANCHOR.MIDDLE,
            shape=MSO_SHAPE.PENTAGON, margin_in=0.16, mtb_in=0.06)


def table(slide, x, y, colw, header, rows, label, size=16, min_row_h=0.42,
          aligns=None, first_bold=False):
    """表(本物の a:tbl)。行の高さは字の折り返しで決め、下の限りを確かめる。"""
    cell_ml, mtb = 0.08, 0.04
    allrows = [header] + rows
    heights = []
    for row in allrows:
        hmax = 0.0
        for ci, val in enumerate(row):
            lines = _lines(val, size, (colw[ci] - 2 * cell_ml) * 72)
            hmax = max(hmax, lines * size * 1.2 + 2 * mtb * 72)
        heights.append(max(hmax, min_row_h * 72))
    total = sum(heights) / 72
    if y + total > BOTTOM + 0.01:
        WARN.append("%s: 表の下が %.2fin > %.2fin" % (label, y + total, BOTTOM))
    shp = slide.shapes.add_table(len(allrows), len(colw), Inches(x), Inches(y),
                                 Inches(sum(colw)), Inches(total))
    tbl = shp.table
    for ci, cw in enumerate(colw):
        tbl.columns[ci].width = Inches(cw)
    for ri, hh in enumerate(heights):
        tbl.rows[ri].height = Pt(hh)
    for ri, row in enumerate(allrows):
        is_h = ri == 0
        for ci, val in enumerate(row):
            cell = tbl.cell(ri, ci)
            cell.margin_left = cell.margin_right = Inches(cell_ml)
            cell.margin_top = cell.margin_bottom = Inches(mtb)
            cell.vertical_anchor = MSO_ANCHOR.MIDDLE
            cell.fill.solid()
            cell.fill.fore_color.rgb = NAVY if is_h else (LIGHT if ri % 2 == 0 else WHITE)
            tf = cell.text_frame
            tf.word_wrap = True
            para = tf.paragraphs[0]
            if aligns and aligns[ci] == "r" and not is_h:
                para.alignment = PP_ALIGN.RIGHT
            r = para.add_run()
            r.text = val
            style_run(r, size, bold=is_h or (first_bold and ci == 0),
                      color=WHITE if is_h else INK)
    return y + total


def lead(slide, text, y=1.36):
    box(slide, LM, y, CW, 0.5, [P(text, 16, False, INK, 0)], "リード:" + text[:12])


def cover(prs, title, sub, foot):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    bar = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, Inches(0.35), prs.slide_height)
    bar.fill.solid()
    bar.fill.fore_color.rgb = NAVY
    bar.line.fill.background()
    bar.shadow.inherit = False
    box(s, 1.0, 2.1, 11.3, 1.9, [P(title, 40, True, NAVY, 0)], "表紙の題:" + title[:8],
        anchor=MSO_ANCHOR.BOTTOM)
    box(s, 1.0, 4.2, 11.3, 1.3, [P(t, 20, False, GRAY, 6) for t in sub], "表紙の副題",)
    box(s, 1.0, 6.6, 11.3, 0.4, [P(foot, 12, False, GRAY, 0)], "表紙の脚注")
    return s


def body(prs, title, n, total, foot):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    box(s, LM, 0.35, CW, 0.8, [P(title, 28, True, NAVY, 0)], "題:" + title,
        anchor=MSO_ANCHOR.BOTTOM)
    rule = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(LM), Inches(1.22),
                              Inches(CW), Inches(0.03))
    rule.fill.solid()
    rule.fill.fore_color.rgb = NAVY
    rule.line.fill.background()
    rule.shadow.inherit = False
    box(s, LM, 6.98, 9.5, 0.3, [P(foot, 10, False, GRAY, 0)], "脚注")
    box(s, 10.9, 6.98, 1.83, 0.3, [P("%d / %d" % (n, total), 10, False, GRAY, 0, PP_ALIGN.RIGHT)],
        "頁番号")
    return s


def finish(prs, path):
    prs.save(path)
    print("saved", path, "slides", len(prs.slides))
    for w in WARN:
        print("WARN", w)
    if not WARN:
        print("no overflow warnings")


def new_prs():
    prs = Presentation()
    prs.slide_width = Inches(13.333)
    prs.slide_height = Inches(7.5)
    return prs
