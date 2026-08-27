package middleware

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// errorResponse 统一错误响应格式。
type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorHandler 统一错误/panic 处理中间件（兼容旧 *log.Logger 调用方）。
func ErrorHandler(logger *log.Logger) func(http.Handler) http.Handler {
	sl := slog.New(slog.NewTextHandler(logger.Writer(), &slog.HandlerOptions{Level: slog.LevelError}))
	return SlogErrorHandler(sl)
}

// SlogErrorHandler 统一错误/panic 处理中间件：拦截 panic 并返回 JSON 500。
func SlogErrorHandler(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"request_id", GetRequestID(r),
					)
					writeErrorJSON(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// writeErrorJSON 输出统一错误 JSON（供各中间件与 handler 复用）。
func writeErrorJSON(w http.ResponseWriter, code int, errCode string, message string) {
	body := errorResponse{Code: code, Message: message}
	payload, _ := json.Marshal(body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(payload)
}
