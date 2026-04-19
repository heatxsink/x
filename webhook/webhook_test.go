package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type testPayload struct {
	Message string `json:"message"`
	ID      int    `json:"id"`
}

func TestSendJSON(t *testing.T) {
	payload := testPayload{Message: "test", ID: 123}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json, got %s", r.Header.Get("Accept"))
		}

		var received testPayload
		err := json.NewDecoder(r.Body).Decode(&received)
		if err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if received.Message != payload.Message || received.ID != payload.ID {
			t.Errorf("Expected %+v, got %+v", payload, received)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := SendJSON(server.URL, payload)
	if err != nil {
		t.Errorf("SendJSON failed: %v", err)
	}
}

func TestSendJSONWithClient(t *testing.T) {
	payload := testPayload{Message: "test with client", ID: 456}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	err := SendJSONWithClient(client, server.URL, payload)
	if err != nil {
		t.Errorf("SendJSONWithClient failed: %v", err)
	}
}

func TestSendJSONWithInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	invalidData := make(chan int)
	err := SendJSON(server.URL, invalidData)
	if err == nil {
		t.Error("Expected error for invalid JSON data, got nil")
	}

	if !strings.Contains(err.Error(), "json: unsupported type") {
		t.Errorf("Expected JSON marshal error, got: %v", err)
	}
}

func TestSendJSONWithHTTPError(t *testing.T) {
	payload := testPayload{Message: "error test", ID: 789}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	err := SendJSON(server.URL, payload)
	if err == nil {
		t.Error("Expected error for HTTP 400, got nil")
	}

	expectedErr := "HTTP status code: 400 HTTP body: Bad Request"
	if err.Error() != expectedErr {
		t.Errorf("Expected error: %s, got: %s", expectedErr, err.Error())
	}
}

func TestSendJSONWithServerError(t *testing.T) {
	payload := testPayload{Message: "server error test", ID: 500}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	err := SendJSON(server.URL, payload)
	if err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}

	expectedErr := "HTTP status code: 500 HTTP body: Internal Server Error"
	if err.Error() != expectedErr {
		t.Errorf("Expected error: %s, got: %s", expectedErr, err.Error())
	}
}

