package internal

import (
	"time"
)

// NowString creates a timestamp string
func NowString() string {
	now := time.Now()
	return now.Format("20060102-150405")
}
