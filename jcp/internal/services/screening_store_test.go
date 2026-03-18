package services

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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
		"screening_runs",
		"screening_run_results",
	} {
		assertSQLiteObjectExists(t, store.db, "table", tableName)
	}

	for _, indexName := range []string{
		"idx_stocks_basic_market",
		"idx_daily_bars_symbol_trade_date",
		"idx_daily_snapshots_symbol_trade_date",
		"idx_screening_runs_created_at",
		"idx_screening_run_results_run_id_rank",
	} {
		assertSQLiteObjectExists(t, store.db, "index", indexName)
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
