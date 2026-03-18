package memory

import (
	"strings"

	"github.com/go-ego/gse"
	"github.com/go-ego/gse/hmm/extracker"
)

// Tokenizer 分词器接口
type Tokenizer interface {
	Extract(text string, topK int) []string
	Cut(text string) []string
}

// GseTokenizer 基于 GSE 的分词器（纯 Go 实现，无 CGO 依赖）
type GseTokenizer struct {
	seg       gse.Segmenter
	extractor extracker.TagExtracter
	stopWords map[string]bool
}

// NewJiebaTokenizer 创建分词器（保持函数名兼容）
func NewJiebaTokenizer() *GseTokenizer {
	t := &GseTokenizer{
		stopWords: defaultStopWords(),
	}
	// 加载内嵌的中文词典
	t.seg.LoadDictEmbed("zh")
	// 初始化关键词提取器
	t.extractor.WithGse(t.seg)
	t.extractor.LoadIdf()
	return t
}

// Free 释放资源（GSE 不需要手动释放，保持接口兼容）
func (t *GseTokenizer) Free() {
	// GSE 是纯 Go 实现，不需要手动释放资源
}

// Extract 提取关键词（使用 TF-IDF）
func (t *GseTokenizer) Extract(text string, topK int) []string {
	segments := t.extractor.ExtractTags(text, topK*2)
	result := make([]string, 0, topK)
	for _, seg := range segments {
		if !t.stopWords[seg.Text] && len([]rune(seg.Text)) >= 2 {
			result = append(result, seg.Text)
			if len(result) >= topK {
				break
			}
		}
	}
	return result
}

// Cut 分词
func (t *GseTokenizer) Cut(text string) []string {
	words := t.seg.Cut(text, true)
	result := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w != "" && !t.stopWords[w] && len([]rune(w)) >= 2 {
			result = append(result, w)
		}
	}
	return result
}

// defaultStopWords 默认停用词
func defaultStopWords() map[string]bool {
	words := []string{
		"的", "是", "在", "了", "和", "与", "或", "这", "那", "有",
		"个", "我", "你", "他", "她", "它", "们", "吗", "呢", "吧",
		"啊", "哦", "嗯", "呀", "哈", "哪", "什么", "怎么", "为什么",
		"可以", "可能", "应该", "需要", "能够", "已经", "正在",
		"一个", "一些", "这个", "那个", "这些", "那些", "如果",
		"但是", "因为", "所以", "虽然", "然后", "而且", "或者",
		"不是", "没有", "不会", "不能", "还是", "就是", "只是",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}
