package services

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchKLineDataReturnsHTTPStatusErrorBeforeJSONParsing(t *testing.T) {
	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 456,
					Status:     "456 blocked",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><body><h1>拒绝访问</h1></body></html>`)),
					Request:    req,
				}, nil
			}),
		},
	}

	_, err := ms.fetchKLineData("sz000541", "1d", 30)
	if err == nil {
		t.Fatalf("fetchKLineData() error = nil, want status error")
	}

	got := err.Error()
	if !strings.Contains(got, "kline api status 456") {
		t.Fatalf("fetchKLineData() error = %q, want status code context", got)
	}
	if strings.Contains(got, "invalid character '<'") {
		t.Fatalf("fetchKLineData() error = %q, should not expose raw json parse error", got)
	}
}

func TestGetScreeningDailyBarsUsesScreeningSource(t *testing.T) {
	want := []models.KLineData{{Time: "2026-03-18", Close: 12.3}}
	source := screeningDailyBarSourceFunc(func(symbol string, lookbackDays int, _ ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
		if symbol != "sh600000" {
			t.Fatalf("symbol = %q, want sh600000", symbol)
		}
		if lookbackDays != 30 {
			t.Fatalf("lookbackDays = %d, want 30", lookbackDays)
		}
		return want, nil
	})

	ms := &MarketService{
		screeningDailyBarSource: source,
	}

	got, err := ms.GetScreeningDailyBars("sh600000", 30)
	if err != nil {
		t.Fatalf("GetScreeningDailyBars() error = %v", err)
	}
	if len(got) != len(want) || got[0].Close != want[0].Close || got[0].Time != want[0].Time {
		t.Fatalf("GetScreeningDailyBars() = %#v, want %#v", got, want)
	}
}

func TestGetScreeningDailyBarsFallsBackToSinaWhenBaostockFails(t *testing.T) {
	want := []models.KLineData{{Time: "2026-03-18", Close: 11.8}}
	baostock := &stubScreeningDailyBarSource{
		err: errors.New("baostock unavailable"),
	}
	sina := &stubScreeningDailyBarSource{
		bars: want,
	}

	source := newScreeningDailyBarSourceChain(baostock, sina)
	got, err := source.Fetch("sh600000", 20, nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if baostock.calls != 1 {
		t.Fatalf("baostock calls = %d, want 1", baostock.calls)
	}
	if sina.calls != 1 {
		t.Fatalf("sina calls = %d, want 1", sina.calls)
	}
	if len(got) != 1 || got[0].Close != want[0].Close {
		t.Fatalf("Fetch() = %#v, want %#v", got, want)
	}
}

func TestGetScreeningDailyBarsStopsAfterBaostockSuccess(t *testing.T) {
	want := []models.KLineData{{Time: "2026-03-18", Close: 15.2}}
	baostock := &stubScreeningDailyBarSource{
		bars: want,
	}
	sina := &stubScreeningDailyBarSource{
		bars: []models.KLineData{{Time: "2026-03-18", Close: 1}},
	}

	source := newScreeningDailyBarSourceChain(baostock, sina)
	got, err := source.Fetch("sz000001", 15, nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if baostock.calls != 1 {
		t.Fatalf("baostock calls = %d, want 1", baostock.calls)
	}
	if sina.calls != 0 {
		t.Fatalf("sina calls = %d, want 0", sina.calls)
	}
	if len(got) != 1 || got[0].Close != want[0].Close {
		t.Fatalf("Fetch() = %#v, want %#v", got, want)
	}
}

func TestGetScreeningDailyBarsReturnsCombinedErrorWhenAllSourcesFail(t *testing.T) {
	source := newScreeningDailyBarSourceChain(
		&stubScreeningDailyBarSource{err: errors.New("baostock timeout")},
		&stubScreeningDailyBarSource{err: errors.New("sina blocked")},
	)

	_, err := source.Fetch("sh600000", 30, nil)
	if err == nil {
		t.Fatalf("Fetch() error = nil, want combined error")
	}
	got := err.Error()
	if !strings.Contains(got, "baostock") || !strings.Contains(got, "sina") {
		t.Fatalf("Fetch() error = %q, want both source contexts", got)
	}
}

func TestBaoStockSymbolConversion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "shanghai", input: "sh600000", want: "sh.600000"},
		{name: "shenzhen", input: "sz000001", want: "sz.000001"},
		{name: "invalid", input: "bj430001", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBaoStockSymbol(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeBaoStockSymbol() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeBaoStockSymbol() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeBaoStockSymbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBaoStockKLines(t *testing.T) {
	rows := [][]string{
		{"2026-03-17", "10.0", "10.5", "9.8", "10.2", "1000", "20000"},
		{"2026-03-18", "10.2", "10.8", "10.1", "10.6", "1200", "24000"},
	}

	got, err := parseBaoStockKLines(rows)
	if err != nil {
		t.Fatalf("parseBaoStockKLines() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(parseBaoStockKLines()) = %d, want 2", len(got))
	}
	if got[1].Time != "2026-03-18" || got[1].Close != 10.6 || got[1].Volume != 1200 {
		t.Fatalf("parseBaoStockKLines() second row = %#v", got[1])
	}
}

func TestMarketServiceListScreeningStocksExcludesChiNextButKeepsChiNextIndex(t *testing.T) {
	ms := &MarketService{}

	stocks, err := ms.ListScreeningStocks(models.ScreeningMarketScopeConfig{
		Shanghai: true,
		Shenzhen: true,
		Indices:  true,
	})
	if err != nil {
		t.Fatalf("ListScreeningStocks() error = %v", err)
	}

	foundMainBoard := false
	foundChiNextStock := false
	foundChiNext301Stock := false
	foundChiNextIndex := false

	for _, stock := range stocks {
		switch stock.Symbol {
		case "sz000001":
			foundMainBoard = true
		case "sz300001":
			foundChiNextStock = true
		case "sz301011":
			foundChiNext301Stock = true
		case "sz399006":
			foundChiNextIndex = true
		}
	}

	if !foundMainBoard {
		t.Fatalf("ListScreeningStocks() missing non-ChiNext Shenzhen stock sz000001")
	}
	if foundChiNextStock {
		t.Fatalf("ListScreeningStocks() unexpectedly contains ChiNext stock sz300001")
	}
	if foundChiNext301Stock {
		t.Fatalf("ListScreeningStocks() unexpectedly contains ChiNext stock sz301011")
	}
	if !foundChiNextIndex {
		t.Fatalf("ListScreeningStocks() missing ChiNext index sz399006 when indices are enabled")
	}
}

type stubScreeningDailyBarSource struct {
	bars  []models.KLineData
	err   error
	calls int
}

func (s *stubScreeningDailyBarSource) Fetch(_ string, _ int, _ ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]models.KLineData(nil), s.bars...), nil
}
