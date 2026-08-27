package store

import (
	"sort"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// CreateBeacon 新增航标灯并落盘。
func (s *Store) CreateBeacon(b *domain.Beacon) error {
	return s.Mutate(func() {
		s.beacons[b.ID] = b
	})
}

// GetBeacon 按 ID 查询航标灯，不存在返回 nil。
func (s *Store) GetBeacon(id string) *domain.Beacon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.beacons[id]
	if b == nil {
		return nil
	}
	return b.Clone()
}

// ListBeacons 返回全部航标灯（按创建时间升序）。
func (s *Store) ListBeacons() []*domain.Beacon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Beacon, 0, len(s.beacons))
	for _, b := range s.beacons {
		out = append(out, b.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// CountBeacons 返回航标灯总数。
func (s *Store) CountBeacons() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.beacons)
}

// UpdateBeacon 更新航标灯字段并落盘。
func (s *Store) UpdateBeacon(b *domain.Beacon) error {
	return s.Mutate(func() {
		s.beacons[b.ID] = b
	})
}
