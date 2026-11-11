# 字体と、置き換わったときの余裕

> 方針(人が決めた):字体はテンプレートの値の表で選び、字体ごとの寸法(文字の
> 幅)を持つ。置き換わったときの余裕は測って決める。Backlog の font-policy(#13)。

## 1. なぜ要るか

Boxloom は文字の幅を自分で測って折り返し、行に分けてから PPTX に書く
(`wrap="none"`。描く側に折り返させない)。だから開く環境で字体が置き換わっても
**行の数は変わらない**。変わるのは次の2つ。

- **1行の幅**:置き換わった字体のほうが広いと、行が箱の右へはみ出す。
- **1行の高さ**:行の送りを書かないと、描く側は字体の ascent+descent で行を
  送る。組むときの見積もり(字の大きさ×1.35)より高い字体なら、箱の下へはみ出す。

以前は測る字体が Noto Sans CJK JP に決め打ちで、PPTX のテーマも欧文の字体
(`a:latin`)しか書いていなかった。和文の字体(`a:ea`)が無いと、PowerPoint は
和文を自分の既定の字体で描く。測った字体と描く字体が、はじめから食い違っていた。

## 2. 値の表の fonts

`boxloom/templates/<名前>.json` に書く。キーは全部必須(知らないキーも誤り)。

```json
"fonts": {
  "latin": "Noto Sans CJK JP",
  "eastAsian": "Noto Sans CJK JP",
  "fallbackFonts": [],
  "widthMargin": 1.05,
  "fixedLineSpacing": false
}
```

| キー | 意味 | 検査 |
|---|---|---|
| `latin` | 欧文の字体。PPTX のテーマの `a:latin` | 寸法が同梱されていること |
| `eastAsian` | 和文の字体。PPTX のテーマの `a:ea` | 寸法があり、仮名と漢字を持つ(`japanese: true`)こと |
| `fallbackFonts` | 置き換わり得る字体。測るときは文字ごとに、選んだ字体とこれらのうち**いちばん広い幅**を使う。PPTX には書かない | どれも寸法があること・重ならないこと |
| `widthMargin` | 測った幅に掛ける余裕(1.05 なら 5% 広めに見積もる) | 1〜1.5 |
| `fixedLineSpacing` | true なら段落に行の送りを pt で書く(字の大きさ×1.35)。字体が替わっても行の高さが組んだとおりになる | 真偽値 |

**文字の振り分け**:U+2000 より前(ASCII・ラテン文字)は欧文の字体、それ以降
(一般句読点・記号・仮名・漢字・全角形)は和文の字体で測る。PowerPoint は文の
言語が ja-JP のとき、おおよそこの線で `a:latin` と `a:ea` を使い分ける
(ギリシャ文字・キリル文字などの細かな例外は扱っていない)。

**いまの既定**:dark・light とも上の値(従来どおり)。推奨する値は 5章。

## 3. 寸法(fonts/*.json)と作り方

寸法は `boxloom/fonts/<slug>.json` で、バイナリに同梱する。字体のファイルそのもの
はリポジトリに入れない(ライセンスのため)。中身は「1em あたりの送り幅」の表と、
ascent・descent。

| 字体 | 寸法 | ライセンス | 和文 | 1行の高さ(ascent+descent) | 手に入れた場所 |
|---|---|---|---|---|---|
| Noto Sans CJK JP | 実測 | OFL-1.1 | ○ | 1.448em | この環境の `/usr/share/fonts/opentype/noto/` |
| BIZ UDPGothic | 実測 | OFL-1.1 | ○ | 1.001em | googlefonts/morisawa-biz-ud-gothic(Windows 10 以降にも同梱) |
| BIZ UDGothic | 実測 | OFL-1.1 | ○ | 1.001em | 同上(欧文が半角の等幅。MS ゴシックと同じ振る舞い) |
| IPAexGothic | 実測(太字なし) | IPA | ○ | 1.001em | moji.or.jp |
| Liberation Sans | 実測 | OFL-1.1 | × | 1.118em | この環境。**Arial と同じ幅に作られた字体** |
| DejaVu Sans | 実測 | Bitstream Vera | × | 1.165em | この環境。欧文の広い字体 |

作り方(字体のファイルがある機械で):

```
python3 tools/export_font_metrics.py --family "字体の名前" \
    --regular 標準のファイル [--bold 太字のファイル] --license ライセンス \
    --out boxloom/fonts/<slug>.json
```

- 測る文字:ASCII・ラテン1補助・ラテン拡張A・一般句読点・矢印・囲み英数字・
  幾何学模様・その他の記号・CJK の記号と句読点・ひらがな・カタカナ・全角形。
  漢字は「一」を測って `default` にする(和文の字体は漢字がみな同じ幅)。表には
  `default` と違う幅の文字だけを載せ、字体に無い文字は載せない。
- 太字のファイルが無い字体は、太字にも標準の幅を使う(PowerPoint の擬似太字は
  送り幅を変えない前提。確かめてはいない)。
- 寸法を作り直しても、従来の Noto Sans CJK JP の 242 文字の幅は以前の表と
  完全に一致した(試験 `TestBundledFontMetrics` の一部)。

**メイリオ・游ゴシック・ヒラギノ角ゴ・Arial** はこの環境に無い。自由なライセンス
ではないので寸法も同梱していない。測るときは代わりの字体を使い、**推定**と
明記する(4章)。持っている人が上の道具で寸法を作れば、そのまま選べるように
なる。ただし、商用の字体から測った寸法をリポジトリに入れてよいかは人が決める。

## 4. 測った数字

道具:`go run . fonts [-v] [-against "欧文/和文;…"] deck…`。組んだスライドの文字の
図形を1つずつ、置き換わり得る字体の寸法で測り直す。さらに `widthMargin` を
0.01 刻みで変えて組み直し、右へのはみ出しが無くなる最小の値を探す。
`go run . fonts -list` で寸法を持つ字体の一覧が出る。

対象:`examples/ir/` の demo-deck・gantt-full・speaker-notes・
check-warnings/layout-overflow(文字の図形 136、行 154)。

置き換わり方(既定の5つ):

| 置き換わり方 | 何の代わりか |
|---|---|
| BIZ UDPGothic | そのもの(Windows 10 以降に同梱) |
| BIZ UDGothic | 欧文が等幅の字体(MS ゴシックの振る舞い)の代わり |
| IPAexGothic | 欧文がプロポーショナルで仮名が全角の字体 |
| Liberation Sans + IPAexGothic | Arial の欧文(幅は Arial と同じ) |
| DejaVu Sans + IPAexGothic | **推定**:メイリオの欧文(広い字体)の代わり |

### 4.1 従来の設定(widthMargin 1.05、fallbackFonts なし)

| 置き換わり方 | 右へはみ出す行 | いちばんのはみ出し | 置き換わった字体で折り返し直すと行の数が変わる箱 | はみ出しが無くなる widthMargin |
|---|---|---|---|---|
| (測った字体のまま) | 0 | — | 0 | 1.00 |
| BIZ UDPGothic | 15 | +6.7pt(「2 / 3」) | 16 | **1.38** |
| BIZ UDGothic | 6 | +4.8pt | 8 | 1.29 |
| IPAexGothic | 6 | +2.6pt | 11 | 1.18 |
| Liberation Sans + IPAexGothic | 0 | — | 1 | 1.00 |
| DejaVu Sans + IPAexGothic | 6 | +2.1pt | 17 | 1.16 |

- 和文(仮名・漢字)はどの字体もほぼ全角(1em)で差が無い。差は**欧文と空白**:
  英数字の平均の幅は Noto 0.578em に対して BIZ UDPGothic 0.700em(+21%)、
  空白は 0.224em に対して 0.334em(+49%)。
- いちばん効くのは、自分の自然な幅で箱を作る短いラベル(ページ番号「2 / 3」・
  フッターの「Boxloom デモ」・Gantt の注釈)。折り返す本文は行の余りがあるので
  はみ出しにくい。
- **縦**:置き換わった字体はどれも 1 行が 1.35em より低い(1.00〜1.17em)ので、
  下へはみ出さない。下へはみ出すのは**測った Noto Sans CJK JP 自身**
  (1.448em。見積もりより 7% 高い)で、行の送りを固定しないと、見積もりどおりの
  高さで作った箱 52 / 136 の下へ、箱の高さの最大 7% はみ出す。

### 4.2 余裕を一律に上げた場合と、置き換わり得る字体で測る場合

| 設定 | 右へはみ出す行(5つの置き換わり方の合計) | 行の数(154) | check の警告 |
|---|---|---|---|
| 従来(1.05) | 33 | 154 | 従来どおり |
| widthMargin 1.38(一律) | 0 | 155 | gantt-full に layout-overflow と subsystem-min が**新しく出る** |
| fallbackFonts に5つ + widthMargin 1.05 | 0 | 154 | 従来どおり |

一律に上げると和文の行まで 38% 広く見積もるので、図が窮屈になる。置き換わり
得る字体で文字ごとに測れば、広がるのは欧文と空白だけで、例の行の数も警告も
変わらなかった。

## 5. 推奨する値(dark・light の既定)

```json
"fonts": {
  "latin": "Noto Sans CJK JP",
  "eastAsian": "Noto Sans CJK JP",
  "fallbackFonts": ["BIZ UDPGothic", "BIZ UDGothic", "IPAexGothic", "Liberation Sans", "DejaVu Sans"],
  "widthMargin": 1.05,
  "fixedLineSpacing": true
}
```

- `fallbackFonts`:同梱した字体の幅の最大で測る。上の5つの置き換わり方で、
  右へのはみ出しは 0 になる(4.2)。
- `widthMargin 1.05`:同梱していない字体(メイリオ・游ゴシック・ヒラギノ)への
  残りの余裕。足りるかは確かめていない。
- `fixedLineSpacing true`:行の高さが字体に依らなくなる。Noto 自身の 7% の
  はみ出しも無くなる。

**見た目の変わり方**(LibreOffice で画像にしたもの、左が従来・右が推奨):

- `verification-screenshots/fonts_compare_demo-deck-2.png`:配置は同じ。文字が
  行の送りの固定で 1〜2px 下がる。
- `verification-screenshots/fonts_compare_gantt-full.png`:注釈の箱が少し広がり、
  右上の注釈の置き場所が右へ移る(文字が箱の縁に触れなくなる)。

変わる画素は1枚あたり 1〜4.5%。**dark・light の既定はこの推奨の値にした**
(人が決めた。Backlog #36)。そのため dark・light で作る PPTX は、それ以前とは
変わっている。従来の Go のデモ `go run .` は字体の既定の一式を使うので変わらない。

## 6. 確かめていないこと

- PowerPoint・Keynote・Google Slides では開いていない。PowerPoint が無い字体を
  何に置き換えるかも確かめていない(4章の置き換わり方は想定)。
- メイリオ・游ゴシック・ヒラギノの寸法は持っていない。DejaVu Sans をメイリオの
  欧文の代わりにしたのは推定。
- 行の高さが ascent+descent になるのは LibreOffice・PowerPoint のおおよその
  振る舞いとしての目安。字体によっては行間(lineGap)が足される。
- 擬似太字(太字のファイルが無い字体)の送り幅。
