"""t03 社内のヘルプデスクの問い合わせ削減(情報システム部長向け)"""
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

DECK = "ヘルプデスクの問い合わせ削減"


def cover(prs):
    s = prs.slides.add_slide(prs.slide_layouts[6])
    rect(s, 0, 0, 13.333, 7.5, ACC)
    text(s, 0.9, 1.0, 11, 0.4, "情報システム部 部長 ご判断資料", size=16, color=ACC_PALE)
    text(s, 0.9, 2.3, 11.5, 0.9, "社内ヘルプデスクの問い合わせ削減", size=40, bold=True, color=WHITE)
    text(s, 0.9, 3.25, 11.5, 0.7, "3 つの施策の優先順位について", size=24, color=WHITE)
    hline(s, 0.9, 4.45, 5.0, WHITE, 1.5)
    text(s, 0.9, 4.7, 11.5, 0.5,
         "ご提案:①パスワードの自己リセット ②よくある質問の作り直し ③会議室の機器の統一 の順",
         size=16, color=WHITE)


def slide2(prs):
    s = content_slide(prs, 2, "問い合わせは月 1,240 件。パスワードが 3 割で最も多い",
                      "対応に月 約 290 時間(6 人で 1 人あたり約 48 時間)。初回で解決するのは 62% にとどまります。")
    text(s, 0.6, 1.75, 6.6, 0.4, "問い合わせの内訳(月 平均、件)", size=15, bold=True)
    cats = ["パスワード", "端末の設定", "権限の申請", "会議室の機器", "その他"]
    vals = [384, 273, 223, 174, 186]
    bar_chart(s, 0.5, 2.15, 6.6, 3.6, cats, vals, horizontal=True,
              colors=[ACC, GREY, GREY, ACC, GREY], size=13, gap=45)
    text(s, 0.6, 5.85, 6.6, 0.9,
         ["件数 = 1,240 件 × 割合(31%・22%・18%・14%・15%)。",
          "青は施策1・施策3が対象とする問い合わせ。施策2(よくある質問)は全体にかかる。"],
         size=12, color=SUB, space=3)
    text(s, 7.7, 1.75, 5.0, 0.4, "月ごとの件数(4〜9 月)", size=15, bold=True)
    bar_chart(s, 7.6, 2.15, 5.15, 2.6, ["4月", "5月", "6月", "7月", "8月", "9月"],
              [1180, 1310, 1260, 1190, 1220, 1280], colors=[GREY] * 6, size=12, gap=40)
    text(s, 7.7, 4.8, 5.0, 0.4, "1,180〜1,310 件で横ばい。減る兆しはない。", size=12, color=SUB)
    kpis = [("14分", "1件の対応 平均"), ("62%", "初回で解決"), ("3.1", "満足度(5点)目標 3.8")]
    for i, (v, l) in enumerate(kpis):
        b = rect(s, 7.7 + i * 1.7, 5.3, 1.6, 1.45, PALE)
        box_text(b, [[(v, {"size": 24, "bold": True, "color": ACC})], [(l, {"size": 11, "color": SUB})]],
                 align=PP_ALIGN.CENTER, margin=0.06)


