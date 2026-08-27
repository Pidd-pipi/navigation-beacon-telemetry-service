package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

func TestTaskLifecycleThroughService(t *testing.T) {
	st, _, tel, _, tasks, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	// 触发灭灯故障 → 自动生成任务
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

	// created → assigned
	task, err := tasks.Assign(task.ID, "张三", now)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if task.Status != domain.TaskStatusAssigned {
		t.Errorf("状态应为 assigned，got %s", task.Status)
	}

	// assigned → repaired
	task, err = tasks.Repair(task.ID, "更换灯器", "李四", now)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if task.Status != domain.TaskStatusRepaired {
		t.Errorf("状态应为 repaired，got %s", task.Status)
	}

	// repaired → verified → closed（复测并关闭）
	task, err = tasks.Verify(task.ID, "灯质复测正常", true, "王五", now)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if task.Status != domain.TaskStatusClosed {
		t.Errorf("复测并关闭后状态应为 closed，got %s", task.Status)
	}
	if task.VerifyResult == "" {
		t.Error("复测结果应被记录")
	}
	if task.ClosedAt == nil {
		t.Error("关闭时间应被记录")
	}
}

func TestTaskInvalidTransitions(t *testing.T) {
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
	task := st.FindTaskByAbnormality(ab.ID)

	// created 直接修复应冲突
	if _, err := tasks.Repair(task.ID, "note", "t", now); err == nil {
		t.Error("created 直接修复应报错")
	}
	// 空派发人应冲突
	if _, err := tasks.Assign(task.ID, "", now); err == nil {
		t.Error("空派发人应报错")
	}
	// 正常派发后重复派发应冲突
	if _, err := tasks.Assign(task.ID, "张三", now); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Assign(task.ID, "李四", now); err == nil {
		t.Error("重复派发应报错")
	}
}

func TestTaskEscalateOverdue(t *testing.T) {
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
	task := st.FindTaskByAbnormality(ab.ID)

	// 未超期不升级
	n, err := tasks.EscalateOverdue(now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("未超期不应升级，升级 %d 个", n)
	}
	if st.GetTask(task.ID).Level != domain.TaskLevelNormal {
		t.Error("未超期级别应为 normal")
	}

	// 超过 4 小时期限 → 升级
	setTaskDeadline(t, st, task.ID, now.Add(-time.Minute))
	n, err = tasks.EscalateOverdue(now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("应升级 1 个任务，got %d", n)
	}
	upgraded := st.GetTask(task.ID)
	if upgraded.Level != domain.TaskLevelUrgent || !upgraded.Escalated {
		t.Errorf("升级后应为 urgent+escalated，got %s/%v", upgraded.Level, upgraded.Escalated)
	}
}

func TestTaskCreateManualAndDuplicate(t *testing.T) {
	st, _, _, abn, tasks, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	ab, err := abn.CreateManual("B-001", domain.AbnormalityTypeLowVoltage, "电池老化", "t", now)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	task, err := tasks.CreateManual(ab.ID, now)
	if err != nil {
		t.Fatalf("CreateManual task: %v", err)
	}
	if task.Status != domain.TaskStatusCreated {
		t.Errorf("任务初始状态应为 created，got %s", task.Status)
	}
	// 已存在进行中任务时再次生成应冲突
	if _, err := tasks.CreateManual(ab.ID, now); err == nil {
		t.Error("重复生成任务应报错")
	}
	// 不存在的异常
	if _, err := tasks.CreateManual("A-999", now); err == nil {
		t.Error("不存在的异常应报错")
	}
}
