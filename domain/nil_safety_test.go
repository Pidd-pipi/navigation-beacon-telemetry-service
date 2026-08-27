package domain

import (
	"testing"
	"time"
)

// TestCloneNilPatternTelemetryNoPanic 灯灭遥测（无实测灯质）克隆不应 panic。
func TestCloneNilPatternTelemetryNoPanic(t *testing.T) {
	now := time.Now()
	tel := NewTelemetryData("T-1", "B-001", LampStateOff, 12.0, 0.8,
		Position{Lat: 30.5, Lng: 122.1}, nil, now, now)
	cp := tel.Clone()
	if cp.MeasuredPattern != nil {
		t.Fatal("克隆后实测灯质应为 nil")
	}
	if cp.BeaconID != "B-001" {
		t.Fatalf("克隆字段异常: %+v", cp)
	}
}

// TestEffectiveStatusNoTelemetryOffline 无遥测航标状态应为离线且不 panic。
func TestEffectiveStatusNoTelemetryOffline(t *testing.T) {
	b := NewBeacon("B-1", "无遥测航标", BeaconTypeBuoy,
		Position{Lat: 30.5, Lng: 122.1}, 30, LampPattern{FlashSec: 1, EclipseSec: 3}, time.Now())
	if got := b.EffectiveStatus(time.Now(), 45*time.Minute); got != BeaconStatusOffline {
		t.Fatalf("无遥测应判定离线，got %s", got)
	}
}

// TestLampOffDurationZeroWhenNotOff 未灭灯航标灭灯时长应为 0 且不 panic。
func TestLampOffDurationZeroWhenNotOff(t *testing.T) {
	b := NewBeacon("B-1", "正常航标", BeaconTypeLighthouse,
		Position{Lat: 30.5, Lng: 122.1}, 50, LampPattern{FlashSec: 2, EclipseSec: 2}, time.Now())
	if got := b.LampOffDuration(time.Now()); got != 0 {
		t.Fatalf("未灭灯时长应为 0，got %v", got)
	}
}

// TestAbnormalityCloneResolvedAtIndependent 异常克隆的 ResolvedAt 指针应相互独立。
func TestAbnormalityCloneResolvedAtIndependent(t *testing.T) {
	now := time.Now()
	a := NewLampAbnormality("A-1", "B-1", AbnormalityTypeLampOut, "灭灯", now)
	a.Resolve("已恢复", now)

	c1 := a.Clone()
	c2 := a.Clone()
	later := now.Add(time.Hour)
	*c1.ResolvedAt = later
	if c2.ResolvedAt.Equal(later) {
		t.Fatal("克隆间 ResolvedAt 指针应独立，修改一个不应影响另一个")
	}
}
