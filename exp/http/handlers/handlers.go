package handlers

import (
	"compress/gzip"
	"errors"
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

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			}
		}()
		next.ServeHTTP(w, r)
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

func Minify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := defaultMinifier.ResponseWriter(w, r)
		defer mw.Close()
		next.ServeHTTP(mw, r)
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

func Patch(mux *http.ServeMux, allowedOrigins []string, allowedMethods []string, allowedHeaders []string) http.Handler {
	return Recover(AccessLog(Compress(Minify(CORS(mux, allowedOrigins, allowedMethods, allowedHeaders)))))
}

func PatchDebug(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	return Recover(AccessLog(Dump(CORS(mux, allowedOrigins, DefaultAllowedMethods, DefaultAllowedHeaders), false)))
}
