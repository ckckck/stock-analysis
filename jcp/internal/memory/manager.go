package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/model"
)

// Manager 记忆管理器
type Manager struct {
	config     Config
	storage    Storage
	tokenizer  Tokenizer
	relevance  *Relevance
	summarizer Summarizer
	dataDir    string
	saveCh     chan *StockMemory // 异步保存通道
	closeCh    chan struct{}     // 关闭信号
}

// NewManager 创建记忆管理器（无 LLM，摘要功能禁用）
func NewManager(dataDir string) *Manager {
	tokenizer := NewJiebaTokenizer()
	m := &Manager{
		config:    DefaultConfig(),
		storage:   NewFileStorage(dataDir),
		tokenizer: tokenizer,
		relevance: NewRelevance(tokenizer),
		dataDir:   dataDir,
		saveCh:    make(chan *StockMemory, 100), // 缓冲通道
		closeCh:   make(chan struct{}),
	}
	go m.asyncSaveLoop()
	return m
}

// SetLLM 设置 LLM（启用摘要功能）
func (m *Manager) SetLLM(llm model.LLM) {
	m.summarizer = NewLLMSummarizer(llm, m.tokenizer)
}

// NewManagerWithConfig 使用自定义配置创建记忆管理器
func NewManagerWithConfig(dataDir string, config Config) *Manager {
	m := NewManager(dataDir)
	m.config = config
	return m
}

// GetOrCreate 获取或创建股票记忆
func (m *Manager) GetOrCreate(stockCode, stockName string) (*StockMemory, error) {
	mem, err := m.storage.Load(stockCode)
	if err != nil {
		// 不存在则创建新的
		mem = NewStockMemory(stockCode, stockName)
	}
	return mem, nil
}

// Save 保存记忆（同步）
func (m *Manager) Save(mem *StockMemory) error {
	mem.UpdatedAt = time.Now().UnixMilli()
	return m.storage.Save(mem)
}

// SaveAsync 异步保存记忆（不阻塞）
func (m *Manager) SaveAsync(mem *StockMemory) {
	mem.UpdatedAt = time.Now().UnixMilli()
	select {
	case m.saveCh <- mem:
	default:
		// 通道满时丢弃，避免阻塞
		fmt.Printf("memory save channel full, dropping save for %s\n", mem.StockCode)
	}
}

// asyncSaveLoop 异步保存循环
func (m *Manager) asyncSaveLoop() {
	for {
		select {
		case mem := <-m.saveCh:
			if err := m.storage.Save(mem); err != nil {
				fmt.Printf("async save memory error: %v\n", err)
			}
		case <-m.closeCh:
			// 退出前保存剩余的
			for {
				select {
				case mem := <-m.saveCh:
					m.storage.Save(mem)
				default:
					return
				}
			}
		}
	}
}

// BuildContext 构建上下文（核心方法）
func (m *Manager) BuildContext(mem *StockMemory, currentQuery string) string {
	var sb strings.Builder

	// 1. 历史摘要
	if mem.Summary != "" {
		sb.WriteString("【历史讨论摘要】\n")
		sb.WriteString(mem.Summary)
		sb.WriteString("\n\n")
	}

	// 2. 相关的关键事实（基于关键词匹配）
	relevantFacts := m.relevance.FindRelevant(mem.KeyFacts, currentQuery, 5)
	if len(relevantFacts) > 0 {
		sb.WriteString("【相关历史信息】\n")
		for _, fact := range relevantFacts {
			timeStr := time.UnixMilli(fact.Timestamp).Format("2006-01-02")
			fmt.Fprintf(&sb, "- [%s] %s\n", timeStr, fact.Content)
		}
		sb.WriteString("\n")
	}

	// 3. 最近几轮讨论的要点
	if len(mem.RecentRounds) > 0 {
		sb.WriteString("【近期讨论】\n")
		for _, round := range mem.RecentRounds {
			timeStr := time.UnixMilli(round.Timestamp).Format("2006-01-02 15:04")
			fmt.Fprintf(&sb, "[%s] 问题: %s\n", timeStr, round.Query)
			fmt.Fprintf(&sb, "结论: %s\n\n", round.Consensus)
		}
	}

	return sb.String()
}

