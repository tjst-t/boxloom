import sys
from common import *

ACC = RGBColor(0x1F, 0x4E, 0x9A)
ACC_MID = RGBColor(0x8F, 0xAA, 0xD6)
ACC_PALE = RGBColor(0xE6, 0xEC, 0xF7)
GRAY = RGBColor(0x9A, 0xA0, 0xA8)
FOOT = "家庭用の浄水器 発売の計画|経営会議"
TOTAL = 4

prs = new_deck()

# 1. 表紙
cover(prs, ACC, "経営会議", "家庭用の浄水器(蛇口に付ける型)\n発売の時期と価格",
      "本体は7,980円、カートリッジは1,980円で、7月の発売を提案する",
      ["決めたいこと:本体の価格(3案から1つ)と、発売の時期(7月、遅れたら9月)",
       "資料の中の数はすべて架空のもの"])

# 2. 本体の価格
s = new_slide(prs)
top = slide_title(s, "本体は7,980円がよい。買いたい人が最も多く、粗利の見込みも最大", ACC,
                  "調べ(30〜60代、回答1,500人)の「買いたい」の割合と、本体の原価3,100円から計算した")
text(s, ML, top, CW, 24, "価格の位置(上が当社の3案、下が競合3製品)", size=13, bold=True, name="h-line")
x0, x1 = ML + 50, SW - ML - 50
lo, hi = 6000, 14000


def pos(v):
    return x0 + (v - lo) / float(hi - lo) * (x1 - x0)


ay = top + 72
hline(s, x0 - 20, ay, x1 - x0 + 40, GRAY, 1.5)
for v in range(6000, 14001, 2000):
    vline(s, pos(v), ay - 4, 8, GRAY, 1)
for v, lab in [(7980, "7,980円"), (9980, "9,980円"), (12800, "12,800円")]:
    rect(s, pos(v) - 7, ay - 7, 14, 14, ACC, shape=MSO_SHAPE.OVAL, name="m-" + lab)
    text(s, pos(v) - 50, ay - 40, 100, 26, lab, size=14, bold=True, color=ACC, align='c', anchor='m',
         name="ours-" + lab)
for v, lab in [(6480, "6,480円"), (8900, "8,900円"), (13200, "13,200円")]:
    rect(s, pos(v) - 6, ay - 6, 12, 12, GRAY, shape=MSO_SHAPE.DIAMOND, name="c-" + lab)
    text(s, pos(v) - 50, ay + 12, 100, 22, "競合 " + lab, size=11, color=MID, align='c', anchor='m',
         name="comp-" + lab)

ty = ay + 52
lw = 540
rows = [
    ["本体の価格", "買いたい", "1台の粗利", "回答者1人あたりの\n粗利の見込み"],
    [{'t': "7,980円", 'color': ACC}, {'t': "34%", 'bold': True, 'color': ACC}, "4,880円",
     {'t': "1,659円", 'bold': True, 'color': ACC}],
    ["9,980円", "22%", "6,880円", "1,514円"],
    ["12,800円", "9%", "9,700円", "873円"],
]
tbl, th = table(s, ML, ty, lw, [120, 100, 130, 190], rows, size=13, header_fill=ACC, name="table-price",
                cell_inset=(10, 5, 10, 5), col_align=['l', 'r', 'r', 'r'], max_h=500 - ty)
rx = ML + lw + 20
text(s, rx, ty, SW - ML - rx, 500 - ty, [
    {'t': "読み取れること", 'bold': True, 'size': 14, 'color': ACC},
    {'t': "見込み=買いたい割合×1台の粗利。7,980円が最も大きい", 'bullet': True, 'before': 4},
    {'t': "台数が多いほどカートリッジも売れるので、差はさらに開く", 'bullet': True, 'before': 3},
    {'t': "7,980円は競合の6,480円と8,900円の間に入る", 'bullet': True, 'before': 3},
], size=12, fill=ACC_PALE, inset=(12, 8, 12, 8), name="readout")
footer(s, FOOT + "|粗利は税・流通の費用を含めない単純な計算", 2, TOTAL)

# 3. カートリッジと販売の計画
s = new_slide(prs)
top = slide_title(s, "続ける人のカートリッジ1年分の粗利は、本体の粗利を超える", ACC,
                  "カートリッジは3か月ごとの交換で1,980円(原価420円)。1年後も続ける人は55%の見込み")
lw = 400
text(s, ML, top, lw, 24, "販売の計画(本体、年あたり千台)", size=13, bold=True, name="h-plan")
c = chart(s, XL_CHART_TYPE.COLUMN_CLUSTERED, ML, top + 28, lw, 500 - top - 28,
          ["1年目", "2年目", "3年目"], [("本体", [40, 65, 80])])
