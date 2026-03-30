package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	compressAfterDays = 7
	deleteAfterDays   = 30
	maxTotalSizeBytes = 300 * 1024 * 1024
	rateLimitWindow   = 5 * time.Second
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelColors = map[Level]string{
	DEBUG: "\033[36m", // cyan
	INFO:  "\033[32m", // green
	WARN:  "\033[33m", // yellow
	ERROR: "\033[31m", // red
}

const resetColor = "\033[0m"

// 全局配置
var (
	globalLevel   = INFO
	moduleLevels  = make(map[string]Level)
	globalFile    *os.File
	globalMu      sync.Mutex
	enableConsole = true  // 是否输出到控制台
	enableFile    = false // 是否输出到文件
	nowFunc       = time.Now
	rateLimits    = make(map[string]*rateLimitEntry)
)

type rateLimitEntry struct {
	lastLoggedAt    time.Time
	suppressedCount int
}

// Logger 日志记录器
type Logger struct {
	module string
}

// SetGlobalLevel 设置全局日志级别
func SetGlobalLevel(level Level) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLevel = level
}

// SetModuleLevels 设置模块级日志级别
func SetModuleLevels(levels map[string]Level) {
	globalMu.Lock()
	defer globalMu.Unlock()
	moduleLevels = make(map[string]Level, len(levels))
	for module, level := range levels {
		normalized := normalizeModuleName(module)
		if normalized == "" {
			continue
		}
		moduleLevels[normalized] = level
	}
}

// InitFileLogger 初始化文件日志
func InitFileLogger(logDir string) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	if err := applyRetentionPolicy(logDir, time.Now()); err != nil {
		return fmt.Errorf("应用日志保留策略失败: %w", err)
	}

	// 按日期命名日志文件
	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	globalFile = f
	enableFile = true
	return nil
}

func applyRetentionPolicy(logDir string, now time.Time) error {
	return applyRetentionPolicyWithLimit(logDir, now, maxTotalSizeBytes)
}

func applyRetentionPolicyWithLimit(logDir string, now time.Time, maxTotalSize int64) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		logDate, compressed, ok := parseLogDate(fileName)
		if !ok {
			continue
		}
		ageDays := int(now.Sub(logDate).Hours() / 24)
		fullPath := filepath.Join(logDir, fileName)

		switch {
		case ageDays > deleteAfterDays:
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		case ageDays > compressAfterDays && !compressed:
			if err := gzipFile(fullPath); err != nil {
				return err
			}
		}
	}

	if maxTotalSize > 0 {
		if err := enforceTotalSizeLimit(logDir, maxTotalSize); err != nil {
			return err
		}
	}

	return nil
}

type logFileInfo struct {
	name       string
	path       string
	date       time.Time
	compressed bool
	size       int64
}

func enforceTotalSizeLimit(logDir string, maxTotalSize int64) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	var files []logFileInfo
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		logDate, compressed, ok := parseLogDate(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, logFileInfo{
			name:       entry.Name(),
			path:       filepath.Join(logDir, entry.Name()),
			date:       logDate,
			compressed: compressed,
			size:       info.Size(),
		})
		totalSize += info.Size()
	}

	if totalSize <= maxTotalSize {
		return nil
	}

	for _, preferCompressed := range []bool{true, false} {
		candidates := make([]logFileInfo, 0)
		for _, file := range files {
			if file.compressed == preferCompressed {
				candidates = append(candidates, file)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].date.Before(candidates[j].date)
		})
		for _, candidate := range candidates {
			if totalSize <= maxTotalSize {
				return nil
			}
			if err := os.Remove(candidate.path); err != nil && !os.IsNotExist(err) {
				return err
			}
			totalSize -= candidate.size
		}
	}

	return nil
}

func parseLogDate(fileName string) (time.Time, bool, bool) {
	compressed := strings.HasSuffix(fileName, ".log.gz")
	baseName := fileName
	if compressed {
		baseName = strings.TrimSuffix(fileName, ".gz")
	}
	if !strings.HasSuffix(baseName, ".log") {
		return time.Time{}, false, false
	}
	datePart := strings.TrimSuffix(baseName, ".log")
	parsed, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return time.Time{}, false, false
	}
	return parsed, compressed, true
}

