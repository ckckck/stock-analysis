package services

import (
	"context"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

func TestScreeningQueryBuildPromptIncludesResultModeAndWhitelist(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{})
	prompt := service.buildPrompt(ScreeningQueryRequest{
		Prompt:      "找最近涨幅最强的股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 50,
	})

	for _, fragment := range []string{
		"v_stock_latest_daily",
		"只支持日线",
		"LIMIT 50",
		"最终 SELECT 必须返回以下列",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
	}
}

func TestScreeningQueryBuildPromptIncludesScopedUniverseWhenProvided(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{})
	prompt := service.buildPrompt(ScreeningQueryRequest{
		Prompt:          "只看测试范围内的强势股",
		ResultMode:      ScreeningResultModeTopN,
		ResultLimit:     10,
		UniverseSymbols: []string{"sh600000", "sz000001"},
	})

	for _, fragment := range []string{
		"screening_scope",
		"sh600000",
		"sz000001",
		"测试同步范围",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
	}
}

func TestScreeningQueryBuildPromptIncludesTradingWindowAndLimitUpRules(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{})
	prompt := service.buildPrompt(ScreeningQueryRequest{
		Prompt:      "找最近三天都上涨且至少一天涨停的股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 50,
	})

	for _, fragment := range []string{
		"最近N天一律指最近 N 个交易日",
		"“上涨”定义为当日 change_percent > 0",
		"ST 股票：change_percent >= 4.8 视为涨停",
		"主板普通股：change_percent >= 9.8 视为涨停",
		"创业板 / 科创板 / 北交所：change_percent >= 19.8 视为涨停",
		"“连续N天”表示最近 N 个交易日每天都满足条件",
		"“N天内至少K天”表示 SUM(事件标记) >= K",
		"“N天内有一天”表示 MAX(事件标记) = 1",
		"is_up",
		"is_limit_up",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
	}
}

func TestScreeningQueryBuildPromptIncludesStoredMarketValues(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{})
	prompt := service.buildPrompt(ScreeningQueryRequest{
		Prompt:      "找最近三天上涨的沪深股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 50,
	})

	for _, fragment := range []string{
		"market 字段的实际取值固定为：上海、深圳、北京",
		"不要写成 SH、SZ、BJ",
		"不要写成 沪市、深市、北交所",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
	}
}

func TestScreeningReasoningPromptIncludesConstraints(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{})
	prompt := service.buildReasoningPrompt(ScreeningQueryRequest{
		Prompt:          "找最近 20 个交易日放量上涨、换手率提升的沪深股票",
		ResultMode:      ScreeningResultModeTopN,
		ResultLimit:     20,
		UniverseSymbols: []string{"sh600000", "sz000001"},
	})

	for _, fragment := range []string{
		"不要输出 SQL",
		"连续自然语言",
		"不要总结成摘要",
		"适合直接展示给用户",
		"sh600000",
		"sz000001",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
	}
}

func TestScreeningQueryRejectsDangerousSQL(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: "DELETE FROM daily_bars",
	})

	_, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "删除所有数据",
		ResultMode:  ScreeningResultModeUnlimited,
		ResultLimit: 0,
		Page:        1,
		PageSize:    50,
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want dangerous SQL rejection")
	}

	runs, err := store.ListScreeningRuns(10)
	if err != nil {
		t.Fatalf("ListScreeningRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("screening runs count = %d, want 0", len(runs))
	}
}

func TestScreeningQueryRunLimitModeStoresResults(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	response, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "找今天涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.TotalCount != 2 {
		t.Fatalf("response.TotalCount = %d, want 2", response.TotalCount)
	}
	if len(response.Results) != 2 {
		t.Fatalf("len(response.Results) = %d, want 2", len(response.Results))
	}
	if response.Results[0].Symbol != "sh600000" || response.Results[1].Symbol != "sz000001" {
		t.Fatalf("response.Results = %#v", response.Results)
	}

	storedResults, err := store.ListScreeningRunResults(response.RunID)
	if err != nil {
		t.Fatalf("ListScreeningRunResults() error = %v", err)
	}
	if len(storedResults) != 2 {
		t.Fatalf("stored results count = %d, want 2", len(storedResults))
	}
}

