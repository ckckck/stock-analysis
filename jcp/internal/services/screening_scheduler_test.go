package services

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestScreeningSchedulerDisabledDoesNotSchedule(t *testing.T) {
	configService, err := NewConfigService(t.TempDir())
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}

	config := configService.GetConfig()
	config.Screening.AutoSyncEnabled = false
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	syncer := &fakeScreeningSchedulerSyncer{}
	scheduler := NewScreeningScheduler(configService, syncer)

	var afterCalls int
	scheduler.after = func(time.Duration) <-chan time.Time {
		afterCalls++
		return make(chan time.Time)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Start(ctx)
	defer scheduler.Stop()

	time.Sleep(20 * time.Millisecond)

	if afterCalls != 0 {
		t.Fatalf("afterCalls = %d, want 0", afterCalls)
	}
	if got := syncer.CallCount(); got != 0 {
		t.Fatalf("Sync() call count = %d, want 0", got)
	}
}

func TestScreeningSchedulerStopsAfterContextDone(t *testing.T) {
	configService, err := NewConfigService(t.TempDir())
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}

	config := configService.GetConfig()
	config.Screening.AutoSyncEnabled = true
	config.Screening.AutoSyncTime = "18:00"
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	syncer := &fakeScreeningSchedulerSyncer{}
	scheduler := NewScreeningScheduler(configService, syncer)
	scheduler.now = func() time.Time {
		return time.Date(2026, 3, 19, 17, 50, 0, 0, time.Local)
	}

	timerCh := make(chan time.Time, 2)
	scheduler.after = func(time.Duration) <-chan time.Time {
		return timerCh
	}

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	defer scheduler.Stop()

	timerCh <- time.Now()
	syncer.WaitForCalls(t, 1)

	cancel()
	time.Sleep(20 * time.Millisecond)
	timerCh <- time.Now()
	time.Sleep(20 * time.Millisecond)

	if got := syncer.CallCount(); got != 1 {
		t.Fatalf("Sync() call count after cancel = %d, want 1", got)
	}
}

func TestScreeningSchedulerNextRunUsesConfiguredLocalTime(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)

	now := time.Date(2026, 3, 19, 17, 30, 0, 0, loc)
	nextRun, err := nextScreeningSyncRun(now, "18:00", loc)
	if err != nil {
		t.Fatalf("nextScreeningSyncRun() error = %v", err)
	}
	if want := time.Date(2026, 3, 19, 18, 0, 0, 0, loc); !nextRun.Equal(want) {
		t.Fatalf("nextRun = %v, want %v", nextRun, want)
	}

	now = time.Date(2026, 3, 19, 18, 1, 0, 0, loc)
	nextRun, err = nextScreeningSyncRun(now, "18:00", loc)
	if err != nil {
		t.Fatalf("nextScreeningSyncRun() error = %v", err)
	}
	if want := time.Date(2026, 3, 20, 18, 0, 0, 0, loc); !nextRun.Equal(want) {
		t.Fatalf("nextRun = %v, want %v", nextRun, want)
	}
}

type fakeScreeningSchedulerSyncer struct {
	mu    sync.Mutex
	calls int
	done  chan struct{}
}

func (f *fakeScreeningSchedulerSyncer) Sync() (*ScreeningSyncStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	if f.done != nil {
		select {
		case f.done <- struct{}{}:
		default:
		}
	}

	return &ScreeningSyncStatus{}, nil
}

func (f *fakeScreeningSchedulerSyncer) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeScreeningSchedulerSyncer) WaitForCalls(t *testing.T, want int) {
	t.Helper()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if f.CallCount() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("Sync() call count = %d, want at least %d", f.CallCount(), want)
}
