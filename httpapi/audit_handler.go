package httpapi

import (
	"net/http"

	"example.com/navigation-beacon-telemetry-service/service"
)

// AuditHandler 审计日志处理器。
type AuditHandler struct {
	audit *service.AuditService
}

// NewAuditHandler 构造审计日志处理器。
func NewAuditHandler(audit *service.AuditService) *AuditHandler {
	return &AuditHandler{audit: audit}
}

// List GET /api/audits 返回审计日志（最新在前），支持 limit/offset 分页。
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	items := h.audit.List(0) // 0 表示取全部，分页在 handler 层统一完成
	OKPage(w, paginate(items, page), len(items))
}
