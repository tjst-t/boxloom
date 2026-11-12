# IR から PPTX を作る(boxloom build)

IR(`docs/ir.md`。JSON か、その写しの YAML `.yaml`・`.yml`)か Boxloom Markdown(`docs/content-syntax.md`。拡張子 `.md`)を読んで、テンプレートで Box の木に組み、PPTX に書き出す道。
Go 版(`boxloom/`)だけにある。コマンドは `boxloom/cmd/boxloom`、同じことを Go の
プログラムから呼ぶなら `docs/library.md`(`ReadDeck` → `Build`)。

```sh
cd boxloom
go run ./cmd/boxloom build ../examples/ir/demo-deck.json                  # → out/demo-deck.pptx
go run ./cmd/boxloom build ../examples/md/demo-deck.bx.md                 # Markdown からも同じものができる
go run ./cmd/boxloom build ../examples/yaml/charts.yaml                    # YAML の deck(IR の写し)。charts.json と同じ PPTX
go run ./cmd/boxloom build -template dark -o out/x.pptx ../examples/ir/gantt-full.json
go run ./cmd/boxloom build -template dark -o out/text_patterns_dark.pptx ../examples/md/text-patterns.bx.md   # 文字のパターン(light も同じく)
go run ./cmd/boxloom                                                      # 従来の Go で組んだデモ(変わらない)
```

- テンプレートは `-template` > IR の `template` > `light` の順で決まる。`-template` には
  外の値の表のファイル(`./my.json`)も渡せる(`docs/template-values.md` の「外の値の表を使う」)。
- 誤りは IR の中の場所を添えて止まる(例 `slides[2]: body[0]: gantt: ticks[0].status:
  unknown status "text_primary"`)。`examples/ir/invalid/` の24件は全部ここで断られる。
- 画像(`image`・`gallery`)は、文書のファイルのフォルダから `src` の相対パスで読む
  (ライブラリには `BuildOptions.Assets` で渡す。`docs/library.md` の「画像」)。PPTX では
  本物の画像(`p:pic`、`ppt/media/`)になり、代わりの文は画像の `descr` に入る。
- 表(`table`)は本物の表(`p:graphicFrame` の `a:tbl`)になり、PowerPoint・LibreOffice で
  表として編集できる。セルの文は折り返さずに書き、折り返しはアプリに任せる(測った幅には
  字体の余裕が入っている)。
- 警告(本文の埋まり具合・本文のはみ出し・サブシステムの最小の大きさ・サブシステム自身の検査・テーマに無い役割名)は
  標準出力にだけ書く。スライドのノートには入れない(ノートは作者の文だけ)。
- 題はスライドのレイアウトのプレースホルダーとして書く。表紙の題と副題は
  `ctrTitle`・`subTitle`(レイアウト「Title Slide」)、本文のスライドの題は `title`
  (レイアウト「Title Only」)。区切りの章の題と主張の1文も `title`(「Title Only」)。
  レイアウトを足す(「Section Header」など)と、全部の PPTX の雛形が変わって
  既存の出力が変わるので、いまある3つで済ませた。題を持たないスライド(従来のデモ)は「Blank」。
  位置・字の大きさ・色は図形の側に書いたままで、見た目はテキストボックスのときと
  同じ(画像で確かめた)。PowerPoint のアウトライン表示・読み上げ・レイアウトの
  差し替えが題として扱う。LibreOffice は `presentation:class="title"` として読む。
  作者の名前(表紙)・本文はプレースホルダーにしない(ただの図形)。
- 本文と主張のスライドのフッターは、マスターとレイアウト「Title Only」にあるフッターの
  置き場に結ぶ。右の番号はスライドの番号の置き場(`sldNum`)の中の番号の欄
  (`<a:fld type="slidenum">`)なので、PowerPoint・LibreOffice でスライドを足す・消す・
  並べ替えると振り直る(1枚消した PPTX で番号が詰まるのを確かめた)。PowerPoint には
  全部の枚数の欄が無いので、後ろの「/ 12」は組み立てたときの数の文字のまま。左の
  作者と機密区分はフッターの置き場(`ftr`。両方あれば作者の側)。位置と字の大きさ・色は
  図形の側に書いたまま。置き場の字体はテーマの本文の字体(LibreOffice は置き場に
  既定の字体を当てるので名指す)で、LibreOffice の描画ではただの図形のとき
  (Liberation Sans に落ちる)と英数字の字形が少し違う。表紙・区切りは番号を出さない。
