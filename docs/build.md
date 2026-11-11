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
- IR の `notes`(発表者ノート)は、改行ごとに1段落でそのスライドのノートに入る。
  ノートのページにはスライドの縮図と文の置き場を置く(LibreOffice のノートの
  ページでも見える)。
- 書き出さずに検査だけするなら `go run . check deck.json`(`.bx.md` も可。誤りと
  警告を `docs/content-syntax.md` の5章の形で出す。`-format json` で JSON。
  `docs/ir.md` の「意味の検査」)。

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

## まだやっていないこと

- 押せるリンク、`code` の等幅の字体。
- 本文の高さがページに収まらないときの自動の扱い(いまは比例で縮むだけ)。
  はみ出しは `layout-overflow` の警告で知らせ、直すのは作者(`docs/ir.md` の
  「はみ出しの測り方」)。
