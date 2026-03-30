package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/logger"
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

func TestFetchKLineDataFallsBackToEastmoneyWhenSinaReturnsBlocked(t *testing.T) {
	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(req.URL.Host, "quotes.sina.cn"):
					return &http.Response{
						StatusCode: 456,
						Status:     "456 blocked",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><body><h1>拒绝访问</h1></body></html>`)),
						Request:    req,
					}, nil
				case strings.Contains(req.URL.Host, "push2his.eastmoney.com"):
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(
							`{"rc":0,"data":{"code":"600519","name":"贵州茅台","klines":["2026-03-17,1468.00,1485.00,1498.07,1461.19,49454,7347310632.00,2.53,1.70,24.82,0.39","2026-03-18,1489.00,1468.80,1496.50,1463.15,35551,5239992522.00,2.25,-1.09,-16.20,0.28"]}}`,
						)),
						Request: req,
					}, nil
				default:
					return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
				}
			}),
		},
	}

	got, err := ms.fetchKLineData("sh600519", "1d", 30)
	if err != nil {
		t.Fatalf("fetchKLineData() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(fetchKLineData()) = %d, want 2", len(got))
	}
	if got[0].Time != "2026-03-17" || got[0].Close != 1485 || got[0].Volume != 49454 {
		t.Fatalf("fetchKLineData() first row = %#v", got[0])
	}
}

func TestGetStockRealTimeDataReturnsHTTPStatusErrorBeforeParsing(t *testing.T) {
	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 503,
					Status:     "503 service unavailable",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`temporarily blocked`)),
					Request:    req,
				}, nil
			}),
		},
	}

	_, err := ms.GetStockRealTimeData("sh600000")
	if err == nil {
		t.Fatalf("GetStockRealTimeData() error = nil, want status error")
	}

	got := err.Error()
	if !strings.Contains(got, "realtime api status 503") {
		t.Fatalf("GetStockRealTimeData() error = %q, want status code context", got)
	}
}

func TestGetRealOrderBookReturnsHTTPStatusErrorBeforeParsing(t *testing.T) {
	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 502,
					Status:     "502 bad gateway",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`gateway timeout`)),
					Request:    req,
				}, nil
			}),
		},
		cache:      make(map[string]*stockCache),
		cacheTTL:   2 * time.Second,
		klineCache: make(map[string]*klineCache),
	}

	_, err := ms.GetRealOrderBook("sz000001")
	if err == nil {
		t.Fatalf("GetRealOrderBook() error = nil, want status error")
	}

	got := err.Error()
	if !strings.Contains(got, "orderbook api status 502") {
		t.Fatalf("GetRealOrderBook() error = %q, want status code context", got)
	}
}

func TestGetKLineDataWithRequestLogsRequestID(t *testing.T) {
	t.Cleanup(func() {
		logger.Close()
		logger.SetGlobalLevel(logger.INFO)
		logger.SetConsoleOutput(true)
	})

	logger.SetConsoleOutput(false)
	logger.SetGlobalLevel(logger.DEBUG)
	logDir := t.TempDir()
	if err := logger.InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`[{"day":"2026-03-18","open":"3.040","high":"3.090","low":"3.030","close":"3.070","volume":"10966540"}]`,
					)),
					Request: req,
				}, nil
			}),
		},
		klineCache:    make(map[string]*klineCache),
		klineCacheTTL: 30 * time.Second,
	}

	_, err := ms.GetKLineDataWithRequest("kline-7", "sz000541", "1d", 30)
	if err != nil {
		t.Fatalf("GetKLineDataWithRequest() error = %v", err)
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "requestId=kline-7") {
		t.Fatalf("log file = %q, want requestId", text)
	}
}

func TestGetKLineDataWithRequestLogsFallbackDiagnostics(t *testing.T) {
	t.Cleanup(func() {
		logger.Close()
		logger.SetGlobalLevel(logger.INFO)
		logger.SetConsoleOutput(true)
	})

	logger.SetConsoleOutput(false)
	logger.SetGlobalLevel(logger.DEBUG)
	logDir := t.TempDir()
	if err := logger.InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(req.URL.Host, "quotes.sina.cn"):
					return &http.Response{
						StatusCode: 456,
						Status:     "456 blocked",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><body><h1>拒绝访问</h1></body></html>`)),
						Request:    req,
					}, nil
				case strings.Contains(req.URL.Host, "push2his.eastmoney.com"):
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(
							`{"rc":0,"data":{"code":"600519","name":"贵州茅台","klines":["2026-03-17,1468.00,1485.00,1498.07,1461.19,49454,7347310632.00,2.53,1.70,24.82,0.39"]}}`,
						)),
						Request: req,
					}, nil
				default:
					return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
				}
			}),
		},
		klineCache:    make(map[string]*klineCache),
		klineCacheTTL: 30 * time.Second,
	}

	if _, err := ms.GetKLineDataWithRequest("kline-fallback-1", "sh600519", "1d", 30); err != nil {
		t.Fatalf("GetKLineDataWithRequest() error = %v", err)
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)

	for _, want := range []string{
		"module=market action=kline.fetch.fallback.start",
		"requestId=kline-fallback-1",
		"source=sina",
		"fallback=eastmoney",
		"status=456",
		"sinaUrl=http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData?symbol=sh600519&scale=240&ma=5,10,20&datalen=30",
		"responsePreview=\"<!doctype html><html><body><h1>拒绝访问</h1></body></html>\"",
		"module=market action=kline.fetch.fallback.success",
		"source=eastmoney",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("log file = %q, want %q", text, want)
		}
	}
}

