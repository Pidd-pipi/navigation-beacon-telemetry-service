package store

import (
	"example.com/navigation-beacon-telemetry-service/domain"
)

// AppendAudit 追加审计日志并落盘；超过保留上限时裁剪最旧记录。
func (s *Store) AppendAudit(log *domain.AuditLog, retention int) error {
	return s.Mutate(func() {
		s.audits = append(s.audits, log)
		if retention > 0 && len(s.audits) > retention {
			s.audits = s.audits[len(s.audits)-retention:]
		}
	})
}

// ListAudits 返回审计日志（最新在前），limit<=0 表示全部。
func (s *Store) ListAudits(limit int) []*domain.AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.audits)
	out := make([]*domain.AuditLog, n)
	for i, l := range s.audits {
		out[n-1-i] = l.Clone()
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// CountAudits 返回审计日志条数。
func (s *Store) CountAudits() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.audits)
}
