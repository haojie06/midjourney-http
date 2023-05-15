package logger

import (
	"go.uber.org/zap"
)

var (
	ZapLogger        *zap.Logger
	SugaredZapLogger *zap.SugaredLogger
)

func init() {
	ZapLogger, _ = zap.NewDevelopment(zap.AddCaller(), zap.AddCallerSkip(1))
	SugaredZapLogger = ZapLogger.Sugar()
}

func Debug(msg string, fields ...zap.Field) {
	ZapLogger.Debug(msg, fields...)
}

func Debugf(template string, args ...interface{}) {
	SugaredZapLogger.Debugf(template, args...)
}

func Info(msg string, fields ...zap.Field) {
	ZapLogger.Info(msg, fields...)
}

func Infof(template string, args ...interface{}) {
	SugaredZapLogger.Infof(template, args...)
}

func Warn(msg string, fields ...zap.Field) {
	ZapLogger.Warn(msg, fields...)
}

func Warnf(template string, args ...interface{}) {
	SugaredZapLogger.Warnf(template, args...)
}

func Error(msg string, fields ...zap.Field) {
	ZapLogger.Error(msg, fields...)
}

func Errorf(template string, args ...interface{}) {
	SugaredZapLogger.Errorf(template, args...)
}