func TestScreeningQueryRunUnlimitedModeCountsAndPaginates(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
`,
	})

	response, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "按今日涨幅排序",
		ResultMode:  ScreeningResultModeUnlimited,
		ResultLimit: 0,
		Page:        2,
		PageSize:    1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.TotalCount != 3 {
		t.Fatalf("response.TotalCount = %d, want 3", response.TotalCount)
	}
	if len(response.Results) != 1 {
		t.Fatalf("len(response.Results) = %d, want 1", len(response.Results))
	}
	if response.Results[0].Symbol != "sz000001" {
		t.Fatalf("page 2 result symbol = %q, want %q", response.Results[0].Symbol, "sz000001")
	}

	storedResults, err := store.ListScreeningRunResults(response.RunID)
	if err != nil {
		t.Fatalf("ListScreeningRunResults() error = %v", err)
	}
	if len(storedResults) != 3 {
		t.Fatalf("stored results count = %d, want 3", len(storedResults))
	}
}

func TestValidateScreeningSQLRejectsOrderByUnknownOutputColumn(t *testing.T) {
	_, err := validateScreeningSQL(`
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY low DESC
`, ScreeningResultModeUnlimited, 0, false)
	if err == nil {
		t.Fatalf("validateScreeningSQL() error = nil, want ORDER BY rejection")
	}
	if !strings.Contains(err.Error(), "ORDER BY") {
		t.Fatalf("validateScreeningSQL() error = %v, want ORDER BY context", err)
	}
}

func TestBuildScreeningCountSQLRemovesOutermostOrderByOnly(t *testing.T) {
	sqlText := `
WITH ranked AS (
  SELECT
    symbol,
    ROW_NUMBER() OVER (PARTITION BY symbol ORDER BY trade_date DESC) AS rn
  FROM daily_snapshots
)
SELECT
  symbol,
  symbol AS name,
  1 AS score,
  '2026-03-19' AS snapshot_trade_date,
  1 AS price,
  1 AS change_percent,
  1 AS volume,
  1 AS amount
FROM ranked
WHERE rn = 1
ORDER BY score DESC
`

	countSQL, err := buildScreeningCountSQL(sqlText)
	if err != nil {
		t.Fatalf("buildScreeningCountSQL() error = %v", err)
	}
	if strings.Contains(strings.ToUpper(countSQL), "ORDER BY SCORE DESC") {
		t.Fatalf("count SQL still contains outer ORDER BY: %s", countSQL)
	}
	if !strings.Contains(strings.ToUpper(countSQL), "ROW_NUMBER() OVER (PARTITION BY SYMBOL ORDER BY TRADE_DATE DESC)") {
		t.Fatalf("count SQL removed window ORDER BY unexpectedly: %s", countSQL)
	}
}

func TestScreeningQueryRunLogsSQLWhenExecutionFails(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	restoreStderr, output := captureStderr(t)
	defer restoreStderr()

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY low DESC
`,
	})

	_, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "按 low 排序",
		ResultMode:  ScreeningResultModeUnlimited,
		ResultLimit: 0,
		Page:        1,
		PageSize:    50,
	})
	if err == nil {
		t.Fatalf("Run() error = nil, want execution rejection")
	}

	logOutput := output()
	if !strings.Contains(logOutput, "screening.query") {
		t.Fatalf("stderr log = %q, want screening.query module", logOutput)
	}
	if !strings.Contains(logOutput, "ORDER BY low DESC") {
		t.Fatalf("stderr log = %q, want failed SQL snippet", logOutput)
	}
}

