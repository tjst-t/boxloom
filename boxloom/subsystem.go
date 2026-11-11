// サブシステムの差し込み口。
//
// サブシステムは、Box レイアウトの葉に埋め込まれる「自分で描く部品」
// (Gantt チャートなど)。outer layout engine はサブシステムの中身を知らず、
// 次の2つだけを尋ねる:
//
//  1. Measure: 「この幅(または高さ)なら、あなたの自然な大きさと最小の大きさは?」
//  2. Render:  「この矩形をあげるので、この役割名の解決器を使って描いて」
//
// サブシステムは座標の計算をすべて自分の中で閉じる。役割名(support / danger …)
// から実際の色を引くのは Roles だけで、サブシステムは16進の色を知らない。
// 設計の理由は docs/subsystem.md を参照。
package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Size は大きさ(pt)。
type Size struct {
	Width, Height float64
}

// Constraint は Measure に渡す「分かっている大きさ」。親が交差軸の大きさを
// 決めたあとに主軸の大きさを尋ねる、という使い方が基本。0 より大きい軸だけが
// 分かっている(0 は「分からない、あなたが決めて」)。
type Constraint struct {
	Width, Height float64
}

// Measurement はサブシステムの自己申告。
//   - Natural: 無理なく描ける大きさ。レイアウトはこれを basis(出発点)に使う。
//   - Min:     これより小さいと読めなくなる大きさ。これより小さく割り当てたとき、
//     レイアウトの検査が警告を出す(黙って潰さない)。
//   - Max:     これより大きくしても見やすくならない大きさ(0 は「上限なし」)。
//     grow で余りを配るとき、レイアウトはここで止め、残りはほかの伸びる子へ
//     回す(回す先が無ければ空きのまま残す)。
//
// Constraint で分かっている軸は、その値をそのまま返してよい。
//
// 幅から高さが決まる図(組織図など)は、Constraint{Width: w} を受けたときの
// Natural.Height を「その幅での高さ」として返す(高さを受けたときは
// Natural.Width を「その高さでの幅」として)。レイアウトはこれで幅に合わせた
// 高さを渡す(heightForWidth)。
//
//   - FixedAspect: 形(縦横比)を保つ図。割り当てられた矩形が、分かっている
//     片方の軸から決まる大きさより大きいとき、コアは Natural の大きさまで
//     切り詰めた矩形を Render に渡し、余りは Spec.Place で置く(拡大縮小は
//     しない。文字の大きさがテンプレートの段から外れないように)。
type Measurement struct {
	Natural     Size
	Min         Size
	Max         Size
	FixedAspect bool
}

// Roles は役割名 → 実際の値を引く解決器。テンプレート(テーマ)が実装する。
type Roles interface {
	Color(role string) string
}

// RenderContext は Render に渡すもの。サブシステムが外の世界に触れる口は
// ここに入っているものだけ。
type RenderContext struct {
	Out   *PPTXBuilder // プリミティブ(図形・テキスト)の書き出し先
	Roles Roles        // 役割名の解決器
}

// Subsystem はすべてのサブシステムが満たす interface。
type Subsystem interface {
	// Name はサブシステムの種類の名前(例 "gantt")。警告の文に使う。
	Name() string
	// Measure は自然な大きさと最小の大きさを申告する。何度呼ばれてもよいように
	// 副作用を持たないこと。
	Measure(c Constraint) Measurement
	// Render は割り当てられた矩形 box の中に描く。
	Render(ctx RenderContext, box LBox)
}

// --- テンプレートがサブシステムを差し替えるための登録簿 ---

// SubsystemFactory はコンテンツ側のデータ(意味だけを持つ)からサブシステムを
// 作る。同じデータに対して、テンプレートは別の Factory(別の描き方)を登録できる。
type SubsystemFactory func(data any) (Subsystem, error)

// SubsystemRegistry は「種類の名前 → Factory」の表。コアは既定の表を持ち、
// テンプレートは Clone した表の一部を Override で差し替える。
type SubsystemRegistry struct {
	factories map[string]SubsystemFactory
	decls     map[string]SubsystemDecl
}

// SubsystemDecl はサブシステムが名乗るもの(作者・AI に見せる書き方)。
// 描き方(Factory)とは別に持ち、Override で描き方を差し替えても変わらない
// (意味の形はサブシステムの種類が決め、テンプレートが変えるのは見た目だけ)。
type SubsystemDecl struct {
	// Schema は IR のブロック({type: "subsystem", subsystem, variant?, data})の
	// JSON Schema(schema/subsystems/*.schema.json と同じもの)。
	Schema json.RawMessage
	// Markdown は Markdown のコードブロックで書くときの、Schema に書けない
	// 約束事(範囲の A .. B など)。1つずつ短い文で。
	Markdown []string
}

func NewSubsystemRegistry() *SubsystemRegistry {
	return &SubsystemRegistry{factories: map[string]SubsystemFactory{}, decls: map[string]SubsystemDecl{}}
}

