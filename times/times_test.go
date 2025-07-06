package times

import (
	"testing"
	"time"
)

func TestConvertToLocation(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	value := "2023-07-15 14:30:00"
	location := "America/New_York"

	result, err := ConvertToLocation(layout, value, location)
	if err != nil {
		t.Fatalf("ConvertToLocation failed: %v", err)
	}

	expectedLoc, _ := time.LoadLocation(location)
	if result.Location().String() != expectedLoc.String() {
		t.Errorf("Expected location %s, got %s", expectedLoc.String(), result.Location().String())
	}

	// The function parses the time in UTC first, then converts to the target location
	// So we need to parse in UTC first and then convert
	parsed, _ := time.Parse(layout, value)
	expected := parsed.In(expectedLoc)
	if !result.Equal(expected) {
		t.Errorf("Expected time %v, got %v", expected, result)
	}
}

func TestConvertToLocationInvalidLocation(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	value := "2023-07-15 14:30:00"
	invalidLocation := "Invalid/Location"

	_, err := ConvertToLocation(layout, value, invalidLocation)
	if err == nil {
		t.Error("ConvertToLocation should fail with invalid location")
	}
}

func TestConvertToLocationInvalidTime(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	invalidValue := "not-a-time"
	location := "UTC"

	_, err := ConvertToLocation(layout, invalidValue, location)
	if err == nil {
		t.Error("ConvertToLocation should fail with invalid time value")
	}
}

func TestConvertToLocationUTC(t *testing.T) {
	layout := "2006-01-02T15:04:05Z"
	value := "2023-12-25T12:00:00Z"
	location := "UTC"

	result, err := ConvertToLocation(layout, value, location)
	if err != nil {
		t.Fatalf("ConvertToLocation failed: %v", err)
	}

	if result.Location().String() != "UTC" {
		t.Errorf("Expected UTC location, got %s", result.Location().String())
	}
}

func TestConvertToLocationDifferentTimezones(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	value := "2023-07-15 12:00:00"

	timezones := []string{
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Australia/Sydney",
		"UTC",
	}

	for _, tz := range timezones {
		result, err := ConvertToLocation(layout, value, tz)
		if err != nil {
			t.Errorf("ConvertToLocation failed for timezone %s: %v", tz, err)
			continue
		}

		expectedLoc, _ := time.LoadLocation(tz)
		if result.Location().String() != expectedLoc.String() {
			t.Errorf("Expected location %s, got %s", tz, result.Location().String())
		}
	}
}

func TestIsWithinDays(t *testing.T) {
	now := time.Now().UTC()

	// Test current time (should NOT be within range since it's not AFTER now)
	if IsWithinDays(now, 1) {
		t.Error("Current time should not be within future days range")
	}

	// Test future time within range
	futureTime := now.Add(12 * time.Hour)
	if !IsWithinDays(futureTime, 1) {
		t.Error("Time 12 hours from now should be within 1 day")
	}

	// Test past time (should not be within future days)
	pastTime := now.Add(-12 * time.Hour)
	if IsWithinDays(pastTime, 1) {
		t.Error("Past time should not be within future days")
	}

	// Test future time outside range
	farFutureTime := now.Add(48 * time.Hour)
	if IsWithinDays(farFutureTime, 1) {
		t.Error("Time 48 hours from now should not be within 1 day")
	}
}

func TestIsWithinDaysEdgeCases(t *testing.T) {
	now := time.Now().UTC()

	// Test with 0 days (should always be false for future times)
	futureTime := now.Add(1 * time.Hour)
	if IsWithinDays(futureTime, 0) {
		t.Error("No future time should be within 0 days")
	}

	// Test with negative days (edge case)
	if IsWithinDays(now, -1) {
		t.Error("Current time should not be within negative days")
	}
}

func TestDateSince(t *testing.T) {
	// Test with a known date in the past
	loc := time.UTC
	result := DateSince(2020, time.January, 1, loc)

	// Should return a non-empty string
	if result == "" {
		t.Error("DateSince should return a non-empty string")
	}

	// Should contain years (since 2020 is more than a year ago from any reasonable test time)
	// Note: We can't test exact values since this depends on when the test runs
	// But we can verify it returns a reasonable format
	expectedKeywords := []string{"day", "hour", "minute", "second"}
	foundKeyword := false
	for _, keyword := range expectedKeywords {
		if contains(result, keyword) {
			foundKeyword = true
			break
		}
	}
	if !foundKeyword {
		t.Errorf("DateSince result should contain time units, got: %s", result)
	}
}

