package httpapi

import (
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/service"
)

// TelemetryHandler 遥测采集与查询处理器。
type TelemetryHandler struct {
	telemetry *service.TelemetryService
}

// NewTelemetryHandler 构造遥测处理器。
func NewTelemetryHandler(telemetry *service.TelemetryService) *TelemetryHandler {
	return &TelemetryHandler{telemetry: telemetry}
}

// Report POST /api/beacons/{id}/telemetry 遥测上报（触发完整诊断链路）。
func (h *TelemetryHandler) Report(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in service.TelemetryInput
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	result, err := h.telemetry.Process(id, in, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	Created(w, result)
}

// List GET /api/beacons/{id}/telemetry 遥测趋势（最新在前），支持 limit/offset 分页。
func (h *TelemetryHandler) List(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	items, err := h.telemetry.ListTelemetry(id, 0) // 0 表示取全部，分页在 handler 层统一完成
	if err != nil {
		Fail(w, err)
		return
	}
	OKPage(w, paginate(items, page), len(items))
}