func gzipFile(sourcePath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetPath := sourcePath + ".gz"
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	gzipWriter := gzip.NewWriter(targetFile)
	_, copyErr := io.Copy(gzipWriter, sourceFile)
	closeWriterErr := gzipWriter.Close()
	closeFileErr := targetFile.Close()
	if copyErr != nil {
		_ = os.Remove(targetPath)
		return copyErr
	}
	if closeWriterErr != nil {
		_ = os.Remove(targetPath)
		return closeWriterErr
	}
	if closeFileErr != nil {
		_ = os.Remove(targetPath)
		return closeFileErr
	}
	if err := os.Remove(sourcePath); err != nil {
		return err
	}
	return nil
}

// SetConsoleOutput 设置是否输出到控制台
func SetConsoleOutput(enable bool) {
	globalMu.Lock()
	defer globalMu.Unlock()
	enableConsole = enable
}

// Close 关闭日志文件
func Close() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalFile != nil {
		globalFile.Close()
		globalFile = nil
	}
	enableFile = false
}

// New 创建新的日志记录器
func New(module string) *Logger {
	return &Logger{
		module: normalizeModuleName(module),
	}
}

func resetRateLimitState() {
	globalMu.Lock()
	defer globalMu.Unlock()
	rateLimits = make(map[string]*rateLimitEntry)
}

// log 内部日志方法
func (l *Logger) log(level Level, format string, args ...any) {
	// 先在锁外准备数据，减少锁持有时间
	now := nowFunc()
	timestamp := now.Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	levelName := levelNames[level]

	globalMu.Lock()
	defer globalMu.Unlock()

	// 检查日志级别
	effectiveLevel := globalLevel
	if moduleLevel, ok := moduleLevels[normalizeModuleName(l.module)]; ok {
		effectiveLevel = moduleLevel
	}
	if level < effectiveLevel {
		return
	}

	if !shouldLogLocked(l.module, level, &msg, now) {
		return
	}

	// 输出到控制台（带颜色）
	if enableConsole {
		color := levelColors[level]
		fmt.Fprintf(os.Stderr, "%s%s%s [%s] %s: %s\n",
			color, levelName, resetColor,
			timestamp, l.module, msg)
	}

	// 输出到文件（无颜色）
	if enableFile && globalFile != nil {
		fmt.Fprintf(globalFile, "%s [%s] %s: %s\n",
			levelName, timestamp, l.module, msg)
	}
}

func shouldLogLocked(module string, level Level, msg *string, now time.Time) bool {
	if level < WARN {
		return true
	}
	key := module + "|" + *msg
	entry, ok := rateLimits[key]
	if !ok {
		rateLimits[key] = &rateLimitEntry{lastLoggedAt: now}
		return true
	}
	if now.Sub(entry.lastLoggedAt) < rateLimitWindow {
		entry.suppressedCount++
		return false
	}
	if entry.suppressedCount > 0 {
		*msg = fmt.Sprintf("%s suppressedCount=%d", *msg, entry.suppressedCount)
	}
	entry.lastLoggedAt = now
	entry.suppressedCount = 0
	return true
}

func normalizeModuleName(module string) string {
	return strings.ToLower(strings.TrimSpace(module))
}

// ParseLevel 解析字符串日志级别
func ParseLevel(raw string) (Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return DEBUG, true
	case "INFO":
		return INFO, true
	case "WARN", "WARNING":
		return WARN, true
	case "ERROR":
		return ERROR, true
	default:
		return INFO, false
	}
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...any) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...any) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...any) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...any) {
	l.log(ERROR, format, args...)
}

// WithError 带错误的日志
func (l *Logger) WithError(err error) *Logger {
	if err != nil {
		l.Error("error: %v", err)
	}
	return l
}
