package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// SearchStocksInput 股票搜索输入参数
type SearchStocksInput struct {
	Keyword string `json:"keyword" jsonschema:"搜索关键词，支持股票代码或名称"`
	Limit   int    `json:"limit,omitzero" jsonschema:"返回条数，默认10条"`
}

// SearchStocksOutput 股票搜索输出
type SearchStocksOutput struct {
	Data string `json:"data" jsonschema:"搜索结果列表"`
}

// createSearchStocksTool 创建股票搜索工具
func (r *Registry) createSearchStocksTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input SearchStocksInput) (SearchStocksOutput, error) {
		fmt.Printf("[Tool:search_stocks] 调用开始, keyword=%s, limit=%d\n", input.Keyword, input.Limit)

		if input.Keyword == "" {
			fmt.Println("[Tool:search_stocks] 错误: 未提供搜索关键词")
			return SearchStocksOutput{Data: "请提供搜索关键词"}, nil
		}

		limit := input.Limit
		if limit == 0 {
			limit = 10
		}

		results := r.configService.SearchStocks(input.Keyword, limit)

		var result string
		for _, s := range results {
			result += fmt.Sprintf("%s(%s) - %s [%s]\n",
				s.Name, s.Symbol, s.Industry, s.Market)
		}

		if result == "" {
			result = "未找到匹配的股票"
		}

		fmt.Printf("[Tool:search_stocks] 调用完成, 返回%d条结果\n", len(results))
		return SearchStocksOutput{Data: result}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "search_stocks",
		Description: "搜索股票，支持按代码或名称搜索",
	}, handler)
}
