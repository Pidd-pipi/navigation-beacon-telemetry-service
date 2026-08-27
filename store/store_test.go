package store

import (
	"path/filepath"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

func TestNextIDSequence(t *testing.T) {
	s := New("")
	if id := s.NextID("B"); id != "B-001" {
		t.Errorf("首个 ID 应为 B-001，got %s", id)
	}
	if id := s.NextID("B"); id != "B-002" {
		t.Errorf("第二个 ID 应为 B-002，got %s", id)
	}
	// 不同前缀独立计数
	if id := s.NextID("T"); id != "T-001" {
		t.Errorf("T 前缀首个 ID 应为 T-001，got %s", id)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := New(path)

	now := time.Now()
	beacon := domain.NewBeacon("B-001", "北港灯塔", domain.BeaconTypeLighthouse,
		domain.Position{Lat: 30.5, Lng: 122.1}, 50, domain.LampPattern{FlashSec: 2, EclipseSec: 2}, now)
	if err := s.CreateBeacon(beacon); err != nil {
		t.Fatalf("CreateBeacon: %v", err)
	}
	tel := domain.NewTelemetryData(s.NextID("T"), "B-001", domain.LampStateOn, 12.3, 0.8,
		domain.Position{Lat: 30.5, Lng: 122.1}, &domain.LampPattern{FlashSec: 2, EclipseSec: 2}, now, now)
	if err := s.AppendTelemetry(tel); err != nil {
		t.Fatalf("AppendTelemetry: %v", err)
	}

	// 重新加载
	s2 := New(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s2.GetBeacon("B-001"); got == nil || got.Name != "北港灯塔" {
		t.Fatalf("加载后航标缺失或不一致: %+v", got)
	}
	if got := s2.CountTelemetryByBeacon("B-001"); got != 1 {
		t.Fatalf("加载后遥测条数应为 1，got %d", got)
	}
	if got := s2.LastTelemetry("B-001"); got == nil || got.Voltage != 12.3 {
		t.Fatalf("加载后最近遥测异常: %+v", got)
	}
}

func TestPersistenceNoFile(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "missing.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("文件不存在时 Load 应返回 nil: %v", err)
	}
}

func TestEmptyPathNoSave(t *testing.T) {
	s := New("")
	if err := s.Save(); err != nil {
		t.Fatalf("空路径 Save 应为 no-op: %v", err)
	}
}

func TestListTelemetryOrder(t *testing.T) {
	s := New("")
	base := time.Now()
	for i := 0; i < 5; i++ {
		rt := base.Add(-time.Duration(i) * time.Hour)
		tel := domain.NewTelemetryData(s.NextID("T"), "B-001", domain.LampStateOn,
			float64(12+i), 0.8, domain.Position{Lat: 30.5, Lng: 122.1},
			&domain.LampPattern{FlashSec: 2, EclipseSec: 2}, rt, base)
		if err := s.AppendTelemetry(tel); err != nil {
			t.Fatal(err)
		}
	}
	items := s.ListTelemetry("B-001", 0)
	if len(items) != 5 {
		t.Fatalf("条数应为 5，got %d", len(items))
	}
	// 最新在前：第一个电压应为 16（i=0 时 rt=base 最新，电压 12+0=12？需核对）
	// i=0 → rt=base（最新）→ 电压 12；i=4 → rt=base-4h（最旧）→ 电压 16
	if items[0].Voltage != 12 {
		t.Errorf("最新遥测电压应为 12，got %v", items[0].Voltage)
	}
	if items[4].Voltage != 16 {
		t.Errorf("最旧遥测电压应为 16，got %v", items[4].Voltage)
	}
	limited := s.ListTelemetry("B-001", 2)
	if len(limited) != 2 {
		t.Errorf("limit=2 应返回 2 条，got %d", len(limited))
	}
}
