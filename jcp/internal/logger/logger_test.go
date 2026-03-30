package logger

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestDefaultInfoLevelFiltersDebugLogs(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
	})

	SetConsoleOutput(false)
	SetGlobalLevel(INFO)

	logDir := t.TempDir()
	if err := InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	log := New("logger-test")
	log.Debug("debug should be filtered")
	log.Info("info should be kept")
	Close()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if strings.Contains(text, "debug should be filtered") {
		t.Fatalf("log file contains filtered debug entry: %q", text)
	}
	if !strings.Contains(text, "info should be kept") {
		t.Fatalf("log file = %q, want info entry", text)
	}
}

func TestInitFileLoggerCompressesLogsOlderThanSevenDays(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
	})

	SetConsoleOutput(false)
	logDir := t.TempDir()
	logName := time.Now().AddDate(0, 0, -8).Format("2006-01-02") + ".log"
	logPath := filepath.Join(logDir, logName)
	if err := os.WriteFile(logPath, []byte("legacy log entry\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}
	Close()

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("old log should be compressed, stat err = %v", err)
	}

	gzPath := logPath + ".gz"
	gzFile, err := os.Open(gzPath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", gzPath, err)
	}
	defer gzFile.Close()

	reader, err := gzip.NewReader(gzFile)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(content) != "legacy log entry\n" {
		t.Fatalf("compressed log content = %q", string(content))
	}
}

func TestInitFileLoggerDeletesLogsOlderThanThirtyDays(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
	})

	SetConsoleOutput(false)
	logDir := t.TempDir()
	oldLogName := time.Now().AddDate(0, 0, -31).Format("2006-01-02") + ".log"
	oldGzName := time.Now().AddDate(0, 0, -45).Format("2006-01-02") + ".log.gz"
	for _, name := range []string{oldLogName, oldGzName} {
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("expired\n"), 0644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}

	if err := InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}
	Close()

	for _, name := range []string{oldLogName, oldGzName} {
		if _, err := os.Stat(filepath.Join(logDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expired log %q should be deleted, stat err = %v", name, err)
		}
	}
}

func TestRetentionPolicyPrefersDeletingOldestCompressedLogsBeforeRawLogsWhenSizeExceedsLimit(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
	})

	SetConsoleOutput(false)
	logDir := t.TempDir()
	files := map[string]int{
		time.Now().AddDate(0, 0, -20).Format("2006-01-02") + ".log.gz": 90,
		time.Now().AddDate(0, 0, -15).Format("2006-01-02") + ".log.gz": 80,
		time.Now().AddDate(0, 0, -3).Format("2006-01-02") + ".log":    70,
	}
	for name, size := range files {
		if err := os.WriteFile(filepath.Join(logDir, name), bytesOfSize(size), 0644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}

	if err := applyRetentionPolicyWithLimit(logDir, time.Now(), 150); err != nil {
		t.Fatalf("applyRetentionPolicyWithLimit() error = %v", err)
	}

	remainingEntries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	remaining := make([]string, 0, len(remainingEntries))
	for _, entry := range remainingEntries {
		remaining = append(remaining, entry.Name())
	}
	sort.Strings(remaining)

	oldestCompressed := time.Now().AddDate(0, 0, -20).Format("2006-01-02") + ".log.gz"
	newerCompressed := time.Now().AddDate(0, 0, -15).Format("2006-01-02") + ".log.gz"
	rawLog := time.Now().AddDate(0, 0, -3).Format("2006-01-02") + ".log"

	if containsString(remaining, oldestCompressed) {
		t.Fatalf("expected oldest compressed log %q to be deleted first, remaining=%v", oldestCompressed, remaining)
	}
	if !containsString(remaining, newerCompressed) {
		t.Fatalf("expected newer compressed log %q to remain, remaining=%v", newerCompressed, remaining)
	}
	if !containsString(remaining, rawLog) {
		t.Fatalf("expected raw log %q to remain while compressed logs still exist, remaining=%v", rawLog, remaining)
	}
}

func TestWarnLogsAreRateLimitedAndReportSuppressedCountAfterWindow(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
		resetRateLimitState()
		nowFunc = time.Now
	})

	SetConsoleOutput(false)
	SetGlobalLevel(INFO)
	resetRateLimitState()

	baseTime := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	timestamps := []time.Time{
		baseTime,
		baseTime.Add(1 * time.Second),
		baseTime.Add(2 * time.Second),
		baseTime.Add(6 * time.Second),
	}
	callIndex := 0
	nowFunc = func() time.Time {
		if callIndex >= len(timestamps) {
			return timestamps[len(timestamps)-1]
		}
		value := timestamps[callIndex]
		callIndex++
		return value
	}

	logDir := t.TempDir()
	if err := InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	log := New("logger-test")
	log.Warn("module=market action=kline.empty symbol=sh600000")
	log.Warn("module=market action=kline.empty symbol=sh600000")
	log.Warn("module=market action=kline.empty symbol=sh600000")
	log.Warn("module=market action=kline.empty symbol=sh600000")
	Close()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if strings.Count(text, "module=market action=kline.empty symbol=sh600000") != 2 {
		t.Fatalf("expected only first and post-window warn logs to remain, log file = %q", text)
	}
	if !strings.Contains(text, "suppressedCount=2") {
		t.Fatalf("expected suppressedCount=2 in resumed warn log, log file = %q", text)
	}
}

func TestModuleLevelOverrideAllowsDebugForConfiguredModule(t *testing.T) {
	t.Cleanup(func() {
		Close()
		SetGlobalLevel(INFO)
		SetConsoleOutput(true)
		SetModuleLevels(nil)
		resetRateLimitState()
		nowFunc = time.Now
	})

	SetConsoleOutput(false)
	SetGlobalLevel(INFO)
	SetModuleLevels(map[string]Level{
		"meeting": DEBUG,
	})

	logDir := t.TempDir()
	if err := InitFileLogger(logDir); err != nil {
		t.Fatalf("InitFileLogger() error = %v", err)
	}

	New("app").Debug("app debug should stay filtered")
	New("Meeting").Debug("meeting debug should be emitted")
	Close()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if strings.Contains(text, "app debug should stay filtered") {
		t.Fatalf("unexpected app debug entry in log file: %q", text)
	}
	if !strings.Contains(text, "meeting debug should be emitted") {
		t.Fatalf("expected meeting module debug entry in log file: %q", text)
	}
}

func bytesOfSize(size int) []byte {
	return []byte(strings.Repeat("a", size))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
