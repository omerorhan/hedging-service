package service

import (
	"math"
	"strings"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

func mustParseISODate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05.000000000",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05.000000000",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func clampNonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func lastDayOfMonth(t time.Time) int {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

func midDayOfMonth(t time.Time) int {
	return int(math.Ceil(float64(lastDayOfMonth(t)) / 2.0))
}

func endOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	loc := t.Location()
	firstNext := time.Date(y, m+1, 1, 0, 0, 0, 0, loc)
	return firstNext.Add(-time.Nanosecond)
}

func daysBetween(a, b time.Time) int { return int(math.Round(b.Sub(a).Hours() / 24)) }

func getPairKey(from, to string) storage.PairKey {
	return storage.PairKey(strings.ToUpper(from) + "->" + strings.ToUpper(to))
}
