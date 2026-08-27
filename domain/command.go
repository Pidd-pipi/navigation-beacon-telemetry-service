package domain

import (
	"fmt"
	"time"
)

// RemoteCommand 遥控指令实体：开灯/关灯/切换灯质，回执与自动重发。
type RemoteCommand struct {
	ID            string       `json:"id"`
	BeaconID      string       `json:"beacon_id"`
	Type          CommandType  `json:"type"`
	TargetPattern *LampPattern `json:"target_pattern,omitempty"`
	Status        AckStatus    `json:"status"`
	RetryCount    int          `json:"retry_count"`
	SentAt        time.Time    `json:"sent_at"`
	Deadline      time.Time    `json:"deadline"`
	AckAt         *time.Time   `json:"ack_at,omitempty"`
	AckMessage    string       `json:"ack_message,omitempty"`
	LastError     string       `json:"last_error,omitempty"`
	Operator      string       `json:"operator,omitempty"`
}

// NewRemoteCommand 构造遥控指令，初始状态 pending。
func NewRemoteCommand(id, beaconID string, ct CommandType, target *LampPattern,
	operator string, sentAt, deadline time.Time) *RemoteCommand {
	return &RemoteCommand{
		ID:            id,
		BeaconID:      beaconID,
		Type:          ct,
		TargetPattern: target,
		Status:        AckStatusPending,
		SentAt:        sentAt,
		Deadline:      deadline,
		Operator:      operator,
	}
}

// Validate 校验指令字段。
func (c *RemoteCommand) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("command id 不能为空")
	}
	if c.BeaconID == "" {
		return fmt.Errorf("beacon_id 不能为空")
	}
	if !c.Type.Valid() {
		return fmt.Errorf("无效的指令类型 %q", c.Type)
	}
	if c.Type == CommandTypeSwitchPattern && c.TargetPattern == nil {
		return fmt.Errorf("切换灯质指令必须携带 target_pattern")
	}
	if c.TargetPattern != nil && !c.TargetPattern.Valid() {
		return fmt.Errorf("目标灯质非法: %s", c.TargetPattern)
	}
	if !c.Status.Valid() {
		return fmt.Errorf("无效的回执状态 %q", c.Status)
	}
	return nil
}

// IsOverdue 判断指令是否超过回执期限。
func (c *RemoteCommand) IsOverdue(now time.Time) bool {
	return c.Status == AckStatusPending && now.After(c.Deadline)
}

// Pending 判断指令是否等待回执。
func (c *RemoteCommand) Pending() bool { return c.Status == AckStatusPending }

// Resend 执行一次自动重发：重试次数 +1 并刷新回执期限。
func (c *RemoteCommand) Resend(now time.Time, timeout time.Duration) {
	c.RetryCount++
	c.SentAt = now
	c.Deadline = now.Add(timeout)
	c.LastError = fmt.Sprintf("第 %d 次重发，等待回执", c.RetryCount)
}

// Ack 处理终端回执。
func (c *RemoteCommand) Ack(status AckStatus, message string, now time.Time) error {
	if !c.Pending() {
		return fmt.Errorf("指令 %s 回执状态为 %s，不可重复回执", c.ID, c.Status)
	}
	if status != AckStatusSuccess && status != AckStatusFailed {
		return fmt.Errorf("非法回执状态 %q", status)
	}
	c.Status = status
	c.AckAt = &now
	c.AckMessage = message
	if status == AckStatusFailed {
		c.LastError = message
	}
	return nil
}

// MarkFailed 指令在重试耗尽后标记失败。
func (c *RemoteCommand) MarkFailed(reason string, now time.Time) {
	c.Status = AckStatusFailed
	c.AckAt = &now
	c.AckMessage = reason
	c.LastError = reason
}