- スライドの地(骨組みの根の `WithBackground`)は全面の四角を描かず、スライドの背景にする。
  マスターの地(`bg`)と同じ色なら何も書かずマスターの `p:bg` を引き継ぎ、違う色(区切りの
  `card_alt`)ならそのスライドの `p:cSld` に `p:bg` を書く(`renderSlide`・
  `SetSlideBackground`)。PowerPoint で背景を押しても図形が選ばれず、背景の書式から
  変えられる。根の外の地(カード・帯)はいままでどおり四角。見た目は変わらない
  (画像で確かめた)。
- IR の `notes`(発表者ノート)は、改行ごとに1段落でそのスライドのノートに入る。
  ノートのページにはスライドの縮図と文の置き場を置く(LibreOffice のノートの
  ページでも見える)。
- 字体が置き換わったときに文字がはみ出すかは `go run ./cmd/boxloom fonts deck.json`
  (`docs/fonts.md`)。`go run ./cmd/boxloom fonts -list` で寸法を持つ字体の一覧。
- YAML の deck(`.yaml`・`.yml`)は JSON の IR と同じキーで書いた写し(`docs/content-syntax.md` の
  1章)。読んで JSON の IR にしてから組むので、同じ内容の JSON と PPTX の部品がバイト単位で
  同じになる(`TestYAMLExampleSameAsJSON`・`TestYAMLMirrorsAllIRExamples`)。誤りには YAML の
  行と列が付く。
- 書き出さずに検査だけするなら `go run ./cmd/boxloom check deck.json`(`.bx.md`・`.yaml` も可。誤りと
  警告を `docs/content-syntax.md` の5章の形で出す。`-format json` で JSON。
  `docs/ir.md` の「意味の検査」)。JSON には本文の容量(残りの高さ・行の数)も
  付く(`-capacity` で text でも出る。`docs/ir.md` の「容量の計測」)。

## 流れ

```
IR (JSON) ──ReadIR──▶ IRDeck ──Template.BuildDeck──▶ Spec の木(1枚ずつ)
   ──computeLayout──▶ 座標 ──renderSpec / Subsystem.Render──▶ PPTXBuilder ──WriteDeck──▶ .pptx
```

| ファイル | 役目 |
|---|---|
| `ir.go` | IR を読む。知らないキーは断る(Schema と同じ方針)。インライン・行の配列・箇条の入れ子を Go の型にする |
| `template.go` | テンプレート(骨組み・部品。値の表は `templates/<名前>.json` を `values.go` が読む。`docs/template-values.md`)。表紙・本文・区切り・主張のスライド、ブロック(段落・箇条・比較・注記・結論・統計強調・引用・サブシステム)を Spec に組む |
| `gantt_ir.go` | Gantt の `data` から `GanttChart` を組む(軸の圧縮・目盛りの文字・注釈の指す先) |
| `gantt.go` | 注釈の置き場所と箱の大きさを決める(`planAnnotations`)。置けない注釈は警告(`Lint`)。確定していない区間を破線で描く |
| `overflow.go` | 本文のはみ出しの検査(`layout-overflow`)。縮めなければ要る高さを測り直し、本文の領域と比べる |
| `pptx.go` | 複数枚の書き出し(`WriteDeck`。io.Writer に書く)と、装飾つきの文字(`AddRichTextBox`) |

## テンプレートが決めていること(dark / light)

- **骨組み**:表紙は題・アクセントの線・副題・作者を左に寄せて置く。本文のスライドは
  題 / 本文 / フッター(作者と「何枚目 / 全部」)。区切り(section)は背景を一段違う色
  (`card_alt`)にし、アクセントの線と大きな太字の章の題を左に寄せ、中ほどより少し下
  (上の余白 3 : 下の余白 2)に置く。フッターは無い(表紙と同じく流れの外のページ)。
  主張(statement)は本文のスライドと同じ背景とフッターで、アクセントの線と大きな太字の
  1文を、外周よりさらに内(`statementInset`)に左に寄せ、中ほどより少し上(2 : 3)に置く。
  どちらも中央揃えは使わない(上下の余白の比で置く。表紙と同じ)。
