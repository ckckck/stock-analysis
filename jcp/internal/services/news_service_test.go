package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTelegraphListParsesTelegraphs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<div class="telegraph-item">
				<div class="telegraph-content-box">
					<span class="telegraph-time-box">09:30</span>
					<span><div> 第一条
						快讯内容 </div></span>
				</div>
				<div class="subject-bottom-box"><a href="/detail/123">详情</a></div>
			</div>
			<div class="telegraph-item">
				<div class="telegraph-content-box">
					<span class="telegraph-time-box">09:31</span>
					<span><div>第二条快讯内容</div></span>
				</div>
			</div>
		`))
	}))
	defer server.Close()

	service := NewNewsService()
	service.client = server.Client()
	service.telegraphURL = server.URL

	telegraphs, err := service.GetTelegraphList()
	if err != nil {
		t.Fatalf("获取快讯失败: %v", err)
	}

	if len(telegraphs) != 2 {
		t.Fatalf("快讯数量 = %d，期望 2", len(telegraphs))
	}
	if telegraphs[0].Time != "09:30" {
		t.Fatalf("第一条时间 = %q，期望 09:30", telegraphs[0].Time)
	}
	if telegraphs[0].Content != "第一条 快讯内容" {
		t.Fatalf("第一条内容 = %q，期望清理后的内容", telegraphs[0].Content)
	}
	if telegraphs[0].URL != "https://www.cls.cn/detail/123" {
		t.Fatalf("第一条 URL = %q", telegraphs[0].URL)
	}
}

func TestGetLatestTelegraphUsesCachedList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<div class="telegraph-item">
				<div class="telegraph-content-box">
					<span class="telegraph-time-box">10:00</span>
					<span><div>最新快讯</div></span>
				</div>
			</div>
		`))
	}))
	defer server.Close()

	service := NewNewsService()
	service.client = server.Client()
	service.telegraphURL = server.URL

	_, err := service.GetTelegraphList()
	if err != nil {
		t.Fatalf("获取快讯失败: %v", err)
	}

	latest := service.GetLatestTelegraph()
	if latest == nil {
		t.Error("未获取到最新快讯")
		return
	}

	if latest.Time != "10:00" || latest.Content != "最新快讯" {
		t.Fatalf("最新快讯 = %+v", latest)
	}
}
