package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// newTestServices 构造一套无持久化的内存测试服务。
func newTestServices(t *testing.T) (*store.Store, *config.Config, *TelemetryService, *AbnormalityService, *TaskService, *CommandService, *AuditService) {
	t.Helper()
	cfg := config.Default()
	cfg.DataFile = "" // 测试不落盘
	st := store.New("")
	audit := NewAuditService(st, cfg)
	tasks := NewTaskService(st, audit, cfg)
	abn := NewAbnormalityService(st, tasks, audit, cfg)
	cmd := NewCommandService(st, audit, cfg)
	tel := NewTelemetryService(st, abn, audit, cfg)
	return st, cfg, tel, abn, tasks, cmd, audit
}

// seedTestBeacon 写入一座测试航标。
func seedTestBeacon(t *testing.T, st *store.Store, id string) *domain.Beacon {
	t.Helper()
	b := domain.NewBeacon(id, "测试航标-"+id, domain.BeaconTypeLighthouse,
		domain.Position{Lat: 30.5, Lng: 122.1}, 50,
		domain.LampPattern{FlashSec: 2, EclipseSec: 2}, time.Now())
	if err := st.CreateBeacon(b); err != nil {
		t.Fatalf("创建测试航标失败: %v", err)
	}
	return b
}

// setBeaconDrifting 强制设置航标漂移标记。
func setBeaconDrifting(t *testing.T, st *store.Store, id string, drifting bool) {
	t.Helper()
	b := st.GetBeacon(id)
	if b == nil {
		t.Fatalf("航标 %s 不存在", id)
	}
	cp := *b
	cp.Drifting = drifting
	if err := st.UpdateBeacon(&cp); err != nil {
		t.Fatalf("更新航标失败: %v", err)
	}
}

// setCommandDeadline 覆盖指令回执期限（模拟超时）。
func setCommandDeadline(t *testing.T, st *store.Store, id string, deadline time.Time) {
	t.Helper()
	c := st.GetCommand(id)
	if c == nil {
		t.Fatalf("指令 %s 不存在", id)
	}
	cp := *c
	cp.Deadline = deadline
	cp.SentAt = deadline.Add(-1 * time.Minute)
	if err := st.UpsertCommand(&cp); err != nil {
		t.Fatalf("更新指令失败: %v", err)
	}
}

// setTaskDeadline 覆盖任务期限（模拟超期）。
func setTaskDeadline(t *testing.T, st *store.Store, id string, deadline time.Time) {
	t.Helper()
	tk := st.GetTask(id)
	if tk == nil {
		t.Fatalf("任务 %s 不存在", id)
	}
	cp := *tk
	cp.Deadline = deadline
	if err := st.UpsertTask(&cp); err != nil {
		t.Fatalf("更新任务失败: %v", err)
	}
}

// telemetryInput 构造一条灯亮正常遥测输入。
func telemetryInput(lamp domain.LampState, voltage float64, pos domain.Position, pattern *domain.LampPattern, reportedAt *time.Time) TelemetryInput {
	return TelemetryInput{
		LampState:       lamp,
		Voltage:         voltage,
		Current:         0.8,
		Position:        pos,
		MeasuredPattern: pattern,
		ReportedAt:      reportedAt,
	}
}
