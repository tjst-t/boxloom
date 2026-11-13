# Boxloom — 宣言的PPTX生成システム(Go移植版)

このファイルはClaude Code(または後継の自作エージェント)がこのリポジトリを
引き継いで作業を続けるための設計メモ。チャットでの長い議論の結論だけを
ここに凝縮してある。経緯の全文は `HANDOFF.md` を参照。

プロジェクト名は **Boxloom** に確定済み(旧作業名「pptxgo」から改名)。
Goモジュール名は `boxloom`。名前決定の経緯は `HANDOFF.md` の「プロジェクト名の
決定経緯」を参照。

## プロジェクトの構想(ぶれない軸)

LaTeX/Beamerのように、**テンプレート(レイアウト・デザイン)とコンテンツを分離**して
宣言的にPowerPointを生成するシステム。手動のピクセル座標指定ではなく、実際の
レイアウトエンジン(Flexboxライクな2パスアルゴリズム)に自動配置させ、
python-pptx/自前OOXMLライターでネイティブ編集可能なシェイプとして出力する。

OSSとして公開する価値があると判断し、その前提で設計している。

方向性(AI が作るスライドの3つの弱さへの答え方・3層の構想・立ち位置・リスク)と
その実装の状態は `docs/vision.md`。内容の型の語彙と、内容から型を選ぶ規則
(ブロックかサブシステムかの線引きを含む)は `docs/patterns.md`。

## 設計原則: 三層の分離(コア=仕組み/テンプレート=方針/コンテンツ=意味)

Boxloom の核は **テンプレートと意味論(コンテンツ)の分離** にある。作者は TeX の
ように意味だけを書き、見た目はテンプレートが決める。そのための手段として、
**入れ子にできる Box レイアウト** があり、その葉には **PPT のプリミティブ要素**
(テキスト・図形など)か、**サブシステム**(Gantt チャートのような複雑な図)を
埋め込める。

| 層 | 持つもの | 持たないもの |
|---|---|---|
| コア(仕組み) | Box の木と配置計算、テキスト計測、プリミティブ、サブシステムの差し込み口、役割名を解決する仕組み、OOXML の書き出し | 具体的な色・余白・フォント、デザインの良し悪しの判断 |
| テンプレート(方針) | 役割名→実際の値の表(テーマ)、スタイル規則、スライドの骨組み、意味ブロックを Box の部分木に組む部品、デザイン規則と検査 | コンテンツそのもの |
| コンテンツ(意味) | 意味の木(スライド・見出し・箇条・強調・比較・Gantt のタスクと状態…) | 座標・色・Box の組み方 |

### コア(仕組み)

- **Box レイアウト**: Row/Col を入れ子にした木を配置する(後述の outer layout
  engine)。
- **コンテナは既定で仮想**: 背景色や枠線などのスタイルを持たない限り、
  シェイプとしては実体化しない。
- **プリミティブ**: PPT のネイティブ要素として出力する葉(いま実装があるのは
  テキストと矩形)。
- **サブシステムの差し込み口**: サブシステムは割り当てられた矩形と役割名の
  解決器を受け取って描く。outer engine とは独立していて、エンジンを差し替えても
  サブシステムは変わらない(後述の inner canvas sub-engine)。
  **サブシステムは自分の自然な大きさと最小の大きさを自己申告する**
  (Go 版で実装済み。`Subsystem` interface の `Measure`。詳細は
  `docs/subsystem.md`)。テンプレートは登録簿(`SubsystemRegistry`)で
  サブシステムの描き方を差し替えられる。
- **役割名の解決**: コアは「役割名→値」を引く仕組みだけを持ち、具体的な値は
  知らない。
- **表現できる範囲を狭めない**: 中央揃えのようにデザインとして避けたいものも、
  コアとしては表現できる。禁じるかどうかはテンプレートが決める。

### テンプレート(方針)

テンプレートが手を入れられる段階を、軽いものから順に重ねる。

