package service

import (
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// TelemetryInput 遥测上报输入（来自 POST /api/beacons/{id}/telemetry）。
type TelemetryInput struct {
	LampState       domain.LampState    `json:"lamp_state"`
	Voltage         float64             `json:"voltage"`
	Current         float64             `json:"current"`
	Position        domain.Position     `json:"position"`
	MeasuredPattern *domain.LampPattern `json:"measured_pattern,omitempty"`
	ReportedAt      *time.Time          `json:"reported_at,omitempty"` // 终端上报时间，缺省取服务端当前时间
}

// TelemetryResult 遥测处理结果：原始数据 + 违规项 + 受影响异常 + 频率建议。
type TelemetryResult struct {
	Telemetry             *domain.TelemetryData     `json:"telemetry"`
	Violations            []string                  `json:"violations"`
	AffectedAbnormalities []*domain.LampAbnormality `json:"affected_abnormalities"`
	SuggestedPeriod       string                    `json:"suggested_period,omitempty"`
	DriftDistanceM        float64                   `json:"drift_distance_m"`
}

// TelemetryService 遥测服务：采集、灯质校验、电压健康、漂移检测、诊断联动。
type TelemetryService struct {
	store     *store.Store
	abnormals *AbnormalityService
	audit     *AuditService
	cfg       *config.Config
}

// NewTelemetryService 构造遥测服务。
func NewTelemetryService(st *store.Store, abnormals *AbnormalityService, audit *AuditService, cfg *config.Config) *TelemetryService {
	return &TelemetryService{store: st, abnormals: abnormals, audit: audit, cfg: cfg}
}

// Process 处理一条遥测上报，执行完整诊断链路：
//  1. 灯质校验：设定灯质与实测闪法偏差超容差 → 灯质偏差异常
//  2. 灭灯判定：灯连续熄灭超阈值 → 灭灯故障（自动生成处置任务）
//  3. 电压健康：低于下限 → 低电预警 + 降低遥测频率建议；恢复后解除
//  4. 漂移检测：位置偏离锚位超半径 → 漂移异常；回位后解除
func (s *TelemetryService) Process(beaconID string, input TelemetryInput, operator string, now time.Time) (*TelemetryResult, error) {
	beacon := s.store.GetBeacon(beaconID)
	if beacon == nil {
		return nil, domain.NotFound("航标 %s 不存在", beaconID)
	}

	reportedAt := now
	if input.ReportedAt != nil && !input.ReportedAt.IsZero() {
		reportedAt = *input.ReportedAt
	}

	telemetry := domain.NewTelemetryData(
		s.store.NextID("T"), beaconID, input.LampState, input.Voltage, input.Current,
		input.Position, input.MeasuredPattern, reportedAt, now,
	)
	if err := telemetry.Validate(); err != nil {
		return nil, domain.Validation("遥测数据非法: %v", err)
	}

	result := &TelemetryResult{
		Telemetry:             telemetry,
		Violations:            make([]string, 0),
		AffectedAbnormalities: make([]*domain.LampAbnormality, 0),
	}

	// 工作副本，避免在锁外修改 store 中共享指针
	beaconUpdated := cloneBeacon(beacon)

	// --- 1. 灭灯判定与灯状态追踪 ---
	if input.LampState == domain.LampStateOff {
		if beaconUpdated.LampOffSince == nil {
			beaconUpdated.LampOffSince = &reportedAt
		}
		offDuration := reportedAt.Sub(*beaconUpdated.LampOffSince)
		if offDuration < 0 {
			offDuration = 0
		}
		if offDuration >= time.Duration(s.cfg.LampOutMinutes)*time.Minute {
			detail := fmt.Sprintf("灯连续熄灭 %.0f 分钟（阈值 %d 分钟），判定灭灯故障", offDuration.Minutes(), s.cfg.LampOutMinutes)
			ab, err := s.abnormals.OpenFromTelemetry(beaconUpdated, domain.AbnormalityTypeLampOut, detail, now)
			if err != nil {
				return nil, err
			}
			result.AffectedAbnormalities = append(result.AffectedAbnormalities, ab)
			telemetry.AddViolation(detail)
		}
	} else {
		if beaconUpdated.LampOffSince != nil {
			beaconUpdated.LampOffSince = nil
			s.abnormals.AutoResolve(beaconID, domain.AbnormalityTypeLampOut, "灯已恢复点亮", operator, now)
		}
	}

	// --- 2. 灯质校验 ---
	if input.LampState == domain.LampStateOn && input.MeasuredPattern != nil {
		if input.MeasuredPattern.DeviatesFrom(beaconUpdated.LampPattern, s.cfg.LampToleranceSec) {
			dev := input.MeasuredPattern.MaxDeviationSec(beaconUpdated.LampPattern)
			detail := fmt.Sprintf("实测灯质 %s 与设定 %s 偏差 %.2fs（容差 %.1fs）",
				input.MeasuredPattern.String(), beaconUpdated.LampPattern.String(), dev, s.cfg.LampToleranceSec)
			ab, err := s.abnormals.OpenFromTelemetry(beaconUpdated, domain.AbnormalityTypeLampMismatch, detail, now)
			if err != nil {
				return nil, err
			}
			result.AffectedAbnormalities = append(result.AffectedAbnormalities, ab)
			telemetry.AddViolation(detail)
		} else {
			s.abnormals.AutoResolve(beaconID, domain.AbnormalityTypeLampMismatch, "灯质校验恢复正常", operator, now)
		}
	}

	// --- 3. 电压健康 ---
	switch {
	case input.Voltage < s.cfg.LowVoltageThreshold:
		if !beaconUpdated.LowPower {
			beaconUpdated.LowPower = true
			beaconUpdated.LowVoltSince = &reportedAt
			detail := fmt.Sprintf("电池电压 %.2fV 低于阈值 %.1fV，进入低电预警并降低遥测频率", input.Voltage, s.cfg.LowVoltageThreshold)
			ab, err := s.abnormals.OpenFromTelemetry(beaconUpdated, domain.AbnormalityTypeLowVoltage, detail, now)
			if err != nil {
				return nil, err
			}
			result.AffectedAbnormalities = append(result.AffectedAbnormalities, ab)
			telemetry.AddViolation(detail)
		} else {
			// 持续低电，仅刷新最后发现时间
			s.abnormals.TouchOpen(beaconID, domain.AbnormalityTypeLowVoltage, now)
		}
		result.SuggestedPeriod = "30m" // 降低遥测频率建议
	case input.Voltage >= s.cfg.RecoveryVoltage && beaconUpdated.LowPower:
		beaconUpdated.LowPower = false
		beaconUpdated.LowVoltSince = nil
		s.abnormals.AutoResolve(beaconID, domain.AbnormalityTypeLowVoltage,
			fmt.Sprintf("电压恢复至 %.2fV", input.Voltage), operator, now)
	default:
		// 滞回区间：低于恢复阈值但高于下限，维持现有标记
		if beaconUpdated.LowPower {
			result.SuggestedPeriod = "30m"
		}
	}

	// --- 4. 漂移检测 ---
	dist := input.Position.DistanceTo(beaconUpdated.Anchor)
	result.DriftDistanceM = dist
	if dist > beaconUpdated.DriftRadiusM {
		if !beaconUpdated.Drifting {
			beaconUpdated.Drifting = true
			detail := fmt.Sprintf("实测位置偏离锚位 %.1f 米（阈值 %.1f 米），判定漂移", dist, beaconUpdated.DriftRadiusM)
			ab, err := s.abnormals.OpenFromTelemetry(beaconUpdated, domain.AbnormalityTypeDrift, detail, now)
			if err != nil {
				return nil, err
			}
			result.AffectedAbnormalities = append(result.AffectedAbnormalities, ab)
			telemetry.AddViolation(detail)
		} else {
			s.abnormals.TouchOpen(beaconID, domain.AbnormalityTypeDrift, now)
		}
	} else if beaconUpdated.Drifting {
		beaconUpdated.Drifting = false
		s.abnormals.AutoResolve(beaconID, domain.AbnormalityTypeDrift,
			fmt.Sprintf("位置回归锚位半径内（偏差 %.1f 米）", dist), operator, now)
	}

	// 汇总违规项到结果
	result.Violations = append(result.Violations, telemetry.Violations...)

	// --- 落库与状态刷新 ---
	beaconUpdated.Status = domain.BeaconStatusActive
	beaconUpdated.LastTelemetryAt = &reportedAt
	beaconUpdated.UpdatedAt = now
	if err := s.store.AppendTelemetry(telemetry); err != nil {
		return nil, domain.Internal("保存遥测失败: %v", err)
	}
	if err := s.store.UpdateBeacon(beaconUpdated); err != nil {
		return nil, domain.Internal("更新航标失败: %v", err)
	}

	s.audit.LogAt(now, "telemetry.received", "telemetry", telemetry.ID, operator,
		fmt.Sprintf("航标 %s 遥测上报: 灯=%s 电压=%.2fV 电流=%.2fA 违规=%d",
			beaconID, input.LampState.Label(), input.Voltage, input.Current, len(telemetry.Violations)))

	return result, nil
}

// ListTelemetry 查询某航标遥测趋势（最新在前）。
func (s *TelemetryService) ListTelemetry(beaconID string, limit int) ([]*domain.TelemetryData, error) {
	if s.store.GetBeacon(beaconID) == nil {
		return nil, domain.NotFound("航标 %s 不存在", beaconID)
	}
	return s.store.ListTelemetry(beaconID, limit), nil
}

func cloneBeacon(b *domain.Beacon) *domain.Beacon {
	return b.Clone()
}
