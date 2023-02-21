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

	srv *http.Server
	log *log.Logger
}

func New(port uint, shutdownTimeout time.Duration, handler http.Handler, logger *log.Logger) *Server {
	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		},
		log:             logger,
		shutdownTimeout: shutdownTimeout,
	}
}

func (s *Server) Run() error {
	s.log.Printf("Starting HTTP server http://0.0.0.0:%s", s.srv.Addr[1:])

	shutdownContext, doShutdown := context.WithCancel(context.Background())
	defer doShutdown()

	go s.ensureGracefulShutdown(shutdownContext, doShutdown)

	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
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
	if err := s.srv.Shutdown(timeoutContext); err != nil {
		// Error from closing listeners, or context timeout:
		s.log.Println(err)
	}
	doShutdown()
}
