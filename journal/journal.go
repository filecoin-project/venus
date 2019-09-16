package journal

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	journalPrefix = "journal"
	genericTopic  = "generic"
)

// Journal hold journal system config
type Journal struct {
	path    string
	generic bool
	logger  map[string]*zap.SugaredLogger
}

var journal *Journal

// InitJournal init journal dir
func InitJournal(repoDir string, generic bool) error {
	journalDir := filepath.Join(repoDir, journalPrefix)
	err := ensureWritableDirectory(journalDir)
	if err != nil {
		return err
	}
	logger := make(map[string]*zap.SugaredLogger)
	if generic {
		generalLogger, err := newLogger(genericTopic)
		if err != nil {
			return err
		}
		logger[genericTopic] = generalLogger
	}

	journal = &Journal{
		path:    journalDir,
		generic: generic,
		logger:  logger,
	}
	return nil
}

// newLogger new journal logger
func newLogger(topic string) (*zap.SugaredLogger, error) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = "json"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.EncoderConfig.LevelKey = ""
	zapCfg.EncoderConfig.CallerKey = ""
	zapCfg.EncoderConfig.MessageKey = "message"
	journalFileName := filepath.Join(journal.path, genJournalFileName(topic))
	zapCfg.OutputPaths = []string{journalFileName}

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

// Record persistent journal for event recording
func Record(topic string, msg string, keysAndValues ...interface{}) {
	if journal == nil {
		return
	}
	if journal.generic {
		kvInterface := make([]interface{}, len(keysAndValues)+2)
		kvInterface[0] = "topic"
		kvInterface[1] = topic
		for i, v := range keysAndValues {
			kvInterface[i+2] = v
		}
		journal.logger[genericTopic].Infow(msg, kvInterface...)
	} else {
		var err error
		logger, found := journal.logger[topic]
		if !found {
			logger, err = newLogger(topic)
			if err != nil {
				return
			}
			journal.logger[topic] = logger
		}
		logger.Infow(msg, keysAndValues...)
	}
}

// Ensures that path points to a read/writable directory, creating it if necessary.
func ensureWritableDirectory(path string) error {
	// Attempt to create the requested directory, accepting that something might already be there.
	err := os.Mkdir(path, 0775)

	if err == nil {
		return nil // Skip the checks below, we just created it.
	} else if !os.IsExist(err) {
		return errors.Wrapf(err, "failed to create directory %s", path)
	}

	// Inspect existing directory.
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed to stat path \"%s\"", path)
	}
	if !stat.IsDir() {
		return errors.Errorf("%s is not a directory", path)
	}
	if (stat.Mode() & 0600) != 0600 {
		return errors.Errorf("insufficient permissions for path %s, got %04o need %04o", path, stat.Mode(), 0600)
	}
	return nil
}

func genJournalFileName(topic string) string {
	if path.Ext(topic) == ".json" {
		return topic
	}
	return fmt.Sprintf("%s.json", topic)
}
