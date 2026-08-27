package service

import (
	"errors"
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// errCommandAlreadySettled 表示指令在并发窗口内已被 Ack/标记失败，
// 本次重发/失败标记应跳过。内部哨兵，不外泄为业务错误。
var errCommandAlreadySettled = errors.New("command already settled, skip")

// CommandService 遥控指令服务：下发、回执、超时重发。
type CommandService struct {
	store *store.Store
	audit *AuditService
	cfg   *config.Config
}

// NewCommandService 构造遥控指令服务。
func NewCommandService(st *store.Store, audit *AuditService, cfg *config.Config) *CommandService {
	return &CommandService{store: st, audit: audit, cfg: cfg}
}

// CommandFilter 指令查询过滤条件。
type CommandFilter struct {
	BeaconID string           // 空串表示不限
	Status   domain.AckStatus // 空串表示不限
	Type     domain.CommandType
}

// Dispatch 下发遥控指令。
// 漂移期间禁止下发关灯指令（服务层防御，与 driftGuard 中间件双重保障）。
func (s *CommandService) Dispatch(beaconID string, ct domain.CommandType, target *domain.LampPattern, operator string, now time.Time) (*domain.RemoteCommand, error) {
	beacon := s.store.GetBeacon(beaconID)
	if beacon == nil {
		return nil, domain.NotFound("航标 %s 不存在", beaconID)
	}
	if !ct.Valid() {
		return nil, domain.Validation("无效的指令类型 %q", ct)
	}
	if ct == domain.CommandTypeOff && beacon.Drifting {
		return nil, domain.DriftGuard("航标 %s 处于漂移状态，为保障航行安全禁止下发关灯指令", beaconID)
	}
	if ct == domain.CommandTypeSwitchPattern && target == nil {
		return nil, domain.Validation("切换灯质指令必须携带 target_pattern")
	}

	cmd := domain.NewRemoteCommand(s.store.NextID("C"), beaconID, ct, target, operator, now, now.Add(s.cfg.CommandAckTimeout))
	if err := cmd.Validate(); err != nil {
		return nil, domain.Validation("指令参数非法: %v", err)
	}
	if err := s.store.UpsertCommand(cmd); err != nil {
		return nil, domain.Internal("保存指令失败: %v", err)
	}
	detail := fmt.Sprintf("向航标 %s 下发【%s】指令，回执期限 %s",
		beaconID, ct.Label(), cmd.Deadline.Format(time.RFC3339))
	if ct == domain.CommandTypeSwitchPattern && target != nil {
		detail += fmt.Sprintf("，目标灯质 %s", target.String())
	}
	s.audit.LogAt(now, "command.dispatch", "command", cmd.ID, operator, detail)
	return cmd, nil
}

// Ack 处理终端回执。
//
// 整个「读取当前状态 → 校验是否可回执（Pending）→ 置位成功/失败 → 写回」
// 在仓储写锁内原子完成：并发到达的两份回执会在锁内串行，后者进入时能读到前者
// 已经置位的状态，从而被 Ack 的「不可重复回执」拒绝。任一方都不应越过状态机。
func (s *CommandService) Ack(commandID string, success bool, message, operator string, now time.Time) (*domain.RemoteCommand, error) {
	status := domain.AckStatusFailed
	if success {
		status = domain.AckStatusSuccess
	}
	updated, err := s.store.UpdateCommandInPlace(commandID, func(cmd *domain.RemoteCommand) error {
		return cmd.Ack(status, message, now)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, domain.NotFound("指令 %s 不存在", commandID)
		}
		return nil, domain.Conflict("回执处理失败: %v", err)
	}
	s.audit.LogAt(now, "command.ack", "command", commandID, operator,
		fmt.Sprintf("指令 %s 回执: %s（%s）", commandID, status.Label(), message))
	return updated, nil
}

// ResendDue 扫描待回执指令：超时自动重发（最多 CommandMaxRetries 次），
// 重试耗尽仍无回执则标记失败并告警。返回重发数与失败数。
//
// 每条指令的重发/标记失败走 UpdateCommandInPlace 原子完成，与并发的 Ack 互斥：
// 若 Ack 已先行置位，Resend 在锁内读取到的实时状态非 pending 即跳过，
// 不会对一个已被回执的指令重复重发。
func (s *CommandService) ResendDue(now time.Time) (resent, failed int) {
	for _, cmd := range s.store.ListPendingCommands() {
		if !cmd.IsOverdue(now) {
			continue
		}
		if cmd.RetryCount < s.cfg.CommandMaxRetries {
			updated, err := s.store.UpdateCommandInPlace(cmd.ID, func(c *domain.RemoteCommand) error {
				if !c.Pending() {
					return errCommandAlreadySettled
				}
				c.Resend(now, s.cfg.CommandAckTimeout)
				return nil
			})
			if err != nil {
				continue
			}
			s.audit.LogAt(now, "command.resend", "command", updated.ID, "sweeper",
				fmt.Sprintf("指令 %s 回执超时，第 %d 次自动重发", updated.ID, updated.RetryCount))
			resent++
			continue
		}
		updated, err := s.store.UpdateCommandInPlace(cmd.ID, func(c *domain.RemoteCommand) error {
			if !c.Pending() {
				return errCommandAlreadySettled
			}
			c.MarkFailed("回执超时且重试耗尽", now)
			return nil
		})
		if err != nil {
			continue
		}
		s.audit.LogAt(now, "command.failed_alert", "command", updated.ID, "sweeper",
			fmt.Sprintf("指令 %s 回执超时重试耗尽，标记失败并告警", updated.ID))
		failed++
	}
	return resent, failed
}

// List 按过滤条件查询指令（按下发时间倒序）。
func (s *CommandService) List(filter CommandFilter) []*domain.RemoteCommand {
	all := s.store.ListCommands()
	out := make([]*domain.RemoteCommand, 0, len(all))
	for _, c := range all {
		if filter.BeaconID != "" && c.BeaconID != filter.BeaconID {
			continue
		}
		if filter.Status != "" && c.Status != filter.Status {
			continue
		}
		if filter.Type != "" && c.Type != filter.Type {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Get 查询单个指令。
func (s *CommandService) Get(id string) (*domain.RemoteCommand, error) {
	c := s.store.GetCommand(id)
	if c == nil {
		return nil, domain.NotFound("指令 %s 不存在", id)
	}
	return c, nil
}
