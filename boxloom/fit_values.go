// 収めの梯子(値の表の fit。#126、親 #125 の自動の収め)。
//
// templates/<名前>.json の任意の "fit": { "shrink": [組…], "grow": [組…] }。各組は
// dense と同じく本文の値の一部を重ねるもの(書いた名前だけを替え、書かない名前は
// 前の段のまま)。詰める梯子(shrink)は dense の組から、広げる梯子(grow)は values
// から1段ずつ積む。読み込みで見るのは:
//
//   - 書ける名前は fitStep の json タグのものだけ(知らないキーは誤り)。空の組も誤り
//   - 値は 0 より大きく、段(scale)に載る
//   - shrink の段は前の段(最初は dense)より大きくならない、grow の段は前の段(最初は
//     values)より小さくならない。前の段と同じだけの組(何も替えない段)も誤り
//
// shrink は -fit の詰め(fit_shrink.go、#127)が使う。grow はまだ使わない(fit-grow)。

package boxloom

import (
	"fmt"
	"reflect"
	"strings"
)

// fitStep は梯子の1段(書いた名前だけを前の段に重ねる)。
type fitStep struct {
	Body        *float64 `json:"body,omitempty"`
	SubPoint    *float64 `json:"subPoint,omitempty"`
	TableText   *float64 `json:"tableText,omitempty"`
	Small       *float64 `json:"small,omitempty"`
	BlockGap    *float64 `json:"blockGap,omitempty"`
	ItemGap     *float64 `json:"itemGap,omitempty"`
	CardPadding *float64 `json:"cardPadding,omitempty"`
	// 行ごとの余白(#136。dense と同じ名前)
	TableCellPadX  *float64 `json:"tableCellPaddingX,omitempty"`
	TableCellPadY  *float64 `json:"tableCellPaddingY,omitempty" zero:"allowed"`
	PairsRowGap    *float64 `json:"pairsRowGap,omitempty"`
	PairMarkSize   *float64 `json:"pairMarkSize,omitempty"`
	MatrixRowTitle *float64 `json:"matrixRowTitle,omitempty"`
}

// templateFit は値の表の fit の形。
type templateFit struct {
	Shrink []fitStep `json:"shrink,omitempty"`
	Grow   []fitStep `json:"grow,omitempty"`
}

// FitValues は梯子の1段を重ねたあとの本文の値の組(全部の名前がそろったもの)。
type FitValues struct {
	Body        float64 `json:"body" scale:"type"`
	SubPoint    float64 `json:"subPoint" scale:"type"`
	TableText   float64 `json:"tableText" scale:"type"`
	Small       float64 `json:"small" scale:"type"`
	BlockGap    float64 `json:"blockGap" scale:"space"`
	ItemGap     float64 `json:"itemGap" scale:"space"`
	CardPadding float64 `json:"cardPadding" scale:"space"`

	TableCellPadX  float64 `json:"tableCellPaddingX" scale:"space"`
	TableCellPadY  float64 `json:"tableCellPaddingY" scale:"space"`
	PairsRowGap    float64 `json:"pairsRowGap" scale:"space"`
	PairMarkSize   float64 `json:"pairMarkSize" scale:"space"`
	MatrixRowTitle float64 `json:"matrixRowTitle" scale:"type"`
}

// TemplateFit は収めの梯子(各段は前の段に重ねたあとの値の組)。Shrink は詰める順
// (dense より1段ずつ小さく)、Grow は広げる順(values より1段ずつ大きく)。どちらも
// 空なら、そのテンプレートは詰めも広げもしない。
type TemplateFit struct {
	Shrink []FitValues
	Grow   []FitValues
}

