// 航标灯遥测遥控服务（navigation-beacon-telemetry-service）
//
// 基于 Go 1.23 实现的航标灯遥测 Web 项目：航标灯台账、遥测数据采集、
// 灯质异常诊断、遥控指令下发与巡检工单闭环。前端通过 go:embed 内嵌，
// 离线可跑，无外部依赖。
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/httpapi"
	"example.com/navigation-beacon-telemetry-service/service"
	"example.com/navigation-beacon-telemetry-service/store"
)

//go:embed all:web
var webFS embed.FS

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("配置校验失败", "error", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)

	// ---- 仓储（内存 + JSON 原子持久化） ----
	st := store.New(cfg.DataFile)
	store.SetDefaultRetention(cfg.MaxTelemetryPerBeacon)
	if err := st.Load(); err != nil {
		var corrupt *store.CorruptedDataError
		if errors.As(err, &corrupt) {
			logger.Warn("持久化文件损坏，已备份并降级为空库启动",
				"path", corrupt.Path, "backup", corrupt.Backup, "error", corrupt.Cause)
		} else {
			logger.Error("加载持久化数据失败", "error", err)
			os.Exit(1)
		}
	}

	// ---- 服务层 ----
	auditSvc := service.NewAuditService(st, cfg)
	taskSvc := service.NewTaskService(st, auditSvc, cfg)
	abnSvc := service.NewAbnormalityService(st, taskSvc, auditSvc, cfg)
	cmdSvc := service.NewCommandService(st, auditSvc, cfg)
	telSvc := service.NewTelemetryService(st, abnSvc, auditSvc, cfg)
	ovSvc := service.NewOverviewService(st, cfg)

	// ---- 演示基线数据 ----
	if err := service.SeedIfEmpty(st, cfg, telSvc); err != nil {
		logger.Error("初始化演示数据失败", "error", err)
		os.Exit(1)
	}

	// ---- 嵌入式前端 ----
	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		logger.Error("加载前端资源失败", "error", err)
		os.Exit(1)
	}

	// ---- HTTP 层 ----
	handler := httpapi.NewRouter(httpapi.Deps{
		Cfg:                cfg,
		Store:              st,
		Slog:               logger,
		WebFS:              webRoot,
		BeaconHandler:      httpapi.NewBeaconHandler(st, abnSvc, taskSvc, telSvc, auditSvc, cfg.OfflineAfter),
		TelemetryHandler:   httpapi.NewTelemetryHandler(telSvc),
		AbnormalityHandler: httpapi.NewAbnormalityHandler(abnSvc),
		TaskHandler:        httpapi.NewTaskHandler(taskSvc),
		CommandHandler:     httpapi.NewCommandHandler(cmdSvc),
		OverviewHandler:    httpapi.NewOverviewHandler(ovSvc),
		HealthHandler:      httpapi.NewHealthHandler(),
		AuditHandler:       httpapi.NewAuditHandler(auditSvc),
	})

	readTimeout, writeTimeout, idleTimeout := httpapi.ServerTimeout(cfg)
	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: readTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// ---- 灭灯超时扫描定时任务 ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	sweeper := service.NewSweeperSlog(cmdSvc, taskSvc, cfg, logger)
	sweeper.Start(ctx, &wg)

	// ---- 启动 HTTP 服务 ----
	errCh := make(chan error, 1)
	go func() {
		logger.Info("航标灯遥测遥控服务已启动",
			"addr", cfg.Addr(),
			"data_file", cfg.DataFile,
			"config", cfg.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// ---- 优雅退出 ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		logger.Error("HTTP 服务异常退出", "error", err)
		cancel()
		wg.Wait()
		os.Exit(1)
	case sig := <-quit:
		logger.Info("收到退出信号，开始优雅关闭", "signal", sig.String())
	}

	cancel()
	wg.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP 服务关闭异常", "error", err)
	}
	logger.Info("服务已关闭")
}

// newLogger 依据 LOG_LEVEL 构造 JSON 结构化日志。
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
