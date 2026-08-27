// Package middleware 提供审计日志、统一错误处理与漂移守卫等横切关注点。
package middleware

import (
	"bufio"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// statusRecorder 捕获响应状态码与字节数，供访问日志输出。
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader 记录状态码。
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write 记录响应字节数。
func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush 支持流式响应。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack 支持 WebSocket 升级场景。
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

// Push 支持 HTTP/2 服务端推送（透传）。
func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap 暴露底层 ResponseWriter。
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// AuditLogger 请求访问日志中间件（兼容旧 *log.Logger 调用方；内部统一走 slog）。
func AuditLogger(logger *log.Logger) func(http.Handler) http.Handler {
	sl := slog.New(slog.NewTextHandler(logger.Writer(), &slog.HandlerOptions{Level: slog.LevelInfo}))
	return SlogAuditLogger(sl)
}

// SlogAuditLogger 请求访问日志中间件：记录方法、路径、状态码、耗时、字节数与 requestID。
func SlogAuditLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			logger.Info(
				r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rec.status),
				"status", rec.status,
				"duration", time.Since(start).Round(time.Microsecond),
				"bytes", rec.bytes,
				"request_id", GetRequestID(r),
			)
		})
	}
}
