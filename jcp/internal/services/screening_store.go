package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
	_ "modernc.org/sqlite"
)

// ScreeningStore 管理 AI 筛选功能所需的本地 SQLite 存储。
type ScreeningStore struct {
	db     *sql.DB
	dbPath string
}

const screeningStoreSymbolChunkSize = 800

var screeningStoreLog = logger.New("screening.store")

type ScreeningSyncState struct {
	Dataset       string
	MarketScope   string
	LastTradeDate string
	UpdatedAt     time.Time
}

type ScreeningSyncJobState struct {
	Dataset             string
	MarketScope         string
	Status              string
	Mode                string
	LimitStocks         int
	TotalStocks         int
	CompletedStocks     int
	CurrentIndex        int
	CurrentSymbol       string
	CurrentName         string
	LastCompletedSymbol string
	ActiveSource        string
	LastMessage         string
	Error               string
	UpdatedAt           time.Time
}

type ScreeningRun struct {
	ID              int64
	Prompt          string
	MarketScope     string
	ResultMode      string
	ResultLimit     int
	UniverseSymbols []string
	GeneratedSQL    string
	MatchedCount    int
	CreatedAt       time.Time
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

func NewScreeningStore(dataDir ...string) (*ScreeningStore, error) {
	dbPath := paths.GetScreeningDBPathFrom(screeningStoreDataDir(dataDir...))
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create screening dir: %w", err)
	}

	values := url.Values{}
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	values.Add("_pragma", "foreign_keys(ON)")
	values.Add("_pragma", "synchronous(NORMAL)")
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     dbPath,
		RawQuery: values.Encode(),
	}).String()

	db, err := sql.Open("sqlite", dsn)
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

	screeningStoreLog.Info("open screening store: path=%s journal_mode=WAL busy_timeout=5000", dbPath)

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
		`CREATE TABLE IF NOT EXISTS sync_jobs (
			dataset TEXT NOT NULL,
			market_scope TEXT NOT NULL,
			status TEXT NOT NULL,
			mode TEXT NOT NULL,
			limit_stocks INTEGER NOT NULL DEFAULT 0,
			total_stocks INTEGER NOT NULL DEFAULT 0,
			completed_stocks INTEGER NOT NULL DEFAULT 0,
			current_index INTEGER NOT NULL DEFAULT 0,
			current_symbol TEXT NOT NULL DEFAULT '',
			current_name TEXT NOT NULL DEFAULT '',
			last_completed_symbol TEXT NOT NULL DEFAULT '',
			active_source TEXT NOT NULL DEFAULT '',
			last_message TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
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
		`CREATE INDEX IF NOT EXISTS idx_sync_jobs_status_updated_at ON sync_jobs (status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_runs_created_at ON screening_runs (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_run_results_run_id_rank ON screening_run_results (run_id, rank)`,
		`CREATE VIEW IF NOT EXISTS v_stock_latest_daily AS
			WITH latest_bar AS (
				SELECT symbol, MAX(trade_date) AS trade_date
				FROM daily_bars
				GROUP BY symbol
			),
			latest_snapshot AS (
				SELECT symbol, MAX(trade_date) AS trade_date
				FROM daily_snapshots
				GROUP BY symbol
			)
			SELECT
				sb.symbol,
				sb.name,
				sb.market,
				sb.industry,
				sb.list_date,
				sb.is_st,
				sb.is_active,
				db.trade_date AS snapshot_trade_date,
				db.open,
				db.high,
				db.low,
				db.close,
				db.volume,
				db.amount,
				COALESCE(ds.price, db.close) AS price,
				COALESCE(ds.change, 0) AS change,
				COALESCE(ds.change_percent, 0) AS change_percent,
				COALESCE(ds.amplitude, 0) AS amplitude,
				COALESCE(ds.turnover_rate, 0) AS turnover_rate
			FROM stocks_basic sb
			JOIN latest_bar lb
				ON lb.symbol = sb.symbol
			JOIN daily_bars db
				ON db.symbol = lb.symbol
				AND db.trade_date = lb.trade_date
			LEFT JOIN latest_snapshot ls
				ON ls.symbol = sb.symbol
			LEFT JOIN daily_snapshots ds
				ON ds.symbol = ls.symbol
				AND ds.trade_date = ls.trade_date`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("init screening schema: %w", err)
		}
	}

	if err := s.ensureScreeningRunsUniverseSymbolsColumn(); err != nil {
		return err
	}

	return nil
}

func (s *ScreeningStore) ensureScreeningRunsUniverseSymbolsColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(screening_runs)`)
	if err != nil {
		return fmt.Errorf("inspect screening_runs columns: %w", err)
	}
	defer rows.Close()

	var hasUniverseSymbols bool
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan screening_runs column info: %w", err)
		}
		if strings.EqualFold(name, "universe_symbols") {
			hasUniverseSymbols = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate screening_runs columns: %w", err)
	}
	if hasUniverseSymbols {
		return nil
	}

	if _, err := s.db.Exec(`ALTER TABLE screening_runs ADD COLUMN universe_symbols TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add screening_runs.universe_symbols: %w", err)
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

func (s *ScreeningStore) UpsertSyncJobState(state ScreeningSyncJobState) error {
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO sync_jobs (
			dataset, market_scope, status, mode, limit_stocks, total_stocks, completed_stocks,
			current_index, current_symbol, current_name, last_completed_symbol, active_source,
			last_message, error, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dataset, market_scope) DO UPDATE SET
			status = excluded.status,
			mode = excluded.mode,
			limit_stocks = excluded.limit_stocks,
			total_stocks = excluded.total_stocks,
			completed_stocks = excluded.completed_stocks,
			current_index = excluded.current_index,
			current_symbol = excluded.current_symbol,
			current_name = excluded.current_name,
			last_completed_symbol = excluded.last_completed_symbol,
			active_source = excluded.active_source,
			last_message = excluded.last_message,
			error = excluded.error,
			updated_at = excluded.updated_at`,
		state.Dataset,
		state.MarketScope,
		state.Status,
		state.Mode,
		state.LimitStocks,
		state.TotalStocks,
		state.CompletedStocks,
		state.CurrentIndex,
		state.CurrentSymbol,
		state.CurrentName,
		state.LastCompletedSymbol,
		state.ActiveSource,
		state.LastMessage,
		state.Error,
		formatScreeningStoreTime(state.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert sync job state: %w", err)
	}
	return nil
}

