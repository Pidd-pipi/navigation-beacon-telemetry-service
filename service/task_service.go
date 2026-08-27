package service

import (
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// TaskService 处置任务服务：状态机流转、灭灯派发期限、超时升级。
type TaskService struct {
	store *store.Store
	audit *AuditService
	cfg   *config.Config
}

// NewTaskService 构造处置任务服务。
func NewTaskService(st *store.Store, audit *AuditService, cfg *config.Config) *TaskService {
	return &TaskService{store: st, audit: audit, cfg: cfg}
}

// TaskFilter 任务查询过滤条件。
type TaskFilter struct {
	Status   domain.TaskStatus // 空串表示不限
	BeaconID string            // 空串表示不限
	Level    domain.TaskLevel  // 空串表示不限
}

// CreateForAbnormality 为异常自动生成处置任务。
// 灭灯异常任务派发期限为 4 小时，其余异常默认 24 小时。
func (s *TaskService) CreateForAbnormality(ab *domain.LampAbnormality, beacon *domain.Beacon, now time.Time) (*domain.DisposalTask, error) {
	deadline := now.Add(s.cfg.TaskAssignDeadline)
	if ab.Type != domain.AbnormalityTypeLampOut {
		deadline = now.Add(24 * time.Hour)
	}
	title := fmt.Sprintf("【%s】%s 处置任务", ab.Type.Label(), beacon.Name)
	task := domain.NewDisposalTask(
		s.store.NextID("TA"), ab.ID, ab.BeaconID, title, domain.TaskLevelNormal, deadline, now,
	)
	if err := task.Validate(); err != nil {
		return nil, domain.Validation("任务参数非法: %v", err)
	}
	if err := s.store.UpsertTask(task); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.created", "task", task.ID, "system",
		fmt.Sprintf("异常 %s 自动生成任务 %s，期限 %s", ab.ID, task.ID, deadline.Format(time.RFC3339)))
	return task, nil
}

// CreateManual 手工为异常生成处置任务（POST /api/tasks）。
func (s *TaskService) CreateManual(abnormalityID string, now time.Time) (*domain.DisposalTask, error) {
	ab := s.store.GetAbnormality(abnormalityID)
	if ab == nil {
		return nil, domain.NotFound("异常 %s 不存在", abnormalityID)
	}
	if !ab.IsOpen() {
		return nil, domain.Conflict("异常 %s 已解决，无需生成任务", abnormalityID)
	}
	if existing := s.store.FindTaskByAbnormality(abnormalityID); existing != nil && existing.Open() {
		return nil, domain.Conflict("异常 %s 已有进行中的任务 %s", abnormalityID, existing.ID)
	}
	beacon := s.store.GetBeacon(ab.BeaconID)
	if beacon == nil {
		return nil, domain.NotFound("航标 %s 不存在", ab.BeaconID)
	}
	return s.CreateForAbnormality(ab, beacon, now)
}

// Assign 派发任务：created → assigned。
func (s *TaskService) Assign(id, assignee string, now time.Time) (*domain.DisposalTask, error) {
	task := s.store.GetTask(id)
	if task == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	updated := cloneTask(task)
	if err := updated.Assign(assignee, now); err != nil {
		return nil, domain.Conflict("派发失败: %v", err)
	}
	if err := s.store.UpsertTask(updated); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.assign", "task", id, assignee, fmt.Sprintf("任务派发给 %s", assignee))
	return updated, nil
}

// Repair 修复任务：assigned → repaired。
func (s *TaskService) Repair(id, note, operator string, now time.Time) (*domain.DisposalTask, error) {
	task := s.store.GetTask(id)
	if task == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	updated := cloneTask(task)
	if err := updated.Repair(note, now); err != nil {
		return nil, domain.Conflict("修复失败: %v", err)
	}
	if err := s.store.UpsertTask(updated); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.repair", "task", id, operator, note)
	return updated, nil
}

// Verify 复测任务：repaired → verified；autoClose 为 true 时随后关闭（复测并关闭）。
func (s *TaskService) Verify(id, result string, autoClose bool, operator string, now time.Time) (*domain.DisposalTask, error) {
	task := s.store.GetTask(id)
	if task == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	updated := cloneTask(task)
	if err := updated.Verify(result, now); err != nil {
		return nil, domain.Conflict("复测失败: %v", err)
	}
	if autoClose {
		if err := updated.Close(now); err != nil {
			return nil, domain.Conflict("复测后关闭失败: %v", err)
		}
	}
	if err := s.store.UpsertTask(updated); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.verify", "task", id, operator, fmt.Sprintf("复测结果: %s；自动关闭=%v", result, autoClose))
	if autoClose {
		s.audit.LogAt(now, "task.close", "task", id, operator, "复测通过，任务关闭")
	}
	return updated, nil
}

// Close 关闭任务：verified → closed。
func (s *TaskService) Close(id, operator string, now time.Time) (*domain.DisposalTask, error) {
	task := s.store.GetTask(id)
	if task == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	updated := cloneTask(task)
	if err := updated.Close(now); err != nil {
		return nil, domain.Conflict("关闭失败: %v", err)
	}
	if err := s.store.UpsertTask(updated); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.close", "task", id, operator, "任务关闭")
	return updated, nil
}

// Escalate 手动升级任务。
func (s *TaskService) Escalate(id, operator string, now time.Time) (*domain.DisposalTask, error) {
	task := s.store.GetTask(id)
	if task == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	updated := cloneTask(task)
	if err := updated.Escalate(now); err != nil {
		return nil, domain.Conflict("升级失败: %v", err)
	}
	if err := s.store.UpsertTask(updated); err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	s.audit.LogAt(now, "task.escalate", "task", id, operator, "任务升级为紧急")
	return updated, nil
}

// EscalateOverdue 扫描并升级所有超过派发期限仍未关闭的任务，返回升级数量。
func (s *TaskService) EscalateOverdue(now time.Time) (int, error) {
	candidates := s.store.ListEscalationCandidates()
	count := 0
	for _, t := range candidates {
		if !t.IsOverdue(now) || t.Escalated {
			continue
		}
		updated := cloneTask(t)
		if err := updated.Escalate(now); err != nil {
			continue
		}
		if err := s.store.UpsertTask(updated); err != nil {
			return count, err
		}
		s.audit.LogAt(now, "task.escalate", "task", updated.ID, "sweeper",
			fmt.Sprintf("任务 %s 超过派发期限 %s，自动升级为紧急", updated.ID, updated.Deadline.Format(time.RFC3339)))
		count++
	}
	return count, nil
}

// List 按过滤条件查询任务（按创建时间倒序）。
func (s *TaskService) List(filter TaskFilter) []*domain.DisposalTask {
	all := s.store.ListTasks()
	out := make([]*domain.DisposalTask, 0, len(all))
	for _, t := range all {
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.BeaconID != "" && t.BeaconID != filter.BeaconID {
			continue
		}
		if filter.Level != "" && t.Level != filter.Level {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Get 查询单个任务。
func (s *TaskService) Get(id string) (*domain.DisposalTask, error) {
	t := s.store.GetTask(id)
	if t == nil {
		return nil, domain.NotFound("任务 %s 不存在", id)
	}
	return t, nil
}

// CountOverdue 统计当前超期未关闭任务数。
func (s *TaskService) CountOverdue(now time.Time) int {
	n := 0
	for _, t := range s.store.ListTasks() {
		if t.IsOverdue(now) {
			n++
		}
	}
	return n
}

func cloneTask(t *domain.DisposalTask) *domain.DisposalTask {
	c := *t
	return &c
}
