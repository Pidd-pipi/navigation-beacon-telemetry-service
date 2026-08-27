package store

import (
	"sort"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// UpsertCommand 新增或覆盖遥控指令并落盘。
func (s *Store) UpsertCommand(c *domain.RemoteCommand) error {
	return s.Mutate(func() {
		s.commands[c.ID] = c
	})
}

// UpdateCommandInPlace 在写锁内读取实时指令记录，交给 fn 做状态迁移后落盘。
// 用于 Ack/Resend/MarkFailed 等「读取-校验-迁移-写回」原子操作：
// fn 拿到的是锁内实时指针（非拷贝），并发回执/重发在同一把写锁内串行，
// 第二个请求进入时能看到前一个的状态变更，从而被状态机（Pending/状态迁移）拒绝。
// 返回迁移后记录的拷贝，未找到时返回 (nil, ErrNotFound)。
func (s *Store) UpdateCommandInPlace(id string, fn func(*domain.RemoteCommand) error) (*domain.RemoteCommand, error) {
	var out *domain.RemoteCommand
	err := s.MutateFn(func() error {
		cur := s.commands[id]
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

// GetCommand 按 ID 查询指令。
func (s *Store) GetCommand(id string) *domain.RemoteCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.commands[id]
	if c == nil {
		return nil
	}
	return c.Clone()
}

// ListCommands 返回指令列表（按下发时间倒序）。
func (s *Store) ListCommands() []*domain.RemoteCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.RemoteCommand, 0, len(s.commands))
	for _, c := range s.commands {
		out = append(out, c.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SentAt.After(out[j].SentAt) })
	return out
}

// ListPendingCommands 返回所有待回执指令。
func (s *Store) ListPendingCommands() []*domain.RemoteCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.RemoteCommand, 0)
	for _, c := range s.commands {
		if c.Pending() {
			out = append(out, c.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Deadline.Before(out[j].Deadline) })
	return out
}

// CountPendingCommands 统计待回执指令数。
func (s *Store) CountPendingCommands() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, c := range s.commands {
		if c.Pending() {
			n++
		}
	}
	return n
}

// CountFailedCommands 统计失败指令数。
func (s *Store) CountFailedCommands() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, c := range s.commands {
		if c.Status == domain.AckStatusFailed {
			n++
		}
	}
	return n
}

// CountCommandsSince 统计指定时间之后下发的指令数。
func (s *Store) CountCommandsSince(cutoff time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, c := range s.commands {
		if !c.SentAt.Before(cutoff) {
			n++
		}
	}
	return n
}
