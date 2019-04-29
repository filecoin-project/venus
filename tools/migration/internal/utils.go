package internal

import (
	"os/user"
	"strings"
	"time"
)

// NowString creates an ISO8601-like timestamp string
func NowString() string {
	now := time.Now()
	return now.Format("20060102-150405")
}

// expandHomedir replaces an initial tilde in a dirname to the home dir.
// if there is no initial ~ , it returns dirname.
func ExpandHomedir(dirname string) string {
	if strings.LastIndex(dirname, "~") != 0 {
		return dirname
	}

	usr, _ := user.Current()
	home := usr.HomeDir
	return strings.Replace(dirname, "~", home, 1)
}
