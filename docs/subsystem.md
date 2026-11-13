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
- **最小でも足りないとき**:列(縦に積む箱)の中では、子は縮めずに要る高さ
  (文字は折り返した高さ、サブシステムは Min)を割りません。割る子をそこで止めて
  残りの子で分け直し、全員が止まれば次の子を下へ押し出します(#133。割って描くと
  図形が枠を越えて次のブロックに重なるため)。本文の箱(骨組みの枠)だけは下限を
  持たず、中のブロックが本文の箱の外へ出た高さは `layout-overflow` の pt と一致します
  (build は誤り、#124)。行(横)の中では従来どおり比例して縮み、Min を割れば検査
  (`lintSubsystemMin`)が `[SUBSYSTEM WARNING] gantt: needs at least …pt width`
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
  `process`・`orgchart`・`matrix`・`funnel`・`cycle`・`pillars`、12節の数値のグラフ `chart`)を持ちます。
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
| 目盛りの文字 | `at` を値の表(`subsystems/gantt.json`)の `dateFormat` で書く(既定 `YYYY/M` → `2029/12`。`YY/M`・`YYYY年M月`・`YYYY-MM` なども書ける。年は YYYY か YY、月は M か MM で、月は必須)。節目の文字の頭の日付も同じ。範囲の両端に目盛りが無ければ日付だけの目盛りを足す |
| 状態の無い目盛りの色 | `text_primary`(両端に足したものは `text_muted`) |
| 節目の文字 | `2028/5 ライセンス更新` のように日付を前に付ける |
| 節目の段 | 下の「節目の段」 |
| 注釈の上下と横のずらし | 下の「注釈の置き場所」 |
| 確定していない区間(`tentative`) | 塗らずに、状態の色の破線の枠で描く |

**節目の段**(Issue #51、`planMilestones`):節目の文字(`★ 日付 名前`)の幅を実測し、
左から順に、横に重ならない(隙間 `milestoneLabelGap` 6pt)一番下の段へ置く。段は2つまで
(`ganttMilestoneMaxRows`)で、軸に一番近い段から埋める。日付が近い節目が2つなら
段をずらして並べ、そのぶん節目の帯が1段高くなる(Measure の自然な高さも同じだけ
増えるので、はみ出しの計測と描画はずれない)。2段のどちらにも空きが無い節目は、
右端が一番左にある段に重ねて置き、警告 `milestone-overlap` を出す(path はその節目
`…/data/milestones/N`)。前は節目をすべて1段に置き、近い日付の文字が重なっても
何も言わなかった。例は `examples/md/warnings/milestone-overlap.bx.md`
(↔ `examples/ir/check-warnings/milestone-overlap.json`)、試験は
`gantt_milestone_test.go`。

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
(字の幅 × 行の高さ。文字の箱そのものではない)を `leaderGap`(2pt)広げ、線のうちその中を
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
隙間 `barInset` 6pt)まで高くし、その分ほかの行を低くします(本体の高さの合計は同じ)。
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
いまは、帯の中で文に使える幅(帯の幅 − 文の左右の余白 `segmentLabelInsetLeft` 5pt・
`segmentLabelInsetRight` 3pt)で、次の順に決めます
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
作らないための目安(行の数で寸法ではないので、値の表には出さず名前つきの定数
`ganttSegLabelMaxLines` にした。13節)。

1行で収まる文の描き方は以前と同じなので、従来のデモの PPTX は変わらない。例は
`examples/md/warnings/gantt-segment-label.bx.md`(↔ `examples/ir/check-warnings/gantt-segment-label.json`)。

### 11.1 Gantt の月の格子・山形の帯・高さを使い切る(Backlog #89)

読む資料のスケジュール(年と月の2段の見出し・月ごとの縦の罫線・右に尖った山形の帯・
ページの高さを使い切る図)を描けるように、テンプレートの値の表の `subsystems.gantt` に
3つの選び方を足した。既定(`subsystems/gantt.json`)は従来の描き方で、dark・light の
Gantt の出力は変わらない。report は `monthGrid`・`chevron`・`fillHeight: true`。
コア(配置のエンジン)は変えていない。

| キー | 値 | 描き方 |
|---|---|---|
| `axisStyle` | `ticks`(既定) | 書いた目盛りだけの軸(従来) |
| | `monthGrid` | span の月ごとに列を作り、上に年の行・月の行の2段を置き、月の境目に縦の細い罫線(`grid`)を引く。年の境目の罫線は年の行から。日付の軸は **月ごとに等しい幅**(月の中は日の数に比べる。書いた目盛りでの縮めはしない)。書いた目盛りは名前を格子の上の行に置き、点と本体の破線(目盛りの状態の色)を重ねる。日付だけの目盛り(範囲の両端に足したもの)は格子が示すので描かない。節目は従来どおり上に置く |
| `barShape` | `rect`(既定) | 角の丸い矩形(従来) |
| | `chevron` | 右に尖った山形(`homePlate`)。面は状態の色を `chevronFillOpacity` の不透明度(`a:alpha`)で薄くし、文は字の色(`text_primary`)。仮の帯は山形の破線の枠。凡例の色見本も同じ形と薄さ(risk は従来の矩形) |
| `fillHeight` | `false`(既定) | Measure の Max は `maxRowHeight` の行で止まる(従来) |
| | `true` | Max を 0(上限なし)にし、本文の領域の高さまで伸びる。自然な高さより余った分は **注記の行を除く行** に等しく配る(注記の行は自然な高さ)。行は `maxRowHeight` で止まり、越える分は行の下に残る(格子の罫線は最後の行で止める) |

**長い span**:月を等しい幅にすると月の文字(`gridMonthFormat`)が1つでも列に入らない
とき(例 `gantt-full` の8年)は、月の行と月の罫線を描かず、日付の軸も書いたとおり
(目盛りで区切って長い区間を縮めたもの)にして、年の行と年の境目だけを描く。細い列に
罫線と字を詰めないため。Measure・描画・Lint は同じ判定(`gridShowMonths`)を使う。

