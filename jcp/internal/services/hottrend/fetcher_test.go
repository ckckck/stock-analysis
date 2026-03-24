package hottrend

import (
	"testing"
)

// TestAllFetchers 测试所有平台的 fetcher
func TestAllFetchers(t *testing.T) {
	fetchers := []Fetcher{
		NewWeiboFetcher(),
		NewZhihuFetcher(),
		NewBilibiliFetcher(),
		NewBaiduFetcher(),
		NewDouyinFetcher(),
		NewToutiaoFetcher(),
	}

	for _, f := range fetchers {
		t.Run(f.Platform(), func(t *testing.T) {
			items, err := f.Fetch()
			if err != nil {
				t.Errorf("%s fetch failed: %v", f.PlatformCN(), err)
				return
			}
			t.Logf("%s: 获取到 %d 条热点", f.PlatformCN(), len(items))
			if len(items) > 0 {
				t.Logf("  第1条: %s", items[0].Title)
			}
		})
	}
}
