package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// DriftGuard 漂移守卫中间件：航标处于漂移状态时，拦截关灯指令下发
// （保障航行可见性）。仅对 POST /api/beacons/{id}/command 生效。
func DriftGuard(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && isCommandPath(r.URL.Path) {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					var maxErr *http.MaxBytesError
					if errors.As(err, &maxErr) {
						writeErrorJSON(w, http.StatusRequestEntityTooLarge, "payload_too_large", "请求体过大")
						return
					}
					writeErrorJSON(w, http.StatusBadRequest, "validation", "读取请求体失败")
					return
				}
				// 无论指令类型如何，都必须把请求体还原给后续 handler 解析，
				// 否则下游 decodeJSON 会因 body 已被读空而得到 EOF。
				r.Body = io.NopCloser(bytes.NewReader(body))
				var payload struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(body, &payload) == nil && payload.Type == string(domain.CommandTypeOff) {
					id := extractBeaconID(r.URL.Path)
					if b := st.GetBeacon(id); b != nil && b.Drifting {
						writeErrorJSON(w, http.StatusConflict, "drift_guard",
							"航标处于漂移状态，为保障航行安全禁止下发关灯指令")
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isCommandPath 判断是否为遥控指令下发路径。
func isCommandPath(p string) bool {
	return strings.HasPrefix(p, "/api/beacons/") && strings.HasSuffix(p, "/command")
}

// extractBeaconID 从路径 /api/beacons/{id}/command 提取航标 ID。
func extractBeaconID(p string) string {
	rest := strings.TrimPrefix(p, "/api/beacons/")
	rest = strings.TrimSuffix(rest, "/command")
	return strings.Trim(rest, "/")
}
