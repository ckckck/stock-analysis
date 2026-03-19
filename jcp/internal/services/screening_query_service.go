package services

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

var (
	screeningForbiddenSQLPattern = regexp.MustCompile(`(?is)\b(INSERT|UPDATE|DELETE|DROP|ALTER|ATTACH|PRAGMA|CREATE|REPLACE|VACUUM|TRUNCATE)\b`)
	screeningOrderByPattern      = regexp.MustCompile(`(?is)\bORDER\s+BY\b`)
	screeningLimitPattern        = regexp.MustCompile(`(?is)\bLIMIT\s+\d+\b`)
	screeningSourcePattern       = regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	screeningCTEPattern          = regexp.MustCompile(`(?is)(?:WITH|,)\s*([A-Za-z_][A-Za-z0-9_]*)\s+AS\s*\(`)
)

var screeningAllowedSources = map[string]struct{}{
	"stocks_basic":         {},
	"daily_bars":           {},
	"daily_snapshots":      {},
	"v_stock_latest_daily": {},
}

type ScreeningResultMode string

const (
	ScreeningResultModeUnlimited ScreeningResultMode = "unlimited"
	ScreeningResultModeTopN      ScreeningResultMode = "top_n"
)

type ScreeningQueryRequest struct {
	Prompt      string              `json:"prompt"`
	AIConfigID  string              `json:"aiConfigId,omitempty"`
	ResultMode  ScreeningResultMode `json:"resultMode"`
	ResultLimit int                 `json:"resultLimit"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"pageSize"`
}

type ScreeningQueryResponse struct {
	RunID        int64                `json:"runId"`
	Prompt       string               `json:"prompt,omitempty"`
	MarketScope  string               `json:"marketScope"`
	ResultMode   string               `json:"resultMode"`
	ResultLimit  int                  `json:"resultLimit"`
	GeneratedSQL string               `json:"generatedSql"`
	TotalCount   int                  `json:"totalCount"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"pageSize"`
	CreatedAt    string               `json:"createdAt,omitempty"`
	Results      []ScreeningRunResult `json:"results"`
	Error        string               `json:"error,omitempty"`
}

type ScreeningHistoryItem struct {
	RunID        int64  `json:"runId"`
	Prompt       string `json:"prompt"`
	MarketScope  string `json:"marketScope"`
	ResultMode   string `json:"resultMode"`
	ResultLimit  int    `json:"resultLimit"`
	MatchedCount int    `json:"matchedCount"`
	CreatedAt    string `json:"createdAt"`
}

type ScreeningHistoryResponse struct {
	Items []ScreeningHistoryItem `json:"items"`
	Error string                 `json:"error,omitempty"`
}

type screeningSQLGenerator interface {
	GenerateSQL(ctx context.Context, prompt string, aiConfigID string) (string, error)
}

// ScreeningQueryService 负责将自然语言转换为只读 SQL 并执行筛选。
type ScreeningQueryService struct {
	configService *ConfigService
	store         *ScreeningStore
	generator     screeningSQLGenerator
	now           func() time.Time
}

func NewScreeningQueryService(
	configService *ConfigService,
	store *ScreeningStore,
	generator screeningSQLGenerator,
) *ScreeningQueryService {
	return &ScreeningQueryService{
		configService: configService,
		store:         store,
		generator:     generator,
		now:           time.Now,
	}
}

