# `boxloom schema` — AI に渡す「書けるもの」の一覧

`docs/content-syntax.md` の6章のコマンド。いまのテンプレートとサブシステムが名乗る
囲み・属性・サブシステムの Schema を、**1つの JSON** にまとめて標準出力に書く。
AI に文書を書かせるときは、これをそのまま渡す。

```sh
cd boxloom
go run ./cmd/boxloom schema                    # 全部のテンプレート(dark・light)
go run ./cmd/boxloom schema -template dark     # そのテンプレートが名乗るものだけ
go run ./cmd/boxloom schema -template ./my.json  # 外の値の表(docs/template-values.md)でも絞れる
```

- 知らないテンプレートの名前は誤り(終了コード 1)。引数の誤りは 2。
- 出力は決まった順に並ぶ(同じ版なら毎回同じバイト列)。

## 出力の形

```jsonc
{
  "boxloom": "0.1",                 // IR の版
  "about": "…",                     // AI 向けの前置き(1段落)
  "workflow": ["書く", "boxloom check", "直す", "boxloom build"],
  "markdown": {                     // テンプレートに依らないコアの記法
    "frontMatter": [{"key": "title", "meaning": "…", "required": true, "ir": "meta.title"}, …],
    "slideAttributes": ["type", "variant"], // `# 題 {type=… variant=…}` に書ける属性
    "blocks": [{"name": "notes", …}], // テンプレートに依らない囲み(発表者ノート)
    "body": [{"syntax": "**…**", "meaning": "強調", "ir": "インラインの em"}, …],
    "rules": ["囲みは :::名前 で開き …", …]
  },
  "templates": [                    // -template を付けると1つだけ
    {
      "name": "dark",
      "variants": [],               // variant に書ける名前(いまのテンプレートは持たない)
      "slides": [                   // スライドの種類(`# 題 {type=名前}`。表紙は文書の頭から作るので無い)
        {"name": "content", "heading": "題(…結論の文)", "body": ["本文", "notes"], …},
        {"name": "section", "heading": "章の題", "body": ["notes"],
         "meaning": "章の変わり目(R1)。本文は書けない …", "ir": "{type: section, title}"},
        {"name": "statement", "heading": "言いたいことの1文", "body": ["notes"], …}
      ],
      "blocks": [                   // このテンプレートが名乗る囲み(:::名前)
        {"name": "compare", "contains": ["option"], "minChildren": 2,
         "attributes": ["variant"], "meaning": "選択肢の比較", "ir": "compare"},
        {"name": "option", "label": "選択肢の名前", "labelRequired": true,
         "inside": ["compare"], "contains": ["pros", "cons"], …},
        {"name": "stat", "contains": ["value"], "minChildren": 1, …},
        {"name": "value", "label": "数(書いたとおりの文字 …)", "labelRequired": true,
         "inside": ["stat"], "contains": ["段落1つ"], "attributes": ["unit"], …},
        …                           // note・pros・cons・quote・takeaway
      ],
      "subsystems": [
        {"name": "gantt", "fence": "```gantt", "attributes": ["variant"],
         "markdown": ["日付は YYYY-MM か …", …],   // Schema に書けない Markdown の約束事
         "schema": "/irSchema/$defs/gantt",         // 下の irSchema の中の場所(JSON Pointer)
         "schemaId": "https://boxloom.invalid/schema/ir/0.1/subsystems/gantt.schema.json"},
        {"name": "shapes", "fence": "```shapes", …},  // 生の図形への逃げ道(最後の手段)
        {"name": "process", "fence": "```process", …}  // ほかに orgchart・matrix・funnel(docs/subsystem.md の11節)
      ]
    }
  ],
  "irSchema": { … }                 // IR(JSON)を直接書くときの JSON Schema
}
```

### `blocks` の欄(`markdown.blocks` と `templates[].blocks` で同じ)

| 欄 | 意味 |
|---|---|
| `name` | 囲みの名前(`:::name`) |
| `label` / `labelRequired` | 名前の後ろに書く文の意味と、それが必須か(無ければ書けない) |
| `inside` | 置ける場所(親の囲み)。無ければ本文に置く |
| `contains` | 中に書けるもの。囲みの名前か、`本文`・`箇条`・`段落1つ` |
| `minChildren` | `contains` の囲みが最低いくつ要るか |
| `attributes` | `{名前=値}` で書ける属性 |
| `meaning` / `ir` | 意味と、できる IR の場所 |

### `slides` の欄(`templates[].slides`)

| 欄 | 意味 |
|---|---|
| `name` | `type` に書く名前(IR の `slides[].type`) |
| `heading` | `#` の行の文の意味(種類で違う) |
| `body` | `#` の行の後ろに書けるもの(`notes` は `:::notes`) |
| `meaning` / `ir` | 意味と、できる IR の形 |

