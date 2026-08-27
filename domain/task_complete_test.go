package domain

import (
	"testing"
	"time"
)

// TestCompleteClosesFromAnyState Complete 应从任意未关闭状态收敛到 closed。
func TestCompleteClosesFromAnyState(t *testing.T) {
	now := time.Now()
	for _, st := range []TaskStatus{TaskStatusCreated, TaskStatusAssigned, TaskStatusRepaired, TaskStatusVerified} {
		task := NewDisposalTask("TA-1", "A-1", "B-1", "处置任务", TaskLevelNormal, now.Add(4*time.Hour), now)
		task.Status = st
		if err := task.Complete(now); err != nil {
			t.Fatalf("状态 %s Complete 应成功: %v", st, err)
		}
		if task.Status != TaskStatusClosed || task.ClosedAt == nil {
			t.Fatalf("状态 %s Complete 后应为 closed 且记录关闭时间", st)
		}
	}
}

// TestCompleteRejectsClosed Complete 已关闭任务应报错。
func TestCompleteRejectsClosed(t *testing.T) {
	now := time.Now()
	task := NewDisposalTask("TA-1", "A-1", "B-1", "处置任务", TaskLevelNormal, now.Add(4*time.Hour), now)
	task.Status = TaskStatusClosed
	if err := task.Complete(now); err == nil {
		t.Fatal("已关闭任务 Complete 应报错")
	}
}
