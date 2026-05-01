package logger

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestGet(t *testing.T) {
	t.Run("stderr logger defaults to info", func(t *testing.T) {
		logger := Get("", true)
		if logger == nil {
			t.Error("Expected logger, got nil")
		}

		core := logger.Core()
		if core == nil {
			t.Error("Expected core, got nil")
		}

		if core.Enabled(zapcore.DebugLevel) {
			t.Error("Expected debug level to be disabled by default")
		}
		if !core.Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled by default")
		}
	})

	t.Run("stderr logger honors WithLevel", func(t *testing.T) {
		logger := Get("", true, WithLevel(zapcore.DebugLevel))
		if !logger.Core().Enabled(zapcore.DebugLevel) {
			t.Error("Expected debug level to be enabled when WithLevel(DebugLevel) is passed")
		}
	})

	t.Run("file logger defaults to info", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		logger := Get(logFile, false)
		if logger == nil {
			t.Error("Expected logger, got nil")
		}

		core := logger.Core()
		if core == nil {
			t.Error("Expected core, got nil")
		}

		if core.Enabled(zapcore.DebugLevel) {
			t.Error("Expected debug level to be disabled by default")
		}
		if !core.Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled by default")
		}

		logger.Info("test message")
		_ = logger.Sync()

		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			t.Error("Log file should have been created")
		}
	})
}

func TestFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_file.log")

	logger := File(logFile)
	if logger == nil {
		t.Error("Expected logger, got nil")
	}

	core := logger.Core()
	if core == nil {
		t.Error("Expected core, got nil")
	}

	if core.Enabled(zapcore.DebugLevel) {
		t.Error("Expected debug level to be disabled by default")
	}
	if !core.Enabled(zapcore.InfoLevel) {
		t.Error("Expected info level to be enabled by default")
	}

	logger.Info("test file message")
	_ = logger.Sync()

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should have been created")
	}
}

func TestStdErr(t *testing.T) {
	logger := StdErr()
	if logger == nil {
		t.Error("Expected logger, got nil")
	}

	core := logger.Core()
	if core == nil {
		t.Error("Expected core, got nil")
	}

	if core.Enabled(zapcore.DebugLevel) {
		t.Error("Expected debug level to be disabled by default")
	}
	if !core.Enabled(zapcore.InfoLevel) {
		t.Error("Expected info level to be enabled by default")
	}
}

func TestInitLoggerToStdErr(t *testing.T) {
	logger := initLoggerToStdErr(defaultConfig())
	if logger == nil {
		t.Error("Expected logger, got nil")
	}

	core := logger.Core()
	if core == nil {
		t.Error("Expected core, got nil")
	}

	if core.Enabled(zapcore.DebugLevel) {
		t.Error("Expected debug level to be disabled at default")
	}

	if !core.Enabled(zapcore.InfoLevel) {
		t.Error("Expected info level to be enabled")
	}

	if !core.Enabled(zapcore.WarnLevel) {
		t.Error("Expected warn level to be enabled")
	}

	if !core.Enabled(zapcore.ErrorLevel) {
		t.Error("Expected error level to be enabled")
	}
}

func TestInitLoggerToFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test_init.log")

	logger := initLoggerToFile(logFile, defaultConfig())
	if logger == nil {
		t.Error("Expected logger, got nil")
	}

	core := logger.Core()
	if core == nil {
		t.Error("Expected core, got nil")
	}

	if !core.Enabled(zapcore.InfoLevel) {
		t.Error("Expected info level to be enabled")
	}

	logger.Info("test init message")
	_ = logger.Sync()

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should have been created")
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test init message") {
		t.Error("Log file should contain the test message")
	}
}

func TestWithLogger(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	middleware := WithLogger(testLogger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := FromRequest(r)
		if logger == nil {
			t.Error("Expected logger from context, got nil")
		}

		logger.Info("test middleware message")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	logs := recorded.All()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].Message != "test middleware message" {
		t.Errorf("Expected 'test middleware message', got '%s'", logs[0].Message)
	}
}

func TestToContext(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	ctx := ToContext(context.Background(), testLogger)

	logger := FromContext(ctx)
	logger.Info("round-trip message")

	logs := recorded.All()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Message != "round-trip message" {
		t.Errorf("Expected 'round-trip message', got '%s'", logs[0].Message)
	}
}

func TestFromContext(t *testing.T) {
	t.Run("with logger in context", func(t *testing.T) {
		core, recorded := observer.New(zapcore.InfoLevel)
		testLogger := zap.New(core)

		ctx := context.WithValue(context.Background(), loggerKey, testLogger)

		logger := FromContext(ctx)
		if logger == nil {
			t.Fatal("Expected logger from context, got nil")
		}

		logger.Info("context message")

		logs := recorded.All()
		if len(logs) != 1 {
			t.Errorf("Expected 1 log entry, got %d", len(logs))
		}
		if logs[0].Message != "context message" {
			t.Errorf("Expected 'context message', got '%s'", logs[0].Message)
		}
	})

	t.Run("without logger in context", func(t *testing.T) {
		logger := FromContext(context.Background())
		if logger == nil {
			t.Fatal("Expected fallback logger, got nil")
		}
		if !logger.Core().Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled on fallback logger")
		}
	})

	t.Run("with wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), loggerKey, "not a logger")

		logger := FromContext(ctx)
		if logger == nil {
			t.Fatal("Expected fallback logger, got nil")
		}
		if !logger.Core().Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled on fallback logger")
		}
	})
}

