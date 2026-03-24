package services

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
)

func TestNewScreeningStoreCreatesSchemaAndIndexes(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	if got, want := store.dbPath, paths.GetScreeningDBPathFrom(tempDir); got != want {
		t.Fatalf("dbPath = %q, want %q", got, want)
	}
	if _, err := sql.Open("sqlite", filepath.Join(tempDir, "missing.db")); err != nil {
		t.Fatalf("sql.Open() sanity check error = %v", err)
	}

	for _, tableName := range []string{
		"stocks_basic",
		"daily_bars",
		"daily_snapshots",
		"sync_state",
		"sync_jobs",
		"screening_runs",
		"screening_run_results",
	} {
		assertSQLiteObjectExists(t, store.db, "table", tableName)
	}

	for _, indexName := range []string{
		"idx_stocks_basic_market",
		"idx_daily_bars_symbol_trade_date",
		"idx_daily_snapshots_symbol_trade_date",
		"idx_sync_jobs_status_updated_at",
		"idx_screening_runs_created_at",
		"idx_screening_run_results_run_id_rank",
	} {
		assertSQLiteObjectExists(t, store.db, "index", indexName)
	}
}

func TestNewScreeningStoreEnablesWALAndBusyTimeout(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	var journalMode string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode error = %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(journalMode)); got != "wal" {
		t.Fatalf("journal_mode = %q, want wal", got)
	}

	var busyTimeout int
	if err := store.db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout error = %v", err)
	}
	if busyTimeout < 5000 {
		t.Fatalf("busy_timeout = %d, want >= 5000", busyTimeout)
	}
}

func TestScreeningStoreDataDirUsesAppDataDirByDefault(t *testing.T) {
	if got, want := screeningStoreDataDir(), paths.GetDataDir(); got != want {
		t.Fatalf("screeningStoreDataDir() = %q, want %q", got, want)
	}

	tempDir := t.TempDir()
	if got := screeningStoreDataDir(tempDir); got != tempDir {
		t.Fatalf("screeningStoreDataDir(tempDir) = %q, want %q", got, tempDir)
	}
}

