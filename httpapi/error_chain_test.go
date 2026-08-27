package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// getJSON 通过真实 router 发送请求并返回响应。
func getJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestMissingBeaconReturns404 查询不存在的航标应返回 404 而非 500。
func TestMissingBeaconReturns404(t *testing.T) {
	h := newTestRouter(t)
	rec := getJSON(t, h, http.MethodGet, "/api/beacons/B-999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的航标应 404，got %d", rec.Code)
	}
}

// TestMissingCommandReturns404 查询不存在的指令应返回 404 而非 500。
func TestMissingCommandReturns404(t *testing.T) {
	h := newTestRouter(t)
	rec := getJSON(t, h, http.MethodGet, "/api/commands/C-999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的指令应 404，got %d", rec.Code)
	}
}

// TestAckMissingCommandReturns404 回执不存在的指令应返回 404 而非 500。
func TestAckMissingCommandReturns404(t *testing.T) {
	h := newTestRouter(t)
	rec := getJSON(t, h, http.MethodPost, "/api/commands/C-999/ack", map[string]any{
		"success": true, "message": "ok",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("回执不存在的指令应 404，got %d", rec.Code)
	}
}

// TestCreateTaskMissingAbnormalityReturns404 为不存在的异常创建任务应返回 404 而非 500。
func TestCreateTaskMissingAbnormalityReturns404(t *testing.T) {
	h := newTestRouter(t)
	rec := getJSON(t, h, http.MethodPost, "/api/tasks", map[string]any{
		"abnormality_id": "A-999",
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的异常创建任务应 404，got %d", rec.Code)
	}
}

// TestDuplicateAckReturns409 重复回执同一指令应返回 409 而非 500。
func TestDuplicateAckReturns409(t *testing.T) {
	h := newTestRouter(t)
	rec := getJSON(t, h, http.MethodPost, "/api/beacons/B-001/command", map[string]any{"type": "on"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("下发指令失败: %d", rec.Code)
	}
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	rec = getJSON(t, h, http.MethodPost, "/api/commands/"+payload.Data.ID+"/ack", map[string]any{"success": true, "message": "ok"})
	if rec.Code != http.StatusOK {
		t.Fatalf("首次回执应成功: %d", rec.Code)
	}
	rec = getJSON(t, h, http.MethodPost, "/api/commands/"+payload.Data.ID+"/ack", map[string]any{"success": true, "message": "again"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("重复回执应 409，got %d", rec.Code)
	}
}
