package httpapi

import (
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/service"
)

// TaskHandler 处置任务处理器。
type TaskHandler struct {
	tasks *service.TaskService
}

// NewTaskHandler 构造处置任务处理器。
func NewTaskHandler(tasks *service.TaskService) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

// List GET /api/tasks 任务列表。
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := service.TaskFilter{
		BeaconID: q.Get("beacon_id"),
		Status:   domain.TaskStatus(q.Get("status")),
		Level:    domain.TaskLevel(q.Get("level")),
	}
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	items := h.tasks.List(filter)
	OKPage(w, paginate(items, page), len(items))
}

// Create POST /api/tasks 为异常生成处置任务。
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AbnormalityID string `json:"abnormality_id"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	if in.AbnormalityID == "" {
		Fail(w, domain.Validation("abnormality_id 不能为空"))
		return
	}
	task, err := h.tasks.CreateManual(in.AbnormalityID, time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	Created(w, task)
}

// Assign POST /api/tasks/{id}/assign 派发任务。
func (h *TaskHandler) Assign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Assignee string `json:"assignee"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	task, err := h.tasks.Assign(pathID(r), in.Assignee, time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, task)
}

// Repair POST /api/tasks/{id}/repair 修复任务。
func (h *TaskHandler) Repair(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	task, err := h.tasks.Repair(pathID(r), in.Note, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, task)
}

// Verify POST /api/tasks/{id}/verify 复测并关闭（repaired→verified→closed）。
func (h *TaskHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Result    string `json:"result"`
		AutoClose *bool  `json:"auto_close"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	autoClose := true
	if in.AutoClose != nil {
		autoClose = *in.AutoClose
	}
	task, err := h.tasks.Verify(pathID(r), in.Result, autoClose, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, task)
}

// Close POST /api/tasks/{id}/close 关闭任务。
func (h *TaskHandler) Close(w http.ResponseWriter, r *http.Request) {
	task, err := h.tasks.Close(pathID(r), operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, task)
}

// Escalate POST /api/tasks/{id}/escalate 升级任务。
func (h *TaskHandler) Escalate(w http.ResponseWriter, r *http.Request) {
	task, err := h.tasks.Escalate(pathID(r), operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	OK(w, task)
}
