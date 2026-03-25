package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

func TestScreeningSyncFirstSyncUsesConfiguredWindow(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 3
	config.Screening.RetentionMode = models.ScreeningRetentionModeForever
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", Industry: "银行", ListDate: "1991-04-03", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-14", []float64{10, 11, 12, 13, 14}),
			"sz000001": makeDailyBars("2026-03-14", []float64{20, 21, 22, 23, 24}),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 14, Change: 1, ChangePercent: 7.6, Volume: 1000, Amount: 100000, Open: 13, High: 14, Low: 12.5, PreClose: 13},
			"sz000001": {Symbol: "sz000001", Name: "平安银行", Price: 24, Change: 1, ChangePercent: 4.3, Volume: 2000, Amount: 200000, Open: 23, High: 24, Low: 22.5, PreClose: 23},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if status.LastTradeDate != "2026-03-18" {
		t.Fatalf("status.LastTradeDate = %q, want %q", status.LastTradeDate, "2026-03-18")
	}
	if status.StocksSynced != 2 || status.BarsSynced != 6 || status.SnapshotsSynced != 6 {
		t.Fatalf("status = %#v", status)
	}
	if got := source.lastLookbackDays["sh600000"]; got != 3 {
		t.Fatalf("lookbackDays for sh600000 = %d, want 3", got)
	}

	assertTableCount(t, store, "stocks_basic", 2)
	assertTableCount(t, store, "daily_bars", 6)
	assertTableCount(t, store, "daily_snapshots", 6)
	assertLastTradeDate(t, store, screeningSyncStateDataset, "shanghai,shenzhen", "2026-03-18")
}

func TestScreeningSyncIncrementalSyncUsesSyncState(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 2
	config.Screening.RetentionMode = models.ScreeningRetentionModeForever
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", Industry: "银行", ListDate: "1991-04-03", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 11, Change: 1, ChangePercent: 10, Volume: 1000, Amount: 100000, Open: 10, High: 11, Low: 9.8, PreClose: 10},
			"sz000001": {Symbol: "sz000001", Name: "平安银行", Price: 21, Change: 1, ChangePercent: 5, Volume: 2000, Amount: 200000, Open: 20, High: 21, Low: 19.8, PreClose: 20},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 17, 16, 0, 0, 0, time.UTC)
	}
	if _, err := svc.Sync(); err != nil {
		t.Fatalf("first Sync() error = %v", err)
	}

	source.bars["sh600000"] = makeDailyBars("2026-03-16", []float64{10, 11, 12})
	source.bars["sz000001"] = makeDailyBars("2026-03-16", []float64{20, 21, 22})
	delete(source.snapshots, "sh600000")
	source.snapshots["sz000001"] = models.Stock{Symbol: "sz000001", Name: "平安银行", Price: 22, Change: 1, ChangePercent: 4.8, Volume: 2200, Amount: 220000, Open: 21, High: 22, Low: 20.8, PreClose: 21}
	svc.now = func() time.Time {
		return time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.Sync()
	if err != nil {
		t.Fatalf("second Sync() error = %v", err)
	}

	if status.LastTradeDate != "2026-03-18" {
		t.Fatalf("status.LastTradeDate = %q, want %q", status.LastTradeDate, "2026-03-18")
	}
	if status.BarsSynced != 2 {
		t.Fatalf("status.BarsSynced = %d, want 2", status.BarsSynced)
	}
	if got := source.lastLookbackDays["sh600000"]; got <= 2 {
		t.Fatalf("incremental lookbackDays = %d, want > 2", got)
	}

	assertTableCount(t, store, "daily_bars", 6)
	assertSnapshotChange(t, store, "sh600000", "2026-03-18", 1)
	assertLastTradeDate(t, store, screeningSyncStateDataset, "shanghai,shenzhen", "2026-03-18")
}

