package logger

import "go.uber.org/zap"

type CustomLogger struct {
	sugaredZapLogger *zap.SugaredLogger
}

func NewCustomLogger() *CustomLogger {
	return &CustomLogger{
		sugaredZapLogger: SugaredZapLogger, // 直接使用全局的sugaredZapLogger作为基础
	}
}

func (l *CustomLogger) With(args ...interface{}) *CustomLogger {
	l.sugaredZapLogger = l.sugaredZapLogger.With(args...)
	return l
}

func (l *CustomLogger) Debugf(template string, args ...interface{}) {
	l.sugaredZapLogger.Debugf(template, args...)
}

func (l *CustomLogger) Infof(template string, args ...interface{}) {
	l.sugaredZapLogger.Infof(template, args...)
}

func (l *CustomLogger) Warnf(template string, args ...interface{}) {
	l.sugaredZapLogger.Warnf(template, args...)
}

func (l *CustomLogger) Errorf(template string, args ...interface{}) {
	l.sugaredZapLogger.Errorf(template, args...)
}
