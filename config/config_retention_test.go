package config

import "testing"

// TestRetentionConfigPositive 遥测保留上限默认必须开启，避免遥测无界累积。
func TestRetentionConfigPositive(t *testing.T) {
	cfg := Default()
	if cfg.MaxTelemetryPerBeacon <= 0 {
		t.Fatalf("MaxTelemetryPerBeacon 默认应大于 0，got %d", cfg.MaxTelemetryPerBeacon)
	}
}

// TestConfigRejectsNegativeRetention 遥测保留上限不能为负。
func TestConfigRejectsNegativeRetention(t *testing.T) {
	cfg := Default()
	cfg.MaxTelemetryPerBeacon = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("负的保留上限应被校验拒绝")
	}
}
