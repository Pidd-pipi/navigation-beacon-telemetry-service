package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

func TestCommandDispatchAndAck(t *testing.T) {
	st, _, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "operator", now)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if cmd.Status != domain.AckStatusPending {
		t.Errorf("下发后状态应为 pending，got %s", cmd.Status)
	}
	if cmd.Deadline.Sub(now) != 5*time.Minute {
		t.Errorf("回执期限应为 5 分钟，got %v", cmd.Deadline.Sub(now))
	}

	// 成功回执
	acked, err := commands.Ack(cmd.ID, true, "终端已执行", "terminal", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if acked.Status != domain.AckStatusSuccess {
		t.Errorf("回执后状态应为 success，got %s", acked.Status)
	}
	// 重复回执应冲突
	if _, err := commands.Ack(cmd.ID, false, "again", "terminal", now.Add(2*time.Minute)); err == nil {
		t.Error("重复回执应报错")
	}
}

func TestCommandDriftGuardBlocksOff(t *testing.T) {
	st, _, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")
	setBeaconDrifting(t, st, "B-001", true)

	now := time.Now()
	// 漂移期间关灯被拦截
	if _, err := commands.Dispatch("B-001", domain.CommandTypeOff, nil, "op", now); err == nil {
		t.Error("漂移期间关灯应被拦截")
	}
	// 漂移期间开灯/切换灯质不受影响
	if _, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now); err != nil {
		t.Errorf("漂移期间开灯不应被拦截: %v", err)
	}
	if _, err := commands.Dispatch("B-001", domain.CommandTypeSwitchPattern, &domain.LampPattern{FlashSec: 3, EclipseSec: 3}, "op", now); err != nil {
		t.Errorf("漂移期间切换灯质不应被拦截: %v", err)
	}
}

func TestCommandSwitchPatternRequiresTarget(t *testing.T) {
	st, _, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")
	if _, err := commands.Dispatch("B-001", domain.CommandTypeSwitchPattern, nil, "op", time.Now()); err == nil {
		t.Error("切换灯质缺目标灯质应报错")
	}
}

func TestCommandResendAndFail(t *testing.T) {
	st, _, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now)
	if err != nil {
		t.Fatal(err)
	}

	// 未超时不应重发
	resent, failed := commands.ResendDue(now.Add(4 * time.Minute))
	if resent != 0 || failed != 0 {
		t.Errorf("未超时不应重发: resent=%d failed=%d", resent, failed)
	}

	// 第一次超时 → 重发 1 次
	setCommandDeadline(t, st, cmd.ID, now.Add(-1*time.Minute))
	resent, failed = commands.ResendDue(now)
	if resent != 1 || failed != 0 {
		t.Fatalf("第一次超时应重发: resent=%d failed=%d", resent, failed)
	}
	if got := st.GetCommand(cmd.ID).RetryCount; got != 1 {
		t.Errorf("重试次数应为 1，got %d", got)
	}

	// 第二次超时 → 重发 1 次（共 2 次）
	setCommandDeadline(t, st, cmd.ID, now.Add(-1*time.Minute))
	resent, failed = commands.ResendDue(now)
	if resent != 1 || failed != 0 {
		t.Fatalf("第二次超时应重发: resent=%d failed=%d", resent, failed)
	}
	if got := st.GetCommand(cmd.ID).RetryCount; got != 2 {
		t.Errorf("重试次数应为 2，got %d", got)
	}

	// 第三次超时 → 重试耗尽，标记失败
	setCommandDeadline(t, st, cmd.ID, now.Add(-1*time.Minute))
	resent, failed = commands.ResendDue(now)
	if resent != 0 || failed != 1 {
		t.Fatalf("重试耗尽应标记失败: resent=%d failed=%d", resent, failed)
	}
	if got := st.GetCommand(cmd.ID).Status; got != domain.AckStatusFailed {
		t.Errorf("状态应为 failed，got %s", got)
	}
}

func TestCommandNotFound(t *testing.T) {
	_, _, _, _, _, commands, _ := newTestServices(t)
	if _, err := commands.Ack("C-999", true, "x", "t", time.Now()); err == nil {
		t.Error("不存在的指令回执应报错")
	}
}
