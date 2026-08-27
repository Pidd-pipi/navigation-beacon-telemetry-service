// Package store 实现内存仓储 + JSON 文件持久化。
//
// 数据全部保存在内存 map 中，任何变更后原子写盘（临时文件 + fsync + rename），
// 无外部服务依赖，可重复构建；DATA_FILE 为空串时仅内存运行。
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

// Store 内存仓储根对象，持有全部实体的索引。
type Store struct {
	mu            sync.RWMutex
	path          string
	beacons       map[string]*domain.Beacon
	telemetry     map[string][]*domain.TelemetryData // beaconID → 按上报时间升序
	abnormalities map[string]*domain.LampAbnormality
	tasks         map[string]*domain.DisposalTask
	commands      map[string]*domain.RemoteCommand
	audits        []*domain.AuditLog
	seq           map[string]uint64
}

// snapshot 持久化快照结构。
type snapshot struct {
	Beacons       map[string]*domain.Beacon          `json:"beacons"`
	Telemetry     map[string][]*domain.TelemetryData `json:"telemetry"`
	Abnormalities map[string]*domain.LampAbnormality `json:"abnormalities"`
	Tasks         map[string]*domain.DisposalTask    `json:"tasks"`
	Commands      map[string]*domain.RemoteCommand   `json:"commands"`
	Audits        []*domain.AuditLog                 `json:"audits"`
	Seq           map[string]uint64                  `json:"seq"`
}

// CorruptedDataError 表示持久化文件已损坏并已备份到 Backup 路径。
type CorruptedDataError struct {
	Path   string
	Backup string
	Cause  error
}

// Error 实现 error 接口。
func (e *CorruptedDataError) Error() string {
	return fmt.Sprintf("持久化文件 %s 损坏，已备份到 %s: %v", e.Path, e.Backup, e.Cause)
}

// Unwrap 暴露底层解析错误。
func (e *CorruptedDataError) Unwrap() error { return e.Cause }

// New 构造空仓储。path 为空串表示不落盘。
func New(path string) *Store {
	return &Store{
		path:          path,
		beacons:       make(map[string]*domain.Beacon),
		telemetry:     make(map[string][]*domain.TelemetryData),
		abnormalities: make(map[string]*domain.LampAbnormality),
		tasks:         make(map[string]*domain.DisposalTask),
		commands:      make(map[string]*domain.RemoteCommand),
		audits:        make([]*domain.AuditLog, 0, 64),
		seq:           make(map[string]uint64),
	}
}

// NextID 生成指定前缀的递增 ID，如 B-001、T-002。
func (s *Store) NextID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq[prefix]++
	return fmt.Sprintf("%s-%03d", prefix, s.seq[prefix])
}

// Load 从磁盘加载快照。
//
// 文件不存在时静默返回 nil；文件损坏时备份为 <path>.bak 并返回
// *CorruptedDataError，调用方应告警后以空库降级启动。
func (s *Store) Load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取持久化文件失败: %w", err)
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		backup := s.path + ".bak"
		if werr := os.WriteFile(backup, data, 0o644); werr != nil {
			return &CorruptedDataError{Path: s.path, Backup: backup, Cause: fmt.Errorf("解析失败: %v；备份失败: %w", err, werr)}
		}
		return &CorruptedDataError{Path: s.path, Backup: backup, Cause: err}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 持久化文件中数组/映射字段可能为 null（或字段缺失），此时反序列化得到 nil。
	// 若不归一化为空集合，后续写入（如 s.beacons[id]=b、s.seq[prefix]++）会触发
	// "assignment to entry in nil map" / "index out of range" 运行时 panic。
	s.beacons = orEmptyBeaconMap(snap.Beacons)
	s.telemetry = orEmptyTelemetryMap(snap.Telemetry)
	s.abnormalities = orEmptyAbnormalityMap(snap.Abnormalities)
	s.tasks = orEmptyTaskMap(snap.Tasks)
	s.commands = orEmptyCommandMap(snap.Commands)
	s.audits = orEmptyAuditSlice(snap.Audits)
	s.seq = orEmptySeqMap(snap.Seq)
	return nil
}

// Save 使用写锁串行化快照生成，并通过「临时文件 + fsync + rename」原子落盘。
// 路径为空串时静默跳过。
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 在写锁内完成快照与序列化，避免与 Mutate 并发时读取到半更新状态。
	s.mu.Lock()
	payload, err := json.MarshalIndent(snapshot{
		Beacons:       s.beacons,
		Telemetry:     s.telemetry,
		Abnormalities: s.abnormalities,
		Tasks:         s.tasks,
		Commands:      s.commands,
		Audits:        s.audits,
		Seq:           s.seq,
	}, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".beacon_state-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	// 成功 rename 前，任何异常路径都要清理临时文件，避免数据目录残留 .tmp。
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("原子替换持久化文件失败: %w", err)
	}
	renamed = true

	// 同步目录项，确保 rename 结果在掉电后仍然可见。
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// Persist 暴露内部持久化入口，供测试与显式落盘调用。
func (s *Store) Persist() error { return s.Save() }

// Mutate 在写锁下执行变更并落盘，保证「改完即存」。
func (s *Store) Mutate(fn func()) error {
	s.mu.Lock()
	fn()
	s.mu.Unlock()
	return s.Save()
}

// sortTelemetryDesc 按上报时间倒序排序副本，返回新切片不污染原数据。
func sortTelemetryDesc(items []*domain.TelemetryData) []*domain.TelemetryData {
	out := make([]*domain.TelemetryData, len(items))
	copy(out, items)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ReportedAt.After(out[j].ReportedAt)
	})
	return out
}

// Now 统一时间源，便于测试注入（默认返回真实当前时间）。
var Now = time.Now

// Lock 返回写锁，供需要跨实体原子操作的 service 使用。
func (s *Store) Lock()   { s.mu.Lock() }
func (s *Store) Unlock() { s.mu.Unlock() }

// BumpSeq 将指定前缀的序列推进到至少 count（用于演示数据占位后避免 ID 冲突）。
func (s *Store) BumpSeq(prefix string, count uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seq[prefix] < count {
		s.seq[prefix] = count
	}
}

// orEmptyBeaconMap 确保返回非 nil map，避免反序列化得到 nil 后写入 panic。
func orEmptyBeaconMap(m map[string]*domain.Beacon) map[string]*domain.Beacon {
	if m == nil {
		return make(map[string]*domain.Beacon)
	}
	return m
}

func orEmptyTelemetryMap(m map[string][]*domain.TelemetryData) map[string][]*domain.TelemetryData {
	if m == nil {
		return make(map[string][]*domain.TelemetryData)
	}
	return m
}

func orEmptyAbnormalityMap(m map[string]*domain.LampAbnormality) map[string]*domain.LampAbnormality {
	if m == nil {
		return make(map[string]*domain.LampAbnormality)
	}
	return m
}

func orEmptyTaskMap(m map[string]*domain.DisposalTask) map[string]*domain.DisposalTask {
	if m == nil {
		return make(map[string]*domain.DisposalTask)
	}
	return m
}

func orEmptyCommandMap(m map[string]*domain.RemoteCommand) map[string]*domain.RemoteCommand {
	if m == nil {
		return make(map[string]*domain.RemoteCommand)
	}
	return m
}

func orEmptyAuditSlice(s []*domain.AuditLog) []*domain.AuditLog {
	if s == nil {
		return make([]*domain.AuditLog, 0, 64)
	}
	return s
}

func orEmptySeqMap(m map[string]uint64) map[string]uint64 {
	if m == nil {
		return make(map[string]uint64)
	}
	return m
}
