import sys

from kit_t01_t05 import *

prs = new_deck()
TOTAL = 3

cover(prs, "市立図書館 平日夜の開館延長の案",
      "駅前館の夕方の需要に応える3つの案と、試行の進め方",
      "教育委員会 ご審議資料", "本日お決めいただきたいこと:4月の試行の決定(案の選択)")

# --- 1. 需要 ---
s = blank(prs)
y0 = header(s, "夕方の需要", "駅前館では夕方に利用が集中し、閉館前に入れない人が週140人いる", 1, TOTAL)
text(s, MX, y0 + 4, 520, 22,
     {"text": "駅前館 平日の時間帯ごとの利用(1時間あたりの人)", "size": 14, "bold": True},
     label="chart-cap")
bar_chart(s, MX, y0 + 30, 520, 300, ["9時台", "12時台", "17時台"],
          [("利用", (18, 41, 63))], point_colors={2: WARN}, font_size=13,
          max_label_chars=5)
text(s, MX, y0 + 336, 520, 36,
     {"text": "17時台は9時台の約3.5倍。閉館の18時まで利用が伸び続けている。",
      "size": 13, "color": SUB}, label="chart-note")

RX, RW = 600, W - MX - 600
card(s, RX, y0 + 4, RW, 128,
     [{"text": "閉館の前の30分に入れなかった人", "size": 14, "bold": True, "color": WARN},
      {"runs": [("週 平均 ", False, WARN), ("140", True, WARN), (" 人", False, WARN)],
       "size": 32, "space_before": 4},
      {"text": "年に直すと約7千人分の利用を逃している", "size": 12, "color": INK,
       "space_before": 2}],
     fill=WARN_LIGHT, label="stat")
text(s, RX, y0 + 148, RW, 22,
     {"text": "4館の年間の利用者数(千人)", "size": 14, "bold": True}, label="tbl-cap")
table(s, RX, y0 + 174, [RW - 150, 150],
      [["館", "利用者数"], ["中央館", "412"], ["駅前館", "268"], ["北館", "95"], ["南館", "121"]],
      size=13, max_h=BODY_BOTTOM - (y0 + 174), highlight_rows=(2,),
      aligns=[PP_ALIGN.LEFT, PP_ALIGN.RIGHT])
footer(s, "今の開館時間:平日 9〜18時、土日 9〜17時(4館とも)")

# --- 2. 3案の比較 ---
s = blank(prs)
y0 = header(s, "3つの案の比較", "案Aは1人あたりの費用が案Bの約半分。夜の利用の多い駅前館に絞るのが効率的", 2, TOTAL)
rows = [
    ["", "案A 駅前館を21時まで", "案B 全館を20時まで", "案C 受け取りの棚(24時間)"],
    ["内容", "駅前館だけ平日21時まで延長", "4館とも平日20時まで延長", "駅前館に予約の本の受け取りの棚を置く"],
    ["費用", "年 1,800万円", "年 5,200万円", "初年 900万円、以降 年 150万円"],
    ["見込み", "利用 年 +38千人", "利用 年 +61千人", "受け取り 年 +45千件"],
    ["1人(1件)あたりの費用", "約 470円", "約 850円", "初年 約 200円、以降 約 33円"],
    ["人手(夜の司書)", "2名増", "7名増", "—"],
]
cw = [176, 236, 236, 216]
table(s, MX, y0 + 6, cw, rows, size=13, max_h=250, highlight_rows=(), inset=8)
card(s, MX, y0 + 276, W - 2 * MX, 88,
     [{"text": "事務局の推す案:案A", "size": 16, "bold": True, "color": ACCENT},
      {"text": "需要がはっきりしている駅前館に絞り、費用と人手を抑える。案Bは利用の少ない北館・南館にも費用がかかり、"
               "北館は夜の人通りが少ない。案Cは延長ではなく受け取りだけを補う案で、案Aとの併用も考えられる。",
       "size": 13, "space_before": 4}],
     fill=ACCENT_LIGHT, label="recommend")
footer(s, "1人あたりの費用 = 年の費用 ÷ 見込みの増加。案Cは受け取り1件あたり。")

# --- 3. 進め方と懸念 ---
s = blank(prs)
y0 = header(s, "進め方と懸念への対応", "4月に試行を決めれば、9月から試行し翌3月に効果を評価できる", 3, TOTAL)
steps = [("4月", "試行の決定", "本日のご審議"), ("6月", "職員の募集", "会計年度任用職員"),
         ("9月", "試行の開始", "平日の夜の開館"), ("翌3月", "評価", "利用と費用を検証")]
sx, sw, gap = MX, 200, 16
for i, (m, t, d) in enumerate(steps):
    x = sx + i * (sw + gap)
    fill = ACCENT if i == 0 else ACCENT_LIGHT
    col = WHITE if i == 0 else INK
    card(s, x, y0 + 6, sw, 92,
         [{"text": m, "size": 20, "bold": True, "color": col},
          {"text": t, "size": 15, "bold": True, "color": col, "space_before": 2},
          {"text": d, "size": 12, "color": col, "space_before": 2}],
         fill=fill, label=f"step{i}")
    if i < len(steps) - 1:
        rect(s, x + sw + 2, y0 + 44, 12, 16, fill=GRAY, shape=MSO_SHAPE.CHEVRON)

text(s, MX, y0 + 116, 400, 22,
     {"text": "懸念と対応", "size": 15, "bold": True}, label="risk-cap")
rows = [
    ["懸念", "内容", "対応"],
    ["夜の安全", "北館の周りは夜の人通りが少ない", "案Aは駅前館だけなので北館は対象外"],
    ["光熱費", "夜の開館で 年 +6%", "試行の評価で利用の増加と比べる"],
    ["司書の研修", "夜の時間の新しい職員に研修が要る", "6月の募集から9月の開始までに行う"],
]
table(s, MX, y0 + 142, [150, 330, W - 2 * MX - 480], rows, size=13,
      max_h=BODY_BOTTOM - (y0 + 142), inset=8)
footer(s, "夜の時間の司書は会計年度任用職員で増やす(案A 2名、案B 7名)")

prs.save(sys.argv[1])
