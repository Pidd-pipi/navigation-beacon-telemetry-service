package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDispatchOnCommandSucceeds 开灯指令经漂移守卫后请求体必须完整可读。
func TestDispatchOnCommandSucceeds(t *testing.T) {
	h := newTestRouter(t)
	raw, _ := json.Marshal(map[string]any{"type": "on"})
	req := httptest.NewRequest(http.MethodPost, "/api/beacons/B-001/command", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("开灯指令应成功下发，got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestDispatchSwitchPatternSucceeds 切换灯质指令经漂移守卫后请求体必须完整可读。
func TestDispatchSwitchPatternSucceeds(t *testing.T) {
	h := newTestRouter(t)
	raw, _ := json.Marshal(map[string]any{
		"type":           "switch_pattern",
		"target_pattern": map[string]any{"flash_sec": 3, "eclipse_sec": 3},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/beacons/B-001/command", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("切换灯质指令应成功下发，got %d body=%s", rec.Code, rec.Body.String())
	}
}
