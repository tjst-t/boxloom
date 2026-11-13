// report(読む資料のテンプレート)が自分で持つ組み方(Backlog #83)。
//
// 見た目の違いは値の表(templates/report.json)と、ここで差し替える骨組みの部品で
// 持つ。コアの標準の部品・骨組みには手を入れない(dark・light の出力は変わらない)。
//
//   - contentHeader:リード文を上の見出し(leadText・基調色・太字)に置き、題(話題)を
//     その下に slideTitle・字の色で置く。題はこれまでどおり title のプレースホルダー
//     (位置だけが下になる)。リードの無いスライドは、題を上の見出しの位置と大きさで置く。

package boxloom

// bundledTemplateParts は同梱のテンプレートのうち、自分の組み方を持つものの登録
// (loadTemplate が読み込みの後に呼ぶ)。
var bundledTemplateParts = map[string]func(t *Template) error{
	"report": registerReportParts,
}

func registerReportParts(t *Template) error {
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
