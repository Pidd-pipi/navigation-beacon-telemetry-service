package service

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// newCanceledCtx 返回已取消的 context。
func newCanceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestSweeperStopsResendAfterCancel ctx 取消后 RunOnce 不应再重发指令。
func TestSweeperStopsResendAfterCancel(t *testing.T) {
	st, cfg, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now)
	if err != nil {
		t.Fatal(err)
	}
	setCommandDeadline(t, st, cmd.ID, now.Add(-time.Minute))

	sw := &Sweeper{
		commandSvc: commands,
		taskSvc:    NewTaskService(st, nil, cfg),
		cfg:        cfg,
		logger:     slog.Default(),
		ctx:        newCanceledCtx(),
	}
	resent, _, _ := sw.RunOnce(now)
	if resent != 0 {
		t.Fatalf("ctx 取消后不应重发指令，resent=%d", resent)
	}
}

// TestSweeperStopsEscalationAfterCancel ctx 取消后 RunOnce 不应升级任务。
func TestSweeperStopsEscalationAfterCancel(t *testing.T) {
	st, cfg, tel, _, tasks, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	t1 := now.Add(-40 * time.Minute)
	t2 := now.Add(-5 * time.Minute)
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t1), "t", now); err != nil {
		t.Fatal(err)
	}
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t2), "t", now); err != nil {
		t.Fatal(err)
	}
	ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampOut)
	var task *domain.DisposalTask
	for _, tk := range tasks.List(TaskFilter{}) {
		if tk.AbnormalityID == ab.ID {
			task = tk
			break
		}
	}
	setTaskDeadline(t, st, task.ID, now.Add(-time.Minute))

	sw := &Sweeper{
		commandSvc: NewCommandService(st, nil, cfg),
		taskSvc:    tasks,
		cfg:        cfg,
		logger:     slog.Default(),
		ctx:        newCanceledCtx(),
	}
	_, _, escalated := sw.RunOnce(now)
	if escalated != 0 {
		t.Fatalf("ctx 取消后不应升级任务，escalated=%d", escalated)
	}
}

// TestSweeperCanceledRunOnceSkipsBookkeeping ctx 取消后 RunOnce 不应推进扫描时间。
func TestSweeperCanceledRunOnceSkipsBookkeeping(t *testing.T) {
	st, cfg, _, _, tasks, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	sw := &Sweeper{
		commandSvc: commands,
		taskSvc:    tasks,
		cfg:        cfg,
		logger:     slog.Default(),
		ctx:        newCanceledCtx(),
	}
	sw.lastEscalation = time.Time{}
	sw.RunOnce(time.Now())
	if !sw.lastEscalation.IsZero() {
		t.Fatal("ctx 取消后 RunOnce 不应推进扫描时间")
	}
}

// TestSweeperStartStoresCtx Start 必须保存 ctx，取消后 RunOnce 立即收敛。
func TestSweeperStartStoresCtx(t *testing.T) {
	st, cfg, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now)
	if err != nil {
		t.Fatal(err)
	}
	setCommandDeadline(t, st, cmd.ID, now.Add(-time.Minute))

	sw := NewSweeperSlog(commands, NewTaskService(st, nil, cfg), cfg, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	sw.Start(ctx, &wg)
	cancel()
	wg.Wait()

	resent, _, _ := sw.RunOnce(now)
	if resent != 0 {
		t.Fatalf("Start 保存 ctx 后取消，RunOnce 不应重发，resent=%d", resent)
	}
}
