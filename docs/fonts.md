# 字体と、置き換わったときの余裕

> 方針(人が決めた):字体はテンプレートの値の表で選び、字体ごとの寸法(文字の
> 幅)を持つ。置き換わったときの余裕は測って決める。Backlog の font-policy(#13)。

## 1. なぜ要るか

Boxloom は文字の幅を自分で測って折り返し、行の数から枠の高さを決める。PPTX には
段落のまま書き(`wrap="square"`)、折り返しは描く側(PowerPoint・LibreOffice)に
任せる(編集すれば流れ直るように)。だから開く環境で字体が置き換わると、変わるのは
次の2つ。

- **1行の幅**:置き換わった字体のほうが広いと、Boxloom が1行と測った文を描く側が
  2行に折り、枠の下へはみ出す(以前の `wrap="none"` では右へはみ出していた)。
  `fonts` の「wider lines」はこの行の数。
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
  "fixedLineSpacing": false,
  "lineHeight": 1.35
}
```

| キー | 意味 | 検査 |
|---|---|---|
| `latin` | 欧文の字体。PPTX のテーマの `a:latin` | 寸法が同梱されていること |
| `eastAsian` | 和文の字体。PPTX のテーマの `a:ea` | 寸法があり、仮名と漢字を持つ(`japanese: true`)こと |
| `fallbackFonts` | 置き換わり得る字体。測るときは文字ごとに、選んだ字体とこれらのうち**いちばん広い幅**を使う。PPTX には書かない | どれも寸法があること・重ならないこと |
| `widthMargin` | 測った幅に掛ける余裕(1.05 なら 5% 広めに見積もる)。**全角より狭い文字(欧文・数字・詰めた仮名など)にだけ掛ける**。漢字・仮名・全角の約物はどの和文の字体でも 1em の正方形なので掛けない | 1〜1.5 |
| `fixedLineSpacing` | true なら段落に行の送りを pt で書く(字の大きさ×`lineHeight`)。字体が替わっても行の高さが組んだとおりになる | 真偽値 |
| `lineHeight` | 行の送り(字の大きさに掛ける倍率)。組むときの行の高さの見積もりと、`fixedLineSpacing` のとき書く送りの両方に使う。dark・light は 1.35(box_engine.py の LINE_HEIGHT_MULT)、report は 1.2(6章) | 1〜2 |

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

道具:`go run ./cmd/boxloom fonts [-v] [-against "欧文/和文;…"] deck…`。組んだスライドの文字の
図形を1つずつ、置き換わり得る字体の寸法で測り直す。さらに `widthMargin` を
0.01 刻みで変えて組み直し、右へのはみ出しが無くなる最小の値を探す。
`go run ./cmd/boxloom fonts -list` で寸法を持つ字体の一覧が出る。

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

### 4.3 描く側が折り返したときに行が増えるか(wrap="square" にしたあと)

PPTX は段落のまま書き、折り返しは描く側に任せる(1章)。Boxloom が測った行の数より
描く側の行が**増える**と、枠(測った行の数の高さ)の下へはみ出す。`fonts` は
置き換わった字体で折り返し直した行の数を、増える箱・減る箱に分けて数える:

```
rewrap changes N (more M, of which overflow K; fewer F)
```

- `more`:行が増える箱。`overflow` はそのうち増えた行が枠に入りきらない箱
  (箇条は1つの枠の中で下の項目を押し下げるので、枠の余りと比べる)。
  `-v` で1つずつ出す(`rewrap 1 -> 2 lines OVERFLOWS "…"`)。
- `fewer`:行が減る箱。枠の下が少し空くだけで、はみ出さない。
- 描く側を真似て、置き換わった字体の幅は**余裕を掛けずに**測り、行の頭に閉じの
  約物(、。」・など)を置かない禁則(前の字と一緒に送る追い出し)も入れる。
  ぶら下げ(約物を行の外に出す)をする描き方なら、行はこれより増えない。

dark・light の既定(fallbackFonts に5つ・widthMargin 1.05)で、`examples/md/` の
14 本を5つの置き換わり方で測った結果(dark・light とも同じ):

| 置き換わり方 | 文字の枠 | 増える | うちはみ出す | 減る |
|---|---|---|---|---|
| (測った字体のまま) | 1011 | 0 | 0 | 16 |
| BIZ UDPGothic | 1011 | 0 | 0 | 23 |
| BIZ UDGothic | 1011 | 0 | 0 | 14 |
| IPAexGothic | 1011 | 0 | 0 | 14 |
| Liberation Sans + IPAexGothic | 1011 | 0 | 0 | 16 |
| DejaVu Sans + IPAexGothic | 1011 | 0 | 0 | 14 |

行が増える枠は無い。減る枠はどれも ai-society のもので、測る幅が置き換わり得る
字体のいちばん広い幅なので、描く側のほうが1行に多く入る。置き換わり得る字体を
持たない設定(4.1)では増える箱が出て、それを数えられることを
`TestFontCheckFallbacksRemoveOverflow` で確かめている。

確かめていないこと:同梱していない字体(メイリオ・游ゴシック・ヒラギノ)そのもの、
PowerPoint の実際の禁則の設定(追い出しかぶら下げか)、PowerPoint で開いた見た目。

## 5. 推奨する値(dark・light の既定)

```json
"fonts": {
  "latin": "Noto Sans CJK JP",
  "eastAsian": "Noto Sans CJK JP",
  "fallbackFonts": ["BIZ UDPGothic", "BIZ UDGothic", "IPAexGothic", "Liberation Sans", "DejaVu Sans"],
  "widthMargin": 1.05,
  "fixedLineSpacing": true,
  "lineHeight": 1.35
}
```

- `fallbackFonts`:同梱した字体の幅の最大で測る。上の5つの置き換わり方で、
  右へのはみ出しは 0 になる(4.2)。
- `widthMargin 1.05`:同梱していない字体(メイリオ・游ゴシック・ヒラギノ)への
  残りの余裕。足りるかは確かめていない。全角の文字(1em)には掛けない
  (掛けると和文だけの行が 5% 早く折れ、実際には1行に入るリード文が2行になっていた)。
  掛けるのを全角より狭い文字だけにしたあとも、`fonts` の5つの置き換わり方で
  右へのはみ出しは 0 のまま(ai-society で確かめた)。
- `fixedLineSpacing true`:行の高さが字体に依らなくなる。Noto 自身の 7% の
  はみ出しも無くなる。

**見た目の変わり方**(LibreOffice で画像にしたもの、左が従来・右が推奨):

- `verification-screenshots/fonts_compare_demo-deck-2.png`:配置は同じ。文字が
  行の送りの固定で 1〜2px 下がる。
- `verification-screenshots/fonts_compare_gantt-full.png`:注釈の箱が少し広がり、
  右上の注釈の置き場所が右へ移る(文字が箱の縁に触れなくなる)。

変わる画素は1枚あたり 1〜4.5%。**dark・light の既定はこの推奨の値にした**
(人が決めた。Backlog #36)。そのため dark・light で作る PPTX は、それ以前とは
変わっている。従来の Go のデモ `go run ./cmd/boxloom` は字体の既定の一式を使うので変わらない。

## 6. report の字体と行送り(#102)

読む資料のサンプルは、公称の字の大きさが同じ 14pt でも、report の約1.3倍の量が載っていた。
元の字体(商用の狭い和文字体)は和文1字の送りが平均約 0.86em、行送りは約1.2倍。report は
Noto Sans CJK JP を置き換わり得る字体5つのいちばん広い幅で測り(和文はほぼ 1em)、行送りは
1.35 だった。どちらも値の表の中で直した(コアは `lineHeight` のキーを足しただけ)。

```json
"fonts": {
  "latin": "BIZ UDPGothic",
  "eastAsian": "BIZ UDPGothic",
  "fallbackFonts": ["Liberation Sans", "DejaVu Sans"],
  "widthMargin": 1.05,
  "fixedLineSpacing": true,
  "lineHeight": 1.2
}
```

### 6.1 和文の送り(架空の段落で測った)

架空の和文の段落(漢字・仮名・約物の混ざった 122 字。`report_density_test.go` の
`densityParagraph`)の1字あたりの平均の送り。寸法は同梱の実測だけ。

| 字体 | 平均の送り | 仮名と約物だけ |
|---|---|---|
| Noto Sans CJK JP | 1.000em | 1.000em |
| BIZ UDGothic | 1.000em | 1.000em |
| IPAexGothic | 1.000em | 1.000em |
| **BIZ UDPGothic** | **0.947em** | **0.916em** |
| (参考)元の資料の字体 | 約 0.86em | — |

同梱の字体で仮名と約物が詰まっているのは BIZ UDPGothic だけ(漢字はどれも 1em)。
測るときの送り(置き換わり得る字体の最大と `widthMargin` を掛けた値)は、変更前 1.000em、
変更後 **0.968em**(BIZ UDPGothic の仮名・約物は全角より狭いので余裕 1.05 が掛かる)。
元の 0.86em にはまだ届かない。もっと狭い字体の実測の寸法が手に入れば寄せられる。

### 6.2 置き換わり得る字体と余裕の決め方

- `fallbackFonts` に**和文を持つ字体を入れない**。和文の字体は文字ごとの最大で測るので、
  BIZ UDGothic・IPAexGothic・Noto Sans CJK JP のどれか1つを入れるだけで仮名が 1em に戻り、
  詰めた分が消える。欧文は BIZ UDPGothic・Liberation Sans(Arial と同じ幅)・DejaVu Sans
  (メイリオの欧文の代わり)の最大で測る。
- BIZ UDPGothic は Windows 10 以降に同梱されているので、読む資料を開く主な環境では
  測った字体のまま描かれる。置き換わるのは BIZ UDPGothic の無い環境(macOS・Linux の
  多く)で、そのとき仮名が全角の字体になると1行が広がる。その量を `fonts` で測った。
- `widthMargin` は 1.05 のまま(全角より狭い文字、つまり欧文と BIZ UDPGothic の仮名・約物に掛かる)。
  1.00 と比べて、全角の字体に置き換わったときに増える行・はみ出す枠が大きく減る(下の表)。
  1.10 にすると仮名が 1em 以上になり、字体を替えた意味が無くなる。

`examples/md/` の全部(22 本、文字の枠 1256)を `-template report` で測った合計
(`go run ./cmd/boxloom fonts`。右へはみ出す行・描く側で行が増える枠・そのうち枠に収まらない枠):

| 置き換わり方 | widthMargin 1.00:はみ出す行 / 増える / 収まらない | **1.05(採用)** |
|---|---|---|
| (測った字体のまま) | 0 / 0 / 0 | 0 / 0 / 0 |
| BIZ UDPGothic | 0 / 0 / 0 | 0 / 0 / 0 |
| BIZ UDGothic | 45 / 11 / 7 | 25 / 1 / 0 |
| IPAexGothic | 39 / 7 / 5 | 22 / 1 / 0 |
| Liberation Sans + IPAexGothic | 38 / 6 / 4 | 22 / 1 / 0 |
| DejaVu Sans + IPAexGothic | 64 / 31 / 29 | 23 / 1 / 0 |

- 採用の値で、全角の和文字体に置き換わっても**枠の下へはみ出す箱は 0**。行が増える1つは
  1行→2行で、枠の余りに収まる。
- 右へはみ出す 22〜25 行は、自分の自然な幅で箱を作る短い文字(1行のラベル)で、ほとんどは
  +3pt 未満。いちばん大きいのは引用の印「“」(32pt。BIZ UDPGothic では詰まり、全角の字体では
  1em になる。+15pt)。この1字は余裕をいくつにしても(1.5 でも)消えない。
- 変更前の report(5つの字体の最大)はどの置き換わり方でも 0 / 0 / 0 だった。読む資料を
  BIZ UDPGothic の無い環境で開くと、短いラベルが少し右へ出ることがある。これは詰めた送りの代わりに
  受け入れたもの。
- 縦:BIZ UDPGothic の1行の高さ(ascent+descent)は 1.001em で、行送り 1.2 より低い。
  `fixedLineSpacing` で送りを pt で書くので、置き換わっても行の高さは変わらない。

### 6.3 要る高さ(試験 `TestReportFontDensity`)

同じ架空の段落を2回つなげたもの(244 字)を report の本文(14pt)で測った。

| 幅 | 変更前(Noto・5つの最大・1.35) | 変更後(BIZ UDPGothic・1.2) | 比 |
|---|---|---|---|
| 420pt(2列の1列) | 9行・170.1pt | 8行・134.4pt | **0.790** |
| 860pt(本文の全幅) | 4行・75.6pt | 4行・67.2pt | **0.889** |

行送りの分(1.2 / 1.35 = 0.889)はどの幅でも効き、送りの分は行の数が1つ減るときに効く。
`examples/md/` を report で check すると、詰まった分 `fill-ratio` の「少なすぎる」が
ai-society(1→3)・cover-image(1)・evidence(2)・table-continues(3)・text-patterns(1)に
新しく出る(はみ出し・題とリードの折り返しの警告は増えない)。dark・light は `lineHeight` 1.35 のままで、出力は変わらない
(`TestOutputsGolden`)。

## 7. 確かめていないこと

- PowerPoint・Keynote・Google Slides では開いていない。PowerPoint が無い字体を
  何に置き換えるかも確かめていない(4章の置き換わり方は想定)。
- メイリオ・游ゴシック・ヒラギノの寸法は持っていない。DejaVu Sans をメイリオの
  欧文の代わりにしたのは推定。
- 行の高さが ascent+descent になるのは LibreOffice・PowerPoint のおおよその
  振る舞いとしての目安。字体によっては行間(lineGap)が足される。
- 擬似太字(太字のファイルが無い字体)の送り幅。

## 8. PPTX に字体の実名を書く場所(#111)

テーマ(`ppt/theme/theme1.xml`)の `a:majorFont`・`a:minorFont` には、これまでどおり
値の表の `latin`・`eastAsian` を `a:latin`・`a:ea` で書く。ただし LibreOffice は和文で
テーマの参照(`+mn-ea`)を解決しない。report(BIZ UDPGothic)の字体を入れた環境でも
本文の和文は Noto Sans CJK で描かれ、測った字幅と描いた字幅がずれてリードが1行多く
折れた(#82 の4回目の比較)。dark・light も Noto Sans CJK **SC** に置き換わっていた
(値の表は JP)。

そこで run の外の既定に実名を書く。選んだのは候補の (1)(既定に書く)で、各 run の
`a:rPr` には書かない。

| 書く場所 | 効く文字 |
|---|---|
| `ppt/presentation.xml` の `p:defaultTextStyle`(`a:lvl1pPr`〜`a:lvl9pPr` の `a:defRPr`) | プレースホルダーでない図形・テキストボックス(本文のほぼすべて) |
| マスター・レイアウトのプレースホルダーの `a:lstStyle`(題・表紙の題と副題・フッター・番号) | プレースホルダーの文字 |
| 表のセルの `a:txBody` の `a:lstStyle` | 表の文字(LibreOffice は表のセルに `defaultTextStyle` を当てない) |
| グラフの run の `a:rPr`(以前から。`chartxml.go`) | グラフの文字 |

理由:

- (1) で解決した。run ごとに書くと全部の run が2要素ずつ長くなり、PowerPoint で
  テーマの字体を替えても文字が追従しなくなる。既定に書けば run は今のまま(字体を
  持たない)で、編集でできた新しい図形の文字も同じ字体になる。
- マスターの `p:txStyles` だけでは足りなかった。LibreOffice はプレースホルダーでない
  図形に `otherStyle` を当てず、副題の置き場も `bodyStyle` から字体を継がなかった
  (試した)。だから `p:txStyles` は書かず、`defaultTextStyle` と置き場の `lstStyle` にした。
- 表のセルだけは `defaultTextStyle` が効かないので、セルごとの `lstStyle` に書く。

確かめたこと(LibreOffice 24.2、`soffice --headless --convert-to pdf` → `pdffonts`):
`examples/md/` の全部(22の deck)を3つのテンプレートで書き出し、PDF に入った字体は
report が BIZUDPGothic-Regular・Bold だけ、dark・light が NotoSansCJKjp-Regular・Bold
だけ(直す前の report は NotoSansCJKsc と Liberation Sans が混ざっていた)。
試験 `TestTextResolvesToNamedFonts`(`pptx_fonts_test.go`)は、出力の run が継ぐ先
(run → 表のセル → 置き場か `defaultTextStyle`)をたどって実名に行き着くこと、
`+mn-`・`+mj-` の参照がテーマの外に無いことを XML で確かめる。

発表者ノート(ノートのマスター)の字体は書いていない(PDF に出ないので確かめていない)。