func TestFromRequest(t *testing.T) {
	t.Run("with logger in context", func(t *testing.T) {
		core, recorded := observer.New(zapcore.InfoLevel)
		testLogger := zap.New(core)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := context.WithValue(req.Context(), loggerKey, testLogger)
		req = req.WithContext(ctx)

		logger := FromRequest(req)
		if logger == nil {
			t.Error("Expected logger from context, got nil")
		}

		logger.Info("test context message")

		logs := recorded.All()
		if len(logs) != 1 {
			t.Errorf("Expected 1 log entry, got %d", len(logs))
		}

		if logs[0].Message != "test context message" {
			t.Errorf("Expected 'test context message', got '%s'", logs[0].Message)
		}
	})

	t.Run("without logger in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		logger := FromRequest(req)
		if logger == nil {
			t.Error("Expected fallback logger, got nil")
		}

		core := logger.Core()
		if core == nil {
			t.Error("Expected core, got nil")
		}

		if !core.Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled on fallback logger")
		}
	})

	t.Run("with wrong type in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := context.WithValue(req.Context(), loggerKey, "not a logger")
		req = req.WithContext(ctx)

		logger := FromRequest(req)
		if logger == nil {
			t.Error("Expected fallback logger, got nil")
		}

		core := logger.Core()
		if core == nil {
			t.Error("Expected core, got nil")
		}

		if !core.Enabled(zapcore.InfoLevel) {
			t.Error("Expected info level to be enabled on fallback logger")
		}
	})
}

func TestLoggerIntegration(t *testing.T) {
	t.Run("stderr logger integration", func(t *testing.T) {
		old := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		done := make(chan string)
		go func() {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			done <- buf.String()
		}()

		logger := StdErr()
		logger.Info("integration test message")
		_ = logger.Sync()

		w.Close()
		os.Stderr = old
		output := <-done

		if !strings.Contains(output, "integration test message") {
			t.Error("Expected log message in stderr output")
		}

		if !strings.Contains(output, "INFO") {
			t.Error("Expected INFO level in stderr output")
		}
	})

	t.Run("file logger integration", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "integration.log")

		logger := File(logFile)
		logger.Info("file integration test")
		logger.Warn("warning message")
		logger.Error("error message")
		_ = logger.Sync()

		time.Sleep(100 * time.Millisecond)

		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Errorf("Failed to read log file: %v", err)
		}

		logContent := string(content)

		if !strings.Contains(logContent, "file integration test") {
			t.Error("Expected info message in log file")
		}

		if !strings.Contains(logContent, "warning message") {
			t.Error("Expected warning message in log file")
		}

		if !strings.Contains(logContent, "error message") {
			t.Error("Expected error message in log file")
		}

		if !strings.Contains(logContent, "INFO") {
			t.Error("Expected INFO level in log file")
		}

		if !strings.Contains(logContent, "WARN") {
			t.Error("Expected WARN level in log file")
		}

		if !strings.Contains(logContent, "ERROR") {
			t.Error("Expected ERROR level in log file")
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	middleware := WithLogger(testLogger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := FromRequest(r)
		logger.Info("first message")

		nestedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nestedLogger := FromRequest(r)
			nestedLogger.Info("nested message")
			w.WriteHeader(http.StatusOK)
		})

		nestedHandler.ServeHTTP(w, r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	logs := recorded.All()
	if len(logs) != 2 {
		t.Errorf("Expected 2 log entries, got %d", len(logs))
	}

	if logs[0].Message != "first message" {
		t.Errorf("Expected 'first message', got '%s'", logs[0].Message)
	}

	if logs[1].Message != "nested message" {
		t.Errorf("Expected 'nested message', got '%s'", logs[1].Message)
	}
}

func TestLoggerLevels(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "levels.log")

	logger := File(logFile, WithLevel(zapcore.DebugLevel))

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
	_ = logger.Sync()

	time.Sleep(100 * time.Millisecond)

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	if !strings.Contains(logContent, "debug message") {
		t.Error("Expected debug message in log file")
	}

	if !strings.Contains(logContent, "info message") {
		t.Error("Expected info message in log file")
	}

	if !strings.Contains(logContent, "warn message") {
		t.Error("Expected warn message in log file")
	}

	if !strings.Contains(logContent, "error message") {
		t.Error("Expected error message in log file")
	}
}

func TestDefaultLevelDropsDebug(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "default_level.log")

	logger := File(logFile)

	logger.Debug("debug message")
	logger.Info("info message")
	_ = logger.Sync()

	time.Sleep(100 * time.Millisecond)

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	if strings.Contains(logContent, "debug message") {
		t.Error("Debug message should be filtered out at default (Info) level")
	}

	if !strings.Contains(logContent, "info message") {
		t.Error("Info message should be present at default (Info) level")
	}
}
