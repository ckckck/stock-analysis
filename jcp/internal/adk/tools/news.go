package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetNewsInput 快讯输入参数
type GetNewsInput struct {
	Limit int `json:"limit,omitzero" jsonschema:"返回条数，默认10条"`
}

// GetNewsOutput 快讯输出
type GetNewsOutput struct {
	Data string `json:"data" jsonschema:"财经快讯列表"`
}

// createNewsTool 创建快讯工具
func (r *Registry) createNewsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input GetNewsInput) (GetNewsOutput, error) {
		fmt.Printf("[Tool:get_news] 调用开始, limit=%d\n", input.Limit)

		news, err := r.newsService.GetTelegraphList()
		if err != nil {
			fmt.Printf("[Tool:get_news] 错误: %v\n", err)
			return GetNewsOutput{}, err
		}

		limit := input.Limit
		if limit == 0 {
			limit = 10
		}
		if limit > len(news) {
			limit = len(news)
		}

		var result string
		for i := 0; i < limit; i++ {
			n := news[i]
			result += fmt.Sprintf("[%s] %s\n", n.Time, n.Content)
		}

		fmt.Printf("[Tool:get_news] 调用完成, 返回%d条快讯\n", limit)
		return GetNewsOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_news",
		Description: "获取最新财经快讯，来源于财联社",
	}, handler)
}