func (s *ScreeningQueryService) Run(ctx context.Context, req ScreeningQueryRequest) (*ScreeningQueryResponse, error) {
	if s == nil || s.configService == nil || s.store == nil || s.generator == nil {
		return nil, fmt.Errorf("screening query service not initialized")
	}

	cfg := s.configService.GetConfig().Screening
	resultLimit := req.ResultLimit
	if req.ResultMode == ScreeningResultModeTopN && resultLimit <= 0 {
		resultLimit = cfg.DefaultResultLimit
	}
	page := normalizePositive(req.Page, 1)
	pageSize := normalizePageSize(req.PageSize)
	marketScope := screeningMarketScopeKey(cfg.Markets)

	prompt := s.buildPrompt(ScreeningQueryRequest{
		Prompt:      req.Prompt,
		AIConfigID:  req.AIConfigID,
		ResultMode:  req.ResultMode,
		ResultLimit: resultLimit,
		Page:        page,
		PageSize:    pageSize,
	})

	queryCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	rawSQL, err := s.generator.GenerateSQL(queryCtx, prompt, req.AIConfigID)
	if err != nil {
		return nil, fmt.Errorf("generate screening sql: %w", err)
	}

	validSQL, err := validateScreeningSQL(rawSQL, req.ResultMode, resultLimit)
	if err != nil {
		return nil, err
	}

	var allResults []ScreeningRunResult
	totalCount := 0
	switch req.ResultMode {
	case ScreeningResultModeUnlimited:
		totalCount, err = s.countMatches(queryCtx, validSQL)
		if err != nil {
			return nil, err
		}
		allResults, err = s.queryResults(queryCtx, validSQL)
	default:
		allResults, err = s.queryResults(queryCtx, validSQL)
		totalCount = len(allResults)
	}
	if err != nil {
		return nil, err
	}

	for index := range allResults {
		allResults[index].Rank = index + 1
	}

	runID, err := s.store.CreateScreeningRun(ScreeningRun{
		Prompt:       req.Prompt,
		MarketScope:  marketScope,
		ResultMode:   string(req.ResultMode),
		ResultLimit:  resultLimit,
		GeneratedSQL: validSQL,
		MatchedCount: totalCount,
		CreatedAt:    s.now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	for index := range allResults {
		allResults[index].RunID = runID
	}
	if err := s.store.ReplaceScreeningRunResults(runID, allResults); err != nil {
		return nil, err
	}

	return &ScreeningQueryResponse{
		RunID:        runID,
		Prompt:       req.Prompt,
		MarketScope:  marketScope,
		ResultMode:   string(req.ResultMode),
		ResultLimit:  resultLimit,
		GeneratedSQL: validSQL,
		TotalCount:   totalCount,
		Page:         page,
		PageSize:     pageSize,
		CreatedAt:    formatScreeningStoreTime(s.now().UTC()),
		Results:      paginateScreeningResults(allResults, page, pageSize),
	}, nil
}

func (s *ScreeningQueryService) ListHistory(limit int) ([]ScreeningHistoryItem, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("screening query service not initialized")
	}

	runs, err := s.store.ListScreeningRuns(limit)
	if err != nil {
		return nil, err
	}

	items := make([]ScreeningHistoryItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, ScreeningHistoryItem{
			RunID:        run.ID,
			Prompt:       run.Prompt,
			MarketScope:  run.MarketScope,
			ResultMode:   run.ResultMode,
			ResultLimit:  run.ResultLimit,
			MatchedCount: run.MatchedCount,
			CreatedAt:    formatScreeningStoreTime(run.CreatedAt),
		})
	}
	return items, nil
}

func (s *ScreeningQueryService) GetRun(runID int64, page, pageSize int) (*ScreeningQueryResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("screening query service not initialized")
	}

	run, err := s.store.GetScreeningRun(runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, sql.ErrNoRows
	}

	results, err := s.store.ListScreeningRunResults(runID)
	if err != nil {
		return nil, err
	}

	page = normalizePositive(page, 1)
	pageSize = normalizePageSize(pageSize)

	return &ScreeningQueryResponse{
		RunID:        run.ID,
		Prompt:       run.Prompt,
		MarketScope:  run.MarketScope,
		ResultMode:   run.ResultMode,
		ResultLimit:  run.ResultLimit,
		GeneratedSQL: run.GeneratedSQL,
		TotalCount:   run.MatchedCount,
		Page:         page,
		PageSize:     pageSize,
		CreatedAt:    formatScreeningStoreTime(run.CreatedAt),
		Results:      paginateScreeningResults(results, page, pageSize),
	}, nil
}

