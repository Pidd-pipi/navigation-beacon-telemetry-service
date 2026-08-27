package store

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// appendAuditsWithBase 以 base 为起点追加 n 条审计日志（CreatedAt 递增）。
func appendAuditsWithBase(t *testing.T, s *Store, n int, base time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		l := domain.NewAuditLog(s.NextID("L"), "audit.append", "test", "E-1", "op",
			"批量操作", base.Add(time.Duration(i)*time.Minute))
		if err := s.AppendAudit(l, 1000); err != nil {
			t.Fatalf("AppendAudit: %v", err)
		}
	}
}

// TestAuditRetentionKeepsNewest 审计超过上限后必须保留最新 N 条。
func TestAuditRetentionKeepsNewest(t *testing.T) {
	s := New("")
	base := time.Now()
	appendAuditsWithBase(t, s, 1500, base)

	if got := s.CountAudits(); got != 1000 {
		t.Fatalf("超过上限后应保留 1000 条，got %d", got)
	}
	items := s.ListAudits(0)
	if len(items) != 1000 {
		t.Fatalf("列表应 1000 条，got %d", len(items))
	}
	// 最新一条（base+1499m）必须保留在首位
	if !items[0].CreatedAt.Equal(base.Add(1499 * time.Minute)) {
		t.Fatalf("最新审计应被保留在首位，got %v", items[0].CreatedAt)
	}
}

// TestListAuditsNewestFirst 审计列表应最新在前。
func TestListAuditsNewestFirst(t *testing.T) {
	s := New("")
	base := time.Now()
	appendAuditsWithBase(t, s, 5, base)

	items := s.ListAudits(0)
	if len(items) != 5 {
		t.Fatalf("列表应 5 条，got %d", len(items))
	}
	if !items[0].CreatedAt.Equal(base.Add(4 * time.Minute)) {
		t.Fatalf("审计列表应最新在前，首位应为 base+4m，got %v", items[0].CreatedAt)
	}
}

// TestListAuditsLimitNewest limit 应返回最新 N 条。
func TestListAuditsLimitNewest(t *testing.T) {
	s := New("")
	base := time.Now()
	appendAuditsWithBase(t, s, 5, base)

	items := s.ListAudits(2)
	if len(items) != 2 {
		t.Fatalf("limit=2 应返回 2 条，got %d", len(items))
	}
	if !items[0].CreatedAt.Equal(base.Add(4 * time.Minute)) {
		t.Fatalf("limit 应取最新 2 条，首位应为 base+4m，got %v", items[0].CreatedAt)
	}
	if !items[1].CreatedAt.Equal(base.Add(3 * time.Minute)) {
		t.Fatalf("limit 第二条应为 base+3m，got %v", items[1].CreatedAt)
	}
}
