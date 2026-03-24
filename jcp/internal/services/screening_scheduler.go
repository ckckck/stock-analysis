package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
)

var screeningSchedulerLog = logger.New("screening.scheduler")

type screeningSchedulerSyncer interface {
	Sync() (*ScreeningSyncStatus, error)
}

// ScreeningScheduler 在应用运行期间按配置触发 AI 筛选数据同步。
type ScreeningScheduler struct {
	configService *ConfigService
	syncer        screeningSchedulerSyncer
	now           func() time.Time
	after         func(time.Duration) <-chan time.Time
	location      *time.Location

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewScreeningScheduler(
	configService *ConfigService,
	syncer screeningSchedulerSyncer,
) *ScreeningScheduler {
	return &ScreeningScheduler{
		configService: configService,
		syncer:        syncer,
		now:           time.Now,
		after:         time.After,
		location:      time.Local,
	}
}

// Start 根据当前配置启动自动同步调度。
func (s *ScreeningScheduler) Start(ctx context.Context) {
	s.reload(ctx)
}

// Refresh 在配置变更后重新应用调度设置。
func (s *ScreeningScheduler) Refresh(ctx context.Context) {
	s.reload(ctx)
}

// Stop 停止应用运行期内的自动同步调度。
func (s *ScreeningScheduler) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *ScreeningScheduler) reload(ctx context.Context) {
	s.Stop()

	if s == nil || ctx == nil || s.configService == nil || s.syncer == nil {
		return
	}

	cfg := s.configService.GetConfig().Screening
	if !cfg.AutoSyncEnabled {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	go s.loop(runCtx)
}

func (s *ScreeningScheduler) loop(ctx context.Context) {
	for {
		cfg := s.configService.GetConfig().Screening
		if !cfg.AutoSyncEnabled {
			return
		}

		now := s.now()
		nextRun, err := nextScreeningSyncRun(now, cfg.AutoSyncTime, s.location)
		if err != nil {
			screeningSchedulerLog.Warn("invalid screening auto sync time %q: %v", cfg.AutoSyncTime, err)
			nextRun, _ = nextScreeningSyncRun(now, "18:00", s.location)
		}

		wait := nextRun.Sub(now.In(s.location))
		if wait < 0 {
			wait = 0
		}

		select {
		case <-ctx.Done():
			return
		case <-s.after(wait):
			if ctx.Err() != nil {
				return
			}
			if _, err := s.syncer.Sync(); err != nil {
				screeningSchedulerLog.Warn("screening auto sync failed: %v", err)
			}
		}
	}
}

func nextScreeningSyncRun(now time.Time, rawClock string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}

	hour, minute, err := parseScreeningSyncClock(rawClock)
	if err != nil {
		return time.Time{}, err
	}

	localNow := now.In(loc)
	nextRun := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		hour,
		minute,
		0,
		0,
		loc,
	)
	if !nextRun.After(localNow) {
		nextRun = nextRun.AddDate(0, 0, 1)
	}
	return nextRun, nil
}

func parseScreeningSyncClock(rawClock string) (int, int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(rawClock))
	if err != nil {
		return 0, 0, fmt.Errorf("parse screening sync clock: %w", err)
	}
	return parsed.Hour(), parsed.Minute(), nil
}