// Declare は登録済みの種類に、名乗るもの(Schema など)を付ける。
func (r *SubsystemRegistry) Declare(kind string, d SubsystemDecl) error {
	if _, ok := r.factories[kind]; !ok {
		return fmt.Errorf("cannot declare unknown subsystem %q (known: %v)", kind, r.Kinds())
	}
	r.decls[kind] = d
	return nil
}

// Decl は種類が名乗るものを返す(無ければ ok=false)。
func (r *SubsystemRegistry) Decl(kind string) (SubsystemDecl, bool) {
	d, ok := r.decls[kind]
	return d, ok
}

// Register は種類を足す。同じ名前がすでにあれば誤り(うっかり上書きを防ぐ)。
func (r *SubsystemRegistry) Register(kind string, f SubsystemFactory) error {
	if _, ok := r.factories[kind]; ok {
		return fmt.Errorf("subsystem %q is already registered (use Override to replace)", kind)
	}
	r.factories[kind] = f
	return nil
}

// Override は既存の種類の描き方を差し替える。無い名前は誤り(打ち間違いを
// 黙って通さない)。
func (r *SubsystemRegistry) Override(kind string, f SubsystemFactory) error {
	if _, ok := r.factories[kind]; !ok {
		return fmt.Errorf("cannot override unknown subsystem %q (known: %v)", kind, r.Kinds())
	}
	r.factories[kind] = f
	return nil
}

// Build はコンテンツのデータからサブシステムを作る。
func (r *SubsystemRegistry) Build(kind string, data any) (Subsystem, error) {
	f, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("unknown subsystem %q (known: %v)", kind, r.Kinds())
	}
	return f(data)
}

// Clone はテンプレートごとに表を分けるために写しを作る。
func (r *SubsystemRegistry) Clone() *SubsystemRegistry {
	c := NewSubsystemRegistry()
	for k, f := range r.factories {
		c.factories[k] = f
	}
	for k, d := range r.decls {
		c.decls[k] = d
	}
	return c
}

func (r *SubsystemRegistry) Kinds() []string {
	ks := make([]string, 0, len(r.factories))
	for k := range r.factories {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// DefaultSubsystems はコアが標準で持つサブシステムの表。
func DefaultSubsystems() *SubsystemRegistry {
	r := NewSubsystemRegistry()
	must(r.Register("gantt", NewGanttSubsystem))
	must(r.Declare("gantt", ganttDecl()))
	must(r.Register("shapes", NewShapesSubsystem))
	must(r.Declare("shapes", shapesDecl()))
	return r
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// --- 役割名の解決器の検査つきの包み ---

// checkedRoles は引けなかった役割名を覚えておく。テーマに無い役割名を
// サブシステムが使ったとき、黙って空の色にせず警告にするため。
type checkedRoles struct {
	inner   Roles
	missing map[string]bool
}

func newCheckedRoles(r Roles) *checkedRoles {
	return &checkedRoles{inner: r, missing: map[string]bool{}}
}

func (c *checkedRoles) Color(role string) string {
	v := c.inner.Color(role)
	if v == "" {
		c.missing[role] = true
	}
	return v
}

func (c *checkedRoles) Missing() []string {
	ks := make([]string, 0, len(c.missing))
	for k := range c.missing {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// SubsystemLinter は、描く前に自分の中身を検査できるサブシステムが満たす
// (任意の interface。満たさなければ検査しない)。box は割り当てられた矩形で、
// 返す文は警告として標準出力(check・build の出力)に出る。ノートには入れない。
type SubsystemLinter interface {
	Lint(box LBox) []string
}

// lintSubsystems は、最小の大きさの検査と、各サブシステム自身の検査
// (SubsystemLinter)をまとめて行う。
func lintSubsystems(root *Spec) []string {
	out := lintSubsystemMin(root)
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindSubsystem && s.Subsystem != nil {
			if l, ok := s.Subsystem.(SubsystemLinter); ok {
				out = append(out, l.Lint(subsystemDrawBox(s))...)
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}

// --- 最小の大きさの検査 ---

// lintSubsystemMin は、サブシステムに割り当てた矩形が自己申告の最小の大きさを
// 下回っていないかを調べ、下回っていれば警告の文を返す。
func lintSubsystemMin(root *Spec) []string {
	var out []string
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.Kind == KindSubsystem && s.Subsystem != nil {
			m := s.Subsystem.Measure(Constraint{Width: s.Box.Width})
			const eps = 0.01
			if m.Min.Height > s.Box.Height+eps {
				out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] %s: needs at least %.1fpt height, got %.1fpt",
					s.Subsystem.Name(), m.Min.Height, s.Box.Height))
			}
			if m.Min.Width > s.Box.Width+eps {
				out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] %s: needs at least %.1fpt width, got %.1fpt",
					s.Subsystem.Name(), m.Min.Width, s.Box.Width))
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}