- **意味から見た目へ**:`em` はアクセントの色の太字。`link` はアクセントの色(押せる
  リンクにはまだしない)。`code` は控えめな色(等幅の字体にはまだしない)。`br` は改行。
- **部品**:箇条は「・」の印(入れ子は字下げして「–」)。比較は選択肢ごとに同じ幅の
  カードを横に並べ、良い点を「＋」(support の色)、悪い点を「－」(danger の色)で示す。
  注記は小さく控えめな色。サブシステムはカードの上に置き、自己申告の上限まで伸ばす。
  結論(takeaway)は左にアクセントの帯、カードの色の上に一段大きな太字。統計強調(stat)は
  数の数にかかわらず同じ幅のカードを1行に並べ(1個なら本文の幅いっぱい)、カードの中に
  大きなアクセントの色の数と小さな単位(1つの文字の図形の2つの塊。下の線はアプリが
  そろえる)、その下に控えめな色の説明。引用(quote)は頭に大きなアクセントの色の引用の印、
  右に言葉、下に「— 出どころ」を控えめな色で。
  スライドの出典(source)は本文の下・フッターの上に「出典:」を頭に小さく控えめな色で。
  表の合計の行(total)は太字で、上に濃い線。日付・機密区分・免責は表紙の下、機密区分は
  フッターの作者の右にも(`docs/template-values.md` の marks の節)。
- **方針**:本文が少なくても引き伸ばして埋めない。本文の箱のうち `fillRatioMin`(dark / light は 65%)より少なくしか
  使っていなければ `[FILL-RATIO WARNING] underfilled` を出す(例のデッキの
  2・3枚目はこれが出る)。本文を持たない表紙・区切り・主張には出さない(区切り・主張は
  ページに収まるかだけを `layout-overflow` で見る)。
- 文字のパターン(区切り・主張・結論・統計強調・引用)の見た目は
  `verification-screenshots/text_patterns_light.png`・`text_patterns_dark.png`(LibreOffice で
  `out/text_patterns_*.pptx` を描いたもの。左上から右へ1枚ずつ)。
- **変種(variant)**:デモのテンプレートは1つも持たないので、`variant` を書くと断る。

## コアに足したもの

- **Measure の上限(Max)**:grow で配るとき、サブシステムが申告した上限で止める
  (`docs/subsystem.md`)。
- **折り返しを含む高さ**:列の中に置いた入れ子の箱(行など)の高さを、中の文字の
  折り返しまで含めて測る(`heightForWidth`)。いままでは文字を1行と見なしていたので、
  比較のカードのように横に並べた文字が折り返すと高さが足りなかった。
- **塊ごとの字の大きさ**:`TextRun.Size`(0 なら文字の図形の大きさ)。統計強調の数と単位の
  ように、1つの文字の中で大きさを変えるため。測るのも描くのも塊ごとの大きさで、行の高さは
  文字の図形の大きさで測る(図形の大きさを一番大きい塊に合わせる)。
- どれも、従来の Go で組んだデモ(`go run ./cmd/boxloom`)の出力を変えない(展開して `diff -r` で
  同じことを確かめた。塊ごとの大きさは、既存の PPTX がバイト単位で同じことを確かめた)。

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
- **PowerPoint の形を通すもの**:`a:buSzPct`(箇条の印の大きさ)の `val` は、ISO の XSD
  では `"79%"` の形だけだが、PowerPoint は 1000 分の 1 % の整数(`79365`。ECMA-376 第1版の
  形)で書き、LibreOffice はこちらしか読まない。Boxloom は整数で書くので、検査は
  25000〜400000 の範囲を見てから ISO の形に直して XSD に通す(範囲の外は `schema`)。
- **XSD のほかに見ること**:内容の種類の抜け・余り、関係の指す部品の有無と種類、
  部品の中の `r:id` が関係にあるか、スライドにレイアウトが1つあるかなど要る関係、
  1つの部品の中の図形の id の重複、コネクタの `a:stCxn`・`a:endCxn` の指す先、
  `sldId`・`sldMasterId`・`sldLayoutId` の重複と範囲。code の一覧は道具の頭の説明。
