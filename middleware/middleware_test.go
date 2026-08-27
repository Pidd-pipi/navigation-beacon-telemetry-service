package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

func TestDriftGuardBlocksOffWhileDrifting(t *testing.T) {
	st := store.New("")
	now := time.Now()
	b := domain.NewBeacon("B-001", "漂移浮标", domain.BeaconTypeBuoy,
		domain.Position{Lat: 30.4, Lng: 122.2}, 30, domain.LampPattern{FlashSec: 1, EclipseSec: 3}, now)
	b.Drifting = true
	if err := st.CreateBeacon(b); err != nil {
		t.Fatal(err)
	}

	handler := DriftGuard(st)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	// 漂移期间关灯 → 409
	body := bytes.NewReader([]byte(`{"type":"off"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/beacons/B-001/command", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("漂移关灯应 409，got %d", rec.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["code"] != float64(http.StatusConflict) {
		t.Errorf("错误码应为 409，got %v", payload["code"])
	}

	// 漂移期间开灯 → 放行
	body = bytes.NewReader([]byte(`{"type":"on"}`))
	req = httptest.NewRequest(http.MethodPost, "/api/beacons/B-001/command", body)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("漂移开灯应放行，got %d", rec.Code)
	}

	// 非指令路径不受影响
	req = httptest.NewRequest(http.MethodPost, "/api/beacons/B-001/telemetry", bytes.NewReader([]byte(`{}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("非指令路径应放行，got %d", rec.Code)
	}
}

func TestErrorHandlerRecoversPanic(t *testing.T) {
	logger := log.New(os.Stderr, "[test] ", 0)
	handler := ErrorHandler(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应 500，got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "服务内部错误") {
		t.Errorf("错误响应应包含统一信息: %s", rec.Body.String())
	}
}

func TestAuditLoggerWritesLog(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "[test] ", 0)
	handler := AuditLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := buf.String()
	if !strings.Contains(out, "GET /api/healthz -> 200") {
		t.Errorf("审计日志应包含方法/路径/状态: %q", out)
	}
}