// AddRound 添加新一轮讨论并触发压缩检查
func (m *Manager) AddRound(ctx context.Context, mem *StockMemory, query, consensus string, keyPoints []string) error {
	mem.TotalRounds++
	round := RoundMemory{
		Round:     mem.TotalRounds,
		Query:     query,
		Consensus: consensus,
		KeyPoints: keyPoints,
		Timestamp: time.Now().UnixMilli(),
	}
	mem.RecentRounds = append(mem.RecentRounds, round)

	// 检查是否需要压缩
	if len(mem.RecentRounds) >= m.config.CompressThreshold {
		if err := m.compress(ctx, mem); err != nil {
			// 压缩失败不影响主流程，记录日志即可
			fmt.Printf("compress memory error: %v\n", err)
		}
	}

	// 异步保存，不阻塞主流程
	m.SaveAsync(mem)
	return nil
}

// compress 压缩旧轮次为摘要
func (m *Manager) compress(ctx context.Context, mem *StockMemory) error {
	keepCount := m.config.MaxRecentRounds
	if len(mem.RecentRounds) <= keepCount {
		return nil
	}

	toCompress := mem.RecentRounds[:len(mem.RecentRounds)-keepCount]
	toKeep := mem.RecentRounds[len(mem.RecentRounds)-keepCount:]

	// 如果没有 summarizer，只保留最近的轮次，不生成摘要
	if m.summarizer == nil {
		mem.RecentRounds = toKeep
		return nil
	}

	// 生成新摘要
	newSummary, err := m.summarizer.SummarizeRounds(ctx, toCompress)
	if err != nil {
		return err
	}

	// 合并摘要
	mem.Summary = m.mergeSummaries(mem.Summary, newSummary)
	mem.RecentRounds = toKeep

	return nil
}

// mergeSummaries 合并摘要
func (m *Manager) mergeSummaries(old, new string) string {
	if old == "" {
		return new
	}
	if new == "" {
		return old
	}

	merged := old + "\n" + new
	runes := []rune(merged)
	maxLen := m.config.MaxSummaryLength * 2
	if len(runes) > maxLen {
		merged = string(runes[len(runes)-maxLen:])
	}
	return merged
}

// AddFacts 添加关键事实
func (m *Manager) AddFacts(mem *StockMemory, facts []MemoryEntry) {
	mem.KeyFacts = append(mem.KeyFacts, facts...)
	// 限制数量
	if len(mem.KeyFacts) > m.config.MaxKeyFacts {
		mem.KeyFacts = mem.KeyFacts[len(mem.KeyFacts)-m.config.MaxKeyFacts:]
	}
}

// ExtractAndAddFacts 从内容中提取并添加事实
func (m *Manager) ExtractAndAddFacts(ctx context.Context, mem *StockMemory, content, source string) error {
	facts, err := m.summarizer.ExtractFacts(ctx, content, source)
	if err != nil {
		return err
	}
	m.AddFacts(mem, facts)
	return nil
}

// ExtractKeyPoints 智能提取讨论关键点
func (m *Manager) ExtractKeyPoints(ctx context.Context, discussions []DiscussionInput) ([]string, error) {
	if m.summarizer == nil {
		// 无 LLM 时使用简单截取
		return m.fallbackExtractKeyPoints(discussions), nil
	}
	return m.summarizer.ExtractKeyPoints(ctx, discussions)
}

// fallbackExtractKeyPoints 无 LLM 时的降级提取
func (m *Manager) fallbackExtractKeyPoints(discussions []DiscussionInput) []string {
	points := make([]string, 0, len(discussions))
	for _, d := range discussions {
		runes := []rune(d.Content)
		content := d.Content
		if len(runes) > 80 {
			content = string(runes[:80]) + "..."
		}
		points = append(points, fmt.Sprintf("%s: %s", d.AgentName, content))
	}
	return points
}

// DeleteMemory 删除指定股票的记忆
func (m *Manager) DeleteMemory(stockCode string) error {
	return m.storage.Delete(stockCode)
}

// Close 释放资源
func (m *Manager) Close() {
	// 关闭异步保存协程
	close(m.closeCh)

	if jt, ok := m.tokenizer.(*GseTokenizer); ok {
		jt.Free()
	}
}