**山形の面の色**:状態の色(役割名)のまま不透明度だけを下げる。状態の意味(凡例)を
保ったまま、読む資料の「薄い基調色の面に字の色」の見た目にするため(16進の色を
サブシステムで作らない)。

**帯の上の文の色**(Backlog #97):帯の見えている地(塗った帯は状態の色、山形は状態の色を
`chevronFillOpacity` で `bg` に重ねた色、仮の帯は塗りが無いので `bg`)に対して読める役割を
1か所の規則(`readableRole`。`gantt.go` の `ganttBarLabelRole`)で選ぶ。従来の役割(塗った帯は
`text_primary`、不透明な `conditional` は on 系の `on_conditional`、仮の帯は状態の色)が
コントラスト比 4.5 に届けばそのまま使い、届かなければ `text_primary`・`on_conditional`・
`bg`・状態の色のうち比が最も大きいもの。値の表の on 系の役割はいま `on_conditional` だけ
(テーマの鍵は決まった13個)。これで出力が変わったのは 4.5 に届いていなかった組だけ:
dark の support・conditional・danger の帯(白系 → `bg` の濃い字)、light の danger の帯
(→ `on_conditional`)、report の danger の山形(→ 状態の色)、各テンプレートで薄い状態の
仮の帯の文(→ `text_primary`)。確かめ方:`gantt_bar_contrast_test.go`(3つのテンプレート・
4つの状態・塗った帯と仮の帯で、描いた文と地のコントラスト比が 4.5 以上)。

確かめ方:`gantt_month_grid_test.go`(架空の8か月:月の列の数と幅・年の境目・山形の帯・
report で本文の高さを使い切ること・注記の行の高さ・dark/light が従来どおり・長い span)。

## 11. 図のサブシステム(process・orgchart・matrix・funnel・phases)

