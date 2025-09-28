package utils

import (
	"fmt"
	"time"
)

// LoadJST loads the JST (Japan Standard Time) timezone
// Returns the timezone location or an error if it cannot be loaded
func LoadJST() (*time.Location, error) {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return nil, fmt.Errorf("failed to load JST timezone: %w", err)
	}
	return jst, nil
}

// NowInJST returns the current time in JST timezone
// If JST cannot be loaded, returns current time in UTC
func NowInJST() time.Time {
	jst, err := LoadJST()
	if err != nil {
		// Fallback to UTC if JST cannot be loaded
		return time.Now().UTC()
	}
	return time.Now().In(jst)
}

// TodayInJST returns today's date string in JST timezone (YYYY-MM-DD format)
// If JST cannot be loaded, returns today's date in UTC
func TodayInJST() string {
	return NowInJST().Format("2006-01-02")
}

// ToJST converts a time.Time to JST timezone
// If JST cannot be loaded, returns the time in UTC
func ToJST(t time.Time) time.Time {
	jst, err := LoadJST()
	if err != nil {
		return t.UTC()
	}
	return t.In(jst)
}
