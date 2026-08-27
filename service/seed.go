package service

import (
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// SeedIfEmpty 在仓储为空时写入演示基线数据（3 座航标 + 近 90 分钟遥测历史），
// 便于前端首屏渲染与 API 演示；已有数据时跳过。
func SeedIfEmpty(st *store.Store, cfg *config.Config, telemetrySvc *TelemetryService) error {
	if st.CountBeacons() > 0 {
		return nil
	}
	now := time.Now()

	seeds := []struct {
		id      string
		name    string
		bt      domain.BeaconType
		anchor  domain.Position
		radiusM float64
		pattern domain.LampPattern
	}{
		{
			id: "B-001", name: "北港灯塔", bt: domain.BeaconTypeLighthouse,
			anchor:  domain.Position{Lat: 30.500000, Lng: 122.100000},
			radiusM: 50, pattern: domain.LampPattern{FlashSec: 2, EclipseSec: 2},
		},
		{
			id: "B-002", name: "东航道1号浮标", bt: domain.BeaconTypeBuoy,
			anchor:  domain.Position{Lat: 30.400000, Lng: 122.200000},
			radiusM: 30, pattern: domain.LampPattern{FlashSec: 1, EclipseSec: 3},
		},
		{
			id: "B-003", name: "南湾导标", bt: domain.BeaconTypeDaybeacon,
			anchor:  domain.Position{Lat: 30.300000, Lng: 122.050000},
			radiusM: 40, pattern: domain.LampPattern{FlashSec: 3, EclipseSec: 1},
		},
	}

	for _, sd := range seeds {
		beacon := domain.NewBeacon(sd.id, sd.name, sd.bt, sd.anchor, sd.radiusM, sd.pattern, now)
		if err := st.CreateBeacon(beacon); err != nil {
			return fmt.Errorf("seed beacon %s: %w", sd.id, err)
		}
		// 演示数据使用显式 ID，需推进序号避免后续新建航标 ID 冲突
		st.BumpSeq("B", uint64(len(seeds)))
		// 近 90 分钟遥测历史（每 15 分钟一条，全部正常）
		for i := 5; i >= 0; i-- {
			reportedAt := now.Add(-time.Duration(i) * cfg.TelemetryPeriod)
			jitter := 0.02 * float64(i%3)
			pattern := domain.LampPattern{FlashSec: sd.pattern.FlashSec, EclipseSec: sd.pattern.EclipseSec}
			input := TelemetryInput{
				LampState:       domain.LampStateOn,
				Voltage:         12.3 - jitter,
				Current:         0.8,
				Position:        sd.anchor,
				MeasuredPattern: &pattern,
				ReportedAt:      &reportedAt,
			}
			if _, err := telemetrySvc.Process(sd.id, input, "seed", reportedAt); err != nil {
				return fmt.Errorf("seed telemetry %s: %w", sd.id, err)
			}
		}
	}
	return nil
}
