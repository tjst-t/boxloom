# サブシステムの差し込み口

サブシステムは、Box レイアウトの葉に埋め込む「自分で描く部品」です(Gantt
チャートなど)。この文書は、その差し込み口(Go の interface)の設計と理由を
まとめたものです。実装は `boxloom/subsystem.go`、Gantt での使い方は
`boxloom/gantt.go`、試験は `boxloom/subsystem_test.go` にあります。

## 決めたこと

```go
type Subsystem interface {
	Name() string                       // 種類の名前。警告の文に使う
	Measure(c Constraint) Measurement   // 自然・最小・上限の大きさの自己申告
	Render(ctx RenderContext, box LBox) // 割り当てられた矩形に描く
}

type Constraint  struct{ Width, Height float64 } // 0 より大きい軸だけが分かっている
type Measurement struct {
	Natural, Min, Max Size // Max の 0 は「上限なし」
	FixedAspect       bool // 形(縦横比)を保つ図(9節)
}

type RenderContext struct {
	Out   *PPTXBuilder // プリミティブの書き出し先
	Roles Roles        // 役割名 → 実際の値の解決器(テーマが実装する)
}
type Roles interface{ Color(role string) string }
```

テンプレートが描き方を差し替えるための登録簿:

```go
type SubsystemFactory func(data any) (Subsystem, error)

reg := DefaultSubsystems().Clone()     // コアの既定の表を写す
reg.Override("gantt", myGanttFactory)  // このテンプレートだけ Gantt の描き方を変える
sub, err := reg.Build("gantt", chart)  // コンテンツのデータ(意味だけ)から作る
spec := SubsystemNode(sub, WithGrow(1))
```

## 1. 自己申告: 自然な大きさと最小の大きさの2つ

`Measure` は2つの大きさを返します。

- **Natural(自然な大きさ)**:無理なく描ける大きさ。レイアウトはこれを
  basis(主軸の出発点)に使います。Gantt なら「行の高さ 26pt × 行数 + 周りの
  部品」です。
- **Min(最小の大きさ)**:これより小さいと読めなくなる大きさ。Gantt なら
  「行の高さ 17pt × 行数」と、「複数行のグループ名が収まる行の高さ」の大きいほう
  です。

**理由**:1つだけ(自然な大きさだけ)だと、スライドに収まらないときに
「どこまで縮めてよいか」をレイアウトが知りません。2つあれば、レイアウトは
「まずゆとりを返してもらう → それでも足りなければ警告」と段階を踏めます。
CSS の min-content / max-content と同じ考え方を、PPT のスライドの事情
(紙面が固定でスクロールできない)に合わせて2つに絞りました。

- **Max(上限)**:これより大きくしても見やすくならない大きさ(0 は上限なし)。
  Gantt なら「行の高さ 40pt × 行数 + 周りの部品」です。

**Max を足した理由**:IR のデモの Gantt は行が2本しかなく、grow で余りを
全部もらうと帯の高さが 1本 140pt 近くになり、間延びしました。サブシステムは
「ここまでなら伸びてよい」を自分で言えるべきで、言わなければ従来どおり
いくらでも伸びます。

**Constraint**:親は先に交差軸(縦に並べるなら幅)を決め、そのあと主軸
(高さ)を尋ねます。サブシステムは分かっている軸を受け取って、もう一方の軸を
答えます。Gantt は幅によって高さが変わりませんが、折り返す文の多い図(表や
組織図など)は、幅に応じて要る高さが変わるので、この形にしてあります。

## 2. レイアウトでの扱い(Phase3a)

- **basis**:サブシステムの葉は、明示の basis・幅・高さが無ければ、自己申告の
  自然な大きさを basis にします。
- **余るとき**:いままでどおり grow で受け取ります。ただしサブシステムが
  申告した Max を超えた分は、Max の無いほかの伸びる子へ grow の比で回します
  (1回だけ。回す先が無ければ空きのまま残し、本文の埋まり具合の検査が
  警告を出します)。
- **足りないとき**:従来の比例の縮め(Phase3)の **前に**、サブシステムが
  「自然な大きさと最小の大きさの間のゆとり」を返します(Phase3a)。
  見出しなどのテキストはここでは縮みません。
- **最小でも足りないとき**:従来どおり全員が比例して縮み、検査
  (`lintSubsystemMin`)が `[SUBSYSTEM WARNING] gantt: needs at least …pt height`
  を check・build の出力に出します(ノートには書きません)。黙って潰しません。

**理由**:サブシステムを入れた最初の試みで、Gantt の自然な高さ(446pt)が
デモのスライドの空き(414pt)より大きく、従来の比例の縮めで **見出しまで
縮んでしまいました**。図は伸び縮みに強く、見出しの文字は弱い。なので、
自己申告で「縮めてよい幅」を言える部品から先に縮めるのが筋です。サブシステムを
含まない木では Phase3a は何もしないので、Python 版との座標の一致は崩れません。