func TestScreeningSyncRetentionCleanupKeepsConfiguredDays(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 40
	config.Screening.RetentionMode = models.ScreeningRetentionModeDays
	config.Screening.RetentionDays = 30
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-02-01", makeCloseValues(40, 10)),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 49, Change: 1, ChangePercent: 2.1, Volume: 1000, Amount: 100000, Open: 48, High: 49, Low: 47.5, PreClose: 48},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 12, 16, 0, 0, 0, time.UTC)
	}

	if _, err := svc.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertTradeDateBounds(t, store, "daily_bars", "2026-02-10", "2026-03-12")
	assertTradeDateBounds(t, store, "daily_snapshots", "2026-02-10", "2026-03-12")
}

func TestScreeningSyncRetentionCleanupKeepsConfiguredSixtyDays(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 71
	config.Screening.RetentionMode = models.ScreeningRetentionModeDays
	config.Screening.RetentionDays = 60
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-01-01", makeCloseValues(71, 10)),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 80, Change: 1, ChangePercent: 1.2, Volume: 1000, Amount: 100000, Open: 79, High: 80, Low: 78.5, PreClose: 79},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 12, 16, 0, 0, 0, time.UTC)
	}

	if _, err := svc.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertTradeDateBounds(t, store, "daily_bars", "2026-01-11", "2026-03-12")
	assertTradeDateBounds(t, store, "daily_snapshots", "2026-01-11", "2026-03-12")
}

func TestScreeningSyncManualLimitOnlyProcessesConfiguredCount(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
			{Symbol: "sz000002", Name: "万科A", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
			"sz000002": makeDailyBars("2026-03-16", []float64{30, 31}),
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode:        ScreeningSyncModeManual,
		LimitStocks: 2,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}

	if status.StocksSynced != 2 {
		t.Fatalf("status.StocksSynced = %d, want 2", status.StocksSynced)
	}
	if source.dailyBarCalls["sz000002"] != 0 {
		t.Fatalf("sz000002 daily bar calls = %d, want 0", source.dailyBarCalls["sz000002"])
	}
}

