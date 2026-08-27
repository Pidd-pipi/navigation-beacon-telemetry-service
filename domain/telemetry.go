package domain

import (
	"fmt"
	"math"
	"time"
)

// TelemetryData 遥测数据实体：终端按周期回传的灯状态、电气与位置数据。
type TelemetryData struct {
	ID              string       `json:"id"`
	BeaconID        string       `json:"beacon_id"`
	LampState       LampState    `json:"lamp_state"`
	Voltage         float64      `json:"voltage"`
	Current         float64      `json:"current"`
	Position        Position     `json:"position"`
	MeasuredPattern *LampPattern `json:"measured_pattern,omitempty"`
	ReportedAt      time.Time    `json:"reported_at"`
	ReceivedAt      time.Time    `json:"received_at"`
	Violations      []string     `json:"violations,omitempty"`
	SuggestedPeriod string       `json:"suggested_period,omitempty"`
}

// NewTelemetryData 构造遥测数据实体。
func NewTelemetryData(id, beaconID string, lampState LampState, voltage, current float64,
	pos Position, measured *LampPattern, reportedAt, receivedAt time.Time) *TelemetryData {
	return &TelemetryData{
		ID:              id,
		BeaconID:        beaconID,
		LampState:       lampState,
		Voltage:         voltage,
		Current:         current,
		Position:        pos,
		MeasuredPattern: measured,
		ReportedAt:      reportedAt,
		ReceivedAt:      receivedAt,
	}
}

// Validate 校验遥测字段。
// 灯亮时必须提供实测灯质；灯灭时实测灯质可省略。
func (t *TelemetryData) Validate() error {
	if t.BeaconID == "" {
		return fmt.Errorf("beacon_id 不能为空")
	}
	if !t.LampState.Valid() {
		return fmt.Errorf("无效的灯状态 %q", t.LampState)
	}
	if math.IsNaN(t.Voltage) || math.IsInf(t.Voltage, 0) || t.Voltage <= 0 {
		return fmt.Errorf("电压必须为大于 0 的有限数值")
	}
	if math.IsNaN(t.Current) || math.IsInf(t.Current, 0) || t.Current < 0 {
		return fmt.Errorf("电流必须为非负有限数值")
	}
	if !t.Position.Valid() {
		return fmt.Errorf("位置坐标非法: %s", t.Position)
	}
	if t.LampState == LampStateOn && t.MeasuredPattern == nil {
		return fmt.Errorf("灯亮时必须上报实测灯质 measured_pattern")
	}
	if t.MeasuredPattern != nil && !t.MeasuredPattern.Valid() {
		return fmt.Errorf("实测灯质非法: %s", t.MeasuredPattern)
	}
	if t.ReportedAt.IsZero() {
		return fmt.Errorf("上报时间不能为空")
	}
	return nil
}

// AddViolation 记录一条校验违规项。
func (t *TelemetryData) AddViolation(v string) {
	t.Violations = append(t.Violations, v)
}

// AddViolations 批量记录违规项。
func (t *TelemetryData) AddViolations(vs []string) {
	t.Violations = append(t.Violations, vs...)
}
