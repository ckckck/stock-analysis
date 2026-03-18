// Package memory 提供 Agent 记忆管理功能
package memory

import "time"

// EntryType 记忆条目类型
type EntryType string

const (
	EntryTypeFact     EntryType = "fact"     // 事实
	EntryTypeOpinion  EntryType = "opinion"  // 观点
	EntryTypeDecision EntryType = "decision" // 决策
)

// MemoryEntry 记忆条目
type MemoryEntry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`    // 来源 Agent
	Keywords  []string  `json:"keywords"`  // 关键词（用于文本匹配）
	Timestamp int64     `json:"timestamp"`
	Weight    float64   `json:"weight"` // 重要性权重 0-1
}

// RoundMemory 单轮讨论记忆
type RoundMemory struct {
	Round     int      `json:"round"`
	Query     string   `json:"query"`      // 用户问题
	Consensus string   `json:"consensus"`  // 本轮结论
	KeyPoints []string `json:"key_points"` // 要点
	Timestamp int64    `json:"timestamp"`
}

// StockMemory 单只股票的会话记忆（按股票隔离）
type StockMemory struct {
	StockCode    string        `json:"stock_code"`
	StockName    string        `json:"stock_name"`
	Summary      string        `json:"summary"`       // 历史摘要
	KeyFacts     []MemoryEntry `json:"key_facts"`     // 关键事实
	RecentRounds []RoundMemory `json:"recent_rounds"` // 最近几轮讨论
	TotalRounds  int           `json:"total_rounds"`  // 总讨论轮次
	CreatedAt    int64         `json:"created_at"`
	UpdatedAt    int64         `json:"updated_at"`
}

// NewStockMemory 创建新的股票记忆
func NewStockMemory(stockCode, stockName string) *StockMemory {
	now := time.Now().UnixMilli()
	return &StockMemory{
		StockCode:    stockCode,
		StockName:    stockName,
		KeyFacts:     []MemoryEntry{},
		RecentRounds: []RoundMemory{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Config 记忆管理配置
type Config struct {
	MaxRecentRounds   int // 保留最近几轮讨论，默认 3
	MaxKeyFacts       int // 最大关键事实数，默认 20
	MaxSummaryLength  int // 摘要最大字数，默认 300
	CompressThreshold int // 触发压缩的轮次数，默认 5
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MaxRecentRounds:   3,
		MaxKeyFacts:       20,
		MaxSummaryLength:  300,
		CompressThreshold: 5,
	}
}
