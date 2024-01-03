package main

import (
	"context"
	"log/slog"
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

	select {
	case <-time.Tick(12 * time.Second):
		w.Write([]byte("Hello, World"))
	case <-r.Context().Done():
		w.Write([]byte("server stopped"))
	}

})

func nextRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	mw := logging(slog.Default())(handler)
	mw = tracing(nextRequestID)(mw)

	server := httpserver.New("", 3000, mw, httpserver.WithLogger(slog.Default()), customOption(), httpserver.WithShutdownTimeout(4*time.Second))

	if err := server.Run(); err != nil {
		slog.Error("server error", "error", err)
	}
}

func customOption() httpserver.Option {
	return func(s *httpserver.Server) {
		s.HTTPServer.ReadHeaderTimeout = time.Millisecond
	}
}

func logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.InfoContext(r.Context(), "",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent())
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
