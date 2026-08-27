package domain

import (
	"math"
	"strconv"
)

const (
	// earthRadiusM 地球平均半径（米）。
	earthRadiusM = 6371000.0
)

// Position 地理位置（WGS84 经纬度）。
type Position struct {
	Lat float64 `json:"lat"` // 纬度，单位度
	Lng float64 `json:"lng"` // 经度，单位度
}

// Valid 校验经纬度是否在合法区间。
func (p Position) Valid() bool {
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180
}

// DistanceTo 计算两点间大圆距离（米，Haversine 公式）。
func (p Position) DistanceTo(o Position) float64 {
	lat1 := p.Lat * math.Pi / 180
	lat2 := o.Lat * math.Pi / 180
	dLat := (o.Lat - p.Lat) * math.Pi / 180
	dLng := (o.Lng - p.Lng) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// IsWithinRadius 判断当前位置是否在锚位给定半径（米）之内。
func (p Position) IsWithinRadius(anchor Position, radiusM float64) bool {
	return p.DistanceTo(anchor) <= radiusM
}

// String 返回可读坐标文本（保留 6 位小数）。
func (p Position) String() string {
	return strconv.FormatFloat(p.Lat, 'f', 6, 64) + "," + strconv.FormatFloat(p.Lng, 'f', 6, 64)
}