func TestScreeningQueryRunRestrictsResultsToUniverseSymbols(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  v.symbol,
  v.name,
  v.change_percent AS score,
  v.snapshot_trade_date,
  v.price,
  v.change_percent,
  v.volume,
  v.amount
FROM v_stock_latest_daily AS v
JOIN screening_scope AS scope
  ON scope.symbol = v.symbol
ORDER BY v.change_percent DESC
LIMIT 5
`,
	})

	response, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:          "在测试范围里找涨幅最高的股票",
		ResultMode:      ScreeningResultModeTopN,
		ResultLimit:     5,
		Page:            1,
		PageSize:        50,
		UniverseSymbols: []string{"sz000001"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.TotalCount != 1 {
		t.Fatalf("response.TotalCount = %d, want 1", response.TotalCount)
	}
	if len(response.Results) != 1 {
		t.Fatalf("len(response.Results) = %d, want 1", len(response.Results))
	}
	if response.Results[0].Symbol != "sz000001" {
		t.Fatalf("response.Results[0].Symbol = %q, want sz000001", response.Results[0].Symbol)
	}
}

func TestScreeningQueryRunNormalizesMarketAliasesInGeneratedSQL(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
WHERE market IN ('SH', 'SZ')
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	response, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "找沪深市场里涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.TotalCount != 2 {
		t.Fatalf("response.TotalCount = %d, want 2", response.TotalCount)
	}
	if len(response.Results) != 2 {
		t.Fatalf("len(response.Results) = %d, want 2", len(response.Results))
	}
	if response.Results[0].Symbol != "sh600000" || response.Results[1].Symbol != "sz000001" {
		t.Fatalf("response.Results = %#v", response.Results)
	}
	if !strings.Contains(response.GeneratedSQL, "market IN ('上海', '深圳')") {
		t.Fatalf("response.GeneratedSQL = %q, want normalized market literals", response.GeneratedSQL)
	}
}

func TestValidateScreeningSQLNormalizesChineseMarketAliases(t *testing.T) {
	sqlText, err := validateScreeningSQL(`
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
WHERE market IN ('沪市', '深市')
ORDER BY change_percent DESC
LIMIT 2
`, ScreeningResultModeTopN, 2, false)
	if err != nil {
		t.Fatalf("validateScreeningSQL() error = %v", err)
	}
	if !strings.Contains(sqlText, "market IN ('上海', '深圳')") {
		t.Fatalf("sqlText = %q, want normalized market literals", sqlText)
	}
}

func TestScreeningQueryRunRestrictsUniverseWithoutExplicitScreeningScopeReference(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 5
`,
	})

	response, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:          "在测试范围里找涨幅最高的股票",
		ResultMode:      ScreeningResultModeTopN,
		ResultLimit:     5,
		Page:            1,
		PageSize:        50,
		UniverseSymbols: []string{"sz000001"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.TotalCount != 1 {
		t.Fatalf("response.TotalCount = %d, want 1", response.TotalCount)
	}
	if len(response.Results) != 1 {
		t.Fatalf("len(response.Results) = %d, want 1", len(response.Results))
	}
	if response.Results[0].Symbol != "sz000001" {
		t.Fatalf("response.Results[0].Symbol = %q, want sz000001", response.Results[0].Symbol)
	}
}

func TestScreeningQueryRunReportsProgressStages(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	var progressEvents []ScreeningQueryProgress
	_, err := service.RunWithProgress(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}, func(progress ScreeningQueryProgress) {
		progressEvents = append(progressEvents, progress)
	})
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}

	if len(progressEvents) == 0 {
		t.Fatalf("progressEvents = 0, want > 0")
	}

	last := progressEvents[len(progressEvents)-1]
	if last.RunStatus != "completed" {
		t.Fatalf("last.RunStatus = %q, want completed", last.RunStatus)
	}
	if last.ProgressPercent != 100 {
		t.Fatalf("last.ProgressPercent = %v, want 100", last.ProgressPercent)
	}
	if len(last.Logs) < 4 {
		t.Fatalf("last.Logs = %#v, want multiple stage logs", last.Logs)
	}

	stageSet := make(map[string]struct{}, len(last.Logs))
	for _, log := range last.Logs {
		stageSet[log.Stage] = struct{}{}
	}
	for _, stage := range []string{"prepare", "generate_sql", "validate_sql", "execute_query", "store_results", "completed"} {
		if _, ok := stageSet[stage]; !ok {
			t.Fatalf("progress stages = %#v, missing %q", stageSet, stage)
		}
	}
}

