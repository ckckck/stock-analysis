package services

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

const (
	baoStockServerAddress       = "www.baostock.com:10030"
	baoStockMessageSplit        = "\x01"
	baoStockClientVersion       = "00.8.90"
	baoStockHeaderLength        = 21
	baoStockLoginRequestType    = "00"
	baoStockLogoutRequestType   = "02"
	baoStockKLineRequestType    = "95"
	baoStockKLineResponseType   = "96"
	baoStockMessageTerminator   = "<![CDATA[]]>\n"
	baoStockDefaultPerPageCount = 10000
)

type baostockDailyBarSource struct {
	now     func() time.Time
	dial    func(network, address string) (net.Conn, error)
	timeout time.Duration
}

type baoStockClient struct {
	conn    net.Conn
	timeout time.Duration
	userID  string
}

func newBaoStockDailyBarSource() *baostockDailyBarSource {
	return &baostockDailyBarSource{
		now: time.Now,
		dial: func(network, address string) (net.Conn, error) {
			return net.DialTimeout(network, address, 5*time.Second)
		},
		timeout: 10 * time.Second,
	}
}

func (s *baostockDailyBarSource) Fetch(symbol string, lookbackDays int, _ ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	if lookbackDays <= 0 {
		lookbackDays = 30
	}

	baoSymbol, err := normalizeBaoStockSymbol(symbol)
	if err != nil {
		return nil, err
	}

	client, err := s.newClient()
	if err != nil {
		return nil, err
	}
	defer client.close()

	if err := client.login("anonymous", "123456", 0); err != nil {
		return nil, fmt.Errorf("login %s: %w", baoSymbol, err)
	}
	defer func() {
		if err := client.logout(); err != nil {
			log.Warn("baostock logout failed for %s: %v", baoSymbol, err)
		}
	}()

	endDate := s.now().Format("2006-01-02")
	startDate := s.now().AddDate(0, 0, -maxInt(lookbackDays*2, lookbackDays+10)).Format("2006-01-02")
	rows, err := client.queryHistoryKDataPlus(
		baoSymbol,
		"date,open,high,low,close,volume,amount",
		startDate,
		endDate,
		"d",
		"2",
	)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", baoSymbol, err)
	}

	bars, err := parseBaoStockKLines(rows)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", baoSymbol, err)
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("empty daily bar response")
	}
	if len(bars) > lookbackDays {
		bars = bars[len(bars)-lookbackDays:]
	}
	return bars, nil
}

func (s *baostockDailyBarSource) newClient() (*baoStockClient, error) {
	if s == nil || s.dial == nil {
		return nil, fmt.Errorf("baostock dialer not configured")
	}
	conn, err := s.dial("tcp", baoStockServerAddress)
	if err != nil {
		return nil, fmt.Errorf("connect baostock: %w", err)
	}
	return &baoStockClient{conn: conn, timeout: s.timeout}, nil
}

func (c *baoStockClient) login(userID, password string, options int) error {
	body := strings.Join([]string{
		"login",
		userID,
		password,
		strconv.Itoa(options),
	}, baoStockMessageSplit)

	resp, err := c.send(baoStockLoginRequestType, body)
	if err != nil {
		return err
	}
	parts := strings.Split(resp, baoStockMessageSplit)
	if len(parts) < 4 {
		return fmt.Errorf("unexpected login response")
	}
	if parts[0] != "0" {
		return fmt.Errorf("%s", parts[1])
	}
	c.userID = parts[3]
	return nil
}

func (c *baoStockClient) logout() error {
	if c == nil || c.conn == nil || c.userID == "" {
		return nil
	}
	body := strings.Join([]string{
		"logout",
		c.userID,
		time.Now().Format("20060102150405"),
	}, baoStockMessageSplit)
	_, err := c.send(baoStockLogoutRequestType, body)
	return err
}

func (c *baoStockClient) queryHistoryKDataPlus(code, fields, startDate, endDate, frequency, adjustflag string) ([][]string, error) {
	body := strings.Join([]string{
		"query_history_k_data_plus",
		c.userID,
		"1",
		strconv.Itoa(baoStockDefaultPerPageCount),
		code,
		fields,
		startDate,
		endDate,
		frequency,
		adjustflag,
	}, baoStockMessageSplit)

	resp, err := c.send(baoStockKLineRequestType, body)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(resp, baoStockMessageSplit)
	if len(parts) < 13 {
		return nil, fmt.Errorf("unexpected kline response")
	}
	if parts[0] != "0" {
		return nil, fmt.Errorf("%s", parts[1])
	}

	var payload struct {
		Record [][]string `json:"record"`
	}
	if err := json.Unmarshal([]byte(parts[6]), &payload); err != nil {
		return nil, fmt.Errorf("decode kline payload: %w", err)
	}
	return payload.Record, nil
}

