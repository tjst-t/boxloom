# 引き継ぎメモ: 宣言的PPTX生成プロジェクトの経緯

Cowork(claude.ai)上での長い議論からClaude Codeへ作業を引き継ぐための
経緯まとめ。設計原則やアーキテクチャの「結論」は `CLAUDE.md` にまとめてある。
こちらは「なぜそうなったか」の経緯。

プロジェクト名は **Boxloom** に確定した(このzipは改名後の版)。経緯は
末尾の「プロジェクト名の決定経緯」を参照。

## 経緯(時系列)

1. **構想**: LaTeX/Beamer的にテンプレートとコンテンツを分離し、レイアウト
   エンジンで自動配置するPowerPoint生成システムを着想。手動座標ではなく
   実レイアウトエンジンを使う方針。OSS化する価値があると判断。
2. **PoC v1〜v7**: Taffy(Rust製レイアウトエンジン)のPythonバインディング
   `stretchable` + python-pptxで構築。仮想コンテナの実体化、テキスト
   オートフィット、fill-ratioリンター、フッター例外まで実装・検証。
3. **v8〜v9**: 構造の複雑なスライド(製品世代ごとのサポート期間の
   Ganttチャート、日付軸の区間圧縮、多数の吹き出し注釈)を題材に検証。
   (リポジトリのデモの題材は架空のデータ)
   Gantt/タイムライン系はFlex/Gridの範囲外で専用サブエンジンが必要と判断し、
   「outer flex layout + inner canvas sub-engine」の二層アーキテクチャに
   到達。
4. **v10**: 同一コンテンツに別カラースキームを適用できるか検証。Ganttの色を
   生のhexから意味役割名(role name)に変更し、`render_gantt()`に渡す
   theme辞書だけが実際の色を知っている設計に変更。ダークテーマとブルー基調
   基調のライトテーマで目視検証。
5. **似たOSSとの比較**: `hirokisakabe/pom`が近い設計に見えたが、ユーザーが
   「意味とデザインの分離が全然できてない」と指摘。実際にコード(content
   ファイルが生のfontSize/colorをハードコード)を確認し、指摘が正しいと
   認めた。
6. **.NET移植の検討**: Python環境を用意しにくいPCでの実行を見据えて
   .NET(ShapeCrawler/OpenXml SDK + MSAGL)を検討したが、作業環境の
   ネットワークポリシーで`nuget.org`がブロックされており検証不能と判明。
   方針転換し、「Pythonのまま、Taffy依存を自作エンジンに置き換えられるか」
   を先に検証することにした。
7. **box_engine.py(自作レイアウトエンジン)**: 実際に使われているスタイルの
   組み合わせ(ROW/COLUMN、START/SPACE_BETWEEN、STRETCH/CENTERのみ)を
   洗い出し、Taffy不要の2パスエンジンを実装。v10のSpec木をそのまま入力し、
   Taffyの計算結果と全ノードで数値比較 → 最大差分0.000037pt(浮動小数点
   誤差のみ)で一致を確認。
8. **v11**: Taffy/stretchableへの依存を実際に完全に削除。`build_chart()`/
   `render_gantt()`/`build_slide_spec()`の中身は一切変更せず、レイアウト
   計算部分だけ差し替え。stretchableを強制的にimport不能にしても動作する
   ことを確認。**これが現在の「正」となるPythonソース**
   (`reference-v11-python/`)。
9. **Go移植の検討**: Python環境を用意しにくいPCへの配布を見据え、Python以外の選択肢(.NET/Go/
   Python embeddable distribution)を比較。Goへの実移植を依頼された。
10. **Go移植 1st pass**: `golang.org/x/image`等のGoモジュール取得も同じ
    ネットワークポリシーでブロックされると判明(.NET/NuGetの時と同種の
    制限)。方針転換し、Pillowで事前計測した「1emあたりのグリフ幅」
    テーブルをGoソースに埋め込む設計にした(Go側はフォント解析ライブラリを
    一切持たない)。pptx自体も無料で成熟したGo製ライブラリが無い
    (`unidoc/unioffice`は商用ライセンス)ため、`archive/zip`のみでOOXMLを
    直接組み立てる自前ライターを実装。依存ゼロのGoバイナリが完成し、動作・
    目視確認もした。
11. **「Python版と出力が違いそう」という指摘**: ユーザーから本物のv11
    ソース(`box_engine.py`/`layout_pptx_poc_v11.py`)が提示され、実際に
    照合した結果、Go版の1st passには設計原則違反を含む複数の乖離が
    見つかった: 色がロール名でなくコンテンツ識別子キーだった、TimeScaleの
    区間圧縮を無視していた、risk_band/open_end/note/break_before/注釈の
    動的重なり回避/fill-ratioリンターが未実装、凡例と検討メモを
    render_gantt()の外に勝手に作っていた、footerがSPACE_BETWEEN原則を
    使っていなかった。