- **別の実装で読めるか**:`--python-pptx` で python-pptx で開き直して全部の文字を読み、
  `--soffice` で LibreOffice で PDF に変換してページの数がスライドの数と合うかを見る。
  `--self-test` は python-pptx があれば必ず使う。
- **自分の試験**:examples の IR・Markdown(警告の例も含む)を dark/light で作った46個と、
  従来のデモ2枚が全部通ること。それを壊したもの(XML の誤り・Schema 外の要素と値・箇条の印の大きさの範囲外・
  部品の削除・内容の種類の抜けと余り・関係の無い r:id・図形の id の重複・コネクタの
  指す先・スライドの id の重複・レイアウトの無いスライド・種類違いの関係・
  `[Content_Types].xml` の削除・ノートの雛形の抜け・画像の部品の削除・png の Default の
  抜け・画像の関係の無い r:embed・表のセルの Schema 外の要素)が、それぞれ決めた code で
  断られること。
- **限り**:XSD と参照の整合は見るが、PowerPoint 自身が開けるかは見ていない
  (PowerPoint だけが嫌う形は捕まらない)。

## 配置の段の木を見る(boxloom layout)

部品が組んだ箱の木(配置の段。`docs/layers.md`)と、エンジンが決めた座標を JSON で出す。
なぜその大きさ・はみ出しになったかを人と AI が読むためのもの。同じ形を読み直す口
(`boxloom layout-build`)は開発者・試験用に限る(後述の「配置の段の木を読み直す」。
`docs/layers.md` の4章)。

```sh
cd boxloom
go run ./cmd/boxloom layout ../examples/md/blocks.bx.md            # 標準出力に JSON
go run ./cmd/boxloom layout -template dark ../examples/ir/demo-deck.json
```

形は `schema/layout.schema.json`(`format` は `boxloom-layout/0.1`)。テンプレートの
選び方・画像の読み方は `build` と同じ。

- スライドごとに `root` の箱から木を出す。箱は `kind`(row・col・text・subsystem)・
  `path`(箱を作った IR の場所。JSON Pointer で、題は `/slides/S/title`、本文は
  `/slides/S/body`、ブロックは `…/body/N`、カードは `…/options/K`・`…/items/K`、並列の
  まとまりは `…/columns/C`(題は `…/title`、中のブロックは `…/body/K`)、表紙の頭の値は
  `/meta/title` など。場所を持たない箱(余白・フッター・箇条の項目)は一番近い親の場所)・
  `.md`・`.yaml` なら `line`・`box{x,y,width,height}`(pt、0.01pt に丸める)。
- 箱の属性は値のあるものだけ:`gap`・`padding{x,y}`・`grow`・`basis`・`width`・`height`・
  `alignItems`(center のとき)・`justifyContent`(spaceBetween のとき)・`background`・`radius`。
- 文字の葉は `text`・`fontSize`、装飾つきなら `runs`(塊ごとの文字・色・太字・大きさ・
  印・折らない・文の切れ目で折る `phrase`・欄)、装飾なしなら `color`・`bold`。`textAlign`・`anchor`・`placeholder`。
- 計測の印は `marks`:`irType`・`cardRow`・`columnGroups`(まとまりの数)・`keepLine`・
  `nestedSlack`・`list`・`card`。
- サブシステム(表・画像・図・グラフ・Gantt)は `subsystem` に名前と、割り当てられた幅で
  尋ねた申告(`natural`・`min`・`max`・`fixedAspect`)と、描いた矩形 `draw` だけ。中身は出さない。
- 部品は木に数と色を書かず、**トークン**(値の表の名前・段の刻み `space(n)`・`type(n)`・
  テーマの役割名)を書く。組み立ての直後・計測の前に1か所(`resolveSpec`、
  `boxloom/token.go`)で数と色に直す(`docs/layers.md` の 2.1・8.3)。出力には解決した値
  (`gap`・`padding`・`fontSize`・`background` など)と、部品が書いたトークンの両方が出る:
  `tokens` の `gap`・`padding`・`paddingY`・`width`・`height`・`fontSize`・`color`・
  `background`・`radius`、塊の色は `runs[].role`、塊だけの字の大きさは `runs[].sizeToken`。
  `pointIndentEm` のような em の名前は、同じ箱の `fontSize` のトークンに掛けた値になる。
