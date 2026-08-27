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
