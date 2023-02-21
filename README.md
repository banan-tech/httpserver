# A Simple Golang Server with Graceful shutdown

**Note** This library doesn't handle routing

### Usage

```go
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/khaledez/httpserver"
)

var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, World"))
})

func main() {
	httpLogger := log.New(os.Stdout, "[http] ", log.Lmsgprefix|log.Ldate|log.Lmicroseconds)
	server := httpserver.New(3000, 10*time.Second, handler, httpLogger)

	if err := server.Run(); err != nil {
		httpLogger.Fatal(err)
	}
}

```