- `box` は build と同じ配置(PPTX はこれをそのまま `a:off`/`a:ext` に書く)。文字の葉・
  カード・箇条・地の箱が PPTX の図形と一致することを `TestLayoutBoxesMatchPPTX`
  (`boxloom/layoutdump_test.go`)が examples の全 deck で確かめる。カードと箇条の中の
  文字の葉は PPTX では図形でなく段落なので、その box は段落の位置(字下げ・段落の前の間)の元になる。
- 出力が Schema に通ることは `python3 tools/validate_layout.py --self-test`(examples の
  全 deck × 同梱のテンプレートと `examples/templates/forest.json`。CI の `python` の手順)。

## 配置の段の木を読み直す(boxloom layout-build。開発者・試験用)

`boxloom layout` が出した木を読み、エンジンで配置して PPTX に書く。エンジンだけを試す、
崩れた例を木のまま再現する、のための口で、**作者(AI)の入口ではない**(`boxloom schema`・
MCP の道具・SKILL.md には出さない。作者は意味(IR)だけを書く)。ライブラリでは
`BuildLayout`(`boxloom/layoutread.go`)。

```sh
cd boxloom
go run ./cmd/boxloom layout ../examples/md/figures.bx.md > /tmp/tree.json
go run ./cmd/boxloom layout-build -deck ../examples/md/figures.bx.md -o /tmp/tree.pptx /tmp/tree.json
```

- 読むのは**トークン**(`tokens`・`runs[].role`・`runs[].sizeToken`)。読むときに値の表で
  引き、間隔・余白は `scale:"space"` の名前か `space(n)`、幅・高さ・角は値の表の長さの名前か
  `space(n)`、字の大きさは `scale:"type"` の名前か `type(n)`(段 `typeSizes` の値)、色は
  テーマの役割名かを確かめる。外れたら誤り `layout-token`(16進の色も誤り)。数と色に直すのは
  組み立てと同じ `resolveSpec` の1か所。
- 解決した値(`gap`・`padding`・`fontSize`・`color`・`background`・`radius`・`width`・
  `height`・塊の `color`・`size`)は出力の控え。省いてよいが、書くならトークンが要り
  (無ければ `layout-raw`)、トークンから引いた値と同じでなければならない
  (`layout-mismatch`)。`grow`・`basis` は比なのでただの数。
- 座標 `box` とサブシステムの `width`・`draw` は**読まない**(エンジンの出力なので、
  あっても誤りにせず無視する。出した木をそのまま読み直せるように)。
- サブシステムの中身は木に無い。`-deck`(ライブラリでは `LayoutReadOptions.Deck`)を渡すと、
  その deck を同じテンプレートで組んだ木のサブシステムを、スライドごとに出てきた順に当てる
  (名前・数が違えば `layout-deck`)。発表者ノートと文書の題も deck から取る。渡さなければ
  サブシステムは木の申告(`natural`・`min`・`max`・`fixedAspect`)を返して何も描かない葉。
- 並列のまとまり・対の並びの、まとまりごと・対ごとのはみ出しの測り(`columnGroups`・
  `pairRows` の中身)は木から作り直さない。警告は出さず、PPTX を書くだけ。
- 誤りは `Issue` の形で、`path` は読んだ JSON の中の場所(例
  `/slides/1/root/children/1/tokens/gap`)。
- examples の全 deck(テンプレートの既定・dark・light)で、出した木を `-deck` つきで読み直すと
  build と同じ PPTX(zip の中身がすべて同じ)になることを `TestLayoutRoundTrip`
  (`boxloom/layoutread_test.go`)が確かめる。段から外れた値・知らない名前・役割名の誤りは
  `TestLayoutReadChecksTokens`。

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
| `python` | `validate_ir.py --self-test` と、`boxloom schema` の出力での `--bundle --self-test`、`validate_layout.py --self-test`(`boxloom layout` の出力を `schema/layout.schema.json` で) |
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
