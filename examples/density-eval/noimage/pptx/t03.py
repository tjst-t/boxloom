import sys

from kit_t01_t05 import *

prs = new_deck()
TOTAL = 3

cover(prs, "ヘルプデスクの問い合わせ削減 3施策の優先順位",
      "月 1,240件の問い合わせを減らし、対応の時間と満足度を改善する",
      "情報システム部 部長 ご判断資料", "お願い:3施策の優先順位のご決定")

# --- 1. 現状 ---
s = blank(prs)
y0 = header(s, "問い合わせの現状",
            "パスワードと端末の設定で半分以上。対応に月 約290時間かかっている", 1, TOTAL)
LW = 520
text(s, MX, y0 + 4, LW, 22,
     {"text": "問い合わせの内訳(月 平均 1,240件)", "size": 14, "bold": True}, label="cap1")
bar_chart(s, MX, y0 + 30, LW, 300,
          ["パスワード", "端末の設定", "権限の申請", "会議室の機器", "その他"],
          [("割合", (0.31, 0.22, 0.18, 0.14, 0.15))], horizontal=True,
          number_format='0%', font_size=13, gap=50,
          point_colors={1: GRAY, 2: GRAY, 4: GRAY}, max_label_chars=7)
text(s, MX, y0 + 334, LW, 36,
     {"text": "濃い色:施策1・施策3が直接ねらう種類。施策2(よくある質問)は種類を問わず効く",
      "size": 12, "color": SUB}, label="note1")

RX = MX + LW + 32
RW = W - MX - RX
stats = [
    ("対応の時間", "月 約290時間", "1件 平均14分 × 1,240件。担当は6人(1人 月 約48時間)"),
    ("初回で解決", "62%", "4割近くは折り返しや再対応になっている"),
    ("満足度(5点)", "3.1 → 目標 3.8", "件数を減らし、待たせない状態を目指す"),
]
cy = y0 + 4
for i, (k, v, d) in enumerate(stats):
    card(s, RX, cy, RW, 112,
         [{"text": k, "size": 13, "bold": True, "color": SUB},
          {"text": v, "size": 24, "bold": True, "color": ACCENT, "space_before": 2},
          {"text": d, "size": 12, "space_before": 4}],
         fill=GRAY_LIGHT, label=f"stat{i}")
    cy += 112 + 10
footer(s, "4〜9月の月ごとの件数は 1,180〜1,310件で横ばい(自然には減っていない)")

# --- 2. 比較 ---
s = blank(prs)
y0 = header(s, "3施策の比較",
            "効果は施策1が最大、費用あたりでは施策2が最も安い。施策3は費用と期間が重い", 2, TOTAL)
rows = [
    ["", "施策1 パスワードの自己リセット", "施策2 よくある質問のページの作り直し", "施策3 会議室の機器の統一"],
    ["費用", "300万円", "80万円", "1,400万円"],
    ["削減の見込み(月)", "330件(27%)", "120件(10%)", "150件(12%)"],
    ["浮く対応の時間(月)", "約 77時間", "約 28時間", "約 35時間"],
    ["削減1件あたりの費用", "約 760円", "約 560円", "約 7,800円"],
    ["導入までの期間", "2か月", "1か月", "6か月"],
]
cw = [192, 224, 224, 224]
th = table(s, MX, y0 + 6, cw, rows, size=13, max_h=260, inset=8,
           aligns=[PP_ALIGN.LEFT] + [PP_ALIGN.LEFT] * 3)
ny = y0 + 6 + th + 14
card(s, MX, ny, W - 2 * MX, 76,
     [{"runs": [("参考 施策4 チャットの自動応答  ", True, SUB),
                ("費用 900万円。見込みは不明で、他部署の試行では 5〜15%(月 約60〜190件)。"
                 "効果の幅が大きいので、今回の順位には入れていない。", False, INK)],
       "size": 13}],
     fill=GRAY_LIGHT, label="ref4", anchor=MSO_ANCHOR.MIDDLE)
footer(s, "割合は月 1,240件に対して。浮く時間 = 削減の件数 × 14分。1件あたりの費用 = 費用 ÷ 1年分の削減の件数")

# --- 3. 順位と進め方 ---
s = blank(prs)
y0 = header(s, "優先順位の案と進め方",
            "施策1と施策2を先に始めれば、3か月目には月 約450件(36%)減らせる見込み", 3, TOTAL)
ranks = [("1", "施策1 パスワードの自己リセット", "最も件数が多い種類を、2か月で減らせる"),
         ("2", "施策2 よくある質問のページ", "最も安く1か月でできる。施策1と並べて始める"),
         ("3", "施策3 会議室の機器の統一", "費用が他の2つの合計の3倍以上。予算の時期に改めて判断")]
LW = 470
ry = y0 + 6
for i, (n, t, d) in enumerate(ranks):
    fill = ACCENT if i < 2 else GRAY
    card(s, MX, ry, 44, 62, {"text": n, "size": 26, "bold": True, "color": WHITE,
                             "align": PP_ALIGN.CENTER},
         fill=fill, label=f"rank{i}", inset=4, anchor=MSO_ANCHOR.MIDDLE)
    card(s, MX + 44, ry, LW - 44, 62,
         [{"text": t, "size": 15, "bold": True},
          {"text": d, "size": 12, "color": SUB, "space_before": 2}],
         fill=GRAY_LIGHT, label=f"rankt{i}", anchor=MSO_ANCHOR.MIDDLE)
    ry += 62 + 8

# schedule bars (month 1..6)
gx = MX
gy = ry + 8
text(s, gx, gy, LW, 22, {"text": "導入の日程(始めてからの月)", "size": 14, "bold": True},
     label="cap-sched")
lab_w = 70
mw = (LW - lab_w) / 6
hy = gy + 26
for m in range(6):
    text(s, gx + lab_w + m * mw, hy, mw, 18,
         {"text": f"{m + 1}か月", "size": 11, "color": SUB, "align": PP_ALIGN.CENTER},
         label=f"m{m}")
bars = [("施策1", 0, 2, ACCENT), ("施策2", 0, 1, ACCENT), ("施策3", 0, 6, GRAY)]
by = hy + 22
for i, (name, a, b, col) in enumerate(bars):
    yy = by + i * 26
    text(s, gx, yy, lab_w, 22, {"text": name, "size": 12, "bold": True}, label=f"bl{i}")
    rect(s, gx + lab_w + a * mw, yy + 3, (b - a) * mw - 2, 16, fill=col)
if by + 3 * 26 > BODY_BOTTOM:
    raise Overflow("schedule too tall")

RX = MX + LW + 32
RW = W - MX - RX
text(s, RX, y0 + 6, RW, 22, {"text": "月の件数の見込み", "size": 14, "bold": True},
     label="cap-chart")
bar_chart(s, RX, y0 + 32, RW, 270, ["今", "施策1・2の後", "3つとも"],
          [("件数", (1240, 790, 640))], font_size=13, gap=70,
          point_colors={0: GRAY}, max_label_chars=7)
text(s, RX, y0 + 308, RW, BODY_BOTTOM - (y0 + 308),
     [{"text": "見込みを単純に足した値。パスワードの質問がよくある質問のページでも減るなど、"
               "重なる分だけ実際の効果は小さくなりうる。",
       "size": 12, "color": SUB}],
     label="note3")
footer(s, "満足度の目標 3.8 に向けて、導入の後に件数・初回で解決の割合・満足度を毎月見る")

prs.save(sys.argv[1])
