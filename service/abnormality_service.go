package service

import (
	"fmt"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// AbnormalityService 异常诊断服务：开关/触达/解决异常记录。
type AbnormalityService struct {
	store *store.Store
	tasks *TaskService
	audit *AuditService
	cfg   *config.Config
}

// NewAbnormalityService 构造异常诊断服务。
func NewAbnormalityService(st *store.Store, tasks *TaskService, audit *AuditService, cfg *config.Config) *AbnormalityService {
	return &AbnormalityService{store: st, tasks: tasks, audit: audit, cfg: cfg}
}

// OpenFromTelemetry 依据遥测诊断结果打开/刷新异常记录。
// 灭灯异常（lamp_out）首次打开时自动生成处置任务。
func (s *AbnormalityService) OpenFromTelemetry(beacon *domain.Beacon, at domain.AbnormalityType, detail string, now time.Time) (*domain.LampAbnormality, error) {
	existing := s.store.FindOpenAbnormality(beacon.ID, at)
	if existing != nil {
		updated := cloneAbnormality(existing)
		updated.Touch(now)
		if err := s.store.UpsertAbnormality(updated); err != nil {
			return nil, err
		}
		return updated, nil
	}

	ab := domain.NewLampAbnormality(s.store.NextID("A"), beacon.ID, at, detail, now)
	if err := s.store.UpsertAbnormality(ab); err != nil {
		return nil, domain.Internal("保存异常失败: %v", err)
	}
	s.audit.LogAt(now, "abnormality.open", "abnormality", ab.ID, "system",
		fmt.Sprintf("航标 %s 触发 %s 异常: %s", beacon.ID, at.Label(), detail))

	// 灭灯异常自动生成处置任务
	if at == domain.AbnormalityTypeLampOut {
		if _, err := s.tasks.CreateForAbnormality(ab, beacon, now); err != nil {
			return nil, err
		}
	}
	return ab, nil
}

// CreateManual 手工登记异常（POST /api/abnormalities）。
// 灭灯异常同时自动生成处置任务。
func (s *AbnormalityService) CreateManual(beaconID string, at domain.AbnormalityType, detail, operator string, now time.Time) (*domain.LampAbnormality, error) {
	beacon := s.store.GetBeacon(beaconID)
	if beacon == nil {
		return nil, domain.NotFound("航标 %s 不存在", beaconID)
	}
	if !at.Valid() {
		return nil, domain.Validation("无效的异常类型 %q", at)
	}
	ab := domain.NewLampAbnormality(s.store.NextID("A"), beaconID, at, detail, now)
	if err := s.store.UpsertAbnormality(ab); err != nil {
		return nil, domain.Internal("保存异常失败: %v", err)
	}
	s.audit.LogAt(now, "abnormality.open", "abnormality", ab.ID, operator,
		fmt.Sprintf("手工登记 %s 异常: %s", at.Label(), detail))
	if at == domain.AbnormalityTypeLampOut {
		if _, err := s.tasks.CreateForAbnormality(ab, beacon, now); err != nil {
			return nil, err
		}
	}
	return ab, nil
}

// Resolve 解决异常。
func (s *AbnormalityService) Resolve(id, reason, operator string, now time.Time) (*domain.LampAbnormality, error) {
	ab := s.store.GetAbnormality(id)
	if ab == nil {
		return nil, domain.NotFound("异常 %s 不存在", id)
	}
	if !ab.IsOpen() {
		return nil, domain.Conflict("异常 %s 已解决", id)
	}
	updated := cloneAbnormality(ab)
	updated.Resolve(reason, now)
	if err := s.store.UpsertAbnormality(updated); err != nil {
		return nil, domain.Internal("保存异常失败: %v", err)
	}
	s.audit.LogAt(now, "abnormality.resolve", "abnormality", id, operator, reason)
	return updated, nil
}

// AutoResolve 依据遥测诊断结果解决指定类型异常（若存在未解决记录）。
func (s *AbnormalityService) AutoResolve(beaconID string, at domain.AbnormalityType, reason, operator string, now time.Time) {
	existing := s.store.FindOpenAbnormality(beaconID, at)
	if existing == nil {
		return
	}
	updated := cloneAbnormality(existing)
	updated.Resolve(reason, now)
	_ = s.store.UpsertAbnormality(updated)
	s.audit.LogAt(now, "abnormality.resolve", "abnormality", updated.ID, operator, reason)
}

// AbnormalityFilter 异常查询过滤条件。
type AbnormalityFilter struct {
	Type     domain.AbnormalityType // 空串表示不限
	Status   domain.AbnormalityStatus
	BeaconID string
}

// List 按过滤条件查询异常（按最后发现时间倒序）。
func (s *AbnormalityService) List(filter AbnormalityFilter) []*domain.LampAbnormality {
	all := s.store.ListAbnormalities()
	out := make([]*domain.LampAbnormality, 0, len(all))
	for _, a := range all {
		if filter.Type != "" && a.Type != filter.Type {
			continue
		}
		if filter.Status != "" && a.Status != filter.Status {
			continue
		}
		if filter.BeaconID != "" && a.BeaconID != filter.BeaconID {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Get 查询单个异常。
func (s *AbnormalityService) Get(id string) (*domain.LampAbnormality, error) {
	a := s.store.GetAbnormality(id)
	if a == nil {
		return nil, domain.NotFound("异常 %s 不存在", id)
	}
	return a, nil
}

// CountOpen 统计未解决异常数。
func (s *AbnormalityService) CountOpen() int { return s.store.CountOpenAbnormalities() }

func cloneAbnormality(a *domain.LampAbnormality) *domain.LampAbnormality {
	return a.Clone()
}

// TouchOpen 刷新某航标某类型未解决异常的最后发现时间（异常持续存在）。
func (s *AbnormalityService) TouchOpen(beaconID string, at domain.AbnormalityType, now time.Time) {
	existing := s.store.FindOpenAbnormality(beaconID, at)
	if existing == nil {
		return
	}
	updated := cloneAbnormality(existing)
	updated.Touch(now)
	_ = s.store.UpsertAbnormality(updated)
}
