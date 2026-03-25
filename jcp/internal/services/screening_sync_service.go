package services

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"
)

const screeningSyncStateDataset = "daily_bars"

var screeningSyncLog = logger.New("screening.sync")

// ScreeningStockBasic 描述同步到筛选库的股票基础信息。
type ScreeningStockBasic struct {
	Symbol   string
	Name     string
	Market   string
	Industry string
	ListDate string
	IsST     bool
	IsActive bool
}

// ScreeningSyncStatus 描述最近一次同步的结果或当前同步状态。
type ScreeningSyncStatus struct {
	MarketScope           string               `json:"marketScope"`
	InitialSyncDays       int                  `json:"initialSyncDays"`
	RetentionMode         string               `json:"retentionMode"`
	RetentionDays         int                  `json:"retentionDays"`
	LastTradeDate         string               `json:"lastTradeDate"`
	LastSyncedAt          string               `json:"lastSyncedAt"`
	TargetTradeDate       string               `json:"targetTradeDate,omitempty"`
	LatestSyncedTradeDate string               `json:"latestSyncedTradeDate,omitempty"`
	StocksSynced          int                  `json:"stocksSynced"`
	BarsSynced            int                  `json:"barsSynced"`
	SnapshotsSynced       int                  `json:"snapshotsSynced"`
	StoredStocks          int                  `json:"storedStocks,omitempty"`
	StoredBars            int                  `json:"storedBars,omitempty"`
	StoredSnapshots       int                  `json:"storedSnapshots,omitempty"`
	MarketStockCount      int                  `json:"marketStockCount,omitempty"`
	SyncedToLatestStocks  int                  `json:"syncedToLatestStocks,omitempty"`
	PendingSyncStocks     int                  `json:"pendingSyncStocks,omitempty"`
	RunStatus             string               `json:"runStatus,omitempty"`
	ProgressPercent       float64              `json:"progressPercent,omitempty"`
	TotalStocks           int                  `json:"totalStocks,omitempty"`
	CompletedStocks       int                  `json:"completedStocks,omitempty"`
	CurrentSymbol         string               `json:"currentSymbol,omitempty"`
	CurrentName           string               `json:"currentName,omitempty"`
	CurrentStage          string               `json:"currentStage,omitempty"`
	ActiveSource          string               `json:"activeSource,omitempty"`
	LastMessage           string               `json:"lastMessage,omitempty"`
	LimitStocks           int                  `json:"limitStocks,omitempty"`
	ResumeFromCheckpoint  bool                 `json:"resumeFromCheckpoint,omitempty"`
	SyncedSymbols         []string             `json:"syncedSymbols,omitempty"`
	Events                []ScreeningSyncEvent `json:"events,omitempty"`
	Error                 string               `json:"error,omitempty"`
}

type ScreeningSyncMode string

const (
	ScreeningSyncModeManual ScreeningSyncMode = "manual"
	ScreeningSyncModeAuto   ScreeningSyncMode = "auto"
)

type ScreeningSyncRunStatus string

const (
	ScreeningSyncRunStatusIdle      ScreeningSyncRunStatus = "idle"
	ScreeningSyncRunStatusRunning   ScreeningSyncRunStatus = "running"
	ScreeningSyncRunStatusCanceled  ScreeningSyncRunStatus = "canceled"
	ScreeningSyncRunStatusFailed    ScreeningSyncRunStatus = "failed"
	ScreeningSyncRunStatusCompleted ScreeningSyncRunStatus = "completed"
)

type ScreeningSyncRunOptions struct {
	Mode        ScreeningSyncMode `json:"mode"`
	LimitStocks int               `json:"limitStocks"`
}

