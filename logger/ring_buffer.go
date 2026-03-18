package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// LogEntry represents a single structured log entry.
type LogEntry struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Category  string `json:"category"`
	Message   string `json:"message"`
}

// RingBufferHook is a logrus Hook that captures log entries into a fixed-size
// circular buffer for live viewing via API.
type RingBufferHook struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	head     int // next write position
	count    int // current number of entries (up to capacity)
	nextID   atomic.Int64
}

// NewRingBufferHook creates a ring buffer hook with the given capacity.
func NewRingBufferHook(capacity int) *RingBufferHook {
	return &RingBufferHook{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Levels implements logrus.Hook — capture all levels.
func (h *RingBufferHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire implements logrus.Hook — called for every log entry.
func (h *RingBufferHook) Fire(entry *logrus.Entry) error {
	source := resolveSource()
	category := classifyCategory(source, entry.Message)

	le := LogEntry{
		ID:        h.nextID.Add(1),
		Timestamp: entry.Time.Format("2006-01-02 15:04:05"),
		Level:     strings.ToUpper(entry.Level.String()),
		Source:    source,
		Category:  category,
		Message:   entry.Message,
	}

	h.mu.Lock()
	h.entries[h.head] = le
	h.head = (h.head + 1) % h.capacity
	if h.count < h.capacity {
		h.count++
	}
	h.mu.Unlock()

	return nil
}

// Entries returns log entries matching the filters.
// sinceID: only entries with ID > sinceID (0 = all).
// limit: max entries to return (0 = no limit).
// level: minimum level filter ("" = all, "WARN" = WARN+ERROR, etc.).
// category: exact match ("" = all).
// Returns the matching entries (oldest first) and the latest ID seen.
func (h *RingBufferHook) Entries(sinceID int64, limit int, level, category string) ([]LogEntry, int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return nil, 0
	}

	minLevel := parseLevelFilter(level)
	result := make([]LogEntry, 0, min(h.count, 256))

	start := h.head - h.count
	if start < 0 {
		start += h.capacity
	}

	var latestID int64
	for i := 0; i < h.count; i++ {
		idx := (start + i) % h.capacity
		e := h.entries[idx]

		if e.ID > latestID {
			latestID = e.ID
		}
		if sinceID > 0 && e.ID <= sinceID {
			continue
		}
		if !matchesLevel(e.Level, minLevel) {
			continue
		}
		if category != "" && e.Category != category {
			continue
		}

		result = append(result, e)
	}

	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result, latestID
}

// resolveSource walks the call stack to find the actual caller, skipping
// logrus internals and our logger wrapper.
func resolveSource() string {
	for i := 4; i < 12; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if strings.Contains(file, "logrus") || strings.HasSuffix(file, "logger/logger.go") || strings.HasSuffix(file, "logger/ring_buffer.go") {
			continue
		}
		dir := filepath.Dir(file)
		pkg := filepath.Base(dir)
		return fmt.Sprintf("%s/%s:%d", pkg, filepath.Base(file), line)
	}
	return ""
}

// classifyCategory determines the log category from source path and message content.
func classifyCategory(source, message string) string {
	if strings.Contains(message, "[Grid]") {
		return "grid"
	}
	if strings.Contains(source, "hyperliquid") ||
		strings.Contains(source, "binance") ||
		strings.Contains(source, "bybit") ||
		strings.Contains(source, "okx") ||
		strings.Contains(source, "bitget") ||
		strings.Contains(source, "gate") ||
		strings.Contains(source, "kucoin") ||
		strings.Contains(source, "aster") ||
		strings.Contains(source, "lighter") {
		return "exchange"
	}
	if strings.Contains(source, "kernel/") {
		return "ai"
	}
	return "system"
}

var levelOrder = map[string]int{
	"DEBUG":   0,
	"INFO":    1,
	"WARNING": 2,
	"WARN":    2,
	"ERROR":   3,
	"FATAL":   4,
	"PANIC":   5,
}

func parseLevelFilter(level string) int {
	if level == "" {
		return 0
	}
	if v, ok := levelOrder[strings.ToUpper(level)]; ok {
		return v
	}
	return 0
}

func matchesLevel(entryLevel string, minLevel int) bool {
	v, ok := levelOrder[entryLevel]
	if !ok {
		return true
	}
	return v >= minLevel
}

// GlobalRingBuffer is the singleton ring buffer used by the logger Hook.
var GlobalRingBuffer = NewRingBufferHook(1000)

// ClassifyLogLine classifies a raw log line (from file) into a category.
// Used by the export endpoint to filter log file contents.
func ClassifyLogLine(line string) string {
	if strings.Contains(line, "[Grid]") {
		return "grid"
	}
	if strings.Contains(line, "hyperliquid") ||
		strings.Contains(line, "binance") ||
		strings.Contains(line, "bybit") ||
		strings.Contains(line, "okx") ||
		strings.Contains(line, "bitget") {
		return "exchange"
	}
	if strings.Contains(line, "kernel/") || strings.Contains(line, "grid_engine") {
		return "ai"
	}
	return "system"
}

// ParseLogLine parses a raw log file line into structured fields.
// Expected format: "MM-DD HH:MM:SS [LEVEL] source message"
func ParseLogLine(line string, year string) *LogEntry {
	if len(line) < 22 {
		return nil
	}

	datePart := line[:14]
	rest := line[15:]

	bracketOpen := strings.Index(rest, "[")
	bracketClose := strings.Index(rest, "]")
	if bracketOpen < 0 || bracketClose < 0 || bracketClose <= bracketOpen {
		return nil
	}

	level := rest[bracketOpen+1 : bracketClose]
	after := strings.TrimSpace(rest[bracketClose+1:])

	source := ""
	message := after
	parts := strings.SplitN(after, " ", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], "/") || strings.Contains(parts[0], ".go:")) {
		source = parts[0]
		message = parts[1]
	}

	ts := fmt.Sprintf("%s-%s", year, datePart)
	// Convert "2026-03-18 10:02:44" format
	if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
		ts = t.Format("2006-01-02 15:04:05")
	}

	return &LogEntry{
		Timestamp: ts,
		Level:     strings.ToUpper(level),
		Source:    source,
		Category:  classifyCategory(source, message),
		Message:   message,
	}
}
