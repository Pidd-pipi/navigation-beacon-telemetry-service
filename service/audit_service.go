// Package service 实现业务用例编排：采集校验、诊断、任务流转、指令回执等。
package service

import (
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// AuditService 操作审计服务：所有关键业务动作统一留痕。
type AuditService struct {
	store *store.Store
	cfg   *config.Config
}

// NewAuditService 构造审计服务。
func NewAuditService(st *store.Store, cfg *config.Config) *AuditService {
	return &AuditService{store: st, cfg: cfg}
}

// Log 记录一条操作审计日志（时间取当前时刻）。
func (s *AuditService) Log(action, entityType, entityID, operator, detail string) {
	s.LogAt(time.Now(), action, entityType, entityID, operator, detail)
}

// LogAt 记录一条操作审计日志（指定时间，便于测试与回放）。
func (s *AuditService) LogAt(now time.Time, action, entityType, entityID, operator, detail string) {
	log := domain.NewAuditLog(s.store.NextID("L"), action, entityType, entityID, operator, detail, now)
	_ = s.store.AppendAudit(log, s.cfg.AuditRetention)
}

// List 查询审计日志，limit<=0 表示全部。
func (s *AuditService) List(limit int) []*domain.AuditLog {
	return s.store.ListAudits(limit)
}
