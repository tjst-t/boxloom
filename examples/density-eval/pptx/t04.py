"""t04 小さな農家の直売所の売り場の見直し(運営の会向け)"""
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

DECK = "直売所 来年度の売り場と営業時間"


def cover(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, ACC)
    text(s, 0.9, 1.0, 11, 0.4, "直売所 運営の会 資料(生産者 22戸)", size=16, color=ACC_PALE)
    text(s, 0.9, 2.3, 11.5, 0.9, "来年度の売り場の配置と営業時間", size=40, bold=True, color=WHITE)
    text(s, 0.9, 3.25, 11.5, 0.7, "客の多い時間に品をそろえ、少ない時間の手間を減らす 3 つの案", size=24, color=WHITE)
    hline(s, 0.9, 4.45, 5.0, WHITE, 1.5)
    text(s, 0.9, 4.7, 11.5, 0.5, "この会で決めること:3 つの案それぞれを来年度に行うかどうか",
         size=16, color=WHITE)


def slide2(prs):
    s = content_slide(prs, 2, "客の半分以上は土日。土日は昼前に葉物が足りなくなる",
                      "土日の朝は品が足りず、平日の夕方は客が少ない。時間によって、品と客がかみ合っていません。")
    text(s, 0.6, 1.75, 7.2, 0.4, "曜日ごとの客数(1日 平均、人)", size=15, bold=True)
    bar_chart(s, 0.5, 2.15, 7.3, 3.7, ["月", "火", "水", "木", "金", "土", "日"],
              [85, 72, 78, 80, 110, 240, 260], colors=[GREY] * 5 + [ACC, ACC], size=13, gap=45)
    text(s, 0.6, 5.95, 7.2, 0.8, "1週間の客 925人のうち、土日が 500人(54%)。",
         size=13, color=SUB)
    facts = [
        ("6割", "土日の午前 11時までに", "葉物の 6割が売り切れ"),
        ("12%", "平日の午後 3時以降の客", "(全体に占める割合)"),
        ("9戸", "朝の搬入が早いのは負担", "(22戸のうち)"),
    ]
    for i, (v, a, b) in enumerate(facts):
        y = 1.8 + i * 1.65
        box = rect(s, 8.3, y, 4.43, 1.5, PALE)
        text(s, 8.45, y, 1.6, 1.5, v, size=30, bold=True, color=ACC if i < 2 else WARN,
             anchor=MSO_ANCHOR.MIDDLE, align=PP_ALIGN.CENTER)
        text(s, 10.1, y, 2.55, 1.5, [[(a, {"size": 14, "bold": True})], [(b, {"size": 13, "color": SUB})]],
             anchor=MSO_ANCHOR.MIDDLE, space=2)


def slide3(prs):
    s = content_slide(prs, 3, "3 つの案と、それぞれの裏づけ・気になる点",
                      "案1・案2は時間、案3は売り場の配置の見直しです。")
    rows = [
        ["案", "変えること", "裏づけ", "気になる点"],
        [[("案1", {"bold": True, "size": 18, "color": ACC})],
         [("土日の朝 8時に開店", {"bold": True}), ("\n今は 9時", {"size": 12, "color": SUB})],
         "土日は 1日 240〜260人。午前 11時までに葉物の 6割が売り切れ",
         "朝の搬入が早いのは負担(22戸中 9戸)。開店が早まると搬入も早まる"],
        [[("案2", {"bold": True, "size": 18, "color": ACC})],
         [("平日は夕方 4時に閉店", {"bold": True}), ("\n今は 5時。午後の品出しも減らす", {"size": 12, "color": SUB})],
         "平日の午後 3時以降の客は全体の 12%",
         "夕方に来る客の分は減る。一方、平日の売れ残りを持ち帰る手間は減る"],
        [[("案3", {"bold": True, "size": 18, "color": ACC})],
         [("加工品の棚を入口へ", {"bold": True})],
         "加工品の客 1人あたり 1,450円(野菜 820円の約 1.8倍)。売上は年 860万円",
         "入口にある品の置き場所を決め直す必要がある"],
    ]
    table(s, 0.6, 1.8, 12.13, 3.6, rows, [0.9, 3.1, 4.2, 3.93], size=14,
          row_h=[0.5, 1.03, 1.03, 1.03],
          aligns=[PP_ALIGN.CENTER, PP_ALIGN.LEFT, PP_ALIGN.LEFT, PP_ALIGN.LEFT])
    text(s, 0.6, 5.6, 6, 0.4, "売上の内訳(年、万円)", size=14, bold=True)
    cats = ["野菜", "果物", "加工品", "米", "花"]
    vals = [3420, 1180, 860, 620, 410]
    total = sum(vals)
    x = 0.6
    W = 12.13
    cols = [GREY, GREY, ACC, GREY, GREY]
    for c, v, col in zip(cats, vals, cols):
        w = W * v / total
        b = rect(s, x, 6.0, w - 0.03, 0.55, col)
        box_text(b, [[(f"{c} {v:,}", {"size": 12 if w > 1.2 else 10, "bold": True,
                                      "color": WHITE})]], align=PP_ALIGN.CENTER, margin=0.02)
        x += w
    text(s, 6.6, 5.62, 6.13, 0.4, f"合計 {total:,}万円。加工品は 13%", size=12, color=SUB,
         align=PP_ALIGN.RIGHT)


def slide4(prs):
    s = content_slide(prs, 4, "案2・案3は来年度から。案1は搬入の分担を決めてから",
                      "生産者の負担が増えるのは案1だけです。案1は、朝に出せる方の品で始められるかを話し合いたいと考えます。")
    rows = [
        ["案", "提案", "この会で話し合いたいこと"],
        [[("案1 土日 8時開店", {"bold": True})], [("条件つきで行う", {"bold": True, "color": ACC})],
         "朝の搬入が負担でない 13戸の品で、8時に葉物を並べられるか。負担のある 9戸の品は今どおり"],
        [[("案2 平日 4時閉店", {"bold": True})], [("行う", {"bold": True, "color": ACC})],
         "夕方に来ていた客への知らせ方"],
        [[("案3 加工品を入口へ", {"bold": True})], [("行う", {"bold": True, "color": ACC})],
         "入口から動かす品と、その置き場所"],
    ]
    table(s, 0.6, 1.8, 12.13, 3.1, rows, [2.9, 2.0, 7.23], size=15,
          row_h=[0.5, 1.0, 0.8, 0.8])
    b = rect(s, 0.6, 5.2, 12.13, 1.5, PALE)
    box_text(b, [[("時間を変えるときの知らせ", {"size": 15, "bold": True, "color": ACC})],
                 [("ポイントカードの会員 1,860人(うち月 1回以上来る人 41%、約 760人)に、"
                   "店の掲示とあわせて知らせる。", {"size": 15})]],
             anchor=MSO_ANCHOR.MIDDLE, margin=0.25)
    for p in b.text_frame.paragraphs[1:]:
        p.space_before = Pt(6)


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

