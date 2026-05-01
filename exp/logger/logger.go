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

// Option configures a logger constructed by Get, File, or StdErr.
type Option func(*config)

type config struct {
	level zapcore.LevelEnabler
}

func defaultConfig() *config {
	return &config{level: zapcore.InfoLevel}
}

func resolve(opts []Option) *config {
	c := defaultConfig()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithLevel sets the minimum level the logger will emit. Defaults to InfoLevel.
func WithLevel(l zapcore.LevelEnabler) Option {
	return func(c *config) { c.level = l }
}

func WithLogger(slogger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ToContext(r.Context(), slogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ToContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return logger
	}
	return initLoggerToStdErr(defaultConfig())
}

func FromRequest(r *http.Request) *zap.Logger {
	return FromContext(r.Context())
}

func Get(filename string, fromStdError bool, opts ...Option) *zap.Logger {
	c := resolve(opts)
	if fromStdError {
		return initLoggerToStdErr(c)
	}
	return initLoggerToFile(filename, c)
}

func File(filename string, opts ...Option) *zap.Logger {
	return initLoggerToFile(filename, resolve(opts))
}

func StdErr(opts ...Option) *zap.Logger {
	return initLoggerToStdErr(resolve(opts))
}

func initLoggerToStdErr(c *config) *zap.Logger {
	stderrSyncer := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, stderrSyncer, c.level)
	return zap.New(core, zap.AddCaller())
}

func initLoggerToFile(filename string, c *config) *zap.Logger {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50, // mb
		MaxBackups: 10,
		MaxAge:     30, // days
		Compress:   false,
	}
	writerSyncer := zapcore.AddSync(lumberJackLogger)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, writerSyncer, c.level)
	return zap.New(core, zap.AddCaller())
}