func (s *ScreeningStore) GetSyncJobState(dataset, marketScope string) (*ScreeningSyncJobState, error) {
	row := s.db.QueryRow(
		`SELECT dataset, market_scope, status, mode, limit_stocks, total_stocks, completed_stocks,
		        current_index, current_symbol, current_name, last_completed_symbol, active_source,
		        last_message, error, updated_at
		 FROM sync_jobs
		 WHERE dataset = ? AND market_scope = ?`,
		dataset,
		marketScope,
	)

	var state ScreeningSyncJobState
	var updatedAt string
	if err := row.Scan(
		&state.Dataset,
		&state.MarketScope,
		&state.Status,
		&state.Mode,
		&state.LimitStocks,
		&state.TotalStocks,
		&state.CompletedStocks,
		&state.CurrentIndex,
		&state.CurrentSymbol,
		&state.CurrentName,
		&state.LastCompletedSymbol,
		&state.ActiveSource,
		&state.LastMessage,
		&state.Error,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get sync job state: %w", err)
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
			prompt, market_scope, result_mode, result_limit, universe_symbols, generated_sql, matched_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.Prompt,
		run.MarketScope,
		run.ResultMode,
		run.ResultLimit,
		marshalScreeningUniverseSymbols(run.UniverseSymbols),
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
		`SELECT id, prompt, market_scope, result_mode, result_limit, universe_symbols, generated_sql, matched_count, created_at
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
		var universeSymbols string
		var createdAt string
		if err := rows.Scan(
			&run.ID,
			&run.Prompt,
			&run.MarketScope,
			&run.ResultMode,
			&run.ResultLimit,
			&universeSymbols,
			&run.GeneratedSQL,
			&run.MatchedCount,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan screening run: %w", err)
		}
		run.UniverseSymbols = unmarshalScreeningUniverseSymbols(universeSymbols)
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

func (s *ScreeningStore) GetScreeningRun(runID int64) (*ScreeningRun, error) {
	row := s.db.QueryRow(
		`SELECT id, prompt, market_scope, result_mode, result_limit, universe_symbols, generated_sql, matched_count, created_at
		 FROM screening_runs
		 WHERE id = ?`,
		runID,
	)

	var run ScreeningRun
	var universeSymbols string
	var createdAt string
	if err := row.Scan(
		&run.ID,
		&run.Prompt,
		&run.MarketScope,
		&run.ResultMode,
		&run.ResultLimit,
		&universeSymbols,
		&run.GeneratedSQL,
		&run.MatchedCount,
		&createdAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get screening run: %w", err)
	}

	parsed, err := parseScreeningStoreTime(createdAt)
	if err != nil {
		return nil, err
	}
	run.UniverseSymbols = unmarshalScreeningUniverseSymbols(universeSymbols)
	run.CreatedAt = parsed
	return &run, nil
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

func (s *ScreeningStore) ListScreeningUniverseSymbols(scopes models.ScreeningMarketScopeConfig, limit int) ([]string, error) {
	query := `
		SELECT DISTINCT sb.symbol
		FROM stocks_basic sb
		WHERE EXISTS (
			SELECT 1
			FROM daily_bars db
			WHERE db.symbol = sb.symbol
		)
	`
	args := make([]any, 0, 5)
	markets := screeningStoreMarketsFromScope(scopes)
	if len(markets) > 0 {
		placeholders := make([]string, 0, len(markets))
		for _, market := range markets {
			placeholders = append(placeholders, "?")
			args = append(args, market)
		}
		query += fmt.Sprintf(" AND sb.market IN (%s)", strings.Join(placeholders, ", "))
	}
	query += " ORDER BY sb.symbol ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list screening universe symbols: %w", err)
	}
	defer rows.Close()

	symbols := make([]string, 0, maxInt(limit, 16))
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("scan screening universe symbol: %w", err)
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate screening universe symbols: %w", err)
	}
	return symbols, nil
}

func (s *ScreeningStore) ListSymbolsMissingDailyBarsOnTradeDate(candidateSymbols []string, tradeDate string) ([]string, error) {
	symbols := normalizeScreeningUniverseSymbols(candidateSymbols)
	if len(symbols) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(tradeDate) == "" {
		return append([]string(nil), symbols...), nil
	}

	existing := make(map[string]struct{}, len(symbols))
	for _, chunk := range chunkScreeningSymbols(symbols, screeningStoreSymbolChunkSize) {
		placeholders := screeningStorePlaceholders(len(chunk))
		args := make([]any, 0, len(chunk)+1)
		args = append(args, tradeDate)
		for _, symbol := range chunk {
			args = append(args, symbol)
		}

		rows, err := s.db.Query(
			fmt.Sprintf(
				`SELECT symbol FROM daily_bars WHERE trade_date = ? AND symbol IN (%s)`,
				placeholders,
			),
			args...,
		)
		if err != nil {
			return nil, fmt.Errorf("list symbols with daily bars on trade date: %w", err)
		}

		for rows.Next() {
			var symbol string
			if err := rows.Scan(&symbol); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan symbol with daily bars on trade date: %w", err)
			}
			existing[symbol] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate symbols with daily bars on trade date: %w", err)
		}
		rows.Close()
	}

	missing := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if _, ok := existing[symbol]; ok {
			continue
		}
		missing = append(missing, symbol)
	}
	return missing, nil
}

func (s *ScreeningStore) GetLatestDailyBarTradeDatesForSymbols(candidateSymbols []string) (map[string]string, error) {
	symbols := normalizeScreeningUniverseSymbols(candidateSymbols)
	if len(symbols) == 0 {
		return map[string]string{}, nil
	}

	latestDates := make(map[string]string, len(symbols))
	for _, chunk := range chunkScreeningSymbols(symbols, screeningStoreSymbolChunkSize) {
		placeholders := screeningStorePlaceholders(len(chunk))
		args := make([]any, 0, len(chunk))
		for _, symbol := range chunk {
			args = append(args, symbol)
		}

		rows, err := s.db.Query(
			fmt.Sprintf(
				`SELECT symbol, MAX(trade_date) FROM daily_bars WHERE symbol IN (%s) GROUP BY symbol`,
				placeholders,
			),
			args...,
		)
		if err != nil {
			return nil, fmt.Errorf("get latest daily bar trade dates: %w", err)
		}

		for rows.Next() {
			var symbol string
			var tradeDate string
			if err := rows.Scan(&symbol, &tradeDate); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan latest daily bar trade date: %w", err)
			}
			latestDates[symbol] = tradeDate
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate latest daily bar trade dates: %w", err)
		}
		rows.Close()
	}
	return latestDates, nil
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

func screeningStoreDataDir(dataDir ...string) string {
	if len(dataDir) > 0 && dataDir[0] != "" {
		return dataDir[0]
	}
	return paths.GetDataDir()
}

func marshalScreeningUniverseSymbols(symbols []string) string {
	if len(symbols) == 0 {
		return ""
	}
	encoded, err := json.Marshal(symbols)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func unmarshalScreeningUniverseSymbols(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var symbols []string
	if err := json.Unmarshal([]byte(raw), &symbols); err != nil {
		return nil
	}
	return normalizeScreeningUniverseSymbols(symbols)
}

func screeningStoreMarketsFromScope(scopes models.ScreeningMarketScopeConfig) []string {
	markets := make([]string, 0, 4)
	if scopes.Shanghai {
		markets = append(markets, "上海")
	}
	if scopes.Shenzhen {
		markets = append(markets, "深圳")
	}
	if scopes.Beijing {
		markets = append(markets, "北交所")
	}
	if scopes.Indices {
		markets = append(markets, "指数")
	}
	sort.Strings(markets)
	return markets
}

func chunkScreeningSymbols(symbols []string, chunkSize int) [][]string {
	if len(symbols) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = screeningStoreSymbolChunkSize
	}

	chunks := make([][]string, 0, (len(symbols)+chunkSize-1)/chunkSize)
	for start := 0; start < len(symbols); start += chunkSize {
		end := start + chunkSize
		if end > len(symbols) {
			end = len(symbols)
		}
		chunks = append(chunks, symbols[start:end])
	}
	return chunks
}

func screeningStorePlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	placeholders := make([]string, 0, count)
	for i := 0; i < count; i++ {
		placeholders = append(placeholders, "?")
	}
	return strings.Join(placeholders, ", ")
}
