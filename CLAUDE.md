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
   16進の色を知っているのはここだけ。
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

**データとコードの混ぜ方(合意済み)**: 値の表・スタイル規則・骨組みはデータ
(宣言ファイル)で書き、部品とサブシステムはコードで書く。いまデータに
なっているのは値の表(色・大きさ・印の文字・字体、Gantt の寸法)で、
`boxloom/templates/<名前>.json` と `boxloom/subsystems/gantt.json`・`shapes.json`(同梱。
知らないキー・欠けたキーは誤り)。骨組みはまだコード。詳細は
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
  他ページからの流用などはしない。
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
- **AI に渡す一覧**:`go run . schema [-template 名前]` が、テンプレートが名乗る囲み・
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
  - `layout.go` = outer engineの移植(box_engine.py相当)
  - `text.go` = テキスト計測(Pillowの代わりに、事前計測した「1emあたりの
    グリフ幅」を使う。字体ごとに `boxloom/fonts/<slug>.json`、
    `tools/export_font_metrics.py` で作る。どの字体で測るかはテンプレートの
    値の表の `fonts`、`fonts.go`)
  - `fontcheck.go` = `go run . fonts`:字体が置き換わったときのはみ出しを測る
  - `gantt.go` = inner canvas sub-engineの移植(render_gantt()相当)。
    `GanttSubsystem` として `Subsystem` interface を満たす
  - `shapes.go` / `md_shapes.go` = shapes サブシステム(生の図形への逃げ道)。
    図形・文字・線・矢印・コネクタ(PPTX では `a:stCxn`/`a:endCxn` で図形に繋がった
    本物のコネクタ)。check の誤り `shape-bounds`・`role-exists`・`target-exists`・
    `connector-self`、警告 `shapes-overlap`・`shapes-text-overflow`
  - `subsystem.go` = サブシステムの差し込み口(`Subsystem` interface・
    `SubsystemRegistry`・役割名の解決器 `Roles`・最小の大きさの検査)
  - `fit.go` = 図と枠の橋渡し。`FixedAspect` の図・Max を越える枠は図の大きさまで
    切り詰め、余りは `Spec.Place` の側に置く(拡大縮小はしない。`docs/subsystem.md` の9節)
  - `subsystem_test.go` = 差し込み口とレイアウトの試験(`go test ./...`)
  - `pptx.go` = python-pptxの代わりに、OOXML(zip+XML)を直接組み立てる自前
    ライター(`archive/zip`のみ使用。無料で成熟したGo製pptxライブラリが
    存在しないため — `unidoc/unioffice`は商用ライセンス)
  - `ir.go` / `template.go` / `gantt_ir.go` = IR(JSON)を読み、テンプレート
    (dark / light)で Box の木に組み、PPTX に書く道。`go run . build
    ../examples/ir/demo-deck.json` → `out/demo-deck.pptx`。Gantt の注釈の
    置き場所はサブシステムが決める。詳細は `docs/build.md`
  - `check.go` = `go run . check deck.json|deck.bx.md`。Schema で書けない IR の
    検査(id の一意性・target の指す先・日付の前後と範囲・variant)と組み立ての
    警告(はみ出しの `layout-overflow` は `overflow.go`)を出す(`docs/ir.md`)。
    `-format json` には本文の容量 `capacity`(`capacity.go`。はみ出しと同じ測り値
    から作る残りの高さ・行の数)も付く。誤りの形はパーサー・`validate_ir.py` と共通の
    `Issue`(`docs/content-syntax.md` の5章。IR を直接検査したときは行と列を省く)
  - `values.go` + `templates/*.json` + `subsystems/gantt.json` = 値の表(データ)の
    読み込みと厳しい検査。テンプレートの一覧は `templates/` のファイルから作る
    (`docs/template-values.md`)
  - `markdown.go` / `md_gantt.go` / `yaml.go` = Boxloom Markdown → IR の
    パーサー(YAML の部分集合も自前)。`go run . parse deck.bx.md`、`build` は
    `.md` もそのまま受け取る。詳細は `docs/content-syntax.md` の8章。
    発表者ノートは `:::notes`(`md_notes.go`、3.6)→ IR の `notes` → PPTX のノート
  - `vocab.go` / `schema_cmd.go` = `go run . schema`。名乗る囲み・文書の頭の
    キー・記法の約束事の表と、それを1つの JSON に組むコマンド
  - `main.go` = デモコンテンツとfill-ratioリンター(本物の
    `content_fill_ratio()`/`lint_fill_ratio()`の移植)。`go build .` →
    `./boxloom` で `out/` 配下に `boxloom_dark.pptx`/`boxloom_light.pptx`
    を生成する。出力パスは相対パス(`out/`)にしてあるので、リポジトリの
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
  (`word_wrap=True`)に任せている。Go版は自前OOXMLライターが全テキストを
  事前折り返しして`wrap="none"`で出力する設計(レンダラ非依存にするため)。
  意図的な設計差であり、結果は視覚的に同等。
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
