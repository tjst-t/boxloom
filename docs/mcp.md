# MCP サーバ(`boxloom mcp`)

AI エージェントが Boxloom を直接呼ぶための入口。`boxloom mcp` は stdio の
MCP(Model Context Protocol)サーバで、道具を3つ出す。

| 道具 | コマンドで言えば | 返すもの |
|---|---|---|
| `schema` | `boxloom schema [-template]` | 囲み・属性・サブシステム・IR の Schema を1つの JSON(`docs/schema-command.md`) |
| `check` | `boxloom check -format json [-template] [-strict] [-fit]` | `{ok, errors, warnings, capacity}`(`fit` で詰めたスライドがあれば `info` も)。容量(`capacity`)はいつも付く(`docs/ir.md` の「容量の計測」) |
| `build` | `boxloom build -o 出力 [-template] [-fit]` | `{output, slides, template, warnings}`(`fit` で詰めたスライドがあれば `info` も)。PPTX はファイルに書く |

## 決めたこと

- **依存は標準ライブラリだけ**。MCP の stdio は「1行に1つの JSON-RPC 2.0 の文」
  なので、使うのは `initialize`・`ping`・`tools/list`・`tools/call` と通知だけ。この量なら
  SDK を入れるより自分で書くほうが軽く、単一バイナリ・依存ゼロ(`go list -m all` が
  自モジュールだけ)を崩さない。答える版は `2025-06-18`・`2025-03-26`・`2024-11-05`
  (クライアントが言った版があればそれ、無ければ最新)。
- **コマンドの中に置く**(`boxloom/cmd/boxloom/mcp.go`)。ライブラリはファイルを
  読み書きしないので、パスを受けて読む・PPTX を書く役はコマンドの側が持つ。道具の中身は
  コマンドと同じ公開の入口(`Schema`・`Check`・`Build`。`docs/library.md`)を呼ぶ。
- **要求は1つずつ順に処理する**(ライブラリは `Build`・`Check` を並行して呼べない)。
- **道具の中で panic が起きてもサーバは落ちない**。その要求は `isError` の結果(文に panic の要旨。Boxloom の不具合で、文書の誤りではない)で返し、呼び出しの跡は stderr に書いて、次の要求に答え続ける。字体などのプロセスの状態はライブラリが戻すので、panic の後も元のまま(`TestMCPToolPanic`)。
- 道具の誤り(ファイルが無い・記法の誤りで組めない・知らない引数)は JSON-RPC の誤り
  ではなく、`isError: true` の結果として文で返す。エージェントが読んで直せるように。
  `check` の誤りは道具の誤りではなく、`ok: false` の結果。
- ログは stderr(stdout は MCP の文だけ)。

## 道具の引数

`check` と `build` は文書を `path`(ファイル)か `source`(中身そのもの)で受ける
(どちらか1つ)。

| 引数 | 道具 | 意味 |
|---|---|---|
| `path` | check・build | 文書のファイル。拡張子で読み方を選ぶ(`.md`・`.bx.md` は Markdown、`.yaml`・`.yml` は YAML の deck、ほかは IR の JSON)。画像はこのフォルダから読む |
| `source` | check・build | 文書の中身。`path` の代わり |
| `name` | check・build | `source` のときの名前(拡張子で読み方が決まる。省くと `deck.bx.md`)。画像はサーバの作業フォルダから読む |
| `template` | 3つとも | テンプレートの名前(`dark`・`light`)か値の表のファイル(`./my.json`)。省くと文書の `template`、それも無ければ light |
| `strict` | check | 警告も誤りにする(`ok` が false になる) |
| `fit` | check・build | はみ出す本文のスライドをテンプレートの梯子(dense → `fit.shrink`)で詰め、薄い本文のスライドは `fit.grow` で広げる(`boxloom build -fit`・`check -fit` と同じ。調整したスライドは `info` の `fit-applied`。最後の手段で、まず文を削る・分ける) |
| `output` | build | 書く PPTX。省くとサーバの作業フォルダの `out/<文書の名前>.pptx`。返す `output` は絶対パス |
| `allowOverflow` | build | はみ出し(`layout-overflow`)があっても書く。省くと(false)はみ出しは誤り:書かずに isError で、はみ出しを hint つきで全件返す(`boxloom build` と同じ、`docs/build.md`) |

相対パスはサーバの作業フォルダから解く。Claude Code はサーバをプロジェクトのフォルダで
起こすので、プロジェクトからの相対パスで書けばよい。知らない引数は誤り。

