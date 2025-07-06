package logger

import (
	"context"
	"net/http"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type contextKey string

const loggerKey contextKey = "zap-logger"

func WithLogger(slogger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), loggerKey, slogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func FromRequest(r *http.Request) *zap.Logger {
	if logger, ok := r.Context().Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return initLoggerToStdErr()
}

func Get(filename string, fromStdError bool) *zap.Logger {
	if fromStdError {
		return initLoggerToStdErr()
	}
	return initLoggerToFile(filename)
}

func File(filename string) *zap.Logger {
	return initLoggerToFile(filename)
}

func StdErr() *zap.Logger {
	return initLoggerToStdErr()
}

func initLoggerToStdErr() *zap.Logger {
	stderrSyncer := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, stderrSyncer, zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller())
}

func initLoggerToFile(filename string) *zap.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50, //mb
		MaxBackups: 10,
		MaxAge:     30, //days
		Compress:   false,
	}
	writerSyncer := zapcore.AddSync(lumberJackLogger)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, writerSyncer, zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller())
}
