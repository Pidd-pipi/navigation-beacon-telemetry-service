package service

import (
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/domain"
)

func TestTelemetryLampMismatch(t *testing.T) {
	st, _, tel, _, _, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}
	badPattern := &domain.LampPattern{FlashSec: 4.0, EclipseSec: 4.0} // 偏差 2s > 0.5s

	res, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, anchor, badPattern, &now), "tester", now)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(res.Violations) == 0 {
		t.Fatal("灯质偏差应产生违规项")
	}
	if len(res.AffectedAbnormalities) == 0 {
		t.Fatal("灯质偏差应生成异常记录")
	}
	ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampMismatch)
	if ab == nil {
		t.Fatal("应存在未解决灯质偏差异常")
	}

	// 实测灯质恢复设定值 → 自动解决
	goodPattern := &domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	now2 := now.Add(15 * time.Minute)
	_, err = tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, anchor, goodPattern, &now2), "tester", now2)
	if err != nil {
		t.Fatalf("Process2: %v", err)
	}
	if ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampMismatch); ab != nil {
		t.Fatal("灯质恢复后灯质偏差异常应被解决")
	}
}

func TestTelemetryLampOutCreatesTask(t *testing.T) {
	st, _, tel, _, tasks, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}

	// 第一次灭灯上报（开始计时）
	t1 := now.Add(-40 * time.Minute)
	_, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t1), "tester", now)
	if err != nil {
		t.Fatalf("Process off#1: %v", err)
	}
	if ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampOut); ab != nil {
		t.Fatal("首次灭灯未达 30 分钟不应判定灭灯故障")
	}

	// 第二次灭灯上报（累计 40 分钟 > 30 分钟）→ 灭灯故障 + 自动任务
	t2 := now.Add(-5 * time.Minute)
	res, err := tel.Process("B-001", telemetryInput(domain.LampStateOff, 12.0, anchor, nil, &t2), "tester", now)
	if err != nil {
		t.Fatalf("Process off#2: %v", err)
	}
	if len(res.Violations) == 0 {
		t.Fatal("灭灯超过 30 分钟应产生违规项")
	}
	ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampOut)
	if ab == nil {
		t.Fatal("应存在灭灯故障异常")
	}
	// 通过异常 ID 找到自动生成的任务
	found := false
	for _, tk := range tasks.List(TaskFilter{}) {
		if tk.AbnormalityID == ab.ID {
			found = true
			if tk.Status != domain.TaskStatusCreated {
				t.Errorf("自动任务初始状态应为 created，got %s", tk.Status)
			}
			if tk.Deadline.Sub(now) < 3*time.Hour {
				t.Errorf("灭灯任务派发期限应约 4 小时，got %v", tk.Deadline.Sub(now))
			}
		}
	}
	if !found {
		t.Fatal("灭灯故障应自动生成处置任务")
	}

	// 灯恢复 → 灭灯异常解决
	t3 := now.Add(1 * time.Minute)
	goodPattern := &domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	_, err = tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, anchor, goodPattern, &t3), "tester", now)
	if err != nil {
		t.Fatalf("Process on: %v", err)
	}
	if ab := st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLampOut); ab != nil {
		t.Fatal("灯恢复后灭灯异常应被解决")
	}
}

func TestTelemetryLowVoltageAndRecovery(t *testing.T) {
	st, _, tel, _, _, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}
	pattern := &domain.LampPattern{FlashSec: 2, EclipseSec: 2}

	// 低电压 10.2V < 10.5V
	res, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 10.2, anchor, pattern, &now), "tester", now)
	if err != nil {
		t.Fatalf("Process low: %v", err)
	}
	if res.SuggestedPeriod != "30m" {
		t.Errorf("低电时应建议降低遥测频率，got %q", res.SuggestedPeriod)
	}
	if b := st.GetBeacon("B-001"); !b.LowPower {
		t.Error("低电后航标 LowPower 应为 true")
	}
	if st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLowVoltage) == nil {
		t.Error("应存在低电压异常")
	}

	// 滞回区间：10.8V（≥阈值但 <恢复阈值 11.0V）应维持低电
	now2 := now.Add(15 * time.Minute)
	_, err = tel.Process("B-001", telemetryInput(domain.LampStateOn, 10.8, anchor, pattern, &now2), "tester", now2)
	if err != nil {
		t.Fatalf("Process hysteresis: %v", err)
	}
	if b := st.GetBeacon("B-001"); !b.LowPower {
		t.Error("滞回区间应维持 LowPower=true")
	}

	// 恢复 11.5V ≥ 11.0V → 解除
	now3 := now.Add(30 * time.Minute)
	_, err = tel.Process("B-001", telemetryInput(domain.LampStateOn, 11.5, anchor, pattern, &now3), "tester", now3)
	if err != nil {
		t.Fatalf("Process recovery: %v", err)
	}
	if b := st.GetBeacon("B-001"); b.LowPower {
		t.Error("电压恢复后 LowPower 应为 false")
	}
	if st.FindOpenAbnormality("B-001", domain.AbnormalityTypeLowVoltage) != nil {
		t.Error("电压恢复后低电压异常应被解决")
	}
}

func TestTelemetryDriftAndRecovery(t *testing.T) {
	st, _, tel, _, _, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}
	pattern := &domain.LampPattern{FlashSec: 2, EclipseSec: 2}
	farPos := domain.Position{Lat: 30.502, Lng: 122.1} // 约 222 米 > 50 米

	res, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, farPos, pattern, &now), "tester", now)
	if err != nil {
		t.Fatalf("Process drift: %v", err)
	}
	if res.DriftDistanceM <= 50 {
		t.Errorf("漂移距离应 > 50 米，got %.1f", res.DriftDistanceM)
	}
	if !st.GetBeacon("B-001").Drifting {
		t.Error("漂移后航标 Drifting 应为 true")
	}
	if st.FindOpenAbnormality("B-001", domain.AbnormalityTypeDrift) == nil {
		t.Error("应存在漂移异常")
	}

	// 回到锚位 → 解除
	now2 := now.Add(15 * time.Minute)
	_, err = tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, anchor, pattern, &now2), "tester", now2)
	if err != nil {
		t.Fatalf("Process back: %v", err)
	}
	if st.GetBeacon("B-001").Drifting {
		t.Error("回位后 Drifting 应为 false")
	}
	if st.FindOpenAbnormality("B-001", domain.AbnormalityTypeDrift) != nil {
		t.Error("回位后漂移异常应被解决")
	}
}

func TestTelemetryValidationErrors(t *testing.T) {
	st, _, tel, _, _, _, _ := newTestServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	anchor := domain.Position{Lat: 30.5, Lng: 122.1}

	// 不存在的航标
	if _, err := tel.Process("B-999", telemetryInput(domain.LampStateOn, 12.3, anchor, &domain.LampPattern{FlashSec: 2, EclipseSec: 2}, &now), "t", now); err == nil {
		t.Error("不存在的航标应报错")
	}
	// 灯亮缺实测灯质
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, anchor, nil, &now), "t", now); err == nil {
		t.Error("灯亮缺实测灯质应报错")
	}
	// 电压非法
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, -1, anchor, &domain.LampPattern{FlashSec: 2, EclipseSec: 2}, &now), "t", now); err == nil {
		t.Error("负电压应报错")
	}
	// 位置非法
	if _, err := tel.Process("B-001", telemetryInput(domain.LampStateOn, 12.3, domain.Position{Lat: 999, Lng: 0}, &domain.LampPattern{FlashSec: 2, EclipseSec: 2}, &now), "t", now); err == nil {
		t.Error("非法坐标应报错")
	}
}
