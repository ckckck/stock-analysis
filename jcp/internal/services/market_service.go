package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/embed"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/models"
	"github.com/run-bigpig/jcp/internal/pkg/paths"
	"github.com/run-bigpig/jcp/internal/pkg/proxy"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var log = logger.New("market")

func requestIDField(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ""
	}
	return fmt.Sprintf(" requestId=%s", requestID)
}

// 预编译正则表达式，避免重复编译
var (
	sinaStockRegex = regexp.MustCompile(`var hq_str_(\w+)="([^"]*)"`)
	sinaIndexRegex = regexp.MustCompile(`var hq_str_s_(\w+)="([^"]*)"`)
)

const (
	sinaStockURL          = "http://hq.sinajs.cn/rn=%d&list=%s"
	sinaKLineURL          = "http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=5,10,20&datalen=%d"
	sinaScreeningKLineURL = "https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d"
	eastmoneyKLineURL     = "https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=%s&fqt=0&lmt=%d&end=20500101&iscca=1&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61"
)

const (
	klineCacheTTLIntraday = 2 * time.Second
	klineCacheTTLDefault  = 30 * time.Second
	klineStaleTTLDefault  = 24 * time.Hour
	klineFailureCooldown  = 20 * time.Second
	eastmoneyRetryDelay   = 150 * time.Millisecond
)

// 默认大盘指数代码
var defaultIndexCodes = []string{
	"s_sh000001", // 上证指数
	"s_sz399001", // 深证成指
	"s_sz399006", // 创业板指
}

var screeningIndexCatalog = []ScreeningStockBasic{
	{Symbol: "sh000001", Name: "上证指数", Market: "指数", Industry: "指数", IsActive: true},
	{Symbol: "sz399001", Name: "深证成指", Market: "指数", Industry: "指数", IsActive: true},
	{Symbol: "sz399006", Name: "创业板指", Market: "指数", Industry: "指数", IsActive: true},
}

// StockWithOrderBook 包含盘口数据的股票信息
type StockWithOrderBook struct {
	models.Stock
	OrderBook models.OrderBook `json:"orderBook"`
}

// stockCache 股票数据缓存
type stockCache struct {
	data      []StockWithOrderBook
	timestamp time.Time
}

// klineCache K线数据缓存
type klineCache struct {
	data      []models.KLineData
	timestamp time.Time
	ttl       time.Duration
}

type klineFailureState struct {
	lastFailureAt time.Time
	err           error
}

// MarketStatus 市场交易状态
type MarketStatus struct {
	Status      string `json:"status"`      // trading, closed, pre_market, lunch_break
	StatusText  string `json:"statusText"`  // 中文状态描述
	IsTradeDay  bool   `json:"isTradeDay"`  // 是否交易日
	HolidayName string `json:"holidayName"` // 节假日名称（如有）
}

// TradingPeriod 交易时段
type TradingPeriod struct {
	Status    string `json:"status"`    // 状态标识
	Text      string `json:"text"`      // 中文描述
	StartTime string `json:"startTime"` // 开始时间 HH:MM
	EndTime   string `json:"endTime"`   // 结束时间 HH:MM
}

// TradingSchedule 交易时间表
type TradingSchedule struct {
	IsTradeDay  bool            `json:"isTradeDay"`  // 今天是否交易日
	HolidayName string          `json:"holidayName"` // 节假日名称
	Periods     []TradingPeriod `json:"periods"`     // 时段列表
}

// MarketService 市场数据服务
type MarketService struct {
	client *http.Client

	screeningDailyBarSource screeningDailyBarSource

	// 股票数据缓存
	cache    map[string]*stockCache
	cacheMu  sync.RWMutex
	cacheTTL time.Duration

	// K线数据缓存
	klineCache    map[string]*klineCache
	klineCacheMu  sync.RWMutex
	klineCacheTTL time.Duration
	klineFailures   map[string]klineFailureState
	klineFailureMu  sync.RWMutex
}

// NewMarketService 创建市场数据服务
func NewMarketService() *MarketService {
	ms := &MarketService{
		client:        proxy.GetManager().GetClientWithTimeout(5 * time.Second),
		cache:         make(map[string]*stockCache),
		cacheTTL:      2 * time.Second, // 股票缓存2秒
		klineCache:    make(map[string]*klineCache),
		klineCacheTTL: klineCacheTTLDefault, // 日/周/月K使用较长缓存，减少API调用
		klineFailures: make(map[string]klineFailureState),
	}
	// 启动缓存清理协程
	go ms.cleanCacheLoop()
	return ms
}

// cleanCacheLoop 定期清理过期缓存，防止内存泄漏
func (ms *MarketService) cleanCacheLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ms.cleanExpiredCache()
	}
}

// cleanExpiredCache 清理过期缓存
func (ms *MarketService) cleanExpiredCache() {
	now := time.Now()

	// 清理股票缓存
	ms.cacheMu.Lock()
	for key, cached := range ms.cache {
		if now.Sub(cached.timestamp) > 10*time.Second {
			delete(ms.cache, key)
		}
	}
	ms.cacheMu.Unlock()

	// 清理K线缓存
	ms.klineCacheMu.Lock()
	for key, cached := range ms.klineCache {
		ttl := cached.ttl
		if ttl <= 0 {
			ttl = ms.klineCacheTTL
		}
		staleTTL := ms.getKLineStaleTTLFromTTL(ttl)
		// 使用更长的 stale TTL 做内存回收，尽量保留最后一次成功结果供失败时兜底
		if now.Sub(cached.timestamp) > staleTTL {
			delete(ms.klineCache, key)
		}
	}
	ms.klineCacheMu.Unlock()

	ms.klineFailureMu.Lock()
	for key, state := range ms.klineFailures {
		if now.Sub(state.lastFailureAt) > klineFailureCooldown {
			delete(ms.klineFailures, key)
		}
	}
	ms.klineFailureMu.Unlock()
}

// getKLineCacheTTL 返回不同周期的缓存策略
func (ms *MarketService) getKLineCacheTTL(period string) time.Duration {
	// 分时需要高时效，避免增量推送读取到过旧缓存
	if period == "1m" {
		return klineCacheTTLIntraday
	}
	return ms.klineCacheTTL
}

func (ms *MarketService) getKLineStaleTTL(period string) time.Duration {
	return ms.getKLineStaleTTLFromTTL(ms.getKLineCacheTTL(period))
}

