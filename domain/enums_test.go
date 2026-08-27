package domain

import "testing"

func TestBeaconTypeValid(t *testing.T) {
	cases := []struct {
		typ  BeaconType
		want bool
	}{
		{BeaconTypeLighthouse, true},
		{BeaconTypeBuoy, true},
		{BeaconTypeDaybeacon, true},
		{BeaconType("rocket"), false},
		{BeaconType(""), false},
	}
	for _, c := range cases {
		if got := c.typ.Valid(); got != c.want {
			t.Errorf("BeaconType(%q).Valid() = %v, want %v", c.typ, got, c.want)
		}
	}
	if BeaconTypeLighthouse.Label() != "灯塔" {
		t.Errorf("灯塔 Label 异常: %s", BeaconTypeLighthouse.Label())
	}
}

func TestAbnormalityTypeValidAndLabel(t *testing.T) {
	valid := []AbnormalityType{AbnormalityTypeLampMismatch, AbnormalityTypeLampOut, AbnormalityTypeLowVoltage, AbnormalityTypeDrift}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("%q 应合法", v)
		}
		if v.Label() == "" {
			t.Errorf("%q Label 为空", v)
		}
	}
	if AbnormalityType("other").Valid() {
		t.Error("other 不应合法")
	}
}

func TestTaskStatusTransitionTable(t *testing.T) {
	// 状态机迁移链：created→assigned→repaired→verified→closed
	flow := []TaskStatus{TaskStatusCreated, TaskStatusAssigned, TaskStatusRepaired, TaskStatusVerified, TaskStatusClosed}
	for i, s := range flow {
		if !s.Valid() {
			t.Fatalf("状态 %q 不合法", s)
		}
		if i+1 < len(flow) {
			if !containsStatus(TaskTransitions[s], flow[i+1]) {
				t.Errorf("迁移表缺少 %s -> %s", s, flow[i+1])
			}
		}
	}
	if TaskStatus("cancelled").Valid() {
		t.Error("cancelled 不应合法")
	}
}

func TestParseHelpers(t *testing.T) {
	if _, err := ParseBeaconType("buoy"); err != nil {
		t.Errorf("ParseBeaconType(buoy) err: %v", err)
	}
	if _, err := ParseBeaconType("bad"); err == nil {
		t.Error("ParseBeaconType(bad) 应报错")
	}
	if _, err := ParseAbnormalityType("drift"); err != nil {
		t.Errorf("ParseAbnormalityType(drift) err: %v", err)
	}
	if _, err := ParseTaskStatus("verified"); err != nil {
		t.Errorf("ParseTaskStatus(verified) err: %v", err)
	}
	if _, err := ParseAckStatus("pending"); err != nil {
		t.Errorf("ParseAckStatus(pending) err: %v", err)
	}
	if _, err := ParseCommandType("switch_pattern"); err != nil {
		t.Errorf("ParseCommandType(switch_pattern) err: %v", err)
	}
}

func containsStatus(list []TaskStatus, s TaskStatus) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
