package store

import (
	"sort"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// UpsertAbnormality 新增或覆盖异常记录并落盘。
func (s *Store) UpsertAbnormality(a *domain.LampAbnormality) error {
	return s.Mutate(func() {
		s.abnormalities[a.ID] = a
	})
}

// GetAbnormality 按 ID 查询异常。
func (s *Store) GetAbnormality(id string) *domain.LampAbnormality {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := s.abnormalities[id]
	if a == nil {
		return nil
	}
	return a.Clone()
}

// ListAbnormalities 返回异常列表（按最后发现时间倒序）。
func (s *Store) ListAbnormalities() []*domain.LampAbnormality {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.LampAbnormality, 0, len(s.abnormalities))
	for _, a := range s.abnormalities {
		out = append(out, a.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt.After(out[j].LastSeenAt) })
	return out
}

// FindOpenAbnormality 查找某航标某类型未解决的异常，不存在返回 nil。
func (s *Store) FindOpenAbnormality(beaconID string, at domain.AbnormalityType) *domain.LampAbnormality {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.abnormalities {
		if a.BeaconID == beaconID && a.Type == at && a.Status == domain.AbnormalityStatusOpen {
			return a.Clone()
		}
	}
	return nil
}

// CountOpenAbnormalities 统计未解决异常总数。
func (s *Store) CountOpenAbnormalities() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, a := range s.abnormalities {
		if a.Status == domain.AbnormalityStatusOpen {
			n++
		}
	}
	return n
}

// CountOpenAbnormalitiesByType 按类型统计未解决异常数。
func (s *Store) CountOpenAbnormalitiesByType() map[domain.AbnormalityType]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[domain.AbnormalityType]int)
	for _, a := range s.abnormalities {
		if a.Status == domain.AbnormalityStatusOpen {
			out[a.Type]++
		}
	}
	return out
}

// ListOpenAbnormalities 返回全部未解决异常。
func (s *Store) ListOpenAbnormalities() []*domain.LampAbnormality {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.LampAbnormality, 0)
	for _, a := range s.abnormalities {
		if a.Status == domain.AbnormalityStatusOpen {
			out = append(out, a.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt.After(out[j].LastSeenAt) })
	return out
}