func (ms *MarketService) getKLineStaleTTLFromTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		ttl = klineCacheTTLDefault
	}
	if ttl < klineStaleTTLDefault {
		return klineStaleTTLDefault
	}
	return ttl * 3
}

func klineCacheKey(code string, period string, days int) string {
	return fmt.Sprintf("%s:%s:%d", code, period, days)
}

func (ms *MarketService) shouldUseCooldownStaleCache(cacheKey string, staleCache []models.KLineData) (error, bool) {
	if len(staleCache) == 0 {
		return nil, false
	}
	ms.klineFailureMu.RLock()
	state, ok := ms.klineFailures[cacheKey]
	ms.klineFailureMu.RUnlock()
	if !ok || time.Since(state.lastFailureAt) > klineFailureCooldown {
		return nil, false
	}
	return state.err, true
}

func (ms *MarketService) noteKLineFetchFailure(code string, period string, days int, err error) {
	if err == nil {
		return
	}
	ms.klineFailureMu.Lock()
	if ms.klineFailures == nil {
		ms.klineFailures = make(map[string]klineFailureState)
	}
	ms.klineFailures[klineCacheKey(code, period, days)] = klineFailureState{
		lastFailureAt: time.Now(),
		err:           err,
	}
	ms.klineFailureMu.Unlock()
}

func (ms *MarketService) clearKLineFetchFailure(code string, period string, days int) {
	ms.klineFailureMu.Lock()
	delete(ms.klineFailures, klineCacheKey(code, period, days))
	ms.klineFailureMu.Unlock()
}

// GetStockDataWithOrderBook 获取股票实时数据（含真实盘口），带缓存
func (ms *MarketService) GetStockDataWithOrderBook(codes ...string) ([]StockWithOrderBook, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	// 排序codes保证缓存key一致性
	sortedCodes := make([]string, len(codes))
	copy(sortedCodes, codes)
	sort.Strings(sortedCodes)
	cacheKey := strings.Join(sortedCodes, ",")

	// 检查缓存
	ms.cacheMu.RLock()
	if cached, ok := ms.cache[cacheKey]; ok {
		if time.Since(cached.timestamp) < ms.cacheTTL {
			ms.cacheMu.RUnlock()
			return cached.data, nil
		}
	}
	ms.cacheMu.RUnlock()

	// 从API获取数据
	data, err := ms.fetchStockDataWithOrderBook(codes...)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	ms.cacheMu.Lock()
	ms.cache[cacheKey] = &stockCache{
		data:      data,
		timestamp: time.Now(),
	}
	ms.cacheMu.Unlock()

	return data, nil
}

