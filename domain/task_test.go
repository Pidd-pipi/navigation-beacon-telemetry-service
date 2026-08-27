package domain

import (
	"testing"
	"time"
)

func newTestTask() *DisposalTask {
	now := time.Now()
	return NewDisposalTask("TA-001", "A-001", "B-001", "测试任务", TaskLevelNormal, now.Add(4*time.Hour), now)
}

func TestTaskStateMachineHappyPath(t *testing.T) {
	task := newTestTask()
	now := time.Now()

	if err := task.Assign("张三", now); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if task.Status != TaskStatusAssigned {
		t.Fatalf("状态应为 assigned，got %s", task.Status)
	}
	if err := task.Repair("已修复", now); err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if err := task.Verify("复测正常", now); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := task.Close(now); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if task.Status != TaskStatusClosed {
		t.Fatalf("状态应为 closed，got %s", task.Status)
	}
}

func TestTaskStateMachineInvalidTransitions(t *testing.T) {
	task := newTestTask()
	now := time.Now()

	// created 不允许直接修复
	if err := task.Repair("note", now); err == nil {
		t.Error("created 直接 Repair 应报错")
	}
	// created 不允许直接复测
	if err := task.Verify("ok", now); err == nil {
		t.Error("created 直接 Verify 应报错")
	}
	// assigned 不允许重复派发
	if err := task.Assign("张三", now); err != nil {
		t.Fatalf("首次 Assign: %v", err)
	}
	if err := task.Assign("李四", now); err == nil {
		t.Error("assigned 重复 Assign 应报错")
	}
	// verified 不允许直接 repair
	task.Status = TaskStatusVerified
	if err := task.Repair("note", now); err == nil {
		t.Error("verified 直接 Repair 应报错")
	}
}

func TestTaskAssignRequiresAssignee(t *testing.T) {
	task := newTestTask()
	if err := task.Assign("", time.Now()); err == nil {
		t.Error("空派发人应报错")
	}
}

func TestTaskOverdueAndEscalate(t *testing.T) {
	task := newTestTask()
	now := time.Now()
	// 未到期限
	if task.IsOverdue(now) {
		t.Error("未到期限不应超期")
	}
	// 超过期限
	if !task.IsOverdue(now.Add(5 * time.Hour)) {
		t.Error("超过期限应判定超期")
	}
	if err := task.Escalate(now); err != nil {
		t.Fatalf("Escalate: %v", err)
	}
	if task.Level != TaskLevelUrgent || !task.Escalated {
		t.Error("升级后应为 urgent 且 escalated=true")
	}
	// 已关闭任务不超期
	closed := newTestTask()
	closed.Status = TaskStatusClosed
	if closed.IsOverdue(now.Add(100 * time.Hour)) {
		t.Error("已关闭任务不应判定超期")
	}
}

func TestStatusFlow(t *testing.T) {
	flow := StatusFlow()
	want := []TaskStatus{TaskStatusCreated, TaskStatusAssigned, TaskStatusRepaired, TaskStatusVerified, TaskStatusClosed}
	if len(flow) != len(want) {
		t.Fatalf("状态机长度 %d != %d", len(flow), len(want))
	}
	for i := range want {
		if flow[i] != want[i] {
			t.Errorf("flow[%d] = %s, want %s", i, flow[i], want[i])
		}
	}
}
