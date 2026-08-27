package httpapi

import (
	"net/http"
	"time"
)

// HealthHandler 健康检查处理器。
type HealthHandler struct {
	startedAt  time.Time
	shutdownCh chan struct{}
}

// SetShutdownCh 注册服务关停信号通道；关停后就绪探针应不再报告就绪。
func (h *HealthHandler) SetShutdownCh(ch chan struct{}) {
	h.shutdownCh = ch
}

// NewHealthHandler 构造健康检查处理器。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{startedAt: time.Now()}
}

// Health GET /healthz 与 GET /api/healthz 返回 200。
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{
		"status":     "ok",
		"service":    "navigation-beacon-telemetry-service",
		"uptime_sec": int(time.Since(h.startedAt).Seconds()),
	})
}

// Ready GET /readyz 返回 200（服务启动后即可就绪）。
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{
		"status":     "ready",
		"service":    "navigation-beacon-telemetry-service",
		"uptime_sec": int(time.Since(h.startedAt).Seconds()),
	})
}
