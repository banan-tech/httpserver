package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjeczalik/notify"
)

type Mode string

const (
	ModeProduction  Mode = "production"
	ModeDevelopment Mode = "development"
)

type Server struct {
	shutdownTimeout time.Duration
	listenAddress   string
	mode            Mode
	watchEnabled    bool

	HTTPServer *http.Server
	log        *slog.Logger

	ctx     context.Context
	stopCtx context.CancelFunc

	shutdownHooks []func(context.Context) error
}

func New(handler http.Handler, options ...Option) *Server {

	srv := &Server{
		HTTPServer: &http.Server{
			Addr:    "",
			Handler: handler,
		},
		shutdownTimeout: defaultShutdownTimeout,
	}

	for _, option := range options {
		option(srv)
	}

	if srv.mode != ModeProduction {
		srv.mode = ModeDevelopment
	}

	if srv.log == nil {
		setDefaultLogger(srv)
	}

	if srv.listenAddress == "" {
		switch srv.mode {
		case ModeProduction:
			srv.listenAddress = "0.0.0.0:80"
		case ModeDevelopment:
			srv.listenAddress = "localhost:8080"
		}
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	srv.ctx = serverCtx
	srv.stopCtx = stop

	return srv
}

func (s *Server) AddShutdownHook(hook func(context.Context) error) {
	s.shutdownHooks = append(s.shutdownHooks, hook)
}

func (s *Server) Run() error {
	defer s.stopCtx()

	s.log.Info("Starting HTTP server",
		"mode", s.mode,
		"listenAddress", formatListenAddress(s.listenAddress),
	)

	s.HTTPServer.BaseContext = func(_ net.Listener) context.Context { return s.ctx }
	s.HTTPServer.Addr = s.listenAddress

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- s.HTTPServer.ListenAndServe()
	}()

	if s.mode == ModeProduction {
		// Wait for interruption.
		select {
		case err := <-srvErr:
			return err
		case <-s.ctx.Done():
			// Wait for first CTRL+C.
			// Stop receiving signal notifications as soon as possible.
			s.stopCtx()
		}
	} else {
		fileChangesChan := watchForFileChanges(s.log)
		defer notify.Stop(fileChangesChan)
	loop:
		for {
			select {
			case err := <-srvErr:
				return err
			case <-s.ctx.Done():
				// Wait for first CTRL+C.
				// Stop receiving signal notifications as soon as possible.
				s.stopCtx()
				break loop
			case changeEvent := <-fileChangesChan:
				s.handleFileChange(changeEvent)
			}
		}
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	return s.startGracefulShutdown()
}

func (s *Server) Context() context.Context {
	return s.ctx
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

	var err error
	for _, hook := range s.shutdownHooks {
		err = errors.Join(err, hook(timeoutContext)) // TODO use multierrors
	}

	return err
}

func (s *Server) handleFileChange(event notify.EventInfo) {
	if !s.watchEnabled {
		return
	}
	isGoFile := strings.HasSuffix(event.Path(), ".go")
	if !isGoFile {
		return
	}

	moduleRoot := modulePath()
	pathToGenerate := strings.Replace(path.Dir(event.Path()), moduleRoot, ".", 1)
	s.log.Info("file changed", "event", event.Event(), "path", pathToGenerate)
	genCmd := exec.Command("go", "generate", pathToGenerate)
	genCmd.Dir = moduleRoot
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr

	err := genCmd.Run()
	if err != nil {
		s.log.Error("go generate failed", "error", err)
		return
	}

	os.Getwd()
}

func watchForFileChanges(logger *slog.Logger) (c chan notify.EventInfo) {
	// Make the channel buffered to ensure no event is dropped. Notify will drop
	// an event if the receiver is not able to keep up the sending pace.
	c = make(chan notify.EventInfo, 1)
	// Set up a watchpoint listening on events within current working directory.
	// Dispatch each create and remove events separately to c.
	watchPath := modulePath()
	if err := notify.Watch(path.Join(watchPath, "..."), c, notify.All); err != nil {
		panic(fmt.Sprintf("Error with watching for file changes, exiting ... %v", err))
	}
	logger.Info("Watching for file changes", "path", watchPath)
	return
}

func modulePath() (roots string) {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	dir = filepath.Clean(dir)
	// Look for enclosing go.mod.
	for {
		ffs := os.DirFS(dir)
		info, err := fs.Stat(ffs, "go.mod")
		if err == nil && !info.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}

	panic(errors.New("couldn't find go.mod!"))
}

func formatListenAddress(addr string) string {
	if strings.Index(addr, ":") == 0 {
		addr = "0.0.0.0" + addr
	}
	return fmt.Sprintf("http://%s", addr)
}
