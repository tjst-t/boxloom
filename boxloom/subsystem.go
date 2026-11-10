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
type Measurement struct {
	Natural Size
	Min     Size
	Max     Size
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
}

func NewSubsystemRegistry() *SubsystemRegistry {
	return &SubsystemRegistry{factories: map[string]SubsystemFactory{}}
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
