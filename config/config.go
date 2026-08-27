// Package config 集中管理航标灯遥测遥控服务的运行参数与业务阈值。
//
// 所有阈值（灯质容差、灭灯判定、电压健康、漂移半径、指令超时重发等）
// 均在此定义，并支持通过环境变量覆盖，便于部署与测试。
package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

// Default 返回一份带默认值的配置，随后可通过环境变量覆盖。
func Default() *Config {
	return &Config{
		Port:                "8080",
		DataFile:            "data/beacon_state.json",
		LogLevel:            "info",
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        15 * time.Second,
		IdleTimeout:         60 * time.Second,
		LampToleranceSec:    0.5,              // 灯质校验偏差容差（秒）
		LampOutMinutes:      30,               // 灯连续熄灭超过该分钟数判定灭灯故障
		LowVoltageThreshold: 10.5,             // 12V 电池低电压下限（伏）
		RecoveryVoltage:     11.0,             // 电压恢复阈值（伏），带滞回避免反复抖动
		DriftRadiusM:        50.0,             // 默认漂移判定半径（米）
		CommandAckTimeout:   5 * time.Minute,  // 指令回执超时
		CommandMaxRetries:   2,                // 回执超时自动重发上限
		TaskAssignDeadline:  4 * time.Hour,    // 灭灯任务派发期限
		SweepInterval:       30 * time.Second, // 定时扫描周期
		TaskEscalationScan:  10 * time.Minute, // 灭灯派发超时升级扫描周期
		TelemetryPeriod:     15 * time.Minute, // 遥测上报周期
		OfflineAfter:        45 * time.Minute, // 超过该时长无遥测判定离线
		AuditRetention:      2000,             // 审计日志保留条数
	}
}

// Config 服务配置。
type Config struct {
	Port                string        // HTTP 监听端口，支持 PORT 环境变量覆盖
	DataFile            string        // JSON 持久化文件路径，空串表示不落盘
	LogLevel            string        // 结构化日志级别：debug/info/warn/error
	ReadTimeout         time.Duration // HTTP 读超时（含请求头）
	WriteTimeout        time.Duration // HTTP 写超时
	IdleTimeout         time.Duration // HTTP 空闲连接超时
	LampToleranceSec    float64       // 灯质校验偏差容差（秒）
	LampOutMinutes      int           // 灯连续熄灭判定分钟
	LowVoltageThreshold float64       // 低电压阈值（伏）
	RecoveryVoltage     float64       // 电压恢复阈值（伏）
	DriftRadiusM        float64       // 默认漂移判定半径（米）
	CommandAckTimeout   time.Duration // 指令回执超时时长
	CommandMaxRetries   int           // 回执超时自动重发最大次数
	TaskAssignDeadline  time.Duration // 灭灯任务派发期限
	SweepInterval       time.Duration // 定时扫描周期
	TaskEscalationScan  time.Duration // 灭灯派发超时升级扫描周期
	TelemetryPeriod     time.Duration // 遥测上报周期
	OfflineAfter        time.Duration // 无遥测离线判定时长
	AuditRetention      int           // 审计日志保留条数
}

// Load 读取环境变量并覆盖默认配置。
func Load() *Config {
	cfg := Default()
	cfg.Port = envStr("PORT", cfg.Port)
	cfg.DataFile = envStr("DATA_FILE", cfg.DataFile)
	cfg.LogLevel = envStr("LOG_LEVEL", cfg.LogLevel)
	cfg.ReadTimeout = envDuration("SERVER_READ_TIMEOUT", cfg.ReadTimeout)
	cfg.WriteTimeout = envDuration("SERVER_WRITE_TIMEOUT", cfg.WriteTimeout)
	cfg.IdleTimeout = envDuration("SERVER_IDLE_TIMEOUT", cfg.IdleTimeout)
	cfg.LampToleranceSec = envFloat("LAMP_TOLERANCE_SEC", cfg.LampToleranceSec)
	cfg.LampOutMinutes = envInt("LAMP_OUT_MINUTES", cfg.LampOutMinutes)
	cfg.LowVoltageThreshold = envFloat("LOW_VOLTAGE_THRESHOLD", cfg.LowVoltageThreshold)
	cfg.RecoveryVoltage = envFloat("RECOVERY_VOLTAGE", cfg.RecoveryVoltage)
	cfg.DriftRadiusM = envFloat("DRIFT_RADIUS_M", cfg.DriftRadiusM)
	cfg.CommandAckTimeout = envDuration("COMMAND_ACK_TIMEOUT", cfg.CommandAckTimeout)
	cfg.CommandMaxRetries = envInt("COMMAND_MAX_RETRIES", cfg.CommandMaxRetries)
	cfg.TaskAssignDeadline = envDuration("TASK_ASSIGN_DEADLINE", cfg.TaskAssignDeadline)
	cfg.SweepInterval = envDuration("SWEEP_INTERVAL", cfg.SweepInterval)
	cfg.TaskEscalationScan = envDuration("TASK_ESCALATION_SCAN", cfg.TaskEscalationScan)
	cfg.TelemetryPeriod = envDuration("TELEMETRY_PERIOD", cfg.TelemetryPeriod)
	cfg.OfflineAfter = envDuration("OFFLINE_AFTER", cfg.OfflineAfter)
	cfg.AuditRetention = envInt("AUDIT_RETENTION", cfg.AuditRetention)
	return cfg
}

