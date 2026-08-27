package service

import (
	"log"
	"os"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

func TestSweeperRunOnce(t *testing.T) {
	st, cfg, tel, _, tasks, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}

	// 触发灭灯故障生成任务
	t1 := now.Add(-40 * time.Minute)
	t2 := now.Add(-5 * time.Minute)
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t1), "t", now); err != nil {
		t.Fatal(err)
	}
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t2), "t", now); err != nil {
		t.Fatal(err)
	}
	ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampOut)
	task := st.FindTaskByAbnormality(ab.ID)
	setTaskDeadline(t, st, task.ID, now.Add(-time.Minute)) // 已超期

	// 下发一条未回执指令并置为超期
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now)
	if err != nil {
		t.Fatal(err)
	}
	setCommandDeadline(t, st, cmd.ID, now.Add(-time.Minute))

	logger := log.New(os.Stderr, "[test-sweeper] ", 0)
	sweeper := NewSweeper(commands, tasks, cfg, logger)
	// 首次运行：因 lastEscalation 初始为 zero，会执行升级扫描
	resent, failed, escalated := sweeper.RunOnce(now)
	if resent != 1 {
		t.Errorf("应重发 1 条指令，got %d", resent)
	}
	if failed != 0 {
		t.Errorf("首次扫描不应有失败指令，got %d", failed)
	}
	if escalated != 1 {
		t.Errorf("应升级 1 个超期任务，got %d", escalated)
	}
	if st.GetCommand(cmd.ID).RetryCount != 1 {
		t.Errorf("指令重试次数应为 1，got %d", st.GetCommand(cmd.ID).RetryCount)
	}
	if st.GetTask(task.ID).Level != domain.TaskLevelUrgent {
		t.Error("任务应升级为 urgent")
	}

	// 第二次立即运行：升级扫描未到周期（TaskEscalationScan=10m），不应再次升级
	resent, failed, escalated = sweeper.RunOnce(now.Add(time.Second))
	if escalated != 0 {
		t.Errorf("未到升级扫描周期不应再次升级，got %d", escalated)
	}
}