func TestScreeningSyncGetStatusIncludesMarketCoverageSummary(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
			{Symbol: "sz000002", Name: "万科A", Market: "深圳", IsActive: true},
		},
		tradeDates: []string{"2026-03-20"},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 21, 8, 30, 45, 0, time.UTC)
	}

	for _, statement := range []string{
		`INSERT INTO stocks_basic (symbol, name, market, is_active) VALUES
			('sh600000', '浦发银行', '上海', 1),
			('sz000001', '平安银行', '深圳', 1),
			('sz000002', '万科A', '深圳', 1)`,
		`INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount) VALUES
			('sh600000', '2026-03-20', 10, 10.5, 9.8, 10.2, 1000, 10000),
			('sz000001', '2026-03-20', 20, 20.8, 19.9, 20.5, 2000, 20000),
			('sz000002', '2026-03-19', 30, 30.5, 29.8, 30.1, 3000, 30000)`,
	} {
		if _, err := store.db.Exec(statement); err != nil {
			t.Fatalf("seed summary fixture error = %v", err)
		}
	}

	if err := store.UpsertSyncState(ScreeningSyncState{
		Dataset:       screeningSyncStateDataset,
		MarketScope:   "shanghai,shenzhen",
		LastTradeDate: "2026-03-19",
		UpdatedAt:     time.Date(2026, 3, 20, 19, 45, 22, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertSyncState() error = %v", err)
	}

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.TargetTradeDate != "2026-03-20" {
		t.Fatalf("status.TargetTradeDate = %q, want %q", status.TargetTradeDate, "2026-03-20")
	}
	if status.LatestSyncedTradeDate != "2026-03-20" {
		t.Fatalf("status.LatestSyncedTradeDate = %q, want %q", status.LatestSyncedTradeDate, "2026-03-20")
	}
	if status.MarketStockCount != 3 {
		t.Fatalf("status.MarketStockCount = %d, want 3", status.MarketStockCount)
	}
	if status.SyncedToLatestStocks != 2 {
		t.Fatalf("status.SyncedToLatestStocks = %d, want 2", status.SyncedToLatestStocks)
	}
	if status.PendingSyncStocks != 1 {
		t.Fatalf("status.PendingSyncStocks = %d, want 1", status.PendingSyncStocks)
	}
	if status.LastSyncedAt != "2026-03-20T19:45:22Z" {
		t.Fatalf("status.LastSyncedAt = %q, want %q", status.LastSyncedAt, "2026-03-20T19:45:22Z")
	}
}

func TestScreeningSyncResolveTargetTradeDateUsesPreviousTradeDateWhenTodayIsTradingDay(t *testing.T) {
	svc := &ScreeningSyncService{
		source: &fakeScreeningMarketSource{
			tradeDates: []string{"2026-03-24", "2026-03-23"},
		},
		now: func() time.Time {
			return time.Date(2026, 3, 24, 15, 0, 0, 0, time.UTC)
		},
	}

	targetTradeDate, err := svc.resolveTargetTradeDate("")
	if err != nil {
		t.Fatalf("resolveTargetTradeDate() error = %v", err)
	}
	if targetTradeDate != "2026-03-23" {
		t.Fatalf("targetTradeDate = %q, want 2026-03-23", targetTradeDate)
	}
}

func TestScreeningSyncResolveTargetTradeDateKeepsLatestClosedTradeDateWhenTodayNotIncluded(t *testing.T) {
	svc := &ScreeningSyncService{
		source: &fakeScreeningMarketSource{
			tradeDates: []string{"2026-03-23", "2026-03-20"},
		},
		now: func() time.Time {
			return time.Date(2026, 3, 24, 9, 0, 0, 0, time.UTC)
		},
	}

	targetTradeDate, err := svc.resolveTargetTradeDate("")
	if err != nil {
		t.Fatalf("resolveTargetTradeDate() error = %v", err)
	}
	if targetTradeDate != "2026-03-23" {
		t.Fatalf("targetTradeDate = %q, want 2026-03-23", targetTradeDate)
	}
}

func TestScreeningSyncCanceledRunSavesCheckpointAndResumeState(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	ctx, cancel := context.WithCancel(context.Background())
	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
			{Symbol: "sz000002", Name: "万科A", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
			"sz000002": makeDailyBars("2026-03-16", []float64{30, 31}),
		},
		afterGetDailyBars: func(symbol string) {
			if symbol == "sh600000" {
				cancel()
			}
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(ctx, ScreeningSyncRunOptions{
		Mode:        ScreeningSyncModeManual,
		LimitStocks: 3,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}
	if status.RunStatus != string(ScreeningSyncRunStatusCanceled) {
		t.Fatalf("status.RunStatus = %q, want %q", status.RunStatus, ScreeningSyncRunStatusCanceled)
	}

	job, err := store.GetSyncJobState(screeningSyncStateDataset, "shanghai,shenzhen")
	if err != nil {
		t.Fatalf("GetSyncJobState() error = %v", err)
	}
	if job == nil {
		t.Fatal("GetSyncJobState() = nil, want checkpoint")
	}
	if job.CurrentIndex != 1 || job.CompletedStocks != 1 {
		t.Fatalf("job = %#v, want completed/current index 1", job)
	}
	if job.LastCompletedSymbol != "sh600000" {
		t.Fatalf("job.LastCompletedSymbol = %q, want sh600000", job.LastCompletedSymbol)
	}
}

func TestScreeningSyncResumesFromCheckpointForManualAndAuto(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
			{Symbol: "sz000002", Name: "万科A", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
			"sz000002": makeDailyBars("2026-03-16", []float64{30, 31}),
		},
	}

	if err := store.UpsertSyncJobState(ScreeningSyncJobState{
		Dataset:             screeningSyncStateDataset,
		MarketScope:         "shanghai,shenzhen",
		Status:              string(ScreeningSyncRunStatusCanceled),
		Mode:                string(ScreeningSyncModeManual),
		LimitStocks:         10,
		TotalStocks:         10,
		CompletedStocks:     1,
		CurrentIndex:        1,
		LastCompletedSymbol: "sh600000",
		UpdatedAt:           time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertSyncJobState() error = %v", err)
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode:        ScreeningSyncModeAuto,
		LimitStocks: 0,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}
	if status.StocksSynced != 3 {
		t.Fatalf("status.StocksSynced = %d, want 3", status.StocksSynced)
	}
	if source.dailyBarCalls["sh600000"] != 0 {
		t.Fatalf("sh600000 daily bar calls = %d, want 0 after resume", source.dailyBarCalls["sh600000"])
	}
	if source.dailyBarCalls["sz000001"] == 0 || source.dailyBarCalls["sz000002"] == 0 {
		t.Fatalf("resume calls = %#v, want remaining stocks to be processed", source.dailyBarCalls)
	}
}

func TestScreeningSyncFreshRunDoesNotReuseStaleCheckpointProgress(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	if err := store.UpsertSyncJobState(ScreeningSyncJobState{
		Dataset:             screeningSyncStateDataset,
		MarketScope:         "shanghai,shenzhen",
		Status:              string(ScreeningSyncRunStatusCanceled),
		Mode:                string(ScreeningSyncModeManual),
		LimitStocks:         0,
		TotalStocks:         5151,
		CompletedStocks:     23,
		CurrentIndex:        23,
		LastCompletedSymbol: "sz000031",
		UpdatedAt:           time.Date(2026, 3, 24, 5, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertSyncJobState() error = %v", err)
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
		},
		beforeGetDailyBars: func(symbol string) {
			if symbol != "sh600000" {
				return
			}
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 24, 16, 0, 0, 0, time.UTC)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
			Mode: ScreeningSyncModeManual,
		}, nil)
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not reach first symbol in time")
	}

	job, err := store.GetSyncJobState(screeningSyncStateDataset, "shanghai,shenzhen")
	if err != nil {
		t.Fatalf("GetSyncJobState() error = %v", err)
	}
	if job == nil {
		t.Fatal("GetSyncJobState() = nil")
	}
	if job.Status != string(ScreeningSyncRunStatusRunning) {
		t.Fatalf("job.Status = %q, want running", job.Status)
	}
	if job.CompletedStocks != 0 || job.CurrentIndex != 0 {
		t.Fatalf("job progress = completed:%d current:%d, want 0/0 for fresh run", job.CompletedStocks, job.CurrentIndex)
	}
	if job.TotalStocks != 2 {
		t.Fatalf("job.TotalStocks = %d, want 2", job.TotalStocks)
	}

	close(release)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SyncWithOptions() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not finish in time")
	}
}

func TestScreeningSyncGetStatusNormalizesStaleRunningJob(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	if err := store.UpsertSyncJobState(ScreeningSyncJobState{
		Dataset:             screeningSyncStateDataset,
		MarketScope:         "shanghai,shenzhen",
		Status:              string(ScreeningSyncRunStatusRunning),
		Mode:                string(ScreeningSyncModeManual),
		LimitStocks:         0,
		TotalStocks:         5151,
		CompletedStocks:     6,
		CurrentIndex:        6,
		LastCompletedSymbol: "sz000008",
		LastMessage:         "completed 6 / 5151",
		UpdatedAt:           time.Date(2026, 3, 24, 5, 53, 54, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertSyncJobState() error = %v", err)
	}

	svc := NewScreeningSyncService(configService, store, &fakeScreeningMarketSource{})
	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.RunStatus != string(ScreeningSyncRunStatusCanceled) {
		t.Fatalf("status.RunStatus = %q, want canceled", status.RunStatus)
	}
	if status.CompletedStocks != 6 || status.TotalStocks != 5151 {
		t.Fatalf("status progress = %#v, want checkpoint progress retained", status)
	}

	job, err := store.GetSyncJobState(screeningSyncStateDataset, "shanghai,shenzhen")
	if err != nil {
		t.Fatalf("GetSyncJobState() error = %v", err)
	}
	if job == nil {
		t.Fatal("GetSyncJobState() = nil")
	}
	if job.Status != string(ScreeningSyncRunStatusCanceled) {
		t.Fatalf("job.Status = %q, want canceled", job.Status)
	}
}

func TestScreeningSyncReportsSourceFallbackInProgress(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
		},
		sourceEvents: map[string][]ScreeningDailyBarSourceEvent{
			"sh600000": {
				{Source: "baostock", Status: "start", Message: "使用 Baostock"},
				{Source: "baostock", Status: "error", Message: "Baostock 超时"},
				{Source: "sina", Status: "switch", Message: "切换到 Sina"},
				{Source: "sina", Status: "success", Message: "Sina 成功"},
			},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	}

	var progress []ScreeningSyncProgress
	_, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, func(update ScreeningSyncProgress) {
		progress = append(progress, update)
	})
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}
	if len(progress) == 0 {
		t.Fatalf("progress updates = 0, want > 0")
	}
	last := progress[len(progress)-1]
	if last.ActiveSource != "sina" {
		t.Fatalf("last.ActiveSource = %q, want sina", last.ActiveSource)
	}
	if len(last.Events) == 0 {
		t.Fatalf("last.Events = %#v, want source history", last.Events)
	}
}

