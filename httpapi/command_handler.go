package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/service"
)

// CommandHandler 遥控指令处理器。
type CommandHandler struct {
	commands *service.CommandService
}

// NewCommandHandler 构造遥控指令处理器。
func NewCommandHandler(commands *service.CommandService) *CommandHandler {
	return &CommandHandler{commands: commands}
}

// dispatchInput 指令下发输入。
type dispatchInput struct {
	Type          domain.CommandType  `json:"type"`
	TargetPattern *domain.LampPattern `json:"target_pattern,omitempty"`
}

// List GET /api/commands 遥控记录。
func (h *CommandHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := service.CommandFilter{
		BeaconID: q.Get("beacon_id"),
		Status:   domain.AckStatus(q.Get("status")),
		Type:     domain.CommandType(q.Get("type")),
	}
	page, err := parsePagination(r)
	if err != nil {
		Fail(w, err)
		return
	}
	items := h.commands.List(filter)
	OKPage(w, paginate(items, page), len(items))
}

// Dispatch POST /api/beacons/{id}/command 遥控指令下发。
func (h *CommandHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var in dispatchInput
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	if in.Type == "" {
		Fail(w, domain.Validation("type 不能为空"))
		return
	}
	ct, err := domain.ParseCommandType(string(in.Type))
	if err != nil {
		Fail(w, domain.Validation("%v", err))
		return
	}
	cmd, err := h.commands.Dispatch(id, ct, in.TargetPattern, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, err)
		return
	}
	Created(w, cmd)
}

// Ack POST /api/commands/{id}/ack 终端回执。
func (h *CommandHandler) Ack(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		invalidBody(w, err)
		return
	}
	cmd, err := h.commands.Ack(pathID(r), in.Success, in.Message, operatorOf(r), time.Now())
	if err != nil {
		Fail(w, fmt.Errorf("指令 %s 不存在", pathID(r)))
		return
	}
	OK(w, cmd)
}

// Get GET /api/commands/{id} 指令详情。
func (h *CommandHandler) Get(w http.ResponseWriter, r *http.Request) {
	cmd, err := h.commands.Get(pathID(r))
	if err != nil {
		Fail(w, fmt.Errorf("指令 %s 不存在", pathID(r)))
		return
	}
	OK(w, cmd)
}
