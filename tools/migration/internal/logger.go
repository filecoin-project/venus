package internal

import (
	"io"
	"log"
	"os"
)

// Logger logs migration events to disk and, if initialized with verbose,
// stdout too.
type Logger struct {
	closer io.WriteCloser
	logger *log.Logger
}

// NewLogger creates a new Logger.  All log writes go to f, the logging file.
// If the verbose flag is true all log writes also go to stdout.
func NewLogger(wc io.WriteCloser, verbose bool) *Logger {
	// by default just write to file
	var w io.Writer
	w = wc
	if verbose {
		w = io.MultiWriter(wc, os.Stdout)
	}
	return &Logger{
		closer: wc,
		logger: log.New(w, "[Filecoin Migration] ", log.LstdFlags),
	}
}

// Error logs an error to the logging output.
func (l *Logger) Error(err error) {
	if err == nil {
		return
	}
	l.logger.Printf("ERROR: %s", err.Error())
}

// Print logs a string to the logging output.
func (l *Logger) Print(msg string) {
	l.logger.Print(msg)
}

// Printf logs and formats a string to the logging output.
func (l *Logger) Printf(format string, v ...interface{}) {
	l.logger.Printf(format, v...)
}

// Close closes the logfile backing the Logger.
func (l *Logger) Close() error {
	return l.closer.Close()
}