func TestDateSinceRecentDate(t *testing.T) {
	// Test with yesterday
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	loc := now.Location()

	result := DateSince(yesterday.Year(), yesterday.Month(), yesterday.Day(), loc)

	if result == "" {
		t.Error("DateSince should return a non-empty string for yesterday")
	}

	// Should contain "day" for yesterday
	if !contains(result, "day") {
		t.Errorf("DateSince for yesterday should contain 'day', got: %s", result)
	}
}

func TestHumanize(t *testing.T) {
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{0, ""},
		{1 * time.Second, "1 second"},
		{2 * time.Second, "2 seconds"},
		{1 * time.Minute, "1 minute"},
		{2 * time.Minute, "2 minutes"},
		{1 * time.Hour, "1 hour"},
		{2 * time.Hour, "2 hours"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
		{90 * time.Second, "1 minute 30 seconds"},
		{3661 * time.Second, "1 hour 1 minute 1 second"},
		{25 * time.Hour, "1 day 1 hour"},
		{24*time.Hour + 61*time.Minute + 1*time.Second, "1 day 1 hour 1 minute 1 second"},
	}

	for _, tc := range testCases {
		result := Humanize(tc.duration)
		if result != tc.expected {
			t.Errorf("Humanize(%v) = %q, expected %q", tc.duration, result, tc.expected)
		}
	}
}

func TestHumanizeLargeValues(t *testing.T) {
	// Test with large durations
	largeDuration := time.Duration(365*24*2+5*24+3) * time.Hour // ~2 years, 5 days, 3 hours
	result := Humanize(largeDuration)

	if result == "" {
		t.Error("Humanize should handle large durations")
	}

	// Should contain days and hours
	if !contains(result, "day") {
		t.Errorf("Large duration should contain days, got: %s", result)
	}
	if !contains(result, "hour") {
		t.Errorf("Large duration should contain hours, got: %s", result)
	}
}

func TestHumanizeComplex(t *testing.T) {
	// Test with complex duration that includes all units
	complexDuration := 3*24*time.Hour + 4*time.Hour + 25*time.Minute + 45*time.Second
	result := Humanize(complexDuration)

	expected := "3 days 4 hours 25 minutes 45 seconds"
	if result != expected {
		t.Errorf("Humanize complex duration = %q, expected %q", result, expected)
	}
}

func TestConstantFormats(t *testing.T) {
	// Test that the time format constants are valid
	now := time.Now()

	formats := map[string]string{
		"FriendlyShort":     FriendlyShort,
		"Friendly":          Friendly,
		"DateDirectory":     DateDirectory,
		"DateTimeDirectory": DateTimeDirectory,
		"AlmostRFC3339":     AlmostRFC3339,
	}

	for name, format := range formats {
		formatted := now.Format(format)
		if formatted == "" {
			t.Errorf("Format %s should produce non-empty output", name)
		}

		// Try to parse it back (except for formats that might not be complete)
		if format == AlmostRFC3339 || format == DateDirectory || format == DateTimeDirectory {
			_, err := time.Parse(format, formatted)
			if err != nil {
				t.Errorf("Format %s should be parseable, got error: %v", name, err)
			}
		}
	}
}

func TestConvertToLocationWithDifferentLayouts(t *testing.T) {
	testCases := []struct {
		layout   string
		value    string
		location string
	}{
		{"2006-01-02", "2023-07-15", "UTC"},
		{"15:04:05", "14:30:25", "America/New_York"},
		{"Jan 2, 2006", "Jul 15, 2023", "Europe/London"},
		{"2006/01/02 15:04", "2023/07/15 14:30", "Asia/Tokyo"},
	}

	for _, tc := range testCases {
		result, err := ConvertToLocation(tc.layout, tc.value, tc.location)
		if err != nil {
			t.Errorf("ConvertToLocation(%s, %s, %s) failed: %v", tc.layout, tc.value, tc.location, err)
			continue
		}

		expectedLoc, _ := time.LoadLocation(tc.location)
		if result.Location().String() != expectedLoc.String() {
			t.Errorf("Expected location %s, got %s", tc.location, result.Location().String())
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
