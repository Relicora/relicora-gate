// Package gate provides a lightweight HTTP application container with
// route registration, middleware support, and nested router handling.
package gate

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// AppOption is a functional configuration option for an App.
type AppOption func(*App)

// WithAddr sets the full address for the HTTP server.
// Example: "127.0.0.1:8080".
func WithAddr(addr string) AppOption {
	return func(a *App) {
		a.addrOption = addr
	}
}

// WithPort sets the port for the HTTP server.
// If WithAddr was also provided, the port is combined with the given host.
func WithPort(port int) AppOption {
	return func(a *App) {
		a.port = port
		a.portSet = true
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
	routes      routeManager
	middlewares []func(http.Handler) http.Handler
	logger      *log.Logger
	addrOption  string
	port        int
	portSet     bool
}

func (a *App) resolveAddr() string {
	if a.addrOption == "" {
		return fmt.Sprintf(":%d", a.port)
	}

	host, _, err := net.SplitHostPort(a.addrOption)
	if err == nil {
		if a.portSet {
			return net.JoinHostPort(host, strconv.Itoa(a.port))
		}
		return a.addrOption
	}

	return net.JoinHostPort(a.addrOption, strconv.Itoa(a.port))
}

func normalizeRoute(route string) string {
	if route == "" {
		return "/"
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	if len(route) > 1 {
		route = strings.TrimRight(route, "/")
	}
	return route
}

func normalizePrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func normalizeMethod(method string) string {
	if method == "" {
		return http.MethodGet
	}
	return strings.ToUpper(strings.TrimSpace(method))
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
		routes:      routeManager{},
		middlewares: make([]func(http.Handler) http.Handler, 0),
		logger:      log.Default(),
		addrOption:  "",
		port:        8080,
		portSet:     false,
	}

	for _, opt := range opts {
		opt(app)
	}

	app.server.Addr = app.resolveAddr()
	return app
}

// AddMiddleware appends a middleware layer to the application.
// Middleware wraps request handling for all registered routes.
func (a *App) AddMiddleware(middleware func(http.Handler) http.Handler) {
	a.middlewares = append(a.middlewares, middleware)
}

// Use is an alias for AddMiddleware and is more idiomatic for middleware stacking.
func (a *App) Use(middleware func(http.Handler) http.Handler) {
	a.AddMiddleware(middleware)
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

func (a *App) addRoute(route, method string, handler http.Handler) {
	a.routes.addRoute(route, method, handler)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var final http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.routes.serve(w, r) {
			return
		}

		cap := newCaptureResponseWriter()
		a.rootMux.ServeHTTP(cap, r)
		if cap.status != http.StatusNotFound {
			cap.flushTo(w)
			return
		}

		a.routes.notFound(w, r)
	})

	for i := len(a.middlewares) - 1; i >= 0; i-- {
		final = a.middlewares[i](final)
	}

	final.ServeHTTP(w, r)
}

// NotFoundHandler registers a custom handler for unmatched routes.
func (a *App) NotFoundHandler(handler func(w http.ResponseWriter, r *http.Request)) {
	a.routes.setNotFoundHandler(http.HandlerFunc(handler))
}

// MethodNotAllowedHandler registers a custom handler for requests with an unsupported HTTP method.
func (a *App) MethodNotAllowedHandler(handler func(w http.ResponseWriter, r *http.Request)) {
	a.routes.setMethodNotAllowedHandler(http.HandlerFunc(handler))
}

// Group creates a child router under the specified prefix.
func (a *App) Group(prefix string) *Router {
	return a.NewRouter(prefix)
}

// Handle registers a handler for the given route and HTTP methods.
func (a *App) Handle(route string, handler http.Handler, methods ...string) {
	if len(methods) == 0 {
		methods = []string{http.MethodGet}
	}
	for _, method := range methods {
		a.addRoute(route, normalizeMethod(method), handler)
	}
}

// HandleFunc registers a handler function for the given route and HTTP methods.
func (a *App) HandleFunc(route string, handler func(w http.ResponseWriter, r *http.Request), methods ...string) {
	a.Handle(route, http.HandlerFunc(handler), methods...)
}

// Get registers a handler for HTTP GET requests at the given route.
func (a *App) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.Handle(route, http.HandlerFunc(handler), http.MethodGet)
}

// Post registers a handler for HTTP POST requests at the given route.
func (a *App) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.Handle(route, http.HandlerFunc(handler), http.MethodPost)
}

// Put registers a handler for HTTP PUT requests at the given route.
func (a *App) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.Handle(route, http.HandlerFunc(handler), http.MethodPut)
}

// Delete registers a handler for HTTP DELETE requests at the given route.
func (a *App) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.Handle(route, http.HandlerFunc(handler), http.MethodDelete)
}

// ListenAndServe applies registered middleware and starts the HTTP server.
// This method blocks until the server exits.
func (a *App) ListenAndServe() error {
	a.logger.Printf("[INFO]\tServer starting...\n")
	a.server.Handler = a
	a.logger.Printf("[INFO]\tServer started at \"%s\"\n", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown works by first closing all open listeners, then closing all idle connections,
// and then waiting indefinitely for connections to return to idle and then shut down.
func (a *App) Shutdown(ctx context.Context) error {
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
