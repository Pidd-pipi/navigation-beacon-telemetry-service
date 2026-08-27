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
				var payload struct {
					Type string `json:"type"`
				}
				// 请求体不合法时交给后续 handler 返回更准确的校验错误
				if json.Unmarshal(body, &payload) == nil && payload.Type == string(domain.CommandTypeOff) {
					id := extractBeaconID(r.URL.Path)
					if b := st.GetBeacon(id); b != nil && b.Drifting {
						writeErrorJSON(w, http.StatusConflict, "drift_guard",
							"航标处于漂移状态，为保障航行安全禁止下发关灯指令")
						return
					}
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
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
