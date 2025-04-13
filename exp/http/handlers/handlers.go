package handlers

import (
	"compress/gzip"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"

	"github.com/gorilla/handlers"

	"github.com/rs/cors"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

type Handlers struct {
	Logger *log.Logger
	Debug  bool
}

func (h *Handlers) Patch(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	return h.Recover(Compress(Minify(h.CORSrs(mux, allowedOrigins))))
}

func (h *Handlers) PatchDebug(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	return h.Recover(h.Dump(h.CORSrs(mux, allowedOrigins)))
}

func (h *Handlers) Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				h.Logger.Println("handlers.Recover(): ", err)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("~~ Internal Server Error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) Dump(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			h.Logger.Println(err)
		}
		h.Logger.Println("handlers.Dump(): ", string(dump))
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) CORSrs(mux *http.ServeMux, allowedOrigins []string) http.Handler {
	allowedMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead}
	options := cors.Options{
		AllowedOrigins:       allowedOrigins,
		AllowedMethods:       allowedMethods,
		AllowedHeaders:       []string{"*"},
		AllowCredentials:     true,
		OptionsSuccessStatus: http.StatusOK,
	}
	if h.Debug {
		options.Logger = h.Logger
	}
	return cors.New(options).Handler(mux)
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
	m := minify.New()
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/css", css.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := m.ResponseWriter(w, r)
		defer mw.Close()
		next.ServeHTTP(mw, r)
	})
}

func Compress(next http.Handler) http.Handler {
	return handlers.CompressHandlerLevel(next, gzip.BestSpeed)
}
