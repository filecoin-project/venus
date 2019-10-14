package journal

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapJournal returns a Journal backed by a zap logger. ZapJournal writes entries as ndjson to
// file at `filepath`.
func NewZapJournal(filepath string) (Journal, error) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = "json"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.EncoderConfig.LevelKey = ""
	zapCfg.EncoderConfig.CallerKey = ""
	zapCfg.EncoderConfig.MessageKey = "_event"
	zapCfg.EncoderConfig.NameKey = "_topic"
	zapCfg.OutputPaths = []string{filepath}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	global, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}

	return &ZapJournal{global}, nil
}

// ZapJournal implemented the Journal interface.
type ZapJournal struct {
	logger *zap.Logger
}

// Topic returns a Writer that records events for a topic.
func (zj *ZapJournal) Topic(topic string) Writer {
	return &ZapWriter{
		logger: zj.logger.Sugar().Named(topic),
		topic:  topic,
	}
}

// ZapWriter implements the Journal Write interface and is backed by a zap logger.
type ZapWriter struct {
	logger *zap.SugaredLogger
	topic  string
}

// Record records an operation and its metadata to a Journal accepting variadic key-value
// pairs.
func (zw *ZapWriter) Write(event string, kvs ...interface{}) {
	zw.logger.Infow(event, kvs...)
}
