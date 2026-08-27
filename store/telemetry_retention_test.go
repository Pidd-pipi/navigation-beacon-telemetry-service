package store

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// TestAppendRespectsRetention 设置保留上限后持续追加也只保留最新一批。
func TestAppendRespectsRetention(t *testing.T) {
	s := New("")
	SetDefaultRetention(5)
	base := time.Now()
	for i := 0; i < 10; i++ {
		rt := base.Add(time.Duration(i) * time.Minute)
		tel := domain.NewTelemetryData(s.NextID("T"), "B-001", domain.LampStateOn,
			12.0, 0.8, domain.Position{Lat: 30.5, Lng: 122.1},
			&domain.LampPattern{FlashSec: 2, EclipseSec: 2}, rt, base)
		if err := s.AppendTelemetry(tel); err != nil {
			t.Fatal(err)
		}
	}
	items := s.ListTelemetry("B-001", 0)
	if len(items) != 5 || !items[0].ReportedAt.Equal(base.Add(9*time.Minute)) {
		t.Fatalf("追加超过上限后应保留最新 5 条，首条=%v len=%d", items[0].ReportedAt, len(items))
	}
}
