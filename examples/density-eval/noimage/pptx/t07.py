import sys
from common import *

ACC = RGBColor(0xB5, 0x4A, 0x1A)
ACC_MID = RGBColor(0xE0, 0x9A, 0x70)
ACC_PALE = RGBColor(0xFB, 0xEC, 0xE3)
FOOT = "地域の防災訓練|自治会 役員会"
TOTAL = 4

prs = new_deck()

# 1. 表紙
cover(prs, ACC, "自治会 役員会", "地域の防災訓練の\n参加者を増やす",
      "参加は5年で半分近くに減った。今年の訓練のやり方を、3つの案から決めたい",
      ["決めたいこと:今年の訓練のやり方(6月に決定、9月第1日曜に本番)",
       "資料の中の数はすべて架空のもの"])


def hbar(s, x, y, w, h, cats, vals, color, highlight=None, maxv=None, fmt='0"%"'):
    c = chart(s, XL_CHART_TYPE.BAR_CLUSTERED, x, y, w, h, cats, [("値", vals)], name="chart-%d" % int(x))
    style_chart(c, 12)
    c.has_legend = False
    pl = c.plots[0]
    pl.gap_width = 50
    pl.has_data_labels = True
    dl = pl.data_labels
    dl.number_format = fmt
    dl.number_format_is_linked = False
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    dl.font.size = Pt(12)
    dl.font.bold = True
    ser = pl.series[0]
    ser.format.fill.solid()
    ser.format.fill.fore_color.rgb = color
    if highlight is not None:
        p = ser.points[highlight]
        p.format.fill.solid()
        p.format.fill.fore_color.rgb = ACC
    ca = c.category_axis
    ca.reverse_order = True
    ca.tick_labels.font.size = Pt(12)
    ca.format.line.color.rgb = RULE
    va = c.value_axis
    va.minimum_scale = 0
    if maxv:
        va.maximum_scale = maxv
    va.visible = False
    va.has_major_gridlines = False
    return c


# 2. 現状
s = new_slide(prs)
top = slide_title(s, "参加は5年で310人から168人に減り、4分の3が60代以上", ACC,
                  "世帯1,240のうち去年の参加は9%。参加しない理由は「日が合わない」が46%でいちばん多い")
lw = 230
text(s, ML, top, lw, 24, "参加の人数", size=13, bold=True, name="h-num")
text(s, ML, top + 28, lw, 104, [
    {'t': "5年前", 'size': 12, 'color': MID},
    {'t': "310人", 'size': 32, 'bold': True, 'color': MID},
], fill=PALE, inset=(12, 6, 12, 6), anchor='m', name="num-old")
text(s, ML, top + 28 + 104 + 10, lw, 104, [
    {'t': "去年", 'size': 12, 'color': MID},
    {'t': "168人", 'size': 32, 'bold': True, 'color': ACC},
], fill=ACC_PALE, inset=(12, 6, 12, 6), anchor='m', name="num-new")
ny = top + 28 + 2 * 104 + 20
text(s, ML, ny, lw, 500 - ny, [
    {'t': "5年で142人(46%)減った。", 'bold': True},
    {'t': "去年の168人は世帯1,240の9%にあたる。", 'before': 2},
], size=12, name="num-note")

mx = ML + lw + 20
mw = 305
text(s, mx, top, mw, 24, "去年の参加者の年代", size=13, bold=True, name="h-age")
hbar(s, mx, top + 28, mw, 230, ["70代以上", "60代", "40〜50代", "30代以下"], [54, 22, 15, 9],
     ACC_MID, highlight=0, maxv=70)
text(s, mx, top + 28 + 236, mw, 500 - (top + 28 + 236), [
    "60代以上が76%。子育ての世代(30代以下)は9%しかいない。",
], size=12, name="age-note")

rx = mx + mw + 20
rw = SW - ML - rx
text(s, rx, top, rw, 24, "参加しない理由(回覧の調べ、回答214)", size=13, bold=True, name="h-why")
hbar(s, rx, top + 28, rw, 230, ["日が合わない", "知らなかった", "内容が毎年同じ"], [46, 31, 24],
     ACC_MID, highlight=0, maxv=60)
text(s, rx, top + 28 + 236, rw, 500 - (top + 28 + 236), [
    "日にちと知らせ方で、理由の大半に手が打てる。",
], size=12, name="why-note")
footer(s, FOOT + "|参加しない理由は回覧の調べ(回答214)", 2, TOTAL)

# 3. 案
s = new_slide(prs)
top = slide_title(s, "3つの案は、応える理由と費用がそれぞれ違う", ACC,
                  "どれか1つにも、組み合わせにもできる。費用はどれも年に数万円から十数万円")