func TestScreeningSyncReportsDiagnosticEventWhenStockFetchFails(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600804", Name: "鹏博士", Market: "上海", IsActive: true},
		},
		bars: map[string][]models.KLineData{},
		tradeDates: []string{"2026-03-25"},
		dailyBarErrors: map[string]error{
			"sh600804": fmt.Errorf("kline api status 456 for sh600804: blocked"),
		},
	}

	if err := seedDailyBar(t, store, "sh600804", "2026-03-24", 11); err != nil {
		t.Fatalf("seedDailyBar sh600804 error = %v", err)
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 25, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, nil)
	if err == nil {
		t.Fatal("SyncWithOptions() error = nil, want fetch failure")
	}
	if status == nil {
		t.Fatal("status = nil, want failure status")
	}
	if len(status.Events) == 0 {
		t.Fatalf("status.Events = %#v, want diagnostic failure events", status.Events)
	}

	last := status.Events[len(status.Events)-1]
	if last.Status != "error" {
		t.Fatalf("last.Status = %q, want error", last.Status)
	}
	if !strings.Contains(last.Message, "sh600804") || !strings.Contains(last.Message, "lookback=") || !strings.Contains(last.Message, "target=") {
		t.Fatalf("last.Message = %q, want symbol/lookback/target diagnostics", last.Message)
	}
}

