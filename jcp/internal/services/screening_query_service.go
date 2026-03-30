package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"
)

var (
	screeningForbiddenSQLPattern = regexp.MustCompile(`(?is)\b(INSERT|UPDATE|DELETE|DROP|ALTER|ATTACH|PRAGMA|CREATE|REPLACE|VACUUM|TRUNCATE)\b`)
	screeningOrderByPattern      = regexp.MustCompile(`(?is)\bORDER\s+BY\b`)
	screeningLimitPattern        = regexp.MustCompile(`(?is)\bLIMIT\s+\d+\b`)
	screeningSourcePattern       = regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	screeningCTEPattern          = regexp.MustCompile(`(?is)(?:WITH|,)\s*([A-Za-z_][A-Za-z0-9_]*)\s+AS\s*\(`)
	screeningSQLLiteralReplacer  = strings.NewReplacer(
		"'SH'", "'上海'",
		"'sh'", "'上海'",
		`"SH"`, `"上海"`,
		`"sh"`, `"上海"`,
		"'SZ'", "'深圳'",
		"'sz'", "'深圳'",
		`"SZ"`, `"深圳"`,
		`"sz"`, `"深圳"`,
		"'BJ'", "'北京'",
		"'bj'", "'北京'",
		`"BJ"`, `"北京"`,
		`"bj"`, `"北京"`,
		"'沪市'", "'上海'",
		`"沪市"`, `"上海"`,
		"'深市'", "'深圳'",
		`"深市"`, `"深圳"`,
		"'北交所'", "'北京'",
		`"北交所"`, `"北京"`,
		"'上交所'", "'上海'",
		`"上交所"`, `"上海"`,
		"'深交所'", "'深圳'",
		`"深交所"`, `"深圳"`,
		"'北京证券交易所'", "'北京'",
		`"北京证券交易所"`, `"北京"`,
		"'上海证券交易所'", "'上海'",
		`"上海证券交易所"`, `"上海"`,
		"'深圳证券交易所'", "'深圳'",
		`"深圳证券交易所"`, `"深圳"`,
	)
)

var screeningQueryLog = logger.New("screening.query")

var screeningAllowedSources = map[string]struct{}{
	"stocks_basic":         {},
	"daily_bars":           {},
	"daily_snapshots":      {},
	"v_stock_latest_daily": {},
}

var screeningAllowedOrderByColumns = map[string]struct{}{
	"symbol":             {},
	"name":               {},
	"score":              {},
	"snapshot_trade_date": {},
	"price":              {},
	"change_percent":     {},
	"volume":             {},
	"amount":             {},
}

type ScreeningResultMode string

const (
	ScreeningResultModeUnlimited ScreeningResultMode = "unlimited"
	ScreeningResultModeTopN      ScreeningResultMode = "top_n"
)

type ScreeningQueryRequest struct {
	Prompt          string              `json:"prompt"`
	AIConfigID      string              `json:"aiConfigId,omitempty"`
	ResultMode      ScreeningResultMode `json:"resultMode"`
	ResultLimit     int                 `json:"resultLimit"`
	Page            int                 `json:"page"`
	PageSize        int                 `json:"pageSize"`
	UniverseSymbols []string            `json:"universeSymbols,omitempty"`
}

type ScreeningQueryResponse struct {
	RunID           int64                `json:"runId"`
	Prompt          string               `json:"prompt,omitempty"`
	MarketScope     string               `json:"marketScope"`
	ResultMode      string               `json:"resultMode"`
	ResultLimit     int                  `json:"resultLimit"`
	UniverseSymbols []string             `json:"universeSymbols,omitempty"`
	GeneratedSQL    string               `json:"generatedSql"`
	TotalCount      int                  `json:"totalCount"`
	Page            int                  `json:"page"`
	PageSize        int                  `json:"pageSize"`
	CreatedAt       string               `json:"createdAt,omitempty"`
	Results         []ScreeningRunResult `json:"results"`
	Error           string               `json:"error,omitempty"`
}