rows = [
    ["案", "やること", "応える理由・ねらう人", "費用"],
    ["案1\n2回にする", "平日の夜と休日の2回に分けて行う",
     "日が合わない(46%)。働いている世代も来やすくなる", {'t': "+6万円", 'bold': True, 'color': ACC}],
    ["案2\n子ども向けの企画", "消火器の体験と非常食の試食を加える",
     "内容が毎年同じ(24%)。参加が9%の30代以下の家族", {'t': "+12万円", 'bold': True, 'color': ACC}],
    ["案3\nマンションごとの訓練", "マンションごとに小さな訓練を年3回行う",
     "日が合わない(46%)。近くで回数が多く、顔見知りが誘いやすい", {'t': "1回2万円\n年3回で6万円", 'bold': True, 'color': ACC}],
]
tw = CW
tbl, th = table(s, ML, top, tw, [190, 270, 280, 140], rows, size=13, header_fill=ACC, name="table-plans",
                cell_inset=(10, 7, 10, 7), max_h=270)
by = top + th + 16
bw = (CW - 16) / 2
bh = 500 - by
text(s, ML, by, bw, bh, [
    {'t': "どの案でも要ること:知らせ方", 'bold': True, 'size': 14, 'color': ACC},
    {'t': "「知らなかった」が31%。8月の案内を回覧だけでなく、マンションの掲示や子どもの学校経由でも届ける。", 'before': 4},
], size=12, fill=ACC_PALE, inset=(14, 10, 14, 10), name="box-notice")
text(s, ML + bw + 16, by, bw, bh, [
    {'t': "組み合わせたときの費用(年)", 'bold': True, 'size': 14, 'color': ACC},
    {'t': "案1+案2:+18万円", 'bullet': True, 'before': 4},
    {'t': "案1+案3:+12万円", 'bullet': True, 'before': 2},
    {'t': "3つとも:+24万円", 'bullet': True, 'before': 2},
], size=12, fill=PALE, inset=(14, 10, 14, 10), name="box-combo")
footer(s, FOOT + "|案3の費用は1回2万円を年3回として計算", 3, TOTAL)

# 4. 準備と日程
s = new_slide(prs)
top = slide_title(s, "6月に決めて、9月第1日曜の本番に向けて準備する", ACC,
                  "今日決めたいのはやり方。人が増えても避難場所は足りるが、要配慮の人の支援者が足りない")
steps = [("6月", "やり方を決める", "今日の役員会"), ("8月", "案内を出す", "回覧・掲示など"),
         ("9月第1日曜", "本番", "小学校の体育館")]
sw_ = (CW - 2 * 4) / 3
for i, (m, what, note) in enumerate(steps):
    shp = MSO_SHAPE.PENTAGON if i == 0 else MSO_SHAPE.CHEVRON
    fill = ACC if i == 2 else ACC_PALE
    col = WHITE if i == 2 else DARK
    text(s, ML + i * (sw_ + 4), top, sw_, 70, [
        {'runs': [(m + "  ", {'bold': True, 'size': 18}), (what, {'size': 15, 'bold': True})]},
        {'t': note, 'size': 12, 'before': 2},
    ], color=col, fill=fill, shape=shp, inset=(34, 6, 30, 6), anchor='m', name="step-%d" % i)

cy = top + 70 + 24
cw2 = (CW - 20) / 2
ch2 = 500 - cy
# 左:避難場所
x0 = ML
text(s, x0, cy, cw2, 24, "避難場所:小学校の体育館(収容450人)", size=13, bold=True, name="h-place")
scale = (cw2 - 70) / 450.0
items = [("収容", 450, LIGHT), ("5年前の参加", 310, ACC_MID), ("去年の参加", 168, ACC)]
yy = cy + 34
for lab, v, col in items:
    text(s, x0, yy, 100, 26, lab, size=12, anchor='m', name="pl-" + lab)
    rect(s, x0 + 104, yy + 4, (cw2 - 104 - 56) * v / 450.0, 18, col, name="pb-" + lab, track=True)
    text(s, x0 + 104 + (cw2 - 104 - 56) * v / 450.0 + 4, yy, 52, 26, "%d人" % v, size=12, bold=True,
         anchor='m', name="pv-" + lab)
    yy += 34
text(s, x0, yy + 6, cw2, 500 - yy - 6,
     "参加が5年前の水準に戻っても、体育館には140人分の余裕がある。",
     size=12, color=MID, name="place-note")

# 右:要配慮の人
x1 = ML + cw2 + 20
text(s, x1, cy, cw2, 24, "要配慮の人の名簿(登録86人)", size=13, bold=True, name="h-care")
bw_full = cw2
seg1 = bw_full * 39 / 86.0
text(s, x1, cy + 34, seg1, 44, "支援する人が\n決まっている 39人", size=11, color=WHITE, bold=True,
     fill=ACC, inset=(6, 3, 6, 3), anchor='m', spacing=1.2, name="care-yes")
text(s, x1 + seg1, cy + 34, bw_full - seg1, 44, "まだ決まっていない 47人", size=12, color=WARN,
     bold=True, fill=ACC_PALE, inset=(8, 3, 6, 3), anchor='m', name="care-no")
text(s, x1, cy + 34 + 44 + 12, cw2, 500 - (cy + 34 + 44 + 12), [
    {'t': "名簿の半分以上(55%)は、災害のときに誰が手を貸すかが決まっていない。", 'bold': True},
    {'t': "訓練の案内と一緒に、支援する人を募ることを提案する。", 'before': 4},
], size=12, name="care-note")
footer(s, FOOT, 4, TOTAL)

save(prs, sys.argv[1])
