package memory

import (
	"testing"
)

func TestNewJiebaTokenizer(t *testing.T) {
	tokenizer := NewJiebaTokenizer()
	if tokenizer == nil {
		t.Fatal("NewJiebaTokenizer() returned nil")
	}
	defer tokenizer.Free()

	if tokenizer.stopWords == nil {
		t.Error("stopWords should not be nil")
	}
}

func TestGseTokenizer_Cut(t *testing.T) {
	tokenizer := NewJiebaTokenizer()
	defer tokenizer.Free()

	tests := []struct {
		name     string
		input    string
		minWords int // 期望最少分出的词数
	}{
		{
			name:     "中文句子",
			input:    "我爱北京天安门",
			minWords: 2,
		},
		{
			name:     "股票相关",
			input:    "贵州茅台股价创新高",
			minWords: 3,
		},
		{
			name:     "混合文本",
			input:    "A股市场今日大涨3%",
			minWords: 2,
		},
		{
			name:     "空字符串",
			input:    "",
			minWords: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizer.Cut(tt.input)
			if len(result) < tt.minWords {
				t.Errorf("Cut(%q) = %v, want at least %d words",
					tt.input, result, tt.minWords)
			}
			t.Logf("Cut(%q) = %v", tt.input, result)
		})
	}
}