func TestScreeningStoreSyncStateAndRunHistoryHelpers(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	expectedState := ScreeningSyncState{
		Dataset:       "daily_bars",
		MarketScope:   "shanghai,shenzhen",
		LastTradeDate: "2026-03-18",
		UpdatedAt:     time.Date(2026, 3, 19, 9, 30, 0, 0, time.UTC),
	}
	if err := store.UpsertSyncState(expectedState); err != nil {
		t.Fatalf("UpsertSyncState() error = %v", err)
	}

	gotState, err := store.GetSyncState(expectedState.Dataset, expectedState.MarketScope)
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if gotState == nil {
		t.Fatal("GetSyncState() = nil, want state")
	}
	if gotState.Dataset != expectedState.Dataset ||
		gotState.MarketScope != expectedState.MarketScope ||
		gotState.LastTradeDate != expectedState.LastTradeDate ||
		!gotState.UpdatedAt.Equal(expectedState.UpdatedAt) {
		t.Fatalf("GetSyncState() = %#v, want %#v", gotState, expectedState)
	}

	expectedJob := ScreeningSyncJobState{
		Dataset:             "daily_bars",
		MarketScope:         "shanghai,shenzhen",
		Status:              "canceled",
		Mode:                "manual",
		LimitStocks:         10,
		TotalStocks:         10,
		CompletedStocks:     4,
		CurrentIndex:        4,
		CurrentSymbol:       "sz000001",
		CurrentName:         "平安银行",
		LastCompletedSymbol: "sh600000",
		ActiveSource:        "baostock",
		LastMessage:         "已完成 4 / 10",
		Error:               "",
		UpdatedAt:           time.Date(2026, 3, 19, 10, 30, 0, 0, time.UTC),
	}
	if err := store.UpsertSyncJobState(expectedJob); err != nil {
		t.Fatalf("UpsertSyncJobState() error = %v", err)
	}

	gotJob, err := store.GetSyncJobState(expectedJob.Dataset, expectedJob.MarketScope)
	if err != nil {
		t.Fatalf("GetSyncJobState() error = %v", err)
	}
	if gotJob == nil {
		t.Fatal("GetSyncJobState() = nil, want job")
	}
	if gotJob.Status != expectedJob.Status ||
		gotJob.Mode != expectedJob.Mode ||
		gotJob.LimitStocks != expectedJob.LimitStocks ||
		gotJob.TotalStocks != expectedJob.TotalStocks ||
		gotJob.CompletedStocks != expectedJob.CompletedStocks ||
		gotJob.CurrentIndex != expectedJob.CurrentIndex ||
		gotJob.CurrentSymbol != expectedJob.CurrentSymbol ||
		gotJob.CurrentName != expectedJob.CurrentName ||
		gotJob.LastCompletedSymbol != expectedJob.LastCompletedSymbol ||
		gotJob.ActiveSource != expectedJob.ActiveSource ||
		gotJob.LastMessage != expectedJob.LastMessage ||
		gotJob.Error != expectedJob.Error ||
		!gotJob.UpdatedAt.Equal(expectedJob.UpdatedAt) {
		t.Fatalf("GetSyncJobState() = %#v, want %#v", gotJob, expectedJob)
	}

	runID, err := store.CreateScreeningRun(ScreeningRun{
		Prompt:       "找出近30天放量上涨的沪深股票",
		MarketScope:  "shanghai,shenzhen",
		ResultMode:   "top_n",
		ResultLimit:  50,
		GeneratedSQL: "SELECT symbol FROM v_stock_latest_daily ORDER BY change_percent DESC LIMIT 50",
		MatchedCount: 2,
		CreatedAt:    time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateScreeningRun() error = %v", err)
	}

	if err := store.ReplaceScreeningRunResults(runID, []ScreeningRunResult{
		{
			Symbol:            "600519.SH",
			Name:              "贵州茅台",
			Rank:              1,
			Score:             98.5,
			SnapshotTradeDate: "2026-03-18",
			Price:             1688.00,
			ChangePercent:     3.2,
			Volume:            1234567,
			Amount:            456789012,
		},
		{
			Symbol:            "000001.SZ",
			Name:              "平安银行",
			Rank:              2,
			Score:             88.1,
			SnapshotTradeDate: "2026-03-18",
			Price:             12.34,
			ChangePercent:     2.1,
			Volume:            2345678,
			Amount:            345678901,
		},
	}); err != nil {
		t.Fatalf("ReplaceScreeningRunResults() error = %v", err)
	}

	runs, err := store.ListScreeningRuns(10)
	if err != nil {
		t.Fatalf("ListScreeningRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(ListScreeningRuns()) = %d, want 1", len(runs))
	}
	if runs[0].ID != runID || runs[0].MatchedCount != 2 || runs[0].ResultLimit != 50 {
		t.Fatalf("ListScreeningRuns()[0] = %#v", runs[0])
	}

	results, err := store.ListScreeningRunResults(runID)
	if err != nil {
		t.Fatalf("ListScreeningRunResults() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(ListScreeningRunResults()) = %d, want 2", len(results))
	}
	if results[0].RunID != runID || results[0].Rank != 1 || results[1].Rank != 2 {
		t.Fatalf("ListScreeningRunResults() = %#v", results)
	}
}

func TestScreeningStoreListScreeningUniverseSymbolsUsesMarketScopeAndLimit(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	statements := []string{
		`INSERT INTO stocks_basic (symbol, name, market, industry, list_date, is_st, is_active) VALUES
			('sh600000', '浦发银行', '上海', '银行', '1999-11-10', 0, 1),
			('sh600519', '贵州茅台', '上海', '白酒', '2001-08-27', 0, 1),
			('sz000001', '平安银行', '深圳', '银行', '1991-04-03', 0, 1),
			('bj430047', '诺思兰德', '北交所', '医药', '2021-11-15', 0, 1)`,
		`INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount) VALUES
			('sh600000', '2026-03-19', 10, 11, 9.8, 11, 1000, 10000),
			('sh600519', '2026-03-19', 20, 21, 19.8, 21, 2000, 20000),
			('sz000001', '2026-03-19', 30, 31, 29.8, 31, 3000, 30000)`,
	}
	for _, statement := range statements {
		if _, err := store.db.Exec(statement); err != nil {
			t.Fatalf("seed screening universe error = %v", err)
		}
	}

	symbols, err := store.ListScreeningUniverseSymbols(models.ScreeningMarketScopeConfig{
		Shanghai: true,
		Shenzhen: true,
	}, 2)
	if err != nil {
		t.Fatalf("ListScreeningUniverseSymbols() error = %v", err)
	}

	if got, want := symbols, []string{"sh600000", "sh600519"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListScreeningUniverseSymbols() = %#v, want %#v", got, want)
	}
}

func TestScreeningStoreListSymbolsMissingDailyBarsOnTradeDate(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(`
		INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount) VALUES
			('sh600000', '2026-03-19', 10, 11, 9.5, 10.8, 1000, 10000),
			('sz000001', '2026-03-18', 20, 21, 19.5, 20.8, 2000, 20000)
	`); err != nil {
		t.Fatalf("seed daily_bars error = %v", err)
	}

	symbols, err := store.ListSymbolsMissingDailyBarsOnTradeDate([]string{"sh600000", "sz000001", "sz000002"}, "2026-03-19")
	if err != nil {
		t.Fatalf("ListSymbolsMissingDailyBarsOnTradeDate() error = %v", err)
	}

	if got, want := symbols, []string{"sz000001", "sz000002"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListSymbolsMissingDailyBarsOnTradeDate() = %#v, want %#v", got, want)
	}
}

func TestScreeningStoreGetLatestDailyBarTradeDatesForSymbols(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	defer store.Close()

	if _, err := store.db.Exec(`
		INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount) VALUES
			('sh600000', '2026-03-18', 10, 11, 9.5, 10.8, 1000, 10000),
			('sh600000', '2026-03-19', 11, 12, 10.5, 11.8, 1100, 11000),
			('sz000001', '2026-03-17', 20, 21, 19.5, 20.8, 2000, 20000)
	`); err != nil {
		t.Fatalf("seed daily_bars error = %v", err)
	}

	got, err := store.GetLatestDailyBarTradeDatesForSymbols([]string{"sh600000", "sz000001", "sz000002"})
	if err != nil {
		t.Fatalf("GetLatestDailyBarTradeDatesForSymbols() error = %v", err)
	}

	if got["sh600000"] != "2026-03-19" {
		t.Fatalf("got[sh600000] = %q, want 2026-03-19", got["sh600000"])
	}
	if got["sz000001"] != "2026-03-17" {
		t.Fatalf("got[sz000001] = %q, want 2026-03-17", got["sz000001"])
	}
	if _, exists := got["sz000002"]; exists {
		t.Fatalf("got[sz000002] = %q, want missing entry", got["sz000002"])
	}
}

func assertSQLiteObjectExists(t *testing.T, db *sql.DB, objectType, name string) {
	t.Helper()

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(1) FROM sqlite_master WHERE type = ? AND name = ?`,
		objectType,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s, %s) error = %v", objectType, name, err)
	}
	if count != 1 {
		t.Fatalf("sqlite object %s:%s count = %d, want 1", objectType, name, count)
	}
}
