package httpapi

import (
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/middleware"
	"example.com/navigation-beacon-telemetry-service/store"
)

// Deps 路由器依赖集合。
type Deps struct {
	Cfg    *config.Config
	Store  *store.Store
	Logger *log.Logger  // 兼容旧调用方；Slog 非空时优先使用结构化日志
	Slog   *slog.Logger // 结构化日志，生产路径使用
	WebFS  fs.FS        // 已裁剪到 web/ 目录的嵌入式文件系统

	BeaconHandler      *BeaconHandler
	TelemetryHandler   *TelemetryHandler
	AbnormalityHandler *AbnormalityHandler
	TaskHandler        *TaskHandler
	CommandHandler     *CommandHandler
	OverviewHandler    *OverviewHandler
	HealthHandler      *HealthHandler
	AuditHandler       *AuditHandler
}

// NewRouter 组装全部路由与中间件链。
//
// 中间件顺序：ErrorHandler(最外) → SecurityHeaders → RequestID →
// AuditLogger → DriftGuard → mux。
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// 健康检查（/healthz 供 runtime_smoke ready_url 探测，/api/healthz 为 JSON API）
	mux.HandleFunc("GET /healthz", d.HealthHandler.Health)
	mux.HandleFunc("GET /api/healthz", d.HealthHandler.Health)
	mux.HandleFunc("GET /readyz", d.HealthHandler.Ready)

	// 总览
	mux.HandleFunc("GET /api/overview", d.OverviewHandler.Get)

	// 航标台账
	mux.HandleFunc("GET /api/beacons", d.BeaconHandler.List)
	mux.HandleFunc("POST /api/beacons", d.BeaconHandler.Create)
	mux.HandleFunc("GET /api/beacons/{id}", d.BeaconHandler.Get)

	// 遥测
	mux.HandleFunc("POST /api/beacons/{id}/telemetry", d.TelemetryHandler.Report)
	mux.HandleFunc("GET /api/beacons/{id}/telemetry", d.TelemetryHandler.List)

	// 异常台账
	mux.HandleFunc("GET /api/abnormalities", d.AbnormalityHandler.List)
	mux.HandleFunc("POST /api/abnormalities", d.AbnormalityHandler.Create)
	mux.HandleFunc("GET /api/abnormalities/{id}", d.AbnormalityHandler.Get)
	mux.HandleFunc("POST /api/abnormalities/{id}/resolve", d.AbnormalityHandler.Resolve)

	// 处置任务
	mux.HandleFunc("GET /api/tasks", d.TaskHandler.List)
	mux.HandleFunc("POST /api/tasks", d.TaskHandler.Create)
	mux.HandleFunc("POST /api/tasks/{id}/assign", d.TaskHandler.Assign)
	mux.HandleFunc("POST /api/tasks/{id}/repair", d.TaskHandler.Repair)
	mux.HandleFunc("POST /api/tasks/{id}/verify", d.TaskHandler.Verify)
	mux.HandleFunc("POST /api/tasks/{id}/close", d.TaskHandler.Close)
	mux.HandleFunc("POST /api/tasks/{id}/escalate", d.TaskHandler.Escalate)

	// 遥控指令
	mux.HandleFunc("GET /api/commands", d.CommandHandler.List)
	mux.HandleFunc("GET /api/commands/{id}", d.CommandHandler.Get)
	mux.HandleFunc("POST /api/commands/{id}/ack", d.CommandHandler.Ack)
	mux.HandleFunc("POST /api/beacons/{id}/command", d.CommandHandler.Dispatch)

	// 审计日志
	mux.HandleFunc("GET /api/audits", d.AuditHandler.List)

	// 前端静态资源（SPA 回退到 index.html）
	mux.Handle("/", spaHandler{fsys: d.WebFS})

	var handler http.Handler = mux
	handler = middleware.DriftGuard(d.Store)(handler)
	if d.Slog != nil {
		handler = middleware.SlogAuditLogger(d.Slog)(handler)
	} else {
		handler = middleware.AuditLogger(d.Logger)(handler)
	}
	handler = middleware.RequestID(handler)
	handler = middleware.SecurityHeaders(handler)
	if d.Slog != nil {
		handler = middleware.SlogErrorHandler(d.Slog)(handler)
	} else {
		handler = middleware.ErrorHandler(d.Logger)(handler)
	}
	return handler
}

// spaHandler 服务嵌入式前端：真实文件直接返回，未知路径回退 index.html。
type spaHandler struct {
	fsys fs.FS
}

// ServeHTTP 实现 http.Handler。
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(h.fsys, name); err != nil {
		name = "index.html"
	}
	content, err := fs.ReadFile(h.fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType(name))
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// contentType 根据扩展名设置媒体类型。
func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "text/plain; charset=utf-8"
	}
}

// ServerTimeout 返回推荐的服务端读写超时配置。
func ServerTimeout(cfg *config.Config) (readTimeout, writeTimeout, idleTimeout time.Duration) {
	if cfg == nil {
		return 10 * time.Second, 15 * time.Second, 60 * time.Second
	}
	return cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout
}