1. **値の表(トークン/テーマ)**: 役割名に実際の色・文字サイズ・余白を当てる。
   16進の色を知っているのはここだけ。余白と字の大きさは値の表の段階(`scale`:
   余白の刻みと字の大きさの段)に載っていなければ読み込みで誤り。
2. **スタイル規則**: 意味の種類と入れ子の位置で見た目を決める(CSS のセレクタに
   近いもの)。
3. **スライドの骨組み**: 名前つきの枠(title / body / aside / footer …)を持つ
   Box の木。コンテンツは枠の名前だけを指す。
4. **部品**: 意味ブロック(比較・箇条など)を Box の部分木に組み立てる。
5. **サブシステム**: Gantt のような独立した図の描き手。
   **shapes**(生の図形への逃げ道)もサブシステムの1つで、既存のパターンで表せない
   図のときだけ使う最後の手段。図形はサブシステムの枠の中にだけ置け、座標は枠の
   内側の 0〜1 の相対、色は役割名だけ、字の大きさはテンプレートの段の名前
   (`small`・`subPoint`・`optionItem`・`body`・`optionLabel`)。図形の重なりは
   check の警告(`shapes-overlap`)。詳細は `docs/subsystem.md` の8節。
   箱を矢印で並べる・親と子の箱・格子に評価・減っていく帯は、shapes ではなく
   **process・orgchart・matrix・funnel** のサブシステムで書く(段ごとに箱を積む事業の全体像は
   **phases**。意味だけを書く。
   `docs/subsystem.md` の11節)。数の系列は **chart**(PowerPoint の本物のグラフ+
   値を持つ埋め込みの xlsx)。作者は比べ方(Zelazny の5分類 `compare`)と項目・系列・
   値・単位・強調だけを書き、グラフの形・色・大きさはサブシステムが選ぶ。軸・ラベルの
   置き場所はアプリが決めるので、Boxloom は枠の Min を測ってはみ出しを判定し、
   項目の名前が入りきらないことだけを警告する(`docs/subsystem.md` の12節)。

**データとコードの混ぜ方(合意済み)**: 値の表・スタイル規則・骨組みはデータ
(宣言ファイル)で書き、部品とサブシステムはコードで書く。いまデータに
なっているのは値の表(色・大きさ・印の文字・字体、Gantt の寸法)で、
`boxloom/templates/<名前>.json` と `boxloom/subsystems/gantt.json`・`shapes.json` ほか(同梱。
知らないキー・欠けたキーは誤り)。骨組みはまだコードで、部品の登録簿と同じ作りの表
(`skeleton.go`。表紙・本文の頭・本文の並べ方・区切り・主張・フッター・出典・目次の8つ)に載せ、
テンプレートが名前ごとに差し替えられる(`docs/layers.md` の8.4)。詳細は
`docs/template-values.md`。字体はテンプレートが選び、字体ごとの寸法
(`boxloom/fonts/*.json`)で測る。置き換わったときの余裕は `docs/fonts.md`。

**プラグイン(候補・未決定)**: こだわりたいテンプレート開発者向けに、コードを
書いてコアの決められた範囲をオーバーライドできる仕組みを持つ案がある。
どこまでを差し替え可能にするかは未定。

**デザイン規則と検査もテンプレートが持つ**。いまのデモのテンプレート
(`dark` / `light`)の方針は次のとおりで、これらはコアの原則ではなく、
このテンプレートの方針の例である。

- **中央揃え禁止**: `align_items=CENTER` は「端をそろえる」原則(ノンデザイナーズ・
  デザインブック的な考え)に反するので使わない。
- **SPACE_BETWEEN はフッター限定**: `justify_content=SPACE_BETWEEN` のような
  主軸方向の余白配分は、フッターなど明示的に宣言された単機能のフレーム要素の
  内部でだけ使う。コンテンツ領域の余白を埋めるのには使わない。宣言のない大きな
  余白は、fill-ratio の数値に関わらず常に NG。
