# IR から PPTX を作る(boxloom build)

IR(`docs/ir.md`)か Boxloom Markdown(`docs/content-syntax.md`。拡張子 `.md`)を読んで、テンプレートで Box の木に組み、PPTX に書き出す道。
Go 版(`boxloom/`)だけにある。

```sh
cd boxloom
go run . build ../examples/ir/demo-deck.json                  # → out/demo-deck.pptx
go run . build ../examples/md/demo-deck.bx.md                 # Markdown からも同じものができる
go run . build -template dark -o out/x.pptx ../examples/ir/gantt-full.json
go run .                                                      # 従来の Go で組んだデモ(変わらない)
```

- テンプレートは `-template` > IR の `template` > `light` の順で決まる。
- 誤りは IR の中の場所を添えて止まる(例 `slides[2]: body[0]: gantt: ticks[0].status:
  unknown status "text_primary"`)。`examples/ir/invalid/` の9件は全部ここで断られる。
- 警告(本文の埋まり具合・本文のはみ出し・サブシステムの最小の大きさ・サブシステム自身の検査・テーマに無い役割名)は
  標準出力にだけ書く。スライドのノートには入れない(ノートは作者の文だけ)。
- 題はスライドのレイアウトのプレースホルダーとして書く。表紙の題と副題は
  `ctrTitle`・`subTitle`(レイアウト「Title Slide」)、本文のスライドの題は `title`
  (レイアウト「Title Only」)。題を持たないスライド(従来のデモ)は「Blank」。
  位置・字の大きさ・色は図形の側に書いたままで、見た目はテキストボックスのときと
  同じ(画像で確かめた)。PowerPoint のアウトライン表示・読み上げ・レイアウトの
  差し替えが題として扱う。LibreOffice は `presentation:class="title"` として読む。
  作者の名前・本文・フッターはプレースホルダーにしない(ただの図形)。
- IR の `notes`(発表者ノート)は、改行ごとに1段落でそのスライドのノートに入る。
  ノートのページにはスライドの縮図と文の置き場を置く(LibreOffice のノートの
  ページでも見える)。
- 字体が置き換わったときに文字がはみ出すかは `go run . fonts deck.json`
  (`docs/fonts.md`)。`go run . fonts -list` で寸法を持つ字体の一覧。
- 書き出さずに検査だけするなら `go run . check deck.json`(`.bx.md` も可。誤りと
  警告を `docs/content-syntax.md` の5章の形で出す。`-format json` で JSON。
  `docs/ir.md` の「意味の検査」)。JSON には本文の容量(残りの高さ・行の数)も
  付く(`-capacity` で text でも出る。`docs/ir.md` の「容量の計測」)。

## 流れ

```
IR (JSON) ──ReadIR──▶ IRDeck ──Template.BuildDeck──▶ Spec の木(1枚ずつ)
   ──computeLayout──▶ 座標 ──renderSpec / Subsystem.Render──▶ PPTXBuilder ──SaveDeck──▶ .pptx
```

| ファイル | 役目 |
|---|---|
| `ir.go` | IR を読む。知らないキーは断る(Schema と同じ方針)。インライン・行の配列・箇条の入れ子を Go の型にする |
| `template.go` | テンプレート(骨組み・部品。値の表は `templates/<名前>.json` を `values.go` が読む。`docs/template-values.md`)。表紙と本文のスライド、ブロック(段落・箇条・比較・注記・サブシステム)を Spec に組む |
| `gantt_ir.go` | Gantt の `data` から `GanttChart` を組む(軸の圧縮・目盛りの文字・注釈の指す先) |
| `gantt.go` | 注釈の置き場所と箱の大きさを決める(`planAnnotations`)。置けない注釈は警告(`Lint`)。確定していない区間を破線で描く |
| `overflow.go` | 本文のはみ出しの検査(`layout-overflow`)。縮めなければ要る高さを測り直し、本文の領域と比べる |
| `pptx.go` | 複数枚の書き出し(`SaveDeck`)と、装飾つきの文字(`AddRichTextBox`) |

## テンプレートが決めていること(dark / light)

- **骨組み**:表紙は題・アクセントの線・副題・作者を左に寄せて置く。本文のスライドは
  題 / 本文 / フッター(作者と「何枚目 / 全部」)。
- **意味から見た目へ**:`em` はアクセントの色の太字。`link` はアクセントの色(押せる
  リンクにはまだしない)。`code` は控えめな色(等幅の字体にはまだしない)。`br` は改行。
- **部品**:箇条は「・」の印(入れ子は字下げして「–」)。比較は選択肢ごとに同じ幅の
  カードを横に並べ、良い点を「＋」(support の色)、悪い点を「－」(danger の色)で示す。
  注記は小さく控えめな色。サブシステムはカードの上に置き、自己申告の上限まで伸ばす。
- **方針**:本文が少なくても引き伸ばして埋めない。本文の箱のうち `fillRatioMin`(dark / light は 65%)より少なくしか
  使っていなければ `[FILL-RATIO WARNING] underfilled` を出す(例のデッキの
  2・3枚目はこれが出る)。
- **変種(variant)**:デモのテンプレートは1つも持たないので、`variant` を書くと断る。

## コアに足したもの

- **Measure の上限(Max)**:grow で配るとき、サブシステムが申告した上限で止める
  (`docs/subsystem.md`)。
