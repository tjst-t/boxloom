import sys

from kit_t01_t05 import *

prs = new_deck()
TOTAL = 3

cover(prs, "町の路線バス 3路線の再編の案",
      "利用に合わせて便と運び方を見直し、町の補助を抑えながら足を守る",
      "議会 委員会 ご説明資料", "お願い:再編の案へのご賛同(説明会 5月、試行 10月から)")

# --- 1. 現状 ---
s = blank(prs)
y0 = header(s, "3路線の現状",
            "町の補助は年 6,750万円。利用の少ない西線・山手線が赤字の7割を占める", 1, TOTAL)
LW = 500
text(s, MX, y0 + 4, LW, 22, {"text": "路線ごとの利用と赤字", "size": 14, "bold": True},
     label="cap1")
rows = [["路線", "1日の便", "1便あたり", "赤字(年)"],
        ["東線", "24便", "6.1人", "2,100万円"],
        ["西線", "18便", "3.2人", "2,900万円"],
        ["山手線", "10便", "1.4人", "1,750万円"],
        ["合計", "", "", "6,750万円"]]
table(s, MX, y0 + 30, [110, 110, 130, 150], rows, size=14, max_h=200,
      highlight_rows=(4,), aligns=[PP_ALIGN.LEFT] + [PP_ALIGN.RIGHT] * 3)
text(s, MX, y0 + 238, LW, 60,
     [{"text": "山手線は1便に平均1.4人。赤字は利用1人あたりで見ると3路線で最も重い。",
       "size": 13, "color": SUB},
      {"text": "西線は赤字が最も大きく、便の数に対して乗る人が少ない。", "size": 13,
       "color": SUB, "space_before": 4}],
     label="note1")

RX = MX + LW + 32
RW = W - MX - RX
text(s, RX, y0 + 4, RW, 22,
     {"text": "住民の調べ:バスを使わない理由(複数回答)", "size": 14, "bold": True},
     label="cap2")
bar_chart(s, RX, y0 + 30, RW, 210, ["車がある", "便が少ない", "停留所が遠い"],
          [("割合", (0.52, 0.41, 0.27))], horizontal=True, number_format='0%',
          font_size=13, point_colors={0: GRAY}, max_label_chars=6)
text(s, RX, y0 + 244, RW, 40,
     {"text": "回答 812人。週1回以上バスを使う人は 23%。", "size": 13, "color": SUB},
     label="note2")

# bottom strip: why keep a public ride
by = y0 + 300
card(s, MX, by, W - 2 * MX, BODY_BOTTOM - by,
     [{"runs": [("車に頼れない人が増えている  ", True, ACCENT),
                ("65歳以上は 38%(10年前 29%)。運転免許の返納は 年 140件。", False, INK)],
       "size": 14}],
     fill=ACCENT_LIGHT, label="strip", anchor=MSO_ANCHOR.MIDDLE)
footer(s, "1便あたり:1便に乗る平均の人数")

# --- 2. 案 ---
s = blank(prs)
y0 = header(s, "再編の案",
            "東線は残し、西線は便を減らして病院へ、山手線は予約の乗合の車に切り替える", 2, TOTAL)
cols = [
    ("東線", "残す", ACCENT,
     ["1日 24便のまま", "1便あたり 6.1人と3路線で最も使われている"]),
    ("西線", "便を減らし、病院に寄る", ACCENT,
     ["1日 18便 → 12便", "経路を病院に寄るように変える", "通院に使える路線にする"]),
    ("山手線", "やめて予約の乗合の車に", WARN,
     ["定時の路線バスをやめ、予約に応じて走る乗合の車(デマンド)に",
      "見込み 1日 30回の予約", "待ち 平均 25分"]),
]
cw, g = 272, 24
for i, (name, act, col, pts) in enumerate(cols):
    x = MX + i * (cw + g)
    card(s, x, y0 + 6, cw, 62,
         [{"text": name, "size": 18, "bold": True, "color": WHITE},
          {"text": act, "size": 14, "bold": True, "color": WHITE}],
         fill=col, label=f"head{i}")
    card(s, x, y0 + 68, cw, 150,
         [{"text": p, "size": 13, "bullet": "・", "space_before": 6} for p in pts],
         fill=GRAY_LIGHT, label=f"body{i}")

cy = y0 + 236
text(s, MX, cy, 500, 22, {"text": "山手線の費用:路線バスとデマンドの比較(年、万円)",
                          "size": 14, "bold": True}, label="cap3")
bar_chart(s, MX, cy + 26, 500, BODY_BOTTOM - cy - 26, ["今の赤字", "デマンド"],
          [("費用", (1750, 1100))], horizontal=True, font_size=13,
          point_colors={0: GRAY}, max_label_chars=6)
card(s, MX + 532, cy, W - 2 * MX - 532, BODY_BOTTOM - cy,
     [{"text": "山手線だけで 年 約650万円 少なく", "size": 16, "bold": True, "color": ACCENT},
      {"text": "1日の利用は約14人(10便 × 1.4人)。予約制にすれば家の近くから乗れ、"
               "「停留所が遠い」という声にも応えられる。",
       "size": 13, "space_before": 6}],
     fill=ACCENT_LIGHT, label="saving")
footer(s, "西線の減便による費用の変化は、試行の結果で示す")

# --- 3. 日程と心配 ---
s = blank(prs)
y0 = header(s, "日程と、寄せられている心配",
            "半年の試行で確かめてから本格に移る。予約の方法と通学は試行の前に決める", 3, TOTAL)
steps = [("5月", "住民への説明会"), ("10月〜翌3月", "試行"), ("翌4月", "本格の運行")]
sw, g = 272, 24
for i, (m, t) in enumerate(steps):
    x = MX + i * (sw + g)
    fill = ACCENT if i == 1 else ACCENT_LIGHT
    col = WHITE if i == 1 else INK
    card(s, x, y0 + 6, sw, 70,
         [{"text": m, "size": 20, "bold": True, "color": col},
          {"text": t, "size": 14, "color": col, "space_before": 2}],
         fill=fill, label=f"step{i}")
    if i < len(steps) - 1:
        rect(s, x + sw + 5, y0 + 33, 14, 16, fill=GRAY, shape=MSO_SHAPE.CHEVRON)

text(s, MX, y0 + 96, 400, 22, {"text": "心配と対応の方向", "size": 15, "bold": True},
     label="cap4")
rows = [["心配", "対応の方向(案)"],
        ["デマンドの予約が電話だけでよいか",
         "高齢の方には電話が使いやすい。試行では電話を基本にし、ほかの予約の方法を足すかを利用の様子で決める"],
        ["山手線で通学に使う生徒 8人",
         "登校・下校の時間の予約を毎日のものとして受ける形を試行の前に詰め、通学の足を途切れさせない"],
        ["西線の便が減ること",
         "便を減らす代わりに病院に寄り、通院の行き帰りに合う時刻にする"]]
table(s, MX, y0 + 122, [300, W - 2 * MX - 300], rows, size=13,
      max_h=BODY_BOTTOM - (y0 + 122), inset=8)
footer(s, "試行の間の利用と費用は議会に報告する")

prs.save(sys.argv[1])
