import sys
from common import *

ACC = RGBColor(0x1F, 0x6F, 0x8B)
ACC_PALE = RGBColor(0xE3, 0xF0, 0xF4)
FOOT = "オフィスの席の自由化|総務 課長会議"
TOTAL = 4

prs = new_deck()

# 1. 表紙
cover(prs, ACC, "総務 課長会議", "オフィスの席の自由化\n2階での試行を決めたい",
      "席を400から300に減らし、集中の席と打ち合わせの小部屋を足す案を、まず2階で3か月試す",
      ["決めたいこと:2階での試行(3か月・対象140人)を始めるか",
       "資料の中の数はすべて架空のもの"])

# 2. 現状
s = new_slide(prs)
top = slide_title(s, "席の4割は毎日空いている。いちばん多い火曜でも319人", ACC,
                  "社員420人に席400。平均の出社は58%で、曜日によって41〜76%と差が大きい")
text(s, ML, top, 520, 24, "曜日ごとの出社の人数(社員420人×その曜日の出社率)", size=13, bold=True, name="charthead")
days = ["月 64%", "火 76%", "水 71%", "木 62%", "金 41%"]
ppl = [269, 319, 298, 260, 172]
c = chart(s, XL_CHART_TYPE.COLUMN_CLUSTERED, ML, top + 26, 520, 500 - top - 26, days, [("出社の人数", ppl)])
style_chart(c, 12)
c.has_legend = False
pl = c.plots[0]
pl.gap_width = 60
pl.has_data_labels = True
dl = pl.data_labels
dl.number_format = '0"人"'
dl.number_format_is_linked = False
dl.position = XL_LABEL_POSITION.OUTSIDE_END
dl.font.size = Pt(12)
dl.font.bold = True
ser = pl.series[0]
ser.format.fill.solid()
ser.format.fill.fore_color.rgb = ACC
pt = ser.points[4]
pt.format.fill.solid()
pt.format.fill.fore_color.rgb = RGBColor(0x9C, 0xC5, 0xD3)
va = c.value_axis
va.maximum_scale = 400
va.minimum_scale = 0
va.major_unit = 100
va.has_major_gridlines = True
va.major_gridlines.format.line.color.rgb = RULE
va.tick_labels.font.size = Pt(11)
va.format.line.fill.background()
c.category_axis.tick_labels.font.size = Pt(12)
c.category_axis.format.line.color.rgb = RULE

# 右:要点の数
cx, cwid = 600, SW - ML - 600
cards = [
    ("今の席", "400席", "社員420人。席は1人1つではなく、すでに少し足りない数"),
    ("いちばん多い火曜", "空き 81席", "火曜の出社は319人。最も混む日でも2割の席が空く"),
    ("平均の日(58%)", "空き 約156席", "出社は約244人。席の約4割が空く。金曜は228席が空く"),
]
y = top
ch = (500 - top - 2 * 10) / 3
for lab, big, note in cards:
    text(s, cx, y, cwid, ch, [
        {'t': lab, 'size': 13, 'color': MID, 'bold': True},
        {'t': big, 'size': 28, 'color': ACC, 'bold': True, 'before': 2},
        {'t': note, 'size': 12, 'color': DARK, 'before': 4},
    ], fill=ACC_PALE, inset=(12, 8, 12, 8), name="card-" + lab)
    y += ch + 10
footer(s, FOOT + "|出社の人数は 420人×出社率 を四捨五入", 2, TOTAL)

# 3. 案
s = new_slide(prs)
top = slide_title(s, "席を300に減らし、集中の席40と小部屋8つを足す", ACC,
                  "面積は同じまま、使われていない席を、ひとりで集中する場所と少人数の打ち合わせの場所に変える")
lw = 440
text(s, ML, top, lw, 24, "席の構成(今 → 案)", size=13, bold=True, name="h-left")
rows = [
    ["", "今", "案"],
    ["自由に座る席", "400(各自の席)", {'t': "300", 'bold': True, 'color': ACC}],
    ["集中の席", "なし", {'t': "40", 'bold': True, 'color': ACC}],
    ["打ち合わせの小部屋", "なし", {'t': "8つ", 'bold': True, 'color': ACC}],
    ["面積", "—", "同じ"],
]
tbl, th = table(s, ML, top + 26, lw, [170, 150, 120], rows, size=13, header_fill=ACC, name="table-seats",
                cell_inset=(8, 5, 8, 5), max_h=190)
y2 = top + 26 + th + 18
text(s, ML, y2, lw, 24, "費用", size=13, bold=True, name="h-cost")
rows2 = [
    ["項目", "金額", "かかり方"],
    ["家具", "2,300万円", "はじめに1回"],
    ["予約のシステム", "360万円", "毎年"],
    ["賃料", "減らない", "面積が同じため"],
]
tbl2, th2 = table(s, ML, y2 + 26, lw, [170, 130, 140], rows2, size=13, header_fill=MID, name="table-cost",
                  cell_inset=(8, 5, 8, 5), max_h=500 - (y2 + 26))