type ScreeningQueryLog struct {
	Time    string `json:"time"`
	Stage   string `json:"stage"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ScreeningQueryProgress struct {
	RunStatus       string              `json:"runStatus"`
	CurrentStage    string              `json:"currentStage"`
	ProgressPercent float64             `json:"progressPercent"`
	Message         string              `json:"message"`
	StreamingText   string              `json:"streamingText,omitempty"`
	Prompt          string              `json:"prompt"`
	UniverseCount   int                 `json:"universeCount,omitempty"`
	Logs            []ScreeningQueryLog `json:"logs"`
	Error           string              `json:"error,omitempty"`
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

type screeningStreamingSQLGenerator interface {
	GenerateSQLStream(ctx context.Context, prompt string, aiConfigID string, onDelta func(string)) (string, error)
}

type screeningStreamingReasoningGenerator interface {
	GenerateReasoningStream(ctx context.Context, prompt string, aiConfigID string, onDelta func(string)) (string, error)
}

// ScreeningQueryService 负责将自然语言转换为只读 SQL 并执行筛选。
type ScreeningQueryService struct {
	configService *ConfigService
	store         *ScreeningStore
	generator     screeningSQLGenerator
	now           func() time.Time
}

const defaultScreeningQueryExecutionTimeout = 45 * time.Second

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
	return s.RunWithProgress(ctx, req, nil)
}

func (s *ScreeningQueryService) RunWithProgress(
	ctx context.Context,
	req ScreeningQueryRequest,
	report func(ScreeningQueryProgress),
) (*ScreeningQueryResponse, error) {
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
	universeSymbols := normalizeScreeningUniverseSymbols(req.UniverseSymbols)
	progressState := ScreeningQueryProgress{
		RunStatus:     "running",
		CurrentStage:  "prepare",
		Prompt:        req.Prompt,
		UniverseCount: len(universeSymbols),
		Logs:          make([]ScreeningQueryLog, 0, 8),
	}
	reportProgress := func() {
		if report == nil {
			return
		}
		cloned := progressState
		cloned.Logs = append([]ScreeningQueryLog(nil), progressState.Logs...)
		report(cloned)
	}
	emitProgress := func(stage string, percent float64, message string, err error) {
		progressState.CurrentStage = stage
		progressState.ProgressPercent = percent
		progressState.Message = message
		if stage != "generate_sql" && stage != "reasoning" {
			progressState.StreamingText = ""
		}
		if err != nil {
			progressState.RunStatus = "failed"
			progressState.Error = err.Error()
		} else if stage == "completed" {
			progressState.RunStatus = "completed"
			progressState.Error = ""
		} else {
			progressState.RunStatus = "running"
			progressState.Error = ""
		}
		progressState.Logs = appendBoundedScreeningQueryLogs(progressState.Logs, ScreeningQueryLog{
			Time:    formatScreeningStoreTime(s.now().UTC()),
			Stage:   stage,
			Status:  progressState.RunStatus,
			Message: message,
		}, 20)
		reportProgress()
	}
	emitStreamingSQLProgress := func(streamingText string) {
		progressState.CurrentStage = "generate_sql"
		progressState.ProgressPercent = 30
		progressState.Message = "AI 正在生成 SQL"
		progressState.StreamingText = streamingText
		progressState.RunStatus = "running"
		progressState.Error = ""
		reportProgress()
	}
	emitStreamingReasoningProgress := func(streamingText string) {
		progressState.CurrentStage = "reasoning"
		progressState.ProgressPercent = 15
		progressState.Message = "AI 正在实时分析筛选条件"
		progressState.StreamingText = streamingText
		progressState.RunStatus = "running"
		progressState.Error = ""
		reportProgress()
	}

	emitProgress("prepare", 5, "准备筛选请求", nil)
	reasoningPrompt := s.buildReasoningPrompt(ScreeningQueryRequest{
		Prompt:          req.Prompt,
		AIConfigID:      req.AIConfigID,
		ResultMode:      req.ResultMode,
		ResultLimit:     resultLimit,
		Page:            page,
		PageSize:        pageSize,
		UniverseSymbols: universeSymbols,
	})
	emitProgress("reasoning", 15, "正在实时分析筛选条件", nil)
	if reasoningGenerator, ok := s.generator.(screeningStreamingReasoningGenerator); ok {
		reasoningCtx, cancelReasoning := s.newSQLGenerationContext(ctx, req.AIConfigID)
		var streamedReasoning strings.Builder
		_, err := reasoningGenerator.GenerateReasoningStream(reasoningCtx, reasoningPrompt, req.AIConfigID, func(delta string) {
			if strings.TrimSpace(delta) == "" {
				return
			}
			streamedReasoning.WriteString(delta)
			emitStreamingReasoningProgress(streamedReasoning.String())
		})
		cancelReasoning()
		if err != nil {
			if timeout := s.resolveSQLGenerationTimeout(req.AIConfigID); timeout > 0 && isScreeningQueryTimeoutError(err) {
				err = fmt.Errorf("AI 分析筛选条件超时（%s）", s.resolveSQLGenerationTimeout(req.AIConfigID).Round(time.Second))
			}
			emitProgress("reasoning", 15, "分析筛选条件失败", err)
			return nil, fmt.Errorf("generate screening reasoning: %w", err)
		}
	}

	prompt := s.buildPrompt(ScreeningQueryRequest{
		Prompt:          req.Prompt,
		AIConfigID:      req.AIConfigID,
		ResultMode:      req.ResultMode,
		ResultLimit:     resultLimit,
		Page:            page,
		PageSize:        pageSize,
		UniverseSymbols: universeSymbols,
	})

	emitProgress("generate_sql", 30, "正在生成筛选 SQL", nil)
	generateCtx, cancelGenerate := s.newSQLGenerationContext(ctx, req.AIConfigID)
	var rawSQL string
	var err error
	if streamingGenerator, ok := s.generator.(screeningStreamingSQLGenerator); ok {
		var streamed strings.Builder
		rawSQL, err = streamingGenerator.GenerateSQLStream(generateCtx, prompt, req.AIConfigID, func(delta string) {
			if strings.TrimSpace(delta) == "" {
				return
			}
			streamed.WriteString(delta)
			emitStreamingSQLProgress(streamed.String())
		})
		if strings.TrimSpace(rawSQL) == "" && streamed.Len() > 0 {
			rawSQL = streamed.String()
		}
	} else {
		rawSQL, err = s.generator.GenerateSQL(generateCtx, prompt, req.AIConfigID)
	}
	cancelGenerate()
	if err != nil {
		if timeout := s.resolveSQLGenerationTimeout(req.AIConfigID); timeout > 0 && isScreeningQueryTimeoutError(err) {
			err = fmt.Errorf("AI 生成 SQL 超时（%s）", s.resolveSQLGenerationTimeout(req.AIConfigID).Round(time.Second))
		}
		emitProgress("generate_sql", 30, "生成筛选 SQL 失败", err)
		return nil, fmt.Errorf("generate screening sql: %w", err)
	}

	queryCtx, cancelQuery := context.WithTimeout(ctx, defaultScreeningQueryExecutionTimeout)
	defer cancelQuery()

	emitProgress("validate_sql", 45, "正在校验 SQL", nil)
	validSQL, err := validateScreeningSQL(rawSQL, req.ResultMode, resultLimit, len(universeSymbols) > 0)
	if err != nil {
		screeningQueryLog.Error(
			"validate screening sql failed: mode=%s limit=%d err=%v rawSQL=%s",
			req.ResultMode,
			resultLimit,
			err,
			summarizeScreeningSQLForLog(rawSQL),
		)
		emitProgress("validate_sql", 45, "SQL 校验失败", err)
		return nil, err
	}

	var allResults []ScreeningRunResult
	totalCount := 0
	runner := screeningQueryExecutor(s.store.db)
	var conn *sql.Conn
	if len(universeSymbols) > 0 {
		conn, err = s.store.db.Conn(queryCtx)
		if err != nil {
			emitProgress("execute_query", 65, "打开筛选查询连接失败", err)
			return nil, fmt.Errorf("open screening query conn: %w", err)
		}
		defer conn.Close()
		if err := prepareScreeningScopeTable(queryCtx, conn, universeSymbols); err != nil {
			emitProgress("execute_query", 65, "准备测试范围失败", err)
			return nil, err
		}
		if err := prepareScreeningScopeViews(queryCtx, conn); err != nil {
			emitProgress("execute_query", 65, "准备测试范围失败", err)
			return nil, err
		}
		defer clearScreeningScopeTable(queryCtx, conn)
		runner = conn
	}

	emitProgress("execute_query", 65, "正在执行筛选查询", nil)
	switch req.ResultMode {
	case ScreeningResultModeUnlimited:
		totalCount, err = s.countMatches(queryCtx, runner, validSQL)
		if err != nil {
			return nil, err
		}
		allResults, err = s.queryResults(queryCtx, runner, validSQL)
	default:
		allResults, err = s.queryResults(queryCtx, runner, validSQL)
		totalCount = len(allResults)
	}
	if err != nil {
		screeningQueryLog.Error(
			"run screening query failed: prompt=%q mode=%s limit=%d err=%v sql=%s",
			req.Prompt,
			req.ResultMode,
			resultLimit,
			err,
			summarizeScreeningSQLForLog(validSQL),
		)
		emitProgress("execute_query", 65, "筛选查询执行失败", err)
		return nil, err
	}

	for index := range allResults {
		allResults[index].Rank = index + 1
	}

	runID, err := s.store.CreateScreeningRun(ScreeningRun{
		Prompt:          req.Prompt,
		MarketScope:     marketScope,
		ResultMode:      string(req.ResultMode),
		ResultLimit:     resultLimit,
		UniverseSymbols: universeSymbols,
		GeneratedSQL:    validSQL,
		MatchedCount:    totalCount,
		CreatedAt:       s.now().UTC(),
	})
	if err != nil {
		emitProgress("store_results", 85, "保存筛选记录失败", err)
		return nil, err
	}

	for index := range allResults {
		allResults[index].RunID = runID
	}
	emitProgress("store_results", 85, "正在保存筛选结果", nil)
	if err := s.store.ReplaceScreeningRunResults(runID, allResults); err != nil {
		emitProgress("store_results", 85, "保存筛选结果失败", err)
		return nil, err
	}

	response := &ScreeningQueryResponse{
		RunID:           runID,
		Prompt:          req.Prompt,
		MarketScope:     marketScope,
		ResultMode:      string(req.ResultMode),
		ResultLimit:     resultLimit,
		UniverseSymbols: universeSymbols,
		GeneratedSQL:    validSQL,
		TotalCount:      totalCount,
		Page:            page,
		PageSize:        pageSize,
		CreatedAt:       formatScreeningStoreTime(s.now().UTC()),
		Results:         paginateScreeningResults(allResults, page, pageSize),
	}
	emitProgress("completed", 100, fmt.Sprintf("筛选完成，命中 %d 条", totalCount), nil)
	return response, nil
}

func (s *ScreeningQueryService) RerunHistoryRun(runID int64, page, pageSize int) (*ScreeningQueryResponse, error) {
	return s.RerunHistoryRunWithContext(context.Background(), runID, page, pageSize, nil)
}

func (s *ScreeningQueryService) RerunHistoryRunWithUniverse(
	runID int64,
	page,
	pageSize int,
	universeSymbols []string,
) (*ScreeningQueryResponse, error) {
	return s.RerunHistoryRunWithUniverseWithContext(context.Background(), runID, page, pageSize, universeSymbols, nil)
}

func (s *ScreeningQueryService) RerunHistoryRunWithProgress(
	runID int64,
	page,
	pageSize int,
	report func(ScreeningQueryProgress),
) (*ScreeningQueryResponse, error) {
	return s.RerunHistoryRunWithContext(context.Background(), runID, page, pageSize, report)
}

func (s *ScreeningQueryService) RerunHistoryRunWithContext(
	ctx context.Context,
	runID int64,
	page,
	pageSize int,
	report func(ScreeningQueryProgress),
) (*ScreeningQueryResponse, error) {
	return s.RerunHistoryRunWithUniverseWithContext(ctx, runID, page, pageSize, nil, report)
}

func (s *ScreeningQueryService) RerunHistoryRunWithUniverseWithContext(
	ctx context.Context,
	runID int64,
	page,
	pageSize int,
	universeOverride []string,
	report func(ScreeningQueryProgress),
) (*ScreeningQueryResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("screening query service not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	run, err := s.store.GetScreeningRun(runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, sql.ErrNoRows
	}

	page = normalizePositive(page, 1)
	pageSize = normalizePageSize(pageSize)
	universeSymbols := normalizeScreeningUniverseSymbols(run.UniverseSymbols)
	if universeOverride != nil {
		universeSymbols = normalizeScreeningUniverseSymbols(universeOverride)
	}
	progressState := ScreeningQueryProgress{
		RunStatus:     "running",
		CurrentStage:  "prepare",
		Prompt:        run.Prompt,
		UniverseCount: len(universeSymbols),
		Logs:          make([]ScreeningQueryLog, 0, 8),
	}
	emitProgress := func(stage string, percent float64, message string, err error) {
		progressState.CurrentStage = stage
		progressState.ProgressPercent = percent
		progressState.Message = message
		if err != nil {
			progressState.RunStatus = "failed"
			progressState.Error = err.Error()
		} else if stage == "completed" {
			progressState.RunStatus = "completed"
			progressState.Error = ""
		} else {
			progressState.RunStatus = "running"
			progressState.Error = ""
		}
		progressState.Logs = appendBoundedScreeningQueryLogs(progressState.Logs, ScreeningQueryLog{
			Time:    formatScreeningStoreTime(s.now().UTC()),
			Stage:   stage,
			Status:  progressState.RunStatus,
			Message: message,
		}, 20)
		if report != nil {
			cloned := progressState
			cloned.Logs = append([]ScreeningQueryLog(nil), progressState.Logs...)
			report(cloned)
		}
	}

	emitProgress("prepare", 10, "准备根据历史 SQL 重新筛选", nil)
	validSQL, err := validateScreeningSQL(run.GeneratedSQL, ScreeningResultMode(run.ResultMode), run.ResultLimit, len(universeSymbols) > 0)
	if err != nil {
		screeningQueryLog.Error(
			"validate history screening sql failed: runID=%d mode=%s limit=%d err=%v sql=%s",
			runID,
			run.ResultMode,
			run.ResultLimit,
			err,
			summarizeScreeningSQLForLog(run.GeneratedSQL),
		)
		emitProgress("validate_sql", 30, "历史 SQL 校验失败", err)
		return nil, err
	}
	emitProgress("validate_sql", 30, "正在校验历史 SQL", nil)

	queryCtx, cancel := context.WithTimeout(ctx, defaultScreeningQueryExecutionTimeout)
	defer cancel()

	var allResults []ScreeningRunResult
	totalCount := 0
	runner := screeningQueryExecutor(s.store.db)
	var conn *sql.Conn
	if len(universeSymbols) > 0 {
		conn, err = s.store.db.Conn(queryCtx)
		if err != nil {
			emitProgress("execute_query", 60, "打开筛选查询连接失败", err)
			return nil, fmt.Errorf("open screening query conn: %w", err)
		}
		defer conn.Close()
		if err := prepareScreeningScopeTable(queryCtx, conn, universeSymbols); err != nil {
			emitProgress("execute_query", 60, "准备测试范围失败", err)
			return nil, err
		}
		if err := prepareScreeningScopeViews(queryCtx, conn); err != nil {
			emitProgress("execute_query", 60, "准备测试范围失败", err)
			return nil, err
		}
		defer clearScreeningScopeTable(queryCtx, conn)
		runner = conn
	}

	emitProgress("execute_query", 60, "正在根据历史 SQL 执行筛选", nil)
	switch ScreeningResultMode(run.ResultMode) {
	case ScreeningResultModeUnlimited:
		totalCount, err = s.countMatches(queryCtx, runner, validSQL)
		if err != nil {
			return nil, err
		}
		allResults, err = s.queryResults(queryCtx, runner, validSQL)
	default:
		allResults, err = s.queryResults(queryCtx, runner, validSQL)
		totalCount = len(allResults)
	}
	if err != nil {
		screeningQueryLog.Error(
			"rerun screening history failed: runID=%d prompt=%q mode=%s limit=%d err=%v sql=%s",
			runID,
			run.Prompt,
			run.ResultMode,
			run.ResultLimit,
			err,
			summarizeScreeningSQLForLog(validSQL),
		)
		emitProgress("execute_query", 60, "历史 SQL 执行失败", err)
		return nil, err
	}

	for index := range allResults {
		allResults[index].Rank = index + 1
	}

	newRunID, err := s.store.CreateScreeningRun(ScreeningRun{
		Prompt:          run.Prompt,
		MarketScope:     run.MarketScope,
		ResultMode:      run.ResultMode,
		ResultLimit:     run.ResultLimit,
		UniverseSymbols: universeSymbols,
		GeneratedSQL:    validSQL,
		MatchedCount:    totalCount,
		CreatedAt:       s.now().UTC(),
	})
	if err != nil {
		emitProgress("store_results", 85, "保存重跑记录失败", err)
		return nil, err
	}

	for index := range allResults {
		allResults[index].RunID = newRunID
	}
	if err := s.store.ReplaceScreeningRunResults(newRunID, allResults); err != nil {
		emitProgress("store_results", 85, "保存重跑结果失败", err)
		return nil, err
	}
	emitProgress("store_results", 85, "正在保存重跑结果", nil)

	response := &ScreeningQueryResponse{
		RunID:           newRunID,
		Prompt:          run.Prompt,
		MarketScope:     run.MarketScope,
		ResultMode:      run.ResultMode,
		ResultLimit:     run.ResultLimit,
		UniverseSymbols: universeSymbols,
		GeneratedSQL:    validSQL,
		TotalCount:      totalCount,
		Page:            page,
		PageSize:        pageSize,
		CreatedAt:       formatScreeningStoreTime(s.now().UTC()),
		Results:         paginateScreeningResults(allResults, page, pageSize),
	}
	emitProgress("completed", 100, fmt.Sprintf("重跑完成，命中 %d 条", totalCount), nil)
	return response, nil
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
		RunID:           run.ID,
		Prompt:          run.Prompt,
		MarketScope:     run.MarketScope,
		ResultMode:      run.ResultMode,
		ResultLimit:     run.ResultLimit,
		UniverseSymbols: run.UniverseSymbols,
		GeneratedSQL:    run.GeneratedSQL,
		TotalCount:      run.MatchedCount,
		Page:            page,
		PageSize:        pageSize,
		CreatedAt:       formatScreeningStoreTime(run.CreatedAt),
		Results:         paginateScreeningResults(results, page, pageSize),
	}, nil
}

func (s *ScreeningQueryService) DeleteRun(runID int64) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("screening query service not initialized")
	}
	return s.store.DeleteScreeningRun(runID)
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
	builder.WriteString("## 字段取值约束\n")
	builder.WriteString("- market 字段的实际取值固定为：上海、深圳、北京。\n")
	builder.WriteString("- 如果要按市场过滤，只能使用这些实际值。\n")
	builder.WriteString("- 不要写成 SH、SZ、BJ，也不要写成 沪市、深市、北交所、上交所、深交所。\n\n")

	builder.WriteString("## 语义解析规则\n")
	builder.WriteString("- 最近N天一律指最近 N 个交易日，不是自然日。\n")
	builder.WriteString("- 必须先用 ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY trade_date DESC) 标记最近交易日顺序。\n")
	builder.WriteString("- “上涨”定义为当日 change_percent > 0。\n")
	builder.WriteString("- “下跌”定义为当日 change_percent < 0。\n")
	builder.WriteString("- “涨停”必须按股票类别判断，不能统一按 10%。\n")
	builder.WriteString("- ST 股票：change_percent >= 4.8 视为涨停。\n")
	builder.WriteString("- 主板普通股：change_percent >= 9.8 视为涨停。\n")
	builder.WriteString("- 创业板 / 科创板 / 北交所：change_percent >= 19.8 视为涨停。\n")
	builder.WriteString("- 判断涨停或跌停时，必须结合 market、is_st 字段，不能只看 change_percent > 0。\n")
	builder.WriteString("- “连续N天”表示最近 N 个交易日每天都满足条件，不能只用 COUNT(condition) >= N 代替。\n")
	builder.WriteString("- “N天内至少K天”表示 SUM(事件标记) >= K。\n")
	builder.WriteString("- “N天内恰好K天”表示 SUM(事件标记) = K。\n")
	builder.WriteString("- “N天内有一天”表示 MAX(事件标记) = 1。\n")
	builder.WriteString("- “最近一天”表示 rn = 1。\n")
	builder.WriteString("- 若条件是“最近三天上涨，且有一天涨停”，必须拆成“最近3天每天上涨”与“最近3天内至少1天涨停”两个条件。\n")
	builder.WriteString("- 若条件是“最近三天连续都涨停”，必须拆成“最近3天每天都满足涨停阈值”。\n")
	builder.WriteString("- 不能把“涨停”弱化成“涨幅较大”或“上涨”。\n")
	builder.WriteString("- 先在 CTE 中生成日级标记，例如 is_up、is_limit_up、is_down、is_limit_down，再在聚合层根据语义使用 MIN、MAX、SUM、COUNT 做筛选。\n\n")

	if len(req.UniverseSymbols) > 0 {
		builder.WriteString("## 测试同步范围\n")
		builder.WriteString("- 当前请求只允许筛选本次测试同步覆盖到的股票。\n")
		builder.WriteString("- 系统会在执行前自动把白名单表/视图限制到这批股票范围。\n")
		builder.WriteString("- 如果你需要显式表达范围约束，也可以使用 screening_scope(symbol)。\n")
		builder.WriteString("- 当前允许的 symbol：")
		builder.WriteString(strings.Join(req.UniverseSymbols, ", "))
		builder.WriteString("\n\n")
	}

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

func (s *ScreeningQueryService) buildReasoningPrompt(req ScreeningQueryRequest) string {
	var builder strings.Builder
	builder.WriteString("你是A股日线筛选助手。\n")
	builder.WriteString("请根据用户条件输出一段连续、实时、尽量贴近当前分析顺序的可展示思考流，让用户知道你正在如何拆解筛选任务。\n\n")
	builder.WriteString("## 输出约束\n")
	builder.WriteString("- 使用连续自然语言，像边分析边说明，尽量保持实时展开。\n")
	builder.WriteString("- 不要总结成摘要，不要压缩成结论。\n")
	builder.WriteString("- 不要使用列表、编号或分点。\n")
	builder.WriteString("- 使用中文。\n")
	builder.WriteString("- 只输出适合直接展示给用户的分析过程，不要输出 SQL、代码块、系统提示或内部规则。\n")
	builder.WriteString("- 可以自然提到接下来会如何落到日线字段，但不要直接给出最终结论。\n")
	builder.WriteString("- 内容只围绕股票池、指标、时间窗口、约束、排序和结果模式。\n\n")
	if len(req.UniverseSymbols) > 0 {
		builder.WriteString("## 当前测试股票池\n")
		builder.WriteString(strings.Join(req.UniverseSymbols, ", "))
		builder.WriteString("\n\n")
	}
	builder.WriteString("## 用户条件\n")
	builder.WriteString(strings.TrimSpace(req.Prompt))
	return builder.String()
}

type screeningQueryExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *ScreeningQueryService) countMatches(ctx context.Context, runner screeningQueryExecutor, sqlText string) (int, error) {
	countSQL, err := buildScreeningCountSQL(sqlText)
	if err != nil {
		screeningQueryLog.Error("build screening count sql failed: err=%v sql=%s", err, summarizeScreeningSQLForLog(sqlText))
		return 0, err
	}

	row := runner.QueryRowContext(ctx, countSQL)

	var count int
	if err := row.Scan(&count); err != nil {
		screeningQueryLog.Error(
			"count screening results failed: err=%v sql=%s countSQL=%s",
			err,
			summarizeScreeningSQLForLog(sqlText),
			summarizeScreeningSQLForLog(countSQL),
		)
		return 0, fmt.Errorf("count screening results: %w", err)
	}
	return count, nil
}

func (s *ScreeningQueryService) queryResults(ctx context.Context, runner screeningQueryExecutor, sqlText string) ([]ScreeningRunResult, error) {
	rows, err := runner.QueryContext(ctx, sqlText)
	if err != nil {
		screeningQueryLog.Error("query screening results failed: err=%v sql=%s", err, summarizeScreeningSQLForLog(sqlText))
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

func validateScreeningSQL(rawSQL string, mode ScreeningResultMode, resultLimit int, allowScreeningScope bool) (string, error) {
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
	if err := validateScreeningOrderByClause(sqlText); err != nil {
		return "", err
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
	if allowScreeningScope {
		allowedSources["screening_scope"] = struct{}{}
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
		sourceName := strings.ToLower(match[1])
		if _, ok := allowedSources[sourceName]; !ok {
			return "", fmt.Errorf("screening sql references non-whitelisted source %q", match[1])
		}
	}

	return sqlText, nil
}

func buildScreeningCountSQL(sqlText string) (string, error) {
	countSource, _, err := stripOutermostOrderByClause(sqlText)
	if err != nil {
		return "", err
	}
	return `SELECT COUNT(1) FROM (` + countSource + `) AS screening_candidates`, nil
}

func normalizeScreeningSQL(rawSQL string) string {
	trimmed := strings.TrimSpace(rawSQL)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			trimmed = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	return screeningSQLLiteralReplacer.Replace(trimmed)
}

func validateScreeningOrderByClause(sqlText string) error {
	clause, found, err := extractTopLevelOrderByClause(sqlText)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	for _, term := range splitSQLTopLevelCSV(clause) {
		expr := normalizeScreeningOrderByExpr(term)
		if expr == "" {
			return fmt.Errorf("screening sql ORDER BY contains empty expression")
		}
		if _, ok := screeningAllowedOrderByColumns[expr]; !ok {
			return fmt.Errorf("screening sql ORDER BY can only use final output columns or aliases")
		}
	}
	return nil
}

func extractTopLevelOrderByClause(sqlText string) (string, bool, error) {
	_, orderByEnd, found, err := findTopLevelOrderBy(sqlText)
	if err != nil || !found {
		return "", found, err
	}

	limitStart, _, limitFound, err := findTopLevelKeyword(sqlText, orderByEnd, "LIMIT")
	if err != nil {
		return "", false, err
	}
	end := len(sqlText)
	if limitFound {
		end = limitStart
	}
	return strings.TrimSpace(sqlText[orderByEnd:end]), true, nil
}

func stripOutermostOrderByClause(sqlText string) (string, bool, error) {
	orderStart, _, found, err := findTopLevelOrderBy(sqlText)
	if err != nil || !found {
		return strings.TrimSpace(sqlText), found, err
	}

	limitStart, _, limitFound, err := findTopLevelKeyword(sqlText, orderStart, "LIMIT")
	if err != nil {
		return "", false, err
	}

	if limitFound {
		return strings.TrimSpace(sqlText[:orderStart] + " " + sqlText[limitStart:]), true, nil
	}
	return strings.TrimSpace(sqlText[:orderStart]), true, nil
}

func findTopLevelOrderBy(sqlText string) (int, int, bool, error) {
	depth := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	upper := strings.ToUpper(sqlText)
	lastStart := -1
	lastEnd := -1

	for i := 0; i < len(upper); i++ {
		ch := upper[i]
		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}
		if inSingle || inDouble || inBacktick {
			continue
		}
		switch ch {
		case '(':
			depth++
			continue
		case ')':
			depth--
			if depth < 0 {
				return 0, 0, false, fmt.Errorf("screening sql has unbalanced parentheses")
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if isSQLWordAt(upper, i, "ORDER") {
			j := i + len("ORDER")
			for j < len(upper) && isSQLWhitespace(upper[j]) {
				j++
			}
			if isSQLWordAt(upper, j, "BY") {
				lastStart = i
				lastEnd = j + len("BY")
				i = lastEnd - 1
			}
		}
	}

	if depth != 0 || inSingle || inDouble || inBacktick {
		return 0, 0, false, fmt.Errorf("screening sql has unterminated clause")
	}
	if lastStart < 0 {
		return 0, 0, false, nil
	}
	return lastStart, lastEnd, true, nil
}

func findTopLevelKeyword(sqlText string, start int, keyword string) (int, int, bool, error) {
	if start < 0 {
		start = 0
	}
	depth := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	upper := strings.ToUpper(sqlText)

	for i := start; i < len(upper); i++ {
		ch := upper[i]
		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}
		if inSingle || inDouble || inBacktick {
			continue
		}
		switch ch {
		case '(':
			depth++
			continue
		case ')':
			depth--
			if depth < 0 {
				return 0, 0, false, fmt.Errorf("screening sql has unbalanced parentheses")
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if isSQLWordAt(upper, i, keyword) {
			return i, i + len(keyword), true, nil
		}
	}

	if depth != 0 || inSingle || inDouble || inBacktick {
		return 0, 0, false, fmt.Errorf("screening sql has unterminated clause")
	}
	return 0, 0, false, nil
}

func splitSQLTopLevelCSV(input string) []string {
	parts := make([]string, 0, 4)
	start := 0
	depth := 0
	inSingle := false
	inDouble := false
	inBacktick := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		}
		if inSingle || inDouble || inBacktick {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(input[start:i]))
				start = i + 1
			}
		}
	}

	parts = append(parts, strings.TrimSpace(input[start:]))
	return parts
}

func normalizeScreeningOrderByExpr(term string) string {
	fields := strings.Fields(strings.TrimSpace(term))
	if len(fields) == 0 {
		return ""
	}
	expr := strings.Trim(fields[0], "`\"[]")
	if idx := strings.LastIndex(expr, "."); idx >= 0 {
		expr = expr[idx+1:]
	}
	expr = strings.Trim(expr, "`\"[]")
	if expr == "" {
		return ""
	}
	if strings.ContainsAny(expr, "()*/+-") {
		return ""
	}
	return strings.ToLower(expr)
}

