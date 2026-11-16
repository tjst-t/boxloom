import sys

from kit_t01_t05 import *

prs = new_deck()
TOTAL = 3

cover(prs, "直売所 来年度の売り場と営業時間",
      "客の来る時間に合わせて、開ける時間と棚の置き方を見直す",
      "直売所の運営の会(生産者 22戸)", "決めること:営業時間(土日の朝・平日の夕方)と加工品の棚の場所")

# --- 1. 客の流れ ---
s = blank(prs)
y0 = header(s, "客の来かた",
            "客の半分以上は土日に来て、朝のうちに葉物が売り切れる。平日の夕方は客が少ない", 1, TOTAL)
LW = 540
text(s, MX, y0 + 4, LW, 22, {"text": "曜日ごとの客数(1日 平均、人)", "size": 14, "bold": True},
     label="cap1")
bar_chart(s, MX, y0 + 30, LW, 300, ["月", "火", "水", "木", "金", "土", "日"],
          [("客数", (85, 72, 78, 80, 110, 240, 260))], font_size=13, gap=50,
          colors=[GRAY], point_colors={5: ACCENT, 6: ACCENT}, max_label_chars=2)
text(s, MX, y0 + 334, LW, 36,
     {"text": "1週間 925人のうち 土日が 500人(54%)。土日は平日の約3倍。", "size": 13, "color": SUB},
     label="note1")

RX = MX + LW + 32
RW = W - MX - RX
facts = [
    ("土日の朝", "11時までに葉物の 6割が売り切れ", "開店の9時から2時間で品が足りなくなる", WARN, WARN_LIGHT),
    ("平日の午後", "15時以降の客は全体の 12%", "品出しを続けても売れ残り、生産者が持ち帰っている", ACCENT, ACCENT_LIGHT),
]
fy = y0 + 4
for i, (k, v, d, col, bg) in enumerate(facts):
    card(s, RX, fy, RW, 150,
         [{"text": k, "size": 13, "bold": True, "color": col},
          {"text": v, "size": 20, "bold": True, "color": col, "space_before": 2},
          {"text": d, "size": 13, "space_before": 6}],
         fill=bg, label=f"fact{i}")
    fy += 150 + 14
footer(s, "今の営業時間:9〜17時")

# --- 2. 3つの案 ---
s = blank(prs)
y0 = header(s, "3つの案",
            "案2・案3は今の数字から効果が見込める。案1は客には良いが、朝の搬入の負担が増える", 2, TOTAL)
rows = [
    ["案", "変えること", "根拠の数字", "生産者への影響"],
    ["案1 土日は朝8時に開店",
     "土日の開店を 9時 → 8時",
     "土日は11時までに葉物の6割が売り切れ。朝の客を早くから受けられる",
     "搬入が1時間早まる。今でも 22戸中 9戸が「朝の搬入が早いのは負担」"],
    ["案2 平日は夕方4時に閉店",
     "平日の閉店を 17時 → 16時。午後の品出しを減らす",
     "平日の15時以降の客は全体の12%",
     "残りの持ち帰りの手間が減る"],
    ["案3 加工品の棚を入口へ",
     "加工品の棚を入口の近くに移す",
     "加工品は客1人あたり 1,450円で、野菜(820円)の約1.8倍",
     "棚の入れ替えの作業(1回)"],
]
th = table(s, MX, y0 + 6, [170, 210, 250, W - 2 * MX - 630], rows, size=14,
           max_h=BODY_BOTTOM - (y0 + 6) - 80, inset=8)
cy = y0 + 6 + th + 16
card(s, MX, cy, W - 2 * MX, 64,
     [{"runs": [("案1と案2を両方行うと  ", True, ACCENT),
                ("営業時間は 土日 +1時間 × 2日、平日 −1時間 × 5日で、週に 3時間 短くなる。"
                 "客の多い時間に開け、少ない時間を閉める組み替えになる。", False, INK)],
       "size": 13}],
     fill=ACCENT_LIGHT, label="combo", anchor=MSO_ANCHOR.MIDDLE)
footer(s, "客1人あたりの単価は、その品を買った客の1回の買い物の平均")

# --- 3. 来年度の案 ---
s = blank(prs)
y0 = header(s, "来年度の進め方(案)",
            "案2と案3は4月から行い、案1は朝の搬入ができる戸の数を確かめてから決める", 3, TOTAL)
LW = 500
plan = [
    ("行う", "案2 平日は16時に閉店", "平日の午後の品出しと持ち帰りを減らす", ACCENT),
    ("行う", "案3 加工品の棚を入口へ", "単価の高い加工品をまず目に入る場所に", ACCENT),
    ("確かめる", "案1 土日の朝8時の開店", "8時に搬入できる戸が何戸あるかを聞き、葉物が揃うなら試す", WARN),
]
py = y0 + 6
for i, (tag, t, d, col) in enumerate(plan):
    card(s, MX, py, 92, 80, {"text": tag, "size": 14, "bold": True, "color": WHITE,
                             "align": PP_ALIGN.CENTER},
         fill=col, label=f"tag{i}", inset=4, anchor=MSO_ANCHOR.MIDDLE)
    card(s, MX + 92, py, LW - 92, 80,
         [{"text": t, "size": 15, "bold": True},
          {"text": d, "size": 13, "color": SUB, "space_before": 2}],
         fill=GRAY_LIGHT, label=f"plan{i}", anchor=MSO_ANCHOR.MIDDLE)
    py += 80 + 10
card(s, MX, py + 4, LW, 68,
     [{"runs": [("変更の知らせ  ", True, ACCENT),
                ("ポイントカードの会員 1,860人(月1回以上来る人 41%)に、新しい営業時間を知らせる。",
                 False, INK)], "size": 13}],
     fill=ACCENT_LIGHT, label="notice", anchor=MSO_ANCHOR.MIDDLE)

RX = MX + LW + 32
RW = W - MX - RX
text(s, RX, y0 + 6, RW, 22, {"text": "品目ごとの売上(年、万円)", "size": 14, "bold": True},
     label="cap3")
bar_chart(s, RX, y0 + 32, RW, 270, ["野菜", "果物", "加工品", "米", "花"],
          [("売上", (3420, 1180, 860, 620, 410))], horizontal=True, font_size=13,
          gap=50, colors=[GRAY], point_colors={2: ACCENT}, max_label_chars=4)
text(s, RX, y0 + 308, RW, BODY_BOTTOM - (y0 + 308),
     {"text": "加工品は売上の13%(年 6,490万円のうち 860万円)。単価が高いので、"
              "手に取る客が増えれば売上を伸ばしやすい。",
      "size": 13, "color": SUB}, label="note3")
footer(s, "決めていただきたいこと:案2・案3の実施、案1を確かめる進め方")

prs.save(sys.argv[1])
