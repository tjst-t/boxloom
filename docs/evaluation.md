# check の警告が作者を導くかの評価(#161)

外部のサンプルの見た目を再現する進め方はやめた(人が決めた)。代わりに、Boxloom の書き方
(`skills/boxloom/SKILL.md`)で書いた架空の資料で、`check`・`build` が作者を正しく導くかを
確かめる。この文書はその1回目の結果(2026-11-15、コミット 8f86056 の上)。

## 1. 資料

`examples/eval/` に置いた。組織名・人名・地名は使っていない(「市」「館」「案A」など)。

| ファイル | テンプレート | 枚数(build) | 含むもの |
|---|---|---|---|
| `proposal.bx.md` | dark | 13 | 表紙、区切り3つ、columns+chart、pairs(引用の側つき)、表(合計)、matrix(emphasis・legend)、compare 3案、columns+cycle、stat+表、process(legend)、番号つきの箇条+表+takeaway |
| `report.bx.md` | report | 19(目次を含む) | 表紙、目次(`toc: true`)、区切り3つ、stat+箇条+takeaway、columns+chart(stacked)、表(groups・total)、topics(label・引用)、orgchart、pairs(側の見出し)、表(groups・points・total)、matrix、compare+表、phases(groups・legend)、stat+表(total)、表+段落、`{density=dense}` の表と箇条、全ての本文に出典 |
| `brief.bx.md` | light | 7 | 表紙、statement、stat+表、columns+chart、Gantt(注釈・節目・legend)、箇条+takeaway、番号つきの箇条+表。画像なし |
| `*-faulty.bx.md` | 同じ | — | 正しい版に誤りを入れた版(2節)。警告になる誤りだけをまとめる |
| `report-faulty-errors.bx.md` | report | — | 誤り(error)になる R5 だけを入れた版。check は誤りがあると警告を出さないので分けた |
| `faults.py` | — | — | 誤った版を正しい版から作る。`--each DIR` で誤り1つずつの版と、hint に従って直した版を書く |

書き方は SKILL.md のとおり:本文のスライドは短い1行の題と `:::lead`、図の中は短い言葉で
全文は横の箇条、数・表・図のあるスライドには `:::source`。

### 1.1 正しい版の結果

3本とも `check -format json` で誤り0件・警告0件・info 0件。`build`(-fit なし・あり)とも
終了コード 0。`go test` の `TestEvalExamplesClean` が、正しい版が警告を出さないことを守る。

作る途中で出た警告と、それで直したこと(作者として導かれたか):

- fill-ratio は14件出た(proposal 4、report 8、brief 0)。hint に従い、根拠の表・箇条・対・話題を
  足して全て消えた。compare の hint の「一番高いカード」は、3つ目の案を足す判断に役立った。
- layout-overflow は3件出た。どれも「表の後のブロックがはみ出す」形で、hint が小さい表を
  分ける案を出した(3節の D1)。
- 隣とまとめる hint は、report の3枚の続きで向きがそろわなかった(D2)。

LibreOffice(PDF→PNG)で全ての枚を見た。崩れは次の4つ(check は何も出さない):

| # | 資料・枚 | 見えたもの |
|---|---|---|
| V1 | report 6枚目(館ごとの利用と人手) | 行の群の縦書きの帯「拠点の館」(4字)が群の2行より高く、駅前館の行の下に空きができる。合計の行の「合計」が細い群の列で「合/計」の2行に折れる |
| V2 | report の全ての本文の枚 | フッターの作者名「図書館の運営の見直しの作業部会」(15字)が2行に折れ、2行目がフッターの下にはみ出す |
| V3 | brief 5枚目(年度の予定) | Gantt の注釈「本人確認の作り直しで2か月の遅れ」の最後の字が注釈の枠の外の2行目に落ちる |
| V4 | proposal 13枚目(今日決めていただきたいこと) | 表の「時期」の列(1月・3月・7月)が数の列として右寄せになり、列も広い |

## 2. 誤った版

各資料に7〜8個の誤りを入れた(`faults.py` の表。P=proposal、R=report、B=brief)。下の表は
誤りを1つずつ入れた版の結果で、まとめた版の結果も同じ。誤り(error)になる R5 だけは
`report-faulty-errors.bx.md` に分けた(check は誤りがあると警告を出さないので、まとめると他の誤りの
警告が見えなくなる)。`report-faulty.bx.md` はほかの6つをまとめ、check は R1〜R4・R6 の警告を出す。

- **出たか**:-fit なしの check の code(-fit ありで変わるときは → の後)。
- **build**:-fit なし / -fit あり の終了コード。
- **読んで直せるか**:message と hint だけを読んだときの判定(分かる・あいまい・誤導)。
- **直したあと**:message・hint の言うとおりに直したときに、その警告が消えたか。