func summarizeScreeningSQLForLog(sqlText string) string {
	normalized := strings.Join(strings.Fields(sqlText), " ")
	if len(normalized) > 400 {
		return normalized[:400] + "...(truncated)"
	}
	return normalized
}

func isSQLWordAt(upper string, index int, word string) bool {
	if index < 0 || index+len(word) > len(upper) {
		return false
	}
	if upper[index:index+len(word)] != word {
		return false
	}
	if index > 0 {
		prev := upper[index-1]
		if isSQLIdentChar(prev) {
			return false
		}
	}
	if end := index + len(word); end < len(upper) {
		next := upper[end]
		if isSQLIdentChar(next) {
			return false
		}
	}
	return true
}

func isSQLIdentChar(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func isSQLWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t'
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

func normalizeScreeningUniverseSymbols(symbols []string) []string {
	if len(symbols) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(symbols))
	normalized := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		trimmed := strings.TrimSpace(symbol)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func (s *ScreeningQueryService) resolveSQLGenerationTimeout(aiConfigID string) time.Duration {
	if s == nil || s.configService == nil {
		return 0
	}

	cfg := s.configService.GetConfig()
	if cfg == nil {
		return 0
	}
	if cfg.Screening.SQLTimeoutSeconds > 0 {
		return time.Duration(cfg.Screening.SQLTimeoutSeconds) * time.Second
	}
	return 0
}

func (s *ScreeningQueryService) newSQLGenerationContext(parent context.Context, aiConfigID string) (context.Context, context.CancelFunc) {
	timeout := s.resolveSQLGenerationTimeout(aiConfigID)
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func isScreeningQueryTimeoutError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")
}

func appendBoundedScreeningQueryLogs(logs []ScreeningQueryLog, log ScreeningQueryLog, limit int) []ScreeningQueryLog {
	logs = append(logs, log)
	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	return logs
}

func prepareScreeningScopeTable(ctx context.Context, runner screeningQueryExecutor, symbols []string) error {
	if _, err := runner.ExecContext(ctx, `CREATE TEMP TABLE IF NOT EXISTS screening_scope (symbol TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create screening_scope temp table: %w", err)
	}
	if _, err := runner.ExecContext(ctx, `DELETE FROM screening_scope`); err != nil {
		return fmt.Errorf("clear screening_scope temp table: %w", err)
	}
	for _, symbol := range symbols {
		if _, err := runner.ExecContext(ctx, `INSERT INTO screening_scope (symbol) VALUES (?)`, symbol); err != nil {
			return fmt.Errorf("insert screening_scope symbol %s: %w", symbol, err)
		}
	}
	return nil
}

func prepareScreeningScopeViews(ctx context.Context, runner screeningQueryExecutor) error {
	statements := []string{
		`DROP VIEW IF EXISTS temp.stocks_basic`,
		`CREATE TEMP VIEW stocks_basic AS
			SELECT b.*
			FROM main.stocks_basic AS b
			JOIN screening_scope AS scope
			  ON scope.symbol = b.symbol`,
		`DROP VIEW IF EXISTS temp.daily_bars`,
		`CREATE TEMP VIEW daily_bars AS
			SELECT b.*
			FROM main.daily_bars AS b
			JOIN screening_scope AS scope
			  ON scope.symbol = b.symbol`,
		`DROP VIEW IF EXISTS temp.daily_snapshots`,
		`CREATE TEMP VIEW daily_snapshots AS
			SELECT s.*
			FROM main.daily_snapshots AS s
			JOIN screening_scope AS scope
			  ON scope.symbol = s.symbol`,
		`DROP VIEW IF EXISTS temp.v_stock_latest_daily`,
		`CREATE TEMP VIEW v_stock_latest_daily AS
			SELECT v.*
			FROM main.v_stock_latest_daily AS v
			JOIN screening_scope AS scope
			  ON scope.symbol = v.symbol`,
	}
	for _, statement := range statements {
		if _, err := runner.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("prepare screening scoped views: %w", err)
		}
	}
	return nil
}

func clearScreeningScopeTable(ctx context.Context, runner screeningQueryExecutor) {
	_, _ = runner.ExecContext(ctx, `DELETE FROM screening_scope`)
	_, _ = runner.ExecContext(ctx, `DROP VIEW IF EXISTS temp.v_stock_latest_daily`)
	_, _ = runner.ExecContext(ctx, `DROP VIEW IF EXISTS temp.daily_snapshots`)
	_, _ = runner.ExecContext(ctx, `DROP VIEW IF EXISTS temp.daily_bars`)
	_, _ = runner.ExecContext(ctx, `DROP VIEW IF EXISTS temp.stocks_basic`)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
