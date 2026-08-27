package service

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
)

// Sweeper 定时任务：
//  1. 指令回执超时重发/失败告警（每个扫描周期执行）；
//  2. 灭灯任务派发超时升级（每 TaskEscalationScan 周期执行一次，默认 10 分钟）。
type Sweeper struct {
	commandSvc *CommandService
	taskSvc    *TaskService
	cfg        *config.Config
	logger     *slog.Logger
	ctx        context.Context

	mu             sync.Mutex
	lastEscalation time.Time
}

// NewSweeper 构造定时扫描器（兼容旧 *log.Logger 调用方；内部统一走 slog）。
func NewSweeper(commandSvc *CommandService, taskSvc *TaskService, cfg *config.Config, logger *log.Logger) *Sweeper {
	sl := slog.New(slog.NewTextHandler(logger.Writer(), &slog.HandlerOptions{Level: slog.LevelInfo}))
	return NewSweeperSlog(commandSvc, taskSvc, cfg, sl)
}

// NewSweeperSlog 构造定时扫描器（结构化日志）。
func NewSweeperSlog(commandSvc *CommandService, taskSvc *TaskService, cfg *config.Config, logger *slog.Logger) *Sweeper {
	return &Sweeper{
		commandSvc: commandSvc,
		taskSvc:    taskSvc,
		cfg:        cfg,
		logger:     logger,
	}
}

// Start 启动后台扫描循环，直到 ctx 取消；wg 用于优雅退出。
//
// ctx 取消即视为服务进入关停：循环停止，且 RunOnce 在关停后成为 no-op，
// 不再重发超时指令、不再升级超期任务，扫描周期随之冻结。
func (s *Sweeper) Start(ctx context.Context, wg *sync.WaitGroup) {
	s.mu.Lock()
	s.ctx = ctx
	s.lastEscalation = time.Now()
	s.mu.Unlock()
	ticker := time.NewTicker(s.cfg.SweepInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("sweeper stopped")
				return
			case now := <-ticker.C:
				s.RunOnce(now)
			}
		}
	}()
	s.logger.Info("sweeper started", "interval", s.cfg.SweepInterval, "escalation_scan", s.cfg.TaskEscalationScan)
}

// shuttingDown 报告是否已进入关停。未注册 ctx（如测试直接构造）视为未关停。
func (s *Sweeper) shuttingDown() bool {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// RunOnce 执行一次扫描，返回（重发数, 失败数, 升级数）。供测试直接调用。
//
// 关停中（ctx 已取消）直接返回零值，确保关停后不再重发指令、不再升级任务、
// 扫描时间也不再推进（lastEscalation 不更新）。
func (s *Sweeper) RunOnce(now time.Time) (resent, failed, escalated int) {
	if s.shuttingDown() {
		return 0, 0, 0
	}
	resent, failed = s.commandSvc.ResendDue(now)

	s.mu.Lock()
	due := s.lastEscalation.IsZero() || now.Sub(s.lastEscalation) >= s.cfg.TaskEscalationScan
	if due {
		s.lastEscalation = now
	}
	s.mu.Unlock()
	if due {
		n, err := s.taskSvc.EscalateOverdue(now)
		if err != nil {
			s.logger.Error("sweeper escalate error", "error", err)
		}
		escalated = n
	}
	if resent > 0 || failed > 0 || escalated > 0 {
		s.logger.Info("sweeper scan", "resent", resent, "failed", failed, "escalated", escalated)
	}
	return resent, failed, escalated
}
