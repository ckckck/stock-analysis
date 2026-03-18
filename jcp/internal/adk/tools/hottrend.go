package tools

import (
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/services/hottrend"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetHotTrendInput 舆情热点输入参数
type GetHotTrendInput struct {
	Platform string `json:"platform,omitzero" jsonschema:"平台名称，可选值：weibo/zhihu/bilibili/baidu/douyin/toutiao，不填则获取所有平台"`
	Limit    int    `json:"limit,omitzero" jsonschema:"每个平台返回的热点条数，默认10条"`
}

// GetHotTrendOutput 舆情热点输出
type GetHotTrendOutput struct {
	Data string `json:"data" jsonschema:"舆情热点数据"`
}

// createHotTrendTool 创建舆情热点工具
func (r *Registry) createHotTrendTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetHotTrendInput) (GetHotTrendOutput, error) {
		fmt.Printf("[Tool:get_hottrend] 调用开始, platform=%s, limit=%d\n", input.Platform, input.Limit)

		if r.hotTrendService == nil {
			return GetHotTrendOutput{}, fmt.Errorf("舆情服务未初始化")
		}

		limit := input.Limit
		if limit <= 0 {
			limit = 10
		}
		if limit > 50 {
			limit = 50
		}

		var result strings.Builder

		if input.Platform != "" {
			// 获取单个平台
			trendResult := r.hotTrendService.GetHotTrend(input.Platform)
			formatTrendResult(&result, trendResult, limit)
		} else {
			// 获取所有平台
			results := r.hotTrendService.GetAllHotTrends()
			for _, trendResult := range results {
				formatTrendResult(&result, trendResult, limit)
				result.WriteString("\n")
			}
		}

		fmt.Printf("[Tool:get_hottrend] 调用完成\n")
		return GetHotTrendOutput{Data: result.String()}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_hottrend",
		Description: "获取全网舆情热点，支持微博、知乎、B站、百度、抖音、头条等平台的实时热搜榜单",
	}, handler)
}

// formatTrendResult 格式化热点结果
func formatTrendResult(sb *strings.Builder, tr hottrend.HotTrendResult, limit int) {
	if tr.Error != "" {
		sb.WriteString(fmt.Sprintf("【%s】获取失败: %s\n", tr.PlatformCN, tr.Error))
		return
	}

	sb.WriteString(fmt.Sprintf("【%s】热搜榜:\n", tr.PlatformCN))
	count := 0
	for _, item := range tr.Items {
		if count >= limit {
			break
		}
		if item.Extra != "" {
			sb.WriteString(fmt.Sprintf("  %d. %s (%s)\n", item.Rank, item.Title, item.Extra))
		} else {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", item.Rank, item.Title))
		}
		count++
	}
}