`docs/patterns.md` の語彙の第3層のうち、**箱と線で描く図**(Backlog #24〜#27、phases は #90)。
shapes で手で置いていた形(箱を矢印で並べる・親と子の箱・格子に評価・減っていく帯)を、
意味だけのデータから描く。実装は `boxloom/process.go`・`orgchart.go`・`matrix.go`・
`funnel.go`・`phases.go`(共通の部分は `figures.go`、Markdown は `md_figures.go`)、Schema は
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
- **状態の凡例**:状態の枠線の色が何を意味するかは図ごとに違うので、作者の言葉で
  `legend: {conditional: 増員が要る, …}`(状態の名前 → 文)に書く。process・orgchart・
  funnel は、使った状態のうち `legend` に文のあるものを、図の下(内側の左端から)に
  「箱と同じ形の見本(地の塗り+状態の色の枠線)+文」で並べる(幅を越えたら次の行)。
  使ったのに文の無い状態は凡例に出さず、警告 `[SUBSYSTEM WARNING] process: legend:
  status "conditional" is used but has no legend text …` を出す(check の `figure-legend`、
  path は `…/data/legend`、.md なら legend の行か図の行)。強調(`emphasis`)は「ここを見て」
  の印なので凡例に出さない。matrix は評価の既定の文があるので、文が無くても既定の文で
  凡例を出す(下の matrix)。凡例の寸法は各図の値の表の `legendGap`・`legendItemGap`・
  `legendSwatchWidth`・`legendSwatchHeight`・`legendSwatchGap`、文は `small`。
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
legend: {support: 保守の係に引き継ぐ}      # status を使ったら書く(共通の決まり)
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
    assistant: {title: 副部長, detail: 架空 次郎}   # 任意。横の補佐(1つまで)
legend: {conditional: 2名を増やす}
```

- 3段まで(root・units・members)。親の下の辺の真ん中から子の上の辺の真ん中へ、
  段の間の真ん中で曲がるコネクタで繋ぐ(同じ親の子への線は同じ高さで曲がり、
  横の1本の線に見える)。
- **横の幅の配り方**:3段目の箱はどれも同じ幅。各 unit の列の幅は、その unit の
  members の数(無ければ1)に比例する。3段目の箱の幅は `maxMemberWidth`(120pt)まで、
  2段目は `maxUnitWidth`、root は `maxRootWidth`。余りは列の両側に等しく置く。
  root・unit は列の真ん中に置く。
- **泣き別れを避ける(Backlog #56)**:箱の文(題・補足の段落)が2行以上に折り返し、
  最後の行が2文字以下(空白を除く)になるとき(「欧州 AI 委員 / 会」)は、列の両側の
  余りの中で箱を 1pt ずつ広げ、泣き別れの段落が一番少なくなる一番狭い幅にする
  (`nodeComfortWidth`)。root は枠の内側の幅まで(このときだけ `maxRootWidth` を越えて
  よい)、2段目は列の幅＋両側の余りまで(上限 `maxUnitWidth`)、3段目は
  列の箱を揃えて同じ幅のまま、列の余りを箱の数で分けた分まで広げる(このときだけ
  `maxMemberWidth` を越えてよい)。列の真ん中は動かさないので、隣の列との間は
  `unitGap`、3段目の箱の間は `memberGap` のまま。広げても直らない(余りが足りない)
  ときは元の幅のままで、警告は出さない(文は収まっている)。泣き別れが無い図は
  前と同じ置き方。`TestOrgChartUnitAvoidsOrphanLine`・`TestNodeComfortWidth` で確かめる。
- **補佐(Backlog #131)**:unit の任意の `assistant: {title, detail}`(1つまで。状態・
  強調は持たない)は、2段目の箱の右に `assistantGap` を空けて小さい箱(幅
  `maxAssistantWidth`、高さは文の高さ・最小 `minBoxHeight`、2段目の箱の高さの真ん中)を
  置き、2段目の箱の右の辺から補佐の左の辺へ破線の本物のコネクタ(`a:prstDash`)で繋ぐ。
  2段目の箱は列の真ん中のままなので、root からの線と3段目の並びは変わらない。補佐が
  列の右半分と余りに入りきらなければ、まず2段目の箱を、それでも足りなければ両方を狭める
  (隣の列との間は `unitGap` のまま)。補佐の文の高さは2段目の段の高さに含め、Measure の
  幅は補佐1つにつき列の両側に「`assistantGap`+幅」を足す(自然は `maxAssistantWidth`、
  Min は `minAssistantWidth`)。補佐の文が収まらなければ `units[i].assistant` の
  `figure-text-overflow`。例は `examples/md/orgchart-assistant.bx.md`、試験は
  `TestOrgChartAssistant`。
- **狭い枠**:箱が狭くなると文を折り返して箱が高くなる。全幅と右の 60% の列で、箱が
  枠に収まり重ならず警告も出ないことを `TestOrgChartFitsFullAndNarrowColumns` で確かめ、
  PPTX を描いて目で見た(下の「確かめ方」)。
- **3段目を縦に積む形(Backlog #134)**:幅が足りず3段目を横に並べると箱が狭くて文が
  何行にも折れるとき(再現の第6回の p33:2単位の組織図を幅約 420pt の列に置くと Min が
  405pt)、3段目を親の下に字下げした縦の並びに切り替える。各 unit の列は同じ幅で、
  2段目の箱(列の真ん中、幅は `maxUnitWidth` まで)の下に、3段目の箱を `stackIndent`(24)
  だけ字下げした幅(2段目の箱の幅−字下げ)で `memberGap`(12)を空けて積む。2段目の箱の下の辺から
  字下げの真ん中に縦の線を下ろし、そこから各箱の左の辺へ横の線(箱の左の辺に繋がった
  本物のコネクタ。縦の線は箱に繋がらない線)。root から2段目への線はそのまま。
  **形の選び方**:`geom` は幅ごとに両方の形を組み、Min の高さ(各段の箱の文が収まる高さと
  間の和)が縦に積む形のほうが低いときだけ切り替える(同じなら横に並べる形)。Measure は
  選んだ形の自然・最小・上限を申告するので、Min が下がり、行の中で最小を割る警告
  (`subsystem-min`)も出ない。形の選択はサブシステムの仕事で、切り替えたことは警告にしない。
  広い枠では横に並べる形のほうが低いので、従来の出力のまま(golden は変わらない)。
  補佐(#131)があれば2段目の箱は補佐の分だけ狭まり、3段目もその幅で積む。試験は
  `TestOrgChartNarrowStacksMembers`(420pt で Min が横並びより低く、描いた図形と線が枠に
  収まり、箱が重ならず、912pt では横並びのまま)と `TestOrgChartNarrowInColumn`(列の中)。

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
emphasis: コンテナ                             # 任意。推す列(列の見出しの文)
```

行の題の下の説明と、列の見出しの上の列の群(どちらも任意):

```yaml
columns: [東, 西, 北]
columnGroups:                                  # 左から。span の合計は列の数
  - {title: 本州, span: 2}
  - {title: 北海道, span: 1}
rows:
  - label: 人手の不足
    detail: [採用が遅れる, 退職が続く]           # 文の列は箇条、1つの文なら段落
    cells:
      - {status: support, text: p.35}
      - {status: none}
      - {status: support}
```

- 列の見出しは列の真ん中に、行の見出しは左に揃えて行の縦の真ん中に置く(shapes の
  文字が左上か図形の真ん中にしか置けなかった問題を、図形の文字の揃えの指定
  `ShapeOpts.Align`・`Anchor` で直した)。行の見出しの列は一番長い見出しの幅
  (内側の幅の 30% まで)、残りを列で等しく分ける。
- セルは `card_alt` の角の小さい箱に、状態の色の丸と文(左揃え・縦の真ん中)。
  **丸はセルの図形の文字の段落の印**(`a:buChar` の ●、`a:buClr` に状態の色、大きさは
  `a:buSzPct`)で、セルの上に別の丸の図形を重ねない。セルを動かせば丸も付いてくる。
  文は印の幅(丸の直径 `dotSize` と空き `dotGap`)だけ字下げした位置から始まり、折り
  返した2行目も同じ位置に揃う。文の無いセルは ● だけを真ん中に置く(印は文のある
  段落にしか出ないので、幅の無い文字 U+200B を置き、丸が真ん中に来る左の余白を取る)。
  セルごとに `p:sp` は1つ。丸の図形(楕円)は凡例の分だけ。
- **セルの描き方(`cellStyle`)**:値の表の `subsystems.matrix.cellStyle` で選ぶ。上は
  `filled`(既定。dark・light)。`mark`(report)はセルの面を塗らず、support は状態の色の
  丸(直径 `markSize`)に白いチェック(`bg` の色の2本の線)、none は何も描かない
  (当てはまらないセルは空ける。凡例にも none を出さない)、conditional・danger は丸を
  その状態の色で塗る。セルの文は丸の下(空き `markGap`)に `small` の字で真ん中揃え。
  丸と文の塊はセルの縦の真ん中。凡例の印も同じ丸(support はチェックつき)。
- **行の説明(`detail`)**:行の題の下に説明を置く。1つの文なら1つの段落、文の列なら
  1つずつ箇条(印は `marks.point`、アクセントの色)。字は `figureDetail`。行の見出しの
  列の幅は、題と説明の一番長い文(自然な幅)と、列の自然な幅(列の見出しとセルの中身。
  `mark` なら印の幅で足りる)から決める:両方が入ればその自然な幅、入らなければどちらも
  折れない塊(欧文の語・和文の4字。表の列と同じ規則)の幅を先に取り、残りを自然な幅との
  差の比で配る(行の見出しは自然な幅と塊の幅の差が大きいので先に縮み、題が折り返す)。
  塊の幅の和も入らなければ、行の見出しの列は最小(塊の幅)のまま、評価の列を最小(印と
  一番長い語が入る幅)まで縮める。両方の最小も入らないときだけ最小の比で配り、文が横に
  はみ出す(`figure-text-overflow`)。以前は塊の幅が入らないとすぐ自然な幅の比に切り替え、
  下限を数 pt 割っただけで評価の列が 4pt ほどに崩れた(#138)。Measure の Min の幅は
  この両方の最小が入る幅(値の表の `minWidth` より狭くはしない)。どの場合も内側の幅の
  `maxRowHeaderFraction`(既定 0.5、report は 0.6)まで(#106)。
- **説明の列(`detailColumn`、#143)**:`detailColumn: 見出しの文` を書くと、各行の
  `detail` を題の下でなく、題の右の見出しつきの列(見出しは左揃え)に置く。行の高さは
  題・説明・セルのうち高いもので決まるので、説明を題の下に積むより行が低くなる(元の
  資料の p28 を列の中に置いたとき、題の下だと Min の高さが 992pt になっていた)。
  幅は文の列(行の題・説明)が取り、評価の列は中身の自然な幅(印・短い文・折り返した列の
  見出し)の最小に留める(#144)。評価の列(印と一番長い語の幅から自然な幅まで)と題の列
  (塊の幅から自然な幅まで)の幅を 2pt の刻みで探し、見出しの行・列の群・各行の最小の高さの
  合計が一番小さい組を選ぶ(pairs の #115 と同じ考え)。高さが同じなら評価の列が狭い方、次に
  題と説明の幅の比が文の量の比に近い方。Measure・place・Lint は同じ幅で測るので、Natural・Min
  の高さは描く形と一致する。#143 は評価の列に自然な幅(1列 47〜57pt)を取らせたので、列の中
  (幅 500〜555pt)では題と説明の列が全角5〜7字に潰れ、題の下に置くより行が高くなっていた。
  評価の列の幅は中身(印・短い文)で決め、列の見出しは折り返す(和文は字の境、3行まで。見出しの
  行の高さは折り返した行の数で測る。見出しが3行に入らないときだけ3行に入る幅が最小)。評価の列の
  左右の余白は `textInsetY` と同じ小さい余白(#151。以前は見出しを折らない幅と `textInsetX` の
  余白で1列 48〜51pt を取り、題と要約の列が狭くなった。全角3字の見出しと「p.35」の列は report で
  約 35pt)。Measure の Min の幅もこの最小で測る。
  最小も入らなければ最小の比で配る。題は `maxRowHeaderFraction` まで。凡例は説明の列の左端から。
  説明が収まらなければ `figure-text-overflow`(`rows[N].detail`)。書かなければ従来どおり
  題の下(出力は変わらない)。

  ```yaml
  detailColumn: 要約
  columns: [案 A, 案 B, 案 C]
  rows:
    - label: 費用
      detail: 初年度の持ち出しと、毎年かかる保守の費用を合わせて比べる
      cells: [{status: support, text: 少}, {status: conditional}, {status: danger, text: 多}]
    - label: 止まる時間
      detail: [夜間に切り替える, 週末の作業は避ける]
      cells: [{status: support}, {status: support}, {status: danger}]
  ```
- **列の群(`columnGroups`)**:列の見出しの上の行に、`span` 列(と間)の幅で群の名前を
  太字で真ん中に置き、下に `grid` の色の罫線(太さ `groupRuleWidth`)を引く。群は左から
  順に列を埋め、`span` の合計は列の数と同じ(違えば読み込みの誤りで、check は
  `matrix-column-groups`。path は `…/data/columnGroups`)。群を書かなければ行は無い。
- **matrix は図のまま(普通の表は `table`)**:matrix を本物の表(`a:tbl`)にするかを
  検討し、図のままにすると決めた。見出し・セルの間の空き・角丸・推す列のアクセントの枠は
  表のセルでは書けず、評価の無い普通の表は第1層の `table` ブロックで本物の表として
  書けるため。
- **凡例は自動**:使った状態を support・conditional・none・danger の順に、セルの1列目の
  左端から並べる。文は `legend` に書いたもの、無ければ既定の「対応・条件つき・非対応・
  問題あり」。`legend` に書いても使っていない状態は出さない。
- **推す列**:`emphasis` に列の見出しの文(文中の記法を除いた文字)を書くと、その列の
  見出しを太字のアクセントの色にし、見出しから最後の行までをアクセントの色の枠
  (塗らない。太さ `emphasisLineWidth`、セルとセルの間の真ん中)で囲む。見出しに無い
  名前は読み込みの誤りで、check は `matrix-emphasis`(path は `…/data/emphasis`)。
- **行の高さ**:1行の文に見合う高さで止める(`naturalRowHeight` 36・`maxRowHeight` 44。
  文が折り返せばその高さ)。上限より大きい枠は余りを空きとして残す(9節)。前は上限が
  64pt で、1行の文のセルが間延びしていた。
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
legend: {conditional: 人手で詰まる段}          # status を使ったら書く
```

- **帯の幅は値に比例させない**:一番上を帯の列の幅いっぱい、一番下を
  `minWidthFraction`(36%)にして、等しい刻みで狭める。値は帯の中に太字で書く
  (数は3桁ごとに区切り、`unit` を付ける)。比例させると小さい段が読めない細さになり、
  文の幅で下駄を履かせると比例が嘘になるため。数の大きさを見せたいなら数のグラフ
  (`chart`、`docs/patterns.md` の R4)。
- 帯は中の文が収まる幅より狭くせず、上の帯より広くもしない。帯は帯の列の真ん中に置く。
- `note` は帯の右の列(内側の幅の 36%)に、その段の縦の真ん中に揃えて左揃えで置く。
- **段の間の転換率**:前後の段の値が数で、前が 0 より大きく後が前を越えないとき、
  前の段に対する割合を `↓ 45%` の形で、帯の列の右の細い列(帯との間 `rateGap`)に、
  2つの帯の境目の高さに揃えて書く(10% 以上は整数、それより小さければ小数1桁。字は
  `subPoint` の太字、色は `text_muted`)。作者の数から一つに決まる値なので図が書き足す
  (作者が note に転換率を書かなくてよい)。値が文字列の段の前後には書かない。
- 数の値が前の段より増えたら警告 `funnel-increase`(funnel は減っていく段階に使う。
  減らない手順は `process`)。

### phases(段ごとに箱を積む事業の全体像)

```yaml
steps:                                         # 2〜5段。左から右へ
  - id: s1                                     # groups が指す名前(任意)
    title: Step1 現状を知る
    period: 4月〜6月                           # 任意。山形の中、題の下
    items:                                     # 段の下に縦に積む箱(1つ以上)
      - title: 乗降の調べ
        detail: [全 42 路線を数える, 時間帯ごとに分ける]   # 文の列は箇条
      - {title: 他の町の事例, detail: 3 つの町を訪ねる, status: conditional}
  - …
groups:                                        # 任意。段をまたぐ帯(左から順に、重ねない)
  - {title: ① 調べと案づくり, from: s1, to: s2}
legend: {conditional: 追加の検討}               # status を使ったら書く
```

- **段の見出しは山形**:左端の段は五角形の矢印(`homePlate`)、2つ目からは山形
  (`chevron`)。塗りは `accent`、文字は `on_conditional`(題は `body` の太字、期間は
  `figureDetail`)。山形の先は次の段の列に食い込み、次の山形の切り欠きに `chevronGap`
  あけて収まる(先の深さ `chevronDepth` は preset の `adj` に書く。文字の枠は preset の
  il〜ir で、測りも同じ幅 `chevronTextWidth`)。
- **群の帯**:山形の上に、`from` の段の列の左端から `to` の段の列の右端まで。見た目は値の表の
  `bandStyle`:`outline`(既定。塗らない `accent` の枠と `accent` の太字)か `fill`(`accent` の
  塗りに `on_conditional` の太字。report、#110)。字は `subPoint`。id の無い段・前後の逆・
  重なる帯は読み込みの誤り。
- **箱**:段の列の幅いっぱいの角丸の箱に、通しの番号と題(太字、番号は `accent`)・
  補足(`figureDetail`)を1つの図形の段落で書く。補足が文の列なら段落の箇条の印
  (`a:buChar`。印はテンプレートの `marks.point`)。番号は段をまたいで通しで、形は
  値の表の `numberStyle`(`arabic` → 1. 2.、`roman` → I. II.。report は roman)。
  状態・強調の色は共通の決まりのとおり。
- **detail の読み分け**:IR では1つの文(inline)か、文の列(各項目が inline)。列の項目に
  文中の要素(`em` などの object)が直に並ぶなら1つの文、文字列と列だけなら文の列と読む。
  Markdown では YAML の1行が文、YAML の列が文の列(process の detail と違って改行で
  つながない)。
- **高さ**:箱の最小は「列の幅で文を折り返して収まる高さ」、自然・上限は
  `naturalItemHeight`・`maxItemHeight` まで。段ごとに箱の列の和を取り、一番高い段を
  Min・Natural・Max として申告する。割り当てられた高さは段ごとに箱へ配り、上限を
  越えた分も箱に等しく足すので、どの段も箱の列の下の端がそろう。
- **枠が Min より低いとき(#107)**:箱は文の要る高さより低くしない(縮めると箱の文が
  次の箱に重なる)。#106 の「図の枠の外に描かない」とぶつかるときは、**字どうしの重なりを
  作らない方を選ぶ**:群の帯・山形は枠の中のまま、箱の列は最小の高さで描いて
  枠の下へはみ出させ、`layout-overflow`・`subsystem-min` の警告で知らせる(他のブロックと
  同じ。自動では直さない)。箱の文が箱に収まらないとき(幅が足りないなど)は
  `figure-text-overflow`(path は `…/data/steps/N/items/K`)。凡例はこのときだけ図の頭
  (群の帯の上、`legendGap` をあけて)に移り、枠の中に残る。下の注・ブロックの上には
  描かない(#114)。枠が足りていれば凡例は箱の列の下。
- **幅**:Min の幅は `minWidth` と、どの箱・山形も一番長い語を割らずに置ける幅(段の列の
  幅を一番広く要るものにそろえた和)の大きいほう(#106)。

### cycle(輪に並んだ段、#91)

```yaml
start: 0                                       # 任意。番号の始まり(0 か 1、既定 1)
steps:                                         # 3〜6つ。上から時計回り
  - {title: 場を知る, detail: 住民の声を集める}
  - {title: 仲間を集める}
  - {title: 小さく試す, emphasis: true}
  - {title: 形にする, status: conditional}
  - {title: ふり返る}
legend: {conditional: 予算が付くときだけ}      # status を使ったら書く
```

最後の段が最初の段に戻る、繰り返す段階(`docs/patterns.md` の R7b)。実装は
`boxloom/cycle.go`、寸法は `boxloom/subsystems/cycle.json`、例は `examples/md/cycle.bx.md`
(↔ `examples/ir/cycle.json`。単独と columns の中)。

- **扇は本物の図形**:段ごとに `blockArc`(ドーナツの扇)を1つ。調整値 `adj1`・`adj2` が
  始まりと終わりの角度(上から時計回りに等しく分け、境目に `gapAngle` の隙間)、`adj3` が
  輪の太さ(外の半径の `ringFraction`)。どの扇も輪の外接の正方形を枠に持つ。
- **色**:扇の塗りは基調色(`accent`)の濃淡。最初の段が `accent`、後の段ほど地の色
  (`bg`)に寄り、最後の段は `shadeRange`(60%)寄せた色(役割名から混ぜて作るので、
  16進の色はテーマにしか無い)。`emphasis` の段があるときは、強調の段だけ `accent`、ほかは
  `mutedShade`(70%)寄せた同じ淡い色。`status` は扇の枠線の色。扇の中の題の色は、
  `on_conditional` と `text_primary` のうち塗りとのコントラスト比が大きいほう。
- **番号と題**:番号の丸(`numberDiameter`、地の色に扇の色の枠線、`subPoint` の太字)は
  扇の真ん中の角度で外の縁に跨がらせる。題(`subPoint`、強調の段は太字)は扇の中に置く:
  1行・2行…と増やしながら、中心の候補(輪の太さの真ん中から内へ寄せる・扇の向きに回す)
  ごとに、扇の縁・境目・番号の丸から `titleInset` 離れて置ける一番広い横長の枠を測り、
  題がその行の数に収まって最後の行が1〜2字にならない最初の枠を使う。枠は題の折れない塊
  (欧文の語・和文の4字。表の列と同じ規則)より狭くしないので、題が1〜2字ずつ縦に折れない。
  どの枠にも入らなければ、題は塊の幅で扇の真ん中に置き、`figure-text-overflow` の警告にする
  (扇の中に取れた枠の大きさを書く)。輪の大きさを申告するとき(幅も高さも決まっていない
  とき)は、全部の題が扇に入る一番小さい輪(`naturalDiameter` の 1.75 倍まで)を自然の
  大きさにする(#106)。
- **題を輪の外に置く(#110)**:枠から決まる輪で題が扇に入らなければ、題を輪の外(番号の丸の
  外、その段の向き。置き方は補足と同じ)に置き、補足はその下の段落にする。題は
  `text_primary` の太字。輪は `outsideMinDiameter` まで小さくして題の置き場を取り、題の枠は
  折れない塊(和文の4字)まで狭められる。題の枠は輪と番号の丸を囲む正方形に掛からないところ
  まで外へ押し出す。外に置いても枠に入らなければ(最小の輪でもはみ出す)扇の中に置いたまま
  `figure-text-overflow` の警告にする。大きさを申告するとき(幅も高さも決まっていない)は、
  1.75 倍の輪でも題が扇に入らなければ、題を外に置いた `naturalDiameter` の輪を自然の大きさに
  する。扇の中か外かは枠の向きで決まる(高さで測った幅で測り直すと、扇に入る輪に戻ることがある)。
- **補足**:`detail` は輪の外の、その段の向きに置く(右の段は左揃え・左の段は右揃え・
  真上と真下は真ん中。上の段は枠の下端、下の段は上端を輪に向ける)。幅は `labelMaxWidth` まで
  で、枠が狭いと `labelMinWidth` まで折り返す。段ごとの長い説明は図に詰めず、`columns` で
  輪の横のまとまりに書く。
- **大きさ**:形は輪で決まるので縦横比を保つ(`FixedAspect`)。幅を受けたらその幅に収まる
  一番大きい輪の高さ、高さを受けたらその高さでの幅を申告し、最小は題が読める一番小さい形
  (#112):題が直径 `minDiameter` の輪の扇に入ればその輪。入らなければ、題が扇に入る輪と、
  題を輪の外に置いた `outsideMinDiameter` の輪(題は塊まで折り返す)のうち幅の狭いほうの幅、
  高さは読める形のうち一番低いもの(外の題は受けた幅で折り返す)。
- **列の中の幅(#112)**:cycle は並列のまとまりの中で、この最小の幅を「まとまりの幅の下限」
  としても申告する(`layout.go` の `columnFloor`)。まとまりの幅は、下限に届かないまとまりを
  下限で止めながら、残りを grow の比(role=main の重み)で配り直す(止めたら残りで配り直す。
  CSS の flexible lengths の凍結と同じ)。両側の図が下限を持っても、どちらも下限を守り、
  主のまとまりが広いのはその上で効く(#132)。下限の和が伸びるまとまりの幅の和を超えるときは、
  下限を持つまとまりに下限の比で配り(どれも同じ割合だけ足りない)、下限を割ったまとまりごとに
  `column-floor` の警告(path はまとまり `/slides/S/body/N/columns/C`、文に要る幅・有る幅・足りない pt)を
  出す。図は狭いまま描くので、`figure-text-overflow`(と `subsystem-min`)も出る。下限を申告するのは cycle と、
  phases・orgchart(図の Min の幅、#117)、matrix(見出しとセルの文が横にはみ出さない一番狭い幅。
  列の多い matrix が数 pt の列に潰れないように、#132。#138 からは Min の幅が列の数を見て、
  行の題の列と評価の列の最小が入る幅になったので、ほぼ Min の幅)と
  pillars(#121)で、グラフ・画像のまとまりの幅は従来どおり。
  枠が図より広ければ、コアが図の幅まで切り詰め、余りを `spare-width`・`remainingWidth` で
  知らせる(9節)。
- **並列のまとまり(columns)の中に置ける**(chart・cycle・phases・matrix・orgchart・pillars。左に輪・右に段ごとの説明)。
- 警告:題が扇の枠に・補足が枠に収まらない(`figure-text-overflow`、path は
  `…/data/steps/N`)、状態に凡例の文が無い(`figure-legend`)。

### pillars(屋根・柱・土台、#121)

```yaml
roof: だれもが立ち寄る、町の居場所になる      # 目指す姿(屋根の文)
pillars:                                      # 2〜6本。左から右へ
  - {title: 場をひらく, items: [週末に拠点を開く, 催しを続ける], label: "1"}
  - {title: 担い手を育てる, items: [運営を3人で回す]}
  - {title: 声を集める}
foundation: 住民の参加と、町の小さな予算      # 任意。柱を支える前提
```

目指す姿(屋根)を、いくつかの柱(要因)が支え、土台(前提)の上に立つ図
(`docs/patterns.md` の R7c)。位置(上・並ぶ・下)が意味なので、作者は向き・置き場所を
書かない。実装は `boxloom/pillars.go`、寸法は `boxloom/subsystems/pillars.json`、例は
`examples/md/pillars.bx.md`(↔ `examples/ir/pillars.json`。3本・5本と columns の中)。

- **形**:屋根は文を持つ台形(`trapezoid`、上の辺が狭い。斜めの辺の横の長さ `roofSlant` を
  調整値 `adj` に直す)で、図の内側の幅いっぱい。柱は縦長の箱(`rect`)を等しい幅で横に並べ
  (間 `pillarGap`)、屋根・土台より左右に `pillarOverhang` だけ内側。土台は横長の箱(`rect`)で
  図の内側の幅いっぱい。屋根・柱・土台の間は `rowGap`。
- **文**:屋根は `body` の太字、真ん中に置く。台形の文字の枠(prstGeom の rect)は斜めの辺の
  2/3 だけ内側なので、残りの 1/3 を左右の余白に足し、文は上の辺(一番狭いところ)の幅で
  折り返す。柱は題(`body` の太字)と、`items` の箇条(`figureDetail`、印は `marks.point` の
  段落の箇条)を1つの図形に、左上から。土台は `subPoint`、真ん中。
- **柱の名前**:`label`(1〜2字)のある柱は、柱の上に丸(`ellipse`、基調色に `subPoint` の太字)を
  置く。丸の直径は `labelDiameter` を下限に、名前が内接する矩形に入るまで広げ、全ての丸で同じ。
  名前の無い柱には丸を置かない(行の高さはどれかの柱に名前があれば取る)。
- **色**(基調色の系統):屋根と名前の丸は `accent`、柱は `accent` を `bg` に `pillarTint`
  (85%)寄せた色、土台は `foundationTint`(40%)寄せた色。文の色は塗りとのコントラスト比で
  役割名から選ぶ(屋根は `on_conditional` と `text_primary` の比が大きいほう、柱・土台は
  `text_primary` を先に見る)。状態・強調・凡例は無い。
- **大きさ**:屋根・柱・土台はそれぞれ最小・自然・上限の高さ(`roof…Height`・`pillar…Height`・
  `foundation…Height`)を持ち、どれも中の文が収まる高さより低くしない(柱の高さは一番文の多い
  柱にそろえる)。割り当てられた高さは部品の間で配る(11節の flexSizes)。幅の下限は、柱が
  `pillarMinWidth` と柱の文の折れない塊(和文の4字)の広いほう・屋根と土台の文の塊が入る幅、と
  `minWidth` の大きいほう。
- **列の中**:並列のまとまり(columns)の中に置ける(左に図・右に柱ごとの説明)。幅の下限は
  まとまりの幅の下限(`columnFloor`)としても申告する(cycle と同じ)。
- 警告:屋根・柱・土台の文が収まらない(`figure-text-overflow`、path は `…/data/roof`・
  `…/data/pillars/N`・`…/data/foundation`)。高さが足りないときは部品を縮めず、
  `subsystem-min`・`layout-overflow` で知らせる。
- 値の表:どの寸法も 0 より大きい(0 以下は誤り)。`pillarTint`・`foundationTint` は 1 未満、
  高さは最小 ≤ 自然 ≤ 上限(`PillarsValues.check`)。余白の寸法は値の表の段(`spacingUnit`)の上。

### 確かめ方

- `pillars_test.go`(#121):架空の文で 3本・5本の柱を、1枚に置いても columns の中に置いても
  report・dark・light で誤り・警告が無く、屋根・名前の丸・柱・土台が図の矩形の中で重ならず、
  描いた図形の文字の枠も重ならない/台形の adj・基調色の系統の塗り・箇条の印/狭い枠で
  `figure-text-overflow`/柱の数・名前の長さ・知らないキー・空の items が IR でも Markdown でも
  誤り/値の表の 0 以下・割合 1 以上・高さの順の誤り。

- `phases_test.go`:例(`examples/md/phases.bx.md` ↔ `examples/ir/phases.json`。3段・群2つ・
  箱7つ)を report・light・dark で組んで警告が無い・山形と番号の形/山形の食い込み・帯の
  幅・箱の下の端がそろう・重ならない/Min が文の量で増える/はみ出しの警告/読み込みの誤り。
  `figure_fit_test.go`:枠が Min より低くても箱は文の要る高さを保ち文の矩形が重ならない・
  足りなければ `layout-overflow`/`subsystem-min` が出る(#107)・枠が Min より低いとき凡例は
  図の頭に移って枠の中にあり、帯・山形・箱と重ならない(#114)。
- `go test ./...`(`cycle_test.go`):Markdown と IR が同じ data・段の数と start の誤り/
  扇が5つの `blockArc` で上から72°ずつ、最初が基調色で段ごとに違う濃さ、強調の塗り、
  状態の枠線/番号が start から/題が輪の中・補足が輪の外/FixedAspect の申告/
  columns の中で light・report ともに最小・はみ出しの警告が無く、単独だと `spare-width`。
- `report_phases_cycle_test.go`(#110):report の phases の群の帯が塗りに白字(light は枠線)/
  列の中の cycle に 20 字前後の題5つで、題は輪の外に置かれ、題どうし・題と輪(番号の丸を
  含む正方形)が重ならず図の矩形の中、輪は扇の中に置くときより小さく、警告が無い。
- `cycle_columns_test.go`(#112):columns の1つ目の列に 20 字前後の題5つの cycle、横に
  role=main の箇条(と、もう1つの列)を report・dark・light で組み、列が最小の幅まで広がって
  題が輪の外に重ならずに入り、警告が無い/幅 160〜760pt のどの枠でも、題が重なれば警告が出て、
  Min の幅からは重ならず警告も無い。
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
- orgchart は3段まで。縦に積む形(#134)は3段目だけで、2段目(units)を縦に積む形は無い
  (unit が多くて2段目の箱が狭くなりすぎても、文を折り返して高くなるだけ。語が収まらなければ警告)。
  縦に積む形の3段目の箱の高さは、全部の箱で一番高い文に揃える。
- 泣き別れを避けるのは orgchart の箱だけ。process の箱・matrix・funnel は余りの幅を
  持たない(幅が上限か枠いっぱい)ので広げない。和文の語の境目は分からないので、
  語の途中で折れること(「委員 / 会」でなく「委 / 員会」)自体は避けられない。折り返しの位置は
  描くアプリが決めるので、測り(幅の余裕 `widthMargin` 込み)とずれることがある。
- matrix の状態は色の丸だけで見分ける(`mark` の support のチェックのほかに、形や印での区別は無い)。推す列は1つだけ(行の強調は無い)。
- funnel の転換率は前の段に対する割合だけ(最初の段に対する累計の割合は出さない)。
- phases は横に並べる形だけ。6段以上は読み込みの誤り(折り返さない)。箱が多すぎるときに
  段の列を分けない(はみ出しの警告だけ)。山形の高さは全部の段で同じ(一番長い題に合わせる)。
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
- **自然(Natural)**:縦の形・散布図は幅×`naturalAspect`(0.5)、横棒は項目の数×
  `naturalBarSlot`。円は、幅から左右の扇の外の名前・値(一番広いもの)と隙間を引いた
  直径に、上下の2行ずつと余白を足した高さ(細い枠でも円が小さくなりすぎない)。
  **上限(Max)**は横棒の高さ(`maxBarSlot`。棒を太らせても読みやすくならない)と、
  円の幅(高さ h を受けたとき、その高さの直径+左右の名前・値の幅+余白)。円は高さで
  直径が決まり、それより広い枠は横が空くだけなので、コアは枠を円の幅まで切り詰めて
  描き(余りは Place の側、既定は左)、余りを `spare-width`・容量の `remainingWidth` で
  知らせる(9節)。図の地(カード)も円の幅に合わせる。ほかは上限なし(大きくして困らない)。
- **並列のまとまり(columns)の中に置ける。** グラフは循環の図・フェーズの図・評価の格子・
  組織図(11節の cycle・phases・matrix・orgchart、#117)と並んで、
  `columns` のまとまりの中に置けるサブシステム(左にグラフ・右に読み取り)。まとまりの幅を使い切らないとき
  (円をまとまりの `role=main` に入れたときなど)は、そのブロックを指して `spare-width`。
- この Min を使って、本文の `layout-overflow`・容量(`check -capacity`)・`subsystem-min`
  が、ほかのブロックと同じに働く(グラフだけ特別扱いしない)。グラフを縮めて文字を
  詰めることはしない。
- **`chart-labels`(警告)**:項目の名前が、アプリが置く場所(縦の形は1項目の幅の 90%、
  横棒は枠の幅の `barLabelFraction`(30%))に、`labelMaxLines`(2)行で収まらない
  (折り返せない語が広すぎるか、行が多すぎる)とき。アプリは名前を斜めにする・間引く・
  何行にも折るので読めなくなる。最初の1つを指し(path は `…/data/categories/N`)、
  残りの数を文に添える。
- 値のラベルの重なり・凡例の折り返し・目盛りの刻みはアプリまかせで、検査しない。
- **値のラベルは折り返さない**:値のラベル(`c:dLbls`)の文字の書式に `wrap="none"` を
  付ける。アプリは値のラベルの幅を自分で決めて折り返すので、単位が長いと
  「2,859 億ド/ル」のように単位の途中で折れていた(LibreOffice)。数と単位は1行に保つ。
  目盛り・凡例はアプリの折り返しのまま。

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

## 13. サブシステムの中の数:値の表か決まりの数か(Backlog #70)

#68 のあと、部品の組み方は段に無い数を書けない。サブシステムの描画と計測に直に
書いていた数も、次の2つに分けた。

- **(a) 見た目の寸法**(余白・間隔・箱の大きさ・線の太さ・角・点の大きさ):値の表
  (`subsystems/<名前>.json`、テンプレートの `subsystems.<名前>`)の名前に移した。
  渡し方は従来どおりで、寸法はテンプレートを選んだときに Factory へ、色は描くときに
  Roles で引く。余白・間隔・箱の大きさのうち段に載るものには `scale:"space"` を付け、
  段から外れると読み込みで誤りにする。**図の中の細部の寸法**(Gantt の `barInset`・
  `axisGap`・文字の余白・凡例の色見本・区間圧縮の印など)は、線の太さ・角と同じく
  段の外とする決まりにした。値の表の名前つきのキーで、0 以下は読み込みで誤り、段の
  刻みには寄せない(どのキーがどちらかは `docs/template-values.md` の
  「subsystems/gantt.json の形」の表)。
- **(b) 決まりの数**(行・段・点の数の上限、比、半分にする 2、アプリの振る舞いの
  見積もり、図形を書ける最小の幅):名前つきの定数にし、なぜその数かを注釈に書いた。

| サブシステム | (b) として残した定数 |
|---|---|
| gantt | `ganttSegLabelMaxLines`(3行)・`ganttSegLabelMinEm`(字2つ分)・`ganttMilestoneMaxRows`(2段)・`ganttOpenEndDots`(3つ)・`ganttAnnotationMaxShifts`(6回)・`ganttMinDrawW`(1pt)・`roundRectTextInset`(1 − 1/√2) |
| chart | `chartAxisHeadroom`(1.2)・`chartBarSeriesGrowth`(0.5)・`chartPieLabelLines`(4)・`chartScatterAxisLines`(2)・`chartCategoryLabelFrac`(0.9) |
| 図(凡例)・funnel・matrix | `noWrapSlack`(4pt。折り返さない文字の箱のゆとりで、左揃えなので見えない) |
| shapes・matrix | 既にあった `connectorAngleMin`・`ellipseSiteFactor`・`matrixDotInk` と 1/√2(楕円に内接する矩形) |

Gantt に足した値の表のキーは `docs/template-values.md` の「subsystems/gantt.json の形」。
出力は1バイトも変わらない(`TestOutputsGolden` と従来のデモ2枚を展開して比べた)。
chart の PPTX の書き手(`chartxml.go`)の `orDefault` の数は、サブシステムが値を
渡さなかったときの書き手の既定で、サブシステムの描画と計測の外に置いた。

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
