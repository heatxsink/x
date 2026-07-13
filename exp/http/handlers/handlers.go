package handlers

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/heatxsink/x/exp/logger"
	"go.uber.org/zap"

	"github.com/rs/cors"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

var DefaultAllowedHeaders = []string{"*"}
var DefaultAllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead}

type corsLoggerAdapter struct {
	logger *zap.SugaredLogger
}

func (c *corsLoggerAdapter) Printf(format string, v ...interface{}) {
	c.logger.Infof(format, v...)
}

func CORS(next http.Handler, allowedOrigins []string, allowedMethods []string, allowedHeaders []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		options := cors.Options{
			AllowedOrigins:       allowedOrigins,
			AllowedMethods:       allowedMethods,
			AllowedHeaders:       allowedHeaders,
			AllowCredentials:     true,
			OptionsSuccessStatus: http.StatusOK,
		}
		cors.New(options).Handler(next).ServeHTTP(w, r)
	})
}

func CORSWithLogger(next http.Handler, allowedOrigins []string, allowedMethods []string, allowedHeaders []string, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		options := cors.Options{
			AllowedOrigins:       allowedOrigins,
			AllowedMethods:       allowedMethods,
			AllowedHeaders:       allowedHeaders,
			AllowCredentials:     true,
			OptionsSuccessStatus: http.StatusOK,
			Logger:               &corsLoggerAdapter{logger: logger.Sugar()},
		}
		cors.New(options).Handler(next).ServeHTTP(w, r)
	})
}

// recoverRecorder tracks whether the underlying ResponseWriter has had a
// status code sent. Used by Recover to decide whether it can safely emit a
// 500 after a panic (only possible when headers haven't already flushed).
type recoverRecorder struct {
	http.ResponseWriter
	wrote bool
}

// Compile-time interface assertions: catch a future refactor that drops
// Flush / Hijack / Push before it becomes a runtime SSE / WebSocket / push
// regression in downstreams.
var (
	_ http.Flusher  = (*recoverRecorder)(nil)
	_ http.Hijacker = (*recoverRecorder)(nil)
	_ http.Pusher   = (*recoverRecorder)(nil)
)