- **折り返しを含む高さ**:列の中に置いた入れ子の箱(行など)の高さを、中の文字の
  折り返しまで含めて測る(`heightForWidth`)。いままでは文字を1行と見なしていたので、
  比較のカードのように横に並べた文字が折り返すと高さが足りなかった。
- どちらも、従来の Go で組んだデモ(`go run .`)の出力を変えない(展開して `diff -r` で
  同じことを確かめた)。

## PPTX の検査(tools/validate_pptx.py)

PowerPoint が「修復が必要です」と言う種類の誤りを、人が開く前に機械で捕まえる。
依存ゼロの Go の本体には入れず、tools/ の Python の道具にした。

```
python3 tools/fetch_ooxml_schemas.py          # 初回だけ。XSD を tools/.ooxml-schemas/ に取ってくる
python3 tools/validate_pptx.py out/deck.pptx  # 検査。誤りは5章の形(`--format json` も)
python3 tools/validate_pptx.py --self-test    # examples を dark/light で作って全部通り、壊したものを断るか
```

- **XSD**:ECMA-376 の公式の Schema を使う。PowerPoint が書く名前空間
  (`schemas.openxmlformats.org`)は Transitional なので Part 4(5th edition)の
  Transitional の XSD、パッケージ(`[Content_Types].xml`・`.rels`)は Part 2 の OPC の
  XSD。リポジトリには入れず、`fetch_ooxml_schemas.py` が sha256 を確かめて取ってくる
  (`.gitignore` 済み)。lxml でそのまま読める。`docProps/core.xml` だけは Dublin Core の
  Schema が要るので、XSD ではなく形を手で見る。
- **XSD のほかに見ること**:内容の種類の抜け・余り、関係の指す部品の有無と種類、
  部品の中の `r:id` が関係にあるか、スライドにレイアウトが1つあるかなど要る関係、
  1つの部品の中の図形の id の重複、コネクタの `a:stCxn`・`a:endCxn` の指す先、
  `sldId`・`sldMasterId`・`sldLayoutId` の重複と範囲。code の一覧は道具の頭の説明。
- **別の実装で読めるか**:`--python-pptx` で python-pptx で開き直して全部の文字を読み、
  `--soffice` で LibreOffice で PDF に変換してページの数がスライドの数と合うかを見る。
  `--self-test` は python-pptx があれば必ず使う。
- **自分の試験**:examples の IR・Markdown(警告の例も含む)を dark/light で作った22個と、
  従来のデモ2枚が全部通ること。それを壊した15通り(XML の誤り・Schema 外の要素と値・
  部品の削除・内容の種類の抜けと余り・関係の無い r:id・図形の id の重複・コネクタの
  指す先・スライドの id の重複・レイアウトの無いスライド・種類違いの関係・
  `[Content_Types].xml` の削除・ノートの雛形の抜け)が、それぞれ決めた code で断られること。
- **限り**:XSD と参照の整合は見るが、PowerPoint 自身が開けるかは見ていない
  (PowerPoint だけが嫌う形は捕まらない)。

## CI と検査の道具

手順は `tools/ci.sh` に1つにまとめ、GitHub Actions(`.github/workflows/ci.yml`)も
ローカルもこれを流す。

```
sh tools/ci.sh            # go python pptx terms
sh tools/ci.sh readers    # LibreOffice で読めるか(重いので既定では流さない。CI では別の job)
```

| 手順 | 中身 |
|---|---|
| `go` | `gofmt -l` が空・`go vet`・`go test` |
| `python` | `validate_ir.py --self-test` と、`boxloom schema` の出力での `--bundle --self-test` |
| `pptx` | `validate_pptx.py --self-test` |
| `readers` | `validate_pptx.py --self-test --soffice` |
| `terms` | `check_terms.py --history`:書いてはいけない語(業務に関わる固有名など) |

**書いてはいけない語**:一覧そのものはリポジトリに書かない。環境変数
`BOXLOOM_FORBIDDEN_TERMS`(1行1語)で渡し、CI では GitHub の秘密
`BOXLOOM_FORBIDDEN_TERMS` から渡す。英数字だけの語は単語の境目で、ほかは部分一致で
照らす。git が追う全部の文字のファイル、PPTX の中の XML、`--history` で全部の
コミットの文と差分、履歴にある全部の版の PPTX の中まで見る。PNG などの画像の中は
見ない(圧縮したバイト列が偶然当たるため。実際に以前の PNG で起きた)。当たった語は
ログに出さず、一覧の何番目かで示す。push のときは一覧が空なら落とし
(`BOXLOOM_REQUIRE_TERMS=1`)、外からの pull request(秘密が渡らない)では飛ばす。

**入れる道具**(ローカルで流すとき):

```
# Go 1.24(PATH に go)
pip install "jsonschema>=4.18" lxml python-pptx
sudo apt-get install -y libreoffice-impress fonts-noto-cjk   # readers のときだけ
```

**まだ確かめていないこと**:リモートのリポジトリが無いので、workflow が GitHub の上で
実際に緑になるかは確かめていない。ローカルで `sh tools/ci.sh go python pptx terms` と
`readers` が通ることだけ確かめた。

## まだやっていないこと

- 押せるリンク、`code` の等幅の字体。
- 本文の高さがページに収まらないときの自動の扱い(いまは比例で縮むだけ)。
  はみ出しは `layout-overflow` の警告で知らせ、直すのは作者(`docs/ir.md` の
  「はみ出しの測り方」)。
