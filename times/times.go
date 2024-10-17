package times

import (
	"strings"
	"time"
)

const (
	FriendlyShort = "Mon, Jan 02 15:04 MST"
	Friendly      = "Mon, Jan 02, 2006 at 15:04 MST"
)

func ConvertToTimezone(timezone string, ts string) (time.Time, error) {
	var t time.Time
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return t, err
	}
	t, err = time.Parse(time.RFC3339, ts)
	if err != nil {
		return t, err
	}
	return t.In(loc), nil
}

func IsWithinDays(t time.Time, days int) bool {
	start := time.Now()
	end := time.Now().AddDate(0, 0, days)
	if t.After(start) && t.Before(end) {
		return true
	}
	return false
}

func FilenameTimestamp(t time.Time) string {
	ts := t.Format(time.RFC3339)
	tsScrub := strings.Replace(ts, ":", "", -1)
	tsScrub = strings.Replace(tsScrub, "-", "", -1)
	return tsScrub
}