// fetchStockDataWithOrderBook 从API获取股票数据（含盘口）
func (ms *MarketService) fetchStockDataWithOrderBook(codes ...string) ([]StockWithOrderBook, error) {
	codeList := strings.Join(codes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)
	startedAt := time.Now()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Warn("module=market action=orderbook.fetch.request_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		log.Warn("module=market action=orderbook.fetch.http_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		log.Warn("module=market action=orderbook.fetch.read_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("orderbook api status %d for %s: %s", resp.StatusCode, codeList, httpErrorPreview(body))
		log.Warn("module=market action=orderbook.fetch.http_status_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}

	return ms.parseSinaStockDataWithOrderBook(string(body))
}

// parseSinaStockDataWithOrderBook 解析新浪股票数据（含盘口）
func (ms *MarketService) parseSinaStockDataWithOrderBook(data string) ([]StockWithOrderBook, error) {
	var stocks []StockWithOrderBook
	matches := sinaStockRegex.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 32 {
			continue
		}
		stock := ms.parseStockWithOrderBook(match[1], parts)
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// GetStockRealTimeData 获取股票实时数据
func (ms *MarketService) GetStockRealTimeData(codes ...string) ([]models.Stock, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	startedAt := time.Now()
	log.Info("module=market action=realtime.fetch.start codes=%d symbols=%s", len(codes), strings.Join(codes, ","))

	codeList := strings.Join(codes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Warn("module=market action=realtime.fetch.request_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		log.Warn("module=market action=realtime.fetch.http_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		log.Warn("module=market action=realtime.fetch.read_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("realtime api status %d for %s: %s", resp.StatusCode, codeList, httpErrorPreview(body))
		log.Warn("module=market action=realtime.fetch.http_status_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}

	stocks, err := ms.parseSinaStockData(string(body), codes)
	if err != nil {
		log.Warn("module=market action=realtime.fetch.parse_failed codes=%d durationMs=%d err=%v", len(codes), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	log.Info("module=market action=realtime.fetch.success codes=%d resultLen=%d durationMs=%d", len(codes), len(stocks), time.Since(startedAt).Milliseconds())
	return stocks, nil
}

// GetStockRealTimeDataWithRequest 获取股票实时数据（带 requestId）
func (ms *MarketService) GetStockRealTimeDataWithRequest(requestID string, codes ...string) ([]models.Stock, error) {
	requestField := requestIDField(requestID)
	if len(codes) == 0 {
		return nil, nil
	}
	startedAt := time.Now()
	log.Info("module=market action=realtime.fetch.start%s codes=%d symbols=%s", requestField, len(codes), strings.Join(codes, ","))
	stocks, err := ms.GetStockRealTimeData(codes...)
	if err != nil {
		log.Warn("module=market action=realtime.fetch.failed%s codes=%d symbols=%s durationMs=%d err=%v", requestField, len(codes), strings.Join(codes, ","), time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	log.Info("module=market action=realtime.fetch.success%s codes=%d resultLen=%d durationMs=%d", requestField, len(codes), len(stocks), time.Since(startedAt).Milliseconds())
	return stocks, nil
}

// parseSinaStockData 解析新浪股票数据
func (ms *MarketService) parseSinaStockData(data string, codes []string) ([]models.Stock, error) {
	var stocks []models.Stock
	matches := sinaStockRegex.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 32 {
			continue
		}

		stock := ms.parseStockFields(match[1], parts)
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// parseStockFields 解析股票字段
func (ms *MarketService) parseStockFields(code string, parts []string) models.Stock {
	price, _ := strconv.ParseFloat(parts[3], 64)
	open, _ := strconv.ParseFloat(parts[1], 64)
	high, _ := strconv.ParseFloat(parts[4], 64)
	low, _ := strconv.ParseFloat(parts[5], 64)
	preClose, _ := strconv.ParseFloat(parts[2], 64)
	volume, _ := strconv.ParseInt(parts[8], 10, 64)
	amount, _ := strconv.ParseFloat(parts[9], 64)

	change := price - preClose
	changePercent := 0.0
	if preClose > 0 {
		changePercent = (change / preClose) * 100
	}

	return models.Stock{
		Symbol:        code,
		Name:          parts[0],
		Price:         price,
		Open:          open,
		High:          high,
		Low:           low,
		PreClose:      preClose,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Amount:        amount,
	}
}

// parseStockWithOrderBook 解析股票字段和真实盘口数据
// 新浪API返回数据格式: 名称,今开,昨收,当前价,最高,最低,买一价,卖一价,成交量,成交额,
// 买一量,买一价,买二量,买二价,买三量,买三价,买四量,买四价,买五量,买五价,
// 卖一量,卖一价,卖二量,卖二价,卖三量,卖三价,卖四量,卖四价,卖五量,卖五价,日期,时间
func (ms *MarketService) parseStockWithOrderBook(code string, parts []string) StockWithOrderBook {
	stock := ms.parseStockFields(code, parts)

	// 解析真实五档盘口数据
	var bids, asks []models.OrderBookItem

	// 买盘数据 (索引 10-19: 买一量,买一价,买二量,买二价...)
	if len(parts) >= 20 {
		for i := 0; i < 5; i++ {
			volIdx := 10 + i*2
			priceIdx := 11 + i*2
			if priceIdx < len(parts) {
				bidVol, _ := strconv.ParseInt(parts[volIdx], 10, 64)
				bidPrice, _ := strconv.ParseFloat(parts[priceIdx], 64)
				if bidPrice > 0 {
					bids = append(bids, models.OrderBookItem{
						Price: bidPrice,
						Size:  bidVol / 100, // 转换为手
					})
				}
			}
		}
	}

	// 卖盘数据 (索引 20-29: 卖一量,卖一价,卖二量,卖二价...)
	if len(parts) >= 30 {
		for i := 0; i < 5; i++ {
			volIdx := 20 + i*2
			priceIdx := 21 + i*2
			if priceIdx < len(parts) {
				askVol, _ := strconv.ParseInt(parts[volIdx], 10, 64)
				askPrice, _ := strconv.ParseFloat(parts[priceIdx], 64)
				if askPrice > 0 {
					asks = append(asks, models.OrderBookItem{
						Price: askPrice,
						Size:  askVol / 100, // 转换为手
					})
				}
			}
		}
	}

	// 计算累计量和占比
	ms.calculateOrderBookTotals(bids)
	ms.calculateOrderBookTotals(asks)

	return StockWithOrderBook{
		Stock:     stock,
		OrderBook: models.OrderBook{Bids: bids, Asks: asks},
	}
}

// calculateOrderBookTotals 计算盘口累计量和占比
func (ms *MarketService) calculateOrderBookTotals(items []models.OrderBookItem) {
	if len(items) == 0 {
		return
	}

	var total int64
	var maxSize int64
	for _, item := range items {
		if item.Size > maxSize {
			maxSize = item.Size
		}
	}

	for i := range items {
		total += items[i].Size
		items[i].Total = total
		if maxSize > 0 {
			items[i].Percent = float64(items[i].Size) / float64(maxSize)
		}
	}
}

// GetKLineData 获取K线数据（带缓存）
func (ms *MarketService) GetKLineData(code string, period string, days int) ([]models.KLineData, error) {
	return ms.getKLineDataWithRequestID("", code, period, days)
}

func (ms *MarketService) getKLineDataWithRequestID(requestID string, code string, period string, days int) ([]models.KLineData, error) {
	cacheKey := klineCacheKey(code, period, days)
	ttl := ms.getKLineCacheTTL(period)
	staleTTL := ms.getKLineStaleTTL(period)
	var staleCache []models.KLineData

	// 检查缓存
	ms.klineCacheMu.RLock()
	if cached, ok := ms.klineCache[cacheKey]; ok {
		cachedTTL := cached.ttl
		if cachedTTL <= 0 {
			cachedTTL = ttl
		}
		if time.Since(cached.timestamp) < cachedTTL {
			ms.klineCacheMu.RUnlock()
			return cached.data, nil
		}
		if time.Since(cached.timestamp) < staleTTL {
			staleCache = append([]models.KLineData(nil), cached.data...)
		}
	}
	ms.klineCacheMu.RUnlock()
	if cooldownErr, shouldUseCooldown := ms.shouldUseCooldownStaleCache(cacheKey, staleCache); shouldUseCooldown {
		log.Warn("module=market action=kline.fetch.cooldown_stale_cache_used%s symbol=%s period=%s days=%d staleResultLen=%d err=%v", requestIDField(requestID), code, period, days, len(staleCache), cooldownErr)
		return staleCache, nil
	}
	log.Info("module=market action=kline.fetch.start symbol=%s period=%s days=%d", code, period, days)

	// 从API获取数据
	klines, err := ms.fetchKLineDataWithRequestID(requestID, code, period, days)
	if err != nil {
		ms.noteKLineFetchFailure(code, period, days, err)
		if len(staleCache) > 0 {
			log.Warn("module=market action=kline.fetch.stale_cache_used%s symbol=%s period=%s days=%d staleResultLen=%d err=%v", requestIDField(requestID), code, period, days, len(staleCache), err)
			return staleCache, nil
		}
		log.Warn("module=market action=kline.fetch.failed symbol=%s period=%s days=%d err=%v", code, period, days, err)
		return nil, err
	}

	// 更新缓存
	ms.klineCacheMu.Lock()
	ms.klineCache[cacheKey] = &klineCache{
		data:      klines,
		timestamp: time.Now(),
		ttl:       ttl,
	}
	ms.klineCacheMu.Unlock()
	ms.clearKLineFetchFailure(code, period, days)
	log.Info("module=market action=kline.fetch.success symbol=%s period=%s days=%d resultLen=%d", code, period, days, len(klines))

	return klines, nil
}

// GetKLineDataWithRequest 获取K线数据（带 requestId）
func (ms *MarketService) GetKLineDataWithRequest(requestID string, code string, period string, days int) ([]models.KLineData, error) {
	requestField := requestIDField(requestID)
	startedAt := time.Now()
	log.Info("module=market action=kline.fetch.start%s symbol=%s period=%s days=%d", requestField, code, period, days)
	klines, err := ms.getKLineDataWithRequestID(requestID, code, period, days)
	if err != nil {
		log.Warn("module=market action=kline.fetch.failed%s symbol=%s period=%s days=%d durationMs=%d err=%v", requestField, code, period, days, time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	log.Info("module=market action=kline.fetch.success%s symbol=%s period=%s days=%d resultLen=%d durationMs=%d", requestField, code, period, days, len(klines), time.Since(startedAt).Milliseconds())
	return klines, nil
}

// ListScreeningStocks 返回 AI 筛选同步使用的股票基础列表。
func (ms *MarketService) ListScreeningStocks(scopes models.ScreeningMarketScopeConfig) ([]ScreeningStockBasic, error) {
	var basicData stockBasicData
	if err := json.Unmarshal(embed.StockBasicJSON, &basicData); err != nil {
		return nil, err
	}

	fieldIndexes := make(map[string]int, len(basicData.Data.Fields))
	for i, field := range basicData.Data.Fields {
		fieldIndexes[field] = i
	}

	var stocks []ScreeningStockBasic
	for _, item := range basicData.Data.Items {
		tsCode := getStringField(item, fieldIndexes, "ts_code")
		symbol := getStringField(item, fieldIndexes, "symbol")
		name := getStringField(item, fieldIndexes, "name")
		if tsCode == "" || symbol == "" || name == "" {
			continue
		}

		fullSymbol, marketName, enabled := screeningStockIdentity(tsCode, symbol, scopes)
		if !enabled {
			continue
		}

		stocks = append(stocks, ScreeningStockBasic{
			Symbol:   fullSymbol,
			Name:     name,
			Market:   marketName,
			Industry: getStringField(item, fieldIndexes, "industry"),
			ListDate: getStringField(item, fieldIndexes, "list_date"),
			IsST:     strings.Contains(strings.ToUpper(name), "ST"),
			IsActive: getStringField(item, fieldIndexes, "list_status") != "D",
		})
	}

	if scopes.Indices {
		stocks = append(stocks, screeningIndexCatalog...)
	}

	return stocks, nil
}

// GetScreeningDailyBars 返回 AI 筛选同步所需的日线数据。
func (ms *MarketService) GetScreeningDailyBars(symbol string, lookbackDays int) ([]models.KLineData, error) {
	return ms.GetScreeningDailyBarsWithObserver(symbol, lookbackDays, nil)
}

func (ms *MarketService) GetScreeningDailyBarsWithObserver(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	source := ms.screeningDailyBarSource
	if source == nil {
		source = newScreeningDailyBarSourceChain(
			newBaoStockDailyBarSource(),
			screeningDailyBarSourceFunc(func(symbol string, lookbackDays int, _ ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
				return ms.fetchScreeningKLineData(symbol, lookbackDays)
			}),
		)
	}
	return source.Fetch(symbol, lookbackDays, observer)
}

func (ms *MarketService) fetchScreeningKLineData(code string, days int) ([]models.KLineData, error) {
	url := fmt.Sprintf(sinaScreeningKLineURL, code, days)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"kline api status %d for %s: %s",
			resp.StatusCode,
			code,
			httpErrorPreview(body),
		)
	}

	return ms.parseKLineData(string(body))
}

// GetScreeningSnapshots 返回 AI 筛选同步使用的最新快照。
func (ms *MarketService) GetScreeningSnapshots(symbols []string) ([]models.Stock, error) {
	var stockSymbols []string
	indexSymbols := make(map[string]struct{})
	for _, symbol := range symbols {
		if isScreeningIndexSymbol(symbol) {
			indexSymbols[symbol] = struct{}{}
			continue
		}
		stockSymbols = append(stockSymbols, symbol)
	}

	var snapshots []models.Stock
	if len(stockSymbols) > 0 {
		stocks, err := ms.GetStockRealTimeData(stockSymbols...)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, stocks...)
	}

	if len(indexSymbols) > 0 {
		indices, err := ms.GetMarketIndices()
		if err != nil {
			return nil, err
		}
		for _, index := range indices {
			if _, ok := indexSymbols[index.Code]; !ok {
				continue
			}
			snapshots = append(snapshots, models.Stock{
				Symbol:        index.Code,
				Name:          index.Name,
				Price:         index.Price,
				Change:        index.Change,
				ChangePercent: index.ChangePercent,
				Volume:        index.Volume,
				Amount:        index.Amount,
			})
		}
	}

	return snapshots, nil
}

// fetchKLineData 从API获取K线数据
func (ms *MarketService) fetchKLineData(code string, period string, days int) ([]models.KLineData, error) {
	return ms.fetchKLineDataWithRequestID("", code, period, days)
}

func (ms *MarketService) fetchKLineDataWithRequestID(requestID string, code string, period string, days int) ([]models.KLineData, error) {
	requestField := requestIDField(requestID)
	scale := ms.periodToScale(period)
	url := fmt.Sprintf(sinaKLineURL, code, scale, days)
	startedAt := time.Now()

	resp, err := ms.client.Get(url)
	if err != nil {
		log.Warn("module=market action=kline.fetch.http_failed symbol=%s period=%s days=%d durationMs=%d err=%v", code, period, days, time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn("module=market action=kline.fetch.read_failed symbol=%s period=%s days=%d durationMs=%d err=%v", code, period, days, time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		responsePreview := httpErrorPreview(body)
		sinaErr := fmt.Errorf(
			"kline api status %d for %s: %s",
			resp.StatusCode,
			code,
			responsePreview,
		)
		if shouldFallbackToEastmoneyKLine(resp.StatusCode, body) {
			fallbackURL, klines, fallbackErr := ms.fetchEastmoneyKLineData(code, period, days)
			log.Info("module=market action=kline.fetch.fallback.start%s source=sina fallback=eastmoney symbol=%s period=%s days=%d status=%d sinaUrl=%s responsePreview=%q durationMs=%d err=%v", requestField, code, period, days, resp.StatusCode, url, responsePreview, time.Since(startedAt).Milliseconds(), sinaErr)
			maxRetries := 1
			if period == "1d" {
				maxRetries = 2
			}
			for attempt := 1; fallbackErr != nil && shouldRetryEastmoneyKLine(fallbackErr) && attempt <= maxRetries; attempt++ {
				log.Warn("module=market action=kline.fetch.fallback.retry%s source=eastmoney symbol=%s period=%s days=%d attempt=%d fallbackUrl=%s durationMs=%d err=%v", requestField, code, period, days, attempt, fallbackURL, time.Since(startedAt).Milliseconds(), fallbackErr)
				time.Sleep(eastmoneyRetryDelay * time.Duration(attempt))
				fallbackURL, klines, fallbackErr = ms.fetchEastmoneyKLineData(code, period, days)
			}
			if fallbackErr == nil {
				if period == "1m" {
					klines = ms.filterTodayKLines(klines)
					klines = ms.calculateAvgLine(klines)
				}
				log.Info("module=market action=kline.fetch.fallback.success%s source=eastmoney symbol=%s period=%s days=%d fallbackUrl=%s resultLen=%d durationMs=%d", requestField, code, period, days, fallbackURL, len(klines), time.Since(startedAt).Milliseconds())
				return klines, nil
			}
			err = fmt.Errorf("%w; eastmoney fallback failed: %v", sinaErr, fallbackErr)
			log.Warn("module=market action=kline.fetch.fallback.failed%s source=eastmoney symbol=%s period=%s days=%d fallbackUrl=%s durationMs=%d err=%v", requestField, code, period, days, fallbackURL, time.Since(startedAt).Milliseconds(), err)
			return nil, err
		}
		log.Warn("module=market action=kline.fetch.http_status_failed symbol=%s period=%s days=%d durationMs=%d err=%v", code, period, days, time.Since(startedAt).Milliseconds(), sinaErr)
		return nil, sinaErr
	}

	klines, err := ms.parseKLineData(string(body))
	if err != nil {
		log.Warn("module=market action=kline.fetch.parse_failed symbol=%s period=%s days=%d durationMs=%d err=%v", code, period, days, time.Since(startedAt).Milliseconds(), err)
		return nil, err
	}

	// 分时模式下只返回当天的数据，并计算均价线
	if period == "1m" {
		klines = ms.filterTodayKLines(klines)
		klines = ms.calculateAvgLine(klines)
	}
	log.Debug("module=market action=kline.fetch.loaded symbol=%s period=%s days=%d resultLen=%d durationMs=%d", code, period, days, len(klines), time.Since(startedAt).Milliseconds())

	return klines, nil
}

func shouldFallbackToEastmoneyKLine(statusCode int, body []byte) bool {
	if statusCode == 456 {
		return true
	}
	preview := strings.ToLower(httpErrorPreview(body))
	return strings.Contains(preview, "blocked") || strings.Contains(preview, "拒绝访问")
}

func (ms *MarketService) fetchEastmoneyKLineData(code string, period string, days int) (string, []models.KLineData, error) {
	secID, err := normalizeEastmoneySecID(code)
	if err != nil {
		return "", nil, err
	}
	url := fmt.Sprintf(eastmoneyKLineURL, secID, eastmoneyKLineType(period), days)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return url, nil, err
	}
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := ms.client.Do(req)
	if err != nil {
		return url, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return url, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return url, nil, fmt.Errorf(
			"eastmoney kline api status %d for %s: %s",
			resp.StatusCode,
			code,
			httpErrorPreview(body),
		)
	}

	klines, parseErr := parseEastmoneyKLineData(body)
	return url, klines, parseErr
}

func normalizeEastmoneySecID(code string) (string, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	if len(code) != 8 {
		return "", fmt.Errorf("unsupported eastmoney symbol %q", code)
	}
	symbol := code[2:]
	switch {
	case strings.HasPrefix(code, "sh"):
		return "1." + symbol, nil
	case strings.HasPrefix(code, "sz"):
		return "0." + symbol, nil
	default:
		return "", fmt.Errorf("unsupported eastmoney symbol %q", code)
	}
}

func eastmoneyKLineType(period string) string {
	switch period {
	case "1m":
		return "1"
	case "1w":
		return "102"
	case "1mo":
		return "103"
	default:
		return "101"
	}
}

func parseEastmoneyKLineData(body []byte) ([]models.KLineData, error) {
	type eastmoneyKLineResponse struct {
		RC   int `json:"rc"`
		Data *struct {
			KLines []string `json:"klines"`
		} `json:"data"`
	}

	var response eastmoneyKLineResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.RC != 0 {
		return nil, fmt.Errorf("eastmoney kline api rc=%d", response.RC)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("eastmoney kline api missing data")
	}

	klines := make([]models.KLineData, 0, len(response.Data.KLines))
	for _, line := range response.Data.KLines {
		fields := strings.Split(line, ",")
		if len(fields) < 7 {
			return nil, fmt.Errorf("unexpected eastmoney kline row %q", line)
		}

		openPrice, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney open: %w", err)
		}
		closePrice, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney close: %w", err)
		}
		highPrice, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney high: %w", err)
		}
		lowPrice, err := strconv.ParseFloat(fields[4], 64)
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney low: %w", err)
		}
		volume, err := parseEastmoneyKLineInt64(fields[5])
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney volume: %w", err)
		}
		amount, err := strconv.ParseFloat(fields[6], 64)
		if err != nil {
			return nil, fmt.Errorf("parse eastmoney amount: %w", err)
		}

		klines = append(klines, models.KLineData{
			Time:   fields[0],
			Open:   openPrice,
			High:   highPrice,
			Low:    lowPrice,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
		})
	}

	return klines, nil
}

func parseEastmoneyKLineInt64(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsedInt, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return parsedInt, nil
	}
	parsedFloat, floatErr := strconv.ParseFloat(value, 64)
	if floatErr != nil {
		return 0, err
	}
	return int64(parsedFloat), nil
}

func httpErrorPreview(body []byte) string {
	preview := strings.TrimSpace(string(body))
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	preview = strings.Join(strings.Fields(preview), " ")
	if preview == "" {
		return "empty response body"
	}
	if len(preview) > 160 {
		return preview[:160] + "..."
	}
	return preview
}

func shouldRetryEastmoneyKLine(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") || strings.Contains(message, "connection reset") || strings.Contains(message, "broken pipe")
}

func getStringField(item []interface{}, fieldIndexes map[string]int, field string) string {
	idx, ok := fieldIndexes[field]
	if !ok || idx < 0 || idx >= len(item) {
		return ""
	}
	value, _ := item[idx].(string)
	return value
}

func screeningStockIdentity(
	tsCode string,
	symbol string,
	scopes models.ScreeningMarketScopeConfig,
) (fullSymbol string, marketName string, enabled bool) {
	switch {
	case strings.HasSuffix(tsCode, ".SH"):
		return "sh" + symbol, "上海", scopes.Shanghai
	case strings.HasSuffix(tsCode, ".SZ"):
		if strings.HasPrefix(symbol, "300") || strings.HasPrefix(symbol, "301") {
			return "", "", false
		}
		return "sz" + symbol, "深圳", scopes.Shenzhen
	case strings.HasSuffix(tsCode, ".BJ"):
		return "bj" + symbol, "北京", scopes.Beijing
	default:
		return "", "", false
	}
}

func isScreeningIndexSymbol(symbol string) bool {
	for _, index := range screeningIndexCatalog {
		if index.Symbol == symbol {
			return true
		}
	}
	return false
}

// periodToScale 周期转换为新浪API的scale参数
func (ms *MarketService) periodToScale(period string) string {
	switch period {
	case "1m":
		return "1" // 1分钟线（分时图）
	case "1d":
		return "240" // 日线
	case "1w":
		return "1680" // 周线
	case "1mo":
		return "7200" // 月线
	default:
		return "240"
	}
}

// filterTodayKLines 过滤只返回当天的K线数据
func (ms *MarketService) filterTodayKLines(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	today := time.Now().Format("2006-01-02")
	result := make([]models.KLineData, 0)

	for _, k := range klines {
		// 时间格式为 "2006-01-02 15:04:05"，取日期部分比较
		if len(k.Time) >= 10 && k.Time[:10] == today {
			result = append(result, k)
		}
	}

	// 如果当天没有数据（非交易日），返回最后一天的数据
	if len(result) == 0 && len(klines) > 0 {
		lastDay := klines[len(klines)-1].Time[:10]
		for _, k := range klines {
			if len(k.Time) >= 10 && k.Time[:10] == lastDay {
				result = append(result, k)
			}
		}
	}

	return result
}

// calculateAvgLine 计算分时均价线 (VWAP = 累计成交额 / 累计成交量)
func (ms *MarketService) calculateAvgLine(klines []models.KLineData) []models.KLineData {
	if len(klines) == 0 {
		return klines
	}

	var totalAmount float64
	var totalVolume int64

	for i := range klines {
		totalAmount += klines[i].Amount
		totalVolume += klines[i].Volume

		if totalVolume > 0 {
			klines[i].Avg = totalAmount / float64(totalVolume)
		}
	}

	return klines
}

// parseKLineData 解析K线数据 - 使用标准JSON解析
func (ms *MarketService) parseKLineData(data string) ([]models.KLineData, error) {
	// 新浪API返回的K线数据结构（含均线和成交额）
	type sinaKLine struct {
		Day       string  `json:"day"`
		Open      string  `json:"open"`
		High      string  `json:"high"`
		Low       string  `json:"low"`
		Close     string  `json:"close"`
		Volume    string  `json:"volume"`
		Amount    string  `json:"amount"`
		MAPrice5  float64 `json:"ma_price5"`
		MAPrice10 float64 `json:"ma_price10"`
		MAPrice20 float64 `json:"ma_price20"`
	}

	var sinaData []sinaKLine
	if err := json.Unmarshal([]byte(data), &sinaData); err != nil {
		return nil, err
	}

	klines := make([]models.KLineData, 0, len(sinaData))
	for _, item := range sinaData {
		open, _ := strconv.ParseFloat(item.Open, 64)
		high, _ := strconv.ParseFloat(item.High, 64)
		low, _ := strconv.ParseFloat(item.Low, 64)
		closePrice, _ := strconv.ParseFloat(item.Close, 64)
		volume, _ := strconv.ParseInt(item.Volume, 10, 64)
		amount, _ := strconv.ParseFloat(item.Amount, 64)

		klines = append(klines, models.KLineData{
			Time:   item.Day,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
			MA5:    item.MAPrice5,
			MA10:   item.MAPrice10,
			MA20:   item.MAPrice20,
		})
	}
	return klines, nil
}

// GetRealOrderBook 获取真实盘口数据
func (ms *MarketService) GetRealOrderBook(code string) (models.OrderBook, error) {
	startedAt := time.Now()
	log.Info("module=market action=orderbook.fetch.start symbol=%s", code)
	data, err := ms.GetStockDataWithOrderBook(code)
	if err != nil {
		log.Warn("module=market action=orderbook.fetch.failed symbol=%s durationMs=%d err=%v", code, time.Since(startedAt).Milliseconds(), err)
		return models.OrderBook{}, err
	}
	if len(data) == 0 {
		log.Warn("module=market action=orderbook.fetch.empty symbol=%s durationMs=%d", code, time.Since(startedAt).Milliseconds())
		return models.OrderBook{}, err
	}
	orderBook := data[0].OrderBook
	log.Info("module=market action=orderbook.fetch.success symbol=%s bids=%d asks=%d durationMs=%d", code, len(orderBook.Bids), len(orderBook.Asks), time.Since(startedAt).Milliseconds())
	return orderBook, nil
}

// GetRealOrderBookWithRequest 获取真实盘口数据（带 requestId）
func (ms *MarketService) GetRealOrderBookWithRequest(requestID string, code string) (models.OrderBook, error) {
	requestField := requestIDField(requestID)
	startedAt := time.Now()
	log.Info("module=market action=orderbook.fetch.start%s symbol=%s", requestField, code)
	orderBook, err := ms.GetRealOrderBook(code)
	if err != nil {
		log.Warn("module=market action=orderbook.fetch.failed%s symbol=%s durationMs=%d err=%v", requestField, code, time.Since(startedAt).Milliseconds(), err)
		return models.OrderBook{}, err
	}
	log.Info("module=market action=orderbook.fetch.success%s symbol=%s bids=%d asks=%d durationMs=%d", requestField, code, len(orderBook.Bids), len(orderBook.Asks), time.Since(startedAt).Milliseconds())
	return orderBook, nil
}

// GenerateOrderBook 生成盘口数据（保留兼容，建议使用 GetRealOrderBook）
func (ms *MarketService) GenerateOrderBook(price float64) models.OrderBook {
	var bids, asks []models.OrderBookItem

	for i := 0; i < 5; i++ {
		bidPrice := price - float64(i+1)*0.01
		askPrice := price + float64(i+1)*0.01

		bids = append(bids, models.OrderBookItem{
			Price:   bidPrice,
			Size:    int64(100 + i*50),
			Total:   int64((100 + i*50) * (i + 1)),
			Percent: float64(100-i*15) / 100,
		})
		asks = append(asks, models.OrderBookItem{
			Price:   askPrice,
			Size:    int64(100 + i*50),
			Total:   int64((100 + i*50) * (i + 1)),
			Percent: float64(100-i*15) / 100,
		})
	}

	return models.OrderBook{Bids: bids, Asks: asks}
}

// GetMarketStatus 获取当前市场交易状态
func (ms *MarketService) GetMarketStatus() MarketStatus {
	now := time.Now()
	// 使用固定时区 UTC+8，避免 Windows 缺少时区数据库的问题
	loc := time.FixedZone("CST", 8*60*60)
	now = now.In(loc)
	// 检查是否为交易日
	isTradeDay, holidayName := ms.isTradeDay(now)
	if !isTradeDay {
		statusText := "休市"
		if holidayName != "" {
			statusText = holidayName + "休市"
		} else if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			statusText = "周末休市"
		}
		result := MarketStatus{
			Status:      "closed",
			StatusText:  statusText,
			IsTradeDay:  false,
			HolidayName: holidayName,
		}
		return result
	}

	// 交易日，判断当前时间段
	hour, minute := now.Hour(), now.Minute()
	currentMinutes := hour*60 + minute

	// A股交易时间: 9:30-11:30, 13:00-15:00
	var result MarketStatus
	switch {
	case currentMinutes < 9*60+15:
		result = MarketStatus{Status: "pre_market", StatusText: "盘前", IsTradeDay: true}
	case currentMinutes < 9*60+30:
		result = MarketStatus{Status: "pre_market", StatusText: "集合竞价", IsTradeDay: true}
	case currentMinutes < 11*60+30:
		result = MarketStatus{Status: "trading", StatusText: "交易中", IsTradeDay: true}
	case currentMinutes < 13*60:
		result = MarketStatus{Status: "lunch_break", StatusText: "午间休市", IsTradeDay: true}
	case currentMinutes < 15*60:
		result = MarketStatus{Status: "trading", StatusText: "交易中", IsTradeDay: true}
	default:
		result = MarketStatus{Status: "closed", StatusText: "已收盘", IsTradeDay: true}
	}
	return result
}

// GetTradingSchedule 获取交易时间表（供前端判断市场状态）
func (ms *MarketService) GetTradingSchedule() TradingSchedule {
	now := time.Now()
	loc := time.FixedZone("CST", 8*60*60)
	now = now.In(loc)

	isTradeDay, holidayName := ms.isTradeDay(now)

	// A股交易时段配置
	periods := []TradingPeriod{
		{Status: "pre_market", Text: "盘前", StartTime: "00:00", EndTime: "09:15"},
		{Status: "pre_market", Text: "集合竞价", StartTime: "09:15", EndTime: "09:30"},
		{Status: "trading", Text: "交易中", StartTime: "09:30", EndTime: "11:30"},
		{Status: "lunch_break", Text: "午间休市", StartTime: "11:30", EndTime: "13:00"},
		{Status: "trading", Text: "交易中", StartTime: "13:00", EndTime: "15:00"},
		{Status: "closed", Text: "已收盘", StartTime: "15:00", EndTime: "24:00"},
	}

	return TradingSchedule{
		IsTradeDay:  isTradeDay,
		HolidayName: holidayName,
		Periods:     periods,
	}
}

// isTradeDay 判断指定日期是否为交易日
// A股交易日判定：非周末 且 非节假日（调休上班也不算交易日）
func (ms *MarketService) isTradeDay(date time.Time) (bool, string) {

	// 周末一律不是交易日
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false, "周末"
	}

	// 工作日：检查是否为节假日
	isOffDay, inList, note := ms.getHolidayStatus(date)
	if inList && isOffDay {
		return false, note
	}

	return true, ""
}

// getHolidayStatus 获取指定日期的节假日状态
// 返回: isOffDay=true表示休息日, inList=是否在节假日列表中, note为节假日名称
func (ms *MarketService) getHolidayStatus(date time.Time) (isOffDay bool, inList bool, note string) {
	dateStr := date.Format("2006-01-02")
	year := date.Year()

	// 加载该年份的节假日数据
	yearData, err := ms.loadHolidayData(year)
	if err != nil {
		log.Warn("加载 %d 年节假日数据失败: %v", year, err)
		return false, false, ""
	}

	// 查找该日期
	if isOff, exists := yearData[dateStr]; exists {
		noteName := ms.getHolidayNote(year, dateStr)
		return isOff, true, noteName
	}

	// 不在节假日列表中
	return false, false, ""
}

// getHolidayNote 获取节假日名称
func (ms *MarketService) getHolidayNote(year int, dateStr string) string {
	cacheFile := getHolidayCacheFile(year)
	fileData, err := os.ReadFile(cacheFile)
	if err != nil {
		return ""
	}

	var hd holidayData
	if json.Unmarshal(fileData, &hd) != nil {
		return ""
	}

	for _, day := range hd.Days {
		if day.Date == dateStr {
			return day.Name
		}
	}
	return ""
}

// tradeDatesCache 交易日缓存文件结构
type tradeDatesCache struct {
	TradeDates []string  `json:"tradeDates"` // 交易日列表
	UpdatedAt  time.Time `json:"updatedAt"`  // 更新时间
}

// holidayData 节假日数据结构
type holidayData struct {
	Year int          `json:"year"`
	Days []holidayDay `json:"days"`
}

type holidayDay struct {
	Name     string `json:"name"`
	Date     string `json:"date"`
	IsOffDay bool   `json:"isOffDay"`
}

// holidayCache 节假日缓存（按年份）
var (
	holidayCacheMu   sync.RWMutex
	holidayCacheData = make(map[int]map[string]bool) // year -> date -> isOffDay
)

const holidayCDNURL = "https://cdn.jsdelivr.net/gh/NateScarlet/holiday-cn@master/%d.json"

// getHolidayCacheFile 获取节假日缓存文件路径
func getHolidayCacheFile(year int) string {
	return filepath.Join(paths.EnsureCacheDir("holiday"), fmt.Sprintf("%d.json", year))
}

// loadHolidayData 加载指定年份的节假日数据
func (ms *MarketService) loadHolidayData(year int) (map[string]bool, error) {
	// 先检查内存缓存
	holidayCacheMu.RLock()
	if data, ok := holidayCacheData[year]; ok {
		holidayCacheMu.RUnlock()
		return data, nil
	}
	holidayCacheMu.RUnlock()

	// 尝试从文件缓存加载
	cacheFile := getHolidayCacheFile(year)
	if fileData, err := os.ReadFile(cacheFile); err == nil {
		var hd holidayData
		if json.Unmarshal(fileData, &hd) == nil {
			data := ms.parseHolidayData(&hd)
			holidayCacheMu.Lock()
			holidayCacheData[year] = data
			holidayCacheMu.Unlock()
			return data, nil
		}
	}

	// 从CDN获取
	return ms.fetchHolidayData(year)
}

// fetchHolidayData 从CDN获取节假日数据
func (ms *MarketService) fetchHolidayData(year int) (map[string]bool, error) {
	url := fmt.Sprintf(holidayCDNURL, year)
	resp, err := ms.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取节假日数据失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var hd holidayData
	if err := json.Unmarshal(body, &hd); err != nil {
		return nil, err
	}

	// 保存到文件缓存
	cacheFile := getHolidayCacheFile(year)
	os.WriteFile(cacheFile, body, 0644)

	// 解析并缓存到内存
	data := ms.parseHolidayData(&hd)
	holidayCacheMu.Lock()
	holidayCacheData[year] = data
	holidayCacheMu.Unlock()

	log.Info("加载 %d 年节假日数据，共 %d 条", year, len(hd.Days))
	return data, nil
}

// parseHolidayData 解析节假日数据为 map
func (ms *MarketService) parseHolidayData(hd *holidayData) map[string]bool {
	data := make(map[string]bool)
	for _, day := range hd.Days {
		data[day.Date] = day.IsOffDay
	}
	return data
}

// isTradeDate 判断指定日期是否为交易日
// A股交易日 = 非周末 且 非节假日（调休上班也不算交易日）
func (ms *MarketService) isTradeDate(date time.Time) bool {
	isTradeDay, _ := ms.isTradeDay(date)
	return isTradeDay
}

// getTradeDatesCacheFile 获取交易日缓存文件路径
func getTradeDatesCacheFile() string {
	return filepath.Join(paths.EnsureCacheDir(""), "trade_dates.json")
}

// GetTradeDates 获取指定天数内的交易日列表（从今天往前推）
func (ms *MarketService) GetTradeDates(days int) ([]string, error) {
	// 先尝试从文件缓存加载
	cached, err := ms.loadTradeDatesCache()
	if err == nil && len(cached.TradeDates) > 0 {
		// 检查缓存是否过期（每天更新一次）
		if time.Since(cached.UpdatedAt) < 24*time.Hour {
			log.Debug("使用交易日缓存，共 %d 天", len(cached.TradeDates))
			return ms.filterTradeDates(cached.TradeDates, days), nil
		}
	}

	// 缓存不存在或过期，重新获取
	log.Info("开始获取交易日列表")
	tradeDates, err := ms.fetchTradeDates(90) // 获取90天的数据
	if err != nil {
		// 如果获取失败但有旧缓存，使用旧缓存
		if cached != nil && len(cached.TradeDates) > 0 {
			log.Warn("获取交易日失败，使用旧缓存: %v", err)
			return ms.filterTradeDates(cached.TradeDates, days), nil
		}
		return nil, err
	}

	// 保存到文件缓存
	if err := ms.saveTradeDatesCache(tradeDates); err != nil {
		log.Warn("保存交易日缓存失败: %v", err)
	}

	return ms.filterTradeDates(tradeDates, days), nil
}

// filterTradeDates 过滤交易日列表，只返回指定天数
func (ms *MarketService) filterTradeDates(dates []string, days int) []string {
	if len(dates) <= days {
		return dates
	}
	return dates[:days]
}

// loadTradeDatesCache 从文件加载交易日缓存
func (ms *MarketService) loadTradeDatesCache() (*tradeDatesCache, error) {
	data, err := os.ReadFile(getTradeDatesCacheFile())
	if err != nil {
		return nil, err
	}
	var cache tradeDatesCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// saveTradeDatesCache 保存交易日缓存到文件
func (ms *MarketService) saveTradeDatesCache(dates []string) error {
	cache := tradeDatesCache{
		TradeDates: dates,
		UpdatedAt:  time.Now(),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getTradeDatesCacheFile(), data, 0644)
}

// fetchTradeDates 获取交易日列表
func (ms *MarketService) fetchTradeDates(days int) ([]string, error) {
	var tradeDates []string
	today := time.Now()

	// 预加载需要的年份节假日数据
	yearsNeeded := make(map[int]bool)
	for i := 0; i < days; i++ {
		yearsNeeded[today.AddDate(0, 0, -i).Year()] = true
	}
	for year := range yearsNeeded {
		if _, err := ms.loadHolidayData(year); err != nil {
			log.Warn("加载 %d 年节假日数据失败: %v", year, err)
		}
	}

	for i := 0; i < days; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")

		if ms.isTradeDate(date) {
			tradeDates = append(tradeDates, dateStr)
		}
	}

	log.Info("获取到 %d 个交易日", len(tradeDates))
	return tradeDates, nil
}

// GetMarketIndices 获取大盘指数数据
func (ms *MarketService) GetMarketIndices() ([]models.MarketIndex, error) {
	codeList := strings.Join(defaultIndexCodes, ",")
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codeList)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return ms.parseMarketIndices(string(body))
}

// parseMarketIndices 解析大盘指数数据
// 新浪简化指数数据格式: var hq_str_s_sh000001="上证指数,3094.668,-128.073,-3.97,436653,5458126"
// 字段: 名称,当前点位,涨跌点数,涨跌幅(%),成交量(手),成交额(万元)
func (ms *MarketService) parseMarketIndices(data string) ([]models.MarketIndex, error) {
	var indices []models.MarketIndex
	matches := sinaIndexRegex.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 6 {
			continue
		}

		price, _ := strconv.ParseFloat(parts[1], 64)
		change, _ := strconv.ParseFloat(parts[2], 64)
		changePercent, _ := strconv.ParseFloat(parts[3], 64)
		volume, _ := strconv.ParseInt(parts[4], 10, 64)
		amount, _ := strconv.ParseFloat(parts[5], 64)

		indices = append(indices, models.MarketIndex{
			Code:          match[1],
			Name:          parts[0],
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
			Volume:        volume,
			Amount:        amount,
		})
	}
	return indices, nil
}
