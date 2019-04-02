package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"time"

	oldlogging "github.com/whyrusleeping/go-logging"
)

// JSONFormatter implements go-logging Formatter for JSON encoded logs
type JSONFormatter struct {
}

type logRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	System    string    `json:"system"`
	Message   string    `json:"message"`
	File      string    `json:"file"`
}

// Format implements go-logging Formatter
func (jf *JSONFormatter) Format(calldepth int, r *oldlogging.Record, w io.Writer) error {
	var fileLine string
	if calldepth > 0 {
		_, file, line, ok := runtime.Caller(calldepth + 1)
		if !ok {
			fileLine = "???:0"
		} else {
			fileLine = fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}
	}
	lr := &logRecord{
		Timestamp: r.Time,
		Level:     r.Level.String(),
		System:    r.Module,
		Message:   r.Message(),
		File:      fileLine,
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(lr)
}
