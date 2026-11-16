"""t05 中学校の部活動の地域への移行(保護者説明会向け)"""
import re
import sys
from pptx import Presentation
from pptx.util import Inches, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE
from pptx.chart.data import CategoryChartData
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION
from pptx.oxml.ns import qn

FONT = "Noto Sans CJK JP"
INK = RGBColor(0x22, 0x2B, 0x36)
SUB = RGBColor(0x5F, 0x6B, 0x78)
LINE = RGBColor(0xC9, 0xD1, 0xDA)
PALE = RGBColor(0xEE, 0xF3, 0xF8)
ACC = RGBColor(0x1F, 0x5F, 0xA8)
ACC_PALE = RGBColor(0xDC, 0xE8, 0xF5)
GREY = RGBColor(0xA9, 0xB4, 0xBF)
WARN = RGBColor(0xC0, 0x50, 0x2A)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)

TOTAL = 4


CJK = r"[\u3000-\u30ff\u4e00-\u9fff\uff00-\uffef]"


def jp(t):
    """和文と数字の間の空白を詰め、括弧を全角にする。"""
    t = re.sub(r"(%s) +(?=[0-9A-Za-z+\-—])" % CJK, r"\1", t)
    t = re.sub(r"([0-9A-Za-z%%]) +(?=%s)" % CJK, r"\1", t)
    return t.replace("(", "（").replace(")", "）")


def set_font(run, size, bold=False, color=INK):
    f = run.font
    f.size = Pt(size)
    f.bold = bold
    f.color.rgb = color
    f.name = FONT
    rpr = run._r.get_or_add_rPr()
    for tag in ("a:ea", "a:cs"):
        el = rpr.find(qn(tag))
        if el is None:
            el = rpr.makeelement(qn(tag), {})
            rpr.append(el)
        el.set("typeface", FONT)


def text(slide, x, y, w, h, paras, size=14, color=INK, bold=False,
         align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, margin=0.0, space=4):
    """paras: list of str or list of (str, opts) runs."""
    tb = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    tf = tb.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    for side in ("margin_left", "margin_right", "margin_top", "margin_bottom"):
        setattr(tf, side, Inches(margin))
    if isinstance(paras, str):
        paras = [paras]
    for i, p in enumerate(paras):
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        para.alignment = align
        if i > 0:
            para.space_before = Pt(space)
        runs = p if isinstance(p, list) else [(p, {})]
        for t, o in runs:
            r = para.add_run()
            r.text = jp(t)
            set_font(r, o.get("size", size), o.get("bold", bold), o.get("color", color))
    return tb


def nostyle(shape):
    st = shape._element.find(qn("p:style"))
    if st is not None:
        shape._element.remove(st)


def rect(slide, x, y, w, h, fill, line=None, shape=MSO_SHAPE.RECTANGLE):
    s = slide.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h))
    s.fill.solid()
    s.fill.fore_color.rgb = fill
    if line is None:
        s.line.fill.background()
    else:
        s.line.color.rgb = line
        s.line.width = Pt(1)
    s.shadow.inherit = False
    nostyle(s)
    return s


def box_text(shape, paras, size=14, color=INK, bold=False, align=PP_ALIGN.LEFT,
             anchor=MSO_ANCHOR.MIDDLE, margin=0.12):
    tf = shape.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    for side in ("margin_left", "margin_right", "margin_top", "margin_bottom"):
        setattr(tf, side, Inches(margin))
    if isinstance(paras, str):
        paras = [paras]
    for i, p in enumerate(paras):
        para = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        para.alignment = align
        runs = p if isinstance(p, list) else [(p, {})]
        for t, o in runs:
            r = para.add_run()
            r.text = jp(t)
            set_font(r, o.get("size", size), o.get("bold", bold), o.get("color", color))


def hline(slide, x, y, w, color=LINE, weight=1.0):
    ln = slide.shapes.add_connector(1, Inches(x), Inches(y), Inches(x + w), Inches(y))
    ln.line.color.rgb = color
    ln.line.width = Pt(weight)
    nostyle(ln)
    return ln


def content_slide(prs, n, title, lead=None):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 0.18, 7.5, ACC)
    text(s, 0.6, 0.38, 12.1, 0.6, title, size=26, bold=True)
    if lead:
        text(s, 0.6, 1.0, 12.1, 0.45, lead, size=15, color=SUB)
    hline(s, 0.6, 1.5, 12.13, LINE, 1.0)
    text(s, 0.6, 7.02, 8, 0.3, DECK, size=10, color=SUB)
    text(s, 10.73, 7.02, 2, 0.3, f"{n} / {TOTAL}", size=10, color=SUB, align=PP_ALIGN.RIGHT)
    return s


