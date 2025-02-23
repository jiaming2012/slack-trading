package logger

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

type LogrusLogger struct {
	logger *logrus.Logger
}

func NewLogrusLogger() *LogrusLogger {
	return &LogrusLogger{
		logger: logrus.New(),
	}
}

func (l *LogrusLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	return &newLogger
}

func (l *LogrusLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.logger.WithContext(ctx).Infof(msg, data...)
}

func (l *LogrusLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.logger.WithContext(ctx).Warnf(msg, data...)
}

func (l *LogrusLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.logger.WithContext(ctx).Errorf(msg, data...)
}

func (l *LogrusLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	if err != nil {
		l.logger.WithContext(ctx).WithFields(logrus.Fields{
			"elapsed": elapsed,
			"rows":    rows,
			"sql":     sql,
		}).Error(err)
	} else if elapsed > 200*time.Millisecond {
		l.logger.WithContext(ctx).WithFields(logrus.Fields{
			"elapsed": elapsed,
			"rows":    rows,
			"sql":     sql,
		}).Warn("SLOW SQL >= 200ms")
	} else {
		l.logger.WithContext(ctx).WithFields(logrus.Fields{
			"elapsed": elapsed,
			"rows":    rows,
			"sql":     sql,
		}).Info("SQL")
	}
}
