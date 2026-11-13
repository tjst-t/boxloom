# テンプレートの値の表(データのファイル)

テンプレートの色・字の大きさ・余白・印の文字、Gantt の寸法を、Go のコードから
JSON のファイルに移した。方針(CLAUDE.md)の「値の表・スタイル規則・骨組みは
データ、部品とサブシステムはコード」のうち、**値の表**(と印の文字という小さな
スタイル規則)がデータになった。骨組み(表紙・本文のスライドの並び)と部品
(箇条・比較など)はまだコード(`template.go`)。

| ファイル | 中身 |
|---|---|
| `boxloom/templates/<名前>.json` | テンプレート1つ(`dark.json`・`light.json`・`report.json`)。名前はファイル名 |
| `boxloom/subsystems/gantt.json` | Gantt の既定の寸法。テンプレートが差し替えなければこれ |
| `boxloom/subsystems/shapes.json` | shapes(生の図形への逃げ道)の既定の寸法 |
| `boxloom/subsystems/process.json`・`orgchart.json`・`matrix.json`・`funnel.json`・`cycle.json`・`pillars.json` | 箱と線の図・循環の図・屋根と柱の図(`docs/subsystem.md` の11節)の既定の寸法 |
| `boxloom/subsystems/chart.json` | 数値のグラフ(`docs/subsystem.md` の12節)の既定の寸法 |
| `boxloom/values.go` | 読み込みと検査 |

どちらも `go:embed` でバイナリに同梱する。テンプレートの一覧はこのフォルダの
ファイルから作るので、`templates/` にファイルを1つ足せば `-template <名前>` で選べる。

## report(読む資料のテンプレート、Backlog #78・#83)

dark / light は**発表**のためのテンプレート(本文 16pt。遠くから読める大きさ)。
report は**読む資料**(手元や画面で読む報告書)のためのテンプレート。

