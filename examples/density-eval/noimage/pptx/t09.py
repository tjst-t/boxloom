import sys
from common import *

ACC = RGBColor(0x2E, 0x7D, 0x5B)
ACC_MID = RGBColor(0x96, 0xC4, 0xAE)
ACC_PALE = RGBColor(0xE5, 0xF2, 0xEC)
FOOT = "外来の待ち時間の短縮|病院の運営会議"
TOTAL = 4

prs = new_deck()

# 1. 表紙
cover(prs, ACC, "病院の運営会議", "外来の待ち時間を短くする\n3つの手",
      "受付から診察までの待ちは平均68分。2年で全体50分以下、内科60分以下を目指す",
      ["決めたいこと:取り組む3つの手(検査を先に・自動精算機・呼び出しの通知)",
       "資料の中の数はすべて架空のもの"])


def hbar(s, x, y, w, h, cats, vals, hl, maxv, name):
    c = chart(s, XL_CHART_TYPE.BAR_CLUSTERED, x, y, w, h, cats, [("分", vals)], name=name)
    style_chart(c, 12)
    c.has_legend = False
    pl = c.plots[0]
    pl.gap_width = 50
    pl.has_data_labels = True
    dl = pl.data_labels
    dl.number_format = '0"分"'
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    dl.font.size = Pt(12)
    dl.font.bold = True
    ser = pl.series[0]
    ser.format.fill.solid()
    ser.format.fill.fore_color.rgb = ACC_MID
    for i in hl:
        p = ser.points[i]
        p.format.fill.solid()
        p.format.fill.fore_color.rgb = ACC
    ca = c.category_axis
    ca.reverse_order = True
    ca.tick_labels.font.size = Pt(12)
    ca.format.line.color.rgb = RULE
    va = c.value_axis
    va.minimum_scale = 0
    va.maximum_scale = maxv
    va.visible = False
    va.has_major_gridlines = False
    return c


# 2. 現状
s = new_slide(prs)
top = slide_title(s, "外来の待ちは平均68分。内科は92分と最も長い", ACC,
                  "外来は1日平均820人。患者の調べ(回答640)でも、不満の1位は待ち時間(58%)")
cw2 = (CW - 30) / 2
text(s, ML, top, cw2, 24, "受付から診察までの待ち(平均)", size=13, bold=True, name="h-dept")
hbar(s, ML, top + 28, cw2, 220, ["内科", "整形外科", "全体", "小児科"], [92, 74, 68, 41], [0], 110,
     "chart-dept")
text(s, ML, top + 28 + 228, cw2, 500 - (top + 28 + 228), [
    {'t': "2年での目標", 'bold': True, 'color': ACC, 'size': 13},
    {'runs': [("内科 ", {'bold': True}), ("92分 → 60分以下(32分縮める)", {})], 'bullet': True, 'before': 3},
    {'runs': [("全体 ", {'bold': True}), ("68分 → 50分以下(18分縮める)", {})], 'bullet': True, 'before': 2},
], size=12, fill=ACC_PALE, inset=(12, 8, 12, 8), name="target")

rx = ML + cw2 + 30
text(s, rx, top, cw2, 24, "内科の待ちの内訳(重なりがあり、合計は92分を超える)", size=13, bold=True, name="h-split")
hbar(s, rx, top + 28, cw2, 220, ["受付", "検査", "診察の前", "会計"], [8, 26, 52, 18], [2], 65,
     "chart-split")
text(s, rx, top + 28 + 228, cw2, 500 - (top + 28 + 228), [
    {'t': "診察の前の52分が最も長い", 'bold': True, 'color': ACC, 'size': 13},
    {'t': "予約の患者は54%。しかし予約の時刻どおりに始まったのは31%だけ", 'bullet': True, 'before': 3},
    {'t': "検査の26分と会計の18分も、手を打てる長さ", 'bullet': True, 'before': 2},
], size=12, fill=PALE, inset=(12, 8, 12, 8), name="split-note")
footer(s, FOOT + "|患者の調べは回答640", 2, TOTAL)

# 3. 3つの手
s = new_slide(prs)
top = slide_title(s, "3つの手で、検査・会計・待つ場所の混みを減らす", ACC,
                  "内科の待ちの内訳と患者の不満に、1つずつ対応させた")