### `irSchema`

JSON Schema 2020-12 の**複合文書**。外を参照しない1つの文書で、そのまま検査器に
渡せる(registry が要らない)。

- 根は `$ref` でコアを指し、`$defs` に次を同梱する。
  - `core`:`schema/core.schema.json`(`$id` はそのまま)
  - サブシステムの名前(`gantt` など):各サブシステムが名乗る Schema(`$id` はそのまま)
  - `subsystem`:差し込み口(`$dynamicAnchor: subsystem`)。**そのテンプレートが
    名乗るサブシステムだけ**を `enum` と `allOf` で受け付ける。
- 中の相対の `$ref` は書き換えない(同梱した文書の `$id` から解決される)。
- 根の `$id` は `https://boxloom.invalid/schema/ir/0.1/bundle/<テンプレート名|all>.schema.json`。
- `schema/boxloom-ir.schema.json`(入口)と同じ働き。違いは、受け付けるサブシステムが
  テンプレートの登録簿から決まること。
- `python3 tools/validate_ir.py --bundle OUT.json --self-test` で、この Schema だけで
  例が通る・断られることを確かめられる。

`variant` の名前は Schema では形だけを見る(コアと同じ)。名前がテンプレートにあるかは
`boxloom check` が見る。

## どこから来るか(Go)

| 出力 | 出どころ |
|---|---|
| `markdown` | `vocab.go`(`frontMatterKeys`・`bodySyntax`・`markdownRules`)。パーサーも文書の頭のキーを同じ表から引く |
| `markdown.blocks` | `coreBlocks()`(`:::notes`) |
| `templates[].blocks` | `Template.Blocks`。いまは dark・light とも `defaultBlocks()`。パーサーの `knownFences` は `coreBlocks()` と `defaultBlocks()` から作る |
| `templates[].variants` | `Template.Variants` |
| `templates[].slides` | `Template.Slides`。いまは dark・light とも `defaultSlides()`。パーサーの `type` の検査も同じ表から引く |
| `templates[].subsystems` | テンプレートの `SubsystemRegistry` の種類と、`Declare` で付けた `SubsystemDecl`(Schema と Markdown の約束事) |
| `irSchema` | 埋め込んだ `boxloom/schemas/`(リポジトリ直下 `schema/` の写し) |

- `go:embed` は上の階層を読めないので、`schema/` を `boxloom/schemas/` に写して
  埋め込む。`schema/` を直したら `go generate` で写し直す。写しがずれると
  `TestEmbeddedSchemasInSync` が落ちる。**正本は `schema/`**。
- `Override` で描き方を差し替えても `SubsystemDecl` は変わらない(意味の形は
  サブシステムの種類が決め、テンプレートが変えるのは見た目だけ)。
- 名乗る表と実際の検査がずれないことは `TestSchemaMatchesParser` が見る。

## まだ無いもの

- Markdown のパーサーはテンプレートを見ずに既定の囲みの一式で検査する。テンプレートが
  違う囲みを名乗るようになったら、パーサーに `Template.Blocks` を渡す。
- 囲みの属性の値の型(仕様の7章の未決)は出していない(いまは `variant` と `:::value` の
  `unit`。どちらも空白を含まない1語)。
- `boxloom fmt --to md`(IR → Markdown)はまだ無い。
