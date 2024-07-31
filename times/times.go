package times

import "time"

var FriendlyShort = "Mon, Jan 02 15:04 MST"
var FriendlyShortWithYear = "Mon, Jan 02, 2006 at 15:04 MST"

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
