package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/service"
	"example.com/navigation-beacon-telemetry-service/store"
)

// BeaconHandler 航标台账处理器。
type BeaconHandler struct {
	store        *store.Store
	abnormals    *service.AbnormalityService
	tasks        *service.TaskService
	telemetry    *service.TelemetryService
	audit        *service.AuditService
	offlineAfter time.Duration
}

// NewBeaconHandler 构造航标处理器。
func NewBeaconHandler(st *store.Store, abnormals *service.AbnormalityService,
	tasks *service.TaskService, telemetry *service.TelemetryService,
	audit *service.AuditService, offlineAfter time.Duration) *BeaconHandler {
	return &BeaconHandler{
		store: st, abnormals: abnormals, tasks: tasks, telemetry: telemetry,
		audit: audit, offlineAfter: offlineAfter,
	}
}

// createBeaconInput 新建航标输入。
type createBeaconInput struct {
	Name         string             `json:"name"`
	Type         domain.BeaconType  `json:"type"`
	Anchor       domain.Position    `json:"anchor"`
	DriftRadiusM float64            `json:"drift_radius_m"`
	LampPattern  domain.LampPattern `json:"lamp_pattern"`
}

// beaconDetail 航标详情视图（含派生状态与关联数据）。
type beaconDetail struct {
	Beacon            *domain.Beacon            `json:"beacon"`
	EffectiveStatus   domain.BeaconStatus       `json:"effective_status"`
	OpenAbnormalities []*domain.LampAbnormality `json:"open_abnormalities"`
	OpenTasks         []*domain.DisposalTask    `json:"open_tasks"`
	LastTelemetry     *domain.TelemetryData     `json:"last_telemetry"`
	TelemetryCount    int                       `json:"telemetry_count"`
}

// List GET /api/beacons 返回航标列表，支持 limit/offset 分页。
func (h *BeaconHandler) List(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	beacons := h.store.ListBeacons()
	OKPage(w, paginate(beacons, page), len(beacons))
}

// Create POST /api/beacons 新建航标。
func (h *BeaconHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in createBeaconInput
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	if in.Type == "" {
		Fail(w, domain.Validation("type 不能为空"))
		return
	}
	if in.DriftRadiusM < 0 {
		Fail(w, domain.Validation("drift_radius_m 不能为负"))
		return
	}
	if in.DriftRadiusM == 0 {
		in.DriftRadiusM = h.driftRadiusDefault()
	}
	now := time.Now()
	beacon := domain.NewBeacon(h.store.NextID("B"), in.Name, in.Type, in.Anchor, in.DriftRadiusM, in.LampPattern, now)
	if err := beacon.Validate(); err != nil {
		Fail(w, domain.Validation("航标参数非法: %v", err))
		return
	}
	if err := h.store.CreateBeacon(beacon); err != nil {
		Fail(w, err)
		return
	}
	h.audit.Log("beacon.created", "beacon", beacon.ID, operatorOf(r), "新建航标 "+beacon.Name)
	Created(w, beacon)
}

// Get GET /api/beacons/{id} 返回航标详情。
func (h *BeaconHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	b := h.store.GetBeacon(id)
	if b == nil {
		Fail(w, fmt.Errorf("航标 %s 不存在", id))
		return
	}
	now := time.Now()
	detail := beaconDetail{
		Beacon:            b,
		EffectiveStatus:   b.EffectiveStatus(now, h.offlineAfter),
		OpenAbnormalities: h.abnormals.List(service.AbnormalityFilter{BeaconID: id, Status: domain.AbnormalityStatusOpen}),
		OpenTasks:         h.tasks.List(service.TaskFilter{BeaconID: id}),
		LastTelemetry:     h.store.LastTelemetry(id),
		TelemetryCount:    h.store.CountTelemetryByBeacon(id),
	}
	OK(w, detail)
}

// driftRadiusDefault 默认漂移半径。
func (h *BeaconHandler) driftRadiusDefault() float64 { return 50.0 }

// operatorOf 从请求头读取操作人，缺省 anonymous。
func operatorOf(r *http.Request) string {
	if op := r.Header.Get("X-Operator"); op != "" {
		return op
	}
	return "anonymous"
}
