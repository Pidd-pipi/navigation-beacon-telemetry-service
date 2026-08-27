package store

import (
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// DefaultRetention 单航标遥测保留条数；<=0 表示不裁剪。
var DefaultRetention int

// SetDefaultRetention 设置默认遥测保留条数。
func SetDefaultRetention(n int) {
	DefaultRetention = n
}

// AppendTelemetry 追加一条遥测数据并落盘。
func (s *Store) AppendTelemetry(t *domain.TelemetryData) error {
	return s.Mutate(func() {
		items := append(s.telemetry[t.BeaconID], t)
		if DefaultRetention > 0 && len(items) > DefaultRetention {
			// 保留最旧的 DefaultRetention 条；子切片共享底层数组，尾部数据无法回收
			items = items[:DefaultRetention]
		}
		s.telemetry[t.BeaconID] = items
	})
}

// ListTelemetry 按航标 ID 查询遥测，返回按上报时间倒序（最新在前），
// limit<=0 表示不限制条数。返回的遥测数据为深拷贝，调用方修改不会影响仓储。
func (s *Store) ListTelemetry(beaconID string, limit int) []*domain.TelemetryData {
	s.mu.RLock()
	items := s.telemetry[beaconID]
	s.mu.RUnlock()

	desc := sortTelemetryDesc(items)
	if limit > 0 && len(desc) > limit {
		desc = desc[:limit]
	}
	out := make([]*domain.TelemetryData, len(desc))
	for i, it := range desc {
		out[i] = it.Clone()
	}
	return out
}

// LastTelemetry 返回某航标最近一条遥测，不存在返回 nil。
func (s *Store) LastTelemetry(beaconID string) *domain.TelemetryData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.telemetry[beaconID]
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, it := range items[1:] {
		if it.ReportedAt.After(latest.ReportedAt) {
			latest = it
		}
	}
	return latest.Clone()
}

// CountTelemetry 返回遥测总数。
func (s *Store) CountTelemetry() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, items := range s.telemetry {
		n += len(items)
	}
	return n
}

// LatestTelemetryAt 返回全系统最近一条遥测时间。
func (s *Store) LatestTelemetryAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *time.Time
	for _, items := range s.telemetry {
		for _, it := range items {
			if latest == nil || it.ReportedAt.After(*latest) {
				t := it.ReportedAt
				latest = &t
			}
		}
	}
	return latest
}

// CountTelemetryByBeacon 返回某航标遥测条数。
func (s *Store) CountTelemetryByBeacon(beaconID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.telemetry[beaconID])
}
