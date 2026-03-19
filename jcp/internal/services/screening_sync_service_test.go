package services

import (
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

func TestScreeningSyncFirstSyncUsesConfiguredWindow(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 3
	config.Screening.RetentionMode = models.ScreeningRetentionModeForever
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", Industry: "银行", ListDate: "1991-04-03", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-14", []float64{10, 11, 12, 13, 14}),
			"sz000001": makeDailyBars("2026-03-14", []float64{20, 21, 22, 23, 24}),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 14, Change: 1, ChangePercent: 7.6, Volume: 1000, Amount: 100000, Open: 13, High: 14, Low: 12.5, PreClose: 13},
			"sz000001": {Symbol: "sz000001", Name: "平安银行", Price: 24, Change: 1, ChangePercent: 4.3, Volume: 2000, Amount: 200000, Open: 23, High: 24, Low: 22.5, PreClose: 23},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.Sync()
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if status.LastTradeDate != "2026-03-18" {
		t.Fatalf("status.LastTradeDate = %q, want %q", status.LastTradeDate, "2026-03-18")
	}
	if status.StocksSynced != 2 || status.BarsSynced != 6 || status.SnapshotsSynced != 6 {
		t.Fatalf("status = %#v", status)
	}
	if got := source.lastLookbackDays["sh600000"]; got != 3 {
		t.Fatalf("lookbackDays for sh600000 = %d, want 3", got)
	}

	assertTableCount(t, store, "stocks_basic", 2)
	assertTableCount(t, store, "daily_bars", 6)
	assertTableCount(t, store, "daily_snapshots", 6)
	assertLastTradeDate(t, store, screeningSyncStateDataset, "shanghai,shenzhen", "2026-03-18")
}

func TestScreeningSyncIncrementalSyncUsesSyncState(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 2
	config.Screening.RetentionMode = models.ScreeningRetentionModeForever
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
			{Symbol: "sz000001", Name: "平安银行", Market: "深圳", Industry: "银行", ListDate: "1991-04-03", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-03-16", []float64{10, 11}),
			"sz000001": makeDailyBars("2026-03-16", []float64{20, 21}),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 11, Change: 1, ChangePercent: 10, Volume: 1000, Amount: 100000, Open: 10, High: 11, Low: 9.8, PreClose: 10},
			"sz000001": {Symbol: "sz000001", Name: "平安银行", Price: 21, Change: 1, ChangePercent: 5, Volume: 2000, Amount: 200000, Open: 20, High: 21, Low: 19.8, PreClose: 20},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 17, 16, 0, 0, 0, time.UTC)
	}
	if _, err := svc.Sync(); err != nil {
		t.Fatalf("first Sync() error = %v", err)
	}

	source.bars["sh600000"] = makeDailyBars("2026-03-16", []float64{10, 11, 12})
	source.bars["sz000001"] = makeDailyBars("2026-03-16", []float64{20, 21, 22})
	delete(source.snapshots, "sh600000")
	source.snapshots["sz000001"] = models.Stock{Symbol: "sz000001", Name: "平安银行", Price: 22, Change: 1, ChangePercent: 4.8, Volume: 2200, Amount: 220000, Open: 21, High: 22, Low: 20.8, PreClose: 21}
	svc.now = func() time.Time {
		return time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	}

	status, err := svc.Sync()
	if err != nil {
		t.Fatalf("second Sync() error = %v", err)
	}

	if status.LastTradeDate != "2026-03-18" {
		t.Fatalf("status.LastTradeDate = %q, want %q", status.LastTradeDate, "2026-03-18")
	}
	if status.BarsSynced != 2 {
		t.Fatalf("status.BarsSynced = %d, want 2", status.BarsSynced)
	}
	if got := source.lastLookbackDays["sh600000"]; got <= 2 {
		t.Fatalf("incremental lookbackDays = %d, want > 2", got)
	}

	assertTableCount(t, store, "daily_bars", 6)
	assertSnapshotChange(t, store, "sh600000", "2026-03-18", 1)
	assertLastTradeDate(t, store, screeningSyncStateDataset, "shanghai,shenzhen", "2026-03-18")
}

func TestScreeningSyncRetentionCleanupKeepsConfiguredDays(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 40
	config.Screening.RetentionMode = models.ScreeningRetentionModeDays
	config.Screening.RetentionDays = 30
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-02-01", makeCloseValues(40, 10)),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 49, Change: 1, ChangePercent: 2.1, Volume: 1000, Amount: 100000, Open: 48, High: 49, Low: 47.5, PreClose: 48},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 12, 16, 0, 0, 0, time.UTC)
	}

	if _, err := svc.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertTradeDateBounds(t, store, "daily_bars", "2026-02-10", "2026-03-12")
	assertTradeDateBounds(t, store, "daily_snapshots", "2026-02-10", "2026-03-12")
}

