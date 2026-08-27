package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// TestResolveClosesLinkedTask 手工解决异常后，关联的自动任务必须关闭。
func TestResolveClosesLinkedTask(t *testing.T) {
	st, _, tel, abn, tasks, _, _ := newTestServices(t)
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
	if ab == nil {
		t.Fatal("无灭灯异常")
	}
	var task *domain.DisposalTask
	for _, tk := range tasks.List(TaskFilter{}) {
		if tk.AbnormalityID == ab.ID {
			task = tk
			break
		}
	}
	if task == nil {
		t.Fatal("无自动任务")
	}

	if _, err := abn.Resolve(ab.ID, "现场核实已恢复", "op", now); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	after, _ := tasks.Get(task.ID)
	if after == nil || after.Status != domain.TaskStatusClosed {
		t.Fatalf("异常解决后任务应关闭，got %+v", after)
	}
}

// TestResolveClosesAssignedTask 任务已派发时解决异常，任务也应关闭。
func TestResolveClosesAssignedTask(t *testing.T) {
	st, _, tel, abn, tasks, _, _ := newTestServices(t)
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
	if _, err := tasks.Assign(task.ID, "张三", now); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if _, err := abn.Resolve(ab.ID, "现场核实已恢复", "op", now); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	after, _ := tasks.Get(task.ID)
	if after == nil || after.Status != domain.TaskStatusClosed {
		t.Fatalf("已派发任务也应随异常解决关闭，got %+v", after)
	}
}

// TestAutoResolveClosesLinkedTask 遥测恢复自动解决异常后，关联任务必须关闭。
func TestAutoResolveClosesLinkedTask(t *testing.T) {
	st, _, tel, _, tasks, _, _ := newTestServices(t)
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
	if task == nil {
		t.Fatal("无自动任务")
	}

	// 灯恢复 → AutoResolve
	pattern := domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.0, anchor, &pattern, &now), "t", now); err != nil {
		t.Fatal(err)
	}
	after, _ := tasks.Get(task.ID)
	if after == nil || after.Status != domain.TaskStatusClosed {
		t.Fatalf("自动解决异常后任务应关闭，got %+v", after)
	}
}
