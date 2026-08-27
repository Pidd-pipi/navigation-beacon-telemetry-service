package domain

import (
	"testing"
	"time"
)

func TestNewRemoteCommandAndResend(t *testing.T) {
	now := time.Now()
	cmd := NewRemoteCommand("C-001", "B-001", CommandTypeOn, nil, "operator", now, now.Add(5*time.Minute))
	if !cmd.Pending() {
		t.Error("新指令应 pending")
	}
	if cmd.IsOverdue(now.Add(4 * time.Minute)) {
		t.Error("未到期限不应超时")
	}
	if !cmd.IsOverdue(now.Add(6 * time.Minute)) {
		t.Error("超过期限应超时")
	}

	cmd.Resend(now.Add(6*time.Minute), 5*time.Minute)
	if cmd.RetryCount != 1 {
		t.Errorf("重试次数应为 1，got %d", cmd.RetryCount)
	}
	if !cmd.Deadline.After(now.Add(6 * time.Minute)) {
		t.Error("重发后应刷新回执期限")
	}
}

func TestCommandAck(t *testing.T) {
	now := time.Now()
	cmd := NewRemoteCommand("C-001", "B-001", CommandTypeOff, nil, "op", now, now.Add(5*time.Minute))
	if err := cmd.Ack(AckStatusSuccess, "已执行", now.Add(1*time.Minute)); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if cmd.Status != AckStatusSuccess {
		t.Error("回执后状态应为 success")
	}
	// 重复回执应报错
	if err := cmd.Ack(AckStatusFailed, "again", now.Add(2*time.Minute)); err == nil {
		t.Error("重复回执应报错")
	}
}

func TestCommandMarkFailed(t *testing.T) {
	now := time.Now()
	cmd := NewRemoteCommand("C-001", "B-001", CommandTypeOn, nil, "op", now, now.Add(5*time.Minute))
	cmd.MarkFailed("重试耗尽", now.Add(20*time.Minute))
	if cmd.Status != AckStatusFailed {
		t.Error("MarkFailed 后状态应为 failed")
	}
	if cmd.LastError == "" {
		t.Error("LastError 不应为空")
	}
}

func TestCommandValidate(t *testing.T) {
	now := time.Now()
	// 切换灯质必须带目标灯质
	cmd := NewRemoteCommand("C-001", "B-001", CommandTypeSwitchPattern, nil, "op", now, now.Add(5*time.Minute))
	if err := cmd.Validate(); err == nil {
		t.Error("switch_pattern 缺 target_pattern 应报错")
	}
	// 合法切换
	cmd.TargetPattern = &LampPattern{FlashSec: 2, EclipseSec: 2}
	if err := cmd.Validate(); err != nil {
		t.Errorf("合法切换指令应通过校验: %v", err)
	}
}
