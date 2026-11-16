"""t01 市の図書館の開館時間の見直し(教育委員会向け)"""
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

DECK = "市立図書館 開館時間の見直し"
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


# ---------------------------------------------------------------- slides

def cover(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, ACC)
    text(s, 0.9, 1.0, 11, 0.4, "教育委員会 審議資料", size=16, color=ACC_PALE)
    text(s, 0.9, 2.3, 11.5, 1.6,
         ["市立図書館の開館時間の見直し", "駅前館の平日夜間延長(案A)の試行について"],
         size=40, bold=True, color=WHITE, space=10)
    # make second line smaller
    tb = s.shapes[-1]
    for r in tb.text_frame.paragraphs[1].runs:
        r.font.size = Pt(26)
        r.font.bold = False
    hline(s, 0.9, 4.45, 5.0, WHITE, 1.5)
    text(s, 0.9, 4.7, 11.5, 1.2,
         ["お決めいただきたいこと:駅前館を平日 21 時まで開館する試行(年 1,800 万円)の可否",
          "市立図書館"],
         size=16, color=WHITE, space=8)


def slide2(prs):
    s = content_slide(prs, 2, "夕方の需要が駅前館に集中し、閉館前に入れない人が週 140 人",
                      "利用は中央館・駅前館に偏り、駅前館では閉館の時刻が近づくほど人が増えています。")
    # left: annual users
    text(s, 0.6, 1.75, 5.6, 0.4, "館ごとの利用者数(年、千人)", size=15, bold=True)
    bar_chart(s, 0.5, 2.15, 5.8, 3.3, ["中央館", "駅前館", "北館", "南館"],
              [412, 268, 95, 121], horizontal=True,
              colors=[GREY, ACC, GREY, GREY], size=13, gap=50)
    text(s, 0.6, 5.55, 5.6, 0.9,
         ["4館とも 平日 9〜18 時、土日 9〜17 時の開館。",
          "駅前館は中央館に次ぐ 2 番目の利用。"], size=13, color=SUB, space=2)
    # right: hourly
    text(s, 6.9, 1.75, 5.8, 0.4, "駅前館 平日の時間帯別 利用(1 時間あたり、人)", size=15, bold=True)
    bar_chart(s, 6.8, 2.15, 3.6, 3.3, ["9時台", "12時台", "17時台"], [18, 41, 63],
              colors=[GREY, GREY, ACC], size=13, gap=45)
    # callout
    c = rect(s, 10.6, 2.35, 2.13, 2.9, PALE)
    box_text(c, [[("閉館前 30 分に", {"size": 13, "color": SUB})],
                 [("入れなかった人", {"size": 13, "color": SUB})],
                 [("週 140 人", {"size": 26, "bold": True, "color": WARN})],
                 [("(平均)", {"size": 12, "color": SUB})]],
             align=PP_ALIGN.CENTER)
    text(s, 6.9, 5.55, 5.9, 0.9,
         ["17 時台は 9 時台の約 3.5 倍。",
          "仕事・学校の帰りに寄りたい人を、18 時の閉館で取りこぼしています。"],
         size=13, color=SUB, space=2)


