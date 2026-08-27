package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/service"
	"example.com/navigation-beacon-telemetry-service/store"
)

// newTestRouter 组装与 main.go 一致的完整路由（内存仓储 + 嵌入式前端模拟）。
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Default()
	cfg.DataFile = "" // 不落盘
	logger := log.New(os.Stderr, "[test] ", 0)

	st := store.New("")
	auditSvc := service.NewAuditService(st, cfg)
	taskSvc := service.NewTaskService(st, auditSvc, cfg)
	abnSvc := service.NewAbnormalityService(st, taskSvc, auditSvc, cfg)
	cmdSvc := service.NewCommandService(st, auditSvc, cfg)
	telSvc := service.NewTelemetryService(st, abnSvc, auditSvc, cfg)
	ovSvc := service.NewOverviewService(st, cfg)
	if err := service.SeedIfEmpty(st, cfg, telSvc); err != nil {
		t.Fatalf("seed: %v", err)
	}

	webFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>nav beacon</body></html>")},
		"app.js":     &fstest.MapFile{Data: []byte("/* app */")},
		"style.css":  &fstest.MapFile{Data: []byte("/* css */")},
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

func doJSON(t *testing.T, h http.Handler, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
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
	req.Header.Set("X-Operator", "tester")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var payload map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	}
	return rec, payload
}

func decodeData[T any](t *testing.T, payload map[string]any, out *T) {
	t.Helper()
	raw, _ := json.Marshal(payload["data"])
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

type telemetryResp struct {
	Telemetry struct {
		ID         string   `json:"id"`
		Violations []string `json:"violations"`
	} `json:"telemetry"`
	Violations []string `json:"violations"`
}

func TestRouterHealthAndPages(t *testing.T) {
	h := newTestRouter(t)

	rec, payload := doJSON(t, h, http.MethodGet, "/api/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d", rec.Code)
	}
	if payload["code"] != float64(0) {
		t.Errorf("healthz code = %v", payload["code"])
	}

	// GET / 返回前端页面
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "nav beacon") {
		t.Errorf("GET / 应返回前端页面: status=%d body=%q", rec2.Code, rec2.Body.String())
	}

	// SPA 回退：/beacons/B-001 也应返回 index.html
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/beacons/B-001", nil))
	if rec3.Code != http.StatusOK || !strings.Contains(rec3.Body.String(), "nav beacon") {
		t.Errorf("SPA 回退失败: status=%d", rec3.Code)
	}

	// 未知 API 返回 404
	rec4, _ := doJSON(t, h, http.MethodGet, "/api/nope", nil)
	if rec4.Code != http.StatusNotFound {
		t.Errorf("未知 API 应 404，got %d", rec4.Code)
	}
}

