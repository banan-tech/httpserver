package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type Server struct {
	shutdownTimeout time.Duration
	port            uint
	address         string

	HTTPServer *http.Server
	log        *slog.Logger
}

func New(address string, port uint, handler http.Handler, options ...Option) *Server {
	if address == "" {
		address = "0.0.0.0"
	}

	srv := &Server{
		port:    port,
		address: address,
		HTTPServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", address, port),
			Handler: handler,
		},
		shutdownTimeout: defaultShutdownTimeout,
	}

	for _, option := range options {
		option(srv)
	}

	if srv.log == nil {
		setDefaultLogger(srv)
	}

	return srv
}

func (s *Server) Run() error {
	s.log.Info(fmt.Sprintf("Starting HTTP server http://%s:%d", s.address, s.port), "port", s.port)

	shutdownContext, doShutdown := context.WithCancel(context.Background())
	defer doShutdown()

	go s.ensureGracefulShutdown(shutdownContext, doShutdown)

	if err := s.HTTPServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	// don't return until shutdown is complete
	<-shutdownContext.Done()
	return nil
}

func (s *Server) ensureGracefulShutdown(shutdownContext context.Context, doShutdown context.CancelFunc) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	<-sigint

	timeoutContext, doCancel := context.WithTimeout(shutdownContext, s.shutdownTimeout)
	defer doCancel()

	// We received an interrupt signal, shut down.
	s.log.Info("Shutting down ..")
	s.HTTPServer.SetKeepAlivesEnabled(false)
	if err := s.HTTPServer.Shutdown(timeoutContext); err != nil {
		// Error from closing listeners, or context timeout:
		s.log.Error("closing server", "error", err)
	}
	doShutdown()
}