func (c *baoStockClient) send(msgType, body string) (string, error) {
	if c == nil || c.conn == nil {
		return "", fmt.Errorf("baostock connection not initialized")
	}

	header := baoStockClientVersion + baoStockMessageSplit + msgType + baoStockMessageSplit + fmt.Sprintf("%010d", len(body))
	headBody := header + body
	crc := crc32.ChecksumIEEE([]byte(headBody))
	request := headBody + baoStockMessageSplit + strconv.FormatUint(uint64(crc), 10) + "\n"

	if c.timeout > 0 {
		if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
			return "", fmt.Errorf("set baostock deadline: %w", err)
		}
	}

	if _, err := c.conn.Write([]byte(request)); err != nil {
		return "", fmt.Errorf("write baostock request: %w", err)
	}

	raw, err := readBaoStockResponse(c.conn)
	if err != nil {
		return "", err
	}
	if len(raw) < baoStockHeaderLength {
		return "", fmt.Errorf("short baostock response")
	}

	headerText := string(raw[:baoStockHeaderLength])
	headerParts := strings.Split(headerText, baoStockMessageSplit)
	if len(headerParts) < 3 {
		return "", fmt.Errorf("invalid baostock header")
	}
	bodyLength, err := strconv.Atoi(headerParts[2])
	if err != nil {
		return "", fmt.Errorf("parse baostock body length: %w", err)
	}

	if headerParts[1] == baoStockKLineResponseType {
		bodyStart := baoStockHeaderLength
		bodyEnd := bodyStart + bodyLength
		if len(raw) < bodyEnd {
			return "", fmt.Errorf("incomplete baostock compressed body")
		}
		reader, err := zlib.NewReader(bytes.NewReader(raw[bodyStart:bodyEnd]))
		if err != nil {
			return "", fmt.Errorf("open baostock zlib body: %w", err)
		}
		defer reader.Close()
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("read baostock zlib body: %w", err)
		}
		return string(decoded), nil
	}

	bodyStart := baoStockHeaderLength
	bodyEnd := bytes.Index(raw, []byte(baoStockMessageTerminator))
	if bodyEnd < 0 {
		bodyEnd = bytes.LastIndexByte(raw, '\n')
	}
	if bodyEnd < bodyStart {
		bodyEnd = len(raw)
	}
	return strings.TrimSpace(string(raw[bodyStart:bodyEnd])), nil
}

func (c *baoStockClient) close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func readBaoStockResponse(conn net.Conn) ([]byte, error) {
	var buffer bytes.Buffer
	chunk := make([]byte, 8192)
	for {
		n, err := conn.Read(chunk)
		if n > 0 {
			buffer.Write(chunk[:n])
			if bytes.HasSuffix(buffer.Bytes(), []byte(baoStockMessageTerminator)) {
				return buffer.Bytes(), nil
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read baostock response: %w", err)
		}
	}
	if buffer.Len() == 0 {
		return nil, fmt.Errorf("empty baostock response")
	}
	return buffer.Bytes(), nil
}

func normalizeBaoStockSymbol(symbol string) (string, error) {
	symbol = strings.TrimSpace(strings.ToLower(symbol))
	switch {
	case strings.HasPrefix(symbol, "sh.") || strings.HasPrefix(symbol, "sz."):
		market, code := symbol[:2], symbol[3:]
		if len(code) != 6 {
			return "", fmt.Errorf("unsupported baostock symbol %q", symbol)
		}
		return market + "." + code, nil
	case len(symbol) == 8 && (strings.HasPrefix(symbol, "sh") || strings.HasPrefix(symbol, "sz")):
		return symbol[:2] + "." + symbol[2:], nil
	default:
		return "", fmt.Errorf("unsupported baostock symbol %q", symbol)
	}
}

func parseBaoStockKLines(rows [][]string) ([]models.KLineData, error) {
	bars := make([]models.KLineData, 0, len(rows))
	for _, row := range rows {
		if len(row) < 7 {
			return nil, fmt.Errorf("unexpected baostock row length %d", len(row))
		}
		openPrice, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse open: %w", err)
		}
		highPrice, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse high: %w", err)
		}
		lowPrice, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, fmt.Errorf("parse low: %w", err)
		}
		closePrice, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("parse close: %w", err)
		}
		volume := int64(0)
		if strings.TrimSpace(row[5]) != "" {
			parsedVolume, err := strconv.ParseInt(row[5], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse volume: %w", err)
			}
			volume = parsedVolume
		}
		amount := float64(0)
		if strings.TrimSpace(row[6]) != "" {
			parsedAmount, err := strconv.ParseFloat(row[6], 64)
			if err != nil {
				return nil, fmt.Errorf("parse amount: %w", err)
			}
			amount = parsedAmount
		}

		bars = append(bars, models.KLineData{
			Time:   row[0],
			Open:   openPrice,
			High:   highPrice,
			Low:    lowPrice,
			Close:  closePrice,
			Volume: volume,
			Amount: amount,
		})
	}
	return bars, nil
}
