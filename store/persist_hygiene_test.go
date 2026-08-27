package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// TestSaveFailureCleansTempFile 保存失败时不得残留临时文件。
func TestSaveFailureCleansTempFile(t *testing.T) {
	dir := t.TempDir()
	// 目标路径是一个目录，rename 必然失败
	bad := filepath.Join(dir, "state.json")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(bad)
	if err := s.Save(); err == nil {
		t.Fatal("Save 应失败")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".beacon_state-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("保存失败后残留临时文件: %s", e.Name())
		}
	}
}

// TestLoadNullMapsInitialized 加载包含 null 映射的快照后，各映射必须保持可写。
func TestLoadNullMapsInitialized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	content := `{"beacons": null, "telemetry": null, "abnormalities": null, "tasks": null, "commands": null, "audits": null, "seq": null}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(path)
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	// 加载后必须能正常写入，不得因 nil 映射 panic
	b := domain.NewBeacon("B-001", "测试航标", domain.BeaconTypeLighthouse,
		domain.Position{Lat: 30.5, Lng: 122.1}, 50,
		domain.LampPattern{FlashSec: 2, EclipseSec: 2}, time.Now())
	if err := s.CreateBeacon(b); err != nil {
		t.Fatalf("null 映射加载后写入失败: %v", err)
	}
	if s.GetBeacon("B-001") == nil {
		t.Fatal("写入后应能读到航标")
	}
}
