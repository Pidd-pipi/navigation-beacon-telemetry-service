package domain

import (
	"fmt"
	"time"
)

// LampAbnormality 灯质/状态异常实体：灯质偏差、灭灯、低电压、漂移。
type LampAbnormality struct {
	ID          string            `json:"id"`
	BeaconID    string            `json:"beacon_id"`
	Type        AbnormalityType   `json:"type"`
	Status      AbnormalityStatus `json:"status"`
	Detail      string            `json:"detail"`
	FirstSeenAt time.Time         `json:"first_seen_at"`
	LastSeenAt  time.Time         `json:"last_seen_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
}

// NewLampAbnormality 构造异常实体。
func NewLampAbnormality(id, beaconID string, at AbnormalityType, detail string, now time.Time) *LampAbnormality {
	return &LampAbnormality{
		ID:          id,
		BeaconID:    beaconID,
		Type:        at,
		Status:      AbnormalityStatusOpen,
		Detail:      detail,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
}

// Validate 校验异常字段。
func (a *LampAbnormality) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("abnormality id 不能为空")
	}
	if a.BeaconID == "" {
		return fmt.Errorf("beacon_id 不能为空")
	}
	if !a.Type.Valid() {
		return fmt.Errorf("无效的异常类型 %q", a.Type)
	}
	if !a.Status.Valid() {
		return fmt.Errorf("无效的异常状态 %q", a.Status)
	}
	return nil
}

// Touch 更新最后发现时间（用于异常持续存在时刷新）。
func (a *LampAbnormality) Touch(now time.Time) {
	a.LastSeenAt = now
	if a.Status != AbnormalityStatusOpen {
		a.Status = AbnormalityStatusOpen
		a.ResolvedAt = nil
	}
}

// Resolve 标记异常已解决。
func (a *LampAbnormality) Resolve(reason string, now time.Time) {
	a.Status = AbnormalityStatusResolved
	a.ResolvedAt = &now
	if reason != "" {
		a.Detail = a.Detail + "；解决原因: " + reason
	}
	a.LastSeenAt = now
}

// IsOpen 判断异常是否未解决。
func (a *LampAbnormality) IsOpen() bool { return a.Status == AbnormalityStatusOpen }