func TestSendWithContext(t *testing.T) {
	payload := testPayload{Message: "context test", ID: 101}
	headers := map[string]string{
		"Authorization": "Bearer token123",
		"X-Custom":      "custom-value",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("Expected X-Custom header, got %s", r.Header.Get("X-Custom"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	err := SendWithContext(ctx, client, server.URL, headers, payload)
	if err != nil {
		t.Errorf("SendWithContext failed: %v", err)
	}
}

func TestSendWithContextTimeout(t *testing.T) {
	payload := testPayload{Message: "timeout test", ID: 102}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := &http.Client{}
	err := SendWithContext(ctx, client, server.URL, nil, payload)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestSendWithContextCancellation(t *testing.T) {
	payload := testPayload{Message: "cancel test", ID: 103}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	client := &http.Client{}
	err := SendWithContext(ctx, client, server.URL, nil, payload)
	if err == nil {
		t.Error("Expected cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}
}

func TestSendWithContextAndRetry(t *testing.T) {
	payload := testPayload{Message: "retry test", ID: 104}
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Server Error"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	err := SendWithContextAndRetry(ctx, 3, 10*time.Millisecond, client, server.URL, nil, payload)
	if err != nil {
		t.Errorf("SendWithContextAndRetry failed: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestSendWithContextAndRetryExhausted(t *testing.T) {
	payload := testPayload{Message: "retry exhausted test", ID: 105}
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	err := SendWithContextAndRetry(ctx, 2, 10*time.Millisecond, client, server.URL, nil, payload)
	if err == nil {
		t.Error("Expected error after retry exhaustion, got nil")
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}

	if !strings.Contains(err.Error(), "HTTP status code: 500") {
		t.Errorf("Expected HTTP 500 error, got: %v", err)
	}
}

func TestSendWithContextAndRetryZeroRetries(t *testing.T) {
	payload := testPayload{Message: "zero retries test", ID: 106}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	err := SendWithContextAndRetry(ctx, 0, 10*time.Millisecond, client, server.URL, nil, payload)
	if err != nil {
		t.Errorf("Expected no error with zero retries, got: %v", err)
	}
}

func TestPostWithInvalidURL(t *testing.T) {
	client := &http.Client{}
	statusCode, content, err := post(client, "invalid-url", []byte("test"))

	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	if statusCode != -1 {
		t.Errorf("Expected status code -1, got %d", statusCode)
	}

	if content != nil {
		t.Errorf("Expected nil content, got %v", content)
	}
}

func TestPostWithContextInvalidURL(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}

	response, err := postWithContext(ctx, client, "invalid-url", nil, []byte("test"))

	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	if response != nil {
		_ = response.Body.Close()
		t.Errorf("Expected nil response, got %v", response)
	}

	if !strings.Contains(err.Error(), "unsupported protocol scheme") {
		t.Errorf("Expected protocol scheme error, got: %v", err)
	}
}

func TestSendJSONWithMalformedResponse(t *testing.T) {
	payload := testPayload{Message: "malformed response test", ID: 107}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()

	err := SendJSON(server.URL, payload)
	if err == nil {
		t.Error("Expected error for malformed response, got nil")
	}

	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Errorf("Expected unexpected EOF error, got: %v", err)
	}
}

func TestSendWithContextHeaders(t *testing.T) {
	payload := testPayload{Message: "headers test", ID: 108}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json, got %s", r.Header.Get("Accept"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	err := SendWithContext(ctx, client, server.URL, nil, payload)
	if err != nil {
		t.Errorf("SendWithContext failed: %v", err)
	}
}

func TestSendWithContextAndRetryWithTimeout(t *testing.T) {
	payload := testPayload{Message: "retry timeout test", ID: 109}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := &http.Client{}

	err := SendWithContextAndRetry(ctx, 3, 10*time.Millisecond, client, server.URL, nil, payload)
	if err == nil {
		t.Error("Expected timeout error during retry, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestSendWithContextAndRetry_CtxCancelDuringDelay(t *testing.T) {
	payload := testPayload{Message: "cancel during delay", ID: 201}

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel shortly after the first attempt completes, while the retry loop
	// is sleeping on the 100ms backoff. Without a ctx-aware backoff, the call
	// would continue sleeping the full delay.
	go func() {
		for attempts.Load() < 1 {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	err := SendWithContextAndRetry(ctx, 5, 100*time.Millisecond, client, server.URL, nil, payload)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected errors.Is(err, context.Canceled), got: %v", err)
	}
	// First attempt + a few ms before cancel + tiny wakeup slack. Must be
	// well under the single 100ms delay window, and far under 5*100ms.
	if elapsed > 80*time.Millisecond {
		t.Errorf("Expected prompt return after cancel, took %v", elapsed)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("Expected exactly 1 attempt before cancel, got %d", got)
	}
}

func TestSendWithContextAndRetry_CtxDeadlineDuringDelay(t *testing.T) {
	payload := testPayload{Message: "deadline during delay", ID: 202}

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	err := SendWithContextAndRetry(ctx, 5, 100*time.Millisecond, client, server.URL, nil, payload)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected errors.Is(err, context.DeadlineExceeded), got: %v", err)
	}
	// 5 retries * 100ms delay would be 500ms+. With a ctx-aware backoff we
	// should return near the 150ms deadline.
	if elapsed > 250*time.Millisecond {
		t.Errorf("Expected return near deadline (150ms), took %v", elapsed)
	}
}

func TestSendWithContextAndRetry_SucceedsAfterRetry(t *testing.T) {
	payload := testPayload{Message: "succeeds after retry", ID: 203}

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Server Error"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	delay := 20 * time.Millisecond
	start := time.Now()
	err := SendWithContextAndRetry(ctx, 5, delay, client, server.URL, nil, payload)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected success after one retry, got: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Errorf("Expected 2 attempts, got %d", got)
	}
	if elapsed < delay {
		t.Errorf("Expected at least one %v backoff between attempts, elapsed %v", delay, elapsed)
	}
}

func TestSendWithContextAndRetry_ExhaustsRetries(t *testing.T) {
	payload := testPayload{Message: "exhausts retries", ID: 204}

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	const retries = 4
	err := SendWithContextAndRetry(ctx, retries, 5*time.Millisecond, client, server.URL, nil, payload)
	if err == nil {
		t.Fatal("Expected error after exhausting retries, got nil")
	}
	if got := attempts.Load(); got != int32(retries) {
		t.Errorf("Expected %d attempts, got %d", retries, got)
	}

	// errors.Join wraps each attempt's error; Unwrap() []error exposes them.
	u, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Expected joined error with Unwrap() []error, got %T: %v", err, err)
	}
	joined := u.Unwrap()
	if len(joined) != retries {
		t.Errorf("Expected %d joined errors, got %d", retries, len(joined))
	}
	for i, e := range joined {
		if !strings.Contains(e.Error(), "HTTP status code: 500") {
			t.Errorf("joined[%d] = %v, want 500 status", i, e)
		}
	}
}
