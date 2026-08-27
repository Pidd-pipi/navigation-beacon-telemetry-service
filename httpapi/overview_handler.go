package httpapi

import (
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/service"
)

// OverviewHandler 总览聚合处理器。
type OverviewHandler struct {
	overview *service.OverviewService
}

// NewOverviewHandler 构造总览处理器。
func NewOverviewHandler(overview *service.OverviewService) *OverviewHandler {
	return &OverviewHandler{overview: overview}
}

// Get GET /api/overview 返回总览聚合数据。
func (h *OverviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	OK(w, h.overview.Build(time.Now()))
}
