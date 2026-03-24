package paths

import (
	"os"
	"path/filepath"
)

// GetDataDir 获取应用数据目录
func GetDataDir() string {
	userConfigDir, err := os.UserConfigDir()
	if err != nil || userConfigDir == "" {
		return filepath.Join(".", "data")
	}
	return filepath.Join(userConfigDir, "jcp")
}

// GetCacheDir 获取缓存目录
func GetCacheDir() string {
	return filepath.Join(GetDataDir(), "cache")
}

// GetScreeningDataDir 获取AI筛选数据目录
func GetScreeningDataDir() string {
	return GetScreeningDataDirFrom(GetDataDir())
}

// GetScreeningDataDirFrom 基于指定数据目录获取AI筛选数据目录
func GetScreeningDataDirFrom(dataDir string) string {
	return filepath.Join(dataDir, "screening")
}

// GetScreeningDBPath 获取AI筛选SQLite数据库路径
func GetScreeningDBPath() string {
	return GetScreeningDBPathFrom(GetDataDir())
}

// GetScreeningDBPathFrom 基于指定数据目录获取AI筛选SQLite数据库路径
func GetScreeningDBPathFrom(dataDir string) string {
	return filepath.Join(GetScreeningDataDirFrom(dataDir), "screening.db")
}

// EnsureCacheDir 确保缓存目录存在并返回路径
func EnsureCacheDir(subDir string) string {
	dir := filepath.Join(GetCacheDir(), subDir)
	os.MkdirAll(dir, 0755)
	return dir
}
