package domain

import "testing"

func TestLampPatternValid(t *testing.T) {
	cases := []struct {
		p    LampPattern
		want bool
	}{
		{LampPattern{FlashSec: 2, EclipseSec: 2}, true},
		{LampPattern{FlashSec: 1, EclipseSec: 0}, true},
		{LampPattern{FlashSec: 0, EclipseSec: 2}, false},
		{LampPattern{FlashSec: 2, EclipseSec: -1}, false},
	}
	for _, c := range cases {
		if got := c.p.Valid(); got != c.want {
			t.Errorf("%+v.Valid() = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestLampPatternDeviation(t *testing.T) {
	configured := LampPattern{FlashSec: 2, EclipseSec: 2}
	const tol = 0.5

	if configured.DeviatesFrom(configured, tol) {
		t.Error("相同灯质不应判定偏差")
	}
	// 偏差 0.4s < 容差 0.5s → 不偏差
	if (LampPattern{FlashSec: 2.4, EclipseSec: 2}).DeviatesFrom(configured, tol) {
		t.Error("0.4s 偏差不应超容差")
	}
	// 偏差 0.6s > 容差 → 偏差
	if !(LampPattern{FlashSec: 2.6, EclipseSec: 2}).DeviatesFrom(configured, tol) {
		t.Error("0.6s 偏差应超容差")
	}
	// 熄灭段偏差
	if !(LampPattern{FlashSec: 2, EclipseSec: 3.0}).DeviatesFrom(configured, tol) {
		t.Error("熄灭段 1s 偏差应超容差")
	}
	// MaxDeviationSec
	if d := (LampPattern{FlashSec: 3, EclipseSec: 2}).MaxDeviationSec(configured); d != 1.0 {
		t.Errorf("MaxDeviationSec = %v, want 1.0", d)
	}
}
