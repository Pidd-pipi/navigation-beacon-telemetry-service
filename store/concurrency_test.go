package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// TestReadInterfacesReturnDeepCopy 验证读接口返回深拷贝，调用方原地修改不会污染仓储。
func TestReadInterfacesReturnDeepCopy(t *testing.T) {
	s := New("")
	now := time.Now()
	b := domain.NewBeacon("B-001", "北港灯塔", domain.BeaconTypeLighthouse,
		domain.Position{Lat: 30.5, Lng: 122.1}, 50,
		domain.LampPattern{FlashSec: 2, EclipseSec: 2}, now)
	b.LampOffSince = &now
	if err := s.CreateBeacon(b); err != nil {
		t.Fatal(err)
	}
	pattern := domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	tel := domain.NewTelemetryData("T-001", "B-001", domain.LampStateOn, 12.3, 0.8,
		domain.Position{Lat: 30.5, Lng: 122.1}, &pattern, now, now)
	tel.Violations = []string{"v1"}
	if err := s.AppendTelemetry(tel); err != nil {
		t.Fatal(err)
	}

	first := s.GetBeacon("B-001")
	second := s.GetBeacon("B-001")
	if first.LampOffSince == nil || second.LampOffSince == nil {
		t.Fatal("LampOffSince 应为非空指针")
	}
	if first.LampOffSince == second.LampOffSince {
		t.Fatal("GetBeacon 的 LampOffSince 指针应独立，不应共享内部对象")
	}

	got := s.GetBeacon("B-001")
	got.Name = "被污染"
	if original := s.GetBeacon("B-001"); original.Name != "北港灯塔" {
		t.Fatalf("GetBeacon 应返回深拷贝，原值被污染: %q", original.Name)
	}

	items := s.ListTelemetry("B-001", 0)
	items[0].Voltage = 999
	items[0].MeasuredPattern.FlashSec = 999
	items[0].Violations[0] = "polluted"
	again := s.ListTelemetry("B-001", 0)
	if again[0].Voltage != 12.3 {
		t.Fatalf("ListTelemetry 应返回深拷贝，原电压被污染: %v", again[0].Voltage)
	}
	if again[0].MeasuredPattern.FlashSec != 2 {
		t.Fatalf("ListTelemetry 的 MeasuredPattern 应独立: %+v", again[0].MeasuredPattern)
	}
	if again[0].Violations[0] != "v1" {
		t.Fatalf("ListTelemetry 的 Violations 切片应独立: %v", again[0].Violations)
	}
}

// TestConcurrentReadWriteIsolated 并发读写同一仓储，配合 -race 验证无数据竞争。
func TestConcurrentReadWriteIsolated(t *testing.T) {
	s := New("")
	now := time.Now()
	for i := 0; i < 8; i++ {
		b := domain.NewBeacon(fmt.Sprintf("B-%03d", i+1), "并发航标", domain.BeaconTypeBuoy,
			domain.Position{Lat: 30.4, Lng: 122.2}, 30,
			domain.LampPattern{FlashSec: 1, EclipseSec: 3}, now)
		if err := s.CreateBeacon(b); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			b := domain.NewBeacon(fmt.Sprintf("W-%03d", n), "写入", domain.BeaconTypeBuoy,
				domain.Position{Lat: 30.4, Lng: 122.2}, 30,
				domain.LampPattern{FlashSec: 1, EclipseSec: 3}, now)
			_ = s.CreateBeacon(b)
		}(i)
		go func() {
			defer wg.Done()
			_ = s.ListBeacons()
			_ = s.GetBeacon("B-001")
			_ = s.CountBeacons()
		}()
	}
	wg.Wait()
	if s.CountBeacons() < 8 {
		t.Fatalf("并发写入后航标数应至少为 8，got %d", s.CountBeacons())
	}
}
