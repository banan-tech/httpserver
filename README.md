# httpserver

A small HTTP server wrapper for Go with Kubernetes-native health probes and
graceful shutdown. It wraps `net/http.Server`, adds `/livez` and `/readyz`
endpoints, and drains connections cleanly on `SIGINT`/`SIGTERM`.

The server is written in [Lisette](https://github.com/ivov/lisette) (in `src/`)
and compiled to Go. The generated Go package lives at the repo root, so other
Go projects use it like any normal library.

> **Note:** this library does not handle routing. Bring your own router
> (`chi`, `gorilla/mux`, `http.ServeMux`, …) and pass it via `WithHandler`.

## Install

```sh
go get github.com/banansys/httpserver
```

## Quick start

```go
package main

import (
	"net/http"

	"github.com/banansys/httpserver"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World"))
	})

	srv := httpserver.New([]httpserver.ServerOption{
		httpserver.WithHandler(mux),
		httpserver.WithAddr(":8080"),
	})

	// Run blocks until SIGINT/SIGTERM, then shuts down gracefully.
	if err := srv.Run(); err != nil {
		panic(err)
	}
}
```

## Endpoints

Registered automatically unless `WithoutDefaultProbes()` is used:

| Path        | Purpose                                                                                   |
| ----------- | ----------------------------------------------------------------------------------------- |
| `/livez`    | Liveness — static `200` while the process can serve. No dependency checks (restart only).  |
| `/readyz`   | Readiness — `200` when ready and all checks pass; `503` while shutting down or on failure. |
| `/_metrics` | Only if `WithMetricsHandler` is set (e.g. Prometheus).                                     |
| `/`         | Your handler, if provided via `WithHandler`.                                               |

A failing readiness check returns `503` with a JSON body naming the check:

```json
{ "failed_check": "db", "error": "connection refused" }
```

## Options

Pass any of these to `New`:

| Option                         | Default | Description                                               |
| ------------------------------ | ------- | --------------------------------------------------------- |
| `WithAddr(addr)`               | `:8080` | Listen address.                                           |
| `WithPortFromEnv()`            | —       | Use `:$PORT` if `PORT` is set (applied before options).   |
| `WithHandler(h)`               | —       | Root handler mounted at `/`.                              |
| `WithMetricsHandler(h)`        | —       | Handler mounted at `/_metrics`.                           |
| `WithReadinessCheck(name, fn)` | —       | Register a named dependency check for `/readyz`.          |
| `WithoutDefaultProbes()`       | off     | Disable the built-in `/livez` and `/readyz`.              |
| `WithLogger(l)`                | JSON    | Structured `*slog.Logger`.                                |
| `WithReadHeaderTimeout(d)`     | `5s`    | Header read deadline (Slowloris protection).              |
| `WithReadTimeout(d)`           | `15s`   | Full request read deadline.                               |
| `WithWriteTimeout(d)`          | `70s`   | Response write deadline.                                  |
| `WithIdleTimeout(d)`           | `90s`   | Keep-alive idle timeout.                                  |
| `WithShutdownTimeout(d)`       | `15s`   | Graceful drain deadline.                                  |
| `WithPreShutdownDelay(d)`      | `5s`    | Keep serving after `SIGTERM` while `/readyz` reports 503. |
| `WithShutdownHook(fn)`         | —       | Run during shutdown, after connections drain.             |

### Readiness checks

```go
srv := httpserver.New([]httpserver.ServerOption{
	httpserver.WithHandler(mux),
	httpserver.WithReadinessCheck("db", func(ctx context.Context) error {
		return db.PingContext(ctx)
	}),
})
```

## Graceful shutdown

On `SIGINT`/`SIGTERM`, `Run` flips `/readyz` to `503` (so Kubernetes stops
routing new traffic), waits `PreShutdownDelay` for endpoint depropagation, then
drains in-flight requests within `ShutdownTimeout` before exiting.

> Keep `PreShutdownDelay + ShutdownTimeout` below the pod's
> `terminationGracePeriodSeconds`, or `SIGKILL` fires before the drain finishes.
> Defaults: `5s + 15s = 20s < 30s` (the Kubernetes default).

## Server API

| Method          | Description                                                       |
| --------------- | ----------------------------------------------------------------- |
| `New(opts)`     | Build a server from options.                                      |
| `Run()`         | Serve, blocking until a signal, then shut down gracefully.        |
| `Start()`       | Serve, blocking (no signal handling). A clean stop returns `nil`. |
| `Shutdown(ctx)` | Drain connections within the shutdown timeout, then run hooks.    |
| `Handler()`     | The root `http.Handler`, handy for `httptest`.                    |

## Development

The source of truth is the Lisette code in `src/`. The root `.go` files are
generated — do not edit them by hand.

```sh
lis run            # run the example (src/main.lis)
lis check          # type-check
make emit          # compile src/ to Go and emit the package into the repo root
```