12. **Go移植 2nd pass**: 本物のソースを見ながら上記すべてを修正。
    Python側`box_engine.compute_layout()`とGo側`computeLayout()`に同一の
    Spec木を通す数値diffを新たに実施し、主要座標が誤差0.01pt未満で一致する
    ことを確認(詳細差異は`CLAUDE.md`の「既知の軽微な相違」参照)。
13. **Claude Codeへの移行判断**: 「そろそろ本格的にClaude Codeに移行しようか」
    という相談を受けた。ユーザーは既にClaude Codeを主力として使用している
    ため、論点は「導入するかどうか」ではなく「このプロジェクトのどの
    フェーズからClaude Code側に移すか」だった。結論: 設計判断(前半の議論)
    はチャット的なやり取りに向いていたが、v11パリティが取れて原則が固まった
    今は実装フェーズに入ったので、ここでgitリポジトリ化してClaude Codeに
    引き継ぐのが適切、という判断で最初のzip(作業名「pptxgo」版)を作成した。
    (このクラウドサンドボックスは使い捨てで、実際に一度コンテナリセットで
    v7〜v11のPoCファイルを失っている — gitに乗せていればこの種の消失は
    起きない、という教訓も引き継ぎの動機になっている)
14. **プロジェクト名の確定とリネーム**: 作業名「pptxgo」のままOSS公開するには
    直接的すぎる/汎用的すぎるとの判断から、正式名称の検討に入った(詳細は
    次節)。最終的に **Boxloom** に確定し、Goモジュール名・OOXMLメタデータ・
    デモの字幕/フッター文字列・ドキュメント・出力ファイル名をすべて
    `pptxgo` → `boxloom` にリネームし、ビルド・実行・fill-ratioチェック
    (両テーマ100% OK)を再確認した上でこのzip(2版目)を作成した。

## プロジェクト名の決定経緯

作業名「pptxgo」を正式名称にするかどうかの検討から始まり、複数の方向性を
試して絞り込んだ。各候補は競合調査(GitHub/pkg.go.dev/npm・PyPI/一般web)を
サブエージェントで都度確認してから次に進めた。

- **TeX系統の名前**: 「TeXのようにPPTXを作る」というイメージを込めて
  `PPTeX` を検討したが、本物のTeXと混同されるリスクが高いと判断して却下。
  `BoxTeX`/`GlueBox`/`PresenTeX` 等の派生も同系統のリスクを引きずるため
  見送り。
- **宣言的生成を短縮した名前**: 「宣言的なPPTX生成」を短縮した `pptx-gen` は
  既存の著名なJSライブラリ `PptxGenJS` と衝突リスクがあるため却下。
  `Declx` は一度採用候補になったが、将来的にdocx等PPTX以外のフォーマットも
  対象にする可能性を踏まえ、`declp`/`declppt`/`dppt`/`pptxd` 等の略称系統を
  検討した末、この方向全体を一旦保留にして再考した。
- **構築・工芸メタファー系統**: `Slidewright`(軽微な衝突あり)、`Formwork`
  (既存のCMS `getformwork`/`Formwork` と衝突するため却下)、`Boxloom`
  の3候補に絞り込んだ。
- **Boxloom の衝突調査**: 当初の調査では「ソフトウェアとしての衝突なし」と
  報告したが、ユーザーから「boxloomはマイクラモッドがあるな」と指摘を受け、
  再調査したところ根拠が弱く不確定だった。その後ユーザーから直接URL
  (`https://modrinth.com/mod/boxloom`)が提示され、実在を確認: Fabric向け
  Minecraft mod、ダウンロード数29件、フォロワー0、最終更新は数か月前という
  内容。ドメインが全く異なり知名度も無視できるレベルであるため、リスクは
  低いと評価し、採用に支障なしと判断した。
- **確定**: ユーザーが「boxloomで確定します」と最終決定。以後、
  リポジトリ名・Goモジュール名・パッケージ名すべて `boxloom` に統一する。

## このzipの中身

- `CLAUDE.md` — 設計原則・アーキテクチャ・現状・次のタスクの要約(Claude
  Codeの起動時コンテキストとして想定)
- `HANDOFF.md` — このファイル。経緯の全文。
- `boxloom/` — Go移植版一式(ビルド・実行可能。`go build .` → `./boxloom`で
  `out/`配下に2テーマ分のpptxを生成)
- `reference-v11-python/` — 「正」となる本物のPythonソース。python-pptx +
  Pillowが必要(`pip install python-pptx pillow`)。`python3
  layout_pptx_poc_v11.py`で実行可能
- `verification-screenshots/` — Go版・Python版それぞれをLibreOfficeで
  PDF変換→PNG化した比較用スクリーンショット(Go版はリネーム後の
  `boxloom` ビルドから再生成済み)

## 未決事項

- Go版をPython版の正式な後継にするか、両方並行保守するかは未決定
- OSS公開時のライセンスは未決定(リポジトリ名/パッケージ名は`boxloom`で確定)
- (解決済み)字体の寸法は `tools/export_font_metrics.py` が字体ごとに作り、
  ASCII・仮名・記号・全角形などを範囲で測るようになった(デモの文字に
  合わせた手動の一覧は無くなった。`docs/fonts.md`)
