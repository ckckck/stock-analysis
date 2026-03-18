package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/run-bigpig/jcp/internal/pkg/paths"
	_ "modernc.org/sqlite"
)

// ScreeningStore 管理 AI 筛选功能所需的本地 SQLite 存储。
type ScreeningStore struct {
	db     *sql.DB
	dbPath string
}

type ScreeningSyncState struct {
	Dataset       string
	MarketScope   string
	LastTradeDate string
	UpdatedAt     time.Time
}

type ScreeningRun struct {
	ID           int64
	Prompt       string
	MarketScope  string
	ResultMode   string
	ResultLimit  int
	GeneratedSQL string
	MatchedCount int
	CreatedAt    time.Time
}

type ScreeningRunResult struct {
	RunID             int64
	Symbol            string
	Name              string
	Rank              int
	Score             float64
	SnapshotTradeDate string
	Price             float64
	ChangePercent     float64
	Volume            float64
	Amount            float64
}

func NewScreeningStore(dataDir string) (*ScreeningStore, error) {
	dbPath := paths.GetScreeningDBPathFrom(dataDir)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create screening dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open screening db: %w", err)
	}

	store := &ScreeningStore{
		db:     db,
		dbPath: dbPath,
	}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *ScreeningStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *ScreeningStore) initSchema() error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS stocks_basic (
			symbol TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			market TEXT NOT NULL,
			industry TEXT,
			list_date TEXT,
			is_st INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS daily_bars (
			symbol TEXT NOT NULL,
			trade_date TEXT NOT NULL,
			open REAL NOT NULL,
			high REAL NOT NULL,
			low REAL NOT NULL,
			close REAL NOT NULL,
			volume REAL NOT NULL,
			amount REAL NOT NULL,
			PRIMARY KEY (symbol, trade_date)
		)`,
		`CREATE TABLE IF NOT EXISTS daily_snapshots (
			symbol TEXT NOT NULL,
			trade_date TEXT NOT NULL,
			change REAL NOT NULL,
			change_percent REAL NOT NULL,
			amplitude REAL NOT NULL DEFAULT 0,
			turnover_rate REAL NOT NULL DEFAULT 0,
			price REAL NOT NULL,
			PRIMARY KEY (symbol, trade_date)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			dataset TEXT NOT NULL,
			market_scope TEXT NOT NULL,
			last_trade_date TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (dataset, market_scope)
		)`,
		`CREATE TABLE IF NOT EXISTS screening_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			market_scope TEXT NOT NULL,
			result_mode TEXT NOT NULL,
			result_limit INTEGER NOT NULL,
			generated_sql TEXT NOT NULL,
			matched_count INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS screening_run_results (
			run_id INTEGER NOT NULL,
			symbol TEXT NOT NULL,
			name TEXT NOT NULL,
			rank INTEGER NOT NULL,
			score REAL NOT NULL DEFAULT 0,
			snapshot_trade_date TEXT,
			price REAL NOT NULL DEFAULT 0,
			change_percent REAL NOT NULL DEFAULT 0,
			volume REAL NOT NULL DEFAULT 0,
			amount REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (run_id, rank),
			FOREIGN KEY (run_id) REFERENCES screening_runs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stocks_basic_market ON stocks_basic (market)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_bars_symbol_trade_date ON daily_bars (symbol, trade_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_snapshots_symbol_trade_date ON daily_snapshots (symbol, trade_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_runs_created_at ON screening_runs (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_run_results_run_id_rank ON screening_run_results (run_id, rank)`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("init screening schema: %w", err)
		}
	}

	return nil
}

func (s *ScreeningStore) UpsertSyncState(state ScreeningSyncState) error {
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO sync_state (dataset, market_scope, last_trade_date, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(dataset, market_scope) DO UPDATE SET
		 	last_trade_date = excluded.last_trade_date,
		 	updated_at = excluded.updated_at`,
		state.Dataset,
		state.MarketScope,
		state.LastTradeDate,
		formatScreeningStoreTime(state.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert sync state: %w", err)
	}
	return nil
}

func (s *ScreeningStore) GetSyncState(dataset, marketScope string) (*ScreeningSyncState, error) {
	row := s.db.QueryRow(
		`SELECT dataset, market_scope, last_trade_date, updated_at
		 FROM sync_state
		 WHERE dataset = ? AND market_scope = ?`,
		dataset,
		marketScope,
	)

	var state ScreeningSyncState
	var updatedAt string
	if err := row.Scan(&state.Dataset, &state.MarketScope, &state.LastTradeDate, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get sync state: %w", err)
	}

	parsed, err := parseScreeningStoreTime(updatedAt)
	if err != nil {
		return nil, err
	}
	state.UpdatedAt = parsed
	return &state, nil
}

func (s *ScreeningStore) CreateScreeningRun(run ScreeningRun) (int64, error) {
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}

	result, err := s.db.Exec(
		`INSERT INTO screening_runs (
			prompt, market_scope, result_mode, result_limit, generated_sql, matched_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.Prompt,
		run.MarketScope,
		run.ResultMode,
		run.ResultLimit,
		run.GeneratedSQL,
		run.MatchedCount,
		formatScreeningStoreTime(run.CreatedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("create screening run: %w", err)
	}

	runID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read screening run id: %w", err)
	}
	return runID, nil
}

func (s *ScreeningStore) ReplaceScreeningRunResults(runID int64, results []ScreeningRunResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin screening result tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM screening_run_results WHERE run_id = ?`, runID); err != nil {
		return fmt.Errorf("clear screening results: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO screening_run_results (
			run_id, symbol, name, rank, score, snapshot_trade_date, price, change_percent, volume, amount
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare screening results insert: %w", err)
	}
	defer stmt.Close()

	for _, result := range results {
		if _, err = stmt.Exec(
			runID,
			result.Symbol,
			result.Name,
			result.Rank,
			result.Score,
			result.SnapshotTradeDate,
			result.Price,
			result.ChangePercent,
			result.Volume,
			result.Amount,
		); err != nil {
			return fmt.Errorf("insert screening result: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit screening result tx: %w", err)
	}
	return nil
}

func (s *ScreeningStore) ListScreeningRuns(limit int) ([]ScreeningRun, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, prompt, market_scope, result_mode, result_limit, generated_sql, matched_count, created_at
		 FROM screening_runs
		 ORDER BY created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list screening runs: %w", err)
	}
	defer rows.Close()

	var runs []ScreeningRun
	for rows.Next() {
		var run ScreeningRun
		var createdAt string
		if err := rows.Scan(
			&run.ID,
			&run.Prompt,
			&run.MarketScope,
			&run.ResultMode,
			&run.ResultLimit,
			&run.GeneratedSQL,
			&run.MatchedCount,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan screening run: %w", err)
		}
		run.CreatedAt, err = parseScreeningStoreTime(createdAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate screening runs: %w", err)
	}
	return runs, nil
}

func (s *ScreeningStore) ListScreeningRunResults(runID int64) ([]ScreeningRunResult, error) {
	rows, err := s.db.Query(
		`SELECT run_id, symbol, name, rank, score, snapshot_trade_date, price, change_percent, volume, amount
		 FROM screening_run_results
		 WHERE run_id = ?
		 ORDER BY rank ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list screening run results: %w", err)
	}
	defer rows.Close()

	var results []ScreeningRunResult
	for rows.Next() {
		var result ScreeningRunResult
		if err := rows.Scan(
			&result.RunID,
			&result.Symbol,
			&result.Name,
			&result.Rank,
			&result.Score,
			&result.SnapshotTradeDate,
			&result.Price,
			&result.ChangePercent,
			&result.Volume,
			&result.Amount,
		); err != nil {
			return nil, fmt.Errorf("scan screening result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate screening results: %w", err)
	}
	return results, nil
}

func formatScreeningStoreTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseScreeningStoreTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse screening store time %q: %w", raw, err)
	}
	return parsed, nil
}