**見立ての誤りと直し(#83)**:#78 で report を作ったときは「読む資料の本文は 8〜10pt」と
見立て、light の色のまま字と余白だけを詰めた(本文 12pt・注 8pt)。読む資料のサンプル
(リポジトリの外。中身は書かない)の PDF を測ると、この見立ては誤りだった:本文は 14pt で、
密な表だけが 10〜11pt、題の場所には結論の文(24pt・基調色・2行まで)、その下に話題の副題
(16pt)を置く。人の決定で、新しいテンプレートは作らず、report を測った値に直して1つにした
(名前は report のまま)。

見た目の違いは report が自分の値の表と組み方で持つ。コアの標準の部品・骨組みには条件分岐を
足していない(dark・light の出力は変わらない)。

**組み方(`report.go`)**:骨組みの部品 `contentHeader`・`section`・`toc`・`cover`・`closing` を差し替える(`docs/layers.md` の8.4)。

- 表紙(#92・#109・#137):全面に写真(`meta.coverImage`)を敷き、縦の中ほど・左の縁から本文と同じ
  外周(`pagePadding`)だけ離して幅 0.53・高さ 0.45 の基調色の箱を置いて(上下に写真が見える)、その下寄せに白い字(`bg`)で題(`coverTitle`)・
  副題(`coverSubtitle`)、続けて作者・日付・機密区分・免責を書く(写真の上に字を置かない)。
  終わりのスライドは同じ写真に同じ位置の箱で、`meta.closing` の文があれば箱の下寄せに白い太字
  (`coverTitle`)で書く。箱の高さは 0.45 を下限に中身が要るだけ伸ばし(#104)、上下の縁からは
  ページの高さの 0.1 以上離す(`reportCoverMinGap`)。写真が無ければ地の色の上に同じ形。

- 区切り(#86・#137):level 1・2 は frame(全面の基調色に白い枠を置き、54pt の白い太字の題)。
  直下に level 3 の区切り(項目)がある節の扉は、題の下に白い縦の罫線と、その節の項目の題の
  一覧(20pt・白・並び順)を置く。一覧は組み立てで数える(IR にも記法にも足さない。次の
  level 1・2 の区切りの手前までの level 3)。level 3 は題だけの簡素な扉(地の色のページの縦の
  中ほど・左は `pagePadding` に、短い基調色の罫線と 28pt の基調色の太字の題。親の節の題や
  兄弟の一覧は置かない)。章の一覧(`sectionTracker`)は出さない。dark・light の区切りは
  level によらず従来どおり。
- 隅の印(#93):`marker` のある本文のスライドは、`contentHeader` の差し替えがページの右上の
  隅に三角の地(幅・高さ 48pt、右上が直角。濃い基調色の役割 `support`)と、その中の丸
  (直径 24pt・地の色 `bg`)に印(`small`・太字・`support`)を、その左にパンくず(直前の区切りの題。
  `small`=10pt・`text_muted`)を描く。どちらもページの上の余白の中で、題・本文の配置は変わらない。
- 目次(#86):全面の基調色に白い字。題 28pt の下に白い罫線、章 20pt の太字、節 16pt を
  `tocIndent` で字下げ、頁番号は右寄せ。フッターは置かない。
- 白は役割名 `bg`(report の地の色)、測った大きさは段の刻み(`type(54)` など)で書く。


- リード文(`:::lead`)を上の見出しに置く:`leadText`(24pt)・基調色(`accent`)・標準の太さ
  (#100。元の資料も太字でない。太字だと字幅が広がり、1行の文が2行に折れていた)。
  2行までに収める(`leadMaxLines` 2。越えると `lead-wrap` の警告。越えた分だけ本文の領域が減る)。
- 題はその下に `slideTitle`(16pt)・字の色(`text_primary`)で置く。題はこれまでどおり
  レイアウトの title のプレースホルダー(位置だけが下になる)。
- リードの無いスライドは、題を上の見出しの位置と大きさ(24pt・基調色・標準の太さ)で置く。
  題の折り返しの警告(`title-wrap`)は今のまま(その大きさで1行)。
- 列の見出し(`columns` のまとまりの題、`pairs` の側の見出し)を `columnTitle`(16pt)・
  基調色(`accent`)にする。部品 `columns` を override し、`pairs` は自分の組み方を登録する
  (同梱の組み方 `registerBundledComponents` は、テンプレートがすでに登録した型には足さない)。
  `columns` の組み方は標準のもの(`columnsHeaded`)で、見出しの色の役割名だけを渡す。
  dark・light は標準のまま字の色(`text_primary`)。
- `pairs` は標準の組み方の見た目を差し替えたもの(`pairsStyled` に `reportPairsStyle`。#87):
  対ごとに A・B・C… の四角い印(一辺 `space(12)`=24pt・基調色の地・白(`bg`)の字)を subject の
  頭に、2つの側の間に丸い山形の印(同じ大きさ)を置き、対の間に罫線(`grid`・高さ
  `tableRuleWidth`)を引く。対の行と罫線の間は `space(2)`=4pt ずつ(対の行の間は約 8.75pt。
  値の表の `pairsRowGap`(normal 4・dense 2。#103・#136)。印の一辺も値の表の `pairMarkSize`(normal 24・dense 20)。側の見出し(`title`)は帯(面)を置かず、基調色(`accent`)の
  太字の本文1行で書く(#103)。印は飾りの葉(`ornament`)で、横の余り(`spare-width`)には数えない。
- 表(`table`、#88)を `tableStyled` に report の見た目(`reportTableLook`)を渡して組む。
  見出しの行は面を塗らず、字を基調色(`accent`)にし、下に基調色の罫線(`tableHeaderRuleWidth`
  1.5pt)を引く。本体の行も面を塗らず、行の間は灰色(`grid`)の細い罫線(`tableRuleWidth`
  0.75pt)。行の群の名前のセル(結合の続きも)は濃い基調色(`support` 187955)の面に白い字(`bg`)の
  縦書きの細い帯(#100。`a:tcPr` の `vert="eaVert"`。表のセルの縦書きは `a:bodyPr` ではなく
  `a:tcPr` に書く)。帯の列の幅は縦の1行(`tableText` × 1.35 + 左右の `tableCellPaddingX`)で、
  表の余りの幅は配らない。見出しの行・合計の行のその列の文は単語を割らない最小の幅で折り返す。
  名前が結合した行より長ければ、群の最後の行を高くする(`blocks.go` の `vertLength`・`rowHeights`)。
  本物の `a:tbl` のまま、罫線と面はセルの書式(`a:tcPr` の `a:lnB` と塗り)で書く。
- 図(`image`・`gallery`、#100)の説明の文を、図の上に基調色(`accent`)で置く(`reportImageLook`。
  標準は図の下に `text_muted`)。高さの測り方は変わらない(説明の文の高さ + `captionGap`)。
  gallery は段ごとに説明の文の上の端をそろえ、画像はその段のいちばん高い説明の文の下から置く。
- 結論(`takeaway`、#88)は地の面と左の帯を置かず、左に丸い矢印(基調色の円に白い右向きの
  矢印。直径 `calloutMark` 24pt)、右に基調色の太字の文(`takeawayText`)。上下にカードと同じ
  余白(`cardPadding`)を取り、本文から離す。
- 引用(`quote`、#88)は薄い面(`card` F1F2F1)の角丸の吹き出しの中に引用の文(`quoteText`)。
  吹き出しには右へ向かう尾(同じ面の三角)を付け、その右に人の印(基調色の円の頭と、上の角の
  丸い四角の肩。`calloutMark` 24pt)、印の下に出どころ(`quoteSource`・`text_muted`。印があるので
  頭の `marks.quoteSource` は付けない)。吹き出しと人の印の列の幅は 3:1。
- 丸い矢印・人の印・尾は shapes ではなく、report の組み方の中の図形の葉(`calloutMark`)で描く。
  色は描くときに役割名で引く。文に添える印なので、横の余り(`spare-width`)には数えない
  (`spare.go` の `ornament`)。
- 試験は `TestReportContentHeader`・`TestReportOverrideIsOnlyReport`・`TestReportColumnHeadings`・
  `TestReportPairsMarks`・`TestReportTableLook`・`TestReportCallouts`、#103 の `TestReportPairsTableSpacing`、#100 の
  `TestReportLeadNotBold`・`TestReportTableGroupVertical`・`TestReportImageCaptionAbove`。

**色(theme)**:測った色を、コアの役割名に当てた(役割名は増やしていない)。

| 役割名 | 値 | 元の資料での役 |
|---|---|---|
| `bg` | FFFFFF | 地 |
| `text_primary` | 575757 | 字 |
| `text_muted` | 7F7F7F | 薄い字(頁番号も) |
| `grid` | 9A9A9A | 罫線 |
| `card`・`card_alt` | F1F2F1 | 薄い面 |
| `accent` | 29BA74 | 基調 |
| `support` | 187955 | 濃い基調 |
| `danger` | 03522C | さらに濃い基調 |
| `conditional` | 30C1D7 | 副の色 |
| `none` | 9A9A9A | 罫線と同じ灰 |
| `risk` | D4F1E3 | 基調の薄い色(Gantt の risk 帯) |
| `on_conditional` | FFFFFF | 状態の色の上の字 |

状態の色(support・conditional・none・danger)は基調の系統から選んだ。隅の印の地(295E7E)は、
隅の印が語彙に入るとき(親のストーリーの波2)に使う。いまは当てる役割名が無い。

**段(scale)**:`spacingUnit` 2、`typeSizes` 8・9・10・11・12・14・16・18・20・24・28・32・54。

**大きさ(values)**:960×540pt。`fillRatioMin`(0.65)・印(marks)は前のまま。

**字体と行送り(fonts、#102)**:元の資料は同じ 14pt でも載る量が約1.3倍だった(行送りが約1.2倍、
和文1字の送りが平均約 0.86em)。report は `latin`・`eastAsian` を BIZ UDPGothic(#102 の時点で同梱の字体で唯一、
仮名と約物がプロポーショナル)、`fallbackFonts` を欧文の字体だけ(Liberation Sans・DejaVu Sans)、
`widthMargin` 1.05、`lineHeight` 1.2 にした。架空の和文の段落(14pt)の要る高さは、変更前の
**0.790 倍**(幅 420pt、9行→8行)・**0.889 倍**(幅 860pt、4行のまま)。測り方と、字体が置き換わった
ときの余裕は `docs/fonts.md` の6章(試験 `TestReportFontDensity`)。もっと狭い自由な字体の候補(IPAPGothic など。既定は変えていない)は同じ文書の9章。

| 値 | light | report |
|---|---|---|
| `pagePadding` / `pagePaddingY` / `pagePaddingBottom` | 40 / 24 / 24 | 50 / 42 / 20 |
| `coverPadding` | 64 | 50 |
| `sectionGap` / `blockGap` | 20 / 20 | 12 / 18 |
| `itemGap` / `headerGap` | 8 / 8 | 2 / 8 |
| `leadLineHeight` / `listLineHeight` | 1.35 / 1.35 | 1.08 / 1.08 |
| `footerText` | 10 | 8 |
| `leadText`(上の見出し) | 18 | 24 |
| `slideTitle`(話題) | 24 | 16 |
| `body` | 16 | 14 |
| `subPoint`(入れ子の箇条) | 14 | 12 |
| `tableText` | 14 | 11 |
| `small`(注・フッター・頁番号) | 10 | 10 |
| `columnTitle`(列の見出し。report は基調色) | 16 | 16 |
| `optionLabel` / `optionItem` | 16 / 12 | 16 / 12 |
| `figureDetail` / `captionText` | 12 / 12 | 12 / 11 |
| `takeawayText` / `quoteText` | 16 / 16 | 16 / 16 |
| `statLabel` / `quoteSource` / `sectionTrackerText` | 12 / 12 / 14 | 12 / 12 / 12 |
| `coverTitle` / `coverSubtitle` / `coverAuthor` | 36 / 16 / 12 | 32 / 18 / 14 |
| `sectionTitle` / `statementText` | 36 / 36 | 32 / 28 |
| `statValue` / `statUnit` / `quoteMark` | 36 / 16 / 36 | 32 / 14 / 32 |
| `cardPadding` / `columnGap` | 20 / 32 | 12 / 24 |
| `tableCellPaddingX` / `tableCellPaddingY` / `tableRuleWidth` | 8 / 4 / 1 | 4 / 2 / 0.75 |
| `tableHeaderRuleWidth` / `calloutMark`(report の組み方だけが使う) | 1 / 24 | 1.5 / 24 |

頁番号は標準のフッターのまま右下に `footerText`(8pt、#146)・`text_muted`(7F7F7F)で置く。列の見出しは
16pt・基調色(上の「組み方」)。`columns` は report が Go で持つ組み方(`report.go`。#84)で、
列の見出しの下に罫線(役割 `grid`・0.75pt。見出しとの間・罫線と中身の間は `itemGap`)、列の間(`columnGap` の
隙間)の真ん中に縦の罫線(`grid`・0.75pt)を引き、`flow` のときは縦の罫線の縦の真ん中に
直径 24pt(隙間より広ければ隙間の幅)の基調色の丸と、地の色(`bg`)の山形(>)を置く。
罫線の太さと丸の直径はサンプルを測った値で、`report.go` の定数に置く。区切りのスライドの
見た目(全面の基調色・54pt の白い字)は、親のストーリーの次のタスクで足す(段の 54 はそのため)。

**Gantt(`subsystems.gantt`)**:#78 のまま(字は `labelText`・`annotationText` 10、
`smallText` 8。余白と行を詰めた寸法)。

| キー | 既定 | report |
|---|---|---|
| `padding` | 16 | 12 |
| `groupGap` | 8 | 4 |
| `annotationLaneGap` | 8 | 4 |
| `naturalRowHeight` | 26 | 22 |
| `maxRowHeight` | 40 | 44 |
| `axisStyle` | `ticks` | `monthGrid` |
| `barShape` | `rect` | `chevron` |
| `fillHeight` | `false` | `true` |
| `gridYearFormat` | `YYYY` | `YYYY年` |
| `gridMonthFormat` | `M` | `M月` |

report の Gantt は月の格子(年と月の2段の見出し)・山形の帯で描き、本文の領域の高さを
使い切る(#89。`docs/subsystem.md` の11.1)。`maxRowHeight` は高さを使い切るときの行の
上限なので、32 から 44 に上げた。

**元の資料の詰め方に寄せる(#146)**:元の資料では入るのに Boxloom では入らない頁の高さの
不足を分解すると(リポジトリの外の測り。4頁で約 697pt)、約6割は値で詰められるもの
(題と箇条の間・項目の間・下の外周とフッター・dense の字・リードと一覧の行送り)だった。
report だけを直した(dark・light の出力は変わらない。golden の report の出力はこの値の分だけ変わる)。

| 箇所 | 前 | 今 | 元の資料 |
|---|---|---|---|
| まとまりの題(対の側の題・列の題・topic の題)とその下の箇条の間 | 対の側は `blockGap` 18(dense 12)、列・topic は `itemGap` 6 | `itemGap` 2(対の側は report の組み方の `titleGap`。列の題の下の罫線の上と下も 2) | 0(行送りだけ) |
| 箇条の項目の間・カードの中の行の間(`itemGap`) | 6(dense 4) | 2(dense 2) | 0 |
| 下の外周(`pagePaddingBottom`) | 42(`pagePaddingY` と同じ) | 20(上は 42 のまま) | 頁番号の線まで本文 |
| フッターの字(`footerText`) | 10(`small`) | 8 | — |
| dense の本文 / 入れ子 | 12 / 11 | 11 / 10 | 11(22 頁) |
| リード文・箇条の行送り(`leadLineHeight`・`listLineHeight`) | 1.2(`fonts.lineHeight`) | 1.08 | 1.08 |

本文の段落の行送りは `fonts.lineHeight` の 1.2 のまま(元の資料の本文も 1.2)。本文の箱の下の端は
486pt → 498.4pt(本文とフッターの間は `sectionGap` 12 のまま。ページ番号・フッターと本文は
重ねない)。下の外周を上と別にするため、コアの箱に下だけの余白(`Spec.PaddingBottom`、トークン
`paddingBottom`)を足した(容量・はみ出しの測りも上と下の和で数える)。行送りは文字の葉の
`LineHeight`(トークン `lineHeight`。PPTX の段落の `a:lnSpc` も同じ値)。
`-fit` の広げの段の `itemGap` も 8・10 → 4・6 に下げた(基の 2 から1段ずつ)。
試験は `report_tighten_test.go`(`TestReportTightenValues`・`TestReportTightenLayout`・
`TestReportTightenBodyAboveFooter`・`TestReportTightenPPTXLineSpacing`)。

**段落の塊の間(`blockGap`、#100)**:元の資料の段落の塊の間を、本文(14pt)の空き行1つ
ぶん(行の高さ 14 × 1.35 ≈ 18.9pt)と見た。段(`spacingUnit` 2)の刻みで近い 18 にした(前は 16)。
ブロックの間の空き全般に効くので、examples の deck が report ではみ出さないことを確かめた。
この値は本文の行の高さからの見積もりで、元の PDF をこのタスクで測り直してはいない
(サンプルはリポジトリの外)。

**対の並びと表の余白(#103)**:#82 の2回目の比較で、字を小さくしても収まらない上乗せが
対の並び(対ごとの空き・側の見出しの灰の帯)と表のセルの内側の余白に残った。元の資料はこの
タスクの作業場所に無く測り直せなかったので、タスクの目安の値に合わせた(元の資料で測ったら
ここを直す)。

| 箇所 | 前 | 今 | 目安 |
|---|---|---|---|
| 対の行の間(罫線の上と下) | `blockGap` 18 ずつ(計 約 36.75pt) | `pairsRowGap` 4 ずつ(計 約 8.75pt。値の表へ移した、#136) | 8〜12pt |
| 側の見出し | 薄い面(`card`)の帯・左右 8pt・上下 4pt の余白・字の色 | 帯なし・基調色の太字の1行 | 帯でなく基調色の字だけ |
| 表のセルの余白(左右 / 上下) | 6 / 4 | 4 / 2 | 左右 4〜6・上下 2〜4 |

試験 `TestReportPairsTableSpacing`:架空の対(見出しつきの側の3行)の本文の要る高さは
292.5 → 212.5pt、架空の表(見出し+6行)は 159.9 → 131.9pt。

**測った値**:本文の領域は、リードの無いスライドで約 386pt(14pt の本文で約 20 行。行の高さ
約 18.9pt)、1行のリード文と1行の題で約 357pt(約 18 行)。1行は全角で約 58 字、題は 16pt で
約 50 字、リードの無いときの題(24pt)は約 34 字。試験 `TestReportPairs`:架空の対(2行ほどの
「分かったこと」と3項目の打ち手)は 3 つまで1枚に収まる(#78 の 12pt では5つ収まった)。

examples の deck は全部 `-template report` で組め、はみ出し(`layout-overflow`)も題の折り返し
(`title-wrap`)も出ない(`gantt-full` が収まるよう `sectionGap` を 12 にした)。

## 外の値の表を使う(Backlog #31)

自分たちの色と字体で使いたいときは、同梱のものと同じ形の JSON を作り、
`-template` にファイルのパスを渡す(`build`・`check`・`schema`・`fonts` のどれでも)。

```
go run ./cmd/boxloom build -template ./my-brand.json deck.bx.md
go run ./cmd/boxloom schema -template ./my-brand.json   # そのテンプレートが名乗るものだけ
```

- `-template` の値が `.json` で終わるか、パスの区切り(`/`・`\`)を含めばファイル、
  そうでなければ同梱の名前として引く。
- 検査は同梱のものと**同じ**(次の節)。`name` はファイル名(拡張子を除いた最後の
  部分)と同じでなければならない。IR の `template` より `-template` が優先する。
- 誤りは5章の形の1件(`code` は `template-values`、`file` は値の表のファイル、
  `path` は値の表の中の場所、`line` はその行の目安)。`check` は値の表に誤りが
  あれば文書を検査せず、その1件だけを返す(終了コード 1)。

  ```
  error: ./my-brand.json:36 /values/blockGap: template-values: values.blockGap: 13 is not on the spacing scale (multiples of 4); nearest: 12 or 16
  ```
- 囲み・部品・骨組みはまだコードなので、外の値の表で変えられるのは値の表の中身
  (色・余白・字の大きさ・字体・印・サブシステムの寸法)だけ。囲みの一覧は同梱の
  テンプレートと同じ。
- ライブラリでは `ParseTemplate(file, data)` で読み、`BuildOptions.Values`・
  `CheckOptions.Values`・`SchemaTemplate` に渡す(`docs/library.md`)。
- 例は `examples/templates/forest.json`(架空の緑の配色)。

## 読み込みの検査

黙って既定値で埋めない。次はすべて誤りで、テンプレートを引いたとき
(`build`・`check`)に `templates/light.json: missing key "values.body"` の形で出る。

- 知らないキー(どの階層でも)。
- 欠けたキー。`subsystems` だけは書かなくてよい(書くなら中の値は全部)。
- `null`、型の違い(数のところに文字など)、最上位の値のあとに続くもの。
- `name` がファイル名と違う。
- テーマの役割名が決まった 13 個(下)と1つでも違う(欠け・余り)。色は
  大文字の16進6桁(`0033CC`)。`#` や小文字は断る。
- 大きさ(数)が 0 以下。
- 段階(`scale`)の誤り、または段階から外れた値(次の節)。
- 意味の違う役割名が同じ色(`accent` `conditional` `support` `danger` `none`
  `text_muted` の6つは互いに違う色。次の節)。
- 印の文字が空(数と単位の間の `statUnit` だけは空でよい)。続けた表の印
  (`marks.tableContinued`)に `{n}` か `{m}` が無い。分けた続きの枚の題(`marks.continuedTitle`)に
  `{title}` が無い。
- 日付の書き方(`marks.date`・`marks.yearMonth`)の印の誤り(次の節の marks)。
- `variants` に空の名前・同じ名前が2回。
- `dense` の値が段に無い・`values` の同じ名前より大きい(次の節の dense)。
- `fit` の段の値が段階に無い・0 以下・順が逆(詰めが前の段より大きい、広げが前の段より
  小さい、非常用の段が `shrink` の最後の段・前の段より大きい)・知らないキー・何も替えない段(次の節の fit)。

同梱のファイルが壊れていれば試験(`values_test.go`)が落ちる。

## templates/<名前>.json の形

```json
{
  "name": "light",
  "theme":  { "bg": "FFFFFF", "card": "F7F9FD", "...": "..." },
  "scale":  { "spacingUnit": 4, "typeSizes": [8, 10, 12, 14, 16, 18, 24, 36] },
  "values": { "pageWidth": 960, "pageHeight": 540, "...": 0 },
  "marks":  { "slideSource": "出典:", "date": "{YYYY}年{M}月{D}日", "yearMonth": "{YYYY}年{M}月",
              "point": "・", "subPoint": "– ", "pro": "＋ ", "con": "－ ", "...": "..." },
  "fonts":  { "latin": "Noto Sans CJK JP", "eastAsian": "Noto Sans CJK JP",
              "fallbackFonts": ["BIZ UDPGothic", "…"], "widthMargin": 1.05, "fixedLineSpacing": true,
              "lineHeight": 1.35 },
  "dense":  { "body": 14, "subPoint": 12, "tableText": 12, "small": 10, "blockGap": 12, "itemGap": 4,
              "tableCellPaddingX": 4, "tableCellPaddingY": 4, "pairsRowGap": 8, "pairMarkSize": 20, "matrixRowTitle": 12,
              "figureDetail": 12, "calloutMark": 24, "...": 0 },
  "fit":    { "shrink": [{ "body": 12, "...": 0 }, { "...": 0 }], "emergency": [{ "body": 8, "...": 0 }],
              "grow": [{ "body": 18, "...": 0 }, { "...": 0 }] },
  "variants": [],
  "subsystems": { "gantt": { "...": "subsystems/gantt.json と同じ形" } }
}
```

### scale(段階。見た目のトークンの刻み)

余白と字の大きさを、自由な数ではなく決まった段階からしか選べないようにする
(スイス・タイポグラフィのグリッドの考え方。テンプレートを足す人の下振れを防ぐ)。

| キー | 意味 |
|---|---|
| `spacingUnit` | 余白・間隔の刻み(pt)。下の「余白」の値はこの倍数だけ |
| `typeSizes` | 字の大きさの段(pt)。小さい順に重ならずに並べる。下の「字」の値はこの中の値だけ |

段階に載せる値(Go の json タグに `scale:"space"` / `scale:"type"` が付いたもの):

| 種類 | values | subsystems/gantt.json | subsystems/shapes.json |
|---|---|---|---|
| 余白 | `pagePadding` `pagePaddingY` `pagePaddingBottom` `coverPadding` `sectionGap` `blockGap` `itemGap` `headerGap` `cardPadding` `statementInset` `columnGap` `tableCellPaddingX` `tableCellPaddingY` `pairsRowGap` `pairMarkSize` `captionGap` `sectionTrackerPadding` | `padding` `legendGap` `groupGap` `annotationPaddingX` `annotationPaddingY` `annotationLaneGap`、#70 で移した `legendSwatchGap` `milestonePaddingX` `milestoneAxisGap` `groupBoxGap` `subgroupInsetRight` `rowLabelGap` `annotationFixedPaddingX` `annotationFixedPaddingY` `annotationGap` `annotationShift`、箱の大きさ `milestoneMinWidth` `tickLabelWidth` `tickSubLabelOffset` `tickSubLabelWidth` `tickSubLabelHeight` | `padding` `textInsetX` `textInsetY` |
| 字 | `coverTitle` `coverSubtitle` `coverAuthor` `slideTitle` `leadText` `body` `subPoint` `optionLabel` `optionItem` `small` `footerText` `figureDetail` `matrixRowTitle` `captionText` `sectionTitle` `sectionTrackerText` `statementText` `takeawayText` `statValue` `statUnit` `statLabel` `quoteText` `quoteMark` `quoteSource` `columnTitle` `tableText` | `annotationText` `labelText` `smallText` | (字は values の段の名前で引く) |

ページの大きさ・角の丸み・線の太さ・行の高さ・欄の幅・倍率、図の中の細部の寸法は
段階に載せない(図の寸法で、刻みに合わせても読みやすさは変わらないため)。0 以下は
誤り。行送りの倍率(`leadLineHeight`・`listLineHeight`、#146)も段に載せず、1 から
`fonts.lineHeight` までに限る(字体の `lineHeight` の 1〜2 と同じく倍率で検査する)。Gantt のキーの分け方は下の「subsystems/gantt.json の形」の表。

- 検査:段階そのもの(刻みが 0 以下、段が空・小さい順でない)と、段から外れた値は誤り。
  誤りの文は近い段の値を添える(例 `values.subPoint: 12.5 is not on the type scale
  [8 10 12 14 16 24 36]; nearest: 12 or 14`)。
- サブシステムの寸法は、そのテンプレートの段階で見る。テンプレートが差し替えない
  ときは既定のファイル(`subsystems/gantt.json` など)の値を見る。段を狭めた
  テンプレートは、既定の Gantt の字が段から外れれば読み込みで断られる。
- dark / light は `spacingUnit` 4、`typeSizes` [8, 10, 12, 14, 16, 18, 24, 36] の8段。
  段の役割はおおむね 36 表紙の題・区切りの題・主張の1文・統計強調の数・引用の印、
  24 スライドの題、18 リード文、16 本文と小見出し(表紙の副題・比較の選択肢の名前・結論・
  引用の言葉・数の単位・図の箱の題)、14 入れ子の箇条・表の文字、12 従の文(利点と欠点・
  表紙の作者・数の説明・引用の出どころ)、10 注(注記・フッター・Gantt の目盛りと行の名前)、8 図の中の細かい字。
  文字のパターン(#17〜#19)のために段は足していない(36 を数と主張に使う)。

**段階に寄せたとき(Backlog #29)に変えた値**(dark / light 共通):

| 値 | 前 | 後 |
|---|---|---|
| `itemGap` | 6 | 8 |
| `cardPadding` | 14 | 16 |
| `coverAuthor` | 11.5 | 12 |
| `subPoint` | 12.5 | 12 |
| `optionLabel` | 15 | 16 |
| `optionItem` | 13 | 12 |
| `small` | 10.5 | 10 |

**余白を見直したときに変えた値**(dark / light / forest 共通)。要素の間が詰まっている
わりにページの下が空いて見え、ページの外周も狭かったため。余白は「近いものは近く、
まとまりの間は広く」の順(項目 12 < ブロック 20 ≦ 題と本文 20 < 左右の外周 40)に揃えた。
あわせて題は短い見出し(1行)にし、言いたいことの文はリード文(`lead`)に分けた:

| 値 | 前 | 後 |
|---|---|---|
| `pagePadding`(左右) | 24 | 40 |
| `pagePaddingY`(上下。新しく足した) | (24) | 24 |
| `sectionGap` | 16 | 20 |
| `blockGap` | 16 | 20 |
| `itemGap` | 8 | 12 |
| `headerGap`(題とリード文の間。新しく足した) | — | 8 |
| `leadText`(リード文。新しく足した) | — | 16 |
| `cardPadding` | 16 | 20 |
| `coverPadding` | 48 | 64 |

本文の領域は、リード文も出典も無いスライドで 414pt → 406pt(左右が広がり上下はそのまま、
gap が増えた分)。1行のリード文で約 30pt、2行で約 52pt 狭くなる。

**字の大きさを見直したときに変えた値**(dark / light / forest 共通)。箇条の字が
リード文より小さく、項目の間が字に比べて広く見えたため。本文を一段上げ、項目の間を
詰め(行の高さ 21.6pt に対して 8pt)、リード文は本文より一段上に置いた。段に 18 を足した:

| 値 | 前 | 後 |
|---|---|---|
| `typeSizes` | 8〜16・24・36 | 18 を足す |
| `leadText` | 16 | 18 |
| `body`(本文・図の箱の題) | 14 | 16 |
| `subPoint`(入れ子の箇条・図の箱の補足) | 12 | 14 |
| `tableText` | 12 | 14 |
| `itemGap` | 12 | 8 |

1行のリード文で本文の領域は約 32pt、2行で約 57pt 狭くなる。本文の1行は 18.9pt → 21.6pt。

リード文・主張・結論・引用(文として読む段落)は、行の数を増やさない範囲で、文の
切れ目(。!?)、次に読点(、,)の後ろで折る(`TextRun.Phrase`、`boxloom/text.go`)。
| gantt `annotationPaddingY` | 6 | 4 |
| gantt `annotationText` | 8.5 | 8 |
| gantt `labelText` | 9 | 10 |
| shapes `textInsetX` | 6 | 8 |

前後の画像は `verification-screenshots/scales_compare_*.png`(LibreOffice、左が前・右が後)。
配置の骨組みは同じで、箇条の間が少し開き、Gantt の目盛りと行の名前が少し大きくなった。
はみ出しの例(`examples/ir/check-warnings/layout-overflow.json`)は、はみ出しが 47.8pt から
75.1pt に増えた(箇条の間が開いたため。もともとはみ出させる例)。ほかの例の警告の
種類と数は変わらない。従来のデモ(`go run ./cmd/boxloom`)も Gantt の既定の寸法を使うので、Gantt の
字と注釈の箱が変わる。

**図の補足の字を上げたとき(Backlog #45)に変えた値**(dark / light 共通):

| 値 | 前 | 後 |
|---|---|---|
| 図の補足(新しい `figureDetail`。以前は `small`) | 10 | 12 |
| 表・画像の説明(新しい `captionText`。以前は `small`) | 10 | 12 |
| gantt `annotationText` | 8 | 10 |
| marks `statUnit` | 半角の空白 | 空 |
| light の theme `conditional` | `0033CC`(`accent` と同じ) | `9A6700` |

結論や数が投影で読めない細字に書かれないように、図の補足と説明を従の文(12)に、
Gantt の注釈を注(10)に上げた。Gantt の区間の文・節目・グループ名(`smallText` 8)は
上げていない:10 にすると `examples/ir/gantt-full.json` が本文に収まらなくなる
(行が高くなり 46.9pt はみ出す)。密な Gantt の中の字は 8 のまま、値の表で差し替えられる。
例の警告の数は変わらない(20 の組み合わせで `check` の件数が同じ)。

### theme(役割名 → 色)

`bg` `card` `card_alt` `text_primary` `text_muted` `grid` `support`
`conditional` `none` `danger` `risk` `accent` `on_conditional` の 13 個、ちょうど。

**意味の違う役割名は違う色(Backlog #45)。** `accent`(見てほしい1か所)・
`conditional`・`support`・`danger`・`none`(状態)・`text_muted`(補足の文字)の6つは、
どの2つも同じ16進にできない。読み込みで `theme.conditional: 0033CC is the same color as
theme.accent` の形の誤り。同じ色だと、図の「条件つき」の丸や「要調整」の枠が強調に
見え、意味が読めなくなる(以前の light は `conditional` と `accent` がどちらも
`0033CC` だった)。PPTX のテーマの accent1〜6 にもこの6つを写す(次の節)。
light の `conditional` は `9A6700`(黄土。白の文字 `on_conditional` を載せても読める濃さ)。

#### PPTX のテーマへの写し方(Backlog #43)

theme の色は、スライドの図形に直接入るほか、PPTX のテーマ(`ppt/theme/theme1.xml` の
`a:clrScheme`)とスライドマスターにも写す。PowerPoint で人が新しく足した文字・図形・
表が、テンプレートの色で描かれるようにするため。テーマ・配色・字体の組の名前
(`a:theme`・`a:clrScheme`・`a:fontScheme`・`a:fmtScheme` の `name`)はテンプレートの
名前(外の値の表ならファイル名から取った name)。

| テーマの枠 | 地が明るいテーマ(light・forest) | 地が暗いテーマ(dark) |
|---|---|---|
| `dk1` | `text_primary` | `bg` |
| `lt1` | `bg` | `text_primary` |
| `dk2` | `text_muted` | `card_alt` |
| `lt2` | `card_alt` | `text_muted` |
| `accent1` | `accent` | 同じ |
| `accent2` | `conditional` | 同じ |
| `accent3` | `support` | 同じ |
| `accent4` | `danger` | 同じ |
| `accent5` | `none` | 同じ |
| `accent6` | `text_muted` | 同じ |
| `hlink` | `accent` | 同じ |
| `folHlink` | `text_muted` | 同じ |

- **明るいか暗いか**は `bg` と `text_primary` の相対輝度(WCAG の式)で決める。`bg` の
  ほうが暗ければ暗いテーマ。テンプレートに印は持たせない(色から決まるので食い違わない)。
- **dk/lt は「暗い色・明るい色」の枠**なので、暗いテーマでは地の色を `dk1` に置き、
  スライドマスターの `p:clrMap` を `bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2"` に向ける
  (PowerPoint の暗いテーマと同じ作り)。これで「背景 1」「テキスト 1」が地と文字の色に
  なり、新しく足した文字が暗い地の上でも読める。明るいテーマは既定の向き
  (`bg1="lt1" tx1="dk1"`)のまま。
- **ノートのマスター**の `clrMap` はいつも明るい向き。ノートは白い紙に刷る前提のため。
- スライドマスターの背景は `bg` の色。スライドの見た目は変わらない(各スライドが
  自分の図形で描くため。LibreOffice の絵で、従来の PPTX と画素まで同じことを確かめた)。
- `risk`・`grid`・`card`・`on_conditional` はテーマの枠に写さない(12枠に入らない。
  スライドの図形には今までどおり入る)。
- 試験は `boxloom/theme_colors_test.go`(light・dark・examples/templates/forest.json)。

### values(pt。断りの無いものは pt)

| キー | 使うところ |
|---|---|
| `pageWidth` `pageHeight` | ページの大きさ |
| `pagePadding` | 本文・主張のスライドの左右の外周 |
| `pagePaddingY` | 本文・主張のスライドの上の外周(題の上)。`pagePaddingBottom` と同じなら下も |
| `pagePaddingBottom` | 本文・主張のスライドの下の外周(フッターの下、#146)。dark / light は `pagePaddingY` と同じ 24、report は 20 |
| `coverPadding` | 表紙の外周 |
| `sectionGap` | 題(とリード文)・本文・出典・フッターの間 |
| `blockGap` | 本文のブロックの間、比較のカードの間 |
| `itemGap` | 箇条の項目の間、カードの中の行の間 |
| `headerGap` | 題とリード文の間 |
| `coverTitle` `coverSubtitle` `coverAuthor` | 表紙の字の大きさ |
| `slideTitle` | 本文のスライドの題(短い見出し。折り返すと `title-wrap` の警告) |
| `leadText` | リード文(`lead`。題の直下、色は `text_primary`、太字にしない) |
| `leadMaxLines` | リード文の行の数の上限(整数)。超えると `lead-wrap` の警告。0 は上限なし(dark / light は 0、report は 2) |
| `body` | 段落・箇条 |
| `leadLineHeight` `listLineHeight` | リード文と箇条(入れ子も)の行送り(字の大きさに掛ける倍率、#146)。1 から `fonts.lineHeight` まで(越えれば読み込みの誤り)。段落の行送りは `fonts.lineHeight` のまま。dark / light は `fonts.lineHeight` と同じ 1.35、report は 1.08 |
| `footerText` | フッター(作者・機密区分・頁番号)の字(#146。dark / light は `small` と同じ 10、report は 8) |
| `subPoint` | 入れ子の箇条 |
| `optionLabel` `optionItem` | 比較の選択肢の名前、利点・欠点 |
| `small` | 注記・フッター・図の凡例 |
| `figureDetail` | 図の補足:process・orgchart の箱の2段目、process の段階の下の文と図の説明(dark / light は 12) |
| `captionText` | 表・画像・画像の並びの説明の文(dark / light は 12。色は `text_muted`) |
| `sectionTitle` | 区切り(section)の章の題 |
| `sectionTracker` | 真偽。`true` なら、区切りが2つ以上ある文書の区切りのスライドに、章の一覧と現在地を題の下に置く(いまの章は帯がアクセント・題が `text_primary` の太字、ほかは帯が `grid`・題が `text_muted`)。`false` なら置かない |
| `sectionTrackerText` | 章の一覧の字の大きさ |
| `sectionTrackerPadding` | 章の一覧の題のまわりの余白(余白の段の上。組み方は値の表の名前だけを書くので、`itemGap` の半分のような計算した値の代わりに置く) |
| `tocIndent` | 目次(`meta.toc`)の節(level 2)の字下げ(余白の段の上) |
| `statementText` | 主張(statement)の1文 |
| `takeawayText` | 結論(takeaway)の1文 |
| `statValue` `statUnit` `statLabel` | 統計強調(stat)の数・単位・説明 |
| `quoteText` `quoteMark` `quoteSource` | 引用(quote)の言葉・頭の印・出どころ |
| `pointIndentEm` | 入れ子の箇条の字下げ。親の字の大きさに掛ける倍率。番号つきの箇条の番号の分の幅にも使う |
| `orderedStyle` | 番号つきの箇条(`ordered`)の1段目の番号の形。`arabicPeriod`(1.)・`romanLcPeriod`(i.)・`circleNumDbPlain`(①)のどれか(PPTX の `a:buAutoNum` の type)。入れ子の段は `arabicPeriod` に決めてある。dark / light / report は `arabicPeriod`。ほかの名前は読み込みで誤り |
| `cardPadding` `cardRadius` | 比較のカード |
| `subsystemRadius` | サブシステムを載せるカードの角 |
| `coverRuleWidth` `coverRuleHeight` | 表紙の題の下の線。区切りの題・主張の1文の上の線にも使う |
| `statementInset` | 主張のスライドの文を、本文のスライドの外周(`pagePadding`)よりさらに内へ下げる幅 |
| `accentBarWidth` | 結論(takeaway)の左のアクセントの帯の幅 |
| `fillRatioMin` | 本文の埋まりがこの割合(0〜1)未満なら `fill-ratio` の警告 |
| `columnTitle` | 並列のまとまりの題(dark / light は 16) |
| `columnGap` | 並列のまとまりの間(32) |
| `columnMainWeight` | 主のまとまり(`role: main`)の幅の比。ほかのまとまりは 1(2。倍率なので段階に載せない) |
| `pairsSideMin` | 対の並び(pairs)の側の幅の下限(2つの側の幅の合計に対する割合、0.5 未満。0.2 → 各側 20〜80%)。この範囲で中身から割る(`docs/layers.md` の8.1) |
| `pairsRowGap` | 対の並びの対の行の間(罫線を引く report では行と罫線の間。dark / light は `blockGap` と同じ 20、report は 4。#136) |
| `pairMarkSize` | 対の印(A・B・C の四角・側の間の印)の一辺。印を描く report の対の行の高さの下限(24。dark / light は使わない) |
| `matrixRowTitle` | matrix の行の題(行の見出しの列の名前)の字。列の見出し・列の群の名前は `body`(normal は `body` と同じ。dense で別に詰める、#136) |
| `personPhoto` | 人物の紹介(people)の写真の一辺(64。正方形。縦横比の違う写真は端を切る) |
| `personPhotoShape` | 写真の形(`circle` は円で切り抜く・`square` は四角。dark・report は circle、light は square) |
| `personMinWidth` | 人を横に1列に並べるときの1人の幅の下限(240)。本文の幅を人数で割ってこれに足りなければ2人ずつの段に積む(`docs/ir.md` の10.3) |
| `tableText` | 表のセルの字(12。見出しの行は太字) |
| `tableCellPaddingX` `tableCellPaddingY` | セルの左右・上下の余白(8・4) |
| `tableRuleWidth` | 表の行の間の線の太さ(1。線の色は役割名 `grid`、見出しの行の塗りは `card_alt`、本体は `card`) |
| `tableHeaderRuleWidth` | 見出しの行の下に別の線を引く組み方の、その線の太さ(report は 1.5 で基調色。dark / light は使わない。値は `tableRuleWidth` と同じにしてある) |
| `calloutMark` | 結論の丸い矢印の直径と、引用の人の印の大きさ(report の組み方だけが使う。dark / light は使わない。24) |
| `imageMinHeight` | 画像を縮めてよい下限の高さ(96。はみ出しの検査はこの高さで測る) |
| `captionGap` | 表・画像と説明の文の間(4。説明の文の字は `captionText`) |

### marks(印の文字)

箇条(`point`、入れ子は `subPoint`)、比較の利点(`pro`)・欠点(`con`)の頭に置く
文字、引用の言葉の頭(`quote`)と出どころの頭(`quoteSource`)、統計強調の数と単位の
間(`statUnit`。dark / light は空=「71%」「38社」と詰める。空でもよいのはこれだけ)。
後ろの空きも含めて書く。続けた表(`continues`)の各枚の印(`tableContinued`。`{n}` を
何枚目か、`{m}` をひと続きの枚数に置き換える。両方が無ければ読み込みで誤り。
dark / light / report は `{n}/{m}`。`-fit` で分けた本文のスライドの各枚の題の隣にもこの印を置く)。
`-fit` で分けた続きの枚(2枚目から)の題(`continuedTitle`。`{title}` を元の題に置き換える。
無ければ読み込みで誤り。dark / light / report は `{title}`(題はそのまま)、例の forest は
`{title}(続き)`)。目次のスライド(`meta.toc`)の題(`toc`。dark / light / report は「目次」)。

`point`・`subPoint`・`pro`・`con` は、PPTX では本文の文字にせず、段落の箇条の印
(`a:buChar`。色は印の色、字体は印の文字で選ぶ:U+2000 未満は `latin`、以降は
`eastAsian`)として書く。本文は印の幅(後ろの空きを含めて測ったとおり)だけ字下げ
した位置から始まり、折り返した行は印の下に戻る(測りと同じ)。印を文字にすると、
描く側が和文と欧文の間に足す空き(LibreOffice の「アジア言語と欧文の間隔」)が
「・」と数字・英字の間にだけ入り、数字で始まる箇条だけ頭がずれて見えたため(#42)。
印の後ろの空白は `buChar` に入れず、字下げの幅にだけ数える。印の太さは指定
できない(描く側の決まりに従う)。

スライドの出典(`source`)の頭に置く文字は `slideSource`(dark / light は「出典:」)。
文書の日付(`meta.date`)の見せ方は `date`(日のある日付)と `yearMonth`(`YYYY-MM` の
日付)で、`{YYYY}`(4桁の年)・`{MM}`・`{M}`(0 を付ける・付けない月)・`{DD}`・`{D}`
(日)を数に置き換える。dark / light は `{YYYY}年{M}月{D}日` と `{YYYY}年{M}月`。
読み込みで、`{YYYY}` と月の印が無い・`date` に日の印が無い・`yearMonth` に日の印がある・
知らない `{…}` があるのは誤り。

出典・日付・機密区分・免責・表の合計の行の置き場所(骨組み。コード):

| 意味 | dark / light |
|---|---|
| リード文 | 題の直下(`leadText`・`text_primary`、題との間は `headerGap`)。本文の領域はこの分だけ狭くなる |
| スライドの出典 | 本文の下・フッターの上(`small`・`text_muted`、間は `sectionGap`)。本文の領域はこの分だけ狭くなる |
| 日付 | 表紙の作者の下(`coverAuthor`・`text_muted`) |
| 機密区分 | 表紙の日付の下と、本文・主張のスライドのフッターの作者の右(`small`・太字・`text_primary`) |
| 免責 | 表紙の一番下(`small`・`text_muted`) |
| 表の合計の行 | 太字、上の線を `text_primary` に(`tableRuleWidth`)。PPTX の `a:tblPr` に `lastRow="1"` |

### fonts(字体)

欧文の字体(`latin`)・和文の字体(`eastAsian`)・置き換わり得る字体
(`fallbackFonts`)・幅の余裕(`widthMargin`)・行の送りを固定するか
(`fixedLineSpacing`)・行の送り(`lineHeight`。字の大きさに掛ける倍率)。字体は寸法
(`boxloom/fonts/*.json`)を持つものだけ選べ、無ければ誤り。和文の字体は仮名と漢字を
持つ字体に限る。`widthMargin` は 1〜1.5、`lineHeight` は 1〜2(dark / light は 1.35、report は 1.2)。
意味・測った数字・推奨する値は `docs/fonts.md`。

### dense(密なスライドの本文の値。必須、#116・#136)

本文のスライドに `density: dense`(Markdown は `# 題 {density=dense}`。`docs/ir.md` の 9.4)を
書いたとき、本文の部品・サブシステムが `values` の同じ名前の代わりに使う値の組。題・リード・
出典・フッターは `values` のまま。28のキーは全部書く(欠け・知らないキーは誤り)。字の大きさは
段(`typeSizes`)、間隔は余白の刻みに載り、`values` の同じ名前(図の寸法は元の図の値。下の2つ目の表)より
大きくはできない。0 以下は誤りで、表のセルの上下の余白(`tableCellPaddingY`)だけは 0 でもよい。

| キー | 意味 | dark・light | report |
|---|---|---|---|
| `body` | 本文の字(段落・箇条・図の箱の題) | 14(normal 16) | 11(normal 14) |
| `subPoint` | 入れ子の箇条・図の2段目 | 12(14) | 10(12) |
| `tableText` | 表のセルの字 | 12(14) | 10(11) |
| `small` | 注記・図の凡例 | 10(10) | 9(10) |
| `blockGap` | 本文のブロックの間 | 12(20) | 12(18) |
| `itemGap` | 箇条の項目の間 | 4(8) | 2(2) |
| `tableCellPaddingX` | 表のセルの左右の余白 | 4(8) | 2(4) |
| `tableCellPaddingY` | 表のセルの上下の余白 | 4(4) | 0(2) |
| `pairsRowGap` | 対の並びの対の行の間(report は行と罫線の間) | 8(20) | 2(4) |
| `pairMarkSize` | 対の印の一辺(report の対の行の高さの下限。dark・light は使わない) | 20(24) | 20(24) |
| `matrixRowTitle` | matrix の行の題の字 | 12(16) | 11(14) |

report の値は読む資料のサンプルの密なページ(本文 10〜12pt、表 10〜11pt でセルの余白が小さい)
から決めた。後の5つ(#136)は、dense にしても字ではなく行ごとの余白で収まらなかったページ
(表のセルの上下の余白・対の行の間と印・matrix の行の題の字が元の資料より大きい)のために足した。
report の表のセルの上下の余白は刻み(2pt)の下が 0 しか無いので 0(行の送り 1.2 の分の空きは残る)。

図と引用の字・寸法(#139)。図の箱の題は `body`、2段目は `subPoint` で上の表のとおり詰まるが、
補足の字・箱の余白と間合い・行の高さの下限は図の値のままだったので、`-fit` で段を下りても
図の最小の高さ(Min)が止まっていた。そのための17のキーで、元の値は `values` の同じ名前か、
このテンプレートの `subsystems.<図>`(書かなければ既定の `subsystems/<図>.json`)の寸法。
同梱の dense はどれも元の値と同じ(dense のスライドの見た目は #139 の前と変わらない)で、
詰めるのは fit の段(次の節)。

| キー | 元の値 | 意味 |
|---|---|---|
| `figureDetail` | `values.figureDetail` | 図の箱の補足(orgchart の3段目・phases の箱の補足)の字 |
| `calloutMark` | `values.calloutMark` | report の引用の人の印・結論の丸い矢印の大きさ |
| `quoteText` `quoteMark` `quoteSource` | `values` の同じ名前 | 引用の言葉・頭の印・出どころの字(report の吹き出しの余白は `cardPadding`) |
| `orgchartTextInsetY` `orgchartMinBoxHeight` `orgchartMemberGap` `orgchartMinLevelGap` | `subsystems.orgchart` の `textInsetY` `minBoxHeight` `memberGap` `minLevelGap` | 組織図の箱の上下の余白・箱の高さの下限・3段目の箱の間・段の間の下限 |
| `phasesTextInsetY` `phasesItemGap` `phasesHeaderGap` `phasesMinHeaderHeight` | `subsystems.phases` の `textInsetY` `itemGap` `headerGap` `minHeaderHeight` | フェーズの箱の上下の余白・箱の間・山形と箱の間・山形の高さの下限 |
| `matrixMinRowHeight` `matrixTextInsetY` `matrixMarkSize` `matrixCellGap` | `subsystems.matrix` の `minRowHeight` `textInsetY` `markSize` `cellGap` | matrix の行の高さの下限・セルの上下の余白・mark の丸・セルの間 |

`values` に無い図の寸法は、ほかの名前と同じく `typeSizes`・余白の刻みに載る(`textInsetY`・
`*Gap` は刻み、`min*Height`・`matrixMarkSize` は刻みを問わない。元の図の値と同じ)。

### fit(収めの梯子。任意、#126)

自動の収め(親 #125。作る側が `-fit` を明示したときだけ)が使う段の並び。`shrink` は
はみ出すスライドを詰める段、`grow` は薄いスライドを広げる段で、どちらも並べた順に1段ずつ
進む。梯子の端が下限・上限で、任意の倍率では縮めない。`shrink` は `build -fit`・`check -fit`
(#127、`docs/build.md`)が使う:はみ出す本文のスライドは今の密度から(normal は dense の組から、
dense は `shrink[0]` から)下り、最初に収まった段で組む。`grow` も同じ `-fit`(#128)が使う:
収まっていて fill-ratio が `fillRatioMin` を下回る normal の本文のスライドは `grow[0]` から上り、
はみ出さない最も上の段(fill-ratio が目標に届いたらその段)で組む。

各段は dense と同じく本文の値の一部を重ねる組で、書いた名前だけを前の段から替える
(書かない名前は前の段のまま)。`shrink` の最初の段は `dense` の組に、`grow` の最初の段は
`values` に重ねる(`cardPadding` は dense に無いので、詰めの最初の段でも `values` が前)。
同梱の梯子の `shrink` は本文と釣り合うように図・引用の名前(#136・#139)も下げる(#139。
下の3つ目の表。#139 の前は本文の字と間合いしか書かず、図が Min で止まると段を下りても
はみ出しが1pt も減らなかった)。`grow` は body と釣り合うように #136 の名前も上げる(#128。下の2つ目の表。matrix の行の題が本文より
小さく残らないよう、`matrixRowTitle` と `body` の差は values と同じ)。
書ける名前:

| キー | 意味 |
|---|---|
| `body` `subPoint` `tableText` `small` | 本文・入れ子の箇条・表・小さい字(字の段 `typeSizes` に載る) |
| `blockGap` `itemGap` `cardPadding` | ブロックの間・項目の間・カードの内側の余白(余白の刻みに載る) |
| `tableCellPaddingX` `tableCellPaddingY` `pairsRowGap` `pairMarkSize` | 表のセルの余白・対の行の間・対の印(余白の刻みに載る。`tableCellPaddingY` は 0 でもよい。#136) |
| `matrixRowTitle` | matrix の行の題の字(字の段に載る。#136) |
| `figureDetail` `calloutMark` `quoteText` `quoteMark` `quoteSource` と図の寸法の12の名前 | 図と引用の字・寸法(dense の節の2つ目の表。#139) |

誤り:知らないキー、`null`、0 以下(`tableCellPaddingY` は 0 まで)、段階に無い値、`shrink` の段で前の段より大きい値、
`grow` の段で前の段より小さい値、前の段から何も替えない段(空の組も)。`fit`・`shrink`・
`grow`・`emergency` はどれも書かなくてよい(書かなければその向きには収めない)。

同梱のテンプレートの梯子(括弧は重ねたあとの値。太字はその段で替えたもの):

| テンプレート | 段 | body | subPoint | tableText | small | blockGap | itemGap | cardPadding |
|---|---|---|---|---|---|---|---|---|
| dark・light | dense(起点) | 14 | 12 | 12 | 10 | 12 | 4 | 20 |
| | shrink[0] | **12** | **10** | **10** | 10 | **8** | 4 | **16** |
| | shrink[1] | **10** | 10 | 10 | 10 | **4** | 4 | **12** |
| | values(起点) | 16 | 14 | 14 | 10 | 20 | 8 | 20 |
| | grow[0] | **18** | **16** | **16** | 10 | **24** | **12** | **24** |
| | grow[1] | 18 | 16 | 16 | 10 | **32** | **16** | **28** |
| report | dense(起点) | 11 | 10 | 10 | 9 | 12 | 2 | 12 |
| | shrink[0] | 11 | 10 | 10 | 9 | **10** | 2 | **10** |
| | shrink[1] | 11 | 10 | 10 | 9 | **8** | 2 | **8** |
| | values(起点) | 14 | 12 | 11 | 10 | 18 | 2 | 12 |
| | grow[0] | **16** | **14** | **12** | 10 | **20** | **4** | **14** |
| | grow[1] | 16 | 14 | 12 | 10 | **24** | **6** | **16** |

広げの段の #136 の名前(重ねたあとの値。太字はその段で替えたもの):

| テンプレート | 段 | matrixRowTitle | tableCellPaddingX | tableCellPaddingY | pairsRowGap | pairMarkSize |
|---|---|---|---|---|---|---|
| dark・light | values(起点) | 16 | 8 | 4 | 20 | 24 |
| | grow[0] | **18** | **12** | **8** | **24** | **28** |
| | grow[1] | 18 | 12 | 8 | **32** | 28 |
| report | values(起点) | 14 | 4 | 2 | 4 | 24 |
| | grow[0] | **16** | **6** | **4** | **6** | **28** |
| | grow[1] | 16 | 6 | 4 | **8** | 28 |

詰めの段の図と引用の名前(#139。重ねたあとの値。太字はその段で替えたもの。dense は元の値と同じ):

| テンプレート | 段 | figureDetail | quoteText / quoteMark / quoteSource | calloutMark | matrixRowTitle | pairsRowGap / pairMarkSize | tableCellPaddingY |
|---|---|---|---|---|---|---|---|
| dark・light | dense(起点) | 12 | 16 / 36 / 12 | 24 | 12 | 8 / 20 | 4 |
| | shrink[0] | **10** | **14** / **24** / **10** | **16** | **10** | **4** / **16** | **0** |
| | shrink[1] | 10 | **12** / **18** / 10 | 16 | 10 | 4 / 16 | 0 |
| report | dense(起点) | 12 | 16 / 32 / 12 | 24 | 11 | 2 / 20 | 0 |
| | shrink[0] | **10** | **14** / **24** / **11** | **18** | **10** | 2 / **18** | 0 |
| | shrink[1] | 10 | **12** / 24 / **10** | **16** | 10 | 2 / **16** | 0 |

| テンプレート | 段 | orgchart の textInsetY / minBoxHeight / memberGap / minLevelGap | phases の textInsetY / itemGap / headerGap / minHeaderHeight | matrix の minRowHeight / textInsetY / markSize / cellGap |
|---|---|---|---|---|
| dark・light | dense(起点) | 4 / 32 / 12 / 24 | 8 / 8 / 12 / 36 | 28 / 4 / 14 / 4 |
| | shrink[0] | 4 / **24** / **8** / **16** | **4** / **4** / **8** / **28** | **20** / 4 / **10** / 4 |
| | shrink[1] | 4 / **20** / **4** / **12** | 4 / 4 / **4** / **24** | **16** / 4 / 10 / 4 |
| report | dense(起点) | 4 / 32 / 12 / 24 | 8 / 8 / 12 / 36 | 28 / 4 / 14 / 4 |
| | shrink[0] | **2** / **24** / **6** / **16** | **4** / **4** / **6** / **28** | **20** / **2** / **10** / **2** |
| | shrink[1] | 2 / **22** / **4** / **12** | 4 / 4 / **4** / **24** | **18** / 2 / **8** / 2 |

dark・light の余白の刻みは 4pt なので、箱の上下の余白(4)・matrix のセルの間(4)はそれより下げない。
report の matrix は mark(丸の下に小さい字)で行の高さが丸と字で決まるので、丸(`matrixMarkSize`)も下げる。

詰めは dense から字と間合いを1段ずつ下げた2段、広げは字と間合いを1段上げ、次に間合いだけを
もう1段上げた2段(同じデッキの中で本文の字がばらつきすぎないよう短くしてある)。report の
本文は元の資料の本文の最小に合わせて 11pt、表の字は元の資料の密な表(10〜11pt)に合わせて
10pt を下限にした(#139。report の shrink[1] は本文の字を下げず、ブロックの間・カードの余白と
図・引用の寸法だけを下げる)。

#### fit.emergency(非常用の段。任意、#150)

`-fit` で梯子(`shrink`)でも続きのスライドに分けても収まらない本文のスライド(分けられない
columns・図・画像、または分けた後も1枚に入らない1つの単位)だけが下りる段(`docs/build.md` の
「`-fit` の非常用の段」)。形は `shrink` と同じ組の並びで、`shrink` の最後の段(`shrink` が無ければ
`dense`)から続けて1段ずつ重ねる。段の検査も `shrink` と同じ(段階に載る・前の段より大きくならない・
何も替えない段は誤り)。字が値の表の決まりより小さくなるので、この段で組んだスライドには警告の
`fit-emergency` が出る。書かなければ非常用の段は無い(梯子と分けで入らなければ誤り)。

同梱の非常用の段(重ねたあとの値。太字はその段で替えたもの):

| テンプレート | 段 | body | subPoint | tableText | small | blockGap | cardPadding | figureDetail | matrixRowTitle | quoteText / quoteMark / quoteSource | calloutMark | pairMarkSize |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| dark・light | shrink[1](起点) | 10 | 10 | 10 | 10 | 4 | 12 | 10 | 10 | 12 / 18 / 10 | 16 | 16 |
| | emergency[0] | **8** | **8** | **8** | **8** | 4 | **8** | **8** | **8** | **10** / **16** / **8** | **12** | **12** |
| report | shrink[1](起点) | 11 | 10 | 10 | 9 | 8 | 8 | 10 | 10 | 12 / 24 / 10 | 16 | 16 |
| | emergency[0] | **10** | **9** | **9** | 9 | **6** | **6** | **9** | **9** | **11** / 24 / **9** | 16 | 16 |
| | emergency[1] | **9** | **8** | **8** | **8** | **4** | **4** | **8** | **8** | **10** / 24 / **8** | **14** | **14** |

図の寸法(orgchart の minBoxHeight / minLevelGap、phases の minHeaderHeight、matrix の minRowHeight / markSize):
dark・light の emergency[0] は **16** / **8**、**20**、**12** / **8**。report の emergency[0] は **20** / 12、24、**16** / 8、
emergency[1] は **18** / 12、**20**、**14** / 8。dark・light は字の段に 9pt が無いので1段(8pt)、report は 10pt → 9pt の2段。

### variants

このテンプレートが用意した変種の名前(IR の `variant`)。dark / light は空。

### subsystems.gantt(任意)

書くと、このテンプレートで作る Gantt だけがその寸法で測られ、描かれる
(登録簿の `Override` で差し替える。既定の表は変わらない)。形は次の節と同じで、
全部のキーが要る。dark / light は書いていない(既定を使う)。report は書いている(上の「report」の節)。

### subsystems.shapes(任意)

`subsystems.gantt` と同じく、書くとこのテンプレートの shapes だけがその寸法になる。
形は下の「subsystems/shapes.json の形」で、全部のキーが要る。dark / light は書いていない(report は phases だけ書き、番号をローマ数字にする)。

shapes の文字の大きさは、作者が `textStyle` に書く段の名前で values から引く:
`small`・`subPoint`・`optionItem`・`body`・`optionLabel`(`docs/subsystem.md` の8節)。
テーマに無い役割名を shapes が使うと、このテンプレートの組み立てが断る。

### subsystems.process・orgchart・matrix・funnel・phases・cycle・pillars(任意)

同じく、書くとこのテンプレートのその図だけがその寸法になる。全部のキーが要り、`padding`・
間隔・文字の余白(Go の json タグに `scale:"space"` の付いたもの)は段階の刻みに載って
いなければ誤り。差し替えないときは既定のファイルの値を、そのテンプレートの段階で見る。
dark / light は書いていない。図の字の大きさはファイルに持たず、values の `body`(題)・
`subPoint`(2段目・セル・注記)・`small`(補足・説明・凡例)から引く。キーの意味は
各ファイルと Go の型(`ProcessValues`・`OrgChartValues`・`MatrixValues`・`FunnelValues`・`PhasesValues`・`CycleValues`)の
注釈に書いた。最小・自然・上限の3つ組(`minBoxHeight`・`naturalBoxHeight`・
`maxBoxHeight` など)は Measure の Min・Natural・Max になる。

process・orgchart・funnel・phases の状態の凡例(`docs/subsystem.md` の11節)の寸法も各ファイルに
ある:`legendGap`(図と凡例の間)・`legendItemGap`(項目の間)・`legendSwatchWidth`・
`legendSwatchHeight`(見本の箱)・`legendSwatchGap`(見本と文の間)。funnel の `rateGap` は
帯の列と転換率の列の間。matrix の `emphasisLineWidth` は推す列の枠線の太さで、
`naturalRowHeight` 36・`maxRowHeight` 44 は1行の文のセルが間延びしない高さ。
matrix の `cellStyle` はセルの描き方で、`filled`(塗りの箱に状態の色の丸と文。既定・dark・light)か
`mark`(塗らない。support は丸に白いチェック、none は何も描かない、その他は状態の色の丸。
丸の直径 `markSize`、丸と文の間 `markGap`、文は丸の下に `small` の字)。ほかの形は誤り。
`groupRuleWidth` は列の群(`columnGroups`)の名前の下の罫線の太さ。report は matrix を
書いていて、`mark`・`markSize` 14・行の高さ 48〜72・行の見出しの列は幅の 60% まで。
`maxRowHeaderFraction`(既定 0.5)は行の見出しの列の幅の上限で、列の幅そのものは題と説明の
自然な幅と列(見出しとセルの中身。`mark` なら印)の自然な幅から決める(`docs/subsystem.md` の11節)。
phases の `numberStyle` は数でなく箱の番号の形で、`arabic`(1. 2.)か `roman`(I. II.)。
ほかの形は誤り。`bandStyle` は群の帯の見た目で、`outline`(枠線)か `fill`(基調色の塗りに
白い字。report)。`chevronDepth` は山形の先の深さ、`chevronGap` は山形と山形の間。
cycle は輪の太さ `ringFraction`(外の半径に対する割合)・扇の間 `gapAngle`(度)・番号の丸
`numberDiameter`・補足の幅 `labelMinWidth`〜`labelMaxWidth`・輪の直径 `naturalDiameter`・
`minDiameter`(扇の中に題が入らなければ、自然の直径は `naturalDiameter` の 1.75 倍まで
大きく申告する)・`outsideMinDiameter`(題が扇に入らず輪の外に置くときの輪の直径の下限)・濃淡 `shadeRange`(最後の段を地の色に寄せる割合)・`mutedShade`(強調が
あるとき、強調でない段を寄せる割合)を持つ。凡例の寸法は上と同じ。
pillars は屋根の斜めの辺 `roofSlant`・屋根/柱/土台の高さ(`roof…`・`pillar…`・`foundation…` の
`MinHeight`・`NaturalHeight`・`MaxHeight`。最小 ≤ 自然 ≤ 上限)・柱の間 `pillarGap`・屋根と土台の
張り出し `pillarOverhang`・柱の幅の下限 `pillarMinWidth`・名前の丸 `labelDiameter`・塗りの淡さ
`pillarTint`・`foundationTint`(基調色を地の色に寄せる割合。1 未満)を持つ(`docs/subsystem.md` の11節)。

### subsystems.chart(任意)

同じく、書くとこのテンプレートのグラフだけがその寸法になる。全部のキーが要り、`padding`
(段階の刻みに載ること)のほかは 0 より大きい数。グラフの字の大きさは values の
`subPoint`、色はテーマの役割名(`accent`・`text_muted`・`none`・`grid`・`bg`)から引く。

| キー | 意味 |
|---|---|
| `naturalWidth` `minWidth` | 幅が分からないときの自然な幅、最小の幅 |
| `minPlotHeight` | 値の軸の長さ(縦の形)・円の直径の最小。Measure の Min のもと |
| `naturalAspect` | 縦の形・円・散布図の自然な高さ(幅に対する割合) |
| `minBarSlot` `naturalBarSlot` `maxBarSlot` | 横棒の1項目の高さ(Min・Natural・Max) |
| `columnMaxPoints` | 時系列を縦棒で描く点の数の上限(越えたら折れ線) |
| `gapWidth` `histogramGapWidth` | 棒の間(棒の幅に対する %)、ヒストグラムの棒の間 |
| `lineWidth` `markerSize` `scatterMarkerSize` | 折れ線の太さ、折れ線・散布図の点の大きさ(pt) |
| `axisLineWidth` `gridLineWidth` `pieOutlineWidth` | 項目の軸の線・目盛りの線・円の扇の境の太さ |
| `labelMaxLines` `barLabelFraction` | 項目の名前を折り返してよい行の数、横棒の名前に使える幅の割合(`chart-labels` の基準) |
| `padding` | アプリがグラフの内側に取る余白の見積もり(Measure と `chart-labels` に使う) |

## subsystems/gantt.json の形

| キー | 意味 |
|---|---|
| `padding` | 箱の内側の余白 |
| `legendHeight` `legendGap` | 凡例の行の高さ、凡例と上の注釈のレーンの間 |
| `gutterGroupWidth` `gutterSubgroupWidth` `gutterLabelWidth` | 左の見出しの列の幅(グループ・サブグループ・行の名前) |
| `milestoneHeight` `axisLabelHeight` | 節目の行、目盛りの文字の行 |
| `groupGap` | グループの間 |
| `minRowHeight` `naturalRowHeight` `maxRowHeight` | 行の高さの下限・自然な高さ・上限(Measure の Min・Natural・Max になる) |
| `naturalChartWidth` `minChartWidth` | 日付軸の部分の自然な幅・最小の幅 |
| `annotationFixedWidth` `annotationFixedHeight` | 手で置いた注釈(Direction が up/down)の箱 |
| `annotationMinWidth` `annotationMaxWidth` | 自動の注釈の箱の幅の範囲 |
| `annotationPaddingX` `annotationPaddingY` | 自動の注釈の箱の中の余白 |
| `annotationLaneGap` | 注釈のレーンとチャートの間 |
| `annotationText` | 注釈の字の大きさ |
| `labelText` `smallText` | 凡例・行の名前・目盛りの字、グループ名・区間の文字・節目の字 |
| `axisStyle` | 軸の描き方(文字列)。`ticks`(書いた目盛りだけ)か `monthGrid`(月ごとの列と年・月の2段の見出し)。ほかは誤り |
| `barShape` | 帯の形(文字列)。`rect`(角の丸い矩形)か `chevron`(右に尖った山形)。ほかは誤り |
| `fillHeight` | 真偽。true なら Measure の Max を上限なしにし、余った高さを注記の行を除く行に配る(行は `maxRowHeight` まで) |
| `gridRowHeight` | 月の格子の年の行・月の行の高さ |
| `gridYearFormat` `gridMonthFormat` | 月の格子の年の文字(`YYYY`・`YY` を置き換え、月は書けない)と月の文字(`MM`・`M` を置き換え、年は書けない) |
| `chevronFillOpacity` | 山形の帯と色見本の面の不透明度(0 より大きく 1 以下) |
| `dateFormat` | IR から作った Gantt の目盛りと節目の日付の書き方(文字列)。`YYYY`・`YY`・`MM`・`M` を年と月に置き換え、ほかの文字はそのまま。既定 `YYYY/M`(`2026/12`)。月(`M` か `MM`)を含まない・年を `YYYY`/`YY` 以外で書いたものは誤り。Go で組んだ従来のデモの目盛りは手で書いた文字のまま |

## subsystems/shapes.json の形

| キー | 意味 |
|---|---|
| `padding` | 枠の内側の余白。相対座標の 0〜1 はその内側を指す |
| `naturalWidth` | 幅が分からないときの自然な幅 |
| `defaultAspect` | `aspect` を省いたときの内側の縦横比(幅÷高さ) |
| `minWidth` `minHeight` | 最小の大きさ(高さは、図形の文が収まる高さとの大きいほう) |
| `textInsetX` `textInsetY` | 図形の中の文字の余白 |
| `lineWidth` | line・arrow・connector の太さ |
| `outlineWidth` | 図形の枠線の太さ |
| `cornerRadius` | roundRect の角 |

**描く処理の細部の寸法**(Backlog #70 でコードから移した。値は移す前のまま):

| キー | 意味 |
|---|---|
| `legendSwatchWidth` `legendSwatchHeight` | 凡例の色見本の大きさ(15×7) |
| `legendSwatchTop` `legendLabelRaise` | 色見本を行の上端から下げる分、凡例の文の箱を上げる分(2・2) |
| `legendSwatchGap` `legendLabelGap` | 色見本と文の間(4)、文の後ろ(6。この後に `padding` を空けて次の項目) |
| `legendSwatchOutlineWidth` `legendSwatchRadius` | 色見本の枠の太さ・角(0.5・2) |
| `milestoneMinWidth` `milestonePaddingX` | 節目の文字の箱の最小の幅(148)、左右の余白(4) |
| `milestoneAxisGap` `milestoneLabelGap` | 節目の段と目盛りの文字の間(4)、同じ段の節目の文字どうしの最小の隙間(6) |
| `tickLabelWidth` `tickLabelHeight` | 目盛りの文字の箱(64×13。日付の真上に中央揃え) |
| `tickSubLabelOffset` `tickSubLabelWidth` `tickSubLabelHeight` | 目盛りの副ラベルの箱を下げる分(12)と箱(80×12) |
| `axisGap` | 目盛りの文字の行と軸の線の間、軸の線と本体の間(5) |
| `tickDotRadius` | 軸の上の目盛りの点(2.2) |
| `breakMarkWidth` `breakMarkHeight` `breakMarkSpacing` `breakMarkHaloWidth` | 区間圧縮の印(//)の1本の幅と高さ(6・14)、2本の間(6)、下に地の色で引く線の太さ(3) |
| `groupBoxGap` `groupInsetX` `groupRadius` | グループ名の箱とサブグループの列の間(4)、箱の中の左右の余白(3)、角(6) |
| `subgroupInsetLeft` `subgroupInsetRight` | サブグループの文字の箱の左右の余白(2・4) |
| `rowLabelGap` | 行の名前と時間軸の間(8) |
| `barInset` `barRadius` | 行の高さと帯・注記の箱の高さの差(6。上下に半分ずつ)、帯・注記の箱の角(3) |
| `segmentLabelInsetLeft` `segmentLabelInsetRight` | 帯の中の文の左右の余白(5・3) |
| `noteInsetX` | 注記の箱の中の左右の余白(5) |
| `openEndDotOffset` `openEndDotStep` `openEndDotRadius` | 終わりの無い帯の後ろの点:帯の端から最初の点まで(5)、点の間(6)、点の大きさ(1.4) |
| `extNoteLabelHeight` `extNoteLabelGap` | 延伸ノートの文字の箱の高さ(13)、箱と線の間(2) |
| `annotationFixedPaddingX` `annotationFixedPaddingY` | 手で置いた注釈の箱の中の余白(8・4) |
| `annotationRadius` | 注釈の箱の角(8) |
| `annotationGap` `annotationShift` | 同じレーンの注釈の箱の間の最小の隙間(4)、置けないときにずらす刻み(箱の幅に足す。8) |
| `anchorDotRadius` `leaderGap` | 注釈が指す先の点(3)、引き出し線が文字を避けるときの隙間(2) |
| `thinLineWidth` | 目盛りの縦の破線・グループの境・注記の破線の枠(0.75) |
| `lineWidth` | 軸・仮の帯の破線の枠・延伸ノートの線・注釈の箱の枠(1.25) |
| `leaderLineWidth` `emphasisLineWidth` | 引き出し線と区間圧縮の印の線(1)、強調した注釈の縦の線(2) |

**決まり:図の中の細部の寸法は段の外**(Backlog #70)。図の中の細部の寸法は、線の太さ・
角と同じく段に載せない。値の表に名前つきのキーとして置き(テンプレートは
`subsystems.gantt` で差し替えられる)、0 以下は読み込みで誤り(`checkPositive`)。
段の刻みには寄せない。ページのリズムに属する余白・間隔(図の外周・図と凡例の間・
グループの間・注釈のレーンなど)と、文字の箱の大きさのうち段で揃えるものは、
これまでどおり段で見る。

subsystems/gantt.json のキーの分け方:

| 種類 | 検査 | キー |
|---|---|---|
| 段で見る余白・間隔・箱の大きさ(`scale:"space"`) | `spacingUnit` の倍数・0 より大きい | `padding` `legendGap` `groupGap` `annotationPaddingX` `annotationPaddingY` `annotationLaneGap` `legendSwatchGap` `milestonePaddingX` `milestoneAxisGap` `milestoneMinWidth` `tickLabelWidth` `tickSubLabelOffset` `tickSubLabelWidth` `tickSubLabelHeight` `groupBoxGap` `subgroupInsetRight` `rowLabelGap` `annotationFixedPaddingX` `annotationFixedPaddingY` `annotationGap` `annotationShift` |
| 字(`scale:"type"`) | `typeSizes` のどれか | `annotationText` `labelText` `smallText` |
| 図の中の細部の寸法(段の外) | 0 より大きい | `legendSwatchWidth` `legendSwatchHeight` `legendSwatchTop` `legendLabelRaise` `legendLabelGap` `milestoneLabelGap` `tickLabelHeight` `axisGap` `groupInsetX` `subgroupInsetLeft` `barInset` `segmentLabelInsetLeft` `segmentLabelInsetRight` `noteInsetX` `extNoteLabelHeight` `extNoteLabelGap` `leaderGap` `breakMarkWidth` `breakMarkHeight` `breakMarkSpacing` `openEndDotOffset` `openEndDotStep` |
| 線の太さ・角・点の大きさ(段の外) | 0 より大きい | `thinLineWidth` `lineWidth` `leaderLineWidth` `emphasisLineWidth` `breakMarkHaloWidth` `legendSwatchOutlineWidth` `legendSwatchRadius` `groupRadius` `barRadius` `annotationRadius` `tickDotRadius` `anchorDotRadius` `openEndDotRadius` |
| 図の大きさ・行の高さ・欄の幅(段の外。以前から) | 0 より大きい | `legendHeight` `gutterGroupWidth` `gutterSubgroupWidth` `gutterLabelWidth` `milestoneHeight` `axisLabelHeight` `minRowHeight` `naturalRowHeight` `maxRowHeight` `naturalChartWidth` `minChartWidth` `annotationFixedWidth` `annotationFixedHeight` `annotationMinWidth` `annotationMaxWidth` `gridRowHeight` |
| 描き方の選び方(#89) | 知っている名前・範囲 | `axisStyle` `barShape` `fillHeight` `gridYearFormat` `gridMonthFormat` `chevronFillOpacity`(0〜1) |

行の数・段の数・比などの決まりの数は値の表に置かず、名前つきの定数としてコードに
残した(`docs/subsystem.md` の13節)。

## コードに残したもの

- **骨組みと部品**(`template.go`):表紙・本文のスライドの並び、ブロックの組み方。
- **従来のデモ**(`go run ./cmd/boxloom`、`demo.go` の `buildSlideSpec`):v11 の再現なので、
  題の大きさなどの数はそのまま。色はテンプレートのファイル(dark / light)から引く。
- `fill-ratio` の上限(100%)。はみ出しはテンプレートの方針ではなくコアの検査なので。
- サブシステムの中の決まりの数(行・段の数の上限、比、アプリの振る舞いの見積もり)。
  名前つきの定数で、理由を注釈に書いた(`docs/subsystem.md` の13節)。Gantt の描く
  処理の細部の寸法は #70 で値の表に移した(上の「subsystems/gantt.json の形」)。

## 確かめたこと

- 移す前と後で、従来のデモ2枚と、examples の IR・Markdown(直下のもの)を dark / light で
  作った PPTX(合わせて14件)の中身が完全に同じ(展開して `diff -r`)。`schema` の出力と、
  `check` の例への結果も同じ。
- 値を変えると出力が変わる(`body` を変えると本文のスライドが変わる)。
- `values_test.go`:同梱のファイルが読める/知らないキー・欠けたキー・null・型の違い・
  0・役割名の欠けと余り・色の形・空の印・名前の食い違い・Gantt の値の一部だけ・
  知らないサブシステム・後ろに続くもの、をどれも断る/`subsystems.gantt` の差し替えが
  そのテンプレートの Gantt の高さだけを変える。
