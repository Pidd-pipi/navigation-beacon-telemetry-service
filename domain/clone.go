package domain

import "time"

// cloneTime 深拷贝一个 *time.Time，避免多个实体共享可变时间指针。
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

// Clone 返回航标灯实体的深拷贝。
func (b *Beacon) Clone() *Beacon {
	c := *b
	c.LampOffSince = cloneTime(b.LampOffSince)
	c.LowVoltSince = cloneTime(b.LowVoltSince)
	c.LastTelemetryAt = cloneTime(b.LastTelemetryAt)
	return &c
}

// Clone 返回遥测数据实体的深拷贝。
// 灯灭遥测允许 measured_pattern 为空，此时不克隆指针，避免空指针解引用。
func (t *TelemetryData) Clone() *TelemetryData {
	c := *t
	if t.MeasuredPattern != nil {
		p := *t.MeasuredPattern
		c.MeasuredPattern = &p
	}
	if t.Violations != nil {
		c.Violations = append([]string(nil), t.Violations...)
	}
	return &c
}

// Clone 返回异常实体的深拷贝。
// resolved_at 必须深拷贝，避免多个副本共享同一个 *time.Time 指针而互相串改解决时间。
func (a *LampAbnormality) Clone() *LampAbnormality {
	c := *a
	c.ResolvedAt = cloneTime(a.ResolvedAt)
	return &c
}

// Clone 返回处置任务实体的深拷贝。
func (t *DisposalTask) Clone() *DisposalTask {
	c := *t
	c.AssignedAt = cloneTime(t.AssignedAt)
	c.RepairedAt = cloneTime(t.RepairedAt)
	c.VerifiedAt = cloneTime(t.VerifiedAt)
	c.ClosedAt = cloneTime(t.ClosedAt)
	c.EscalatedAt = cloneTime(t.EscalatedAt)
	return &c
}

// Clone 返回遥控指令实体的深拷贝。
func (c *RemoteCommand) Clone() *RemoteCommand {
	cp := *c
	if c.TargetPattern != nil {
		p := *c.TargetPattern
		cp.TargetPattern = &p
	}
	cp.AckAt = cloneTime(c.AckAt)
	return &cp
}

// Clone 返回审计日志实体的深拷贝。
func (a *AuditLog) Clone() *AuditLog {
	cp := *a
	return &cp
}