func TestScreeningSyncGetStatusIncludesLastProgressSnapshot(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	svc := NewScreeningSyncService(configService, store, &fakeScreeningMarketSource{})
	svc.setLastProgress(ScreeningSyncProgress{
		MarketScope:          "shanghai,shenzhen",
		Mode:                 string(ScreeningSyncModeManual),
		RunStatus:            string(ScreeningSyncRunStatusRunning),
		ProgressPercent:      50,
		TotalStocks:          10,
		CompletedStocks:      5,
		CurrentSymbol:        "sz000001",
		CurrentName:          "平安银行",
		CurrentStage:         "syncing",
		ActiveSource:         "baostock",
		LastMessage:          "切换到 Baostock",
		LimitStocks:          10,
		ResumeFromCheckpoint: true,
		Events: []ScreeningSyncEvent{
			{
				Time:    "2026-03-19T10:00:00Z",
				Symbol:  "sz000001",
				Name:    "平安银行",
				Source:  "sina",
				Status:  "error",
				Message: "Sina 接口异常",
			},
			{
				Time:    "2026-03-19T10:00:02Z",
				Symbol:  "sz000001",
				Name:    "平安银行",
				Source:  "baostock",
				Status:  "switch",
				Message: "切换到 Baostock",
			},
		},
	})

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.RunStatus != string(ScreeningSyncRunStatusRunning) {
		t.Fatalf("status.RunStatus = %q, want running", status.RunStatus)
	}
	if status.ProgressPercent != 50 || status.CompletedStocks != 5 || status.TotalStocks != 10 {
		t.Fatalf("status progress = %#v, want last progress snapshot", status)
	}
	if status.CurrentSymbol != "sz000001" || status.ActiveSource != "baostock" {
		t.Fatalf("status current/source = %#v, want last progress fields", status)
	}
	if !status.ResumeFromCheckpoint {
		t.Fatalf("status.ResumeFromCheckpoint = false, want true")
	}
	if len(status.Events) != 2 {
		t.Fatalf("status.Events = %#v, want copied event history", status.Events)
	}
}

