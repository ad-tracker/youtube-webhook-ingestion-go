package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func Init(level string, logFile string) error {
	var config zap.Config

	if logFile != "" {
		config = zap.NewProductionConfig()
		config.OutputPaths = []string{logFile, "stdout"}
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

func Sync() error {
	if Log != nil {
		return Log.Sync()
	}
	return nil
}