style_chart(c, 12)
c.has_legend = False
pl = c.plots[0]
pl.gap_width = 70
pl.has_data_labels = True
dl = pl.data_labels
dl.number_format = '0"千台"'
dl.number_format_is_linked = False
dl.position = XL_LABEL_POSITION.OUTSIDE_END
dl.font.size = Pt(13)
dl.font.bold = True
ser = pl.series[0]
ser.format.fill.solid()
ser.format.fill.fore_color.rgb = ACC
va = c.value_axis
va.minimum_scale = 0
va.maximum_scale = 100
va.major_unit = 20
va.visible = False
va.has_major_gridlines = False
c.category_axis.tick_labels.font.size = Pt(13)
c.category_axis.format.line.color.rgb = RULE

rx = ML + lw + 30
rw = SW - ML - rx
text(s, rx, top, rw, 24, "1人の客から得る粗利(本体7,980円のとき)", size=13, bold=True, name="h-calc")
rows = [
    ["", "売値", "原価", "粗利"],
    ["本体(1回)", "7,980円", "3,100円", "4,880円"],
    ["カートリッジ1本", "1,980円", "420円", "1,560円"],
    [{'t': "カートリッジ1年分(4本)", 'color': ACC}, "7,920円", "1,680円",
     {'t': "6,240円", 'bold': True, 'color': ACC}],
]
tbl, th = table(s, rx, top + 28, rw, [rw - 270, 90, 85, 95], rows, size=13, header_fill=ACC,
                name="table-calc", cell_inset=(8, 5, 8, 5), col_align=['l', 'r', 'r', 'r'], max_h=200)
by = top + 28 + th + 16
text(s, rx, by, rw, 500 - by, [
    {'t': "読み取れること", 'bold': True, 'size': 14, 'color': ACC},
    {'t': "続ける人からは、本体の粗利(4,880円)を上回る6,240円を毎年得る", 'bullet': True, 'before': 4},
    {'t': "1年後に続ける人は55%。続ける割合を上げることが、本体の値付けと同じくらい効く", 'bullet': True, 'before': 3},
    {'t': "1,980円は競合のカートリッジ(1,650〜2,400円)の中ほど", 'bullet': True, 'before': 3},
], size=12, fill=ACC_PALE, inset=(12, 8, 12, 8), name="readout")
footer(s, FOOT + "|1年分は3か月ごとに4本交換するとして計算", 3, TOTAL)

# 4. 日程と懸念
s = new_slide(prs)
top = slide_title(s, "夏の需要に合わせて7月に発売する。弁の調達が遅れれば9月", ACC,
                  "量産の試作から発売まで4か月。部品の弁は取引先が1社だけで、納期が12週かかる")
steps = [("3月", "量産の試作"), ("5月", "品質の確認"), ("7月", "発売(夏の需要)")]
sw_ = 250
for i, (m, what) in enumerate(steps):
    shp = MSO_SHAPE.PENTAGON if i == 0 else MSO_SHAPE.CHEVRON
    fill = ACC if i == 2 else ACC_PALE
    col = WHITE if i == 2 else DARK
    text(s, ML + i * (sw_ + 4), top, sw_, 64, [
        {'t': m, 'size': 13, 'bold': True},
        {'t': what, 'size': 16, 'bold': True},
    ], color=col, fill=fill, shape=shp, inset=(14, 4, 8, 4), anchor='m', spacing=1.25, name="step-%d" % i)
alt_x = ML + 3 * (sw_ + 4) + 8
text(s, alt_x, top, SW - ML - alt_x, 64, [
    {'t': "遅れたとき", 'size': 12, 'color': WARN, 'bold': True},
    {'t': "9月に発売", 'size': 16, 'bold': True, 'color': WARN},
], line=WARN, inset=(10, 4, 8, 4), anchor='m', spacing=1.25, name="step-alt")

cy = top + 64 + 24
cw2 = (CW - 20) / 2
text(s, ML, cy, cw2, 500 - cy, [
    {'t': "懸念:弁の取引先が1社だけ", 'bold': True, 'size': 15, 'color': WARN},
    {'t': "納期は12週(約3か月)。注文の遅れや取引先の問題が、そのまま発売の遅れになる。", 'before': 6},
    {'t': "7月を逃すと夏の需要の山を外す。9月の発売は、遅れたときの代わりとして持っておく。", 'before': 6},
    {'t': "対応の案:量産の分の弁を早めに注文する。並行して2社目の取引先を探す。", 'before': 6, 'bold': True},
], size=13, line=WARN, inset=(14, 10, 14, 10), name="risk")
text(s, ML + cw2 + 20, cy, cw2, 500 - cy, [
    {'t': "今日決めたいこと", 'bold': True, 'size': 15, 'color': ACC},
    {'runs': [("価格:", {'bold': True}), ("本体7,980円、カートリッジ1,980円", {})], 'bullet': True, 'before': 8},
    {'runs': [("時期:", {'bold': True}), ("7月に発売。弁が間に合わなければ9月", {})], 'bullet': True, 'before': 6},
    {'runs': [("弁:", {'bold': True}), ("早めの注文と、2社目の取引先探しを始めること", {})], 'bullet': True, 'before': 6},
], size=13, fill=ACC_PALE, inset=(14, 10, 14, 10), name="decide")
footer(s, FOOT, 4, TOTAL)

save(prs, sys.argv[1])