func TestScreeningQueryRunReportsStreamingGenerateSQLProgressEvents(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
		streamChunks: []string{
			"SELECT\n",
			"  symbol,\n",
			"  name,\n",
			"  change_percent AS score\n",
			"FROM v_stock_latest_daily\n",
			"ORDER BY change_percent DESC\n",
			"LIMIT 2\n",
		},
	})

	var progressEvents []ScreeningQueryProgress
	response, err := service.RunWithProgress(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}, func(progress ScreeningQueryProgress) {
		progressEvents = append(progressEvents, progress)
	})
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}
	if response == nil {
		t.Fatalf("RunWithProgress() response = nil")
	}

	generateSQLEvents := 0
	lastStreamingText := ""
	for _, event := range progressEvents {
		if event.CurrentStage == "generate_sql" {
			generateSQLEvents++
			if strings.TrimSpace(event.StreamingText) != "" {
				lastStreamingText = event.StreamingText
			}
		}
	}
	if generateSQLEvents <= 1 {
		t.Fatalf("generate_sql progress events = %d, want > 1 for streaming generator", generateSQLEvents)
	}
	if !strings.Contains(lastStreamingText, "FROM v_stock_latest_daily") {
		t.Fatalf("last streaming text = %q, want generated SQL content", lastStreamingText)
	}
}

func TestScreeningQueryRunReportsReasoningBeforeGenerateSQL(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		reasoningChunks: []string{
			"1. 正在解析筛选目标\n",
			"2. 正在识别指标和约束\n",
			"3. 正在确定股票池范围\n",
		},
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	var stages []string
	_, err := service.RunWithProgress(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}, func(progress ScreeningQueryProgress) {
		stages = append(stages, progress.CurrentStage)
	})
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}

	reasoningIndex := -1
	generateSQLIndex := -1
	for index, stage := range stages {
		if reasoningIndex == -1 && stage == "reasoning" {
			reasoningIndex = index
		}
		if generateSQLIndex == -1 && stage == "generate_sql" {
			generateSQLIndex = index
		}
	}
	if reasoningIndex == -1 {
		t.Fatalf("stages = %#v, want reasoning stage", stages)
	}
	if generateSQLIndex == -1 {
		t.Fatalf("stages = %#v, want generate_sql stage", stages)
	}
	if reasoningIndex >= generateSQLIndex {
		t.Fatalf("stages = %#v, want reasoning before generate_sql", stages)
	}
}

func TestScreeningQueryRunStreamsReasoningText(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		reasoningChunks: []string{
			"1. 正在解析筛选目标\n",
			"2. 正在识别指标和约束\n",
			"3. 正在确定股票池范围\n",
		},
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	var reasoningEvents []ScreeningQueryProgress
	_, err := service.RunWithProgress(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}, func(progress ScreeningQueryProgress) {
		if progress.CurrentStage == "reasoning" {
			reasoningEvents = append(reasoningEvents, progress)
		}
	})
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}
	if len(reasoningEvents) <= 1 {
		t.Fatalf("reasoningEvents = %d, want > 1 for streaming reasoning", len(reasoningEvents))
	}

	lastReasoning := reasoningEvents[len(reasoningEvents)-1].StreamingText
	if !strings.Contains(lastReasoning, "正在确定股票池范围") {
		t.Fatalf("last reasoning text = %q, want streamed reasoning content", lastReasoning)
	}
}

func TestScreeningQueryRunAllowsUnlimitedSQLGenerationTimeout(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	cfg := configService.GetConfig()
	cfg.Screening.SQLTimeoutSeconds = 0
	cfg.AIConfigs = []models.AIConfig{{
		ID:        "test-default",
		Name:      "Test Default",
		Provider:  models.AIProviderOpenAI,
		ModelName: "fake-model",
		Timeout:   77,
		IsDefault: true,
	}}
	cfg.DefaultAIID = "test-default"
	if err := configService.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	generator := &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	}
	service := NewScreeningQueryService(configService, store, generator)

	if _, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if generator.observedHasDeadline {
		t.Fatalf("observedHasDeadline = true, want false when screening timeout disabled")
	}
}