type ScreeningSyncEvent struct {
	Time    string `json:"time"`
	Symbol  string `json:"symbol"`
	Name    string `json:"name"`
	Source  string `json:"source"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ScreeningSyncProgress struct {
	MarketScope          string               `json:"marketScope"`
	Mode                 string               `json:"mode"`
	RunStatus            string               `json:"runStatus"`
	ProgressPercent      float64              `json:"progressPercent"`
	TotalStocks          int                  `json:"totalStocks"`
	CompletedStocks      int                  `json:"completedStocks"`
	CurrentSymbol        string               `json:"currentSymbol"`
	CurrentName          string               `json:"currentName"`
	CurrentStage         string               `json:"currentStage"`
	ActiveSource         string               `json:"activeSource"`
	LastMessage          string               `json:"lastMessage"`
	LimitStocks          int                  `json:"limitStocks"`
	ResumeFromCheckpoint bool                 `json:"resumeFromCheckpoint"`
	Events               []ScreeningSyncEvent `json:"events"`
	Error                string               `json:"error,omitempty"`
}

type screeningObservedMarketSource interface {
	GetScreeningDailyBarsWithObserver(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error)
}

type screeningTradeDateSource interface {
	GetTradeDates(days int) ([]string, error)
}

type screeningMarketSource interface {
	ListScreeningStocks(scopes models.ScreeningMarketScopeConfig) ([]ScreeningStockBasic, error)
	GetScreeningDailyBars(symbol string, lookbackDays int) ([]models.KLineData, error)
	GetScreeningSnapshots(symbols []string) ([]models.Stock, error)
}

// ScreeningSyncService 负责手动与增量同步本地筛选数据库。
type ScreeningSyncService struct {
	configService *ConfigService
	store         *ScreeningStore
	source        screeningMarketSource
	now           func() time.Time
	mu            sync.Mutex
	lastProgress  *ScreeningSyncProgress
	runCancel     context.CancelFunc
}

func NewScreeningSyncService(
	configService *ConfigService,
	store *ScreeningStore,
	source screeningMarketSource,
) *ScreeningSyncService {
	return &ScreeningSyncService{
		configService: configService,
		store:         store,
		source:        source,
		now:           time.Now,
	}
}

func (s *ScreeningSyncService) Sync() (*ScreeningSyncStatus, error) {
	return s.SyncWithOptions(context.Background(), ScreeningSyncRunOptions{
		Mode: ScreeningSyncModeAuto,
	}, nil)
}

func (s *ScreeningSyncService) SyncWithOptions(
	ctx context.Context,
	options ScreeningSyncRunOptions,
	report func(ScreeningSyncProgress),
) (*ScreeningSyncStatus, error) {
	if s == nil || s.store == nil || s.source == nil || s.configService == nil {
		return nil, fmt.Errorf("screening sync service not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.lastProgress != nil && s.lastProgress.RunStatus == string(ScreeningSyncRunStatusRunning) {
		s.mu.Unlock()
		return nil, fmt.Errorf("screening sync already running")
	}
	s.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	s.setRunCancel(cancel)
	defer s.clearRunCancel()

	cfg := s.configService.GetConfig().Screening
	scopeKey := screeningMarketScopeKey(cfg.Markets)
	options = normalizeScreeningSyncRunOptions(options)
	screeningSyncLog.Info("sync start: scope=%s mode=%s limit=%d", scopeKey, options.Mode, options.LimitStocks)
	status := &ScreeningSyncStatus{
		MarketScope:     scopeKey,
		InitialSyncDays: cfg.InitialSyncDays,
		RetentionMode:   string(cfg.RetentionMode),
		RetentionDays:   cfg.RetentionDays,
		RunStatus:       string(ScreeningSyncRunStatusRunning),
		CurrentStage:    "prepare",
		LimitStocks:     options.LimitStocks,
	}
	progress := ScreeningSyncProgress{
		MarketScope: scopeKey,
		Mode:        string(options.Mode),
		RunStatus:   string(ScreeningSyncRunStatusRunning),
		LimitStocks: options.LimitStocks,
		Events:      make([]ScreeningSyncEvent, 0, 8),
	}
	s.setLastProgress(progress)
	defer func() {
		if status != nil {
			s.attachStoredCounts(status)
			s.setLastProgress(buildScreeningSyncProgress(status))
		}
	}()

	stocks, err := s.source.ListScreeningStocks(cfg.Markets)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = fmt.Sprintf("list screening stocks: %v", err)
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, fmt.Errorf("list screening stocks: %w", err)
	}
	stocks = filterActiveScreeningStocks(stocks)

	if options.Mode == ScreeningSyncModeManual && options.LimitStocks > 0 && len(stocks) > options.LimitStocks {
		stocks = stocks[:options.LimitStocks]
	}

	if err := s.upsertStocksBasic(stocks); err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}
	candidateSymbols := extractScreeningSyncSymbols(stocks)
	status.SyncedSymbols = candidateSymbols

	state, err := s.store.GetSyncState(screeningSyncStateDataset, scopeKey)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}
	jobState, err := s.store.GetSyncJobState(screeningSyncStateDataset, scopeKey)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}

	lookbackDays := cfg.InitialSyncDays
	if lookbackDays <= 0 {
		lookbackDays = 30
	}
	lastTradeDate := ""
	if state != nil {
		lastTradeDate = state.LastTradeDate
		lookbackDays = maxInt(lookbackDays, screeningIncrementalLookbackDays(lastTradeDate, s.now()))
	}

	targetTradeDate, err := s.resolveTargetTradeDate(lastTradeDate)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}

	latestLocalTradeDates, err := s.store.GetLatestDailyBarTradeDatesForSymbols(candidateSymbols)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}

	syncSymbols, err := s.store.ListSymbolsMissingDailyBarsOnTradeDate(candidateSymbols, targetTradeDate)
	if err != nil {
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "prepare"
		status.Error = err.Error()
		progress.RunStatus = string(ScreeningSyncRunStatusFailed)
		progress.CurrentStage = "prepare"
		progress.Error = status.Error
		s.emitScreeningSyncProgress(progress, report)
		return status, err
	}
	stocks = filterScreeningStocksBySymbols(stocks, syncSymbols)
	status.StocksSynced = len(stocks)
	status.TotalStocks = len(stocks)
	progress.TotalStocks = len(stocks)

	startIndex, resumed := screeningResumeStartIndex(stocks, jobState)
	if resumed {
		status.ResumeFromCheckpoint = true
		progress.ResumeFromCheckpoint = true
	}
	screeningSyncLog.Debug(
		"sync queue ready: scope=%s target=%s total=%d startIndex=%d resumed=%t pending=%d latest=%d",
		scopeKey,
		targetTradeDate,
		len(stocks),
		startIndex,
		resumed,
		len(syncSymbols),
		len(candidateSymbols)-len(syncSymbols),
	)
	status.CompletedStocks = startIndex
	status.ProgressPercent = calculateScreeningSyncPercent(startIndex, len(stocks))
	progress.CompletedStocks = startIndex
	progress.ProgressPercent = status.ProgressPercent
	s.appendStatusEvent(status, ScreeningSyncEvent{
		Time:   formatScreeningStoreTime(s.now().UTC()),
		Status: "queue",
		Message: fmt.Sprintf(
			"sync queue ready: target=%s total=%d pending=%d resumed=%t limit=%d",
			targetTradeDate,
			len(stocks),
			len(syncSymbols),
			resumed,
			options.LimitStocks,
		),
	})
	initialJobState := (*ScreeningSyncJobState)(nil)
	if resumed {
		initialJobState = jobState
	}
	s.persistScreeningSyncJobState(scopeKey, options, status, initialJobState, "")
	s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)

	latestBars := make(map[string]latestBarInfo, len(stocks))
	latestSyncedTradeDate := lastTradeDate

	for idx := startIndex; idx < len(stocks); idx++ {
		stock := stocks[idx]
		if runCtx.Err() != nil {
			lastCompletedSymbol := status.CurrentSymbol
			screeningSyncLog.Info(
				"sync canceled: scope=%s completed=%d total=%d lastSymbol=%s currentSymbol=%s",
				scopeKey,
				status.CompletedStocks,
				len(stocks),
				lastCompletedSymbol,
				status.CurrentSymbol,
			)
			status.RunStatus = string(ScreeningSyncRunStatusCanceled)
			status.CurrentStage = "canceled"
			status.LastMessage = "sync canceled, checkpoint saved"
			status.CurrentSymbol = ""
			status.CurrentName = ""
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:        status.CompletedStocks,
				CompletedStocks:     status.CompletedStocks,
				LastCompletedSymbol: lastCompletedSymbol,
			}, "")
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			return status, nil
		}

		status.CurrentStage = "syncing"
		status.CurrentSymbol = stock.Symbol
		status.CurrentName = stock.Name
		status.LastMessage = fmt.Sprintf("syncing %s", stock.Symbol)
		status.ActiveSource = ""
		s.appendStatusEvent(status, ScreeningSyncEvent{
			Time:    formatScreeningStoreTime(s.now().UTC()),
			Symbol:  stock.Symbol,
			Name:    stock.Name,
			Status:  "fetch",
			Message: fmt.Sprintf(
				"syncing %s: target=%s localLatest=%s lookback=%d",
				stock.Symbol,
				targetTradeDate,
				latestLocalTradeDates[stock.Symbol],
				stockLookbackDaysForPreview(lookbackDays, latestLocalTradeDates[stock.Symbol], s.now()),
			),
		})
		s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)

		observer := func(event ScreeningDailyBarSourceEvent) {
			status.ActiveSource = event.Source
			status.LastMessage = event.Message
			s.appendStatusEvent(status, ScreeningSyncEvent{
				Time:    formatScreeningStoreTime(s.now().UTC()),
				Symbol:  stock.Symbol,
				Name:    stock.Name,
				Source:  event.Source,
				Status:  event.Status,
				Message: event.Message,
			})
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
		}

		stockLookbackDays := lookbackDays
		if stockLatestTradeDate := latestLocalTradeDates[stock.Symbol]; stockLatestTradeDate != "" {
			stockLookbackDays = maxInt(stockLookbackDays, screeningIncrementalLookbackDays(stockLatestTradeDate, s.now()))
		}

		bars, err := s.getScreeningDailyBars(stock.Symbol, stockLookbackDays, observer)
		if err != nil {
			screeningSyncLog.Error("sync fetch bars failed: symbol=%s lookback=%d err=%v", stock.Symbol, stockLookbackDays, err)
			status.CompletedStocks = idx + 1
			status.ProgressPercent = calculateScreeningSyncPercent(status.CompletedStocks, len(stocks))
			status.LastMessage = fmt.Sprintf("skip %s after fetch failure", stock.Symbol)
			s.appendStatusEvent(status, ScreeningSyncEvent{
				Time:    formatScreeningStoreTime(s.now().UTC()),
				Symbol:  stock.Symbol,
				Name:    stock.Name,
				Source:  status.ActiveSource,
				Status:  "error",
				Message: fmt.Sprintf(
					"fetch failed for %s: target=%s localLatest=%s lookback=%d err=%v",
					stock.Symbol,
					targetTradeDate,
					latestLocalTradeDates[stock.Symbol],
					stockLookbackDays,
					err,
				),
			})
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:        idx + 1,
				CompletedStocks:     status.CompletedStocks,
				LastCompletedSymbol: stock.Symbol,
			}, "")
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			continue
		}

		filteredBars := filterBarsForSync(bars, latestLocalTradeDates[stock.Symbol], cfg.InitialSyncDays)
		if len(filteredBars) == 0 {
			screeningSyncLog.Debug(
				"no new bars for symbol: symbol=%s target=%s localLatest=%s sourceBars=%d",
				stock.Symbol,
				targetTradeDate,
				latestLocalTradeDates[stock.Symbol],
				len(bars),
			)
			status.CompletedStocks = idx + 1
			status.ProgressPercent = calculateScreeningSyncPercent(status.CompletedStocks, len(stocks))
			status.LastMessage = fmt.Sprintf("no new bars for %s", stock.Symbol)
			s.appendStatusEvent(status, ScreeningSyncEvent{
				Time:    formatScreeningStoreTime(s.now().UTC()),
				Symbol:  stock.Symbol,
				Name:    stock.Name,
				Source:  status.ActiveSource,
				Status:  "skip",
				Message: fmt.Sprintf(
					"no new bars for %s: target=%s localLatest=%s sourceBars=%d",
					stock.Symbol,
					targetTradeDate,
					latestLocalTradeDates[stock.Symbol],
					len(bars),
				),
			})
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:        idx + 1,
				CompletedStocks:     status.CompletedStocks,
				LastCompletedSymbol: stock.Symbol,
			}, "")
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			continue
		}

		if err := s.upsertDailyBars(stock.Symbol, filteredBars); err != nil {
			screeningSyncLog.Error("sync upsert daily bars failed: symbol=%s bars=%d err=%v", stock.Symbol, len(filteredBars), err)
			status.RunStatus = string(ScreeningSyncRunStatusFailed)
			status.CurrentStage = "failed"
			status.Error = err.Error()
			status.LastMessage = status.Error
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:    idx,
				CompletedStocks: status.CompletedStocks,
			}, status.Error)
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			return status, err
		}
		status.BarsSynced += len(filteredBars)

		snapshots := deriveSnapshotsForBars(bars, filteredBars)
		if err := s.upsertDailySnapshots(stock.Symbol, snapshots); err != nil {
			screeningSyncLog.Error("sync upsert snapshots failed: symbol=%s snapshots=%d err=%v", stock.Symbol, len(snapshots), err)
			status.RunStatus = string(ScreeningSyncRunStatusFailed)
			status.CurrentStage = "failed"
			status.Error = err.Error()
			status.LastMessage = status.Error
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:    idx,
				CompletedStocks: status.CompletedStocks,
			}, status.Error)
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			return status, err
		}
		status.SnapshotsSynced += len(snapshots)

		lastBar := filteredBars[len(filteredBars)-1]
		lastBarDate := normalizeTradeDate(lastBar.Time)
		latestBars[stock.Symbol] = latestBarInfo{
			tradeDate: lastBarDate,
			bar:       lastBar,
		}
		latestLocalTradeDates[stock.Symbol] = lastBarDate
		if lastBarDate > latestSyncedTradeDate {
			latestSyncedTradeDate = lastBarDate
		}
		status.CompletedStocks = idx + 1
		status.ProgressPercent = calculateScreeningSyncPercent(status.CompletedStocks, len(stocks))
		status.LastMessage = fmt.Sprintf("completed %d / %d", status.CompletedStocks, len(stocks))
		s.appendStatusEvent(status, ScreeningSyncEvent{
			Time:    formatScreeningStoreTime(s.now().UTC()),
			Symbol:  stock.Symbol,
			Name:    stock.Name,
			Source:  status.ActiveSource,
			Status:  "stored",
			Message: fmt.Sprintf(
				"stored %d bars for %s: latest=%s completed=%d/%d",
				len(filteredBars),
				stock.Symbol,
				lastBarDate,
				status.CompletedStocks,
				len(stocks),
			),
		})
		s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
			CurrentIndex:        idx + 1,
			CompletedStocks:     status.CompletedStocks,
			LastCompletedSymbol: stock.Symbol,
		}, "")
		s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
	}

	if len(stocks) > 0 {
		if err := s.refreshLatestSnapshots(stocks, latestBars); err != nil {
			screeningSyncLog.Error("refresh latest snapshots failed: scope=%s err=%v", scopeKey, err)
			status.RunStatus = string(ScreeningSyncRunStatusFailed)
			status.CurrentStage = "failed"
			status.Error = err.Error()
			status.LastMessage = status.Error
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:    status.CompletedStocks,
				CompletedStocks: status.CompletedStocks,
			}, status.Error)
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			return status, err
		}
	}

	if latestSyncedTradeDate != "" {
		syncTime := s.now().UTC()
		if err := s.store.UpsertSyncState(ScreeningSyncState{
			Dataset:       screeningSyncStateDataset,
			MarketScope:   scopeKey,
			LastTradeDate: latestSyncedTradeDate,
			UpdatedAt:     syncTime,
		}); err != nil {
			status.RunStatus = string(ScreeningSyncRunStatusFailed)
			status.CurrentStage = "failed"
			status.Error = err.Error()
			status.LastMessage = status.Error
			s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
				CurrentIndex:    status.CompletedStocks,
				CompletedStocks: status.CompletedStocks,
			}, status.Error)
			s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
			return status, err
		}
		status.LastTradeDate = latestSyncedTradeDate
		status.LastSyncedAt = formatScreeningStoreTime(syncTime)
	} else if state != nil {
		status.LastTradeDate = state.LastTradeDate
		status.LastSyncedAt = formatScreeningStoreTime(state.UpdatedAt)
	}

	if err := s.applyRetention(cfg); err != nil {
		screeningSyncLog.Error("apply retention failed: scope=%s err=%v", scopeKey, err)
		status.RunStatus = string(ScreeningSyncRunStatusFailed)
		status.CurrentStage = "failed"
		status.Error = err.Error()
		status.LastMessage = status.Error
		s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
			CurrentIndex:    status.CompletedStocks,
			CompletedStocks: status.CompletedStocks,
		}, status.Error)
		s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)
		return status, err
	}

	status.RunStatus = string(ScreeningSyncRunStatusCompleted)
	status.CurrentStage = "completed"
	status.ProgressPercent = 100
	status.CompletedStocks = len(stocks)
	status.CurrentSymbol = ""
	status.CurrentName = ""
	if len(stocks) == 0 {
		status.LastMessage = "all candidate stocks are already synced"
	} else {
		status.LastMessage = "screening sync completed"
	}
	screeningSyncLog.Info(
		"sync completed: scope=%s total=%d completed=%d bars=%d snapshots=%d lastTradeDate=%s",
		scopeKey,
		len(stocks),
		status.CompletedStocks,
		status.BarsSynced,
		status.SnapshotsSynced,
		status.LastTradeDate,
	)
	s.persistScreeningSyncJobState(scopeKey, options, status, &ScreeningSyncJobState{
		CurrentIndex:        len(stocks),
		CompletedStocks:     len(stocks),
		LastCompletedSymbol: status.CurrentSymbol,
	}, "")
	s.emitScreeningSyncProgress(buildScreeningSyncProgress(status), report)

	return status, nil
}

func (s *ScreeningSyncService) GetStatus() (*ScreeningSyncStatus, error) {
	if s == nil || s.store == nil || s.configService == nil {
		return nil, fmt.Errorf("screening sync service not initialized")
	}

	cfg := s.configService.GetConfig().Screening
	scopeKey := screeningMarketScopeKey(cfg.Markets)
	status := &ScreeningSyncStatus{
		MarketScope:     scopeKey,
		InitialSyncDays: cfg.InitialSyncDays,
		RetentionMode:   string(cfg.RetentionMode),
		RetentionDays:   cfg.RetentionDays,
	}

	state, err := s.store.GetSyncState(screeningSyncStateDataset, scopeKey)
	if err != nil {
		return nil, err
	}
	if state != nil {
		status.LastTradeDate = state.LastTradeDate
		status.LastSyncedAt = formatScreeningStoreTime(state.UpdatedAt)
	}
	jobState, err := s.store.GetSyncJobState(screeningSyncStateDataset, scopeKey)
	if err != nil {
		return nil, err
	}
	jobState = s.normalizeStaleRunningJobState(scopeKey, jobState)
	if jobState != nil {
		status.RunStatus = jobState.Status
		status.TotalStocks = jobState.TotalStocks
		status.CompletedStocks = jobState.CompletedStocks
		status.ProgressPercent = calculateScreeningSyncPercent(jobState.CompletedStocks, jobState.TotalStocks)
		status.CurrentSymbol = jobState.CurrentSymbol
		status.CurrentName = jobState.CurrentName
		status.ActiveSource = jobState.ActiveSource
		status.LastMessage = jobState.LastMessage
		status.LimitStocks = jobState.LimitStocks
		status.Error = jobState.Error
	}
	if lastProgress := s.getLastProgress(); lastProgress != nil {
		status.RunStatus = lastProgress.RunStatus
		status.ProgressPercent = lastProgress.ProgressPercent
		status.TotalStocks = lastProgress.TotalStocks
		status.CompletedStocks = lastProgress.CompletedStocks
		status.CurrentSymbol = lastProgress.CurrentSymbol
		status.CurrentName = lastProgress.CurrentName
		status.CurrentStage = lastProgress.CurrentStage
		status.ActiveSource = lastProgress.ActiveSource
		status.LastMessage = lastProgress.LastMessage
		status.LimitStocks = lastProgress.LimitStocks
		status.ResumeFromCheckpoint = lastProgress.ResumeFromCheckpoint
		status.Events = append([]ScreeningSyncEvent(nil), lastProgress.Events...)
		if lastProgress.Error != "" {
			status.Error = lastProgress.Error
		}
	}
	s.attachStoredCounts(status)
	s.attachCoverageSummary(status, cfg)
	return status, nil
}

func (s *ScreeningSyncService) ListScreeningUniverseSymbols(limit int) ([]string, error) {
	if s == nil || s.store == nil || s.configService == nil {
		return nil, fmt.Errorf("screening sync service not initialized")
	}

	cfg := s.configService.GetConfig().Screening
	return s.store.ListScreeningUniverseSymbols(cfg.Markets, limit)
}

type derivedSnapshot struct {
	TradeDate     string
	Price         float64
	Change        float64
	ChangePercent float64
	Amplitude     float64
	TurnoverRate  float64
}

type latestBarInfo struct {
	tradeDate string
	bar       models.KLineData
}

func (s *ScreeningSyncService) upsertStocksBasic(stocks []ScreeningStockBasic) error {
	screeningSyncLog.Debug("tx begin: stocks_basic total=%d", len(stocks))
	tx, err := s.store.db.Begin()
	if err != nil {
		screeningSyncLog.Error("tx begin failed: stocks_basic total=%d err=%v", len(stocks), err)
		return fmt.Errorf("begin stocks_basic tx: %w", err)
	}
	defer rollbackOnError(tx, &err)

	stmt, err := tx.Prepare(`
		INSERT INTO stocks_basic (symbol, name, market, industry, list_date, is_st, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol) DO UPDATE SET
			name = excluded.name,
			market = excluded.market,
			industry = excluded.industry,
			list_date = excluded.list_date,
			is_st = excluded.is_st,
			is_active = excluded.is_active
	`)
	if err != nil {
		return fmt.Errorf("prepare stocks_basic insert: %w", err)
	}
	defer stmt.Close()

	for _, stock := range stocks {
		if _, err = stmt.Exec(
			stock.Symbol,
			stock.Name,
			stock.Market,
			stock.Industry,
			stock.ListDate,
			boolToInt(stock.IsST),
			boolToInt(stock.IsActive),
		); err != nil {
			return fmt.Errorf("upsert stocks_basic: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		screeningSyncLog.Error("tx commit failed: stocks_basic total=%d err=%v", len(stocks), err)
		return fmt.Errorf("commit stocks_basic tx: %w", err)
	}
	screeningSyncLog.Debug("tx commit: stocks_basic total=%d", len(stocks))
	return nil
}

func (s *ScreeningSyncService) upsertDailyBars(symbol string, bars []models.KLineData) error {
	screeningSyncLog.Debug("tx begin: daily_bars symbol=%s bars=%d", symbol, len(bars))
	tx, err := s.store.db.Begin()
	if err != nil {
		screeningSyncLog.Error("tx begin failed: daily_bars symbol=%s bars=%d err=%v", symbol, len(bars), err)
		return fmt.Errorf("begin daily_bars tx: %w", err)
	}
	defer rollbackOnError(tx, &err)

	stmt, err := tx.Prepare(`
		INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			open = excluded.open,
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			volume = excluded.volume,
			amount = excluded.amount
	`)
	if err != nil {
		return fmt.Errorf("prepare daily_bars insert: %w", err)
	}
	defer stmt.Close()

	for _, bar := range bars {
		if _, err = stmt.Exec(
			symbol,
			normalizeTradeDate(bar.Time),
			bar.Open,
			bar.High,
			bar.Low,
			bar.Close,
			bar.Volume,
			bar.Amount,
		); err != nil {
			return fmt.Errorf("upsert daily_bars: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		screeningSyncLog.Error("tx commit failed: daily_bars symbol=%s bars=%d err=%v", symbol, len(bars), err)
		return fmt.Errorf("commit daily_bars tx: %w", err)
	}
	screeningSyncLog.Debug("tx commit: daily_bars symbol=%s bars=%d", symbol, len(bars))
	return nil
}

func (s *ScreeningSyncService) upsertDailySnapshots(symbol string, snapshots []derivedSnapshot) error {
	screeningSyncLog.Debug("tx begin: daily_snapshots symbol=%s snapshots=%d", symbol, len(snapshots))
	tx, err := s.store.db.Begin()
	if err != nil {
		screeningSyncLog.Error("tx begin failed: daily_snapshots symbol=%s snapshots=%d err=%v", symbol, len(snapshots), err)
		return fmt.Errorf("begin daily_snapshots tx: %w", err)
	}
	defer rollbackOnError(tx, &err)

	stmt, err := tx.Prepare(`
		INSERT INTO daily_snapshots (symbol, trade_date, change, change_percent, amplitude, turnover_rate, price)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			change = excluded.change,
			change_percent = excluded.change_percent,
			amplitude = excluded.amplitude,
			turnover_rate = excluded.turnover_rate,
			price = excluded.price
	`)
	if err != nil {
		return fmt.Errorf("prepare daily_snapshots insert: %w", err)
	}
	defer stmt.Close()

	for _, snapshot := range snapshots {
		if _, err = stmt.Exec(
			symbol,
			snapshot.TradeDate,
			snapshot.Change,
			snapshot.ChangePercent,
			snapshot.Amplitude,
			snapshot.TurnoverRate,
			snapshot.Price,
		); err != nil {
			return fmt.Errorf("upsert daily_snapshots: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		screeningSyncLog.Error("tx commit failed: daily_snapshots symbol=%s snapshots=%d err=%v", symbol, len(snapshots), err)
		return fmt.Errorf("commit daily_snapshots tx: %w", err)
	}
	screeningSyncLog.Debug("tx commit: daily_snapshots symbol=%s snapshots=%d", symbol, len(snapshots))
	return nil
}

func (s *ScreeningSyncService) refreshLatestSnapshots(
	stocks []ScreeningStockBasic,
	latestBars map[string]latestBarInfo,
) error {
	symbols := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		if _, ok := latestBars[stock.Symbol]; ok {
			symbols = append(symbols, stock.Symbol)
		}
	}
	if len(symbols) == 0 {
		return nil
	}

	realtimeStocks, err := s.source.GetScreeningSnapshots(symbols)
	if err != nil {
		return fmt.Errorf("get screening snapshots: %w", err)
	}
	if len(realtimeStocks) == 0 {
		return nil
	}

	tx, err := s.store.db.Begin()
	if err != nil {
		screeningSyncLog.Error("tx begin failed: refresh_latest_snapshots total=%d err=%v", len(symbols), err)
		return fmt.Errorf("begin refresh latest snapshots tx: %w", err)
	}
	defer rollbackOnError(tx, &err)

	stmt, err := tx.Prepare(`
		INSERT INTO daily_snapshots (symbol, trade_date, change, change_percent, amplitude, turnover_rate, price)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			change = excluded.change,
			change_percent = excluded.change_percent,
			amplitude = excluded.amplitude,
			turnover_rate = excluded.turnover_rate,
			price = excluded.price
	`)
	if err != nil {
		return fmt.Errorf("prepare refresh latest snapshots insert: %w", err)
	}
	defer stmt.Close()

	for _, stock := range realtimeStocks {
		latestBar, ok := latestBars[stock.Symbol]
		if !ok {
			continue
		}
		amplitude := 0.0
		if stock.PreClose > 0 {
			amplitude = ((stock.High - stock.Low) / stock.PreClose) * 100
		}
		if _, err = stmt.Exec(
			stock.Symbol,
			latestBar.tradeDate,
			stock.Change,
			stock.ChangePercent,
			amplitude,
			0,
			stock.Price,
		); err != nil {
			return fmt.Errorf("refresh latest snapshot: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		screeningSyncLog.Error("tx commit failed: refresh_latest_snapshots total=%d err=%v", len(symbols), err)
		return fmt.Errorf("commit refresh latest snapshots tx: %w", err)
	}
	screeningSyncLog.Debug("tx commit: refresh_latest_snapshots total=%d", len(symbols))
	return nil
}

func (s *ScreeningSyncService) applyRetention(cfg models.ScreeningConfig) error {
	if cfg.RetentionMode != models.ScreeningRetentionModeDays || cfg.RetentionDays <= 0 {
		return nil
	}

	cutoffDate := s.now().AddDate(0, 0, -cfg.RetentionDays).Format("2006-01-02")
	for _, tableName := range []string{"daily_bars", "daily_snapshots"} {
		if _, err := s.store.db.Exec(
			`DELETE FROM `+tableName+` WHERE trade_date < ?`,
			cutoffDate,
		); err != nil {
			return fmt.Errorf("apply retention for %s: %w", tableName, err)
		}
	}
	return nil
}

func screeningMarketScopeKey(scopes models.ScreeningMarketScopeConfig) string {
	values := make([]string, 0, 4)
	if scopes.Shanghai {
		values = append(values, "shanghai")
	}
	if scopes.Shenzhen {
		values = append(values, "shenzhen")
	}
	if scopes.Beijing {
		values = append(values, "beijing")
	}
	if scopes.Indices {
		values = append(values, "indices")
	}
	return strings.Join(values, ",")
}

func screeningIncrementalLookbackDays(lastTradeDate string, now time.Time) int {
	if lastTradeDate == "" {
		return 30
	}

	lastDate, err := time.Parse("2006-01-02", lastTradeDate)
	if err != nil {
		return 30
	}
	days := int(now.Sub(lastDate).Hours()/24) + 5
	if days < 5 {
		return 5
	}
	return days
}

func filterScreeningStocksBySymbols(stocks []ScreeningStockBasic, symbols []string) []ScreeningStockBasic {
	if len(stocks) == 0 || len(symbols) == 0 {
		return nil
	}

	symbolSet := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbolSet[strings.TrimSpace(symbol)] = struct{}{}
	}

	filtered := make([]ScreeningStockBasic, 0, len(symbols))
	for _, stock := range stocks {
		if _, ok := symbolSet[stock.Symbol]; !ok {
			continue
		}
		filtered = append(filtered, stock)
	}
	return filtered
}

func filterBarsForSync(bars []models.KLineData, lastTradeDate string, initialSyncDays int) []models.KLineData {
	if len(bars) == 0 {
		return nil
	}

	filtered := append([]models.KLineData(nil), bars...)
	sort.Slice(filtered, func(i, j int) bool {
		return normalizeTradeDate(filtered[i].Time) < normalizeTradeDate(filtered[j].Time)
	})

	if lastTradeDate == "" {
		if initialSyncDays > 0 && len(filtered) > initialSyncDays {
			filtered = filtered[len(filtered)-initialSyncDays:]
		}
		return filtered
	}

	var result []models.KLineData
	for _, bar := range filtered {
		if normalizeTradeDate(bar.Time) > lastTradeDate {
			result = append(result, bar)
		}
	}
	return result
}

func deriveSnapshotsForBars(allBars []models.KLineData, selectedBars []models.KLineData) []derivedSnapshot {
	if len(allBars) == 0 || len(selectedBars) == 0 {
		return nil
	}

	sortedBars := append([]models.KLineData(nil), allBars...)
	sort.Slice(sortedBars, func(i, j int) bool {
		return normalizeTradeDate(sortedBars[i].Time) < normalizeTradeDate(sortedBars[j].Time)
	})

	selectedDates := make(map[string]struct{}, len(selectedBars))
	for _, bar := range selectedBars {
		selectedDates[normalizeTradeDate(bar.Time)] = struct{}{}
	}

	result := make([]derivedSnapshot, 0, len(selectedBars))
	var previousClose float64
	for _, bar := range sortedBars {
		tradeDate := normalizeTradeDate(bar.Time)
		change := bar.Close - bar.Open
		changePercent := 0.0
		if bar.Open > 0 {
			changePercent = (change / bar.Open) * 100
		}
		if previousClose > 0 {
			change = bar.Close - previousClose
			changePercent = (change / previousClose) * 100
		}
		amplitude := 0.0
		if bar.Open > 0 {
			amplitude = ((bar.High - bar.Low) / bar.Open) * 100
		}

		if _, ok := selectedDates[tradeDate]; ok {
			result = append(result, derivedSnapshot{
				TradeDate:     tradeDate,
				Price:         bar.Close,
				Change:        change,
				ChangePercent: changePercent,
				Amplitude:     amplitude,
				TurnoverRate:  0,
			})
		}
		previousClose = bar.Close
	}
	return result
}

func normalizeTradeDate(raw string) string {
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func rollbackOnError(tx *sql.Tx, errPtr *error) {
	if *errPtr != nil {
		_ = tx.Rollback()
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeScreeningSyncRunOptions(options ScreeningSyncRunOptions) ScreeningSyncRunOptions {
	if options.Mode == "" {
		options.Mode = ScreeningSyncModeManual
	}
	if options.Mode != ScreeningSyncModeManual {
		options.LimitStocks = 0
	}
	if options.LimitStocks < 0 {
		options.LimitStocks = 0
	}
	return options
}

func screeningResumeStartIndex(stocks []ScreeningStockBasic, jobState *ScreeningSyncJobState) (int, bool) {
	if len(stocks) == 0 || jobState == nil || jobState.Status == string(ScreeningSyncRunStatusCompleted) {
		return 0, false
	}

	if symbol := strings.TrimSpace(jobState.CurrentSymbol); symbol != "" {
		for idx, stock := range stocks {
			if stock.Symbol == symbol {
				return idx, true
			}
		}
	}
	if symbol := strings.TrimSpace(jobState.LastCompletedSymbol); symbol != "" {
		for idx, stock := range stocks {
			if stock.Symbol == symbol {
				if idx+1 < len(stocks) {
					return idx + 1, true
				}
				return len(stocks), true
			}
		}
	}
	if jobState.CurrentIndex > 0 && jobState.CurrentIndex < len(stocks) {
		return jobState.CurrentIndex, true
	}
	return 0, false
}

func calculateScreeningSyncPercent(completed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return (float64(completed) / float64(total)) * 100
}

func appendBoundedScreeningSyncEvents(events []ScreeningSyncEvent, event ScreeningSyncEvent, limit int) []ScreeningSyncEvent {
	events = append(events, event)
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events
}

func (s *ScreeningSyncService) appendStatusEvent(status *ScreeningSyncStatus, event ScreeningSyncEvent) {
	if status == nil {
		return
	}
	status.Events = appendBoundedScreeningSyncEvents(status.Events, event, 20)
}

func stockLookbackDaysForPreview(defaultLookback int, latestTradeDate string, now time.Time) int {
	lookbackDays := defaultLookback
	if latestTradeDate != "" {
		lookbackDays = maxInt(lookbackDays, screeningIncrementalLookbackDays(latestTradeDate, now))
	}
	return lookbackDays
}

func buildScreeningSyncProgress(status *ScreeningSyncStatus) ScreeningSyncProgress {
	if status == nil {
		return ScreeningSyncProgress{}
	}
	events := append([]ScreeningSyncEvent(nil), status.Events...)
	return ScreeningSyncProgress{
		MarketScope:          status.MarketScope,
		Mode:                 "",
		RunStatus:            status.RunStatus,
		ProgressPercent:      status.ProgressPercent,
		TotalStocks:          status.TotalStocks,
		CompletedStocks:      status.CompletedStocks,
		CurrentSymbol:        status.CurrentSymbol,
		CurrentName:          status.CurrentName,
		CurrentStage:         status.CurrentStage,
		ActiveSource:         status.ActiveSource,
		LastMessage:          status.LastMessage,
		LimitStocks:          status.LimitStocks,
		ResumeFromCheckpoint: status.ResumeFromCheckpoint,
		Events:               events,
		Error:                status.Error,
	}
}

func (s *ScreeningSyncService) setLastProgress(progress ScreeningSyncProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := progress
	cloned.Events = append([]ScreeningSyncEvent(nil), progress.Events...)
	s.lastProgress = &cloned
}

func (s *ScreeningSyncService) getLastProgress() *ScreeningSyncProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastProgress == nil {
		return nil
	}
	cloned := *s.lastProgress
	cloned.Events = append([]ScreeningSyncEvent(nil), s.lastProgress.Events...)
	return &cloned
}

func (s *ScreeningSyncService) setRunCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runCancel = cancel
}

func (s *ScreeningSyncService) clearRunCancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runCancel = nil
}

func (s *ScreeningSyncService) Cancel() bool {
	s.mu.Lock()
	cancel := s.runCancel
	s.mu.Unlock()
	if cancel == nil {
		screeningSyncLog.Debug("cancel ignored: no running sync")
		return false
	}
	screeningSyncLog.Info("cancel requested")
	cancel()
	return true
}

func (s *ScreeningSyncService) emitScreeningSyncProgress(progress ScreeningSyncProgress, report func(ScreeningSyncProgress)) {
	s.setLastProgress(progress)
	if report != nil {
		report(progress)
	}
}

func (s *ScreeningSyncService) resolveTargetTradeDate(lastTradeDate string) (string, error) {
	if s == nil || s.source == nil {
		return lastTradeDate, nil
	}
	todayTradeDate := s.now().Format("2006-01-02")
	lastTradeDate = strings.TrimSpace(lastTradeDate)
	if provider, ok := s.source.(screeningTradeDateSource); ok {
		tradeDates, err := provider.GetTradeDates(2)
		if err != nil {
			return "", fmt.Errorf("resolve target trade date: %w", err)
		}
		normalizedTradeDates := make([]string, 0, len(tradeDates))
		for _, tradeDate := range tradeDates {
			tradeDate = strings.TrimSpace(tradeDate)
			if tradeDate == "" {
				continue
			}
			normalizedTradeDates = append(normalizedTradeDates, tradeDate)
		}
		if len(normalizedTradeDates) > 0 {
			if normalizedTradeDates[0] == todayTradeDate && len(normalizedTradeDates) > 1 {
				screeningSyncLog.Debug(
					"resolve target trade date: today=%s latest=%s previous=%s chosen=%s",
					todayTradeDate,
					normalizedTradeDates[0],
					normalizedTradeDates[1],
					normalizedTradeDates[1],
				)
				return normalizedTradeDates[1], nil
			}
			screeningSyncLog.Debug(
				"resolve target trade date: today=%s latest=%s chosen=%s",
				todayTradeDate,
				normalizedTradeDates[0],
				normalizedTradeDates[0],
			)
			return normalizedTradeDates[0], nil
		}
	}

	fallbackTradeDate := todayTradeDate
	if lastTradeDate != "" && lastTradeDate > fallbackTradeDate {
		return lastTradeDate, nil
	}
	return fallbackTradeDate, nil
}

func (s *ScreeningSyncService) persistScreeningSyncJobState(scopeKey string, options ScreeningSyncRunOptions, status *ScreeningSyncStatus, base *ScreeningSyncJobState, errMessage string) {
	if s == nil || s.store == nil || status == nil {
		return
	}

	job := ScreeningSyncJobState{
		Dataset:             screeningSyncStateDataset,
		MarketScope:         scopeKey,
		Status:              status.RunStatus,
		Mode:                string(options.Mode),
		LimitStocks:         options.LimitStocks,
		TotalStocks:         status.TotalStocks,
		CompletedStocks:     status.CompletedStocks,
		CurrentIndex:        status.CompletedStocks,
		CurrentSymbol:       status.CurrentSymbol,
		CurrentName:         status.CurrentName,
		LastCompletedSymbol: "",
		ActiveSource:        status.ActiveSource,
		LastMessage:         status.LastMessage,
		Error:               errMessage,
		UpdatedAt:           s.now().UTC(),
	}
	if base != nil {
		if base.CurrentIndex != 0 || status.RunStatus == string(ScreeningSyncRunStatusCompleted) || status.RunStatus == string(ScreeningSyncRunStatusCanceled) || status.RunStatus == string(ScreeningSyncRunStatusFailed) {
			job.CurrentIndex = base.CurrentIndex
		}
		if base.CompletedStocks != 0 || status.CompletedStocks == 0 {
			job.CompletedStocks = base.CompletedStocks
		}
		if base.CurrentSymbol != "" {
			job.CurrentSymbol = base.CurrentSymbol
		}
		if base.CurrentName != "" {
			job.CurrentName = base.CurrentName
		}
		if base.LastCompletedSymbol != "" {
			job.LastCompletedSymbol = base.LastCompletedSymbol
		}
	}
	screeningSyncLog.Debug(
		"persist sync job state: scope=%s status=%s completed=%d total=%d currentIndex=%d currentSymbol=%s lastCompleted=%s resume=%t err=%s",
		scopeKey,
		job.Status,
		job.CompletedStocks,
		job.TotalStocks,
		job.CurrentIndex,
		job.CurrentSymbol,
		job.LastCompletedSymbol,
		status.ResumeFromCheckpoint,
		errMessage,
	)
	if err := s.store.UpsertSyncJobState(job); err != nil {
		screeningSyncLog.Warn(
			"persist sync job state failed: scope=%s status=%s completed=%d total=%d currentIndex=%d err=%v",
			scopeKey,
			job.Status,
			job.CompletedStocks,
			job.TotalStocks,
			job.CurrentIndex,
			err,
		)
	}
}

func (s *ScreeningSyncService) normalizeStaleRunningJobState(scopeKey string, jobState *ScreeningSyncJobState) *ScreeningSyncJobState {
	if s == nil || s.store == nil || jobState == nil || jobState.Status != string(ScreeningSyncRunStatusRunning) {
		return jobState
	}

	s.mu.Lock()
	hasActiveRun := s.runCancel != nil || (s.lastProgress != nil && s.lastProgress.RunStatus == string(ScreeningSyncRunStatusRunning))
	s.mu.Unlock()
	if hasActiveRun {
		return jobState
	}

	cloned := *jobState
	cloned.Status = string(ScreeningSyncRunStatusCanceled)
	if strings.TrimSpace(cloned.LastMessage) == "" || strings.EqualFold(strings.TrimSpace(cloned.LastMessage), "running") {
		cloned.LastMessage = "detected interrupted sync, checkpoint restored"
	}
	cloned.UpdatedAt = s.now().UTC()
	if err := s.store.UpsertSyncJobState(cloned); err != nil {
		screeningSyncLog.Warn(
			"normalize stale running job failed: scope=%s completed=%d total=%d err=%v",
			scopeKey,
			cloned.CompletedStocks,
			cloned.TotalStocks,
			err,
		)
		return jobState
	}
	screeningSyncLog.Warn(
		"normalized stale running job: scope=%s completed=%d total=%d lastCompleted=%s",
		scopeKey,
		cloned.CompletedStocks,
		cloned.TotalStocks,
		cloned.LastCompletedSymbol,
	)
	return &cloned
}

func (s *ScreeningSyncService) getScreeningDailyBars(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	if observed, ok := s.source.(screeningObservedMarketSource); ok {
		return observed.GetScreeningDailyBarsWithObserver(symbol, lookbackDays, observer)
	}
	return s.source.GetScreeningDailyBars(symbol, lookbackDays)
}

func extractScreeningSyncSymbols(stocks []ScreeningStockBasic) []string {
	if len(stocks) == 0 {
		return nil
	}
	symbols := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		if strings.TrimSpace(stock.Symbol) == "" {
			continue
		}
		symbols = append(symbols, stock.Symbol)
	}
	return symbols
}

func filterActiveScreeningStocks(stocks []ScreeningStockBasic) []ScreeningStockBasic {
	if len(stocks) == 0 {
		return nil
	}
	filtered := make([]ScreeningStockBasic, 0, len(stocks))
	for _, stock := range stocks {
		if !stock.IsActive && !isScreeningIndexSymbol(stock.Symbol) {
			continue
		}
		filtered = append(filtered, stock)
	}
	return filtered
}

func (s *ScreeningSyncService) attachStoredCounts(status *ScreeningSyncStatus) {
	if s == nil || s.store == nil || status == nil {
		return
	}
	status.StoredStocks = s.countScreeningTableRows("stocks_basic")
	status.StoredBars = s.countScreeningTableRows("daily_bars")
	status.StoredSnapshots = s.countScreeningTableRows("daily_snapshots")
}

func (s *ScreeningSyncService) attachCoverageSummary(status *ScreeningSyncStatus, cfg models.ScreeningConfig) {
	if s == nil || s.store == nil || s.source == nil || status == nil {
		return
	}

	stocks, err := s.source.ListScreeningStocks(cfg.Markets)
	if err != nil {
		return
	}
	stocks = filterActiveScreeningStocks(stocks)

	symbols := extractScreeningSyncSymbols(stocks)
	status.MarketStockCount = len(symbols)

	targetTradeDate, err := s.resolveTargetTradeDate(status.LastTradeDate)
	if err == nil {
		status.TargetTradeDate = targetTradeDate
	}

	latestTradeDates, err := s.store.GetLatestDailyBarTradeDatesForSymbols(symbols)
	if err == nil {
		for _, tradeDate := range latestTradeDates {
			if tradeDate > status.LatestSyncedTradeDate {
				status.LatestSyncedTradeDate = tradeDate
			}
		}
	}
	if status.LatestSyncedTradeDate == "" {
		status.LatestSyncedTradeDate = status.LastTradeDate
	}

	if status.TargetTradeDate == "" {
		return
	}

	missingSymbols, err := s.store.ListSymbolsMissingDailyBarsOnTradeDate(symbols, status.TargetTradeDate)
	if err != nil {
		return
	}

	status.PendingSyncStocks = len(missingSymbols)
	if status.MarketStockCount >= status.PendingSyncStocks {
		status.SyncedToLatestStocks = status.MarketStockCount - status.PendingSyncStocks
	}
}

func (s *ScreeningSyncService) countScreeningTableRows(tableName string) int {
	var count int
	if err := s.store.db.QueryRow(`SELECT COUNT(1) FROM ` + tableName).Scan(&count); err != nil {
		return 0
	}
	return count
}