// apply は v の本文の値を f に替えたもの。
func (f FitValues) apply(v TemplateValues) TemplateValues {
	v.Body, v.SubPoint, v.TableText, v.Small = f.Body, f.SubPoint, f.TableText, f.Small
	v.BlockGap, v.ItemGap, v.CardPadding = f.BlockGap, f.ItemGap, f.CardPadding
	v.TableCellPadX, v.TableCellPadY = f.TableCellPadX, f.TableCellPadY
	v.PairsRowGap, v.PairMarkSize, v.MatrixRowTitle = f.PairsRowGap, f.PairMarkSize, f.MatrixRowTitle
	return v
}

func fitBaseValues(v TemplateValues) FitValues {
	return FitValues{Body: v.Body, SubPoint: v.SubPoint, TableText: v.TableText, Small: v.Small,
		BlockGap: v.BlockGap, ItemGap: v.ItemGap, CardPadding: v.CardPadding,
		TableCellPadX: v.TableCellPadX, TableCellPadY: v.TableCellPadY,
		PairsRowGap: v.PairsRowGap, PairMarkSize: v.PairMarkSize, MatrixRowTitle: v.MatrixRowTitle}
}

// fitBaseDense は詰める梯子の土台:dense の組を重ねた values の値(dense の持たない
// cardPadding は values のまま)。
func fitBaseDense(d DenseValues, v TemplateValues) FitValues {
	return fitBaseValues(d.apply(v))
}

// resolveFit は値の表の fit を検査し、各段を重ねた値の組の梯子にする。
func resolveFit(f *templateFit, v TemplateValues, d DenseValues, sc TemplateScale) (TemplateFit, error) {
	var out TemplateFit
	if f == nil {
		return out, nil
	}
	var err error
	if out.Shrink, err = resolveLadder("fit.shrink", f.Shrink, fitBaseDense(d, v), "dense", -1, sc); err != nil {
		return out, err
	}
	if out.Grow, err = resolveLadder("fit.grow", f.Grow, fitBaseValues(v), "values", +1, sc); err != nil {
		return out, err
	}
	return out, nil
}

// resolveLadder は梯子を base から1段ずつ重ねる。dir が -1 なら段は前より大きく
// ならない(詰める)、+1 なら小さくならない(広げる)。
func resolveLadder(at string, steps []fitStep, base FitValues, baseName string, dir float64, sc TemplateScale) ([]FitValues, error) {
	var out []FitValues
	prev, prevName := base, baseName
	for i, st := range steps {
		here := fmt.Sprintf("%s[%d]", at, i)
		next := prev
		sv, nv, pv := reflect.ValueOf(st), reflect.ValueOf(&next).Elem(), reflect.ValueOf(prev)
		changed := false
		for k := 0; k < sv.NumField(); k++ {
			p := sv.Field(k)
			if p.IsNil() {
				continue
			}
			name, _, _ := strings.Cut(sv.Type().Field(k).Tag.Get("json"), ",")
			x, was := p.Elem().Float(), pv.Field(k).Float()
			from := prevName
			if from == "dense" && name == "cardPadding" {
				from = "values" // dense は cardPadding を持たない(values のまま)
			}
			if !(x > 0) && !(x == 0 && sv.Type().Field(k).Tag.Get("zero") == "allowed") {
				return nil, fmt.Errorf("%s.%s: must be greater than 0 (got %v)", here, name, x)
			}
			if dir < 0 && x > was+scaleEps {
				return nil, fmt.Errorf("%s.%s: %v is larger than %s.%s (%v); the shrink ladder only packs, so each step cannot be larger than the one before", here, name, x, from, name, was)
			}
			if dir > 0 && x < was-scaleEps {
				return nil, fmt.Errorf("%s.%s: %v is smaller than %s.%s (%v); the grow ladder only widens, so each step cannot be smaller than the one before", here, name, x, from, name, was)
			}
			if x != was {
				changed = true
			}
			nv.Field(k).SetFloat(x)
		}
		if !changed {
			return nil, fmt.Errorf("%s: changes nothing from %s; each step must change at least one value", here, prevName)
		}
		if err := checkOnScale(here, next, sc); err != nil {
			return nil, err
		}
		out = append(out, next)
		prev, prevName = next, here
	}
	return out, nil
}
