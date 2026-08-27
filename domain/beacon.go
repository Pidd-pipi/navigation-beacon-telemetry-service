package domain

import (
	"fmt"
	"math"
	"time"
)

// Beacon 航标灯台账实体。
type Beacon struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Type            BeaconType   `json:"type"`
	Anchor          Position     `json:"anchor"`
	DriftRadiusM    float64      `json:"drift_radius_m"`
	LampPattern     LampPattern  `json:"lamp_pattern"`
	Status          BeaconStatus `json:"status"`
	LowPower        bool         `json:"low_power"`
	Drifting        bool         `json:"drifting"`
	LampOffSince    *time.Time   `json:"lamp_off_since,omitempty"`
	LowVoltSince    *time.Time   `json:"low_volt_since,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	LastTelemetryAt *time.Time   `json:"last_telemetry_at,omitempty"`
}

// NewBeacon 构造航标灯实体并初始化时间戳。
func NewBeacon(id, name string, bt BeaconType, anchor Position, driftRadiusM float64,
	lampPattern LampPattern, now time.Time) *Beacon {
	return &Beacon{
		ID:           id,
		Name:         name,
		Type:         bt,
		Anchor:       anchor,
		DriftRadiusM: driftRadiusM,
		LampPattern:  lampPattern,
		Status:       BeaconStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Validate 校验台账字段完整性。
func (b *Beacon) Validate() error {
	if b.ID == "" {
		return fmt.Errorf("beacon id 不能为空")
	}
	if b.Name == "" {
		return fmt.Errorf("beacon name 不能为空")
	}
	if !b.Type.Valid() {
		return fmt.Errorf("无效的航标灯类型 %q", b.Type)
	}
	if !b.Anchor.Valid() {
		return fmt.Errorf("锚位坐标非法: %s", b.Anchor)
	}
	if math.IsNaN(b.DriftRadiusM) || math.IsInf(b.DriftRadiusM, 0) || b.DriftRadiusM <= 0 {
		return fmt.Errorf("漂移半径必须为大于 0 的有限数值")
	}
	if !b.LampPattern.Valid() {
		return fmt.Errorf("设定灯质非法: %s", b.LampPattern)
	}
	if b.Status != "" && !b.Status.Valid() {
		return fmt.Errorf("无效的运行状态 %q", b.Status)
	}
	return nil
}

// EffectiveStatus 依据最近遥测时间推导展示状态（离线判定）。
func (b *Beacon) EffectiveStatus(now time.Time, offlineAfter time.Duration) BeaconStatus {
	if b.LastTelemetryAt == nil || now.Sub(*b.LastTelemetryAt) > offlineAfter {
		return BeaconStatusOffline
	}
	return BeaconStatusActive
}

// LampOffDuration 返回当前连续灭灯时长，未灭灯时返回 0。
func (b *Beacon) LampOffDuration(now time.Time) time.Duration {
	if b.LampOffSince == nil {
		return 0
	}
	d := now.Sub(*b.LampOffSince)
	if d < 0 {
		return 0
	}
	return d
}

// HasOpenLampOut 通过外部异常集合判断当前是否存在未解决的灭灯异常。
// 供总览聚合与详情展示使用。
func (b *Beacon) HasOpenLampOut(openAbnormalities []*LampAbnormality) bool {
	for _, a := range openAbnormalities {
		if a.BeaconID == b.ID && a.Type == AbnormalityTypeLampOut && a.Status == AbnormalityStatusOpen {
			return true
		}
	}
	return false
}
