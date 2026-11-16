import sys

from kit_t01_t05 import *

prs = new_deck()
TOTAL = 3

cover(prs, "休日の部活動の地域のクラブへの移行",
      "来年度からの移り方と、お寄せいただいた不安へのお答え",
      "保護者の皆さまへの説明会", "9月から休日の部活動はすべて地域のクラブで行います")

# --- 1. 何が変わるか ---
s = blank(prs)
y0 = header(s, "何が変わるか",
            "休日の部活動(13部)は、9月から地域のクラブが運営し、クラブの指導者が教えます", 1, TOTAL)
LW = 600
rows = [
    ["", "今", "9月から(休日)"],
    ["運営", "学校", "地域のクラブ(スポーツ協会・文化団体)"],
    ["教える人", "学校の先生", "クラブに登録した指導者。希望する先生も報酬ありで登録できる"],
    ["練習の場所", "学校", "8部は学校のまま。5部は学校の外(学校から 1.2〜4.5km)"],
    ["費用", "—", "会費 月 1,500〜3,000円(部による)と保険 年 800円"],
]
table(s, MX, y0 + 6, [110, 110, LW - 220], rows, size=13,
      max_h=BODY_BOTTOM - (y0 + 6), inset=8)

RX = MX + LW + 28
RW = W - MX - RX
stats = [
    ("対象", "休日の部活動 13部", "運動部 9・文化部 4。平日の部活動は今までどおり"),
    ("生徒", "486人・加入 78%", "約380人が部活動に入っている"),
    ("先生の休日の部活", "月16時間 → 0時間", "1人あたりの平均。希望する先生は指導者として続けられる"),
]
cy = y0 + 6
for i, (k, v, d) in enumerate(stats):
    card(s, RX, cy, RW, 110,
         [{"text": k, "size": 13, "bold": True, "color": SUB},
          {"text": v, "size": 20, "bold": True, "color": ACCENT, "space_before": 2},
          {"text": d, "size": 12, "space_before": 4}],
         fill=GRAY_LIGHT, label=f"stat{i}")
    cy += 110 + 10
footer(s, "翌4月からは、平日の部活動の一部も地域に移すかを検討します")

# --- 2. 不安へのお答え ---
s = blank(prs)
y0 = header(s, "お寄せいただいた不安へのお答え",
            "多かった不安は費用・送り迎え・指導の質。それぞれ次のように進めます", 2, TOTAL)
items = [
    ("費用", 0.52,
     ["会費は 月 1,500〜3,000円(部によって違う)。保険は 年 800円",
      "就学援助を受けているご家庭は会費が免除になります"]),
    ("送り迎え", 0.47,
     ["学校の外で練習するのは 5部。距離は学校から 1.2〜4.5km",
      "8部は今までどおり学校で練習します"]),
    ("指導の質", 0.35,
     ["指導者は 41人が登録済み。要るのは 58人で、あと 17人を9月までに集めます",
      "希望する先生も指導者として登録できます(報酬あり)"]),
]
iy = y0 + 6
ih = 104
for i, (k, v, pts) in enumerate(items):
    card(s, MX, iy, 200, ih,
         [{"text": k, "size": 18, "bold": True, "color": WHITE},
          {"text": f"不安と答えた人 {round(v * 100)}%", "size": 13, "color": WHITE,
           "space_before": 4}],
         fill=ACCENT, label=f"k{i}", anchor=MSO_ANCHOR.MIDDLE, inset=14)
    card(s, MX + 200, iy, W - 2 * MX - 200, ih,
         [{"text": p, "size": 14, "bullet": "・", "space_before": 6} for p in pts],
         fill=GRAY_LIGHT, label=f"a{i}", anchor=MSO_ANCHOR.MIDDLE, inset=14)
    iy += ih + 10
text(s, MX, iy + 2, W - 2 * MX, BODY_BOTTOM - iy - 2,
     {"text": "保護者の調べ(回答 302人):移行に 賛成 44%、どちらとも 38%、反対 18%。"
              "不安は複数回答。",
      "size": 12, "color": SUB}, label="survey")
footer(s, "会費の額と練習の場所は部によって違います")

# --- 3. 日程と窓口 ---
s = blank(prs)
y0 = header(s, "これからの日程とご相談の窓口",
            "6月に運動部3つで試し、9月から休日の活動をすべて移します", 3, TOTAL)
steps = [("4月", "説明", "本日の説明会"),
         ("6月", "試行", "運動部 3つで先に始める"),
         ("9月", "全部の休日の活動", "13部すべてが地域のクラブに"),
         ("翌4月", "平日の一部も検討", "試行と9月からの様子を見て")]
sw, g = 200, 16
for i, (m, t, d) in enumerate(steps):
    x = MX + i * (sw + g)
    hi = i == 2
    card(s, x, y0 + 6, sw, 110,
         [{"text": m, "size": 22, "bold": True, "color": WHITE if hi else ACCENT},
          {"text": t, "size": 15, "bold": True, "color": WHITE if hi else INK,
           "space_before": 2},
          {"text": d, "size": 12, "color": WHITE if hi else INK, "space_before": 4}],
         fill=ACCENT if hi else ACCENT_LIGHT, label=f"step{i}")
    if i < len(steps) - 1:
        rect(s, x + sw + 2, y0 + 53, 12, 16, fill=GRAY, shape=MSO_SHAPE.CHEVRON)

wy = y0 + 140
text(s, MX, wy, 400, 24, {"text": "ご相談の窓口", "size": 16, "bold": True}, label="cap-w")
desks = [("学校の事務室", "部活動の移行について、学校でご相談を受けます"),
         ("クラブの事務局", "地域のクラブの側でご相談を受けます")]
dw = (W - 2 * MX - 20) / 2
for i, (n, d) in enumerate(desks):
    card(s, MX + i * (dw + 20), wy + 30, dw, 96,
         [{"text": n, "size": 18, "bold": True, "color": ACCENT},
          {"text": "平日 9〜17時", "size": 14, "bold": True, "space_before": 4},
          {"text": d, "size": 13, "color": SUB, "space_before": 4}],
         fill=GRAY_LIGHT, line=GRAY, label=f"desk{i}")
card(s, MX, wy + 142, W - 2 * MX, BODY_BOTTOM - (wy + 142),
     {"text": "費用・送り迎え・指導についてのご不安は、どちらの窓口にもお寄せください。",
      "size": 14, "color": INK},
     fill=ACCENT_LIGHT, label="close", anchor=MSO_ANCHOR.MIDDLE)
footer(s, "対象:休日の部活動(運動部 9・文化部 4)")

prs.save(sys.argv[1])