func TestMarketSummaryLogsRemainVisibleAtInfoLevel(t *testing.T) {
	t.Cleanup(func() {
		logger.Close()
		logger.SetGlobalLevel(logger.INFO)
		logger.SetConsoleOutput(true)
	})

	logger.SetConsoleOutput(false)
	logger.SetGlobalLevel(logger.INFO)
	logDir := t.TempDir()
	if err := logger.InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(req.URL.String(), "CN_MarketDataService.getKLineData"):
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(
							`[{"day":"2026-03-18","open":"3.040","high":"3.090","low":"3.030","close":"3.070","volume":"10966540"}]`,
						)),
						Request: req,
					}, nil
				default:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(
							`var hq_str_sz000001="平安银行,10.00,9.90,10.10,10.20,9.80,10.09,10.10,10000,200000,100,10.09,200,10.08,300,10.07,400,10.06,500,10.05,100,10.10,200,10.11,300,10.12,400,10.13,500,10.14,2026-03-30,15:00:00";`,
						)),
						Request: req,
					}, nil
				}
			}),
		},
		cache:         make(map[string]*stockCache),
		cacheTTL:      2 * time.Second,
		klineCache:    make(map[string]*klineCache),
		klineCacheTTL: 30 * time.Second,
	}

	if _, err := ms.GetStockRealTimeDataWithRequest("realtime-1", "sz000001"); err != nil {
		t.Fatalf("GetStockRealTimeDataWithRequest() error = %v", err)
	}
	if _, err := ms.GetKLineDataWithRequest("kline-1", "sz000001", "1d", 30); err != nil {
		t.Fatalf("GetKLineDataWithRequest() error = %v", err)
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)

	for _, want := range []string{
		"module=market action=realtime.fetch.start requestId=realtime-1",
		"module=market action=realtime.fetch.success requestId=realtime-1",
		"module=market action=kline.fetch.start requestId=kline-1",
		"module=market action=kline.fetch.success requestId=kline-1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("log file = %q, want %q", text, want)
		}
	}
}

func TestFetchScreeningKLineDataUsesNewSinaEndpoint(t *testing.T) {
	ms := &MarketService{
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=sz002231&scale=240&ma=no&datalen=120" {
					t.Fatalf("url = %q", req.URL.String())
				}
				if got := req.Header.Get("Referer"); got != "http://finance.sina.com.cn" {
					t.Fatalf("Referer = %q, want finance referer", got)
				}
				if got := req.Header.Get("User-Agent"); got == "" {
					t.Fatal("User-Agent = empty, want request header")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`[{"day":"2026-03-18","open":"3.040","high":"3.090","low":"3.030","close":"3.070","volume":"10966540"}]`,
					)),
					Request: req,
				}, nil
			}),
		},
	}

	got, err := ms.fetchScreeningKLineData("sz002231", 120)
	if err != nil {
		t.Fatalf("fetchScreeningKLineData() error = %v", err)
	}
	if len(got) != 1 || got[0].Time != "2026-03-18" || got[0].Close != 3.07 || got[0].Volume != 10966540 {
		t.Fatalf("fetchScreeningKLineData() = %#v", got)
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

func TestParseBaoStockKLinesAllowsEmptyVolumeAndAmount(t *testing.T) {
	rows := [][]string{
		{"2026-01-29", "3.04", "3.09", "3.03", "3.07", "", ""},
	}

	got, err := parseBaoStockKLines(rows)
	if err != nil {
		t.Fatalf("parseBaoStockKLines() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(parseBaoStockKLines()) = %d, want 1", len(got))
	}
	if got[0].Volume != 0 || got[0].Amount != 0 {
		t.Fatalf("parseBaoStockKLines() row = %#v, want zero volume/amount", got[0])
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