func TestScreeningSyncStatusIncludesStoredDataCountsAndSyncedSymbols(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-18", []float64{10}),
			"sz000001": makeDailyBars("2026-03-18", []float64{20}),
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode:        ScreeningSyncModeManual,
		LimitStocks: 2,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}

	if status.StoredStocks != 2 || status.StoredBars != 2 {
		t.Fatalf("stored counts = stocks:%d bars:%d, want 2/2", status.StoredStocks, status.StoredBars)
	}
	if len(status.SyncedSymbols) != 2 || status.SyncedSymbols[0] != "sh600000" || status.SyncedSymbols[1] != "sz000001" {
		t.Fatalf("status.SyncedSymbols = %#v, want synced subset symbols", status.SyncedSymbols)
	}

	loadedStatus, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if loadedStatus.StoredStocks != 2 || loadedStatus.StoredBars != 2 {
		t.Fatalf("loaded stored counts = stocks:%d bars:%d, want 2/2", loadedStatus.StoredStocks, loadedStatus.StoredBars)
	}
}

func TestScreeningSyncSkipsAlreadySyncedStocksForTargetTradeDate(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-18", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-18", []float64{20, 21}),
		},
		tradeDates: []string{"2026-03-19"},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	if _, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, nil); err != nil {
		t.Fatalf("first SyncWithOptions() error = %v", err)
	}

	for symbol := range source.dailyBarCalls {
		source.dailyBarCalls[symbol] = 0
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, nil)
	if err != nil {
		t.Fatalf("second SyncWithOptions() error = %v", err)
	}

	if source.dailyBarCalls["sh600000"] != 0 || source.dailyBarCalls["sz000001"] != 0 {
		t.Fatalf("daily bar calls = %#v, want all zero when already synced", source.dailyBarCalls)
	}
	if status.TotalStocks != 0 || status.CompletedStocks != 0 {
		t.Fatalf("status progress = %#v, want empty sync queue", status)
	}
	if status.ProgressPercent != 100 {
		t.Fatalf("status.ProgressPercent = %v, want 100 for empty queue", status.ProgressPercent)
	}
}

func TestScreeningSyncOnlyProcessesSymbolsMissingTargetTradeDate(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
			{Symbol: "sz000002", Name: "万科A", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-18", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-18", []float64{20, 21}),
			"sz000002": makeDailyBars("2026-03-18", []float64{30, 31}),
		},
		tradeDates: []string{"2026-03-19"},
	}

	if err := seedDailyBar(t, store, "sh600000", "2026-03-19", 11); err != nil {
		t.Fatalf("seedDailyBar sh600000 error = %v", err)
	}
	if err := seedDailyBar(t, store, "sz000001", "2026-03-19", 21); err != nil {
		t.Fatalf("seedDailyBar sz000001 error = %v", err)
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}

	if source.dailyBarCalls["sh600000"] != 0 || source.dailyBarCalls["sz000001"] != 0 {
		t.Fatalf("already synced symbols should be skipped, calls = %#v", source.dailyBarCalls)
	}
	if source.dailyBarCalls["sz000002"] != 1 {
		t.Fatalf("sz000002 daily bar calls = %d, want 1", source.dailyBarCalls["sz000002"])
	}
	if status.TotalStocks != 1 || status.CompletedStocks != 1 {
		t.Fatalf("status progress = %#v, want only one missing symbol in queue", status)
	}
}

