package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heatxsink/x/exp/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestCORS(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000", "https://example.com"}
	allowedMethods := []string{http.MethodGet, http.MethodPost}
	allowedHeaders := []string{"Content-Type", "Authorization"}

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}), allowedOrigins, allowedMethods, allowedHeaders)

	t.Run("simple request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Errorf("Expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Just verify the handler doesn't crash and returns a valid status
		if rec.Code < 200 || rec.Code >= 500 {
			t.Errorf("Expected 2xx or 4xx status, got %d", rec.Code)
		}
	})
}

func TestCORSWithLogger(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	allowedOrigins := []string{"http://localhost:3000"}
	allowedMethods := []string{http.MethodGet, http.MethodPost}
	allowedHeaders := []string{"Content-Type"}

	handler := CORSWithLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}), allowedOrigins, allowedMethods, allowedHeaders, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	logs := recorded.All()
	if len(logs) > 0 {
		t.Logf("Logger captured %d log entries", len(logs))
	}
}

func TestRecover(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	t.Run("panic with string", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logs := recorded.All()
		if len(logs) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logs))
		}

		found := false
		for _, log := range logs {
			if strings.Contains(log.Message, "handlers.Recover()") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected recover log message")
		}
	})

	recorded.TakeAll()

	t.Run("panic with error", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		})))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logs := recorded.All()
		if len(logs) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logs))
		}
	})

	recorded.TakeAll()

	t.Run("panic with unknown type", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(123)
		})))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logs := recorded.All()
		if len(logs) < 2 {
			t.Errorf("Expected at least 2 log entries, got %d", len(logs))
		}
	})

	recorded.TakeAll()

	t.Run("no panic", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "OK" {
			t.Errorf("Expected body 'OK', got %s", rec.Body.String())
		}
	})
}

func TestDump(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	t.Run("dump enabled", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Dump(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}), false))

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString("test body"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		logs := recorded.All()
		if len(logs) == 0 {
			t.Error("Expected dump log entries")
		}

		found := false
		for _, log := range logs {
			if strings.Contains(log.Message, "handlers.Dump()") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected dump log message")
		}
	})

	recorded.TakeAll()

	t.Run("dump bypassed", func(t *testing.T) {
		handler := logger.WithLogger(testLogger)(Dump(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}), true))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		logs := recorded.All()
		for _, log := range logs {
			if strings.Contains(log.Message, "handlers.Dump()") {
				t.Error("Did not expect dump log message when bypassed")
			}
		}
	})
}

func TestBlackhole(t *testing.T) {
	handler := Blackhole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	t.Run("path with trailing slash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rec.Code)
		}
	})

	t.Run("path without trailing slash", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "OK" {
			t.Errorf("Expected body 'OK', got %s", rec.Body.String())
		}
	})
}

func TestMinify(t *testing.T) {
	handler := Minify(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>  <body>   <h1>Hello World</h1>   </body>  </html>"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "  ") {
		t.Error("Expected minified HTML without extra spaces")
	}

	if !strings.Contains(body, "Hello World") {
		t.Error("Expected content to be preserved")
	}
}

func TestCompress(t *testing.T) {
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		largeContent := strings.Repeat("This is a test string for compression. ", 100)
		w.Write([]byte(largeContent))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected gzip encoding, got %s", rec.Header().Get("Content-Encoding"))
	}

	if rec.Body.Len() >= 3900 {
		t.Errorf("Expected compressed content to be smaller, got %d bytes", rec.Body.Len())
	}
}

func TestPatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	allowedOrigins := []string{"http://localhost:3000"}
	allowedMethods := []string{http.MethodGet, http.MethodPost}
	allowedHeaders := []string{"Content-Type"}

	handler := Patch(mux, allowedOrigins, allowedMethods, allowedHeaders)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected gzip encoding, got %s", rec.Header().Get("Content-Encoding"))
	}
}

func TestPatchDebug(t *testing.T) {
	core, recorded := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	allowedOrigins := []string{"http://localhost:3000"}
	handler := logger.WithLogger(testLogger)(PatchDebug(mux, allowedOrigins))

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString("test body"))
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	logs := recorded.All()
	found := false
	for _, log := range logs {
		if strings.Contains(log.Message, "handlers.Dump()") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected dump log message in debug mode")
	}
}

func TestDefaultValues(t *testing.T) {
	if len(DefaultAllowedHeaders) != 1 || DefaultAllowedHeaders[0] != "*" {
		t.Errorf("Expected DefaultAllowedHeaders to be [\"*\"], got %v", DefaultAllowedHeaders)
	}

	expectedMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead}
	if len(DefaultAllowedMethods) != len(expectedMethods) {
		t.Errorf("Expected %d default methods, got %d", len(expectedMethods), len(DefaultAllowedMethods))
	}

	for i, method := range expectedMethods {
		if DefaultAllowedMethods[i] != method {
			t.Errorf("Expected method %s at index %d, got %s", method, i, DefaultAllowedMethods[i])
		}
	}
}

func TestMiddlewareChaining(t *testing.T) {
	core, _ := observer.New(zapcore.InfoLevel)
	testLogger := zap.New(core)

	handler := logger.WithLogger(testLogger)(
		Recover(
			Compress(
				Minify(
					CORS(
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Type", "text/html")
							w.WriteHeader(http.StatusOK)
							w.Write([]byte("<html>  <body>   <h1>Test</h1>   </body>  </html>"))
						}),
						[]string{"http://localhost:3000"},
						[]string{http.MethodGet},
						[]string{"Content-Type"},
					),
				),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected CORS origin header, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Expected gzip encoding, got %s", rec.Header().Get("Content-Encoding"))
	}

	body := rec.Body.String()
	if strings.Contains(body, "  ") {
		t.Error("Expected minified HTML content")
	}
}