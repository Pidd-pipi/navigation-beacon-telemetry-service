package store

import (
	"sort"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// UpsertTask 新增或覆盖处置任务并落盘。
func (s *Store) UpsertTask(t *domain.DisposalTask) error {
	return s.Mutate(func() {
		s.tasks[t.ID] = t
	})
}

// UpdateTaskInPlace 在写锁内读取实时任务记录，交给 fn 做状态迁移后落盘。
// 用于 Assign/Repair/Verify/Close/Escalate 等「读取-校验-迁移-写回」原子操作：
// 并发派发/复测/关闭在同一把写锁内串行，第二个请求进入时能看到前一个的迁移结果，
// 从而被状态机拒绝，避免同一任务被并发执行两遍。
// 返回迁移后记录的拷贝，未找到时返回 (nil, ErrNotFound)。
func (s *Store) UpdateTaskInPlace(id string, fn func(*domain.DisposalTask) error) (*domain.DisposalTask, error) {
	var out *domain.DisposalTask
	err := s.MutateFn(func() error {
		cur := s.tasks[id]
		if cur == nil {
			return ErrNotFound
		}
		if err := fn(cur); err != nil {
			return err
		}
		out = cur.Clone()
		return nil
	})
	return out, err
}

// CreateTaskIfAbsent 在写锁内检查某异常是否已有「进行中」任务，
// 没有则插入新任务并落盘，已有则返回 (existing, false, nil)。
// 用于 CreateManual/CreateForAbnormality 的「查重 + 创建」原子化，
// 避免并发创建出两条任务（同一件事被并发做两遍）。
func (s *Store) CreateTaskIfAbsent(t *domain.DisposalTask) (created *domain.DisposalTask, inserted bool, err error) {
	err = s.MutateFn(func() error {
		for _, existing := range s.tasks {
			if existing.AbnormalityID == t.AbnormalityID && existing.Open() {
				created = existing.Clone()
				return nil
			}
		}
		s.tasks[t.ID] = t
		created = t.Clone()
		inserted = true
		return nil
	})
	return created, inserted, err
}

// GetTask 按 ID 查询任务。
func (s *Store) GetTask(id string) *domain.DisposalTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := s.tasks[id]
	if t == nil {
		return nil
	}
	return t.Clone()
}

// ListTasks 返回任务列表（按创建时间倒序）。
func (s *Store) ListTasks() []*domain.DisposalTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.DisposalTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// FindTaskByAbnormality 查询某异常关联的任务（取最近一条）。
func (s *Store) FindTaskByAbnormality(abnormalityID string) *domain.DisposalTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found *domain.DisposalTask
	for _, t := range s.tasks {
		if t.AbnormalityID == abnormalityID {
			if found == nil || t.CreatedAt.After(found.CreatedAt) {
				found = t
			}
		}
	}
	if found == nil {
		return nil
	}
	return found.Clone()
}

// CountOpenTasks 统计未关闭任务数。
func (s *Store) CountOpenTasks() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.tasks {
		if t.Open() {
			n++
		}
	}
	return n
}

// CountTasksByStatus 按状态统计任务数。
func (s *Store) CountTasksByStatus() map[domain.TaskStatus]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[domain.TaskStatus]int)
	for _, t := range s.tasks {
		out[t.Status]++
	}
	return out
}

// ListEscalationCandidates 返回需要扫描的未关闭任务（按创建时间升序）。
func (s *Store) ListEscalationCandidates() []*domain.DisposalTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.DisposalTask, 0)
	for _, t := range s.tasks {
		if t.Open() {
			out = append(out, t.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