func TestScreeningSyncBackfillsLaggingSymbolDespiteGlobalSyncState(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-17", []float64{10, 11, 12}),
			"sz000001": makeDailyBars("2026-03-17", []float64{20, 21, 22}),
		},
		tradeDates: []string{"2026-03-19"},
	}

	if err := seedDailyBar(t, store, "sh600000", "2026-03-17", 10); err != nil {
		t.Fatalf("seedDailyBar sh600000 error = %v", err)
	}
	if err := seedDailyBar(t, store, "sh600000", "2026-03-18", 11); err != nil {
		t.Fatalf("seedDailyBar sh600000 error = %v", err)
	}
	if err := seedDailyBar(t, store, "sh600000", "2026-03-19", 12); err != nil {
		t.Fatalf("seedDailyBar sh600000 error = %v", err)
	}
	if err := seedDailyBar(t, store, "sz000001", "2026-03-17", 20); err != nil {
		t.Fatalf("seedDailyBar sz000001 error = %v", err)
	}
	if err := store.UpsertSyncState(ScreeningSyncState{
		Dataset:       screeningSyncStateDataset,
		MarketScope:   "shanghai,shenzhen",
		LastTradeDate: "2026-03-19",
		UpdatedAt:     time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("UpsertSyncState() error = %v", err)
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeManual,
	}, nil)
	if err != nil {
		t.Fatalf("SyncWithOptions() error = %v", err)
	}

	if source.dailyBarCalls["sh600000"] != 0 {
		t.Fatalf("sh600000 daily bar calls = %d, want 0", source.dailyBarCalls["sh600000"])
	}
	if source.dailyBarCalls["sz000001"] != 1 {
		t.Fatalf("sz000001 daily bar calls = %d, want 1", source.dailyBarCalls["sz000001"])
	}
	if status.BarsSynced != 2 {
		t.Fatalf("status.BarsSynced = %d, want 2 for lagging symbol backfill", status.BarsSynced)
	}
	assertTableCount(t, store, "daily_bars", 6)
}

type fakeScreeningMarketSource struct {
	stocks             []ScreeningStockBasic
	bars               map[string][]models.KLineData
	snapshots          map[string]models.Stock
	tradeDates         []string
	lastLookbackDays   map[string]int
	dailyBarCalls      map[string]int
	sourceEvents       map[string][]ScreeningDailyBarSourceEvent
	dailyBarErrors     map[string]error
	beforeGetDailyBars func(symbol string)
	afterGetDailyBars  func(symbol string)
}

func (f *fakeScreeningMarketSource) ListScreeningStocks(_ models.ScreeningMarketScopeConfig) ([]ScreeningStockBasic, error) {
	return append([]ScreeningStockBasic(nil), f.stocks...), nil
}

func (f *fakeScreeningMarketSource) GetScreeningDailyBars(symbol string, lookbackDays int) ([]models.KLineData, error) {
	if f.lastLookbackDays == nil {
		f.lastLookbackDays = make(map[string]int)
	}
	if f.dailyBarCalls == nil {
		f.dailyBarCalls = make(map[string]int)
	}
	f.lastLookbackDays[symbol] = lookbackDays
	f.dailyBarCalls[symbol]++

	if f.beforeGetDailyBars != nil {
		f.beforeGetDailyBars(symbol)
	}
	if f.afterGetDailyBars != nil {
		defer f.afterGetDailyBars(symbol)
	}
	if err := f.dailyBarErrors[symbol]; err != nil {
		return nil, err
	}

	allBars := append([]models.KLineData(nil), f.bars[symbol]...)
	if lookbackDays >= len(allBars) {
		return allBars, nil
	}
	return allBars[len(allBars)-lookbackDays:], nil
}

func (f *fakeScreeningMarketSource) GetScreeningDailyBarsWithObserver(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	if observer != nil {
		for _, event := range f.sourceEvents[symbol] {
			observer(event)
		}
	}
	return f.GetScreeningDailyBars(symbol, lookbackDays)
}

func (f *fakeScreeningMarketSource) GetScreeningSnapshots(symbols []string) ([]models.Stock, error) {
	results := make([]models.Stock, 0, len(symbols))
	for _, symbol := range symbols {
		if snapshot, ok := f.snapshots[symbol]; ok {
			results = append(results, snapshot)
		}
	}
	return results, nil
}

