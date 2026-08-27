// Package domain 定义航标灯遥测遥控服务的核心领域模型。
//
// 枚举与常量要求前后端保持一致，README 中已列出所有出现位置。
package domain

import "fmt"

// BeaconType 航标灯类型。
type BeaconType string

const (
	BeaconTypeLighthouse BeaconType = "lighthouse" // 灯塔
	BeaconTypeBuoy       BeaconType = "buoy"       // 浮标
	BeaconTypeDaybeacon  BeaconType = "daybeacon"  // 导标
)

// Valid 校验航标灯类型是否合法。
func (t BeaconType) Valid() bool {
	switch t {
	case BeaconTypeLighthouse, BeaconTypeBuoy, BeaconTypeDaybeacon:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (t BeaconType) Label() string {
	switch t {
	case BeaconTypeLighthouse:
		return "灯塔"
	case BeaconTypeBuoy:
		return "浮标"
	case BeaconTypeDaybeacon:
		return "导标"
	}
	return string(t)
}

// BeaconStatus 航标灯运行状态。
type BeaconStatus string

const (
	BeaconStatusActive  BeaconStatus = "active"  // 在线
	BeaconStatusOffline BeaconStatus = "offline" // 离线
)

// Valid 校验运行状态是否合法。
func (s BeaconStatus) Valid() bool {
	switch s {
	case BeaconStatusActive, BeaconStatusOffline:
		return true
	}
	return false
}

// AbnormalityType 异常类型。
type AbnormalityType string

const (
	AbnormalityTypeLampMismatch AbnormalityType = "lamp_mismatch" // 灯质偏差
	AbnormalityTypeLampOut      AbnormalityType = "lamp_out"      // 灭灯
	AbnormalityTypeLowVoltage   AbnormalityType = "low_voltage"   // 低电压
	AbnormalityTypeDrift        AbnormalityType = "drift"         // 漂移
)

// Valid 校验异常类型是否合法。
func (t AbnormalityType) Valid() bool {
	switch t {
	case AbnormalityTypeLampMismatch, AbnormalityTypeLampOut, AbnormalityTypeLowVoltage, AbnormalityTypeDrift:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (t AbnormalityType) Label() string {
	switch t {
	case AbnormalityTypeLampMismatch:
		return "灯质偏差"
	case AbnormalityTypeLampOut:
		return "灭灯"
	case AbnormalityTypeLowVoltage:
		return "低电压"
	case AbnormalityTypeDrift:
		return "漂移"
	}
	return string(t)
}

// AbnormalityStatus 异常状态。
type AbnormalityStatus string

const (
	AbnormalityStatusOpen     AbnormalityStatus = "open"     // 未解决
	AbnormalityStatusResolved AbnormalityStatus = "resolved" // 已解决
)

// Valid 校验异常状态是否合法。
func (s AbnormalityStatus) Valid() bool {
	switch s {
	case AbnormalityStatusOpen, AbnormalityStatusResolved:
		return true
	}
	return false
}

// TaskStatus 处置任务状态（状态机）。
type TaskStatus string

const (
	TaskStatusCreated  TaskStatus = "created"  // 已生成
	TaskStatusAssigned TaskStatus = "assigned" // 已派发
	TaskStatusRepaired TaskStatus = "repaired" // 已修复
	TaskStatusVerified TaskStatus = "verified" // 已复测
	TaskStatusClosed   TaskStatus = "closed"   // 已关闭
)

// Valid 校验任务状态是否合法。
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusCreated, TaskStatusAssigned, TaskStatusRepaired, TaskStatusVerified, TaskStatusClosed:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (s TaskStatus) Label() string {
	switch s {
	case TaskStatusCreated:
		return "已生成"
	case TaskStatusAssigned:
		return "已派发"
	case TaskStatusRepaired:
		return "已修复"
	case TaskStatusVerified:
		return "已复测"
	case TaskStatusClosed:
		return "已关闭"
	}
	return string(s)
}

// TaskTransitions 任务状态机合法迁移表。
var TaskTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusCreated:  {TaskStatusAssigned},
	TaskStatusAssigned: {TaskStatusRepaired},
	TaskStatusRepaired: {TaskStatusVerified},
	TaskStatusVerified: {TaskStatusClosed},
}

// TaskLevel 任务级别。
type TaskLevel string

const (
	TaskLevelNormal TaskLevel = "normal" // 普通
	TaskLevelUrgent TaskLevel = "urgent" // 紧急（超时升级）
)

// Valid 校验任务级别是否合法。
func (l TaskLevel) Valid() bool {
	switch l {
	case TaskLevelNormal, TaskLevelUrgent:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (l TaskLevel) Label() string {
	switch l {
	case TaskLevelNormal:
		return "普通"
	case TaskLevelUrgent:
		return "紧急"
	}
	return string(l)
}

// AckStatus 指令回执状态。
type AckStatus string

const (
	AckStatusPending AckStatus = "pending" // 等待回执
	AckStatusSuccess AckStatus = "success" // 回执成功
	AckStatusFailed  AckStatus = "failed"  // 回执失败/超时失败
)

// Valid 校验回执状态是否合法。
func (s AckStatus) Valid() bool {
	switch s {
	case AckStatusPending, AckStatusSuccess, AckStatusFailed:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (s AckStatus) Label() string {
	switch s {
	case AckStatusPending:
		return "待回执"
	case AckStatusSuccess:
		return "成功"
	case AckStatusFailed:
		return "失败"
	}
	return string(s)
}

// CommandType 遥控指令类型。
type CommandType string

const (
	CommandTypeOn            CommandType = "on"             // 开灯
	CommandTypeOff           CommandType = "off"            // 关灯
	CommandTypeSwitchPattern CommandType = "switch_pattern" // 切换灯质
)

// Valid 校验指令类型是否合法。
func (t CommandType) Valid() bool {
	switch t {
	case CommandTypeOn, CommandTypeOff, CommandTypeSwitchPattern:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (t CommandType) Label() string {
	switch t {
	case CommandTypeOn:
		return "开灯"
	case CommandTypeOff:
		return "关灯"
	case CommandTypeSwitchPattern:
		return "切换灯质"
	}
	return string(t)
}

// LampState 灯状态（实测）。
type LampState string

const (
	LampStateOn  LampState = "on"  // 亮
	LampStateOff LampState = "off" // 灭
)

// Valid 校验灯状态是否合法。
func (s LampState) Valid() bool {
	switch s {
	case LampStateOn, LampStateOff:
		return true
	}
	return false
}

// Label 返回中文名称，供前端展示。
func (s LampState) Label() string {
	switch s {
	case LampStateOn:
		return "亮"
	case LampStateOff:
		return "灭"
	}
	return string(s)
}

// ParseBeaconType 解析字符串为航标灯类型。
func ParseBeaconType(s string) (BeaconType, error) {
	t := BeaconType(s)
	if !t.Valid() {
		return "", fmt.Errorf("无效的航标灯类型 %q（可选值: lighthouse/buoy/daybeacon）", s)
	}
	return t, nil
}

// ParseAbnormalityType 解析字符串为异常类型。
func ParseAbnormalityType(s string) (AbnormalityType, error) {
	t := AbnormalityType(s)
	if !t.Valid() {
		return "", fmt.Errorf("无效的异常类型 %q（可选值: lamp_mismatch/lamp_out/low_voltage/drift）", s)
	}
	return t, nil
}

// ParseTaskStatus 解析字符串为任务状态。
func ParseTaskStatus(s string) (TaskStatus, error) {
	t := TaskStatus(s)
	if !t.Valid() {
		return "", fmt.Errorf("无效的任务状态 %q（可选值: created/assigned/repaired/verified/closed）", s)
	}
	return t, nil
}

// ParseCommandType 解析字符串为指令类型。
func ParseCommandType(s string) (CommandType, error) {
	t := CommandType(s)
	if !t.Valid() {
		return "", fmt.Errorf("无效的指令类型 %q（可选值: on/off/switch_pattern）", s)
	}
	return t, nil
}

// ParseAckStatus 解析字符串为回执状态。
func ParseAckStatus(s string) (AckStatus, error) {
	t := AckStatus(s)
	if !t.Valid() {
		return "", fmt.Errorf("无效的回执状态 %q（可选值: pending/success/failed）", s)
	}
	return t, nil
}