def table(slide, x, y, w, h, rows, widths, size=13, header_fill=ACC, row_h=None,
          aligns=None, highlight_row=None):
    nr, nc = len(rows), len(rows[0])
    gt = slide.shapes.add_table(nr, nc, Inches(x), Inches(y), Inches(w), Inches(h))
    tbl = gt.table
    tblPr = tbl._tbl.tblPr
    style = tblPr.find(qn("a:tableStyleId"))
    if style is not None:
        style.text = "{5940675A-B579-460E-94D1-54222C63F5DA}"  # No Style, Table Grid
    for j, wd in enumerate(widths):
        tbl.columns[j].width = Inches(wd)
    for i in range(nr):
        if row_h:
            tbl.rows[i].height = Inches(row_h[i] if isinstance(row_h, list) else row_h)
        for j in range(nc):
            c = tbl.cell(i, j)
            c.margin_left = c.margin_right = Inches(0.1)
            c.margin_top = c.margin_bottom = Inches(0.05)
            c.vertical_anchor = MSO_ANCHOR.MIDDLE
            val = rows[i][j]
            runs = val if isinstance(val, list) else [(val, {})]
            tf = c.text_frame
            tf.word_wrap = True
            p = tf.paragraphs[0]
            p.alignment = (aligns[j] if aligns else PP_ALIGN.LEFT)
            for t, o in runs:
                if t.startswith("\n"):
                    t = t[1:]
                    p = tf.add_paragraph()
                    p.alignment = (aligns[j] if aligns else PP_ALIGN.LEFT)
                r = p.add_run()
                r.text = jp(t)
                if i == 0:
                    set_font(r, o.get("size", size), True, WHITE)
                else:
                    set_font(r, o.get("size", size), o.get("bold", False), o.get("color", INK))
            c.fill.solid()
            if i == 0:
                c.fill.fore_color.rgb = header_fill
            elif highlight_row is not None and i == highlight_row:
                c.fill.fore_color.rgb = ACC_PALE
            else:
                c.fill.fore_color.rgb = WHITE
    return tbl


def chart_fonts(chart, size=12):
    chart.font.size = Pt(size)
    chart.font.name = FONT
    chart.font.color.rgb = INK
    txpr = chart._chartSpace.find(qn("c:txPr"))
    if txpr is not None:
        for rpr in txpr.iter(qn("a:defRPr")):
            for tag in ("a:ea",):
                el = rpr.makeelement(qn(tag), {"typeface": FONT})
                rpr.append(el)


def bar_chart(slide, x, y, w, h, cats, vals, horizontal=False, colors=None,
              fmt='#,##0', size=12, gap=60, value_axis=False):
    cd = CategoryChartData()
    cd.categories = cats
    cd.add_series("値", vals)
    kind = XL_CHART_TYPE.BAR_CLUSTERED if horizontal else XL_CHART_TYPE.COLUMN_CLUSTERED
    gf = slide.shapes.add_chart(kind, Inches(x), Inches(y), Inches(w), Inches(h), cd)
    ch = gf.chart
    chart_fonts(ch, size)
    ch.has_legend = False
    ch.has_title = False
    plot = ch.plots[0]
    plot.gap_width = gap
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.number_format = fmt
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    dl.font.size = Pt(size)
    dl.font.bold = True
    va = ch.value_axis
    va.visible = value_axis
    va.has_major_gridlines = False
    va.minimum_scale = 0
    ca = ch.category_axis
    ca.format.line.color.rgb = LINE
    ca.has_major_gridlines = False
    if horizontal:
        ca.reverse_order = True
    ser = plot.series[0]
    for i in range(len(vals)):
        pt = ser.points[i]
        pt.format.fill.solid()
        pt.format.fill.fore_color.rgb = (colors[i] if colors else ACC)
    return ch


