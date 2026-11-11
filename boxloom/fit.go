package boxloom

// 図(サブシステム)の大きさと、割り当てられた矩形のあいだの橋渡し。
//
// レイアウトは図の自己申告(Measure)を出発点に矩形を割り当てるが、交差軸の
// stretch などで、図が使い切れない大きさの矩形が来ることがある。そのとき
// コアは図を拡大縮小せず(文字の大きさがテンプレートの段から外れるため)、
// 図が申告した大きさまで矩形を切り詰め、余りを Place で決めた側に置く。
// 詳しくは docs/subsystem.md の「縦横比と余りの置き方」。

// PlaceAlign は余りを置く向き。既定(0)は始まりの側(左・上)。
type PlaceAlign int

const (
	PlaceStart PlaceAlign = iota
	PlaceCenter
	PlaceEnd
)

// Place は横(X)と縦(Y)の置き方。テンプレートの方針(骨組みのコード)で決める。
type Place struct {
	X, Y PlaceAlign
}

func (a PlaceAlign) offset(spare float64) float64 {
	switch a {
	case PlaceCenter:
		return spare / 2
	case PlaceEnd:
		return spare
	}
	return 0
}

// fitBox は、割り当てられた矩形 box の中で図が使う大きさを返す。
//   - FixedAspect の図: 幅から決まる高さが box に収まればその高さ、収まらなければ
//     高さから決まる幅(どちらも Natural)。
//   - それ以外: 申告した上限(Max、0 は上限なし)で各軸を止める。
//
// どちらも box より大きくはしない(足りないときは box のまま。最小を下回れば
// lintSubsystemMin が警告する)。
func fitBox(sub Subsystem, box LBox) (w, h float64) {
	w, h = box.Width, box.Height
	m := sub.Measure(Constraint{Width: box.Width})
	if m.FixedAspect {
		if nh := m.Natural.Height; nh > 0 && nh < h {
			return w, nh
		}
		if nw := sub.Measure(Constraint{Height: box.Height}).Natural.Width; nw > 0 && nw < w {
			return nw, h
		}
		return w, h
	}
	if m.Max.Height > 0 && m.Max.Height < h {
		h = m.Max.Height
	}
	if mw := sub.Measure(Constraint{Height: h}).Max.Width; mw > 0 && mw < w {
		w = mw
	}
	return w, h
}

// subsystemDrawBox は Render と Lint に渡す矩形。s.Box(割り当て)の中で fitBox の
// 大きさを s.Place に従って置く。
func subsystemDrawBox(s *Spec) LBox {
	if s.Subsystem == nil {
		return s.Box
	}
	w, h := fitBox(s.Subsystem, s.Box)
	const eps = 0.01
	if w >= s.Box.Width-eps && h >= s.Box.Height-eps {
		return s.Box
	}
	return LBox{
		X:      s.Box.X + s.Place.X.offset(s.Box.Width-w),
		Y:      s.Box.Y + s.Place.Y.offset(s.Box.Height-h),
		Width:  w,
		Height: h,
	}
}
