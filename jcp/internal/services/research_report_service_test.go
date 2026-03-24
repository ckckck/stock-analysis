package services

import (
	"fmt"
	"testing"
)

func TestGetResearchReports(t *testing.T) {
	service := NewResearchReportService()

	// 测试获取平安银行(000001)的研报
	result, err := service.GetResearchReports("000001", 5, 1)
	if err != nil {
		t.Fatalf("获取研报失败: %v", err)
	}

	fmt.Printf("总数量: %d, 总页数: %d\n", result.TotalCount, result.TotalPage)
	fmt.Printf("本次返回: %d 条\n\n", len(result.Data))

	// 格式化输出
	text := service.FormatReportsToText(result.Data)
	fmt.Println(text)
}

func TestGetResearchReportsWithPrefix(t *testing.T) {
	service := NewResearchReportService()

	// 测试带前缀的股票代码
	result, err := service.GetResearchReports("sz000001", 3, 1)
	if err != nil {
		t.Fatalf("获取研报失败: %v", err)
	}

	fmt.Printf("带前缀测试 - 返回: %d 条\n", len(result.Data))
}

func TestGetReportContent(t *testing.T) {
	service := NewResearchReportService()

	// 先获取研报列表，拿到 infoCode
	result, err := service.GetResearchReports("000001", 1, 1)
	if err != nil {
		t.Fatalf("获取研报列表失败: %v", err)
	}

	if len(result.Data) == 0 {
		t.Skip("没有研报数据")
	}

	infoCode := result.Data[0].InfoCode
	fmt.Printf("研报标题: %s\n", result.Data[0].Title)
	fmt.Printf("InfoCode: %s\n\n", infoCode)

	// 获取研报内容
	content, err := service.GetReportContent(infoCode)
	if err != nil {
		t.Fatalf("获取研报内容失败: %v", err)
	}

	fmt.Printf("PDF链接: %s\n\n", content.PDFUrl)
	fmt.Printf("研报正文:\n%s\n", content.Content)
}