def timeline(slide, y, steps, x0=0.6, total=12.13, h=0.95, first_dark=True, size=16):
    """steps: list of (month, title, detail)."""
    n = len(steps)
    over = 0.25
    w = (total - over) / n
    for i, (m, t, d) in enumerate(steps):
        x = x0 + i * w
        dark = first_dark and i == 0
        shp = MSO_SHAPE.PENTAGON if i == 0 else MSO_SHAPE.CHEVRON
        b = rect(slide, x, y, w - 0.08 + over, h, ACC if dark else ACC_PALE, shape=shp)
        b.adjustments[0] = 0.3
        box_text(b, [[(m + "  ", {"size": size - 2, "color": WHITE if dark else ACC, "bold": True}),
                      (t, {"size": size, "bold": True, "color": WHITE if dark else INK})]],
                 align=PP_ALIGN.CENTER, margin=0.05)
        if d:
            text(slide, x + 0.25, y + h + 0.08, w - 0.3, 0.6, d, size=13, color=SUB,
                 align=PP_ALIGN.CENTER)


# ---------------------------------------------------------------- slides

DECK = "休日の部活動の地域への移行"


def cover(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, ACC)
    text(s, 0.9, 1.0, 11, 0.4, "保護者説明会", size=16, color=ACC_PALE)
    text(s, 0.9, 2.3, 11.5, 0.9, "休日の部活動が地域のクラブに移ります", size=40, bold=True, color=WHITE)
    text(s, 0.9, 3.25, 11.5, 0.7, "来年度からの移り方と、いただいたご不安へのお答え", size=24, color=WHITE)
    hline(s, 0.9, 4.45, 5.0, WHITE, 1.5)
    text(s, 0.9, 4.7, 11.5, 0.9,
         ["お配りした資料は、ご家庭で読み返していただけるようにしています。",
          "ご質問は最後のページの相談の窓口へ。"], size=16, color=WHITE, space=6)


def slide2(prs):
    s = content_slide(prs, 2, "休日の部活動は、9月から地域のクラブが受け持ちます",
                      "平日の部活動は今のまま学校で行います。変わるのは土日・休日の活動です。")
    # before / after
    for i, (hd, col, items) in enumerate([
        ("今", GREY, [("活動", "学校の部活動として"), ("指導", "学校の先生(休日に 月 平均 16時間)"),
                      ("場所", "学校"), ("費用", "かからない")]),
        ("9月から(休日)", ACC, [("活動", "地域のクラブ(スポーツ協会・文化団体)"),
                               ("指導", "クラブの指導者(希望する先生も登録できます)"),
                               ("場所", "学校。ただし 5つの部は学校の外"),
                               ("費用", "会費 月 1,500〜3,000円 と保険 年 800円")]),
    ]):
        x = 0.6 + i * 6.25
        w = 5.88
        h = rect(s, x, 1.8, w, 0.55, col)
        box_text(h, [[(hd, {"size": 17, "bold": True, "color": WHITE})]], margin=0.18)
        for k, (lab, val) in enumerate(items):
            y = 2.45 + k * 0.62
            text(s, x + 0.15, y, 0.9, 0.55, lab, size=14, bold=True, color=SUB, anchor=MSO_ANCHOR.MIDDLE)
            text(s, x + 1.0, y, w - 1.1, 0.55, val, size=15, anchor=MSO_ANCHOR.MIDDLE)
            hline(s, x, y + 0.6, w, LINE, 0.75)
    arr = rect(s, 6.52, 3.4, 0.3, 0.5, ACC, shape=MSO_SHAPE.RIGHT_ARROW)
    text(s, 0.6, 5.1, 6, 0.4, "移るまでの日程", size=15, bold=True)
    timeline(s, 5.5, [("4月", "説明", "本日の会"), ("6月", "試行", "運動部 3つ"),
                      ("9月", "休日の全部の活動", "運動部 9・文化部 4"), ("翌4月", "平日の一部", "移すかを検討")],
             h=0.7, first_dark=False, size=14)