# 右:席の数の目安(図形で描く横棒)
rx = 520
rw = SW - ML - rx
text(s, rx, top, rw, 24, "席の数の目安:出社の最大の1.1倍", size=13, bold=True, name="h-right")
text(s, rx, top + 24, rw, 40,
     "業界の調べでは、席の数を出社の最大の1.1倍にした会社で満足度が高い。火曜の319人×1.1=約351席",
     size=11, color=MID, name="note-right")
bars = [
    ("今の席", 400, LIGHT),
    ("目安(319×1.1)", 351, RGBColor(0x9C, 0xC5, 0xD3)),
    ("案の席+集中の席", 340, ACC),
    ("火曜の出社", 319, DARK),
]
label_w = 130
bar_x = rx + label_w
bar_max_w = rw - label_w - 64  # 右に数の文字の場所を残す
by = top + 80
bh = 30
for lab, v, col in bars:
    text(s, rx, by, label_w - 6, bh, lab, size=12, anchor='m', name="bl-" + lab)
    bw = bar_max_w * v / 400.0
    rect(s, bar_x, by + 4, bw, bh - 8, col, name="bar-" + lab, track=True)
    text(s, bar_x + bw + 4, by, 60, bh, "%d" % v, size=13, bold=True, anchor='m', name="bv-" + lab)
    by += bh + 8
text(s, rx, by + 8, rw, 500 - by - 8, [
    {'t': "読み取れること", 'bold': True, 'size': 13, 'color': ACC},
    {'t': "自由に座る席300に集中の席40を合わせると340。火曜の319人は座れて、21席が残る", 'bullet': True, 'before': 4},
    {'t': "目安の351にはあと11席足りない。打ち合わせの小部屋を席として数えるかで見方が変わる", 'bullet': True, 'before': 4},
], size=12, fill=PALE, inset=(12, 8, 12, 8), name="readout")
footer(s, FOOT + "|他社の例は業界の調べによる", 3, TOTAL)

# 4. 試行
s = new_slide(prs)
top = slide_title(s, "まず2階だけで3か月、140人で試して確かめる", ACC,
                  "決めたいこと:2階での試行を始めること。心配な点は、試行の中で測って確かめる")
# 左:試行の中身
lw = 230
text(s, ML, top, lw, 24, "試行の中身", size=13, bold=True, name="h-trial")
facts = [("場所", "2階だけ"), ("期間", "3か月"), ("対象", "140人"), ("2階の席", "100")]
fy = top + 26
fh = 54
for k, v in facts:
    text(s, ML, fy, lw, fh, [
        {'t': k, 'size': 12, 'color': MID},
        {'t': v, 'size': 22, 'bold': True, 'color': ACC},
    ], fill=ACC_PALE, inset=(12, 4, 12, 4), anchor='m', name="fact-" + k)
    fy += fh + 8

# 右:心配と確かめ方
rx = ML + lw + 24
rw = SW - ML - rx
text(s, rx, top, rw, 24, "心配な点と、試行で確かめること", size=13, bold=True, name="h-worry")
rows = [
    ["心配な点", "いまの見込み", "試行で確かめること"],
    ["火曜に席が足りるか", {'t': "76%×140人=106人。2階の席100に6人あふれる", 'color': WARN},
     "火曜の空きの席の数と、席を探す時間を毎週測る"],
    ["紙の書類の置き場所", "各自の席が無くなり、机に置いていた書類の行き場が無い",
     "書類の量と、要る置き場所の大きさを調べる"],
    ["部署の会話が減る", "同じ部署の人が離れて座る日が増える", "満足度の調べで、話しやすさを聞く"],
]
tbl, th = table(s, rx, top + 26, rw, [150, 230, rw - 380], rows, size=12, header_fill=ACC,
                name="table-worry", cell_inset=(8, 5, 8, 5), max_h=fy - 8 - (top + 26))
# 下:評価の物差し
my = max(fy, top + 26 + th) + 12
text(s, ML, my, CW, 24, "評価の物差し(試行の前と後で比べる)", size=13, bold=True, name="h-measure")
mw = (CW - 2 * 16) / 3
mh = 500 - (my + 26)
for i, (k, v) in enumerate([("席の空きを探す時間", "出社してから座るまでの時間"),
                            ("満足度", "席・働きやすさ・部署の話しやすさ"),
                            ("会議室の予約の取りやすさ", "小部屋を足した効果を見る")]):
    text(s, ML + i * (mw + 16), my + 26, mw, mh, [
        {'t': k, 'size': 14, 'bold': True, 'color': ACC},
        {'t': v, 'size': 12, 'color': DARK, 'before': 2},
    ], line=ACC, inset=(12, 6, 12, 6), anchor='m', name="measure-%d" % i)
footer(s, FOOT, 4, TOTAL)

save(prs, sys.argv[1])