**デモでの数字**:Gantt の自然な高さ 446.0pt、最小 406.4pt、割り当て
413.8pt(幅 912pt)。最小以上なので警告は出ません(値の段階に寄せる前、Backlog #29 より前の数字)。

## 3. 役割名の解決器(Roles)

サブシステムは色を `ctx.Roles.Color("danger")` のように役割名で引きます。
テーマ(`Theme`)が `Roles` を満たします。

- **理由**:サブシステムが16進の色を知らないことで、テンプレートとコンテンツの
  分離が守られます(CLAUDE.md の設計原則)。interface にしたので、将来
  色以外の値(文字の大きさ・線の太さ)を役割名で引く形にも広げられます。
- **テーマに無い役割名**:描くときに `checkedRoles` が覚えておき、
  `[ROLE WARNING] theme has no value for roles: [...]` として出します。いままでは
  空の色のまま黙って出力していました。

## 4. テンプレートによる差し替え(SubsystemRegistry)

- コアは既定の表(`DefaultSubsystems()`:いまは `gantt`・`shapes` と、11節の図
  `process`・`orgchart`・`matrix`・`funnel`、12節の数値のグラフ `chart`)を持ちます。
- テンプレートは `Clone()` した表に `Override` で自分の描き方を入れます。
  コンテンツのデータ(`GanttChart`)は変わらず、描き方だけが変わります。
- 種類は `Declare(kind, SubsystemDecl{Schema, Markdown})` で、作者に見せるもの
  (IR のブロックの JSON Schema と、Markdown で書くときの約束事)を名乗ります。
  `boxloom schema` がこれを集めます(`docs/schema-command.md`)。`Override` で描き方を
  差し替えても名乗るものは変わりません。
- `Override` は無い名前を断り、`Register` は重複を断ります。打ち間違いを黙って
  通さないためです。
- **理由**:Memory の「部品とサブシステムはコードで書く」に合わせ、差し替えの
  単位を「種類の名前ごとの Factory」にしました。テンプレートが Gantt を
  丸ごと別の描き方にする、という一番大きな自由度を、コアに手を入れずに持てます。

## 5. 既存の canvas との関係

`Canvas(func(ctx, box) {...})` は残してあり、「自然な大きさ 0 を申告する簡易な
サブシステム」として動きます。場所は grow で受け取る、旧来の canvas と同じ
ふるまいです。

## 6. IR から作る(注釈の置き場所はサブシステムが決める)

登録簿の Factory には IR の `data`(`json.RawMessage`)をそのまま渡します。
Gantt の Factory は `ganttFromIR`(`gantt_ir.go`)で読み、`GanttChart` を
組みます。IR は意味だけを持つので、見た目に関わる次のことは Gantt の側が
決めます。

| IR に無いもの | Gantt が決めること |
|---|---|
| 軸の圧縮の制御点 | 目盛り(と軸を切る日付)で区切った区間に、日数の平方根に比べた幅を割り当てる |
| 目盛りの文字 | `at` から `29/12` の形を作る。範囲の両端に目盛りが無ければ日付だけの目盛りを足す |
| 状態の無い目盛りの色 | `text_primary`(両端に足したものは `text_muted`) |
| 節目の文字 | `28/5 ライセンス更新` のように日付を前に付ける |
| 注釈の上下と横のずらし | 下の「注釈の置き場所」 |
| 確定していない区間(`tentative`) | 塗らずに、状態の色の破線の枠で描く |

**注釈の置き場所**(`planAnnotations`):

1. 指す先(`target` のグループか行)が本体の上半分なら上のレーン、下半分か
   真ん中なら下のレーンを先に試す。指す先の高さは行の数だけで決める(行の
   高さに依らないので、Measure と描くときで同じ答えになる)。
2. 日付の真上に箱を置けなければ、もう一方のレーンの真上を試す。
3. それも埋まっていれば、箱の幅の半分ずつ左右へずらし、最初に空いた場所に
   置く。
4. どこも空いていなければ、最初の候補(希望のレーンの真上)に重ねて置き、
   警告を出す(下の「描く前の検査」)。
5. 点は指す先の縦の真ん中に打ち、箱と破線で結ぶ。`emphasis` なら日付の
   縦線を本体の上から下まで引く。
6. 破線(引き出し線)は、目盛りの文字(`27/6` と下の段の文字)と節目の文字
   (`★ 27/6 仕様の凍結`)を横切るところだけ空けて描く(下の「引き出し線と文字」)。

**引き出し線と文字**(Issue #40、`splitAround`・`ganttLabelRects`):上のレーンの
箱から本体の点へ引く線は、必ず節目と軸の文字の帯を通る。注釈の日付に目盛りか
節目があると(箱は日付の真上に置くので、ほとんどの場合そうなる)、線が文字の
真ん中を通って字を隠していた。いまは、目盛りと節目の文字が実際に占める矩形
(字の幅 × 行の高さ。文字の箱そのものではない)を 2pt 広げ、線のうちその中を
通る部分を描かない。線は文字の前後で切れた数本の線分になり、点から箱まで
続いて見える(1pt に満たない切れ端は描かない)。どの文字とも交わらない線は
従来どおり1本で、その PPTX は変わらない。

- **採らなかった案**:文字の後ろに背景色の帯を敷く(テーマの背景と本体の地の色が
  違う場所で浮く。dark と light の両方で正しい色を選ぶ仕組みが要る)/線を文字の
  脇へ曲げる(日付を指す縦の線が日付からずれ、指す先が読めなくなる)/箱を
  ずらす(目盛りはどの日付にもあり得るので、避けられないことがある)。
- **警告は足さない**:線の両端(本体の中の点と、レーンの箱の縁)はどちらも文字の
  帯の外にある(レーンと節目の間には `annotationLaneGap` の隙間がある)ので、
  切っても点と箱を結ぶ線は必ず残る。黙って消えるものが無いので、警告にする
  ことも無い。
- **本体の中の文字は避けない**:線は点に届くために本体に入り、区間の帯や
  `emphasis` の縦線と同じく帯の上を通る。区間の文や行の注記を避けるかどうかは
  別の問題として残している(点が文字の上にあると避けようがない)。
- 例は `examples/md/gantt-leader.bx.md`(↔ `examples/ir/gantt-leader.json`)。試験は
  `gantt_leader_test.go`(この例・`subsystems-showcase`・`gantt-full` を dark と light で
  組み、どの引き出し線の線分も文字の矩形に入らず、点から箱まで続くことを確かめる)。
  比較の画像は `verification-screenshots/gantt_leader_compare_*.png`(左が以前・右が直したもの)。

**箱の大きさ**(`annBoxSize`):自動の注釈の箱は文に合わせる。幅は文の一番
長い行が収まる幅で、96〜240pt に収める(数は `subsystems/gantt.json`。
`docs/template-values.md`)。240pt に収まらない文は折り返し、その
行数ぶん高くする。レーンの高さは、そのレーンに置く箱のうち一番高いものに
合わせる。上のレーンは箱の下端を揃え(チャートに近い側)、下のレーンは上端を
揃える。手で置いた注釈は従来どおり 176×38pt の固定。

上下のレーンのどちらを使うか・箱の高さでチャートの高さが変わるので、Measure も同じ
計画(幅だけを入力にする)からレーンの有無を出します。Go で直接組んだ
デモの注釈(`Direction` が `up`/`down`)は、従来どおり手で決めた置き方の
ままです。

## 7. 描く前の検査(SubsystemLinter)

サブシステムが自分の中身について警告を出せるように、**任意の** interface を
足した。

```go
type SubsystemLinter interface {
	Lint(box LBox) []string // box は割り当てられた矩形
}
```

レイアウトのあと、描く前に `lintSubsystems` が木をたどり、最小の大きさの検査と
一緒に `Lint` を呼ぶ。返った文は、ほかの警告と同じく標準出力(check・build の出力)に
出る。スライドのノートには入れない。本文のスライドでは本文のブロックごとに呼び、文に
`body[N]: ` を付ける(`lintBlockSubsystems`)ので、check はその警告の path をブロック
(`/slides/S/body/N`)にし、.md ならブロックの行を添える。Gantt は、注釈を重ねて置くしかなかったとき
`[SUBSYSTEM WARNING] gantt: annotation 3 ("…") has no free place and overlaps another annotation`
を返す。

**Subsystem interface に足さず、別の任意の interface にした理由**:検査する
ことの無いサブシステム(Canvas の簡易なものなど)に空の関数を書かせずに済み、
既存のサブシステムを壊さない。Go では型の判定(`s.(SubsystemLinter)`)で
満たすかどうかが分かるので、コアは「検査できるなら検査する」とだけ決めればよい。
Render の中で警告を返す形も考えたが、描く処理に副作用以外の出口を作ると、
Render を呼ばずに検査だけしたい道具(将来の `boxloom check`)が使えないので
採らなかった。

## 8. shapes — 生の図形への逃げ道(最後の手段)

**既存のパターン(箇条・比較・注記・Gantt …)で表せないときだけ使う最後の手段。**
架空のシステム構成図のように、意味の部品がまだ無い図を、サブシステムの枠
(キャンバス)の中だけで図形を並べて描く。実装は `boxloom/shapes.go`
(Markdown は `md_shapes.go`、Schema は `schema/subsystems/shapes.schema.json`、
寸法は `boxloom/subsystems/shapes.json`)。例は `examples/ir/shapes.json` と
`examples/md/shapes.bx.md`(同じ IR になる)、絵は
`verification-screenshots/shapes_light.png`・`shapes_dark.png`。

自由に置ける代わりに、分離が崩れないよう次のように絞った(人が決めた方針)。

| 決まり | 中身 | 破ったとき |
|---|---|---|
| 枠の中だけ | 図形はサブシステムの枠の中にだけ置ける。スライドのほかの場所には出せない | — |
| 座標は 0〜1 の相対 | `x`・`y`・`w`・`h`・線の点 `[x, y]` は、枠の内側(`padding` を除いた矩形)に対する割合。`x+w`・`y+h` も 1 以下 | Schema が 0〜1 の外を断り、`boxloom check` が `shape-bounds`(和が 1 を超えるものも) |
| 色は役割名だけ | `fill`・`line`・`textColor`(図形)、`color`(text・線)にテーマの役割名(13個)を書く。16進は書けない | 16進は Schema が断る。テーマに無い名前は check の `role-exists`(build はテンプレートの Factory が断る) |
| 字の大きさは段の名前 | `textStyle` にテンプレートの values のキーを書く。数は書けない | Schema の enum |
| 重なりは警告 | 図形どうしが重なると `shapes-overlap` の警告(誤りにはしない) | — |

### IR の形

```json
{"type": "subsystem", "subsystem": "shapes", "data": {
  "aspect": 2.6,
  "items": [
    {"id": "web", "kind": "roundRect", "x": 0.22, "y": 0.38, "w": 0.15, "h": 0.24,
     "text": "Web 層", "textStyle": "body", "line": "accent"},
    {"id": "api", "kind": "rect", "x": 0.44, "y": 0.38, "w": 0.15, "h": 0.24,
     "text": "API 層", "fill": "accent", "textColor": "on_conditional"},
    {"kind": "text", "x": 0, "y": 0, "w": 0.4, "h": 0.1, "text": "架空の構成", "color": "text_muted"},
    {"kind": "arrow", "from": [0, 0.92], "to": [0.59, 0.92]},
    {"kind": "connector", "from": "web", "to": "api", "arrow": true, "color": "accent"}
  ]}}
```

| kind | 書けるキー(太字は必須) | 省いたとき |
|---|---|---|
| `rect`・`roundRect`・`ellipse` | `id` **`x` `y` `w` `h`** `fill` `line` `text` `textStyle` `textColor` | 塗り `card_alt`、枠線なし、文字 `text_primary` |
| `text` | `id` **`x` `y` `w` `h` `text`** `textStyle` `color` | 文字 `text_primary` |
| `line`・`arrow` | **`from` `to`**(点 `[x, y]`)`color` | `text_muted`。`arrow` は `to` の端に矢印 |
| `connector` | **`from` `to`**(図形の id)`arrow` `color` | `text_muted`、矢印なし |

- `aspect`(任意)は枠の内側の 幅÷高さ。
- `id` は図形(`rect`・`roundRect`・`ellipse`・`text`)にだけ付けられる。文書の中で一意
  (check の `unique-id`)。
- `text` は本文と同じインライン(強調・コード・リンク・改行)。強調はテンプレートの部品と
  同じくアクセントの色の太字。

### 字の段(textStyle)

`small`・`subPoint`・`optionItem`・`body`・`optionLabel`(小さい順。dark / light では
10・12・14・16pt。値の段階 `scale` に載る)。省くと `small`。テンプレートの values のうち、本文の部品が
使う段だけを許した。表紙・題の段(`coverTitle`・`slideTitle` …)は、図の中の文字が
スライドの題と競うので許さない。大きさはテンプレートが values から入れる(登録簿の
`Override`)ので、テンプレートが values を変えれば図の文字も変わる。

### 描き方

- 枠の矩形から `padding` を除いた内側に、相対座標を掛けて置く。書いた順に重ね、
  **コネクタは最後に(図形の上に)描く**(PPTX のコネクタは繋ぐ図形の id を指すので、
  図形を先に書く)。
- 図形の中の文字は図形の中央に置く(図形が文字の枠なので、端を揃える相手が無い。
  テンプレートの「中央揃え禁止」は Box の並べ方の規則で、図の中の札には当てない)。
  幅で事前に折り返す(楕円は内接する矩形の幅)。`text` は左上から置く。
- **コネクタは本物**:PPTX では `p:cxnSp`(`straightConnector1`)で、`a:stCxn`・
  `a:endCxn` で両端の図形の接続点に繋ぐ。PowerPoint で図形を動かすと線が付いてくる。
  辺の真ん中どうしを直線で結ぶ(接続点の番号は presetShapeDefinitions の cxnLst の順で、
  矩形・角丸は上0・左1・下2・右3、楕円は上0・左2・下4・右6)。矢印は `a:tailEnd`(終点)。
  曲がる線(elbow)は無い。
- **使う辺の選び方**(#41):線が辺に浅い角度で入ると、辺をかすめて角で終わったように
  見える。そこで、両端の辺が相手のほうを向いている組だけを候補にし、線と辺のなす角の
  浅いほう(両端の小さいほう)で比べる。向かい合う辺の組(右→左・下→上など)のうち最も
  良いものが 30°以上ならそれを使い、足りなければ隣り合う辺の組(右→上・下→左など)も
  含めて最も良い組を使う(同じなら向かい合う組、次に短いほう)。それでも 30°に
  届かなければ `shapes-connector-angle` の警告(図形の位置を変えないと直らない)。
  端点の座標と `a:stCxn`・`a:endCxn` の idx が指す接続点が一致することは
  `shapes_test.go` の `TestConnectorEndsMatchSites` が確かめる(LibreOffice も同じ点に
  描くことを odp に変えて確かめた)。
- `line`・`arrow` も `p:cxnSp` で書くが、図形には繋がない。

### 自己申告(Measure)

- 自然な大きさ:幅は分かっていればそれ(無ければ `naturalWidth`)。高さは内側の縦横比が
  `aspect`(省けば `defaultAspect`)になる高さ。
- 最小:`minHeight` と「どの図形の文も収まる高さ」(図形の高さは内側の高さに比例するので、
  文が要る高さ ÷ 相対の高さ)の大きいほう。
- 上限:`aspect` を書いたときだけ自然な高さ(作者が決めた形を縦に崩さない。余りは
  空きとして残り、本文の埋まり具合の検査が見る)。書かなければ上限なしで、余りの高さを
  grow で受け取る。

### 検査

| 段階 | code | 見ること |
|---|---|---|
| Schema | `schema` | キー・型・0〜1・16進でないこと・字の段の enum |
| check(誤り) | `shape-bounds` | 座標が 0〜1 にあり、`x+w`・`y+h` が 1 以下 |
| check(誤り) | `role-exists` | 役割名がそのテンプレートのテーマにある(`expected` に役割名の一覧) |
| check(誤り) | `target-exists` | コネクタの `from`・`to` が、同じ図の図形の id を指す |
| check(誤り) | `connector-self` | コネクタの両端が同じ図形 |
| check(誤り) | `unique-id` | id の重複(既存の規則) |
| 警告(Lint) | `shapes-overlap` | 図形どうし(`rect`・`roundRect`・`ellipse`・`text`)の矩形が面積を持って重なる。辺が接するだけなら重ならない。線・矢印・コネクタは見ない。図形の `text` は図形の中身なので重なりではない |
| 警告(Lint) | `shapes-text-overflow` | 図形・`text` の文が、割り当てられた大きさで折り返しても収まらない |
| 警告(Lint) | `shapes-connector-angle` | コネクタがどの辺の組でも、辺と 30°未満の浅い角度でしか繋げない(角で終わったように見える)。path はそのコネクタ。図形を離すか、横か縦に揃える |

警告の文は `[SUBSYSTEM WARNING] shapes: items[7] ("db") overlaps items[6] ("auth")` の
形で、check はその path を後から置いた図形(`/slides/1/body/1/data/items/7`)にする。
.md なら、その図形を書いた行が付く。

Markdown のパーサーは、Gantt と違って shapes をサブシステム自身に読ませない。枠の外・
知らない役割名・無い id を `parse` の `invalid-value`(コードブロックの開きの行)で
断ると、check が図形ごとの行と決まった code(`shape-bounds` など)で返せなくなるため。
`build` は組み立てのときに同じものを断る。

## 9. 縦横比と余りの置き方(図と枠の橋渡し)

外の Box が決めた枠(「スライドの右 60%」など)と、図が持つ自分の形(組織図の縦横比
など)は、ふつう一致しません。その橋渡しを次のように決めました(Backlog #23)。

- **幅から高さを決める申告**:新しいメソッドは足さず、`Measure` の約束にしました。
  `Constraint{Width: w}` を受けた図は、`Natural.Height` に「幅 w での高さ」を返します
  (高さを受けたら `Natural.Width` に「その高さでの幅」)。レイアウトは列の中で
  これを使って幅に合わせた高さを渡し(`heightForWidth`)、行の中では高さから幅を
  尋ねます。縦横比を持つ図は、Measure の中で比から片方の軸を計算すればよい。
- **形を保つ(`FixedAspect`)**:交差軸の stretch などで、図が使い切れない大きさの
  枠が来ることがあります。`FixedAspect` を申告した図には、コアが枠を図の大きさまで
  **切り詰めて** `Render`(と `Lint`)に渡します。幅から決まる高さが枠に収まれば
  その高さ、収まらなければ高さから決まる幅(どちらも Natural)。`fit.go` の
  `fitBox` / `subsystemDrawBox`。
- **上限(Max)で止める**:形を保たない図でも、申告した Max を越える枠(交差軸の
  stretch)は Max まで切り詰めます。主軸は以前から grow が Max で止まっていました。
- **余りの置き方**:切り詰めて残った余りをどちらに置くかは `Spec.Place`
  (`WithPlace(x, y)`:`PlaceStart`・`PlaceCenter`・`PlaceEnd`)。テンプレートの
  骨組み(コード)が決める方針で、既定は始まりの側(左・上)。dark / light は既定の
  まま(中央揃えを避ける方針と合う)。
- **拡大縮小はしない**(object-fit: contain は採らない)。図を縮めると、中の文字の
  大きさがテンプレートの段から外れ、PowerPoint で編集したときの字の大きさも半端に
  なるため。図は枠に合わせて **組み直す**(自分の Measure・Render で大きさを
  決める)。最後の手段としても残していません。枠が最小より小さいときは、従来どおり
  枠のまま渡し、`lintSubsystemMin` が警告します(黙って潰さない)。

**shapes の `aspect` との関係**:shapes は `aspect` を書いたときだけ
`FixedAspect` を申告します(Max も自然な大きさ)。書かなければ形を保たず、余りの
高さを grow で受け取って図形を引き伸ばします(座標が相対なので崩れはしない)。
`aspect` を書いた shapes を行の中に置いて高さが伸ばされても、以前のように図形が
縦に引き伸ばされず、形を保った大きさで `Place` の側に置かれます。

**画像(`image`)も同じ仕組み**:画像はサブシステムではなく第1層のブロックだが、
Box の木の葉としては Subsystem の約束で測って描く(`blocks.go`)。`FixedAspect` を
申告するので、割り当てが画像より大きければ画像の大きさまで切り詰め、余りは `Place`
(既定の左・上)に残る。画像は文字を持たないので、図と違って**縮めてよい**
(`imageMinHeight` まで)。並列のまとまりの中の画像も、テンプレートが印を付けた入れ子の
箱(`nestedSlack`)を通じて Phase3a で文字より先に縮む。

**試験**(`fit_test.go`):2:1 を申告する試験用の図が、列の中で幅 600 に対して
高さ 300 をもらう/行の中で高さ 400 に伸ばされても描くのは 2:1 で、余りが
Place の側(上・中央・下で 0・75・150pt)に残る/形を保たない図は交差軸で Max に
止まり、Max が無ければ枠のまま。

## 10. Gantt の行の注記が箱に収まらないとき(Backlog #37)

行の注記(`note`。破線の箱に文を入れる)の文は箱の中で折り返します。以前は行の
高さを文に関わらず決めていたので、2行に折り返した文が箱の下へはみ出していました
(`gantt-full` の「Loom 3/4/5」の行)。

**直し方**:文が収まらない **その行だけ** を、収まる高さ(文の高さ + 箱の上下の
隙間 6pt)まで高くし、その分ほかの行を低くします(本体の高さの合計は同じ)。
Measure の Natural・Min・Max にもこの高さを入れるので、枠が足りなければ
`subsystem-min` の警告が出ます。

**ほかの案を採らなかった理由**:

- 切り詰める(1行にして「…」):作者の文が黙って欠ける。「黙って潰さない」に反する。
- 字を小さくする:字の大きさがテンプレートの段から外れる(9節と同じ理由)。
- 全部の行を一律に高くする:最小の高さが行の数だけ増え、`gantt-full` が本文に
  収まらなくなった(最小 406pt → 454pt、`layout-overflow` と `subsystem-min` が出た)。
- 警告だけ出す:はみ出した絵が残る。

注記の無い・収まる図は以前と同じ数で描くので、PPTX は変わりません(変わったのは
`gantt-full` だけ。比較の画像は `verification-screenshots/gantt_note_fit_compare.png`、
左が以前・右が直したもの)。区間(segment)の文は 11節。

## 11. Gantt の区間の文が帯に収まらないとき(Backlog #39)

以前は、区間(segment)の文を折り返さずに帯へ置き、帯の幅が 46pt 以下なら文を
出していませんでした(黙って消す・帯からはみ出す、のどちらも警告が無かった)。
いまは、帯の中で文に使える幅(帯の幅 − 8pt)で、次の順に決めます
(`planSegmentLabel`。Measure・Lint・描画が同じ計算を使う)。

| 場合 | 置き方 |
|---|---|
| 1行で収まる | 以前と同じ(帯の中に1行) |
| 収まらないが、折り返せば **3行まで** で、文に使える幅が字2つ分以上ある | 帯の中で折り返す。行の注記(10節)と同じく、その行だけを収まる高さまで高くし、ほかの行が譲る。Measure にも入るので、枠が足りなければ `subsystem-min` |
| それ以外(帯が狭すぎる・切れない語が帯より長い) | 文を描かず、check・build に `gantt-segment-label` の警告を出す。path はその区間(`…/data/groups/G/rows/R/segments/S`)、.md ならその区間の行 |

**帯の外に置く案を採らなかった理由**:帯の外(右隣・上下)はほかの区間・注釈・
目盛りの場所で、置き場所を探す仕組み(注釈の置き場所の計画)を区間にも広げる
ことになる。狭い区間の説明は、作者が注釈(`annotations`)か行の注記に移すのが
意味として正しいので、警告で知らせて作者に選ばせる。3行の上限は、細長い文の柱を
作らないための目安(値の表には出していない)。

1行で収まる文の描き方は以前と同じなので、従来のデモの PPTX は変わらない。例は
`examples/md/warnings/gantt-segment-label.bx.md`(↔ `examples/ir/check-warnings/gantt-segment-label.json`)。

## 11. 図のサブシステム(process・orgchart・matrix・funnel)

`docs/patterns.md` の語彙の第3層のうち、**箱と線で描く4つの図**(Backlog #24〜#27)。
shapes で手で置いていた形(箱を矢印で並べる・親と子の箱・格子に評価・減っていく帯)を、
意味だけのデータから描く。実装は `boxloom/process.go`・`orgchart.go`・`matrix.go`・
`funnel.go`(共通の部分は `figures.go`、Markdown は `md_figures.go`)、Schema は
`schema/subsystems/<名前>.schema.json`、寸法は `boxloom/subsystems/<名前>.json`。例は
`examples/md/figures.bx.md`(↔ `examples/ir/figures.json`)と、
`examples/md/subsystems-showcase.bx.md` の4〜7枚目。

### 共通の決まり

- **作者が書けるのは意味だけ**:座標・大きさ・色・字の大きさは書けない(shapes と違う)。
  書けるのは文(title・detail・note・セルの text)と、状態 `status`
  (`support`・`conditional`・`none`・`danger`)、目立たせる `emphasis`(真偽値)だけ。
- **色**:状態と強調から役割名を決める。強調は塗り `accent`・文字 `on_conditional`。
  状態は塗り `card_alt` と状態の色の枠線(役割名は状態の名前そのもの)。どちらも無ければ
  塗り `card_alt` だけ。補足の文は `text_muted`、強調(`**…**`)は `accent` の太字。
- **字の大きさ**:テンプレートの values の段から引く(題 `body`、2段目・セル・注記
  `subPoint`、箱の2段目・段階の下の補足・図の説明 `figureDetail`、凡例 `small`)。
  テンプレートが values を変えれば図の字も変わる。
- **大きさの申告(Measure)**:縦に積む部品(箱の行・段の間・補足の文)ごとに
  「最小・自然・上限」を持ち、その和を Min・Natural・Max として申告する。箱の最小は
  「その幅で中の文を折り返して収まる高さ」なので、狭い枠ほど高くなる(幅から高さを
  決める。9節)。割り当てられた高さは、最小〜自然の間なら(自然−最小)に比例して、
  自然〜上限の間なら(上限−自然)に比例して配る(`flexSizes`)。上限より大きい枠は
  上限で止まり(`fitBox`)、余りは空きとして残る。どれも形(縦横比)は保たない。
- **本物のコネクタ**:箱と箱の線は `p:cxnSp` で、`a:stCxn`・`a:endCxn` で両端の箱に
  繋ぐ(PowerPoint で箱を動かすと線が付いてくる)。曲がる線は `bentConnector3` を
  90° 回して「縦 → 横 → 縦」にしたもの(`AddElbowConnector`)。横にずれていなければ
  直線(`straightConnector1`)にする。
- **文は黙って削らない**:箱の中の文は幅で折り返し、そのぶん箱を高くする。折り返せない
  語(長い英単語など)が箱より広い、または割り当てられた高さが足りないときは
  `[SUBSYSTEM WARNING] process: steps[2] ("…"): text does not fit (…)` を出す。check は
  `figure-text-overflow` とし、path をその要素(`…/data/steps/2`)にする。
- 箱の中の文は中央に置く(shapes と同じく、図の中の札には「中央揃え禁止」を当てない)。

### process(段階の流れ)

```yaml
caption: 利用者が使い始めるまでの5段階     # 任意。図の上の短い説明
numbered: true                           # 任意(既定 true)。題の前に 1. 2. …
steps:
  - {title: 申し込み, detail: [窓口で受付, 1営業日]}
  - {title: 導入, detail: 平均5営業日, emphasis: true}
  - {title: 本番, status: support}
```

- 段階は左から右へ角丸の箱で並べ、右の辺の真ん中から次の箱の左の辺の真ん中へ、矢印の
  コネクタで繋ぐ。`detail` は箱の下に、箱の左端に揃えて置く。箱の高さは全部同じ。
- **幅が足りないときは折り返す**(警告ではなく): 箱の幅が `minStepWidth`(88pt)より
  狭くなるなら1行の段階の数を減らし、行の長さを揃える(7段階は 6+1 ではなく 4+3)。
  行の終わりの箱の下の辺から、次の行の始めの箱の上の辺へ、行の間で曲がるコネクタで
  繋ぐ(曲がる高さは上の行の補足より下)。折り返しても文が収まらなければ
  `figure-text-overflow`。
- 箱の幅は `maxStepWidth`(200pt)まで。余りは矢印を長くして、図の幅いっぱいに並べる。
- 「1. 題」が1行に収まらず題だけなら収まるときは、番号の後ろで改行する(題が語の途中で
  折り返さないように)。

### orgchart(組織図)

```yaml
root: {title: 責任者, detail: 架空 太郎, emphasis: true}
units:
  - title: 開発
    members:
      - {title: 基盤, detail: 4名}
      - {title: 画面, detail: 3名, status: conditional}
  - title: 営業                        # members は任意
```

- 3段まで(root・units・members)。親の下の辺の真ん中から子の上の辺の真ん中へ、
  段の間の真ん中で曲がるコネクタで繋ぐ(同じ親の子への線は同じ高さで曲がり、
  横の1本の線に見える)。
- **横の幅の配り方**:3段目の箱はどれも同じ幅。各 unit の列の幅は、その unit の
  members の数(無ければ1)に比例する。3段目の箱の幅は `maxMemberWidth`(120pt)まで、
  2段目は `maxUnitWidth`、root は `maxRootWidth`。余りは列の両側に等しく置く。
  root・unit は列の真ん中に置く。
- **狭い枠**:箱が狭くなると文を折り返して箱が高くなる(縦に積む・横にずらす、と
  いった組み替えはしない)。全幅と右の 60% の列で、箱が枠に収まり重ならず警告も
  出ないことを `TestOrgChartFitsFullAndNarrowColumns` で確かめ、PPTX を描いて目で見た
  (下の「確かめ方」)。

### matrix(評価の表)

```yaml
columns: [コンテナ, 仮想マシン, 専用の機器]
rows:
  - label: 導入の速さ
    cells:
      - {status: support, text: 速い(1日)}
      - {status: conditional, text: 中(3日)}
      - {status: danger}                      # text は任意(丸だけ)
legend: {support: 良い, danger: 悪い}          # 任意
```

- 列の見出しは列の真ん中に、行の見出しは左に揃えて行の縦の真ん中に置く(shapes の
  文字が左上か図形の真ん中にしか置けなかった問題を、図形の文字の揃えの指定
  `ShapeOpts.Align`・`Anchor` で直した)。行の見出しの列は一番長い見出しの幅
  (内側の幅の 30% まで)、残りを列で等しく分ける。
- セルは `card_alt` の角の小さい箱に、状態の色の丸と文(左揃え・縦の真ん中)。
- **凡例は自動**:使った状態を support・conditional・none・danger の順に、セルの1列目の
  左端から並べる。文は `legend` に書いたもの、無ければ既定の「対応・条件つき・非対応・
  問題あり」。`legend` に書いても使っていない状態は出さない。
- 各行の `cells` の数は `columns` の数と同じ(Schema では書けないので check の
  `matrix-cells`。path は `…/rows/N/cells`)。セルの `status` は必須(評価の無い表は
  第1層の `table`。`docs/patterns.md` の2.1・R6)。

### funnel(減っていく段階)

```yaml
unit: 社                                       # 任意。数の値の後ろに付ける
stages:
  - {label: 説明会の参加, value: 400, note: 展示会と紹介が主な入口}
  - {label: 環境の確認, value: 60}
  - {label: 契約, value: 10, emphasis: true}
  - {label: 見込み, value: 約 5}               # 値は文字列でもよい
```

- **帯の幅は値に比例させない**:一番上を帯の列の幅いっぱい、一番下を
  `minWidthFraction`(36%)にして、等しい刻みで狭める。値は帯の中に太字で書く
  (数は3桁ごとに区切り、`unit` を付ける)。比例させると小さい段が読めない細さになり、
  文の幅で下駄を履かせると比例が嘘になるため。数の大きさを見せたいなら数のグラフ
  (`chart`、`docs/patterns.md` の R4)。
- 帯は中の文が収まる幅より狭くせず、上の帯より広くもしない。帯は帯の列の真ん中に置く。
- `note` は帯の右の列(内側の幅の 36%)に、その段の縦の真ん中に揃えて左揃えで置く。
- 数の値が前の段より増えたら警告 `funnel-increase`(funnel は減っていく段階に使う。
  減らない手順は `process`)。

### 確かめ方

- `go test ./...`(`figures_test.go`):例を dark・light で組んで警告が無い・コネクタの
  数と接続・凡例の文・見出しの揃え/組織図が全幅と 60% の列で枠に収まり重ならない/
  process が 500pt の幅で 4+3 に折り返し、曲がるコネクタが補足の下で曲がる/
  Min ≤ Natural ≤ Max、狭いほど高い/収まらない文と増える数の警告と check の code・path/
  読み込みと Markdown の誤り/`schema` が4つを名乗る/寸法が段から外れると誤り。
- 目で見る:`BOXLOOM_FIGURE_SHOTS=<dir> go test -run 'TestOrgChartFits|TestProcessWraps' .` が
  試験の PPTX(組織図の全幅・右 60%、折り返した process)を `<dir>` に書くので、
  `soffice --headless --convert-to pdf` と `pdftoppm -r 60 -png` で絵にして見る。
  IR にはまだ段組み(columns)が無いので、60% の列は試験の中で Row を組んで作る。
- showcase の4〜7枚目を shapes で手で置いた版と並べて見て、見劣りしないことを確かめた
  (LibreOffice)。

### まだやっていないこと

- PowerPoint で開いて、曲がるコネクタが箱に付いてくることは確かめていない
  (LibreOffice では回した `bentConnector3` が正しい道筋で描かれる)。
- process は横向きだけ(縦に並べる形は無い)。8段階以上でも警告しない(R7 は分けるよう
  勧める)。
- orgchart は3段まで。members が多くて箱が狭くなりすぎるとき、縦に積む形に組み替えない
  (文を折り返して高くなるだけ。語が収まらなければ警告)。
- matrix の状態は色の丸だけで見分ける(形や印での区別は無い)。列の強調(推す案)は無い。
- 状態の色を凡例で説明するのは matrix だけ。process・orgchart・funnel の状態の枠線の
  意味は、作者が注記などで書く。
- 図の寸法の数(箱の高さの最小・自然・上限など)は目安で決めた値。

## 12. 数値のグラフ(chart)

`docs/patterns.md` の R4(数の系列)を受けるサブシステム(Backlog #38)。**PowerPoint の
本物のグラフ**(`c:chartSpace` と、値を持つ埋め込みの xlsx)で出すので、PowerPoint で
グラフとして値を書き換え・種類を変えられる(図形の集まりではない)。実装は
`boxloom/chart.go`(意味・形の選び方・色・大きさ・検査)と `chartxml.go`(OOXML と
xlsx を標準ライブラリだけで書く)、Markdown は `md_chart.go`、Schema は
`schema/subsystems/chart.schema.json`、寸法は `boxloom/subsystems/chart.json`。例は
`examples/md/charts.bx.md`(↔ `examples/ir/charts.json`)。見た目は
`verification-screenshots/charts_light.png`・`charts_dark.png`(LibreOffice)。

### 作者が書くもの:比べ方と数だけ

```yaml
compare: item              # 比べ方(必須)。下の表
unit: 点                   # 任意。値の後ろに付く単位(目盛りと値のラベルに出る)
categories: [検索, 通知, 共有, 設定, 印刷]
emphasis: [印刷]           # 任意。目立たせる項目(1系列のとき)
series:
  - name: 満足度
    values: [4.5, 4.1, 3.8, 3.2, 2.6]   # 項目ごとに1つ
```

```yaml
compare: correlation       # 相関だけは項目を持たず、軸と点で書く
xAxis: {title: 1件あたりの作業時間, unit: 分}
yAxis: {title: 誤りの数, unit: 件}
series:
  - name: 担当者
    points: [[1, 9], [2, 8], [3, 6]]
```

グラフの**形は書けない**。作者は Zelazny の比較の5分類(`docs/patterns.md` の3.4)の
どれかを `compare` に書き、形はサブシステムが比べ方とデータの数から選ぶ。形は見た目で
あって意味ではない(同じ「時系列」でも点が多ければ折れ線が読みやすい)ので、色・大きさと
同じくテンプレートの側に置いた(Factory を差し替えれば形の選び方も変えられる)。

| compare | 意味 | 形 |
|---|---|---|
| `item` | 項目どうしの大小(順位) | 横棒(上から書いた順)。`stacked: true` なら積み上げ横棒 |
| `timeSeries` | 時点の順の変化 | 1系列で点が `columnMaxPoints`(8)以下なら縦棒、それ以外は折れ線。`stacked: true` なら積み上げ縦棒 |
| `component` | 全体に占める割合 | 円(1系列・部分は6つまで) |
| `frequency` | 区間ごとの度数の分布 | ヒストグラム(棒の間を詰めた縦棒、1系列) |
| `correlation` | 2つの量の関係 | 散布図(軸の題に `xAxis`・`yAxis` の題と単位) |

- **系列は3つまで**(色を見分けられる数。4つ以上は誤り。分けるか表にする)。
- **強調**:1系列なら `emphasis: [項目の名前]` で項目を、複数の系列なら目立たせる系列に
  `emphasis: true`(1つまで)。項目と系列を両方強調するのは誤り。
- **数の書式**:値の小数の桁(2桁まで)と単位から Excel の書式を作る
  (`#,##0.0"点"` など)。単位 `%` は割合ではなく書いた数をそのまま出す(46 → 46%)。
  散布図の単位は軸の題に `(分)` の形で付ける。
- **埋め込みの表**:A 列に項目、1行目に系列の名前、B 列から値(散布図は系列ごとに
  X と Y の2列)。PowerPoint・LibreOffice の「データの編集」でこの表が開く。

### 色(テーマの役割名)

- 系列:強調した系列(無ければ1つ目)が `accent`、ほかは順に `text_muted`・`none`。
- 1系列で項目を強調:強調した項目が `accent`、ほかは `none`(灰色の中の1本)。
  強調が無ければ全部 `accent`。
- 円:強調した扇(無ければ1つ目の扇)が `accent`、ほかは `none` と `text_muted` を交互に。
  扇の境の線は背景の色(`bg`)。状態の色(support・danger)は意味を持つので使わない。
- 文字は `text_muted`、目盛りの線・軸は `grid`。字の大きさは values の `subPoint`。
  字体はテンプレートの字体(`a:latin`・`a:ea`)をグラフの文字にも書く。

### 大きさ(Measure)と、はみ出しの検査の扱い

**軸・目盛り・ラベル・凡例をグラフの中のどこに置くかはアプリが決める。** Boxloom は
グラフの枠の大きさだけを決め、中の置き方は測らない。代わりに、枠の大きさの申告と、
アプリが置けなくなる場合の警告を、字体の寸法で測って出す:

- **最小(Min)**:縦の形は「値の軸の最小の長さ `minPlotHeight`(96pt)+項目の名前の行
  (`labelMaxLines` まで)+値のラベルの行+凡例の行+内側の余白」。円は直径の最小に
  上下のラベル2行ずつ、散布図は目盛りと軸の題の行を足す。横棒は項目の数×1項目の高さ
  (`minBarSlot`、名前が折り返すならその高さ)。
- **自然(Natural)**:縦の形・円・散布図は幅×`naturalAspect`(0.5)、横棒は項目の数×
  `naturalBarSlot`。**上限(Max)**は横棒だけ(`maxBarSlot`。棒を太らせても読みやすく
  ならない)。ほかは上限なし(大きくして困らない)。
- この Min を使って、本文の `layout-overflow`・容量(`check -capacity`)・`subsystem-min`
  が、ほかのブロックと同じに働く(グラフだけ特別扱いしない)。グラフを縮めて文字を
  詰めることはしない。
- **`chart-labels`(警告)**:項目の名前が、アプリが置く場所(縦の形は1項目の幅の 90%、
  横棒は枠の幅の `barLabelFraction`(30%))に、`labelMaxLines`(2)行で収まらない
  (折り返せない語が広すぎるか、行が多すぎる)とき。アプリは名前を斜めにする・間引く・
  何行にも折るので読めなくなる。最初の1つを指し(path は `…/data/categories/N`)、
  残りの数を文に添える。
- 値のラベルの重なり・凡例の折り返し・目盛りの刻みはアプリまかせで、検査しない。

### アプリごとの見え方(試作で確かめたこと)

本実装の前に、6つの形(縦棒の強調・折れ線2系列・円・横棒・積み上げ縦棒・散布図)の
試作を作って開いた(`verification-screenshots/chart_prototype_light.png`・`_dark.png`、
Google Slides で PDF にしたもの `chart-prototype-light-googleslide.pdf`)。

| アプリ | 結果 |
|---|---|
| LibreOffice 24(描画・PDF) | 6つとも正しく描く。強調の色・横棒の順と値の軸の位置・円の名前と値の2行・凡例も合う。ODP に変換すると値の表を持つ本物のグラフとして読む |
| Google Slides(人が開いて PDF にした) | 6つとも崩れない。強調の色・横棒の向き・値のラベル・凡例も合う。字体は Google の字体に置き換わる。**円の一番小さい扇(9%)のラベルから項目の名前が落ちた**(値だけ残る) |
| python-pptx | 種類・項目・値を読める |
| PowerPoint | まだ確かめていない |

- 円の小さい扇の名前が Google Slides で落ちることへの手当ては、まだしていない
  (部分を6つまでにし、小さいものは「その他」にまとめる、を R4 と Skill で勧める)。
- XSD の検査(`tools/validate_pptx.py`)はグラフの部品(`dml-chart.xsd`)と、埋め込みの
  xlsx の中(`sml.xsd`、場所は `/ppt/embeddings/….xlsx!/xl/…`)まで見る。グラフに
  埋め込みの表への関係(package)がちょうど1つあることも見る。

### 確かめ方

- `go test ./...`(`chart_test.go`):比べ方とデータの数から形が決まる/例を dark・light
  で組んで警告が無く、グラフ・関係・xlsx が揃い、強調した項目だけが accent・横棒の順・
  単位つきの書式・凡例の有無・積み上げ・円の境の色・ヒストグラムの間・散布図の軸の題・
  xlsx の値/横棒の高さは項目の数で増え上限があり、ほかは幅×0.5/`chart-labels` と
  check の code・path/意味の誤り(`chart-values`・`chart-emphasis`)を check と読み込みが
  同じに断る/Markdown の誤りの行。
- 目で見る:`BOXLOOM_CHART_SHOTS=<dir> go test -run TestChartsExampleBuilds .` が
  `charts-{light,dark}.pptx` を `<dir>` に書く。
- `tools/validate_pptx.py --self-test` が例を XSD で検査し、グラフの XML・埋め込みの
  表を壊したもの・表への関係を消したものを見つけられることを確かめる。

### まだやっていないこと

- PowerPoint で開いて確かめていない(「データの編集」で表が開くことも)。
- 値の軸の範囲・刻みはアプリまかせ(0 から始まらない軸・対数の軸・2つの値の軸は無い)。
- 積み上げの100%(割合の積み上げ)・面グラフ・ウォーターフォール・2つの軸の組み合わせは無い。
- 値のラベルは1系列の棒と円だけ(折れ線・積み上げには付けない)。
- 円の小さい扇のラベルの手当て(上の表)。

## 確かめたこと

- 載せ替えの前後で、デモの PPTX 2枚(dark / light)の中身が完全に同じ
  (全ファイルを展開して `diff -r`)。fill-ratio は両方 100.0% OK。
- `go test ./...`:自然な高さをもらう/足りないときテキストより先に最小まで
  縮む/最小を下回ると警告/grow で伸びる/Gantt の申告(行が減ると低くなる)/
  デモのスライドで警告が出ない/登録簿の差し替えと誤り/テーマに無い役割名、を
  確かめる試験が通る。
- `go list -m all` は自モジュールだけ(依存ゼロのまま)。

## まだ決めていないこと・やっていないこと

- **Python 参照版は変えていない**:API(Subsystem interface)は Go 版だけ。
  出力は同じ。
- **Gantt の申告の数字**(自然な行の高さ 26pt、日付軸の自然な幅 560pt・
  最小 240pt)は目安で決めた値。とくに幅の最小は目盛りの文字の重なりを実際に
  測っていない。値は `subsystems/gantt.json`(既定)にあり、テンプレートは
  `subsystems.gantt` で差し替えられる(`docs/template-values.md`)。
- 横に並べたとき(Row の中)のサブシステムの幅の申告は、試験の
  `fixedSub`・`aspectSub` でしか確かめていない(Gantt を横に並べる例が無い)。
- Gantt の区間の文を帯の外に出すことはしない(置けなければ警告するだけ。11節)。
- サブシステムが入れ子の Box を中に持つ(サブシステムの中で outer engine を
  呼ぶ)形は扱っていない。
- 注釈の箱の大きさの上限(幅 240pt)を超える長い文は、高さで受けるだけで
  上限が無い。とても長い文はレーンが高くなりチャートの本体を圧迫する
  (そのときは最小の大きさの検査が警告を出す)。
- 手で置いた注釈(Direction が up/down)の箱は固定の大きさのまま。3行を超える
  文ははみ出す。
- 区間を平方根で縮める軸は、目盛りの文字が重ならないことを保証しない
  (目盛りの間が 64pt より狭くなると重なる)。
- shapes: コネクタは辺の真ん中を結ぶ直線だけ(曲がる線・接続点の指定は無い)。線や
  コネクタが図形の上を横切っても警告しない(重なりの判定は図形どうしだけ)。
  破線・線の太さ・文字の揃えは作者が選べない(太さは `subsystems/shapes.json`)。
  `variant` は名前を受け取るだけで、dark / light は変種を持たない。
- shapes の LibreOffice での見た目は確かめた(`verification-screenshots/shapes_*.png`)が、
  PowerPoint で開いて図形を動かしたときにコネクタが付いてくることは確かめていない。
