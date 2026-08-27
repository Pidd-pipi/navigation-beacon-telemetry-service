package domain

import (
	"fmt"
	"math"
)

// LampPattern 灯质（闪法）：一次闪光时长 + 一次熄灭时长，单位秒。
// 例如「闪 2s / 灭 2s」即 FlashSec=2、EclipseSec=2。
type LampPattern struct {
	FlashSec   float64 `json:"flash_sec"`
	EclipseSec float64 `json:"eclipse_sec"`
}

// Valid 校验灯质参数：闪光时长必须为正，熄灭时长必须非负。
func (p LampPattern) Valid() bool {
	return p.FlashSec > 0 && p.EclipseSec >= 0 &&
		!math.IsNaN(p.FlashSec) && !math.IsNaN(p.EclipseSec) &&
		!math.IsInf(p.FlashSec, 0) && !math.IsInf(p.EclipseSec, 0)
}

// String 返回「闪 Xs/灭 Ys」可读文本。
func (p LampPattern) String() string {
	return fmt.Sprintf("闪%.1fs/灭%.1fs", p.FlashSec, p.EclipseSec)
}

// EqualWithin 判断两个灯质在容差（秒）内是否一致。
func (p LampPattern) EqualWithin(o LampPattern, toleranceSec float64) bool {
	return math.Abs(p.FlashSec-o.FlashSec) <= toleranceSec &&
		math.Abs(p.EclipseSec-o.EclipseSec) <= toleranceSec
}

// DeviatesFrom 判断实测灯质相对设定灯质的偏差是否超过容差（秒）。
func (p LampPattern) DeviatesFrom(configured LampPattern, toleranceSec float64) bool {
	return !p.EqualWithin(configured, toleranceSec)
}

// Devise 计算实测与设定的最大偏差秒数。
func (p LampPattern) MaxDeviationSec(o LampPattern) float64 {
	d1 := math.Abs(p.FlashSec - o.FlashSec)
	d2 := math.Abs(p.EclipseSec - o.EclipseSec)
	if d1 > d2 {
		return d1
	}
	return d2
}

// JSON 序列化时输出字符串形式辅助字段。
func (p LampPattern) Display() string { return p.String() }