def slide3(prs):
    s = content_slide(prs, 3, "3 つの案のうち、利用 1 人あたりの費用は案Aが最も小さい",
                      "案Aを推します。案Bは効果が 1.6 倍でも費用が 2.9 倍、案Cは貸出の受け取りに限られます。")
    rows = [
        ["", "案A 駅前館だけ延長", "案B 全館延長", "案C 駅前館に受け取り棚"],
        ["内容", "駅前館を平日 21 時まで", "全館を平日 20 時まで", "予約本を24時間受け取れる棚"],
        ["費用", [("年 1,800 万円", {"bold": True})], [("年 5,200 万円", {"bold": True})],
         [("初年 900 万円", {"bold": True}), ("\n以降 年 150 万円", {})]],
        ["見込み", [("利用 年 +38 千人", {"bold": True})], [("利用 年 +61 千人", {"bold": True})],
         [("受け取り 年 +45 千件", {"bold": True}), ("\n(来館・閲覧は増えない)", {"size": 12, "color": SUB})]],
        ["1 人あたり費用", [("約 470 円", {"bold": True, "color": ACC})], "約 850 円",
         [("—", {}), ("\n(件あたり 初年 約 200 円)", {"size": 12, "color": SUB})]],
        ["人手", "司書 2 名増", "司書 7 名増", "増員なし"],
        ["主な懸念", "光熱費 年 +6%、司書の研修", [("北館周辺の夜の人通りが少ない", {}), ("\n7名の確保と研修", {})], "本を借りる以外の利用には効かない"],
    ]
    table(s, 0.6, 1.75, 12.13, 4.6, rows, [1.9, 3.3, 3.3, 3.63], size=14,
          row_h=[0.5, 0.6, 0.75, 0.75, 0.75, 0.5, 0.75])
    # highlight column A
    hl = s.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(2.5), Inches(1.75), Inches(3.3), Inches(4.6))
    hl.fill.background()
    hl.line.color.rgb = ACC
    hl.line.width = Pt(3)
    hl.shadow.inherit = False
    text(s, 0.6, 6.5, 12.1, 0.4,
         "1 人あたり費用 = 年の費用 ÷ 見込みの利用増。司書は会計年度任用職員。光熱費の増は延長する館に共通。",
         size=12, color=SUB)


def slide4(prs):
    s = content_slide(prs, 4, "4 月に試行を決めれば、9 月に始めて翌 3 月に評価できる",
                      "まずは 1 年の試行とし、評価をもとに続けるか・他の館へ広げるかを改めてお諮りします。")
    steps = [("4月", "試行の決定", "本日の審議"), ("6月", "職員の募集", "司書 2 名"),
             ("9月", "試行の開始", "平日 21 時まで"), ("翌3月", "評価", "利用者数・費用")]
    x0, w, gap = 0.6, 2.95, 0.11
    for i, (m, t, d) in enumerate(steps):
        x = x0 + i * (w + gap)
        shp = MSO_SHAPE.PENTAGON if i == 0 else MSO_SHAPE.CHEVRON
        b = rect(s, x, 1.85, w + 0.25, 0.95, ACC if i == 0 else ACC_PALE, shape=shp)
        b.adjustments[0] = 0.3
        box_text(b, [[(m + "  ", {"size": 14, "color": WHITE if i == 0 else ACC, "bold": True}),
                      (t, {"size": 16, "bold": True, "color": WHITE if i == 0 else INK})]],
                 align=PP_ALIGN.CENTER, margin=0.05)
        text(s, x + 0.25, 2.9, w - 0.3, 0.4, d, size=13, color=SUB, align=PP_ALIGN.CENTER)

    text(s, 0.6, 3.55, 7.2, 0.4, "懸念と対応", size=16, bold=True)
    rows = [
        ["懸念", "対応"],
        ["司書の研修", "6 月の採用後、9 月の開始までに研修の期間を確保"],
        ["光熱費 年 +6%", "試行の 1 年で実額を測り、評価に含める"],
        ["北館周辺の夜の人通り", "案Aは駅前館のみで該当しない。広げる際に改めて検討"],
    ]
    table(s, 0.6, 3.95, 7.4, 2.4, rows, [2.4, 5.0], size=13, row_h=0.6)

    b = rect(s, 8.4, 3.55, 4.33, 2.8, PALE)
    box_text(b, [[("お決めいただきたいこと", {"size": 16, "bold": True, "color": ACC})],
                 [("駅前館を平日 21 時まで開館する 1 年の試行", {"size": 15, "bold": True})],
                 [("・費用 年 1,800 万円", {"size": 14})],
                 [("・司書 2 名増(会計年度任用職員)", {"size": 14})],
                 [("・見込み 利用 年 +38 千人", {"size": 14})]],
             anchor=MSO_ANCHOR.TOP, margin=0.25)
    for p in b.text_frame.paragraphs[1:]:
        p.space_before = Pt(8)


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
