package handlers

import (
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"

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

func Minify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := minify.New()
		m.AddFunc("text/html", html.Minify)
		m.AddFunc("text/css", css.Minify)
		m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
		mw := m.ResponseWriter(w, r)
		defer mw.Close()
		next.ServeHTTP(mw, r)
	})
}

func Compress(next http.Handler) http.Handler {
	return handlers.CompressHandlerLevel(next, gzip.BestSpeed)
}

func Patch(mux *http.ServeMux, allowedOrigins []string, allowedMethods []string, allowedHeaders []string) http.Handler {
	return Recover(Compress(Minify(CORS(mux, allowedOrigins, allowedMethods, allowedHeaders))))
}

func PatchDebug(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	return Recover(Dump(CORS(mux, allowedOrigins, DefaultAllowedMethods, DefaultAllowedHeaders), false))
}
