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

## 確定している設計原則(議論の末に合意済み。覆さない)

- **中央揃え禁止**: `align_items=CENTER` はデザイン原則(ノンデザイナーズ・デザイン
  ブック的な「端をそろえる」原則)に反するため使わない。
- **SPACE_BETWEENはフッター限定**: `justify_content=SPACE_BETWEEN` のような
  主軸方向の余白配分は、フッターなど明示的に宣言された単機能フレーム要素の
  内部でのみ許可。コンテンツ領域の余白消費には使わない。無宣言の大きな余白は
  fill-ratioの数値に関わらず常にNG。
- **コンテンツ不足を自動で埋めない**: 警告を出すだけで、他ページからの流用などは
  しない。
- **コンテナは基本「仮想」**: 背景色や枠線などスタイルを持たない限り、実際の
  シェイプとしては実体化しない。
- **色は意味役割名で分離する**: Ganttチャートの色は `support`/`conditional`/
  `none`/`danger`/`risk`/`accent`/`on_conditional` のような意味役割名の文字列で
  表現し、実際の16進色はtheme(dict/map)経由でしか解決しない。コンテンツ構築側
  (`build_chart()`相当)はtheme引数を一切持たない。
  (これが守れていないと「意味とデザインの分離ができていない」という、既存の
  類似OSS `hirokisakabe/pom` と同じ欠陥になる。実際、Go移植の最初のバージョンは
  この原則を破っていた — 行ラベルのようなコンテンツ識別子を色キーに使っていた。
  本物のPythonソースと突き合わせて修正済み。)

## アーキテクチャ: 二層構造

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
2. **inner canvas sub-engine**: `canvas`種別のleaf(outer側はgrow=1, basis=0で
   領域確保のみ)の中に、Gantt専用の座標スケール(`TimeScale`: 日付→x座標の
   区分線形補間、区間圧縮に対応)でチャートを描画する(`render_gantt()` /
   Go版`gantt.go`)。outerのレイアウトエンジンを何度差し替えても
   (Taffy→自作Python→Go)、この層のロジックは一切変更していない。

この2層分離こそがこのプロジェクトの核心。canvas内のロジックを外側のFlex計算から
独立させることで、エンジンの移植・差し替えがコンテンツに影響しない。

## 現在の実装状態

- **正(せい)となる参照実装**: `reference-v11-python/box_engine.py` +
  `reference-v11-python/layout_pptx_poc_v11.py`(python-pptx依存、Pillowで
  テキスト計測)。これが「本物」。
- **Go移植版**: `boxloom/`(標準ライブラリのみ、サードパーティ依存ゼロ、
  モジュール名`boxloom`)。`go list -m all` で自モジュールのみであることを
  確認済み。
  - `layout.go` = outer engineの移植(box_engine.py相当)
  - `text.go` = テキスト計測(Pillowの代わりに、事前計測した「1emあたりの
    グリフ幅」テーブル`metrics_gen.go`を使う。`export_font_metrics.py`で
    生成)
  - `gantt.go` = inner canvas sub-engineの移植(render_gantt()相当)
  - `pptx.go` = python-pptxの代わりに、OOXML(zip+XML)を直接組み立てる自前
    ライター(`archive/zip`のみ使用。無料で成熟したGo製pptxライブラリが
    存在しないため — `unidoc/unioffice`は商用ライセンス)
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