## Claude Code から使う手順

1. バイナリを作る(Go 1.24 以上。リポジトリの `boxloom/` で)。

   ```sh
   cd boxloom
   go build -o ~/.local/bin/boxloom ./cmd/boxloom
   ```

2. Claude Code に登録する。どのプロジェクトでも使うなら `-s user`、このプロジェクト
   だけなら `-s project`(`.mcp.json` に書かれる)。

   ```sh
   claude mcp add -s user boxloom -- ~/.local/bin/boxloom mcp
   claude mcp list          # boxloom: … mcp - ✓ Connected
   ```

   `.mcp.json` に手で書くなら:

   ```json
   { "mcpServers": { "boxloom": { "command": "/home/you/.local/bin/boxloom", "args": ["mcp"] } } }
   ```

3. Claude Code の中で `/mcp` を開くと `boxloom` に道具 `schema`・`check`・`build` が並ぶ。
   道具の名前は `mcp__boxloom__schema`・`mcp__boxloom__check`・`mcp__boxloom__build`。

4. 頼む。例:

   > `mcp__boxloom__schema` で light の書き方を見て、新しいクラスタへの移行計画を
   > 3枚の `plan.bx.md` に書いて。`mcp__boxloom__check` で誤りと `layout-overflow` が
   > 無くなるまで直してから、`mcp__boxloom__build` で `out/plan.pptx` にして。

   エージェントは schema → 書く → check(`capacity` の `remaining`・
   `remainingBodyLines` を見て足す・削る・分ける)→ build と進む。書き方・選ぶ規則・
   直し方は Skill(`skills/boxloom/SKILL.md`)にある。Skill のコマンドの代わりに
   この道具を呼べばよい。

## 確かめたこと

- `TestMCPSession`(`boxloom/cmd/boxloom/mcp_test.go`)が、Claude Code が送るのと同じ順の
  文(`initialize` → `notifications/initialized` → `tools/list` → `tools/call`)を流し、
  `schema` の JSON、`examples/md/demo-deck.bx.md` の `check`(`ok` と容量)、同じ文書の
  `build`(3枚・dark・zip として開ける PPTX)、`source` で渡した誤りのある文書の行つきの
  誤り、知らないメソッド・知らない道具・知らない引数・読めない行の扱いを確かめる。
- 手で流すときは次のとおり(作業フォルダに `demo-deck.bx.md` を置いて)。最後の行の
  答えの `output` に PPTX ができる。

  ```sh
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
    '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check","arguments":{"path":"demo-deck.bx.md"}}}' \
    '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"build","arguments":{"path":"demo-deck.bx.md","output":"o/deck.pptx"}}}' \
  | boxloom mcp
  ```

- **Claude Code から呼んで PPTX を作った**(2026-11-12)。
  - 環境:Claude Code を Agent Client Protocol のアダプター
    `@agentclientprotocol/claude-agent-acp` 0.81.1 で動かした。`claude` コマンドは入って
    いなかったので、`claude mcp add` そのものは流しておらず、CLI の版も分からない。
  - 登録:このブランチの `boxloom/` で `go build ./cmd/boxloom` したバイナリを、
    プロジェクトの `.mcp.json` に
    `{"mcpServers":{"boxloom":{"type":"stdio","command":"<作ったバイナリ>","args":["mcp"]}}}`
    と書き、`.claude/settings.json` に `{"enableAllProjectMcpServers":true}` を書いた。
  - 文書:架空の内容の、2枚の Markdown の deck(`deck.bx.md`)。
  - 結果:
    - Claude Code に `mcp__boxloom__schema`・`mcp__boxloom__check`・`mcp__boxloom__build`
      が並んだ(遅れて読み込む道具として、ToolSearch で読み込んで使った)。
    - `mcp__boxloom__check` `{"path":"deck.bx.md"}` は `ok` で、誤り 0・警告 1
      (fill-ratio の警告。容量つき)。
    - `mcp__boxloom__build` `{"path":"deck.bx.md","output":"deck.pptx"}` は light の
      テンプレートで2枚の PPTX(10119 バイト)を書いた。中には期待する部品がそろっていた。
- **まだ確かめていないこと**:CLI での `claude mcp add -s user` による登録(手順 2 の
  コマンドの道)と、対話の CLI での `/mcp` の一覧(手順 3)。CLI のある環境で流したら、
  CLI の版と結果をここに書き足す。
