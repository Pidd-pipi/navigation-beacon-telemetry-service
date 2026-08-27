package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// TestProcessRespectsRetention 遥测上报链路遵守仓储保留上限，内存有界。
func TestProcessRespectsRetention(t *testing.T) {
	st, _, tel, _, _, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")
	store.SetDefaultRetention(5)

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}
	pattern := domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	for i := 0; i < 8; i++ {
		rt := now.Add(time.Duration(i) * time.Minute)
		_, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.0, anchor, &pattern, &rt), "op", now)
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
	}
	if got := st.CountTelemetryByBeacon("B-001"); got != 5 {
		t.Fatalf("上报超过上限后应保留 5 条，got %d", got)
	}
	items := st.ListTelemetry("B-001", 0)
	if len(items) != 5 || !items[0].ReportedAt.Equal(now.Add(7*time.Minute)) {
		t.Fatalf("应保留最新 5 条且最新在前，首条=%v len=%d", items[0].ReportedAt, len(items))
	}
}
