package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/store"
)

// TestSeedFailsOnBeaconCreateError 种子初始化时航标创建失败必须报错并指明航标。
func TestSeedFailsOnBeaconCreateError(t *testing.T) {
	f, err := os.CreateTemp("", "notadir-*")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	_ = f.Close()
	cfg := config.Default()
	cfg.DataFile = ""
	st := store.New(filepath.Join(name, "state.json"))
	audit := NewAuditService(st, cfg)
	tasks := NewTaskService(st, audit, cfg)
	abn := NewAbnormalityService(st, tasks, audit, cfg)
	tel := NewTelemetryService(st, abn, audit, cfg)

	err = SeedIfEmpty(st, cfg, tel)
	if err == nil {
		t.Fatal("种子初始化遇航标创建失败应返回错误")
	}
	if !strings.Contains(err.Error(), "seed beacon") {
		t.Fatalf("错误应指明航标创建失败，got %v", err)
	}
}
