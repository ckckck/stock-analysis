package memory

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Relevance 相关性计算器
type Relevance struct {
	tokenizer Tokenizer
}

// NewRelevance 创建相关性计算器
func NewRelevance(tokenizer Tokenizer) *Relevance {
	return &Relevance{tokenizer: tokenizer}
}

// ScoredEntry 带分数的记忆条目
type ScoredEntry struct {
	Entry MemoryEntry
	Score float64
}

// FindRelevant 查找相关的记忆条目
func (r *Relevance) FindRelevant(facts []MemoryEntry, query string, limit int) []MemoryEntry {
	if len(facts) == 0 {
		return nil
	}

	// 提取查询关键词
	queryKeywords := r.tokenizer.Extract(query, 10)
	if len(queryKeywords) == 0 {
		queryKeywords = r.tokenizer.Cut(query)
	}

	// 计算每个事实的相关性分数
	scored := make([]ScoredEntry, 0, len(facts))
	for _, fact := range facts {
		score := r.calculateScore(queryKeywords, fact)
		if score > 0.1 {
			scored = append(scored, ScoredEntry{Entry: fact, Score: score})
		}
	}

	// 按分数排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 取 Top N
	result := make([]MemoryEntry, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		result = append(result, scored[i].Entry)
	}
	return result
}

// calculateScore 计算相关性分数
func (r *Relevance) calculateScore(queryKeywords []string, fact MemoryEntry) float64 {
	if len(queryKeywords) == 0 {
		return 0
	}

	matches := 0
	// 检查关键词匹配
	for _, qk := range queryKeywords {
		for _, fk := range fact.Keywords {
			if strings.Contains(fk, qk) || strings.Contains(qk, fk) {
				matches++
				break
			}
		}
		// 检查内容匹配
		if strings.Contains(fact.Content, qk) {
			matches++
		}
	}

	// 基础分数
	score := float64(matches) / float64(len(queryKeywords)*2)

	// 乘以重要性权重
	score *= math.Max(0.5, fact.Weight)

	// 乘以时间衰减
	score *= r.timeDecay(fact.Timestamp)

	return score
}

// timeDecay 时间衰减函数
func (r *Relevance) timeDecay(timestamp int64) float64 {
	age := time.Now().UnixMilli() - timestamp
	days := float64(age) / (24 * 60 * 60 * 1000)
	// 7天内权重为1，之后逐渐衰减，最低0.3
	if days <= 7 {
		return 1.0
	}
	return math.Max(0.3, 1.0-0.05*(days-7))
}
