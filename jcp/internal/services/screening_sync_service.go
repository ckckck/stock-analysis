package services

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

const screeningSyncStateDataset = "daily_bars"

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
	MarketScope     string `json:"marketScope"`
	InitialSyncDays int    `json:"initialSyncDays"`
	RetentionMode   string `json:"retentionMode"`
	RetentionDays   int    `json:"retentionDays"`
	LastTradeDate   string `json:"lastTradeDate"`
	LastSyncedAt    string `json:"lastSyncedAt"`
	StocksSynced    int    `json:"stocksSynced"`
	BarsSynced      int    `json:"barsSynced"`
	SnapshotsSynced int    `json:"snapshotsSynced"`
	Error           string `json:"error,omitempty"`
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
	if s == nil || s.store == nil || s.source == nil || s.configService == nil {
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

	stocks, err := s.source.ListScreeningStocks(cfg.Markets)
	if err != nil {
		return nil, fmt.Errorf("list screening stocks: %w", err)
	}
	if err := s.upsertStocksBasic(stocks); err != nil {
		return nil, err
	}
	status.StocksSynced = len(stocks)

	state, err := s.store.GetSyncState(screeningSyncStateDataset, scopeKey)
	if err != nil {
		return nil, err
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

	latestBars := make(map[string]latestBarInfo, len(stocks))
	latestSyncedTradeDate := lastTradeDate

	for _, stock := range stocks {
		bars, err := s.source.GetScreeningDailyBars(stock.Symbol, lookbackDays)
		if err != nil {
			return nil, fmt.Errorf("get daily bars for %s: %w", stock.Symbol, err)
		}

		filteredBars := filterBarsForSync(bars, lastTradeDate, cfg.InitialSyncDays)
		if len(filteredBars) == 0 {
			continue
		}

		if err := s.upsertDailyBars(stock.Symbol, filteredBars); err != nil {
			return nil, err
		}
		status.BarsSynced += len(filteredBars)

		snapshots := deriveSnapshotsForBars(bars, filteredBars)
		if err := s.upsertDailySnapshots(stock.Symbol, snapshots); err != nil {
			return nil, err
		}
		status.SnapshotsSynced += len(snapshots)

		lastBar := filteredBars[len(filteredBars)-1]
		lastBarDate := normalizeTradeDate(lastBar.Time)
		latestBars[stock.Symbol] = latestBarInfo{
			tradeDate: lastBarDate,
			bar:       lastBar,
		}
		if lastBarDate > latestSyncedTradeDate {
			latestSyncedTradeDate = lastBarDate
		}
	}

	if len(stocks) > 0 {
		if err := s.refreshLatestSnapshots(stocks, latestBars); err != nil {
			return nil, err
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
			return nil, err
		}
		status.LastTradeDate = latestSyncedTradeDate
		status.LastSyncedAt = formatScreeningStoreTime(syncTime)
	} else if state != nil {
		status.LastTradeDate = state.LastTradeDate
		status.LastSyncedAt = formatScreeningStoreTime(state.UpdatedAt)
	}

	if err := s.applyRetention(cfg); err != nil {
		return nil, err
	}

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
	return status, nil
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
	tx, err := s.store.db.Begin()
	if err != nil {
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
		return fmt.Errorf("commit stocks_basic tx: %w", err)
	}
	return nil
}

func (s *ScreeningSyncService) upsertDailyBars(symbol string, bars []models.KLineData) error {
	tx, err := s.store.db.Begin()
	if err != nil {
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
		return fmt.Errorf("commit daily_bars tx: %w", err)
	}
	return nil
}

func (s *ScreeningSyncService) upsertDailySnapshots(symbol string, snapshots []derivedSnapshot) error {
	tx, err := s.store.db.Begin()
	if err != nil {
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
		return fmt.Errorf("commit daily_snapshots tx: %w", err)
	}
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
		return fmt.Errorf("commit refresh latest snapshots tx: %w", err)
	}
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