func (r *recoverRecorder) WriteHeader(code int) {
	r.wrote = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *recoverRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

// Flush forwards to the wrapped writer if it implements http.Flusher, so
// streaming handlers wrapped by Recover keep working — matches the pattern
// used by responseRecorder and minifyResponseWriter.
func (r *recoverRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the wrapped writer if it implements http.Hijacker.
func (r *recoverRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push forwards to the wrapped writer if it implements http.Pusher.
func (r *recoverRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &recoverRecorder{ResponseWriter: w}
		defer func() {
			if rr := recover(); rr != nil {
				slogger := logger.FromRequest(r)
				slogger.Info("handlers.Recover()", zap.Any("rr := recover()", rr))
				switch x := rr.(type) {
				case string:
					err := errors.New(x)
					slogger.Error("handlers.Recover(): ", zap.Error(err))
				case error:
					slogger.Error("handlers.Recover(): ", zap.Error(x))
				default:
					err := errors.New("unknown panic")
					slogger.Error("handlers.Recover(): ", zap.Error(err))
				}
				// If the handler hadn't emitted anything before panicking,
				// synthesize a 500 so the client sees an error rather than
				// the default "200 OK with empty body." Headers already
				// flushed can't be overwritten (silent no-op in that case).
				if !rec.wrote {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

func Dump(next http.Handler, bypass bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bypass {
			next.ServeHTTP(w, r)
			return
		}
		slogger := logger.FromRequest(r)
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			slogger.Error("httputil.DumpRequest()", zap.Error(err))
		}
		slogger.Info("handlers.Dump()", zap.String("dump", string(dump)))
		next.ServeHTTP(w, r)
	})
}

func Blackhole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var defaultMinifier = func() *minify.M {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	return m
}()

// minifyResponseWriter wraps the tdewolff/minify ResponseWriter so an
// SSE (or any other flushing) handler downstream can still find an
// http.Flusher via type assertion.
//
// The upstream *minify.ResponseWriter embeds http.ResponseWriter but
// doesn't proxy the optional Flush/Hijack/Push interfaces. Handlers
// that do `w.(http.Flusher)` inside a Minify-wrapped chain get
// ok=false and typically 500 with "SSE not supported" (or fail to
// upgrade to WebSocket for Hijacker). This wrapper forwards each
// optional interface when the outer writer supports it, mirroring
// the pattern already used by responseRecorder further down this
// file.
type minifyResponseWriter struct {
	http.ResponseWriter // the tdewolff wrapper — Write / WriteHeader / Header pass through
	outer               http.ResponseWriter
}

// Flush forwards to the outer writer's Flush() when it implements
// http.Flusher. Silent no-op otherwise, matching the standard
// library's behaviour on writers that don't support flushing.
//
// Note: for minifiable content types (text/html, text/css, JS),
// tdewolff/minify buffers until Close(), so a mid-stream Flush()
// won't push those bytes to the client — outer.Flush() runs on
// whatever's already been forwarded through. For streaming
// pass-through types like text/event-stream (SSE) tdewolff doesn't
// buffer and this works as expected. Don't try to stream
// server-rendered HTML through Minify; use a bypass instead.
func (m *minifyResponseWriter) Flush() {
	if f, ok := m.outer.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the outer writer's Hijack() when it implements
// http.Hijacker. Errors with a clear message otherwise so WebSocket
// upgrade attempts fail loud instead of pretending to succeed.
func (m *minifyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := m.outer.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("handlers: outer ResponseWriter does not implement http.Hijacker")
}

// Push forwards to the outer writer's Push() when it implements
// http.Pusher. Returns http.ErrNotSupported otherwise, which is the
// documented signal callers should already handle.
func (m *minifyResponseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := m.outer.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func Minify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := defaultMinifier.ResponseWriter(w, r)
		defer mw.Close()
		next.ServeHTTP(&minifyResponseWriter{ResponseWriter: mw, outer: w}, r)
	})
}

func Compress(next http.Handler) http.Handler {
	return handlers.CompressHandlerLevel(next, gzip.BestSpeed)
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush forwards to the wrapped writer if it implements http.Flusher,
// so SSE handlers wrapped by AccessLog can still flush events to the
// client. Without this method, a type assertion `w.(http.Flusher)` on
// the recorder fails -- embedding the http.ResponseWriter interface
// only promotes methods that exist on that interface, and Flush lives
// on a separate http.Flusher interface. Silent no-op when the
// underlying writer doesn't support flushing (HTTP/1.0, test
// recorders), matching the standard library's behaviour.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, err := net.SplitHostPort(strings.TrimSpace(strings.Split(xff, ",")[0])); err == nil {
			return ip
		}
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slogger := logger.FromRequest(r)
		slogger.Info("http",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rec.status),
			zap.Int("bytes", rec.bytes),
			zap.String("ip", clientIP(r)),
			zap.String("ua", r.UserAgent()),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// Patch composes the full Recover→AccessLog→Compress→Minify→CORS chain over
// a concrete *http.ServeMux. Use PatchNoCORS for the same chain without CORS,
// or when you need to pass an http.Handler (not *http.ServeMux) — e.g. to
// compose with SkipPaths for streaming endpoints.
func Patch(mux *http.ServeMux, allowedOrigins []string, allowedMethods []string, allowedHeaders []string) http.Handler {
	return Recover(AccessLog(Compress(Minify(CORS(mux, allowedOrigins, allowedMethods, allowedHeaders)))))
}

func PatchDebug(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	return Recover(AccessLog(Dump(CORS(mux, allowedOrigins, DefaultAllowedMethods, DefaultAllowedHeaders), false)))
}

// PatchNoCORS is the same Recover→AccessLog→Compress→Minify chain as Patch
// but without the CORS layer. For single-user / same-origin applications
// where CORS response headers aren't needed.
//
// Accepts http.Handler (not *http.ServeMux specifically) so callers can
// pre-compose their own routing, e.g. combine with SkipPaths for streaming.
func PatchNoCORS(mux http.Handler) http.Handler {
	return Recover(AccessLog(Compress(Minify(mux))))
}

// SkipPaths returns a handler that routes requests whose r.URL.Path exactly
// matches one of paths to raw, and everything else to chain. Common pattern:
// bypass Compress/Minify for Server-Sent Events or WebSocket endpoints that
// can't tolerate buffering middleware.
//
// Match is exact string equality on r.URL.Path only:
//   - No prefix matching. "/events/42" is NOT matched by "/events".
//   - No trailing-slash normalization. "/events/" is a different key.
//   - Not method-aware. Under Go 1.22+ mux patterns like "GET /foo" and
//     "POST /foo", SkipPaths(_, _, "/foo") bypasses both — matching happens
//     before the mux resolves the method. Callers wanting per-method bypass
//     compose their own.
//
// Panics if chain or raw is nil — catches a programmer error at construction
// time rather than on the first request.
//
// Example:
//
//	Handler: SkipPaths(PatchNoCORS(mux), mux, "/api/1/events", "/ws")
func SkipPaths(chain http.Handler, raw http.Handler, paths ...string) http.Handler {
	if chain == nil {
		panic("handlers.SkipPaths: chain handler is nil")
	}
	if raw == nil {
		panic("handlers.SkipPaths: raw handler is nil")
	}
	if len(paths) == 0 {
		return chain
	}
	skip := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		skip[p] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := skip[r.URL.Path]; ok {
			raw.ServeHTTP(w, r)
			return
		}
		chain.ServeHTTP(w, r)
	})
}
