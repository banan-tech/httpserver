package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/khaledez/httpserver"
)

const (
	requestIDKey = iota
)

var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World"))
})

func nextRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func main() {
	httpLogger := log.New(os.Stdout, "[http] ", log.Lmsgprefix|log.Ldate|log.Lmicroseconds)

	mw := logging(httpLogger)(handler)
	mw = tracing(nextRequestID)(mw)

	server := httpserver.New(3000, mw, httpserver.WithLogger(httpLogger), customOption())

	if err := server.Run(); err != nil {
		httpLogger.Fatal(err)
	}
}

func customOption() httpserver.Option {
	return func(s *httpserver.Server) {
		s.HTTPServer.ReadHeaderTimeout = time.Millisecond
	}
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
