package services

import (
	"testing"
)

func TestGetTelegraphList(t *testing.T) {
	service := NewNewsService()

	telegraphs, err := service.GetTelegraphList()
	if err != nil {
		t.Fatalf("获取快讯失败: %v", err)
	}

	t.Logf("获取到 %d 条快讯", len(telegraphs))

	if len(telegraphs) == 0 {
		t.Error("未获取到任何快讯，可能是解析选择器有问题")
		return
	}

	// 打印前5条快讯
	for i, tg := range telegraphs {
		if i >= 5 {
			break
		}
		t.Logf("[%d] 时间: %s", i+1, tg.Time)
		t.Logf("    内容: %s", truncate(tg.Content, 80))
		t.Logf("    URL: %s", tg.URL)
	}

	// 验证 URL 格式
	hasURL := false
	for _, tg := range telegraphs {
		if tg.URL != "" {
			hasURL = true
			break
		}
	}
	if !hasURL {
		t.Log("警告: 没有解析到任何快讯 URL")
	}
}

func TestGetLatestTelegraph(t *testing.T) {
	service := NewNewsService()

	// 先获取列表填充缓存
	_, err := service.GetTelegraphList()
	if err != nil {
		t.Fatalf("获取快讯失败: %v", err)
	}

	latest := service.GetLatestTelegraph()
	if latest == nil {
		t.Error("未获取到最新快讯")
		return
	}

	t.Logf("最新快讯时间: %s", latest.Time)
	t.Logf("最新快讯内容: %s", truncate(latest.Content, 150))
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
