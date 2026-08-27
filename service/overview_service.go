package service

import (
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// OverviewService 总览聚合服务：汇总航标、异常、任务、指令、遥测、审计概览。
type OverviewService struct {
	store *store.Store
	cfg   *config.Config
}

// NewOverviewService 构造总览聚合服务。
func NewOverviewService(st *store.Store, cfg *config.Config) *OverviewService {
	return &OverviewService{store: st, cfg: cfg}
}

// Build 构建总览聚合数据。
func (s *OverviewService) Build(now time.Time) *domain.Overview {
	ov := &domain.Overview{}
	beacons := s.store.ListBeacons()
	openAbns := s.store.ListOpenAbnormalities()
	tasks := s.store.ListTasks()
	commands := s.store.ListCommands()

	// --- 航标 ---
	ov.Beacons.Total = len(beacons)
	ov.Beacons.ByType = make(map[domain.BeaconType]int)
	ov.Beacons.Summaries = make([]domain.BeaconSummary, 0, len(beacons))
	lampOutSet := make(map[string]bool)
	for _, a := range openAbns {
		if a.Type == domain.AbnormalityTypeLampOut {
			lampOutSet[a.BeaconID] = true
		}
	}
	for _, b := range beacons {
		ov.Beacons.ByType[b.Type]++
		status := b.EffectiveStatus(now, s.cfg.OfflineAfter)
		if status == domain.BeaconStatusActive {
			ov.Beacons.Active++
		} else {
			ov.Beacons.Offline++
		}
		if b.LowPower {
			ov.Beacons.LowPower++
		}
		if b.Drifting {
			ov.Beacons.Drifting++
		}
		lampOut := lampOutSet[b.ID]
		if lampOut {
			ov.Beacons.LampOut++
		}
		summary := domain.BeaconSummary{
			ID:              b.ID,
			Name:            b.Name,
			Type:            b.Type,
			Status:          status,
			LowPower:        b.LowPower,
			Drifting:        b.Drifting,
			LampOut:         lampOut,
			LastTelemetryAt: b.LastTelemetryAt,
		}
		if last := s.store.LastTelemetry(b.ID); last != nil {
			summary.Voltage = last.Voltage
			summary.LampState = last.LampState
		}
		ov.Beacons.Summaries = append(ov.Beacons.Summaries, summary)
	}

	// --- 异常 ---
	ov.Abnormalities.Open = len(openAbns)
	ov.Abnormalities.ByType = make(map[domain.AbnormalityType]int)
	for _, a := range openAbns {
		ov.Abnormalities.ByType[a.Type]++
	}

	// --- 任务 ---
	ov.Tasks.ByStatus = make(map[domain.TaskStatus]int)
	for _, t := range tasks {
		ov.Tasks.ByStatus[t.Status]++
		if t.Open() {
			ov.Tasks.Open++
		}
		if t.IsOverdue(now) {
			ov.Tasks.Overdue++
		}
	}

	// --- 指令 ---
	ov.Commands.Total = len(commands)
	for _, c := range commands {
		switch c.Status {
		case domain.AckStatusPending:
			ov.Commands.Pending++
		case domain.AckStatusFailed:
			ov.Commands.Failed++
		}
		if c.SentAt.Before(now.Add(-24 * time.Hour)) {
			ov.Commands.Today++
		}
	}

	// --- 遥测 ---
	ov.Telemetry.Total = s.store.CountTelemetry()
	ov.Telemetry.LastReceivedAt = s.store.LatestTelemetryAt()

	// --- 最近指令与审计 ---
	recentCmd := make([]domain.RemoteCommand, 0, 5)
	for i := len(commands) - 1; i >= 0 && len(recentCmd) < 5; i-- {
		recentCmd = append(recentCmd, *commands[i])
	}
	ov.RecentCommands = recentCmd
	recentAudits := s.store.ListAudits(3)
	ov.RecentAudits = make([]domain.AuditLog, 0, len(recentAudits))
	for i := len(recentAudits) - 1; i >= 0; i-- {
		ov.RecentAudits = append(ov.RecentAudits, *recentAudits[i])
	}

	return ov
}
