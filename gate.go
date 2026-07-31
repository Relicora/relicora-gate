// Package gate provides a lightweight HTTP application container with
// route registration, middleware support, and nested router handling.
package gate

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
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
	routeTable  map[string]map[string]http.Handler
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
		routeTable:  make(map[string]map[string]http.Handler),
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

func methodHandler(method string, handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

func allowMethods(methods map[string]http.Handler) string {
	allowed := make([]string, 0, len(methods))
	for method := range methods {
		allowed = append(allowed, method)
	}
	sort.Strings(allowed)
	return strings.Join(allowed, ", ")
}

func (a *App) addRoute(route, method string, handler http.Handler) {
	route = normalizeRoute(route)

	if a.routeTable == nil {
		a.routeTable = make(map[string]map[string]http.Handler)
	}

	methodHandlers, ok := a.routeTable[route]
	if !ok {
		methodHandlers = make(map[string]http.Handler)
		a.routeTable[route] = methodHandlers
		a.rootMux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			a.serveRoute(route, w, r)
		})
	}

	methodHandlers[method] = handler
}

func (a *App) serveRoute(route string, w http.ResponseWriter, r *http.Request) {
	methodHandlers, ok := a.routeTable[route]
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler, ok := methodHandlers[r.Method]
	if ok {
		handler.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Allow", allowMethods(methodHandlers))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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

// ServeHTTP applies registered middleware and serves the request.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler http.Handler = a.rootMux
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		handler = a.middlewares[i](handler)
	}
	handler.ServeHTTP(w, r)
}

// ListenAndServe applies registered middleware and starts the HTTP server.
// This method blocks until the server exits.
func (a *App) ListenAndServe() error {
	a.logger.Printf("[INFO]\tServer starting...\n")
	a.server.Handler = a
	a.logger.Printf("[INFO]	Server started at \"%s\"\n", a.server.Addr)
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
