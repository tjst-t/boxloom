// report(読む資料のテンプレート)が自分で持つ組み方(Backlog #83)。
//
// 見た目の違いは値の表(templates/report.json)と、ここで差し替える骨組みの部品で
// 持つ。コアの標準の部品・骨組みには手を入れない(dark・light の出力は変わらない)。
//
//   - contentHeader:リード文を上の見出し(leadText・基調色・太字)に置き、題(話題)を
//     その下に slideTitle・字の色で置く。題はこれまでどおり title のプレースホルダー
//     (位置だけが下になる)。リードの無いスライドは、題を上の見出しの位置と大きさで置く。
//   - columns・pairs:列の見出し(まとまりの題・対の側の見出し)を基調色(accent)にする。
//     組み方は標準のもの(columnsHeaded・pairsHeaded)で、見出しの色の役割名だけを渡す。

package boxloom

// bundledTemplateParts は同梱のテンプレートのうち、自分の組み方を持つものの登録
// (loadTemplate が読み込みの後に呼ぶ)。
var bundledTemplateParts = map[string]func(t *Template) error{
	"report": registerReportParts,
}

func registerReportParts(t *Template) error {
	if err := t.components.override("columns", componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return ctx.t.columnsHeaded(ctx, b, "accent")
	})); err != nil {
		return err
	}
	// pairs は標準でない型なので、同梱の組み方(registerBundledComponents)の代わりに登録する。
	if err := t.components.register(pairsType, componentFunc(func(ctx componentContext, b *IRBlock) (*Spec, error) {
		return pairsHeaded(ctx, b, "accent")
	})); err != nil {
		return err
	}
	return t.skeleton.override(skelContentHeader, skeletonFunc(func(ctx skeletonContext) (*Spec, error) {
		return ctx.t.reportContentHeader(ctx.slide), nil
	}))
}

// reportContentHeader は report の本文のスライドの頭:上にリード文(上の見出し)、
// 下に題(話題)。リードが無ければ題が上の見出しの位置と大きさを取る。
func (t *Template) reportContentHeader(s *IRSlide) *Spec {
	if s.Lead == nil {
		return t.text(s.Title, "leadText", "accent", true, WithPlaceholder("title"), fromIR("title"))
	}
	lead := t.prose(s.Lead, "leadText", "accent", true, fromIR("lead"))
	title := t.text(s.Title, "slideTitle", "text_primary", false, WithPlaceholder("title"), fromIR("title"))
	return Col([]*Spec{lead, title}, tokGap("headerGap"))
}
