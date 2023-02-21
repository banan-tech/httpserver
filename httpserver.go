package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type Server struct {
	shutdownTimeout time.Duration

	HTTPServer *http.Server
	log        *log.Logger
}

func New(port uint, handler http.Handler, options ...Option) *Server {
	srv := &Server{
		HTTPServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
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
	s.log.Printf("Starting HTTP server http://0.0.0.0:%s", s.HTTPServer.Addr[1:])

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
	s.log.Println("Shutting down ..")
	s.HTTPServer.SetKeepAlivesEnabled(false)
	if err := s.HTTPServer.Shutdown(timeoutContext); err != nil {
		// Error from closing listeners, or context timeout:
		s.log.Println(err)
	}
	doShutdown()
}