- **コンテンツ不足を自動で埋めない**: fill-ratio の検査で警告を出すだけで、
  他ページからの流用などはしない。図・画像が横に残した空きも `spare-width` の警告と
  容量の `remainingWidth` で知らせるだけ。警告の hint に直し方(隣の薄いページと
  まとめる見込みなど)を添え、対応は SKILL.md で作者に促す(`hints.go`・`spare.go`)。
- **内容の良し悪しは判定しない**: 題が結論か・論の立て方などは見ない。量で測れる
  ものだけを警告にする。
- **はみ出しも自動で直さない**: 本文が収まらないときは `layout-overflow` の
  警告(はみ出したブロックと高さ)を出すだけで、字を小さくしたり次のページへ
  送ったりはしない。

### コンテンツ(意味)

- 作者が書くのは意味の木だけ。座標・色・Box の組み方は書かない。見た目を
  変えたいときは、テンプレートが用意した **名前つきの変種**
  (例: `variant: dense`)を選ぶだけにする。
- **サブシステムのデータも意味役割名で書く**: Gantt の状態は
  `support`/`conditional`/`none`/`danger`/`risk`/`accent`/`on_conditional`
  のような役割名で表し、実際の色はテーマ経由でしか解決しない。コンテンツ構築側
  (`build_chart()` 相当)は theme 引数を一切持たない。
  (これが守れていないと「意味とデザインの分離ができていない」という、既存の
  類似 OSS `hirokisakabe/pom` と同じ欠陥になる。実際、Go 移植の最初のバージョンは
  この原則を破っていて、行ラベルのようなコンテンツの識別子を色のキーに
  使っていた。本物の Python ソースと突き合わせて修正済み。)
- **記法(決定)**: 作者の主な入口は厳格な Markdown(Boxloom Markdown。意味ブロックは
  `:::名前` の囲み、サブシステムは言語名つきのコードブロックに YAML)。知らない囲み・属性・
  生の HTML は誤りにし、`boxloom check` が場所つきの JSON で返す。IR の写し(YAML/JSON)を
  併存させ、Go DSL は開発者・試験用、TeX 風は採らない。仕様は `docs/content-syntax.md`、
  比較は `docs/content-syntax-comparison.md`。
- **意味の中間表現(IR)**:どの記法も JSON の IR に落とし、テンプレートは IR だけを
  見る。Schema は `schema/`(入口 `boxloom-ir.schema.json`)、設計は `docs/ir.md`、
  検査は `python3 tools/validate_ir.py`。
  できた PPTX の OOXML としての検査は `python3 tools/validate_pptx.py`(ECMA-376 の XSD と
  参照の整合)。CI の手順は `sh tools/ci.sh`(`.github/workflows/ci.yml` も同じ)。書いては
  いけない語の検査 `tools/check_terms.py` の一覧はリポジトリに書かず、環境変数・CI の秘密で
  渡す。詳細は `docs/build.md` の「PPTX の検査」「CI と検査の道具」。
- **AI に渡す一覧**:`go run ./cmd/boxloom schema [-template 名前]` が、テンプレートが名乗る囲み・
  属性・サブシステムの Schema と、IR の Schema(外を参照しない1つの文書)を1つの
  JSON で出す(`docs/schema-command.md`)。囲みは `Template.Blocks`、サブシステムは
  登録簿の `Declare` で名乗る。Go は `schema/` の写し `boxloom/schemas/` を埋め込む
  (`schema/` を直したら `go generate`)。

## アーキテクチャ: コアの二層構造(レイアウトとサブシステム)