func (s *ScreeningQueryService) buildPrompt(req ScreeningQueryRequest) string {
	cfg := s.configService.GetConfig().Screening

	var builder strings.Builder
	builder.WriteString("你是A股筛选SQL生成器。请把用户的自然语言筛选条件转换成一条可在 SQLite 执行的只读 SQL。\n\n")
	builder.WriteString("## 固定约束\n")
	builder.WriteString("- 只支持日线数据。\n")
	builder.WriteString("- 只能生成一条 SQL。\n")
	builder.WriteString("- 只能使用白名单表/视图。\n")
	builder.WriteString("- 最终 SQL 只能是 SELECT 或 WITH ... SELECT。\n")
	builder.WriteString("- 最终 SELECT 必须返回以下列，且顺序固定：symbol, name, score, snapshot_trade_date, price, change_percent, volume, amount。\n")
	builder.WriteString("- 如果没有自然评分，请使用一个可排序的数值列并命名为 score。\n")
	builder.WriteString("- SQL 中必须包含 ORDER BY。\n\n")

	builder.WriteString("## 当前市场范围\n")
	builder.WriteString(screeningMarketScopeDescription(cfg.Markets))
	builder.WriteString("\n\n")

	builder.WriteString("## 白名单表/视图\n")
	builder.WriteString("- v_stock_latest_daily(symbol, name, market, industry, list_date, is_st, is_active, snapshot_trade_date, open, high, low, close, volume, amount, price, change, change_percent, amplitude, turnover_rate)\n")
	builder.WriteString("- stocks_basic(symbol, name, market, industry, list_date, is_st, is_active)\n")
	builder.WriteString("- daily_bars(symbol, trade_date, open, high, low, close, volume, amount)\n")
	builder.WriteString("- daily_snapshots(symbol, trade_date, change, change_percent, amplitude, turnover_rate, price)\n\n")

	builder.WriteString("## 结果模式\n")
	switch req.ResultMode {
	case ScreeningResultModeTopN:
		fmt.Fprintf(&builder, "- 当前模式为前 N 条。SQL 必须以 LIMIT %d 结束。\n", req.ResultLimit)
	default:
		builder.WriteString("- 当前模式为不限。SQL 不允许出现 LIMIT。\n")
	}
	builder.WriteString("\n")

	builder.WriteString("## 只输出 SQL\n")
	builder.WriteString("不要输出解释、不要输出 Markdown、不要输出代码块。\n\n")
	builder.WriteString("## 用户条件\n")
	builder.WriteString(strings.TrimSpace(req.Prompt))
	return builder.String()
}

