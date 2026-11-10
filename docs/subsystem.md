# サブシステムの差し込み口

サブシステムは、Box レイアウトの葉に埋め込む「自分で描く部品」です(Gantt
チャートなど)。この文書は、その差し込み口(Go の interface)の設計と理由を
まとめたものです。実装は `boxloom/subsystem.go`、Gantt での使い方は
`boxloom/gantt.go`、試験は `boxloom/subsystem_test.go` にあります。

## 決めたこと

```go
type Subsystem interface {
	Name() string                       // 種類の名前。警告の文に使う
	Measure(c Constraint) Measurement   // 自然な大きさと最小の大きさの自己申告
	Render(ctx RenderContext, box LBox) // 割り当てられた矩形に描く
}

type Constraint  struct{ Width, Height float64 } // 0 より大きい軸だけが分かっている
type Measurement struct{ Natural, Min Size }

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

**Constraint**:親は先に交差軸(縦に並べるなら幅)を決め、そのあと主軸
(高さ)を尋ねます。サブシステムは分かっている軸を受け取って、もう一方の軸を
答えます。Gantt は幅によって高さが変わりませんが、折り返す文の多い図(表や
組織図など)は、幅に応じて要る高さが変わるので、この形にしてあります。

## 2. レイアウトでの扱い(Phase3a)

- **basis**:サブシステムの葉は、明示の basis・幅・高さが無ければ、自己申告の
  自然な大きさを basis にします。
- **余るとき**:いままでどおり grow で受け取ります。
- **足りないとき**:従来の比例の縮め(Phase3)の **前に**、サブシステムが
  「自然な大きさと最小の大きさの間のゆとり」を返します(Phase3a)。
  見出しなどのテキストはここでは縮みません。
- **最小でも足りないとき**:従来どおり全員が比例して縮み、検査
  (`lintSubsystemMin`)が `[SUBSYSTEM WARNING] gantt: needs at least …pt height`
  を出して、スライドのノートにも書きます。黙って潰しません。

**理由**:サブシステムを入れた最初の試みで、Gantt の自然な高さ(446pt)が
デモのスライドの空き(414pt)より大きく、従来の比例の縮めで **見出しまで
縮んでしまいました**。図は伸び縮みに強く、見出しの文字は弱い。なので、
自己申告で「縮めてよい幅」を言える部品から先に縮めるのが筋です。サブシステムを
含まない木では Phase3a は何もしないので、Python 版との座標の一致は崩れません。

**デモでの数字**:Gantt の自然な高さ 446.0pt、最小 406.4pt、割り当て
413.8pt(幅 912pt)。最小以上なので警告は出ません。

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

- コアは既定の表(`DefaultSubsystems()`:いまは `gantt` だけ)を持ちます。
- テンプレートは `Clone()` した表に `Override` で自分の描き方を入れます。
  コンテンツのデータ(`GanttChart`)は変わらず、描き方だけが変わります。
- `Override` は無い名前を断り、`Register` は重複を断ります。打ち間違いを黙って
  通さないためです。
- **理由**:Memory の「部品とサブシステムはコードで書く」に合わせ、差し替えの
  単位を「種類の名前ごとの Factory」にしました。テンプレートが Gantt を
  丸ごと別の描き方にする、という一番大きな自由度を、コアに手を入れずに持てます。

## 5. 既存の canvas との関係

`Canvas(func(ctx, box) {...})` は残してあり、「自然な大きさ 0 を申告する簡易な
サブシステム」として動きます。場所は grow で受け取る、旧来の canvas と同じ
ふるまいです。

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
  測っていない。これらはデザインの値なので、いずれテンプレートの値の表に移す
  のがよい。
- 横に並べたとき(Row の中)のサブシステムの幅の申告は、試験の
  `fixedSub` でしか確かめていない(Gantt を横に並べる例が無い)。
- サブシステムが入れ子の Box を中に持つ(サブシステムの中で outer engine を
  呼ぶ)形は扱っていない。
- 登録簿はまだコンテンツの記法・IR と繋がっていない。IR の
  `{"gantt": {...}}` のような種類の名前で `Build` する想定。