rows = [
    ["手", "やること", "縮める所", "費用"],
    ["1. 検査を先に受ける流れ",
     "予約の前日に検査の時刻を知らせ、診察の前に検査を済ませておく",
     "検査26分と診察の前52分。予約どおりに始まる割合(今31%)を上げる",
     {'t': "見積もりが要る", 'color': MID}],
    ["2. 会計の自動精算機",
     "会計に自動精算機を4台置く",
     "会計18分。会計の職員6人の手が空く",
     {'t': "2,400万円\n(1台600万円×4)", 'bold': True, 'color': ACC}],
    ["3. 呼び出しの通知",
     "診察の呼び出しをスマートフォンに知らせ、待合の外でも待てるようにする",
     "待つ場所の混み(不満の2位、21%)。待ちの体感を軽くする",
     {'t': "年240万円", 'bold': True, 'color': ACC}],
]
tbl, th = table(s, ML, top, CW, [190, 280, 260, 150], rows, size=13, header_fill=ACC, name="table-plans",
                cell_inset=(10, 7, 10, 7), max_h=300)
by = top + th + 16
text(s, ML, by, CW, 500 - by, [
    {'t': "効き方の違い", 'bold': True, 'size': 14, 'color': ACC},
    {'t': "1は待ちの時間そのもの(最も長い診察の前)を縮める本命。2は会計の時間、3は待つ場所の混みと不満に効く。", 'before': 4},
], size=12, fill=ACC_PALE, inset=(14, 8, 14, 8), name="box-effect")
footer(s, FOOT + "|待ちの数は内科の平均", 3, TOTAL)

# 4. 目標と決めること
s = new_slide(prs)
top = slide_title(s, "2年で内科60分以下、全体50分以下を目指す", ACC,
                  "今日決めたいのは、3つの手に取り組むことと、その費用")
lw = 440
text(s, ML, top, lw, 24, "今と目標(受付から診察までの待ち)", size=13, bold=True, name="h-gap")
scale = (lw - 110 - 84) / 100.0
yy = top + 40
for lab, now, goal in [("内科", 92, 60), ("全体", 68, 50)]:
    text(s, ML, yy, lw, 26, lab, size=14, bold=True, color=ACC, name="gl-" + lab)
    yy += 30
    for sub, v, col in [("今", now, LIGHT), ("目標", goal, ACC)]:
        text(s, ML, yy, 100, 28, sub, size=12, anchor='m', name="gs-%s-%s" % (lab, sub))
        rect(s, ML + 110, yy + 4, v * scale, 20, col, name="gb-%s-%s" % (lab, sub), track=True)
        text(s, ML + 110 + v * scale + 4, yy, 80, 28, ("%d分" % v) + ("" if sub == "今" else "以下"),
             size=12, bold=True, anchor='m', name="gv-%s-%s" % (lab, sub))
        yy += 32
    text(s, ML + 110, yy - 2, lw - 110, 22, "%d分縮める" % (now - goal), size=12, color=WARN, bold=True,
         name="gd-" + lab)
    yy += 30
text(s, ML, yy, lw, 500 - yy, "職員は受付12人、会計6人。精算機で空いた会計の手を、案内に回せるかも検討する。",
     size=12, color=MID, name="staff-note")

rx = ML + lw + 30
rw = SW - ML - rx
text(s, rx, top, rw, 500 - top, [
    {'t': "今日決めたいこと", 'bold': True, 'size': 15, 'color': ACC},
    {'runs': [("手1:", {'bold': True}), ("検査を先に受ける流れを内科から始め、費用を見積もる", {})], 'bullet': True, 'before': 8},
    {'runs': [("手2:", {'bold': True}), ("自動精算機4台、2,400万円", {})], 'bullet': True, 'before': 6},
    {'runs': [("手3:", {'bold': True}), ("呼び出しの通知、年240万円", {})], 'bullet': True, 'before': 6},
    {'t': "効果の測り方", 'bold': True, 'size': 15, 'color': ACC, 'before': 16},
    {'t': "科ごとの待ちと、予約どおりに始まった割合を毎月測る", 'bullet': True, 'before': 8},
    {'t': "患者の調べを繰り返し、待ち時間への不満の割合(今58%)を見る", 'bullet': True, 'before': 6},
], size=13, fill=ACC_PALE, inset=(16, 12, 16, 12), name="decide")
footer(s, FOOT, 4, TOTAL)

save(prs, sys.argv[1])
