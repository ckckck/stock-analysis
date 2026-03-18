package hottrend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      []HotItem `json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileCache 文件缓存管理器
type FileCache struct {
	cacheDir string
	ttl      time.Duration
	mu       sync.RWMutex
}

// NewFileCache 创建文件缓存
func NewFileCache(cacheDir string, ttl time.Duration) (*FileCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	return &FileCache{
		cacheDir: cacheDir,
		ttl:      ttl,
	}, nil
}

// cacheFilePath 获取缓存文件路径
func (c *FileCache) cacheFilePath(platform string) string {
	return filepath.Join(c.cacheDir, platform+".json")
}

// Get 获取缓存数据
func (c *FileCache) Get(platform string) ([]HotItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := c.cacheFilePath(platform)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.UpdatedAt) > c.ttl {
		return nil, false
	}

	return entry.Data, true
}

// Set 设置缓存数据
func (c *FileCache) Set(platform string, items []HotItem) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := CacheEntry{
		Data:      items,
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(c.cacheFilePath(platform), data, 0644)
}
