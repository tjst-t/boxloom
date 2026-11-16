import sys
from common import *

ACC = RGBColor(0x6A, 0x3D, 0x9A)
ACC_MID = RGBColor(0xB9, 0xA3, 0xD3)
ACC_PALE = RGBColor(0xEF, 0xE9, 0xF6)
SEG = [RGBColor(0x6A, 0x3D, 0x9A), RGBColor(0x93, 0x6F, 0xBE), RGBColor(0xB9, 0xA3, 0xD3)]
FOOT = "留学生の生活の支え|学生支援の委員会"
TOTAL = 4

prs = new_deck()

# 1. 表紙
cover(prs, ACC, "学生支援の委員会", "大学の留学生の生活の支え\n来年度の3つの施策と予算",
      "住まい・手続き・孤立の困りごとに3つの施策で応える。3つとも行うと予算の枠を270万円超える",
      ["決めたいこと:来年度に行う施策と予算(年1,000万円の枠の中で)。2月に決めて4月に始める",
       "資料の中の数はすべて架空のもの"])

# 2. 困りごと
s = new_slide(prs)
top = slide_title(s, "困りごとの1位は住まい探し、2位は日本語の手続き", ACC,
                  "留学生は380人(学部140・大学院240)、出身は32の国と地域。困りごとの調べは回答211")
lw = 470
text(s, ML, top, lw, 24, "困っていること(調べ、回答211)", size=13, bold=True, name="h-chart")
c = chart(s, XL_CHART_TYPE.BAR_CLUSTERED, ML, top + 28, lw, 500 - top - 28,
          ["住まい探し", "日本語の手続き", "アルバイト探し", "孤立", "病院"], [("割合", [48, 44, 31, 27, 22])])
style_chart(c, 12)
c.has_legend = False
pl = c.plots[0]
pl.gap_width = 45
pl.has_data_labels = True
dl = pl.data_labels
dl.number_format = '0"%"'
dl.number_format_is_linked = False
dl.position = XL_LABEL_POSITION.OUTSIDE_END
dl.font.size = Pt(13)
dl.font.bold = True
ser = pl.series[0]
ser.format.fill.solid()
ser.format.fill.fore_color.rgb = ACC_MID
for i in (0, 1):
    p = ser.points[i]
    p.format.fill.solid()
    p.format.fill.fore_color.rgb = ACC
ca = c.category_axis
ca.reverse_order = True
ca.tick_labels.font.size = Pt(13)
ca.format.line.color.rgb = RULE
va = c.value_axis
va.minimum_scale = 0
va.maximum_scale = 60
va.visible = False
va.has_major_gridlines = False

rx = ML + lw + 30
rw = SW - ML - rx
cards = [
    ("住まい", "寮の空き待ち 64人", "大学の寮は120室。民間の部屋では、保証人がいない人が37%"),
    ("手続き", "日本語の手続き 44%", "役所・銀行・病院などの手続きが日本語だけで進む"),
    ("退学・休学", "去年 14人", "理由の1位は経済(6人)。住まいとアルバイトの困りごとにつながる"),
]
gap = 10
ch = (500 - top - 2 * gap) / 3
y = top
for lab, big, note in cards:
    text(s, rx, y, rw, ch, [
        {'t': lab, 'size': 12, 'color': MID, 'bold': True},
        {'t': big, 'size': 22, 'color': ACC, 'bold': True, 'before': 2},
        {'t': note, 'size': 12, 'before': 3},
    ], fill=ACC_PALE, inset=(12, 6, 12, 6), anchor='m', name="card-" + lab)
    y += ch + gap
footer(s, FOOT + "|困りごとは複数の回答", 2, TOTAL)

# 3. 3つの施策
s = new_slide(prs)
top = slide_title(s, "3つの施策を合わせると年1,270万円。枠を270万円超える", ACC,
                  "どの施策も上位の困りごとに応えるが、予算の枠は年1,000万円")
rows = [
    ["施策", "中身", "応える困りごと", "年の費用"],
    ["1. 保証人の代わり", "民間の部屋を借りるとき、大学が保証人の代わりになる(積み立て)",
     "住まい探し(48%)。保証人のいない37%", {'t': "300万円", 'bold': True, 'color': ACC}],
    ["2. 日本人学生の相棒", "日本人学生の相棒(バディ)を1人ずつ付け、謝礼を払う",
     "孤立(27%)。生活の相談の相手", {'t': "450万円", 'bold': True, 'color': ACC}],
    ["3. 手続きの付き添いの窓口", "職員1名の窓口で、手続きに付き添う",
     "日本語の手続き(44%)。病院(22%)", {'t': "520万円", 'bold': True, 'color': ACC}],
]
tbl, th = table(s, ML, top, CW, [200, 330, 230, 120], rows, size=13, header_fill=ACC, name="table-plans",
                cell_inset=(10, 6, 10, 6), col_align=['l', 'l', 'l', 'r'], max_h=250)
