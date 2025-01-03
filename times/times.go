package times

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	FriendlyShort     = "Mon, Jan 02 15:04 MST"
	Friendly          = "Mon, Jan 02, 2006 at 15:04 MST"
	DateDirectory     = "2006.01.02"
	DateTimeDirectory = "2006.01.02.15.04.05"
	AlmostRFC3339     = "2006-01-02T15:04:05"
)

func ConvertToLocation(layout string, value string, location string) (time.Time, error) {
	var t time.Time
	loc, err := time.LoadLocation(location)
	if err != nil {
		return t, err
	}
	t, err = time.Parse(layout, value)
	if err != nil {
		return t, err
	}
	return t.In(loc), nil
}

func IsWithinDays(utc time.Time, days int) bool {
	start := time.Now().UTC()
	end := time.Now().UTC().AddDate(0, 0, days)
	if utc.After(start) && utc.Before(end) {
		return true
	}
	return false
}

func DateSince(year int, month time.Month, day int, location *time.Location) string {
	start := time.Date(year, month, day, 0, 0, 0, 0, location)
	now := time.Now().In(location)
	since := now.Sub(start)
	return Humanize(since)
}

func Humanize(duration time.Duration) string {
	days := int64(duration.Hours() / 24)
	hours := int64(math.Mod(duration.Hours(), 24))
	minutes := int64(math.Mod(duration.Minutes(), 60))
	seconds := int64(math.Mod(duration.Seconds(), 60))
	chunks := []struct {
		singularName string
		amount       int64
	}{
		{"day", days},
		{"hour", hours},
		{"minute", minutes},
		{"second", seconds},
	}
	parts := []string{}
	for _, chunk := range chunks {
		switch chunk.amount {
		case 0:
			continue
		case 1:
			parts = append(parts, fmt.Sprintf("%d %s", chunk.amount, chunk.singularName))
		default:
			parts = append(parts, fmt.Sprintf("%d %ss", chunk.amount, chunk.singularName))
		}
	}
	return strings.Join(parts, " ")
}
