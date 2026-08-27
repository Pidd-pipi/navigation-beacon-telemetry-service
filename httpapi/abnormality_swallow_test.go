package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/service"
	"example.com/navigation-beacon-telemetry-service/store"
)

// badPathDataFile 返回一个持久化必然失败的数据文件路径。
func badPathDataFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "notadir-*")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	_ = f.Close()
	return filepath.Join(name, "state.json")
}

// newTestRouterWithStore 使用指定数据文件路径构造路由（用于模拟持久化失败）。
func newTestRouterWithStore(dataFile string) http.Handler {
	cfg := config.Default()
	cfg.DataFile = dataFile
	logger := log.New(os.Stderr, "[test] ", 0)
	st := store.New(dataFile)
	auditSvc := service.NewAuditService(st, cfg)
	taskSvc := service.NewTaskService(st, auditSvc, cfg)
	abnSvc := service.NewAbnormalityService(st, taskSvc, auditSvc, cfg)
	cmdSvc := service.NewCommandService(st, auditSvc, cfg)
	telSvc := service.NewTelemetryService(st, abnSvc, auditSvc, cfg)
	ovSvc := service.NewOverviewService(st, cfg)
	webFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>nav beacon</body></html>")},
	}
	return NewRouter(Deps{
		Cfg:                cfg,
		Store:              st,
		Logger:             logger,
		WebFS:              webFS,
		BeaconHandler:      NewBeaconHandler(st, abnSvc, taskSvc, telSvc, auditSvc, cfg.OfflineAfter),
		TelemetryHandler:   NewTelemetryHandler(telSvc),
		AbnormalityHandler: NewAbnormalityHandler(abnSvc),
		TaskHandler:        NewTaskHandler(taskSvc),
		CommandHandler:     NewCommandHandler(cmdSvc),
		OverviewHandler:    NewOverviewHandler(ovSvc),
		HealthHandler:      NewHealthHandler(),
		AuditHandler:       NewAuditHandler(auditSvc),
	})
}

// doPost 通过真实 router 发送 JSON POST 请求。
func doPost(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestCreateAbnormalityPropagatesError 持久化失败时手工登记异常不得返回成功。
func TestCreateAbnormalityPropagatesError(t *testing.T) {
	h := newTestRouterWithStore(badPathDataFile(t))
	rec := doPost(t, h, "/api/abnormalities", map[string]any{
		"beacon_id": "B-001",
		"type":      "drift",
		"detail":    "漂移",
	})
	if rec.Code == http.StatusCreated {
		t.Fatal("持久化失败时创建异常不应返回 201")
	}
}

// TestResolveAbnormalityPropagatesError 解决异常失败时不得返回成功。
func TestResolveAbnormalityPropagatesError(t *testing.T) {
	h := newTestRouterWithStore(badPathDataFile(t))
	rec := doPost(t, h, "/api/abnormalities/A-999/resolve", map[string]any{"reason": "已恢复"})
	if rec.Code == http.StatusOK {
		t.Fatal("解决不存在的异常不应返回 200")
	}
}

// TestGetMissingAbnormalityReturns404 查询不存在的异常应返回 404。
func TestGetMissingAbnormalityReturns404(t *testing.T) {
	h := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/abnormalities/A-999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的异常应 404，got %d", rec.Code)
	}
}
