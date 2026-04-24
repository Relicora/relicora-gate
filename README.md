# relicora-gate

`relicora-gate` is a lightweight Go library for building HTTP applications with a simple router, middleware support, and nested routers.

## Overview

The library provides:

- `App` as the main HTTP application container
- functional options for configuring address, port, and logger
- `Get`, `Post`, `Put`, `Delete` methods to register routes
- middleware support for both `App` and each `Router`
- nested routers via `App.NewRouter` and `Router.NewRouter`
- automatic `405 Method Not Allowed` responses for wrong HTTP methods

## Installation

Run:

```bash
go get github.com/Relicora/relicora-gate
```

Then import the package in your Go code.

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/Relicora/relicora-gate"
)

func main() {
    app := gate.New(
        gate.WithPort(8080),
        gate.WithLogger(log.Default()),
    )

    app.AddMiddleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            log.Printf("incoming request %s %s", r.Method, r.URL.Path)
            next.ServeHTTP(w, r)
        })
    })

    app.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello from relicora-gate"))
    })

    api := app.NewRouter("/api")
    api.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("pong"))
    })

    log.Println("starting server")
    app.ListenAndServe()
}
```

## API Reference

### `gate.New(opts ...AppOption) *App`

Creates a new `App` instance.

Supported options:

- `gate.WithAddr(addr string)` sets the full server address, for example `"127.0.0.1:8080"`
- `gate.WithPort(port int)` sets the server port; default address is `:8080`
- `gate.WithLogger(logger *log.Logger)` sets a custom logger; if `nil` is passed, `log.Default()` is used

### `(*App) AddMiddleware(middleware func(http.Handler) http.Handler)`

Adds middleware to the entire application. Middleware is executed in the order it is added.

### `(*App) Get/Post/Put/Delete(route string, handler func(http.ResponseWriter, *http.Request))`

Registers a route on the main `ServeMux`.

### `(*App) NewRouter(prefix string) *Router`

Creates a nested router mounted under the given prefix.

### `(*Router) NewRouter(prefix string) *Router`

Creates a nested router under the current router.

### `(*Router) AddMiddleware(middleware func(http.Handler) http.Handler)`

Adds middleware only for this specific router.

### `(*Router) Get/Post/Put/Delete(route string, handler func(http.ResponseWriter, *http.Request))`

Registers a route inside a router. The route is defined without the parent prefix.

### `(*App) ListenAndServe()`

Starts the HTTP server. To stop the server from tests or another context, use `app.server.Close()`.

## Nested Router Example

```go
api := app.NewRouter("/api")
api.AddMiddleware(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-API", "true")
        next.ServeHTTP(w, r)
    })
})

v1 := api.NewRouter("/v1")
v1.Get("/status", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("ok"))
})
```

A request to `/api/v1/status` will be handled by the nested router and will also pass through the `api` middleware.

## Wrong Method Handling

If a route is registered with `Get`, but the request uses `POST`, the library returns `405 Method Not Allowed`.

## Testing

Tests are located in `gate_test.go`.

Run them with:

```bash
go test ./...
```

## Files and Recommendations

- `CHANGELOG.md` contains the history of changes.
- `.gitignore` includes `coverage.out`.

`coverage.out` is a temporary artifact produced by `go test -coverprofile=coverage.out`. It should not be committed to the repository.
