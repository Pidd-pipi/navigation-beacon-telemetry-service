package domain

import "time"

// BeaconSummary 航标灯状态卡片摘要。
type BeaconSummary struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Type            BeaconType   `json:"type"`
	Status          BeaconStatus `json:"status"`
	LowPower        bool         `json:"low_power"`
	Drifting        bool         `json:"drifting"`
	LampOut         bool         `json:"lamp_out"`
	Voltage         float64      `json:"voltage,omitempty"`
	LampState       LampState    `json:"lamp_state,omitempty"`
	LastTelemetryAt *time.Time   `json:"last_telemetry_at,omitempty"`
}

// Overview 总览聚合数据，供 GET /api/overview 与首页渲染。
type Overview struct {
	Beacons struct {
		Total     int                `json:"total"`
		ByType    map[BeaconType]int `json:"by_type"`
		Active    int                `json:"active"`
		Offline   int                `json:"offline"`
		LowPower  int                `json:"low_power"`
		Drifting  int                `json:"drifting"`
		LampOut   int                `json:"lamp_out"`
		Summaries []BeaconSummary    `json:"summaries"`
	} `json:"beacons"`
	Abnormalities struct {
		Open   int                     `json:"open"`
		ByType map[AbnormalityType]int `json:"by_type"`
	} `json:"abnormalities"`
	Tasks struct {
		Open     int                `json:"open"`
		Overdue  int                `json:"overdue"`
		ByStatus map[TaskStatus]int `json:"by_status"`
	} `json:"tasks"`
	Commands struct {
		Total   int `json:"total"`
		Pending int `json:"pending"`
		Failed  int `json:"failed"`
		Today   int `json:"today"`
	} `json:"commands"`
	Telemetry struct {
		Total          int        `json:"total"`
		LastReceivedAt *time.Time `json:"last_received_at,omitempty"`
	} `json:"telemetry"`
	RecentCommands []RemoteCommand `json:"recent_commands"`
	RecentAudits   []AuditLog      `json:"recent_audits"`
}
