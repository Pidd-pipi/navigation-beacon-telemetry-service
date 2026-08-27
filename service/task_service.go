package service

import (
	"errors"
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// errTaskAlreadyTransitioned 表示任务在并发窗口内已被其它请求迁移过，
// 本次扫描/重试应跳过。内部哨兵，不外泄为业务错误。
var errTaskAlreadyTransitioned = errors.New("task already transitioned, skip")

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
//
// 「查重 + 创建」在仓储写锁内原子完成（CreateTaskIfAbsent），避免并发触发
// （如两次遥测上报同时打开灭灯异常）为同一异常生成两条任务。
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
	created, inserted, err := s.store.CreateTaskIfAbsent(task)
	if err != nil {
		return nil, domain.Internal("保存任务失败: %v", err)
	}
	if !inserted {
		return nil, domain.Conflict("异常 %s 已有进行中的任务 %s", ab.ID, created.ID)
	}
	s.audit.LogAt(now, "task.created", "task", created.ID, "system",
		fmt.Sprintf("异常 %s 自动生成任务 %s，期限 %s", ab.ID, created.ID, deadline.Format(time.RFC3339)))
	return created, nil
}

// CreateManual 手工为异常生成处置任务（POST /api/tasks）。
//
// 与 CreateForAbnormality 一致走 CreateTaskIfAbsent 原子查重；此处前置校验
// （异常存在/未解决）仅用于给出更精确的 4xx，最终并发安全由仓储层保证。
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
//
// 「读取当前状态 → 校验可否派发 → 置位 assigned → 写回」在仓储写锁内原子完成：
// 同一人快速点两下派发时，两请求在锁内串行，后者读到前者已置位的 assigned，
// 被 CanTransitionTo 拒绝，从而保证同一任务只被派发一次。
func (s *TaskService) Assign(id, assignee string, now time.Time) (*domain.DisposalTask, error) {
	updated, err := s.store.UpdateTaskInPlace(id, func(t *domain.DisposalTask) error {
		return t.Assign(assignee, now)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("任务 %s 不存在", id)
		}
		return nil, domain.Conflict("派发失败: %v", err)
	}
	s.audit.LogAt(now, "task.assign", "task", id, assignee, fmt.Sprintf("任务派发给 %s", assignee))
	return updated, nil
}

// Repair 修复任务：assigned → repaired。
func (s *TaskService) Repair(id, note, operator string, now time.Time) (*domain.DisposalTask, error) {
	updated, err := s.store.UpdateTaskInPlace(id, func(t *domain.DisposalTask) error {
		return t.Repair(note, now)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("任务 %s 不存在", id)
		}
		return nil, domain.Conflict("修复失败: %v", err)
	}
	s.audit.LogAt(now, "task.repair", "task", id, operator, note)
	return updated, nil
}

// Verify 复测任务：repaired → verified；autoClose 为 true 时随后关闭（复测并关闭）。
func (s *TaskService) Verify(id, result string, autoClose bool, operator string, now time.Time) (*domain.DisposalTask, error) {
	updated, err := s.store.UpdateTaskInPlace(id, func(t *domain.DisposalTask) error {
		if err := t.Verify(result, now); err != nil {
			return err
		}
		if autoClose {
			if err := t.Close(now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("任务 %s 不存在", id)
		}
		return nil, domain.Conflict("复测失败: %v", err)
	}
	s.audit.LogAt(now, "task.verify", "task", id, operator, fmt.Sprintf("复测结果: %s；自动关闭=%v", result, autoClose))
	if autoClose {
		s.audit.LogAt(now, "task.close", "task", id, operator, "复测通过，任务关闭")
	}
	return updated, nil
}

// Close 关闭任务：verified → closed。
func (s *TaskService) Close(id, operator string, now time.Time) (*domain.DisposalTask, error) {
	updated, err := s.store.UpdateTaskInPlace(id, func(t *domain.DisposalTask) error {
		return t.Close(now)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("任务 %s 不存在", id)
		}
		return nil, domain.Conflict("关闭失败: %v", err)
	}
	s.audit.LogAt(now, "task.close", "task", id, operator, "任务关闭")
	return updated, nil
}

// Escalate 手动升级任务。
func (s *TaskService) Escalate(id, operator string, now time.Time) (*domain.DisposalTask, error) {
	updated, err := s.store.UpdateTaskInPlace(id, func(t *domain.DisposalTask) error {
		return t.Escalate(now)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("任务 %s 不存在", id)
		}
		return nil, domain.Conflict("升级失败: %v", err)
	}
	s.audit.LogAt(now, "task.escalate", "task", id, operator, "任务升级为紧急")
	return updated, nil
}

// EscalateOverdue 扫描并升级所有超过派发期限仍未关闭的任务，返回升级数量。
//
// 每条候选任务的升级走 UpdateTaskInPlace 原子完成，与并发的 Assign/Close/Escalate
// 互斥：在锁内重新读取实时状态判定 IsOverdue/Escalated，避免重复升级或对已流转
// 任务误操作。
func (s *TaskService) EscalateOverdue(now time.Time) (int, error) {
	candidates := s.store.ListEscalationCandidates()
	count := 0
	for _, t := range candidates {
		if !t.IsOverdue(now) || t.Escalated {
			continue
		}
		updated, err := s.store.UpdateTaskInPlace(t.ID, func(cur *domain.DisposalTask) error {
			if cur.Status == domain.TaskStatusClosed {
				return errTaskAlreadyTransitioned
			}
			if cur.Escalated {
				return errTaskAlreadyTransitioned
			}
			return cur.Escalate(now)
		})
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, errTaskAlreadyTransitioned) {
				continue
			}
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