def slide3(prs):
    s = content_slide(prs, 3, "施策1と施策2を先に。施策3は効果に比べて費用が大きい",
                      "減らせる件数は施策1が最大。1件を減らす費用は施策2が最も安く、施策3はその 10 倍以上です。")
    R, L, C = PP_ALIGN.RIGHT, PP_ALIGN.LEFT, PP_ALIGN.CENTER
    rows = [
        ["順位", "施策", "費用", "減る件数\n(月)", "減る対応時間\n(月)", "1件を減らす費用\n(初年)", "導入"],
        [[("1", {"bold": True, "color": ACC, "size": 18})], [("施策1 パスワードの自己リセット", {"bold": True})],
         "300万円", [("330件", {"bold": True})], "約77時間", "約760円", "2か月"],
        [[("2", {"bold": True, "color": ACC, "size": 18})], [("施策2 よくある質問のページの作り直し", {"bold": True})],
         "80万円", "120件", "約28時間", [("約560円", {"bold": True})], [("1か月", {"bold": True})]],
        [[("3", {"bold": True, "color": SUB, "size": 18})], "施策3 会議室の機器の統一",
         "1,400万円", "150件", "約35時間", [("約7,800円", {"color": WARN, "bold": True})], "6か月"],
    ]
    # header with line breaks
    for j, h in enumerate(rows[0]):
        parts = h.split("\n")
        rows[0][j] = [(parts[0], {})] + [("\n" + p, {"size": 11}) for p in parts[1:]]
    table(s, 0.6, 1.8, 12.13, 2.6, rows, [0.8, 3.9, 1.3, 1.3, 1.6, 2.13, 1.1], size=14,
          row_h=[0.65, 0.65, 0.65, 0.65], aligns=[C, L, R, R, R, R, R])
    text(s, 0.6, 4.45, 12.1, 0.4,
         "減る対応時間 = 減る件数 × 14分。1件を減らす費用 = 費用 ÷(減る件数 × 12か月)。",
         size=12, color=SUB)
    text(s, 0.6, 5.0, 2, 0.4, "考え方", size=16, bold=True)
    pts = [
        [("施策1:", {"bold": True}), ("いちばん多いパスワードの問い合わせに効き、減る件数が最大。2か月で入る。", {})],
        [("施策2:", {"bold": True}), ("費用 80万円・1か月と軽く、施策1と並べて進められる。", {})],
        [("施策3:", {"bold": True}), (" 1件あたりの費用が大きく、導入にも半年。機器の入れ替えの時期に合わせて検討する。", {})],
    ]
    text(s, 0.6, 5.45, 12.1, 1.5, pts, size=15, space=8)


def slide4(prs):
    s = content_slide(prs, 4, "施策1・2で月 450 件(36%)減り、2か月後には約 790 件に",
                      "施策3まで入れると約 640 件(48% 減)。チャットの自動応答(施策4)は効果が読めず、今回は見送ります。")
    text(s, 0.6, 1.75, 6.4, 0.4, "問い合わせの見込み(月、件)", size=15, bold=True)
    bar_chart(s, 0.5, 2.15, 6.5, 3.9, ["今", "施策1の後", "+施策2", "+施策3"],
              [1240, 910, 790, 640], colors=[GREY, ACC, ACC, ACC_PALE], size=13, gap=50)
    text(s, 0.6, 6.15, 6.4, 0.7, "見込みは各施策の削減件数をそのまま足した値(重なりは見ていない)。",
         size=12, color=SUB)
    # roadmap
    text(s, 7.5, 1.75, 5.2, 0.4, "進め方(月)", size=15, bold=True)
    x0, w = 9.6, 3.1
    for k in range(6):
        text(s, x0 + k * w / 6, 2.15, w / 6, 0.3, f"{k + 1}", size=11, color=SUB, align=PP_ALIGN.CENTER)
    items = [("施策2 FAQ", 0, 1, ACC), ("施策1 自己リセット", 0, 2, ACC), ("施策3 機器の統一", 0, 6, GREY)]
    for i, (n, a, b, col) in enumerate(items):
        y = 2.5 + i * 0.55
        text(s, 7.5, y, 2.1, 0.4, n, size=13, anchor=MSO_ANCHOR.MIDDLE)
        rect(s, x0 + a * w / 6, y + 0.07, (b - a) * w / 6, 0.3, col)
    text(s, 7.5, 4.15, 5.2, 0.4, "施策3は判断を後にする場合、開始もずらす。", size=12, color=SUB)
    b = rect(s, 7.5, 4.65, 5.23, 1.95, PALE)
    box_text(b, [[("ご判断いただきたいこと", {"size": 16, "bold": True, "color": ACC})],
                 [("優先順位 ① 施策1 → ② 施策2 → ③ 施策3", {"size": 14, "bold": True})],
                 [("施策1・2 を今期に着手(計 380万円)", {"size": 14})],
                 [("施策3 は機器の入れ替えの時期に改めて", {"size": 14})]],
             anchor=MSO_ANCHOR.TOP, margin=0.22)
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

