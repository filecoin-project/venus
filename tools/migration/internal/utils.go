package internal

import (
	"os"
	"strings"
	"time"
)

// NowString creates an ISO8601-like timestamp string
func NowString() string {
	now := time.Now()
	return now.Format("2006-01-02_150405")
}

// expandHomedir replaces an initial tilde in a dirname to the actual value of HOME.
// if there is no initial ~ , it returns dirname.
func ExpandHomedir(dirname string) string {
	if strings.LastIndex(dirname, "~") != 0 {
		return dirname
	}
	// This counts on $HOME being set, just as FSRepo does
	home := os.Getenv("HOME")
	return strings.Replace(dirname, "~", home, 1)
}