1. **outer layout engine**: Row/Col/Text/Canvasの木を、2パスのFlexboxライクな
   アルゴリズムで配置する(`box_engine.py` / Go版`layout.go`)。
   - Phase1: 交差軸サイズをトップダウンで解決(STRETCH or CENTER=自然サイズ)
   - Phase2: 主軸方向のbasisをボトムアップで計測(テキストは実測、コンテナは
     子の合計)
   - Phase3: grow/shrinkを**一回だけ**比例配分(Taffyのflexible lengths反復凍結
     アルゴリズム(CSS Flexbox 9.7節)は未実装。min/max制約を使わない実際の
     コンテンツに対しては無害と数値検証済み — 最大差分0.000037pt)
   - Phase4: justify_content(START/SPACE_BETWEENのみ)
   - Phase5: 位置確定+再帰
   - サポート範囲はROW/COLUMNのみ、JustifyContent START/SPACE_BETWEENのみ、
     AlignItems STRETCH/CENTERのみ。flex-wrap・min/max制約は未使用(実際の
     コンテンツを機械的に洗い出した結果)。
   - Go 版だけの追加: サブシステムの葉は、basis を自己申告の自然な大きさから
     取る。足りないときは Phase3 の前に、サブシステムが「自然な大きさと最小の
     大きさの間のゆとり」を先に返す(Phase3a)。サブシステムを含まない木では
     何もしないので、Python 版との一致は崩れない。最小を下回ったら
     `[SUBSYSTEM WARNING]` を出す。
   - Go 版だけの追加(その2): サブシステムは上限(Max)も申告でき、grow は
     そこで止まる。列の中の入れ子の箱の高さは、中の文字の折り返しまで含めて
     測る(`heightForWidth`)。どちらも従来のデモの出力を変えない。
   - Go 版だけの追加(その3): テンプレートが印(`nestedSlack`)を付けた入れ子の箱
     (並列のまとまり)は、Phase3a で中のサブシステム(画像など)の縮められる分を
     先に返す。印の無い箱は従来どおり(既存の出力は変わらない)。
2. **inner canvas sub-engine**: サブシステムの葉(`Subsystem` interface。
   Python 版では grow=1, basis=0 で領域確保のみの `canvas`)の中に、Gantt専用の座標スケール(`TimeScale`: 日付→x座標の
   区分線形補間、区間圧縮に対応)でチャートを描画する(`render_gantt()` /
   Go版`gantt.go`)。outerのレイアウトエンジンを何度差し替えても
   (Taffy→自作Python→Go)、この層のロジックは一切変更していない。

この2層分離は、コアの「Box レイアウト+サブシステムの差し込み口」を
実装したもの。canvas内のロジックを外側のFlex計算から独立させることで、
エンジンの移植・差し替えがコンテンツとサブシステムに影響しない。

## 現在の実装状態

- **正(せい)となる参照実装**: `reference-v11-python/box_engine.py` +
  `reference-v11-python/layout_pptx_poc_v11.py`(python-pptx依存、Pillowで
  テキスト計測)。これが「本物」。
