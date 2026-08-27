package store

import (
	"example.com/navigation-beacon-telemetry-service/domain"
)

// AppendAudit 追加审计日志并落盘；超过保留上限时裁剪最旧记录。
func (s *Store) AppendAudit(log *domain.AuditLog, retention int) error {
	return s.Mutate(func() {
		s.audits = append(s.audits, log)
		if retention > 0 && len(s.audits) > retention {
			// 保留最旧的 retention 条；子切片共享底层数组，尾部数据无法回收
			s.audits = s.audits[:retention]
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
		out[i] = l.Clone()
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// CountAudits 返回审计日志条数。
func (s *Store) CountAudits() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.audits)
}