def slide3(prs):
    s = content_slide(prs, 3, "ご不安の多かった、費用・送り迎え・指導の質へのお答え",
                      "保護者の調べ(回答 302人)で多かった不安の順に、今決まっていることをお伝えします。")
    text(s, 0.6, 1.75, 3.6, 0.4, "不安に思うこと(複数回答)", size=14, bold=True)
    bar_chart(s, 0.5, 2.1, 3.7, 2.4, ["費用", "送り迎え", "指導の質"], [52, 47, 35], horizontal=True,
              fmt='0"%"', colors=[ACC, ACC, ACC], size=13, gap=40)
    text(s, 0.6, 4.7, 3.6, 0.4, "移行への考え", size=14, bold=True)
    # 100% stacked bar drawn with shapes
    parts = [("賛成", 44, ACC), ("どちらとも", 38, GREY), ("反対", 18, WARN)]
    x = 0.6
    W = 3.6
    for name, v, col in parts:
        w = W * v / 100
        b = rect(s, x, 5.1, w - 0.02, 0.5, col)
        box_text(b, [[(f"{v}%", {"size": 13, "bold": True, "color": WHITE})]], align=PP_ALIGN.CENTER, margin=0.02)
        text(s, x, 5.65, w, 0.35, name, size=11, color=SUB, align=PP_ALIGN.CENTER)
        x += w
    text(s, 0.6, 6.1, 3.6, 0.7, "「どちらとも」が 4割近く。分からないことに、はっきりお答えします。",
         size=12, color=SUB)

    answers = [
        ("費用", "52%", ["会費は 月 1,500〜3,000円(部によって違います)", "保険 年 800円",
                         "就学援助を受けているご家庭は、会費がかかりません"]),
        ("送り迎え", "47%", ["学校の外で練習する部は 5つ", "学校からの距離は 1.2〜4.5km",
                           "そのほかの部は、今と同じく学校で練習します"]),
        ("指導の質", "35%", ["指導者は今 41人が登録、要るのは 58人(あと 17人)",
                           "希望する先生は、報酬を受けて指導者として登録できます"]),
    ]
    y = 1.8
    for name, pct, pts in answers:
        h = 0.5 + 0.38 * len(pts)
        hd = rect(s, 4.7, y, 1.7, h, ACC_PALE)
        box_text(hd, [[(name, {"size": 16, "bold": True, "color": ACC})], [(pct, {"size": 12, "color": SUB})]],
                 align=PP_ALIGN.CENTER, margin=0.05)
        body = rect(s, 6.4, y, 6.33, h, WHITE, line=LINE)
        box_text(body, [[("・" + p, {"size": 14})] for p in pts], margin=0.18)
        y += h + 0.15


def slide4(prs):
    s = content_slide(prs, 4, "分からないこと・ご心配は、相談の窓口へお寄せください",
                      "学校とクラブのどちらに聞いても大丈夫です。")
    for i, (hd, lines) in enumerate([
        ("学校の事務室", ["学校の部活動・平日の活動のこと", "就学援助と会費の免除のこと"]),
        ("クラブの事務局", ["休日の活動の内容・場所・時間", "会費・保険の払い方、指導者の登録"]),
    ]):
        x = 0.6 + i * 4.25
        b = rect(s, x, 1.8, 4.0, 2.4, PALE)
        box_text(b, [[(hd, {"size": 20, "bold": True, "color": ACC})],
                     [("平日 9〜17時", {"size": 16, "bold": True})]] +
                 [[("・" + l, {"size": 14})] for l in lines], anchor=MSO_ANCHOR.TOP, margin=0.25)
        for p in b.text_frame.paragraphs[1:]:
            p.space_before = Pt(8)
    text(s, 9.15, 1.75, 3.6, 0.4, "これからお知らせすること", size=15, bold=True)
    text(s, 9.15, 2.2, 3.6, 2.2,
         ["・試行する 3つの部(6月から)",
          "・部ごとの練習場所と時間",
          "・平日の活動の扱い(翌4月に検討)"],
         size=14, space=8)
    text(s, 0.6, 4.6, 12, 0.4, "今日お伝えしたこと", size=15, bold=True)
    rows = [
        ["いつから", "6月に運動部 3つで試行、9月から休日の部活動の全部が地域のクラブへ"],
        ["何が変わる", "休日の指導はクラブの指導者。5つの部は学校の外で練習"],
        ["費用", "会費 月 1,500〜3,000円・保険 年 800円。就学援助のご家庭は会費なし"],
    ]
    t = table(s, 0.6, 5.0, 12.13, 1.65, rows, [1.8, 10.33], size=14, row_h=0.55)
    # first row is styled as header by helper; restyle to plain
    for j in range(2):
        c = t.cell(0, j)
        c.fill.solid()
        c.fill.fore_color.rgb = WHITE
        for r in c.text_frame.paragraphs[0].runs:
            r.font.color.rgb = INK
            r.font.bold = (j == 0)
    for i in range(1, 3):
        for r in t.cell(i, 0).text_frame.paragraphs[0].runs:
            r.font.bold = True


def main(path):
    prs = Presentation()
    prs.slide_width = Inches(13.333)
    prs.slide_height = Inches(7.5)
    cover(prs)
    slide2(prs)
    slide3(prs)
    slide4(prs)
    prs.save(path)


if __name__ == "__main__":
    main(sys.argv[1])