func TestScreeningSyncRetentionCleanupKeepsConfiguredSixtyDays(t *testing.T) {
	configService, store := newScreeningSyncTestServices(t)

	config := configService.GetConfig()
	config.Screening.InitialSyncDays = 71
	config.Screening.RetentionMode = models.ScreeningRetentionModeDays
	config.Screening.RetentionDays = 60
	if err := configService.UpdateConfig(config); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	source := &fakeScreeningMarketSource{
		stocks: []ScreeningStockBasic{
			{Symbol: "sh600000", Name: "浦发银行", Market: "上海", Industry: "银行", ListDate: "1999-11-10", IsActive: true},
		},
		bars: map[string][]models.KLineData{
			"sh600000": makeDailyBars("2026-01-01", makeCloseValues(71, 10)),
		},
		snapshots: map[string]models.Stock{
			"sh600000": {Symbol: "sh600000", Name: "浦发银行", Price: 80, Change: 1, ChangePercent: 1.2, Volume: 1000, Amount: 100000, Open: 79, High: 80, Low: 78.5, PreClose: 79},
		},
	}

	svc := NewScreeningSyncService(configService, store, source)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 12, 16, 0, 0, 0, time.UTC)
	}

	if _, err := svc.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertTradeDateBounds(t, store, "daily_bars", "2026-01-11", "2026-03-12")
	assertTradeDateBounds(t, store, "daily_snapshots", "2026-01-11", "2026-03-12")
}

type fakeScreeningMarketSource struct {
	stocks           []ScreeningStockBasic
	bars             map[string][]models.KLineData
	snapshots        map[string]models.Stock
	lastLookbackDays map[string]int
}

func (f *fakeScreeningMarketSource) ListScreeningStocks(_ models.ScreeningMarketScopeConfig) ([]ScreeningStockBasic, error) {
	return append([]ScreeningStockBasic(nil), f.stocks...), nil
}

func (f *fakeScreeningMarketSource) GetScreeningDailyBars(symbol string, lookbackDays int) ([]models.KLineData, error) {
	if f.lastLookbackDays == nil {
		f.lastLookbackDays = make(map[string]int)
	}
	f.lastLookbackDays[symbol] = lookbackDays

	allBars := append([]models.KLineData(nil), f.bars[symbol]...)
	if lookbackDays >= len(allBars) {
		return allBars, nil
	}
	return allBars[len(allBars)-lookbackDays:], nil
}

func (f *fakeScreeningMarketSource) GetScreeningSnapshots(symbols []string) ([]models.Stock, error) {
	results := make([]models.Stock, 0, len(symbols))
	for _, symbol := range symbols {
		if snapshot, ok := f.snapshots[symbol]; ok {
			results = append(results, snapshot)
		}
	}
	return results, nil
}

func newScreeningSyncTestServices(t *testing.T) (*ConfigService, *ScreeningStore) {
	t.Helper()

	tempDir := t.TempDir()
	configService, err := NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}
	store, err := NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return configService, store
}

func makeDailyBars(startDate string, closes []float64) []models.KLineData {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		panic(err)
	}

	bars := make([]models.KLineData, 0, len(closes))
	for i, closePrice := range closes {
		day := start.AddDate(0, 0, i)
		bars = append(bars, models.KLineData{
			Time:   day.Format("2006-01-02"),
			Open:   closePrice - 0.5,
			High:   closePrice + 0.5,
			Low:    closePrice - 1,
			Close:  closePrice,
			Volume: int64(1000 + i*100),
			Amount: float64(10000 + i*1000),
		})
	}
	return bars
}

func makeCloseValues(count int, start float64) []float64 {
	values := make([]float64, count)
	for i := range values {
		values[i] = start + float64(i)
	}
	return values
}

func assertTableCount(t *testing.T, store *ScreeningStore, tableName string, want int) {
	t.Helper()

	var got int
	if err := store.db.QueryRow(`SELECT COUNT(1) FROM ` + tableName).Scan(&got); err != nil {
		t.Fatalf("count %s error = %v", tableName, err)
	}
	if got != want {
		t.Fatalf("count(%s) = %d, want %d", tableName, got, want)
	}
}

func assertLastTradeDate(t *testing.T, store *ScreeningStore, dataset, scope, want string) {
	t.Helper()

	state, err := store.GetSyncState(dataset, scope)
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if state == nil {
		t.Fatalf("GetSyncState() = nil, want %q", want)
	}
	if state.LastTradeDate != want {
		t.Fatalf("LastTradeDate = %q, want %q", state.LastTradeDate, want)
	}
}

func assertTradeDateBounds(t *testing.T, store *ScreeningStore, tableName, wantMin, wantMax string) {
	t.Helper()

	var minDate string
	var maxDate string
	if err := store.db.QueryRow(`SELECT MIN(trade_date), MAX(trade_date) FROM `+tableName).Scan(&minDate, &maxDate); err != nil {
		t.Fatalf("trade date bounds for %s error = %v", tableName, err)
	}
	if minDate != wantMin || maxDate != wantMax {
		t.Fatalf("%s bounds = [%s, %s], want [%s, %s]", tableName, minDate, maxDate, wantMin, wantMax)
	}
}

func assertSnapshotChange(t *testing.T, store *ScreeningStore, symbol, tradeDate string, want float64) {
	t.Helper()

	var got float64
	if err := store.db.QueryRow(
		`SELECT change FROM daily_snapshots WHERE symbol = ? AND trade_date = ?`,
		symbol,
		tradeDate,
	).Scan(&got); err != nil {
		t.Fatalf("snapshot change for %s %s error = %v", symbol, tradeDate, err)
	}
	if got != want {
		t.Fatalf("snapshot change for %s %s = %v, want %v", symbol, tradeDate, got, want)
	}
}
