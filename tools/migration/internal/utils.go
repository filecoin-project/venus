package internal

import (
	"time"
)

// NowString creates an ISO8601-like timestamp string
func NowString() string {
	now := time.Now()
	return now.Format("20060102-150405")
}
