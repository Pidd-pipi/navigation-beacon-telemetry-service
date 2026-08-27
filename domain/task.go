package domain

import (
	"fmt"
	"time"
)

// DisposalTask 处置任务实体：异常处置全生命周期（created→assigned→repaired→verified→closed）。
type DisposalTask struct {
	ID            string     `json:"id"`
	AbnormalityID string     `json:"abnormality_id"`
	BeaconID      string     `json:"beacon_id"`
	Title         string     `json:"title"`
	Level         TaskLevel  `json:"level"`
	Status        TaskStatus `json:"status"`
	Assignee      string     `json:"assignee,omitempty"`
	Deadline      time.Time  `json:"deadline"`
	Escalated     bool       `json:"escalated"`
	CreatedAt     time.Time  `json:"created_at"`
	AssignedAt    *time.Time `json:"assigned_at,omitempty"`
	RepairedAt    *time.Time `json:"repaired_at,omitempty"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	RepairNote    string     `json:"repair_note,omitempty"`
	VerifyResult  string     `json:"verify_result,omitempty"`
	EscalatedAt   *time.Time `json:"escalated_at,omitempty"`
}

// NewDisposalTask 构造处置任务，默认级别为普通。
func NewDisposalTask(id, abnormalityID, beaconID, title string, level TaskLevel, deadline time.Time, now time.Time) *DisposalTask {
	return &DisposalTask{
		ID:            id,
		AbnormalityID: abnormalityID,
		BeaconID:      beaconID,
		Title:         title,
		Level:         level,
		Status:        TaskStatusCreated,
		Deadline:      deadline,
		CreatedAt:     now,
	}
}

// Validate 校验任务字段。
func (t *DisposalTask) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task id 不能为空")
	}
	if t.AbnormalityID == "" {
		return fmt.Errorf("abnormality_id 不能为空")
	}
	if t.BeaconID == "" {
		return fmt.Errorf("beacon_id 不能为空")
	}
	if !t.Status.Valid() {
		return fmt.Errorf("无效的任务状态 %q", t.Status)
	}
	if !t.Level.Valid() {
		return fmt.Errorf("无效的任务级别 %q", t.Level)
	}
	return nil
}

// CanTransitionTo 判断是否允许迁移到目标状态。
func (t *DisposalTask) CanTransitionTo(next TaskStatus) bool {
	for _, s := range TaskTransitions[t.Status] {
		if s == next {
			return true
		}
	}
	return false
}

// Assign 派发任务：created → assigned。
func (t *DisposalTask) Assign(assignee string, now time.Time) error {
	if !t.CanTransitionTo(TaskStatusAssigned) {
		return fmt.Errorf("任务状态 %s 不允许派发（仅 created 可派发）", t.Status)
	}
	if assignee == "" {
		return fmt.Errorf("派发人不能为空")
	}
	t.Status = TaskStatusAssigned
	t.Assignee = assignee
	t.AssignedAt = &now
	return nil
}

// Repair 修复任务：assigned → repaired。
func (t *DisposalTask) Repair(note string, now time.Time) error {
	if !t.CanTransitionTo(TaskStatusRepaired) {
		return fmt.Errorf("任务状态 %s 不允许修复（仅 assigned 可修复）", t.Status)
	}
	t.Status = TaskStatusRepaired
	t.RepairNote = note
	t.RepairedAt = &now
	return nil
}

// Verify 复测任务：repaired → verified。
func (t *DisposalTask) Verify(result string, now time.Time) error {
	if !t.CanTransitionTo(TaskStatusVerified) {
		return fmt.Errorf("任务状态 %s 不允许复测（仅 repaired 可复测）", t.Status)
	}
	t.Status = TaskStatusVerified
	t.VerifyResult = result
	t.VerifiedAt = &now
	return nil
}

// Close 关闭任务：verified → closed。
func (t *DisposalTask) Close(now time.Time) error {
	if !t.CanTransitionTo(TaskStatusClosed) {
		return fmt.Errorf("任务状态 %s 不允许关闭（仅 verified 可关闭）", t.Status)
	}
	t.Status = TaskStatusClosed
	t.ClosedAt = &now
	return nil
}

// Escalate 升级任务级别（灭灯派发超时升级）：紧急并标记升级。
func (t *DisposalTask) Escalate(now time.Time) error {
	if t.Status == TaskStatusClosed {
		return fmt.Errorf("已关闭任务无需升级")
	}
	if t.Escalated {
		return nil
	}
	t.Level = TaskLevelUrgent
	t.Escalated = true
	t.EscalatedAt = &now
	return nil
}

// IsOverdue 判断任务是否超过派发期限（仅对未派发/未关闭任务有意义）。
func (t *DisposalTask) IsOverdue(now time.Time) bool {
	if t.Status == TaskStatusClosed || t.Status == TaskStatusVerified {
		return false
	}
	return now.After(t.Deadline)
}

// Open 判断任务是否处于进行中（未关闭）。
func (t *DisposalTask) Open() bool {
	return t.Status != TaskStatusClosed
}

// StatusFlow 返回状态机迁移链，供文档与前端提示。
func StatusFlow() []TaskStatus {
	return []TaskStatus{TaskStatusCreated, TaskStatusAssigned, TaskStatusRepaired, TaskStatusVerified, TaskStatusClosed}
}
