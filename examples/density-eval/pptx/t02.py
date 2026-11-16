"""t02 町の路線バスの再編(議会の委員会向け)"""
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

DECK = "町の路線バス 3路線の再編"


def cover(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, ACC)
    text(s, 0.9, 1.0, 11, 0.4, "議会 委員会 説明資料", size=16, color=ACC_PALE)
    text(s, 0.9, 2.3, 11.5, 0.9, "町の路線バス 3路線の再編案", size=40, bold=True, color=WHITE)
    text(s, 0.9, 3.25, 11.5, 0.7, "東線は残し、西線は病院へ寄る形に、山手線は予約の乗合の車に",
         size=24, color=WHITE)
    hline(s, 0.9, 4.45, 5.0, WHITE, 1.5)
    text(s, 0.9, 4.7, 11.5, 0.5, "お願いしたいこと:再編の案へのご賛同(10月から試行)",
         size=16, color=WHITE)


def slide2(prs):
    s = content_slide(prs, 2, "西線・山手線は乗る人が少なく、町の補助の 7 割を占める",
                      "補助は年 6,750 万円。1 人を運ぶのにかかる補助は、山手線が東線の約 9 倍です。")
    rows = [
        ["路線", [("便数", {}), ("\n1日", {"size": 12})], [("乗る人", {}), ("\n1便あたり", {"size": 12})], [("乗る人", {}), ("\n1日", {"size": 12})], [("赤字", {}), ("\n年", {"size": 12})], [("補助", {}), ("\n1人あたり", {"size": 12})]],
        ["東線", "24便", "6.1人", "約146人", "2,100万円", "約390円"],
        ["西線", "18便", "3.2人", "約58人", "2,900万円", "約1,380円"],
        ["山手線", "10便", "1.4人", "約14人", "1,750万円", "約3,420円"],
        [[("合計", {"bold": True})], "52便", "", "約218人", [("6,750万円", {"bold": True})], ""],
    ]
    R = PP_ALIGN.RIGHT
    table(s, 0.6, 1.8, 7.3, 3.0, rows, [1.2, 1.15, 1.15, 1.2, 1.3, 1.3], size=14,
          row_h=[0.75, 0.55, 0.55, 0.55, 0.55], aligns=[PP_ALIGN.LEFT, R, R, R, R, R])
    text(s, 0.6, 4.95, 7.3, 0.9,
         ["1日の乗る人 = 便数 × 1便あたり。1人あたり補助 = 赤字 ÷(1日の利用 × 365日)の試算。",
          "西線と山手線の赤字は合わせて 4,650 万円(補助の 69%)。"],
         size=12, color=SUB, space=3)
    text(s, 8.4, 1.75, 4.3, 0.4, "1人を運ぶのにかかる補助(円)", size=15, bold=True)
    bar_chart(s, 8.3, 2.15, 4.45, 2.9, ["東線", "西線", "山手線"], [390, 1380, 3420],
              horizontal=True, colors=[GREY, ACC, ACC], size=13, gap=45)
    b = rect(s, 0.6, 5.85, 12.13, 0.95, PALE)
    box_text(b, [[("町の様子も変わっています  ", {"size": 14, "bold": True, "color": ACC}),
                  ("65歳以上 38%(10年前 29%)・運転免許の返納 年 140件。"
                   "車を使えない人の足を残す形への切り替えが要ります。", {"size": 14})]],
             margin=0.2)


def slide3(prs):
    s = content_slide(prs, 3, "3 路線それぞれに合った形に変える",
                      "乗る人の多い東線は今のまま。少ない路線は便を絞るか、予約で走る車に置き換えます。")
    cards = [
        ("東線", "残す", "1日 24便のまま", ["1便あたり 6.1人と最も多い", "今の便数を保つ"]),
        ("西線", "便を減らし病院へ", "1日 18便 → 12便", ["病院に寄る経路にする", "便が減る分、使われる行き先へ"]),
        ("山手線", "予約の乗合の車へ", "定時の便 → デマンド", ["1日 30回の予約を見込む", "費用 年 1,100万円", "待ち 平均 25分"]),
    ]
    w, gap = 3.9, 0.215
    for i, (name, verb, change, pts) in enumerate(cards):
        x = 0.6 + i * (w + gap)
        hd = rect(s, x, 1.8, w, 0.6, ACC)
        box_text(hd, [[(name, {"size": 18, "bold": True, "color": WHITE}),
                       ("  " + verb, {"size": 16, "color": WHITE})]], margin=0.18)
        body = rect(s, x, 2.4, w, 1.9, WHITE, line=LINE)
        paras = [[(change, {"size": 17, "bold": True, "color": ACC})]]
        paras += [[("・" + p, {"size": 14})] for p in pts]
        box_text(body, paras, anchor=MSO_ANCHOR.TOP, margin=0.2)
        for p in body.text_frame.paragraphs[1:]:
            p.space_before = Pt(6)
    text(s, 0.6, 4.6, 12.1, 0.4, "住民の調べ(回答 812人)との対応", size=15, bold=True)
    rows = [
        ["バスを使わない理由(複数回答)", "割合", "再編で考えること"],
        ["車がある", "52%", "高齢化と免許返納で、車を使えない人は増えていく"],
        ["便が少ない", "41%", "西線は便が減る。病院に寄り、使われる行き先に絞る"],
        ["停留所が遠い", "27%", "山手線のデマンドで、乗り降りの場所の決め方を試行で確かめる"],
    ]
    table(s, 0.6, 5.0, 12.13, 1.8, rows, [3.4, 1.0, 7.73], size=14, row_h=0.45,
          aligns=[PP_ALIGN.LEFT, PP_ALIGN.RIGHT, PP_ALIGN.LEFT])


def slide4(prs):
    s = content_slide(prs, 4, "5月に説明会、10月から半年の試行を経て翌4月に本格運行",
                      "試行の半年で予約の数・待ち時間を確かめ、本格運行の前に委員会へご報告します。")
    timeline(s, 1.85, [("5月", "住民説明会", "路線ごとに開催"),
                       ("10月", "試行の開始", "翌3月まで半年"),
                       ("翌4月", "本格運行", "試行の結果で調整")])
    text(s, 0.6, 3.6, 7.5, 0.4, "ご心配の点と今の考え", size=16, bold=True)
    rows = [
        ["心配", "今の考え"],
        ["デマンドの予約は電話だけでよいか",
         "高齢の方は電話が使いやすい。ほかの受け方は説明会で意見を聞き、試行の前に決める"],
        ["山手線で通学する 8人の扱い",
         "登下校の時刻に合わせた乗り方を、学校・家庭と試行の前に決める"],
    ]
    table(s, 0.6, 4.0, 7.5, 2.4, rows, [2.7, 4.8], size=13, row_h=[0.5, 0.95, 0.95])
    b = rect(s, 8.5, 3.6, 4.23, 2.8, PALE)
    box_text(b, [[("ご賛同をお願いしたいこと", {"size": 16, "bold": True, "color": ACC})],
                 [("3路線の再編の案", {"size": 15, "bold": True})],
                 [("・東線 24便を維持", {"size": 14})],
                 [("・西線 12便、病院に寄る", {"size": 14})],
                 [("・山手線 デマンドへ(年 1,100万円)", {"size": 14})]],
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

