package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	s.HTTPServer.BaseContext = func(_ net.Listener) context.Context { return serverCtx }

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- s.HTTPServer.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err := <-srvErr:
		return err
	case <-serverCtx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	return s.startGracefulShutdown()
}

func (s *Server) startGracefulShutdown() error {
	timeoutContext, doCancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer doCancel()

	// We received an interrupt signal, shut down.
	s.log.Info("Shutting down ..")
	s.HTTPServer.SetKeepAlivesEnabled(false)
	if err := s.HTTPServer.Shutdown(timeoutContext); err != nil {
		// Error from closing listeners, or context timeout.
		return err
	}

	return nil
}
