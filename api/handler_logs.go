package api

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net/http"
	"nofx/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetLogs returns live log entries from the in-memory ring buffer.
// Query params: since_id, limit, level, category
func (s *Server) handleGetLogs(c *gin.Context) {
	sinceID, _ := strconv.ParseInt(c.DefaultQuery("since_id", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	level := c.DefaultQuery("level", "")
	category := c.DefaultQuery("category", "")

	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	entries, latestID := logger.GlobalRingBuffer.Entries(sinceID, limit, level, category)
	if entries == nil {
		entries = []logger.LogEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":   entries,
		"latest_id": latestID,
	})
}

// handleExportLogs exports logs from the log file as CSV or JSON download.
// Query params: format (csv|json), category, date (YYYY-MM-DD), level
func (s *Server) handleExportLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	category := c.DefaultQuery("category", "")
	level := c.DefaultQuery("level", "")
	dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))

	logFileName := filepath.Join("data", fmt.Sprintf("nofx_%s.log", dateStr))
	file, err := os.Open(logFileName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Log file not found for date %s", dateStr)})
		return
	}
	defer file.Close()

	year := dateStr[:4]

	var entries []logger.LogEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry := logger.ParseLogLine(line, year)
		if entry == nil {
			continue
		}

		if category != "" && entry.Category != category {
			continue
		}
		if level != "" {
			entryLevelVal := levelValue(entry.Level)
			filterLevelVal := levelValue(strings.ToUpper(level))
			if entryLevelVal < filterLevelVal {
				continue
			}
		}

		entries = append(entries, *entry)
	}

	suffix := category
	if suffix == "" {
		suffix = "all"
	}
	baseName := fmt.Sprintf("nofx_%s_%s", dateStr, suffix)

	switch format {
	case "json":
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, baseName))
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, entries)

	default:
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, baseName))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Writer.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel compatibility

		w := csv.NewWriter(c.Writer)
		w.Write([]string{"timestamp", "level", "source", "category", "message"})
		for _, e := range entries {
			w.Write([]string{e.Timestamp, e.Level, e.Source, e.Category, e.Message})
		}
		w.Flush()
	}
}

// handleGetLogDates returns available log file dates for the export date picker.
func (s *Server) handleGetLogDates(c *gin.Context) {
	files, err := filepath.Glob("data/nofx_*.log")
	if err != nil {
		c.JSON(http.StatusOK, []string{})
		return
	}

	dates := make([]string, 0, len(files))
	for _, f := range files {
		base := filepath.Base(f)
		base = strings.TrimPrefix(base, "nofx_")
		base = strings.TrimSuffix(base, ".log")
		if len(base) == 10 {
			dates = append(dates, base)
		}
	}

	c.JSON(http.StatusOK, dates)
}

func levelValue(level string) int {
	switch level {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARNING", "WARN":
		return 2
	case "ERROR", "ERRO":
		return 3
	case "FATAL":
		return 4
	case "PANIC":
		return 5
	default:
		return 0
	}
}

// handleGetLogStats returns count of entries per level from the ring buffer.
func (s *Server) handleGetLogStats(c *gin.Context) {
	entries, _ := logger.GlobalRingBuffer.Entries(0, 0, "", "")

	stats := map[string]int{
		"total": len(entries),
		"info":  0,
		"warn":  0,
		"error": 0,
		"debug": 0,
	}
	for _, e := range entries {
		switch e.Level {
		case "INFO":
			stats["info"]++
		case "WARNING", "WARN":
			stats["warn"]++
		case "ERROR", "ERRO":
			stats["error"]++
		case "DEBUG":
			stats["debug"]++
		}
	}

	c.JSON(http.StatusOK, stats)
}
