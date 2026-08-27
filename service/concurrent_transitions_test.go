package service

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"example.com/navigation-beacon-telemetry-service/config"
	"example.com/navigation-beacon-telemetry-service/domain"
	"example.com/navigation-beacon-telemetry-service/store"
)

// newPathServices 构造使用真实落盘路径的服务，拉长写路径窗口。
func newPathServices(t *testing.T) (*store.Store, *config.Config, *CommandService, *TaskService, *AbnormalityService) {
	t.Helper()
	cfg := config.Default()
	cfg.DataFile = filepath.Join(t.TempDir(), "state.json")
	st := store.New(cfg.DataFile)
	audit := NewAuditService(st, cfg)
	tasks := NewTaskService(st, audit, cfg)
	abn := NewAbnormalityService(st, tasks, audit, cfg)
	cmd := NewCommandService(st, audit, cfg)
	return st, cfg, cmd, tasks, abn
}

// TestConcurrentDuplicateAckRejected 并发回执同一指令只允许一次成功。
// 测试先持有仓储写锁，让两个回执都先读到 pending，再同时放行，
// 确定性复现「校验与落库分离」的竞态窗口。
func TestConcurrentDuplicateAckRejected(t *testing.T) {
	st, _, commands, _, _ := newPathServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	cmd, err := commands.Dispatch("B-001", domain.CommandTypeOn, nil, "op", now)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// 持有写锁，令两个回执的读路径全部阻塞在读锁上
	st.Lock()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			_, err := commands.Ack(cmd.ID, true, "终端已执行", "terminal", now.Add(time.Second))
			results[g] = err
		}(g)
	}
	close(start)
	time.Sleep(50 * time.Millisecond) // 等两个 goroutine 都阻塞在读锁上
	st.Unlock()                       // 同时放行，两个回执都读到 pending

	wg.Wait()
	okCount := 0
	for _, err := range results {
		if err == nil {
			okCount++
		}
	}
	if okCount > 1 {
		t.Fatalf("并发回执同一指令应只有一个成功，got %d", okCount)
	}
}

// TestConcurrentDuplicateAssignRejected 并发派发同一任务只允许一次成功。
func TestConcurrentDuplicateAssignRejected(t *testing.T) {
	st, _, _, tasks, abn := newPathServices(t)
	seedTestBeacon(t, st, "B-001")

	now := time.Now()
	ab, err := abn.CreateManual("B-001", domain.AbnormalityTypeDrift, "并发漂移", "op", now)
	if err != nil {
		t.Fatalf("CreateManual abnormality: %v", err)
	}
	task, err := tasks.CreateManual(ab.ID, now)
	if err != nil {
		t.Fatalf("CreateManual task: %v", err)
	}

	st.Lock()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			_, err := tasks.Assign(task.ID, fmt.Sprintf("作业员%d", g), now)
			results[g] = err
		}(g)
	}
	close(start)
	time.Sleep(50 * time.Millisecond)
	st.Unlock()

	wg.Wait()
	okCount := 0
	for _, err := range results {
		if err == nil {
			okCount++
		}
	}
	if okCount > 1 {
		t.Fatalf("并发派发同一任务应只有一个成功，got %d", okCount)
	}
}
