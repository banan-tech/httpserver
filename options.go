package httpserver

import (
	"log/slog"
	"time"
)

const defaultShutdownTimeout = 10 * time.Second

type Option func(*Server)

func setDefaultLogger(server *Server) {
	WithLogger(slog.Default().WithGroup("httpserver"))(server)
}

// WithLogger configures error and server logger
func WithLogger(logger *slog.Logger) Option {
	return func(server *Server) {
		server.HTTPServer.ErrorLog = slog.NewLogLogger(logger.Handler(), slog.LevelError)
		server.log = logger
	}
}

// WithShutdownTimeout so server doesn't exceed the provided duration after receiving a stop signal
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(server *Server) {
		server.shutdownTimeout = timeout
	}
}

// WithServerTimeouts configures "net/http".Server ReadTimeout, WriteTimeout and IdleTimeout
func WithServerTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration) Option {
	return func(server *Server) {
		server.HTTPServer.ReadTimeout = readTimeout
		server.HTTPServer.WriteTimeout = writeTimeout
		server.HTTPServer.IdleTimeout = idleTimeout
	}
}
