package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// TestOverviewRecentCommandsNewest 总览最近指令应取最新 5 条且最新在前。
func TestOverviewRecentCommandsNewest(t *testing.T) {
	st, cfg, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	for i := 0; i < 8; i++ {
		if _, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	ov := NewOverviewService(st, cfg).Build(now.Add(8 * time.Minute))
	if len(ov.RecentCommands) != 5 {
		t.Fatalf("最近指令应取 5 条，got %d", len(ov.RecentCommands))
	}
	if !ov.RecentCommands[0].SentAt.Equal(now.Add(7 * time.Minute)) {
		t.Fatalf("最近指令首位应为最新一条，got %v", ov.RecentCommands[0].SentAt)
	}
}

// TestOverviewRecentAuditsNewest 总览最近审计应取最新 5 条且最新在前。
func TestOverviewRecentAuditsNewest(t *testing.T) {
	st, cfg, _, _, _, _, audit := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	for i := 0; i < 8; i++ {
		audit.LogAt(now.Add(time.Duration(i)*time.Minute), "audit.op", "test", "E-1", "op", "批量操作")
	}

	ov := NewOverviewService(st, cfg).Build(now.Add(8 * time.Minute))
	if len(ov.RecentAudits) != 5 {
		t.Fatalf("最近审计应取 5 条，got %d", len(ov.RecentAudits))
	}
	if !ov.RecentAudits[0].CreatedAt.Equal(now.Add(7 * time.Minute)) {
		t.Fatalf("最近审计首位应为最新一条，got %v", ov.RecentAudits[0].CreatedAt)
	}
}

// TestOverviewTodayCount 今日指令数只统计最近 24 小时内下发的指令。
func TestOverviewTodayCount(t *testing.T) {
	st, cfg, _, _, _, commands, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	// 3 条今天下发，2 条两天前下发
	for i := 0; i < 3; i++ {
		if _, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now.Add(-time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now.Add(-48*time.Hour-time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	ov := NewOverviewService(st, cfg).Build(now)
	if ov.Commands.Today != 3 {
		t.Fatalf("今日指令数应为 3，got %d", ov.Commands.Today)
	}
}
