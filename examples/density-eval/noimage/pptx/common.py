# 共通の部品。画像を見ずに組むので、文字の幅を字体の送り幅で測り、
# 枠に入るか・枠どうしが重ならないかを保存の前に検査して、問題を表示する。
import sys
from pptx import Presentation
from pptx.util import Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR, MSO_AUTO_SIZE
from pptx.enum.shapes import MSO_SHAPE, MSO_CONNECTOR
from pptx.chart.data import CategoryChartData
from pptx.enum.chart import XL_CHART_TYPE, XL_LEGEND_POSITION, XL_LABEL_POSITION
from pptx.oxml.ns import qn

try:
    from PIL import ImageFont
    _FR = ImageFont.truetype('/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc', 1000, index=0)
    _FB = ImageFont.truetype('/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc', 1000, index=0)
except Exception:  # 測れなければ控えめな概算
    _FR = _FB = None

FONT = "Noto Sans CJK JP"
SW, SH = 960, 540           # pt(16:9)
ML = 40                     # 左右の外周
CW = SW - 2 * ML            # 本文の幅 880

DARK = RGBColor(0x22, 0x26, 0x2E)
MID = RGBColor(0x55, 0x5B, 0x66)
LIGHT = RGBColor(0x8A, 0x90, 0x9A)
RULE = RGBColor(0xD5, 0xD9, 0xDF)
PALE = RGBColor(0xF2, 0xF4, 0xF6)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
WARN = RGBColor(0xC0, 0x39, 0x2B)

_problems = []
_rects = {}   # slide id -> list of (name, x, y, w, h)
_cur = {'slide': None, 'n': 0}


def _adv(ch, bold):
    f = _FB if bold else _FR
    if f is not None:
        return f.getlength(ch) / 1000.0
    return 1.0 if ord(ch) > 0x2000 else 0.62


_NOSTART = set("、。,.)」』】%〜ー・:;!?ぁぃぅぇぉっゃゅょァィゥェォッャュョ")


def _tokens(text):
    toks, buf = [], ""
    for ch in text:
        if ord(ch) < 0x2000 and not ch.isspace():
            buf += ch
        else:
            if buf:
                toks.append(buf)
                buf = ""
            toks.append(ch)
    if buf:
        toks.append(buf)
    return toks


def count_lines(runs, width):
    """runs: [(text, size, bold)]。強制の改行は '\n'。行の数と最大の字の大きさを返す。"""
    width = width * 0.95  # 禁則や描き手の差の余裕
    lines, cur = 1, 0.0
    prev_tok = None
    for text, size, bold in runs:
        for part_i, part in enumerate(text.split("\n")):
            if part_i > 0:
                lines += 1
                cur = 0.0
            for tok in _tokens(part):
                w = sum(_adv(c, bold) for c in tok) * size
                if cur + w > width and cur > 0:
                    lines += 1
                    # 行頭に置けない字は前の字を連れて改行する
                    cur = w + (size if tok[0] in _NOSTART and prev_tok else 0)
                    if tok.isspace():
                        cur = 0.0
                else:
                    cur += w
                prev_tok = tok
    return lines


def text_width(text, size, bold=False):
    return sum(_adv(c, bold) for c in text) * size


def problem(msg):
    _problems.append("slide %d: %s" % (_cur['n'], msg))


def _register(name, x, y, w, h, track):
    if x < -0.1 or y < -0.1 or x + w > SW + 0.1 or y + h > SH + 0.1:
        problem("%s がスライドの外に出る (%.0f,%.0f,%.0f,%.0f)" % (name, x, y, w, h))
    if not track:
        return
    lst = _rects.setdefault(_cur['n'], [])
    for (n2, x2, y2, w2, h2) in lst:
        if x < x2 + w2 - 0.5 and x2 < x + w - 0.5 and y < y2 + h2 - 0.5 and y2 < y + h - 0.5:
            problem("%s と %s が重なる" % (name, n2))
    lst.append((name, x, y, w, h))