func (s *ScreeningQueryService) countMatches(ctx context.Context, sqlText string) (int, error) {
	row := s.store.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM (`+sqlText+`) AS screening_candidates`)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count screening results: %w", err)
	}
	return count, nil
}

func (s *ScreeningQueryService) queryResults(ctx context.Context, sqlText string) ([]ScreeningRunResult, error) {
	rows, err := s.store.db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("query screening results: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read screening columns: %w", err)
	}
	if !matchScreeningColumns(columns) {
		return nil, fmt.Errorf("screening sql must return columns: symbol, name, score, snapshot_trade_date, price, change_percent, volume, amount")
	}

	results := make([]ScreeningRunResult, 0)
	for rows.Next() {
		var result ScreeningRunResult
		var snapshotTradeDate sql.NullString
		if err := rows.Scan(
			&result.Symbol,
			&result.Name,
			&result.Score,
			&snapshotTradeDate,
			&result.Price,
			&result.ChangePercent,
			&result.Volume,
			&result.Amount,
		); err != nil {
			return nil, fmt.Errorf("scan screening result: %w", err)
		}
		if snapshotTradeDate.Valid {
			result.SnapshotTradeDate = snapshotTradeDate.String
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate screening results: %w", err)
	}

	return results, nil
}

func validateScreeningSQL(rawSQL string, mode ScreeningResultMode, resultLimit int) (string, error) {
	sqlText := normalizeScreeningSQL(rawSQL)
	if sqlText == "" {
		return "", fmt.Errorf("screening sql is empty")
	}
	if strings.Contains(sqlText, "--") || strings.Contains(sqlText, "/*") {
		return "", fmt.Errorf("screening sql cannot contain comments")
	}
	if strings.Count(sqlText, ";") > 0 {
		return "", fmt.Errorf("screening sql must be a single statement")
	}

	upperSQL := strings.ToUpper(sqlText)
	if !strings.HasPrefix(upperSQL, "SELECT") && !strings.HasPrefix(upperSQL, "WITH") {
		return "", fmt.Errorf("screening sql must start with SELECT or WITH")
	}
	if screeningForbiddenSQLPattern.MatchString(sqlText) {
		return "", fmt.Errorf("screening sql contains forbidden statements")
	}
	if !screeningOrderByPattern.MatchString(sqlText) {
		return "", fmt.Errorf("screening sql must include ORDER BY")
	}

	switch mode {
	case ScreeningResultModeTopN:
		if resultLimit <= 0 {
			return "", fmt.Errorf("screening sql limit mode requires result limit")
		}
		expectedLimit := regexp.MustCompile(fmt.Sprintf(`(?is)\bLIMIT\s+%d\s*$`, resultLimit))
		if !expectedLimit.MatchString(sqlText) {
			return "", fmt.Errorf("screening sql must end with LIMIT %d", resultLimit)
		}
	default:
		if screeningLimitPattern.MatchString(sqlText) {
			return "", fmt.Errorf("screening sql unlimited mode cannot include LIMIT")
		}
	}

	allowedSources := make(map[string]struct{}, len(screeningAllowedSources))
	for key := range screeningAllowedSources {
		allowedSources[key] = struct{}{}
	}
	for _, cteName := range screeningCTEPattern.FindAllStringSubmatch(sqlText, -1) {
		if len(cteName) > 1 {
			allowedSources[strings.ToLower(cteName[1])] = struct{}{}
		}
	}

	sourceMatches := screeningSourcePattern.FindAllStringSubmatch(sqlText, -1)
	if len(sourceMatches) == 0 {
		return "", fmt.Errorf("screening sql must reference allowed tables or views")
	}
	for _, match := range sourceMatches {
		if len(match) < 2 {
			continue
		}
		if _, ok := allowedSources[strings.ToLower(match[1])]; !ok {
			return "", fmt.Errorf("screening sql references non-whitelisted source %q", match[1])
		}
	}

	return sqlText, nil
}

func normalizeScreeningSQL(rawSQL string) string {
	trimmed := strings.TrimSpace(rawSQL)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			trimmed = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
}

func matchScreeningColumns(columns []string) bool {
	expected := []string{"symbol", "name", "score", "snapshot_trade_date", "price", "change_percent", "volume", "amount"}
	if len(columns) != len(expected) {
		return false
	}
	for index, column := range columns {
		if !strings.EqualFold(column, expected[index]) {
			return false
		}
	}
	return true
}

func paginateScreeningResults(results []ScreeningRunResult, page, pageSize int) []ScreeningRunResult {
	if len(results) == 0 {
		return nil
	}

	page = normalizePositive(page, 1)
	pageSize = normalizePageSize(pageSize)

	start := (page - 1) * pageSize
	if start >= len(results) {
		return []ScreeningRunResult{}
	}

	end := minInt(start+pageSize, len(results))
	return append([]ScreeningRunResult(nil), results[start:end]...)
}

func normalizePositive(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func normalizePageSize(pageSize int) int {
	pageSize = normalizePositive(pageSize, 50)
	if pageSize > 200 {
		return 200
	}
	return pageSize
}

func screeningMarketScopeDescription(scopes models.ScreeningMarketScopeConfig) string {
	parts := make([]string, 0, 4)
	if scopes.Shanghai {
		parts = append(parts, "沪市")
	}
	if scopes.Shenzhen {
		parts = append(parts, "深市")
	}
	if scopes.Beijing {
		parts = append(parts, "北交所")
	}
	if scopes.Indices {
		parts = append(parts, "指数")
	}
	if len(parts) == 0 {
		return "未启用任何市场范围"
	}
	return strings.Join(parts, "、")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