- **Go移植版**: `boxloom/`(標準ライブラリのみ、サードパーティ依存ゼロ、
  モジュール名`boxloom`)。`go list -m all` で自モジュールのみであることを
  確認済み。
  - **ライブラリとコマンドに分けてある**:`boxloom/` の直下が `package boxloom`
    (import path `boxloom`。埋め込みのデータと試験もここ)、コマンドは
    `boxloom/cmd/boxloom`(ライブラリの公開した入口だけを使う)。`boxloom/` から
    `go run ./cmd/boxloom <cmd>`。安定させる入口(`ParseMarkdown`・`ReadIR`・
    `ReadDeck`・`Check`・`Build`・`Schema`・`Templates`・`Issue` など)は
    `docs/library.md`。ライブラリはファイルを読み書きしない(バイト列と
    `io.Writer`)。別のモジュールから使う例は `boxloom/examples/libuse/`
    (`TestExternalModule` がネットワークなしでビルドして確かめる)
  - `layout.go` = outer engineの移植(box_engine.py相当)
  - `text.go` = テキスト計測(Pillowの代わりに、事前計測した「1emあたりの
    グリフ幅」を使う。字体ごとに `boxloom/fonts/<slug>.json`、
    `tools/export_font_metrics.py` で作る。どの字体で測るかはテンプレートの
    値の表の `fonts`、`fonts.go`)
  - `fontcheck.go` = `go run ./cmd/boxloom fonts`:字体が置き換わったときのはみ出しを測る
  - `gantt.go` = inner canvas sub-engineの移植(render_gantt()相当)。
    `GanttSubsystem` として `Subsystem` interface を満たす。値の表の `axisStyle`(`monthGrid` で
    年と月の2段の見出しと月の列)・`barShape`(`chevron` で山形の帯)・`fillHeight`(本文の高さを
    使い切る)で描き方を選ぶ(既定は従来。report は3つとも使う。`docs/subsystem.md` の11.1)
  - `shapes.go` / `md_shapes.go` = shapes サブシステム(生の図形への逃げ道)。
    図形・文字・線・矢印・コネクタ(PPTX では `a:stCxn`/`a:endCxn` で図形に繋がった
    本物のコネクタ)。check の誤り `shape-bounds`・`role-exists`・`target-exists`・
    `connector-self`、警告 `shapes-overlap`・`shapes-text-overflow`
  - `process.go` / `orgchart.go` / `matrix.go` / `funnel.go`(共通は `figures.go`、
    Markdown は `md_figures.go`)= 箱と線の図のサブシステム。作者は文・状態・強調だけを
    書き、置き場所・色はサブシステムが決める。線は箱に繋がった本物のコネクタ(組織図は
    曲がる線)。check の誤り `matrix-cells`・`matrix-emphasis`、警告 `figure-text-overflow`・`funnel-increase`・`figure-legend`。状態の凡例(legend)は4つの図すべて、funnel は段の間の転換率を描き、matrix は推す列(emphasis)を持つ。
    詳細は `docs/subsystem.md` の11節
  - `phases.go` = フェーズの図(phases)。段の山形(左端は homePlate、続きは chevron。先の深さは
    `ShapeOpts.Depth`)・段をまたぐ群の帯・段ごとに縦に積む番号つきの箱(題と補足。補足の列は
    段落の箇条)。番号は段をまたいで通しで、形は値の表 `subsystems.phases.numberStyle`(report は
    ローマ数字)。余りの高さは箱に配り、段の箱の列の下の端をそろえる。`docs/subsystem.md` の11節
  - `chart.go` / `chartxml.go` / `md_chart.go` = 数値のグラフ(chart)。比べ方から形を
    選び(item→横棒・timeSeries→縦棒か折れ線・component→円・frequency→ヒストグラム・
    correlation→散布図)、`c:chartSpace` と埋め込みの xlsx を書く。check の誤り
    `chart-values`・`chart-emphasis`、警告 `chart-labels`。詳細は `docs/subsystem.md` の12節
  - `typevocab.go` = 型の語彙(IR の type の名前・Schema・囲み・代わりの組み方。
    プロセスに1つ。テンプレートで変わらない。`docs/layers.md` の8.2)
  - `component.go` = 部品の登録簿(IR のブロック → 箱の木。組み方だけを持ち、
    登録できるのは語彙にある型だけ。テンプレートは写しを持って組み方を差し替えられる。
    組み方の無い標準でない型は代わりの組み方で組み `component-fallback` を警告。
    package の中だけ。`docs/layers.md` の7章)。出力が変わらないことは `TestOutputsGolden`
    (`testdata/outputs-golden.json`。出力を変える変更では `-update-golden` で書き直す)
  - `pairs.go` = 対の並び(`pairs`)。型の語彙に足した最初の標準でない型(Schema は
    `schema/types/pairs.schema.json`、Markdown は `::::pairs 見出し | 見出し` の中に
    `:::subject`・`:::response` を交互に。側は1つのブロックか、見出しと1〜4個のブロック `{title, body}`、Markdown は `:::subject 見出し`)。report は A・B・C の印・側の間の山形・対の間の罫線を足す。組み方は同梱のテンプレート(dark・light)が持ち、
    対ごとに2つの側の行をそろえる。はみ出しは対ごと(印 `measurePairRows`)。組み方を
    持たないテンプレートは `columns` で組んで `component-fallback`。`docs/layers.md` の8.1・8.3
  - `skeleton.go` = 骨組みの部品の表(`cover`・`contentHeader`・`contentFrame`・`section`・
    `statement`・`footer`・`source`・`toc`。標準は `defaultSkeleton`、テンプレートは写しを名前ごとに
    override)。コアは本文の箱の印(`frameBody`)で容量・はみ出しを、`title` のプレースホルダーの
    印の葉で題の1行を測る。`docs/layers.md` の8.4
  - `report.go` = report(読む資料のテンプレート)が自分で持つ組み方。骨組みの `contentHeader` を
    差し替え、リード文を上の見出し(24pt・基調色)、題(title のプレースホルダー)をその下(16pt)に
    置く。列の見出し(columns・pairs)は基調色。columns は見出しの下の罫線・列の間の縦の罫線と、`flow`(左が右につながる。Markdown は `{flow=true}`)のときの丸い矢印を描く(標準の組み方は flow のときだけ隙間に小さい矢印)。区切りは level 1・2 が frame(全面の基調色と白い枠)、3 が agenda(親の節の題と兄弟の項目・今の印)、目次は基調色の地に白い字。表は見出しの行が罫線・群のセルが濃い面、結論は丸い矢印、引用は吹き出しと人の印(部品 table・takeaway・quote を override。印は図形の葉 `calloutMark`)。値は読む資料のサンプルを測ったもの(本文 14pt。`docs/template-values.md` の report の節)
  - `toc.go` = 区切りの階層(section の `level` 1〜3。親の無い level は check の `section-level`)と
    目次(`meta.toc`)。目次のスライドは IR に書かず、組み立てで表紙の直後に足す(`deckPages`)。
    頁番号(目次・フッター)は頁の並びで、check の場所と容量は IR の番号のまま。`docs/ir.md` の9.1
  - `token.go` = 配置の段のトークン。部品と骨組みは Spec に数・色を書かず、値の表の名前・
    段の刻み(`space(n)`・`type(n)`)・役割名を書き、`BuildSlide` の中の `resolveSpec`(組み立ての
    直後・計測の前の1か所)で数と色に直す。トークンの無い数・色・段に無い刻み・知らない名前は
    誤り。標準の部品は名前だけ(`TestStandardComponentsWriteNamesOnly`)。`docs/layers.md` の2.1・8.3
  - `subsystem.go` = サブシステムの差し込み口(`Subsystem` interface・
    `SubsystemRegistry`・役割名の解決器 `Roles`・最小の大きさの検査)
  - `blocks.go` / `image.go` / `pptx_blocks.go` / `md_blocks.go` = 第1層のブロックの
    並列のまとまり(`columns`)・表(`table`、本物の `a:tbl`。行の群 `groups` は先頭の列の `rowSpan`・`vMerge`、セルの中の箇条 `{points: [...]}`(Markdown は `{points=列名}` と `;`)はセルの段落の `a:buChar`。長い表は作者が分け、続きの表に `continues`(Markdown は `{continues}`)を書く。`tablechain.go` がひと続きをつなぎ、各枚に見出しの行と「n/m」の印(`marks.tableContinued`)を付けて列の幅をそろえ、はみ出した表には check が分け方の案(容量の `split` と hint)を出す)・画像(`image`・`gallery`、
    本物の `p:pic`)。表と画像は Subsystem の約束で測って描く葉(登録簿には載せない)。
    画像はライブラリに渡す `fs.FS`(`BuildOptions.Assets`)から読む(`docs/library.md`)
  - `fit.go` = 図と枠の橋渡し。`FixedAspect` の図・Max を越える枠は図の大きさまで
    切り詰め、余りは `Spec.Place` の側に置く(拡大縮小はしない。`docs/subsystem.md` の9節)
  - `subsystem_test.go` = 差し込み口とレイアウトの試験(`go test ./...`)
  - `pptx.go` = python-pptxの代わりに、OOXML(zip+XML)を直接組み立てる自前
    ライター(`archive/zip`のみ使用。無料で成熟したGo製pptxライブラリが
    存在しないため — `unidoc/unioffice`は商用ライセンス)
  - `ir.go` / `template.go` / `gantt_ir.go` = IR(JSON)を読み、テンプレート
    (dark / light)で Box の木に組み、PPTX に書く道。`go run ./cmd/boxloom build
    ../examples/ir/demo-deck.json` → `out/demo-deck.pptx`。Gantt の注釈の
    置き場所はサブシステムが決める。詳細は `docs/build.md`
  - `check.go` = `go run ./cmd/boxloom check deck.json|deck.bx.md`。Schema で書けない IR の
    検査(id の一意性・target の指す先・日付の前後と範囲・variant)と組み立ての
    警告(はみ出しの `layout-overflow` は `overflow.go`)を出す(`docs/ir.md`)。
    `-format json` には本文の容量 `capacity`(`capacity.go`。はみ出しと同じ測り値
    から作る残りの高さ・行の数)も付く。誤りの形はパーサー・`validate_ir.py` と共通の
    `Issue`(`docs/content-syntax.md` の5章。IR を直接検査したときは行と列を省く)
  - `values.go` + `templates/*.json` + `subsystems/gantt.json` = 値の表(データ)の
    読み込みと厳しい検査(段階 `scale` から外れた余白・字の大きさも誤り)。
    テンプレートの一覧は `templates/` のファイルから作る(`docs/template-values.md`)
  - 題はスライドのレイアウトのプレースホルダー(表紙は `ctrTitle`・`subTitle` で
    「Title Slide」、本文は `title` で「Title Only」、題の無いスライドは「Blank」)。
    位置と字の大きさは図形の側に書いたままなので、見た目は変わらない(`docs/build.md`)
  - `markdown.go` / `md_gantt.go` / `yaml.go` = Boxloom Markdown → IR の
    パーサー(YAML の部分集合も自前)。`go run ./cmd/boxloom parse deck.bx.md`、`build` は
    `.md` もそのまま受け取る。詳細は `docs/content-syntax.md` の8章。
    発表者ノートは `:::notes`(`md_notes.go`、3.6)→ IR の `notes` → PPTX のノート。
    出典は `:::source`(`md_source.go`、3.6.1)→ IR のスライドの `source` → 本文の下・
    フッターの上の枠(`docs/ir.md` の設計判断11)
    リード文は `:::lead`(`md_lead.go`、3.6.0)→ IR のスライドの `lead` → 題の直下の枠。
    本文のスライドの題は短い見出しで1行(折り返すと `title-wrap` の警告)
  - `yaml_deck.go` = YAML の deck(IR の写し。同じキー・同じ type)→ IR。拡張子 `.yaml`・`.yml` で
    parse・check・build・`ReadDeck` が受け取る。誤りには YAML の行と列(IR の場所から引く)。
    例は `examples/yaml/charts.yaml`(`examples/ir/charts.json` と同じ PPTX。`docs/content-syntax.md` の1章)
  - `layoutdump.go` = `go run ./cmd/boxloom layout deck.(bx.md|yaml|json)`。配置の段の木(箱・
    IR の場所・印・値・決まった座標)を JSON で出す(出すだけ。形は `schema/layout.schema.json`、
    `docs/build.md` の「配置の段の木を見る」)。box が PPTX の図形と一致することは
    `TestLayoutBoxesMatchPPTX`。開発者・試験用に同じ木を読み直して組む口が `layoutread.go`
    (`boxloom layout-build`。トークンを値の表の段で検査。作者の入口ではない)
  - `vocab.go` / `schema_cmd.go` = `go run ./cmd/boxloom schema`。名乗る囲み・文書の頭の
    キー・記法の約束事の表と、それを1つの JSON に組むコマンド
  - `api.go` / `doc.go` = ライブラリの入口とパッケージの説明(`docs/library.md`)
  - `cmd/boxloom/mcp.go` = `boxloom mcp`。stdio の MCP サーバ(標準ライブラリだけ)。道具は
    `schema`・`check`(容量つき)・`build`。Claude Code への登録の手順は `docs/mcp.md`
  - `cmd/boxloom/main.go` = コマンド(フラグ・ファイルの読み書き・終了コード)
  - `demo.go` = デモコンテンツとfill-ratioリンター(本物の
    `content_fill_ratio()`/`lint_fill_ratio()`の移植。`WriteDemo`)。
    `go build ./cmd/boxloom` → `./boxloom`(引数なし)で `out/` 配下に
    `boxloom_dark.pptx`/`boxloom_light.pptx` を生成する。出力パスは相対パス(`out/`)にしてあるので、リポジトリの
    どこに置いても動く。

