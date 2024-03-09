package httpserver

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

const defaultShutdownTimeout = 10 * time.Second

var (
	DefaultLoggerProduction = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	DefaultLoggerDevelopment = slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.TimeOnly,
	}))
)

type Option func(*Server)

func setDefaultLogger(server *Server) {
	var logger *slog.Logger

	switch server.mode {

	case ModeProduction:
		logger = DefaultLoggerProduction
	default:
		logger = DefaultLoggerDevelopment
	}

	WithLogger(logger.WithGroup("httpserver"))(server)
}

// WithLogger configures error and server logger.
func WithLogger(logger *slog.Logger) Option {
	return func(server *Server) {
		server.HTTPServer.ErrorLog = slog.NewLogLogger(logger.Handler(), slog.LevelError)
		server.log = logger
	}
}

// WithShutdownTimeout so server doesn't exceed the provided duration after receiving a stop signal.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(server *Server) {
		server.shutdownTimeout = timeout
	}
}

// WithServerTimeouts configures "net/http".Server ReadTimeout, WriteTimeout and IdleTimeout.
func WithServerTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration) Option {
	return func(server *Server) {
		server.HTTPServer.ReadTimeout = readTimeout
		server.HTTPServer.WriteTimeout = writeTimeout
		server.HTTPServer.IdleTimeout = idleTimeout
	}
}

// ProductionMode don't listen for file changes and the default logger is a JSON logger.
func ProductionMode() Option {
	return WithMode(ModeProduction)
}

// DevelopmentMode listens for file changes and restarts accordingly (as well as running go generate)
// and the default logger is pretty printer.
func DevelopmentMode() Option {
	return WithMode(ModeDevelopment)
}

// WithMode allows the user to specify the server mode by value.
// DevelopmentMode() and ProductionMode() options are just a syntactic sugar around this function.
func WithMode(mode Mode) Option {
	return func(server *Server) {
		server.mode = mode
	}
}

// ListenOn the host and the port (e.g: localhost:3000 or :8080)
// The default value depends on the server mode:
// In Development = localhost:8080 (http only)
// In Production = 0.0.0.0:80 & 0.0.0.0:443 (if TLS is enabled)
func ListenOn(listen string) Option {
	return func(s *Server) {
		s.listenAddress = listen
	}
}
