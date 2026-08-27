package httpapi

import (
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/service"
)

// AbnormalityHandler 异常台账处理器。
type AbnormalityHandler struct {
	abnormals *service.AbnormalityService
}

// NewAbnormalityHandler 构造异常处理器。
func NewAbnormalityHandler(abnormals *service.AbnormalityService) *AbnormalityHandler {
	return &AbnormalityHandler{abnormals: abnormals}
}

// createAbnormalityInput 手工登记异常输入。
type createAbnormalityInput struct {
	BeaconID string                 `json:"beacon_id"`
	Type     domain.AbnormalityType `json:"type"`
	Detail   string                 `json:"detail"`
}

// List GET /api/abnormalities 异常台账。
func (h *AbnormalityHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := service.AbnormalityFilter{
		BeaconID: q.Get("beacon_id"),
		Status:   domain.AbnormalityStatus(q.Get("status")),
		Type:     domain.AbnormalityType(q.Get("type")),
	}
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	items := h.abnormals.List(filter)
	OKPage(w, paginate(items, page), len(items))
}

// Create POST /api/abnormalities 手工登记异常（灭灯异常自动生成处置任务）。
func (h *AbnormalityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in createAbnormalityInput
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	if in.BeaconID == "" {
		Fail(w, domain.Validation("beacon_id 不能为空"))
		return
	}
	if in.Type == "" {
		Fail(w, domain.Validation("type 不能为空"))
		return
	}
	at, err := domain.ParseAbnormalityType(string(in.Type))
	if err != nil {
		Fail(w, domain.Validation("%v", err))
		return
	}
	ab, err := h.abnormals.CreateManual(in.BeaconID, at, in.Detail, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	Created(w, ab)
}

// Get GET /api/abnormalities/{id} 异常详情。
func (h *AbnormalityHandler) Get(w http.ResponseWriter, r *http.Request) {
	ab, err := h.abnormals.Get(pathID(r))
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, ab)
}

// Resolve POST /api/abnormalities/{id}/resolve 解决异常。
func (h *AbnormalityHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	ab, err := h.abnormals.Resolve(pathID(r), in.Reason, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, ab)
}