### 検証済み事項

- Python版 `box_engine.compute_layout()` とGo版 `computeLayout()` に**全く同じ
  Spec木**を通して座標を数値比較 → 主要座標は誤差0.01pt未満で一致。
  フッター文字列幅だけ0.84pt差があるが、原因はPillowの`get_font()`が
  フォントサイズをpx丸め(整数ピクセルでの実測、ヒンティング由来の誤差)する
  のに対し、Go版は1000px基準からの線形スケールという**計測方式の違い**であり、
  バグではないと特定済み。
- LibreOfficeでPDF変換→PNG化して目視確認。risk帯・breakマーク("//")・
  open-endの点列・注釈ボックスの動的重なり回避まで含めて、本物とほぼ同じ
  見た目になることを確認済み(`verification-screenshots/`参照。改名後の
  `boxloom` ビルドから再生成済み)。
- fill-ratioリンターは両テーマとも100.0% OK。

### 既知の軽微な相違(気にしなくてよいレベル)

- 本物の`wrap_text()`内の`if tok.isspace(): continue`は到達不能なデッドコード
  (条件分岐の構造上、呼ばれることがない)。Go版はこれを再現していないが、
  実害はない。
- 本物はGanttサブエンジン内の複数行テキスト(グループラベル等)をbox_engineの
  `wrap_text()`経由ではなく、PowerPoint/LibreOffice側のネイティブ折り返し
  (`word_wrap=True`)に任せている。Go版も流れる文字は段落ごとに1つの `a:p` にして
  `wrap="square"` で書き、折り返しはアプリに任せる(PowerPoint で編集すれば流れ直る)。
  Boxloom は同じ文を自分でも折り返して行の数を測り、高さを決める。強制の改行(`a:br`)は
  リード文などの文の切れ目と、統計強調の数の塊の境目だけ。箇条は1つのテキストボックスの
  段落(印は `a:buChar`、入れ子は `lvl`、項目の間は `a:spcBef`)。カード(compare・stat・
  takeaway の地)も同じ書き方で、地の図形そのものが文字を持つ1つの図形(余白は
  `lIns` 等。takeaway の帯だけ別の図形)。図の中の折り返さない
  名前だけが `wrap="none"`。