func TestScreeningQueryRunUsesScreeningConfigTimeoutBeforeAIProvider(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	cfg := configService.GetConfig()
	cfg.Screening.SQLTimeoutSeconds = 123
	cfg.AIConfigs = []models.AIConfig{{
		ID:        "test-default",
		Name:      "Test Default",
		Provider:  models.AIProviderOpenAI,
		ModelName: "fake-model",
		Timeout:   77,
		IsDefault: true,
	}}
	cfg.DefaultAIID = "test-default"
	if err := configService.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	generator := &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	}
	service := NewScreeningQueryService(configService, store, generator)

	if _, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "找涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if generator.observedTimeout < 121*time.Second || generator.observedTimeout > 125*time.Second {
		t.Fatalf("observed timeout = %s, want about 123s", generator.observedTimeout)
	}
}

func TestScreeningQueryServiceExposesHistoryRerunAndCreatesNewRun(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 2
`,
	})

	first, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:      "找今天涨幅最高的两只股票",
		ResultMode:  ScreeningResultModeTopN,
		ResultLimit: 2,
		Page:        1,
		PageSize:    50,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	method := reflect.ValueOf(service).MethodByName("RerunHistoryRun")
	if !method.IsValid() {
		t.Fatalf("RerunHistoryRun method is missing")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(first.RunID),
		reflect.ValueOf(1),
		reflect.ValueOf(50),
	})
	if len(results) != 2 {
		t.Fatalf("RerunHistoryRun results len = %d, want 2", len(results))
	}

	if !results[1].IsNil() {
		t.Fatalf("RerunHistoryRun() error = %v", results[1].Interface())
	}

	rerun, ok := results[0].Interface().(*ScreeningQueryResponse)
	if !ok {
		t.Fatalf("RerunHistoryRun() response type = %T", results[0].Interface())
	}
	if rerun == nil {
		t.Fatalf("RerunHistoryRun() response = nil")
	}
	if rerun.RunID == first.RunID {
		t.Fatalf("rerun.RunID = %d, want new run id", rerun.RunID)
	}
	if rerun.GeneratedSQL != first.GeneratedSQL {
		t.Fatalf("rerun.GeneratedSQL = %q, want %q", rerun.GeneratedSQL, first.GeneratedSQL)
	}
	if rerun.TotalCount != first.TotalCount {
		t.Fatalf("rerun.TotalCount = %d, want %d", rerun.TotalCount, first.TotalCount)
	}

	runs, err := store.ListScreeningRuns(10)
	if err != nil {
		t.Fatalf("ListScreeningRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("screening runs count = %d, want 2", len(runs))
	}
}

func TestScreeningQueryRerunHistoryRunWithUniverseOverridesStoredScope(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)
	seedScreeningQueryFixture(t, store)

	service := NewScreeningQueryService(configService, store, &fakeScreeningSQLGenerator{
		sql: `
SELECT
  symbol,
  name,
  change_percent AS score,
  snapshot_trade_date,
  price,
  change_percent,
  volume,
  amount
FROM v_stock_latest_daily
ORDER BY change_percent DESC
LIMIT 1
`,
	})

	first, err := service.Run(context.Background(), ScreeningQueryRequest{
		Prompt:          "在测试范围里找涨幅最高的一只股票",
		ResultMode:      ScreeningResultModeTopN,
		ResultLimit:     1,
		Page:            1,
		PageSize:        50,
		UniverseSymbols: []string{"sh600000"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(first.Results) != 1 || first.Results[0].Symbol != "sh600000" {
		t.Fatalf("first results = %#v, want sh600000 only", first.Results)
	}

	rerun, err := service.RerunHistoryRunWithUniverse(first.RunID, 1, 50, []string{"sz000001"})
	if err != nil {
		t.Fatalf("RerunHistoryRunWithUniverse() error = %v", err)
	}
	if rerun == nil {
		t.Fatalf("RerunHistoryRunWithUniverse() response = nil")
	}
	if rerun.GeneratedSQL != first.GeneratedSQL {
		t.Fatalf("rerun.GeneratedSQL = %q, want %q", rerun.GeneratedSQL, first.GeneratedSQL)
	}
	if !reflect.DeepEqual(rerun.UniverseSymbols, []string{"sz000001"}) {
		t.Fatalf("rerun.UniverseSymbols = %#v, want %#v", rerun.UniverseSymbols, []string{"sz000001"})
	}
	if len(rerun.Results) != 1 || rerun.Results[0].Symbol != "sz000001" {
		t.Fatalf("rerun results = %#v, want sz000001 only", rerun.Results)
	}
}

type fakeScreeningSQLGenerator struct {
	sql                 string
	reasoning           string
	streamChunks        []string
	reasoningChunks     []string
	prompts             []string
	aiConfig            []string
	err                 error
	observedTimeout     time.Duration
	observedHasDeadline bool
}

func captureStderr(t *testing.T) (func(), func() string) {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = writer

	return func() {
		_ = writer.Close()
		os.Stderr = original
	}, func() string {
		_ = writer.Close()
		data, _ := io.ReadAll(reader)
		_ = reader.Close()
		os.Stderr = original
		return string(data)
	}
}

func (f *fakeScreeningSQLGenerator) GenerateSQL(ctx context.Context, prompt string, aiConfigID string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	f.aiConfig = append(f.aiConfig, aiConfigID)
	if deadline, ok := ctx.Deadline(); ok {
		f.observedHasDeadline = true
		f.observedTimeout = time.Until(deadline).Round(time.Second)
	}
	return f.sql, f.err
}

func (f *fakeScreeningSQLGenerator) GenerateReasoningStream(
	ctx context.Context,
	prompt string,
	aiConfigID string,
	onDelta func(string),
) (string, error) {
	f.prompts = append(f.prompts, prompt)
	f.aiConfig = append(f.aiConfig, aiConfigID)
	if deadline, ok := ctx.Deadline(); ok {
		f.observedHasDeadline = true
		f.observedTimeout = time.Until(deadline).Round(time.Second)
	}
	for _, chunk := range f.reasoningChunks {
		if onDelta != nil {
			onDelta(chunk)
		}
	}
	return f.reasoning, f.err
}

func (f *fakeScreeningSQLGenerator) GenerateSQLStream(
	ctx context.Context,
	prompt string,
	aiConfigID string,
	onDelta func(string),
) (string, error) {
	f.prompts = append(f.prompts, prompt)
	f.aiConfig = append(f.aiConfig, aiConfigID)
	if deadline, ok := ctx.Deadline(); ok {
		f.observedHasDeadline = true
		f.observedTimeout = time.Until(deadline).Round(time.Second)
	}
	for _, chunk := range f.streamChunks {
		if onDelta != nil {
			onDelta(chunk)
		}
	}
	return f.sql, f.err
}

func seedScreeningQueryFixture(t *testing.T, store *ScreeningStore) {
	t.Helper()

	statements := []string{
		`INSERT INTO stocks_basic (symbol, name, market, industry, list_date, is_st, is_active) VALUES
			('sh600000', '浦发银行', '上海', '银行', '1999-11-10', 0, 1),
			('sz000001', '平安银行', '深圳', '银行', '1991-04-03', 0, 1),
			('sh600519', '贵州茅台', '上海', '白酒', '2001-08-27', 0, 1)`,
		`INSERT INTO daily_bars (symbol, trade_date, open, high, low, close, volume, amount) VALUES
			('sh600000', '2026-03-19', 10, 11, 9.8, 11, 1000, 10000),
			('sz000001', '2026-03-19', 20, 21, 19.8, 21, 2000, 20000),
			('sh600519', '2026-03-19', 30, 30.5, 29.5, 30.2, 3000, 30000)`,
		`INSERT INTO daily_snapshots (symbol, trade_date, change, change_percent, amplitude, turnover_rate, price) VALUES
			('sh600000', '2026-03-19', 1, 9.5, 12.0, 0, 11),
			('sz000001', '2026-03-19', 1, 5.2, 6.0, 0, 21),
			('sh600519', '2026-03-19', 0.2, 0.7, 2.5, 0, 30.2)`,
	}

	for _, statement := range statements {
		if _, err := store.db.Exec(statement); err != nil {
			t.Fatalf("seed screening query fixture error = %v", err)
		}
	}
}
