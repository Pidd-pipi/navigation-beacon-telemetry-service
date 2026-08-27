package service

import (
	"testing"
	"time"
)

// TestAuditServiceRetentionApplied 审计服务记录超过上限后应裁剪。
func TestAuditServiceRetentionApplied(t *testing.T) {
	st, cfg, _, _, _, _, audit := newTestServices(t)
	cfg.AuditRetention = 1000

	now := time.Now()
	for i := 0; i < 1500; i++ {
		audit.LogAt(now.Add(time.Duration(i)*time.Minute), "audit.op", "test", "E-1", "op", "批量操作")
	}

	if got := st.CountAudits(); got != 1000 {
		t.Fatalf("审计服务裁剪后应保留 1000 条，got %d", got)
	}
}