func TestRouterCoreBusinessChains(t *testing.T) {
	h := newTestRouter(t)
	now := time.Now()

	// ---- 链路 1：航标创建 → 遥测上报（灯质偏差） ----
	rec, payload := doJSON(t, h, http.MethodPost, "/api/beacons", map[string]any{
		"name": "测试灯塔", "type": "lighthouse",
		"anchor":         map[string]any{"lat": 30.5, "lng": 122.1},
		"drift_radius_m": 50,
		"lamp_pattern":   map[string]any{"flash_sec": 2, "eclipse_sec": 2},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建航标 status=%d body=%s", rec.Code, rec.Body.String())
	}
	beaconID := payload["data"].(map[string]any)["id"].(string)

	rec, payload = doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/telemetry", map[string]any{
		"lamp_state": "on", "voltage": 12.3, "current": 0.8,
		"position":         map[string]any{"lat": 30.5, "lng": 122.1},
		"measured_pattern": map[string]any{"flash_sec": 4.0, "eclipse_sec": 4.0},
		"reported_at":      now.Format(time.RFC3339),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("遥测上报 status=%d body=%s", rec.Code, rec.Body.String())
	}
	var tr telemetryResp
	decodeData(t, payload, &tr)
	if len(tr.Violations) == 0 {
		t.Fatalf("灯质偏差遥测应返回违规项: %+v", payload)
	}

	// 异常台账应包含 lamp_mismatch
	rec, payload = doJSON(t, h, http.MethodGet, "/api/abnormalities?type=lamp_mismatch&status=open", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("异常台账 status=%d", rec.Code)
	}
	abns := payload["data"].([]any)
	if len(abns) == 0 {
		t.Fatal("应存在灯质偏差异常")
	}

	// ---- 链路 2：漂移 → 关灯被守卫拦截 → 回位 → 关灯成功 → 回执 ----
	doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/telemetry", map[string]any{
		"lamp_state": "on", "voltage": 12.3, "current": 0.8,
		"position":         map[string]any{"lat": 30.502, "lng": 122.1}, // 约 222 米 > 50 米
		"measured_pattern": map[string]any{"flash_sec": 2, "eclipse_sec": 2},
		"reported_at":      now.Add(1 * time.Minute).Format(time.RFC3339),
	})

	rec, payload = doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/command", map[string]any{"type": "off"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("漂移期间关灯应 409，got %d body=%s", rec.Code, rec.Body.String())
	}
	if payload["code"] != float64(http.StatusConflict) {
		t.Errorf("漂移拦截错误码应为 409，got %v", payload["code"])
	}

	// 回位
	doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/telemetry", map[string]any{
		"lamp_state": "on", "voltage": 12.3, "current": 0.8,
		"position":         map[string]any{"lat": 30.5, "lng": 122.1},
		"measured_pattern": map[string]any{"flash_sec": 2, "eclipse_sec": 2},
		"reported_at":      now.Add(2 * time.Minute).Format(time.RFC3339),
	})

	rec, payload = doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/command", map[string]any{"type": "off"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("回位后关灯应成功，got %d body=%s", rec.Code, rec.Body.String())
	}
	cmdID := payload["data"].(map[string]any)["id"].(string)

	// 终端回执
	rec, payload = doJSON(t, h, http.MethodPost, "/api/commands/"+cmdID+"/ack", map[string]any{"success": true, "message": "已关灯"})
	if rec.Code != http.StatusOK {
		t.Fatalf("回执 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if payload["data"].(map[string]any)["status"] != "success" {
		t.Errorf("回执后状态应为 success: %v", payload)
	}

	// ---- 链路 3：灭灯 → 自动任务 → 派发 → 修复 → 复测关闭 ----
	t1 := now.Add(-40 * time.Minute).Format(time.RFC3339)
	t2 := now.Add(-5 * time.Minute).Format(time.RFC3339)
	doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/telemetry", map[string]any{
		"lamp_state": "off", "voltage": 12.0, "current": 0.2,
		"position":    map[string]any{"lat": 30.5, "lng": 122.1},
		"reported_at": t1,
	})
	doJSON(t, h, http.MethodPost, "/api/beacons/"+beaconID+"/telemetry", map[string]any{
		"lamp_state": "off", "voltage": 12.0, "current": 0.2,
		"position":    map[string]any{"lat": 30.5, "lng": 122.1},
		"reported_at": t2,
	})

	rec, payload = doJSON(t, h, http.MethodGet, "/api/tasks?beacon_id="+beaconID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("任务列表 status=%d", rec.Code)
	}
	tasks := payload["data"].([]any)
	if len(tasks) == 0 {
		t.Fatal("灭灯故障应自动生成任务")
	}
	task := tasks[0].(map[string]any)
	if task["status"] != "created" {
		t.Errorf("任务初始状态应为 created，got %v", task["status"])
	}
	taskID := task["id"].(string)

	rec, payload = doJSON(t, h, http.MethodPost, "/api/tasks/"+taskID+"/assign", map[string]any{"assignee": "张三"})
	if rec.Code != http.StatusOK || payload["data"].(map[string]any)["status"] != "assigned" {
		t.Fatalf("派发失败: %d %s", rec.Code, rec.Body.String())
	}
	doJSON(t, h, http.MethodPost, "/api/tasks/"+taskID+"/repair", map[string]any{"note": "更换灯器"})
	rec, payload = doJSON(t, h, http.MethodPost, "/api/tasks/"+taskID+"/verify", map[string]any{"result": "复测正常", "auto_close": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("复测关闭 status=%d body=%s", rec.Code, rec.Body.String())
	}
	if payload["data"].(map[string]any)["status"] != "closed" {
		t.Errorf("复测并关闭后状态应为 closed，got %v", payload["data"].(map[string]any)["status"])
	}

	// 非法流转：已关闭任务再次派发应 409
	rec, _ = doJSON(t, h, http.MethodPost, "/api/tasks/"+taskID+"/assign", map[string]any{"assignee": "李四"})
	if rec.Code != http.StatusConflict {
		t.Errorf("已关闭任务重复派发应 409，got %d", rec.Code)
	}

	// ---- 总览聚合 ----
	rec, payload = doJSON(t, h, http.MethodGet, "/api/overview", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview status=%d", rec.Code)
	}
	beacons := payload["data"].(map[string]any)["beacons"].(map[string]any)
	t.Logf("overview beacons: %v", beacons)
	if int(beacons["total"].(float64)) < 4 {
		t.Errorf("overview 航标总数应 ≥4（3 演示+1 新建），got %v", beacons["total"])
	}

	// ---- 审计日志 ----
	rec, payload = doJSON(t, h, http.MethodGet, "/api/audits?limit=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audits status=%d", rec.Code)
	}
	if len(payload["data"].([]any)) == 0 {
		t.Error("审计日志不应为空")
	}
}

func TestRouterValidationErrors(t *testing.T) {
	h := newTestRouter(t)

	// 非法航标类型
	rec, _ := doJSON(t, h, http.MethodPost, "/api/beacons", map[string]any{
		"name": "x", "type": "rocket",
		"anchor":       map[string]any{"lat": 30.5, "lng": 122.1},
		"lamp_pattern": map[string]any{"flash_sec": 2, "eclipse_sec": 2},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法航标类型应 400，got %d", rec.Code)
	}

	// 灯亮缺实测灯质
	rec, _ = doJSON(t, h, http.MethodPost, "/api/beacons/B-001/telemetry", map[string]any{
		"lamp_state": "on", "voltage": 12.3,
		"position": map[string]any{"lat": 30.5, "lng": 122.1},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("缺实测灯质应 400，got %d", rec.Code)
	}

	// 不存在的航标遥测
	rec, _ = doJSON(t, h, http.MethodPost, "/api/beacons/B-999/telemetry", map[string]any{
		"lamp_state": "off", "voltage": 12.3,
		"position": map[string]any{"lat": 30.5, "lng": 122.1},
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("不存在的航标应 404，got %d", rec.Code)
	}
}

func TestRouterPanicRecovery(t *testing.T) {
	// 通过 /api/healthz 正常路径验证 errorHandler 不破坏正常请求；
	// panic 恢复由中间件单测覆盖，这里验证统一响应结构。
	h := newTestRouter(t)
	rec, payload := doJSON(t, h, http.MethodGet, "/api/beacons", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("beacons status=%d", rec.Code)
	}
	if _, ok := payload["code"]; !ok {
		t.Error("响应应包含统一 code 字段")
	}
}