func (f *fakeScreeningMarketSource) GetTradeDates(days int) ([]string, error) {
	if len(f.tradeDates) == 0 {
		return nil, nil
	}
	if days <= 0 || days >= len(f.tradeDates) {
		return append([]string(nil), f.tradeDates...), nil
	}
	return append([]string(nil), f.tradeDates[:days]...), nil
}

func newScreeningSyncTestServices(t *testing.T) (*ConfigService, *ScreeningStore) {
	t.Helper()

	tempDir := t.TempDir()
	configService, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}
	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return configService, store
}

func makeDailyBars(startDate string, closes []float64) []models.KLineData {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		panic(err)
	}

	bars := make([]models.KLineData, 0, len(closes))
	for i, closePrice := range closes {
		day := start.AddDate(0, 0, i)
		bars = append(bars, models.KLineData{
			Time:   day.Format("2006-01-02"),
			Open:   closePrice - 0.5,
			High:   closePrice + 0.5,
			Low:    closePrice - 1,
			Close:  closePrice,
			Volume: int64(1000 + i*100),
			Amount: float64(10000 + i*1000),
		})
	}
	return bars
}

func makeCloseValues(count int, start float64) []float64 {
	values := make([]float64, count)
	for i := range values {
		values[i] = start + float64(i)
	}
	return values
}

func assertTableCount(t *testing.T, store *ScreeningStore, tableName string, want int) {
	t.Helper()

	var got int
	if err := store.db.QueryRow(`SELECT COUNT(1) FROM ` + tableName).Scan(&got); err != nil {
		t.Fatalf("count %s error = %v", tableName, err)
	}
	if got != want {
		t.Fatalf("count(%s) = %d, want %d", tableName, got, want)
	}
}

func assertLastTradeDate(t *testing.T, store *ScreeningStore, dataset, scope, want string) {
	t.Helper()

	state, err := store.GetSyncState(dataset, scope)
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if state == nil {
		t.Fatalf("GetSyncState() = nil, want %q", want)
	}
	if state.LastTradeDate != want {
		t.Fatalf("LastTradeDate = %q, want %q", state.LastTradeDate, want)
	}
}

func assertTradeDateBounds(t *testing.T, store *ScreeningStore, tableName, wantMin, wantMax string) {
	t.Helper()

	var minDate string
	var maxDate string
	if err := store.db.QueryRow(`SELECT MIN(trade_date), MAX(trade_date) FROM `+tableName).Scan(&minDate, &maxDate); err != nil {
		t.Fatalf("trade date bounds for %s error = %v", tableName, err)
	}
	if minDate != wantMin || maxDate != wantMax {
		t.Fatalf("%s bounds = [%s, %s], want [%s, %s]", tableName, minDate, maxDate, wantMin, wantMax)
	}
}

func assertSnapshotChange(t *testing.T, store *ScreeningStore, symbol, tradeDate string, want float64) {
	t.Helper()

	var got float64
	if err := store.db.QueryRow(
		`SELECT change FROM daily_snapshots WHERE symbol = ? AND trade_date = ?`,
		symbol,
		tradeDate,
	).Scan(&got); err != nil {
		t.Fatalf("snapshot change for %s %s error = %v", symbol, tradeDate, err)
	}
	if got != want {
		t.Fatalf("snapshot change for %s %s = %v, want %v", symbol, tradeDate, got, want)
	}
}

func seedDailyBar(t *testing.T, store *ScreeningStore, symbol, tradeDate string, closePrice float64) error {
	t.Helper()

	_, err := store.db.Exec(
		`INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(symbol, trade_date) DO UPDATE SET
			open = excluded.open,
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			volume = excluded.volume,
			amount = excluded.amount`,
		symbol,
		tradeDate,
		closePrice-0.5,
		closePrice+0.5,
		closePrice-1,
		closePrice,
		1000,
		10000,
	)
	return err
}