// Validate 校验配置是否合法，避免无效参数在运行期才暴露。
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT 不能为空")
	}
	if n, err := strconv.Atoi(c.Port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("PORT 必须是 1-65535 的整数")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL 必须是 debug/info/warn/error 之一")
	}
	if c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 {
		return fmt.Errorf("HTTP 超时必须大于 0")
	}
	if math.IsNaN(c.LampToleranceSec) || math.IsInf(c.LampToleranceSec, 0) || c.LampToleranceSec < 0 {
		return fmt.Errorf("LAMP_TOLERANCE_SEC 必须为非负有限数值")
	}
	if c.LampOutMinutes <= 0 {
		return fmt.Errorf("LAMP_OUT_MINUTES 必须大于 0")
	}
	if !validFloat(c.LowVoltageThreshold) || c.LowVoltageThreshold <= 0 {
		return fmt.Errorf("LOW_VOLTAGE_THRESHOLD 必须为大于 0 的有限数值")
	}
	if !validFloat(c.RecoveryVoltage) || c.RecoveryVoltage <= 0 {
		return fmt.Errorf("RECOVERY_VOLTAGE 必须为大于 0 的有限数值")
	}
	if c.RecoveryVoltage < c.LowVoltageThreshold {
		return fmt.Errorf("RECOVERY_VOLTAGE 不能低于 LOW_VOLTAGE_THRESHOLD")
	}
	if !validFloat(c.DriftRadiusM) || c.DriftRadiusM <= 0 {
		return fmt.Errorf("DRIFT_RADIUS_M 必须为大于 0 的有限数值")
	}
	if c.CommandAckTimeout <= 0 || c.TaskAssignDeadline <= 0 || c.SweepInterval <= 0 ||
		c.TaskEscalationScan <= 0 || c.TelemetryPeriod <= 0 || c.OfflineAfter <= 0 {
		return fmt.Errorf("定时与超时参数必须大于 0")
	}
	if c.CommandMaxRetries < 0 {
		return fmt.Errorf("COMMAND_MAX_RETRIES 不能为负")
	}
	if c.AuditRetention < 0 {
		return fmt.Errorf("AUDIT_RETENTION 不能为负")
	}
	return nil
}

func validFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// Addr 返回监听地址。
func (c *Config) Addr() string { return ":" + c.Port }

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// String 打印当前生效的配置摘要，便于启动日志核对。
func (c *Config) String() string {
	return fmt.Sprintf(
		"port=%s data_file=%s log_level=%s read_timeout=%s write_timeout=%s idle_timeout=%s lamp_tolerance=%.2fs lamp_out=%dm low_volt=%.1fV recovery=%.1fV drift=%.0fm ack_timeout=%s max_retries=%d task_deadline=%s sweep=%s escalation=%s",
		c.Port, c.DataFile, c.LogLevel, c.ReadTimeout, c.WriteTimeout, c.IdleTimeout,
		c.LampToleranceSec, c.LampOutMinutes,
		c.LowVoltageThreshold, c.RecoveryVoltage, c.DriftRadiusM,
		c.CommandAckTimeout, c.CommandMaxRetries, c.TaskAssignDeadline,
		c.SweepInterval, c.TaskEscalationScan,
	)
}