by = top + th + 20
text(s, ML, by, CW, 24, "年の費用の積み上げと予算の枠(万円)", size=13, bold=True, name="h-budget")
bx = ML
bwid = CW - 60
scale = bwid / 1270.0
yb = by + 56
x = bx
for i, (lab, v) in enumerate([("施策1 300", 300), ("施策2 450", 450), ("施策3 520", 520)]):
    text(s, x, yb, v * scale, 40, lab, size=13, bold=True, color=WHITE, fill=SEG[i], align='l', anchor='m',
         inset=(8, 3, 4, 3), name="seg-%d" % i, track=False)
    x += v * scale
text(s, x + 4, yb, 56, 40, "1,270", size=13, bold=True, anchor='m', name="seg-total")
lx = bx + 1000 * scale
vline(s, lx, yb - 24, 40 + 32, WARN, 2)
text(s, lx - 140, yb - 30, 136, 24, "枠 1,000", size=13, bold=True, color=WARN, align='r', anchor='m',
     name="limit-label")
text(s, lx + 4, yb - 30, 200, 24, "超える分 270", size=13, bold=True, color=WARN, anchor='m',
     name="over-label")
ny = yb + 40 + 14
text(s, ML, ny, CW, 500 - ny, "3つとも行うには、謝礼や窓口の費用を合わせて270万円減らすか、枠を広げる必要がある。",
     size=12, color=MID, name="budget-note")
footer(s, FOOT, 3, TOTAL)

# 4. 選び方と日程
s = new_slide(prs)
top = slide_title(s, "枠に収まる組み合わせを選び、4月に始める", ACC,
                  "上位2つの困りごと(住まい・手続き)に応えるのは、施策1と3の組み合わせ")
rows = [
    ["組み合わせ", "年の費用", "枠の残り", "応える困りごと"],
    [{'t': "施策1+施策3", 'color': ACC}, {'t': "820万円", 'bold': True}, "180万円",
     {'t': "住まい探し(48%)・日本語の手続き(44%)・病院(22%)", 'bold': True, 'color': ACC}],
    ["施策1+施策2", "750万円", "250万円", "住まい探し(48%)・孤立(27%)"],
    ["施策2+施策3", "970万円", "30万円", "日本語の手続き(44%)・孤立(27%)・病院(22%)"],
    ["3つとも", "1,270万円", {'t': "270万円 超える", 'color': WARN, 'bold': True}, "すべて(費用を減らす工夫が要る)"],
]
tbl, th = table(s, ML, top, CW, [190, 130, 160, 400], rows, size=13, header_fill=ACC, name="table-combo",
                cell_inset=(10, 6, 10, 6), col_align=['l', 'r', 'r', 'l'], max_h=240)
sy = top + th + 20
text(s, ML, sy, CW, 24, "日程", size=13, bold=True, name="h-sched")
steps = [("2月", "施策と予算を決める"), ("4月", "施策を始める"), ("10月", "中間の調べ")]
sw_ = (CW - 8) / 3
for i, (m, what) in enumerate(steps):
    shp = MSO_SHAPE.PENTAGON if i == 0 else MSO_SHAPE.CHEVRON
    fill = ACC if i == 0 else ACC_PALE
    col = WHITE if i == 0 else DARK
    text(s, ML + i * (sw_ + 4), sy + 28, sw_, 60, [
        {'runs': [(m + "  ", {'bold': True, 'size': 16}), (what, {'size': 13, 'bold': True})]},
    ], color=col, fill=fill, shape=shp, inset=(12, 4, 10, 4), anchor='m', name="step-%d" % i)
ny = sy + 28 + 60 + 14
text(s, ML, ny, CW, 500 - ny,
     "10月の中間の調べでは、困りごとの割合と、退学・休学の人数(去年14人)の変化を見る。",
     size=12, color=MID, name="sched-note")
footer(s, FOOT, 4, TOTAL)

save(prs, sys.argv[1])
