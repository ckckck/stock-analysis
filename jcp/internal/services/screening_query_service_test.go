package services

import (
	"context"
	"strings"
	"testing"
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

type fakeScreeningSQLGenerator struct {
	sql      string
	prompts  []string
	aiConfig []string
	err      error
}

func (f *fakeScreeningSQLGenerator) GenerateSQL(_ context.Context, prompt string, aiConfigID string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	f.aiConfig = append(f.aiConfig, aiConfigID)
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