def new_deck():
    prs = Presentation()
    prs.slide_width = Pt(SW)
    prs.slide_height = Pt(SH)
    return prs


def new_slide(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    _cur['n'] += 1
    return s


def _font(run, size, bold, color):
    f = run.font
    f.name = FONT
    f.size = Pt(size)
    f.bold = bold
    f.color.rgb = color
    rPr = run._r.get_or_add_rPr()
    for tag in ('a:ea', 'a:cs'):
        el = rPr.find(qn(tag))
        if el is None:
            el = rPr.makeelement(qn(tag), {})
            rPr.append(el)
        el.set('typeface', FONT)


def rect(slide, x, y, w, h, fill, line=None, shape=MSO_SHAPE.RECTANGLE, name="rect", track=False, lw=1):
    _register(name, x, y, w, h, track)
    sp = slide.shapes.add_shape(shape, Pt(x), Pt(y), Pt(w), Pt(h))
    sp.shadow.inherit = False
    if fill is None:
        sp.fill.background()
    else:
        sp.fill.solid()
        sp.fill.fore_color.rgb = fill
    if line is None:
        sp.line.fill.background()
    else:
        sp.line.color.rgb = line
        sp.line.width = Pt(lw)
    return sp


def hline(slide, x, y, w, color=RULE, lw=1):
    c = slide.shapes.add_connector(MSO_CONNECTOR.STRAIGHT, Pt(x), Pt(y), Pt(x + w), Pt(y))
    c.line.color.rgb = color
    c.line.width = Pt(lw)
    return c


def vline(slide, x, y, h, color=RULE, lw=1):
    c = slide.shapes.add_connector(MSO_CONNECTOR.STRAIGHT, Pt(x), Pt(y), Pt(x), Pt(y + h))
    c.line.color.rgb = color
    c.line.width = Pt(lw)
    return c


def _norm_para(p, size, bold, color):
    """段落を {'runs': [(text, size, bold, color)], 'before': pt, 'align': ..} に揃える。"""
    if isinstance(p, str):
        p = {'t': p}
    psize = p.get('size', size)
    pbold = p.get('bold', bold)
    pcolor = p.get('color', color)
    if 'runs' in p:
        runs = []
        for r in p['runs']:
            if isinstance(r, str):
                r = (r, {})
            t, o = r
            runs.append((t, o.get('size', psize), o.get('bold', pbold), o.get('color', pcolor)))
    else:
        runs = [(p['t'], psize, pbold, pcolor)]
    if p.get('bullet'):
        b = p['bullet'] if isinstance(p['bullet'], str) else "・"
        runs = [(b, runs[0][1], False, p.get('bcolor', pcolor))] + runs
    return {'runs': runs, 'before': p.get('before', 0), 'align': p.get('align')}


def text(slide, x, y, w, h, paras, size=14, color=DARK, bold=False, align='l', anchor='t',
         fill=None, line=None, inset=(4, 3, 4, 3), spacing=1.3, name="text", track=True,
         shape=MSO_SHAPE.RECTANGLE, lw=1):
    """文字の枠。exact の行送り(字の大きさ×spacing)で高さを予測し、入らなければ問題として記録する。"""
    if isinstance(paras, (str, dict)):
        paras = [paras]
    ps = [_norm_para(p, size, bold, color) for p in paras]
    l, t, r, b = inset
    inner_w = w - l - r
    # 山形・矢羽根は図形の側でも文字の枠が先端の分だけ狭い(先端の深さ=高さの半分)
    if shape == MSO_SHAPE.CHEVRON:
        inner_w -= h
    elif shape == MSO_SHAPE.PENTAGON:
        inner_w -= h / 4
    need = t + b
    for i, p in enumerate(ps):
        mx = max(rs for _, rs, _, _ in p['runs'])
        n = count_lines([(tx, rs, rb) for tx, rs, rb, _ in p['runs']], inner_w)
        need += n * mx * spacing + (p['before'] if i > 0 else 0)
    if need > h + 0.5:
        problem("%s: 文字が入らない(要る %.1fpt / 枠 %.1fpt)「%s」" % (name, need, h, ps[0]['runs'][-1][0][:16]))
    _register(name, x, y, w, h, track)
    if fill is not None or line is not None or shape != MSO_SHAPE.RECTANGLE:
        sp = slide.shapes.add_shape(shape, Pt(x), Pt(y), Pt(w), Pt(h))
        sp.shadow.inherit = False
        if fill is None:
            sp.fill.background()
        else:
            sp.fill.solid()
            sp.fill.fore_color.rgb = fill
        if line is None:
            sp.line.fill.background()
        else:
            sp.line.color.rgb = line
            sp.line.width = Pt(lw)
    else:
        sp = slide.shapes.add_textbox(Pt(x), Pt(y), Pt(w), Pt(h))
    tf = sp.text_frame
    tf.word_wrap = True
    tf.auto_size = MSO_AUTO_SIZE.NONE
    tf.margin_left, tf.margin_top, tf.margin_right, tf.margin_bottom = Pt(l), Pt(t), Pt(r), Pt(b)
    tf.vertical_anchor = {'t': MSO_ANCHOR.TOP, 'm': MSO_ANCHOR.MIDDLE, 'b': MSO_ANCHOR.BOTTOM}[anchor]
    for i, p in enumerate(ps):
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        mx = max(rs for _, rs, _, _ in p['runs'])
        para.line_spacing = Pt(mx * spacing)
        if i > 0 and p['before']:
            para.space_before = Pt(p['before'])
        para.alignment = {'l': PP_ALIGN.LEFT, 'c': PP_ALIGN.CENTER, 'r': PP_ALIGN.RIGHT}[p['align'] or align]
        for tx, rs, rb, rc in p['runs']:
            run = para.add_run()
            run.text = tx
            _font(run, rs, rb, rc)
    return sp


def slide_title(slide, title, accent, lead=None):
    """本文のスライドの頭。題(結論)と、任意で1行のリード。返すのは本文の上の端。"""
    rect(slide, ML, 30, 6, 34, accent, name="titlebar")
    text(slide, ML + 16, 26, CW - 16, 42, title, size=24, bold=True, anchor='m', name="title")
    if lead:
        text(slide, ML + 16, 70, CW - 16, 26, lead, size=14, color=MID, anchor='m', name="lead")
        return 108
    return 84


def footer(slide, src, page, total):
    hline(slide, ML, 506, CW, RULE, 0.75)
    if src:
        text(slide, ML, 510, CW - 70, 20, src, size=9, color=LIGHT, anchor='m', name="source")
    text(slide, SW - ML - 60, 510, 60, 20, "%d / %d" % (page, total), size=9, color=LIGHT, align='r',
         anchor='m', name="page")


def cover(prs, accent, kicker, title, sub, meta):
    s = new_slide(prs)
    rect(s, 0, 0, 24, SH, accent, name="coverbar")
    text(s, 72, 148, 820, 28, kicker, size=16, color=accent, bold=True, name="kicker")
    text(s, 72, 182, 820, 110, title, size=36, bold=True, spacing=1.25, anchor='t', name="covertitle")
    text(s, 72, 300, 820, 56, sub, size=18, color=MID, name="coversub")
    hline(s, 72, 380, 820, RULE, 1)
    text(s, 72, 392, 820, 60, meta, size=13, color=MID, name="covermeta")
    return s


def style_chart(chart, size=11):
    chart.font.name = FONT
    chart.font.size = Pt(size)
    chart.font.color.rgb = MID
    # 東アジアの字体も指定する
    txPr = chart._chartSpace.find(qn('c:txPr'))
    if txPr is not None:
        for rPr in txPr.iter(qn('a:defRPr')):
            for tag in ('a:latin', 'a:ea'):
                el = rPr.find(qn(tag))
                if el is None:
                    el = rPr.makeelement(qn(tag), {})
                    rPr.append(el)
                el.set('typeface', FONT)


def chart(slide, kind, x, y, w, h, cats, series, name="chart"):
    _register(name, x, y, w, h, True)
    cd = CategoryChartData()
    cd.categories = cats
    for sname, vals in series:
        cd.add_series(sname, vals)
    gf = slide.shapes.add_chart(kind, Pt(x), Pt(y), Pt(w), Pt(h), cd)
    return gf.chart


def table(slide, x, y, w, col_w, rows, size=12, header_fill=None, header_color=WHITE, name="table",
          bold_first_col=True, max_h=None, row_min=0, cell_inset=(6, 4, 6, 4), zebra=None, col_align=None,
          spacing=1.25):
    """表。各行の高さを中身から計算して決め、合計が max_h を越えれば問題に記録する。
    rows: 1行目は見出し。セルは str か {'t','bold','color','fill','size'}。"""
    assert abs(sum(col_w) - w) < 0.5, "列の幅の合計が w と違う"
    l, t, r, b = cell_inset
    heights = []
    for ri, row in enumerate(rows):
        hh = row_min
        for ci, cell in enumerate(row):
            c = cell if isinstance(cell, dict) else {'t': cell}
            cs = c.get('size', size)
            cb = c.get('bold', ri == 0 or (bold_first_col and ci == 0))
            total = 0
            for part in c['t'].split("\n"):
                total += count_lines([(part, cs, cb)], col_w[ci] - l - r) * cs * spacing
            hh = max(hh, total + t + b)
        heights.append(hh)
    total_h = sum(heights)
    if max_h is not None and total_h > max_h + 0.5:
        problem("%s: 表が高すぎる(%.1fpt / 枠 %.1fpt)" % (name, total_h, max_h))
    _register(name, x, y, w, total_h, True)
    gf = slide.shapes.add_table(len(rows), len(col_w), Pt(x), Pt(y), Pt(w), Pt(total_h))
    tbl = gf.table
    # 既定の表のスタイル(縞)を外す
    tblPr = tbl._tbl.tblPr
    tblPr.set('firstRow', '0')
    tblPr.set('bandRow', '0')
    for ci, cw_ in enumerate(col_w):
        tbl.columns[ci].width = Pt(cw_)
    for ri, row in enumerate(rows):
        tbl.rows[ri].height = Pt(heights[ri])
        for ci, cell in enumerate(row):
            c = cell if isinstance(cell, dict) else {'t': cell}
            cs = c.get('size', size)
            cb = c.get('bold', ri == 0 or (bold_first_col and ci == 0))
            if ri == 0 and header_fill is not None:
                fill, color = header_fill, header_color
            else:
                fill = c.get('fill', zebra if (zebra is not None and ri % 2 == 0) else WHITE)
                color = c.get('color', DARK)
            cel = tbl.cell(ri, ci)
            cel.fill.solid()
            cel.fill.fore_color.rgb = fill
            cel.margin_left, cel.margin_top, cel.margin_right, cel.margin_bottom = Pt(l), Pt(t), Pt(r), Pt(b)
            cel.vertical_anchor = MSO_ANCHOR.MIDDLE
            tf = cel.text_frame
            tf.word_wrap = True
            parts = c['t'].split("\n")
            al = c.get('align') or (col_align[ci] if col_align else 'l')
            for pi, part in enumerate(parts):
                para = tf.paragraphs[0] if pi == 0 else tf.add_paragraph()
                para.line_spacing = Pt(cs * spacing)
                para.alignment = {'l': PP_ALIGN.LEFT, 'c': PP_ALIGN.CENTER, 'r': PP_ALIGN.RIGHT}[al]
                run = para.add_run()
                run.text = part
                _font(run, cs, cb, color)
    return tbl, total_h


def save(prs, path):
    prs.save(path)
    if _problems:
        print("配置の問題 %d 件:" % len(_problems))
        for p in _problems:
            print("  - " + p)
    else:
        print("配置の検査: 問題なし")
