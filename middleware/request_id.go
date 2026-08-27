package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type requestIDKey struct{}

// RequestID 中间件：优先沿用客户端 X-Request-ID，否则生成 16 字节随机 ID，
// 并将 ID 写入响应头与请求上下文，便于全链路追踪。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set("X-Request-ID", rid)
		ctx := context.WithValue(r.Context(), requestIDKey{}, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID 从请求上下文读取 requestID，不存在时返回空串。
func GetRequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if rid, ok := r.Context().Value(requestIDKey{}).(string); ok {
		return rid
	}
	return r.Header.Get("X-Request-ID")
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
