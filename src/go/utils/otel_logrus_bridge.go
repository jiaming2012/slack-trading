package utils

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel/log"
	otelglobal "go.opentelemetry.io/otel/log/global"
)

// OTelLogrusHook is a logrus hook that bridges log entries to the OTel Log SDK,
// which exports them via OTLP to Loki. Unlike otellogrus.Hook (which only adds
// events to the active span), this hook emits standalone log records that appear
// in Loki regardless of whether a trace is active.
type OTelLogrusHook struct {
	levels []logrus.Level
	logger log.Logger
}

// NewOTelLogrusHook returns a logrus hook that exports logs via the OTel Log SDK.
// Must be called after SetupOTelSDK() so the global LoggerProvider is set.
func NewOTelLogrusHook(levels ...logrus.Level) *OTelLogrusHook {
	if len(levels) == 0 {
		levels = []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
		}
	}
	return &OTelLogrusHook{
		levels: levels,
		logger: otelglobal.GetLoggerProvider().Logger("grodt"),
	}
}

func (h *OTelLogrusHook) Levels() []logrus.Level {
	return h.levels
}

func (h *OTelLogrusHook) Fire(entry *logrus.Entry) error {
	var record log.Record

	record.SetBody(log.StringValue(entry.Message))
	record.SetTimestamp(entry.Time)
	record.SetSeverity(logrusToOTelSeverity(entry.Level))
	record.SetSeverityText(entry.Level.String())

	attrs := make([]log.KeyValue, 0, len(entry.Data))
	for k, v := range entry.Data {
		attrs = append(attrs, logKeyValue(k, v))
	}
	record.AddAttributes(attrs...)

	ctx := entry.Context
	if ctx == nil {
		ctx = context.Background()
	}

	h.logger.Emit(ctx, record)
	return nil
}

func logrusToOTelSeverity(level logrus.Level) log.Severity {
	switch level {
	case logrus.TraceLevel:
		return log.SeverityTrace
	case logrus.DebugLevel:
		return log.SeverityDebug
	case logrus.InfoLevel:
		return log.SeverityInfo
	case logrus.WarnLevel:
		return log.SeverityWarn
	case logrus.ErrorLevel:
		return log.SeverityError
	case logrus.FatalLevel:
		return log.SeverityFatal
	case logrus.PanicLevel:
		return log.SeverityFatal4
	default:
		return log.SeverityInfo
	}
}

func logKeyValue(key string, val interface{}) log.KeyValue {
	switch v := val.(type) {
	case string:
		return log.String(key, v)
	case int:
		return log.Int(key, v)
	case int64:
		return log.Int64(key, v)
	case float64:
		return log.Float64(key, v)
	case bool:
		return log.Bool(key, v)
	case error:
		return log.String(key, v.Error())
	default:
		return log.String(key, fmt.Sprintf("%v", v))
	}
}
