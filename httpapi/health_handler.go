package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler 健康检查处理器。
//
// 存活探针 /healthz 始终返回 200（进程存活即算）；就绪探针 /readyz
// 在服务进入关停（shutdownCh 关闭）后返回 503，使部署平台停止下发新流量。
type HealthHandler struct {
	startedAt  time.Time
	shutdownCh chan struct{}
}

// SetShutdownCh 注册服务关停信号通道；关停后 Ready 不再报告就绪。
// 通道关闭即视为进入关停，应在 cancel sweeper 之前关闭，使 /readyz 先摘流量。
func (h *HealthHandler) SetShutdownCh(ch chan struct{}) {
	h.shutdownCh = ch
}

// shuttingDown 报告服务是否正在关停。nil 通道视为未注册（永不关停）。
func (h *HealthHandler) shuttingDown() bool {
	if h.shutdownCh == nil {
		return false
	}
	select {
	case <-h.shutdownCh:
		return true
	default:
		return false
	}
}

// NewHealthHandler 构造健康检查处理器。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{startedAt: time.Now()}
}

// Health GET /healthz 与 GET /api/healthz 返回 200。
// 存活探针只反映进程是否存活，不因关停而失败。
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{
		"status":     "ok",
		"service":    "navigation-beacon-telemetry-service",
		"uptime_sec": int(time.Since(h.startedAt).Seconds()),
	})
}

// Ready GET /readyz 返回就绪状态。
// 关停中（shutdownCh 已关闭）返回 503，部署平台据此摘除流量；其余返回 200。
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.shuttingDown() {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Response{
			Code:    http.StatusServiceUnavailable,
			Message: "shutting down",
			Data: map[string]any{
				"status":     "shutting_down",
				"service":    "navigation-beacon-telemetry-service",
				"uptime_sec": int(time.Since(h.startedAt).Seconds()),
			},
		})
		return
	}
	OK(w, map[string]any{
		"status":     "ready",
		"service":    "navigation-beacon-telemetry-service",
		"uptime_sec": int(time.Since(h.startedAt).Seconds()),
	})
}
