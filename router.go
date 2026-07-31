package gate

import (
	"net/http"
)

// Router is a nested route container that supports its own middleware and routes.
type Router struct {
	app         *App
	parent      *Router
	middlewares []func(http.Handler) http.Handler
	prefix      string
}

// NewRouter creates a nested router mounted under the specified prefix.
func (a *App) NewRouter(prefix string) *Router {
	return &Router{
		app:         a,
		parent:      nil,
		middlewares: make([]func(http.Handler) http.Handler, 0),
		prefix:      normalizePrefix(prefix),
	}
}

// NewRouter creates a child router under the current router prefix.
func (r *Router) NewRouter(prefix string) *Router {
	return &Router{
		app:         r.app,
		parent:      r,
		middlewares: make([]func(http.Handler) http.Handler, 0),
		prefix:      r.prefix + normalizePrefix(prefix),
	}
}

// AddMiddleware adds middleware specifically for this router.
func (r *Router) AddMiddleware(middleware func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middleware)
}

// Use is an alias for AddMiddleware.
func (r *Router) Use(middleware func(http.Handler) http.Handler) {
	r.AddMiddleware(middleware)
}

func (r *Router) collectMiddlewares() []func(http.Handler) http.Handler {
	var chain []func(http.Handler) http.Handler
	if r.parent != nil {
		chain = append(chain, r.parent.collectMiddlewares()...)
	}
	return append(chain, r.middlewares...)
}

func (r *Router) wrapHandler(handler http.Handler) http.Handler {
	middlewares := r.collectMiddlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func (r *Router) addRoute(route, method string, handler http.Handler) {
	r.app.addRoute(r.prefix+normalizeRoute(route), method, r.wrapHandler(handler))
}

// Handle registers a handler for the given route and HTTP methods.
func (r *Router) Handle(route string, handler http.Handler, methods ...string) {
	if len(methods) == 0 {
		methods = []string{http.MethodGet}
	}
	for _, method := range methods {
		r.addRoute(route, normalizeMethod(method), handler)
	}
}

// HandleFunc registers a handler function for the given route and HTTP methods.
func (r *Router) HandleFunc(route string, handler func(w http.ResponseWriter, r *http.Request), methods ...string) {
	r.Handle(route, http.HandlerFunc(handler), methods...)
}

// Group creates a child router under the current router prefix.
func (r *Router) Group(prefix string) *Router {
	return r.NewRouter(prefix)
}

// Get registers a handler for HTTP GET requests on this router.
func (r *Router) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.Handle(route, http.HandlerFunc(handler), http.MethodGet)
}

// Post registers a handler for HTTP POST requests on this router.
func (r *Router) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.Handle(route, http.HandlerFunc(handler), http.MethodPost)
}

// Put registers a handler for HTTP PUT requests on this router.
func (r *Router) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.Handle(route, http.HandlerFunc(handler), http.MethodPut)
}

// Delete registers a handler for HTTP DELETE requests on this router.
func (r *Router) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.Handle(route, http.HandlerFunc(handler), http.MethodDelete)
}
