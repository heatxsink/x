package times

import "time"

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
