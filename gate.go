// Package gate provides a lightweight HTTP application container with
// route registration, middleware support, and nested router handling.
package gate

import (
	"fmt"
	"log"
	"net/http"
)

// AppOption is a functional configuration option for an App.
type AppOption func(*App)

// WithAddr sets the full address for the HTTP server.
// Example: "127.0.0.1:8080".
func WithAddr(addr string) AppOption {
	return func(a *App) {
		a.server.Addr = addr
	}
}

// WithPort sets the port for the HTTP server.
// The address is configured as ":<port>".
func WithPort(port int) AppOption {
	return func(a *App) {
		a.server.Addr = fmt.Sprintf(":%d", port)
	}
}

// WithLogger sets a custom logger for application startup and request logging.
// If logger is nil, the default standard logger is preserved.
func WithLogger(logger *log.Logger) AppOption {
	return func(a *App) {
		if logger != nil {
			a.logger = logger
		}
	}
}

// App represents the HTTP application and its route/middleware configuration.
type App struct {
	server      *http.Server
	rootMux     *http.ServeMux
	middlewares []func(http.Handler) http.Handler
	logger      *log.Logger
}

// New creates a new App with optional configuration options.
// The default server address is ":8080" unless overridden.
func New(opts ...AppOption) *App {
	rootMux := http.NewServeMux()
	s := &http.Server{
		Addr:    ":8080",
		Handler: rootMux,
	}
	app := &App{
		server:      s,
		rootMux:     rootMux,
		middlewares: make([]func(http.Handler) http.Handler, 0),
		logger:      log.Default(),
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

// AddMiddleware appends a middleware layer to the application.
// Middleware wraps request handling for all registered routes.
func (a *App) AddMiddleware(middleware func(http.Handler) http.Handler) {
	a.middlewares = append(a.middlewares, middleware)
}

func methodHandler(method string, handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

// Get registers a handler for HTTP GET requests at the given route.
func (a *App) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodGet, handler))
}

// Post registers a handler for HTTP POST requests at the given route.
func (a *App) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodPost, handler))
}

// Put registers a handler for HTTP PUT requests at the given route.
func (a *App) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodPut, handler))
}

// Delete registers a handler for HTTP DELETE requests at the given route.
func (a *App) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodDelete, handler))
}

// ListenAndServe applies registered middleware and starts the HTTP server.
// This method blocks until the server exits.
func (a *App) ListenAndServe() {
	a.logger.Printf("[INFO]	Server starting...\n")
	var handler http.Handler = a.rootMux
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		handler = a.middlewares[i](handler)
	}
	a.server.Handler = handler
	a.logger.Printf("[INFO]	Server started at \"%s\"\n", a.server.Addr)
	a.server.ListenAndServe()
}
