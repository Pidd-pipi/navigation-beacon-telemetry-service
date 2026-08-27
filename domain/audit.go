package domain

import "time"

// AuditLog 操作审计日志实体。
// 遥控指令下发/回执、任务状态流转、异常处置等关键操作全部留痕。
type AuditLog struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Operator   string    `json:"operator,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// NewAuditLog 构造审计日志。
func NewAuditLog(id, action, entityType, entityID, operator, detail string, now time.Time) *AuditLog {
	return &AuditLog{
		ID:         id,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Operator:   operator,
		Detail:     detail,
		CreatedAt:  now,
	}
}
