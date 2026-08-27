package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReadyReturns503DuringShutdown 服务进入关停后就绪探针应返回 503。
func TestReadyReturns503DuringShutdown(t *testing.T) {
	h := NewHealthHandler()
	ch := make(chan struct{})
	h.SetShutdownCh(ch)
	close(ch)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("关停中就绪探针应 503，got %d", rec.Code)
	}
}

// TestReadyOkBeforeShutdown 未进入关停时就绪探针应 200。
func TestReadyOkBeforeShutdown(t *testing.T) {
	h := NewHealthHandler()
	ch := make(chan struct{})
	h.SetShutdownCh(ch)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("未关停时就绪探针应 200，got %d", rec.Code)
	}
}