| # | 入れた誤り | 出たか | build | 読んで直せるか | 直したあと |
|---|---|---|---|---|---|
| P1 | 題が2行に折れる(本文の題に主張の文を書く) | `title-wrap` | 0 / 0 | 分かる(「1行に」。hint は空で、主張を lead に移すことは言わない) | 消えた |
| P2 | リードが3行(dark) | 出ない(#162 で `lead-wrap` が出るようにした) | 0 / 0 | — | — |
| P3 | cycle の題が長い(9〜12字) | `cycle-cramped` | 0 / 0 | 分かる(題を短く・全文は横の文に) | 消えた |
| P4 | matrix を columns の狭い列に置く(文の列を role=main) | `layout-overflow`(列1、291.6pt)→ -fit で `fit-emergency` | 1 / 0 | あいまい(「dense でも足りない。分けるか短く」。図が列に押されて縦に伸びたことも、図を列から出す・図の列を広げることも言わない) | 消えた(matrix を列から出した) |
| P5 | process の下の箇条を増やしてはみ出す | `layout-overflow` → -fit で `fit-applied` | 1 / 0 | 分かる | 消えた |
| P6 | compare の下の段落と takeaway を消す | `fill-ratio`(48.8%)→ -fit で `fit-applied` だけ | 0 / 0 | 分かる(一番高いカードの名前も出る) | 消えた。-fit では警告が info に変わり、`-strict` でも通る |
| P7 | chart の項目の名前が長い(「2024年度の上期」) | 出ない | 0 / 0 | — | —(名前は2行に折れて読める。警告は要らないと判断) |
| P8 | 数の表のスライドに出典が無い | 出ない(#166 で `source-missing` が出るようにした) | 0 / 0 | — | — |
| R1 | リードが4行(report) | `lead-wrap` | 0 / 0 | 分かる(「短く」) | 消えた |
| R2 | リードの無いスライドの題が2行に折れる | `title-wrap` | 0 / 0 | あいまい(「1行に」だけ。主張を `:::lead` に移す直し方は SKILL.md にしか無く、題を削るだけの直しに誘う) | 消えた(lead を足した) |
| R3 | groups・points の表が長すぎる(13行) | `layout-overflow`、分け方の案 rows[9] の前で 9・3 行 → -fit で `fit-applied` | 1 / 0 | 分かる(どこで分けるか・continues の書き方まで) | 消えたが、案のとおりに分けると続きの枚(3行)に `fill-ratio` が出る。1回目の直しで合計の行を最初の表に残して `table-continues` の誤りになったが、誤りの文で直せた |
| R4 | phases を columns の狭い列に置く | `layout-overflow`(列1、52.2pt)→ -fit で `fit-applied` | 1 / 0 | 誤導(hint は「{density=dense} で収まります」と勧める。図を狭い列に詰めたことが原因で、dense は読ませる参照のページのためのもの) | 消えた(phases を列から出した) |
| R5 | 群の列の頭が空、`points=` の列名の誤り | 誤り `table-groups` ×2、`table-points` | 1 / 1 | 分かる。ただし空の行ごとに同じ「最初の行の群の名前が空」が出て、2件目は2行目を指す | 消えた |
| R6 | 提言の表を消す | `fill-ratio`(40.9%。-fit でも 52.1%) | 0 / 0 | 分かる | 消えた |
| R7 | orgchart の箱の名前が長い | 出ない | 0 / 0 | — | —(細い箱の中で4行に折れて読みにくいが、はみ出しはしない) |
| B1 | statement の文が長い(5行) | 出ない(#167 で `statement-wrap` が出るようにした) | 0 / 0 | — | — |
| B2 | Gantt の帯の名前が長い | 出ない | 0 / 0 | — | —(帯の中で2行に折れて読める。決まりどおり) |
| B3 | 節目を3つ近くに置く | `milestone-overlap` | 0 / 0 | 分かる(hint は空だが message で分かる) | 消えた |
| B4 | stat の数と単位が長い | `stat-value` | 0 / 0 | 分かる(補足は label へ) | 消えた |
| B5 | 箇条を増やしてはみ出す | `layout-overflow`(**takeaway** を指す)→ -fit で `fit-applied` | 1 / 0 | あいまい(はみ出すのは最後の takeaway で、伸びたのは前の箇条。SKILL.md の「そのブロックから後を次の枚へ」に従うと結論だけが次の枚に行く) | 消えた(箇条を戻した) |
| B6 | 下期の表を消す | `fill-ratio`(41.1%。-fit でも 54.5%) | 0 / 0 | 分かる | 消えた |
| B7 | takeaway を2つ書く | `layout-overflow`(2つ目の takeaway)だけ | 1 / 0 | 誤導(「1枚に1つまで」の決まりは出ず、はみ出しとしてしか分からない) | 消えた |

まとめ:22個の誤りのうち、警告・誤りが出たのは16個。そのうち分かる 11、あいまい 3(P4・R2・B5)、
誤導 2(R4・B7)。出なかった6個のうち、P7・B2 は見た目が崩れないので出なくてよい。P2・P8・
R7・B1 と、B7 の「takeaway は1枚に1つ」の決まりは出るべき(4節)。

## 3. 作る途中・見た目で見つかったこと

- **D1**:表の後の小さいブロック(箇条・takeaway)がはみ出すと、hint が表を分ける案を出す。
  proposal の下書きでは3行の表を「1行と2行」に、brief の下書きでは4行の表を「2行と2行」に
  分けるよう勧めた。表が原因でないときに表を分けさせるのは誤導。
- **D2**:fill-ratio の「隣とまとめる」hint が連なる薄い3枚で向きがそろわない
  (15→16、16→17、17→16)。どの2枚をまとめればよいか読み取りにくい。
- **-fit**:P4 の図が詰まった枚は -fit で非常用の段(本文 8pt)で組まれ、build は 0 で終わる
  (警告 `fit-emergency` は残る)。P6 は -fit の広げで fill-ratio が info だけになる。どちらも
  設計どおり(#128・#150)だが、-fit を付けた作者は警告を見落としやすい。
- 見た目の崩れ V1〜V4(1.1節)。

## 4. 直すべきこと(起票済み)

下の候補は Backlog に起票した(番号は表の Backlog の列)。このタスクでは直していない。

| 候補 | Backlog | 種類 | 内容 | もとの判定 | 優先 |
|---|---|---|---|---|---|
| E1 | #162 | 警告が出ない | dark・light にもリードの行の上限(2行)を持たせ、`lead-wrap` を出す | P2 | 高 |
| E2 | #163 | hint | 列の中の図(matrix・phases など)のはみ出しでは、dense を勧めず「図を列から出す・図の列を広げる(role=main を図の側に)」を言う(#163 で直した。容量の `subsystem`・`fullWidthNeed`) | P4・R4 | 高 |
| E3 | #164 | hint | はみ出すブロックが表・本文の後の小さいブロック(takeaway・箇条)だけのとき、表を分ける案を出さない。伸びたのが前のブロックなら、そのブロックを短くする・分けると言う | D1・B5 | 高 |
| E4 | #165 | 警告が出ない | 1枚に `:::takeaway` が2つ以上あれば誤り(または警告)にする(SKILL.md の「1枚に1つまで」) | B7 | 中 |
| E5 | #166 | 警告が出ない | 数・表・図・引用のある本文のスライドに `:::source` が無ければ警告する(#166 で直した。数・数の表・グラフ(chart・funnel・stat)が対象、引用は `quote` の source が必須なので見ない。架空の数の資料は `meta.fictional`。report の要約の stat に出典を足した) | P8 | 中 |
| E6 | #167 | 警告が出ない | statement の文の行の上限(3行、`statementMaxLines`)を超えたら `statement-wrap` を警告する(#167 で直した) | B1 | 中 |
| E7 | #168 | hint | `title-wrap`・`lead-wrap`・`milestone-overlap` に hint を付ける(title-wrap は「主張を `:::lead` に移し、題は話題の短い見出しに」。#168 で直した。lead-wrap は「言いたいことを1文にし、根拠は本文へ」、milestone-overlap は「近い節目をまとめるか、片方を注釈に」) | P1・R2・B3 | 中 |
| E8 | #169 | hint | 表を分ける案を、最後の表が薄くならないように行をならす(R3 なら 9・3 ではなく群の境で 5・7 など)。または分けた後の fill-ratio を見込む | R3 | 中 |
| E9 | #170 | 見た目 | 行の群の縦書きの帯が群の行より高いとき行が伸びる。合計の行の名前が細い群の列で縦に折れる | V1 | 中 |
| E10 | #171 | 見た目 | フッターの作者名が長いと2行に折れてはみ出す。check で警告するか、1行に収める | V2 | 中 |
| E11 | #172 | 見た目 | Gantt の注釈の枠が文より狭く、最後の字が枠の外に落ちる(LibreOffice) | V3 | 中 |
| E12 | #173 | 見た目 | 「1月」のような数+単位の短い列を数の列として右寄せにしない(または列の幅を中身に合わせる) | V4 | 低 |
| E13 | #174 | 誤りの文 | `table-groups` が空の行ごとに「最初の行の群の名前が空」を繰り返す。最初の1件にまとめる | R5 | 低 |
| E14 | #175 | hint | 「隣とまとめる」hint の向きを、連なる薄い枚でそろえる | D2 | 低 |
| E15 | #176 | 警告が出ない | orgchart の細い箱で名前が3行以上に折れるとき警告する(はみ出しは無い) | R7 | 低 |

## 5. 流し方

```sh
cd boxloom && go build -o /tmp/boxloom ./cmd/boxloom && cd ../examples/eval
python3 faults.py                       # *-faulty.bx.md と report-faulty-errors.bx.md を正しい版から作り直す
python3 faults.py --each /tmp/each      # 誤り1つずつの版(*-P1.bx.md)と直した版(*-P1-fixed.bx.md)
/tmp/boxloom check -format json proposal-faulty.bx.md
/tmp/boxloom check -format json -fit proposal-faulty.bx.md
/tmp/boxloom check -format json report-faulty-errors.bx.md   # 誤り(R5)だけ
/tmp/boxloom build -o /tmp/p.pptx proposal.bx.md && /tmp/boxloom build -fit -o /tmp/p-fit.pptx proposal.bx.md
soffice --headless --convert-to pdf /tmp/p.pptx
```

正しい版を直したら `python3 faults.py` で誤った版も作り直す(正しい版の文が見つからなければ
止まる)。