- python-pptxの`add_shape()`はテーマ由来の`<p:style>`参照(lnRef/fillRef/
  effectRef/fontRef)を自動付与するが(それを打ち消すために`shadow.inherit =
  False`を呼んでいる)、Go版の手書きXMLはそもそも`<p:style>`を持たない。
  net visualには影響しない(どちらも影なし)が、OOXML構造としては異なる。

## 次にやること(Claude Codeでの続き)

1. このディレクトリをgitリポジトリ化してコミットする(クラウドサンドボックス
   上で作業していたため、これまでコミット履歴がなく、実際に一度コンテナ
   リセットでv7〜v11のPoCを失っている)。リポジトリ名は `boxloom` を推奨。
2. 以降の実装作業(OOXMLの細部の仕上げ、Ganttサブエンジンの追加機能、
   テスト整備など)はこのリポジトリ内で、Claude Codeで継続する。
3. Go版がpython版の正式な後継になるか、両方並行保守するかは未決定。
   Python環境を用意しにくいPCでの配布容易性(単一バイナリ・依存ゼロ)
   という点ではGo版が有利、という評価は出ている。
4. OSS公開する場合のライセンスは未決定。リポジトリ名/パッケージ名は
   `boxloom` で確定(名前の衝突調査は`HANDOFF.md`参照 — Minecraft用の
   同名modが存在するが、ダウンロード数29件程度の無名modで、ドメインも
   完全に異なるため実害はないと判断)。
