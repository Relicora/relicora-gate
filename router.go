package gate

import (
	"net/http"
)

// Router is a nested route container that supports its own middleware and routes.
type Router struct {
	routerMux   *http.ServeMux
	middlewares []func(http.Handler) http.Handler
	prefix      string
}

// NewRouter creates a nested router mounted under the specified prefix.
// Requests beginning with the prefix are routed through the new Router.
func (a *App) NewRouter(prefix string) *Router {
	routerMux := http.NewServeMux()
	r := &Router{
		routerMux:   routerMux,
		middlewares: make([]func(http.Handler) http.Handler, 0),
		prefix:      prefix,
	}

	a.rootMux.Handle(prefix+"/", http.StripPrefix(prefix, r))
	return r
}

// NewRouter creates a child router under the current router prefix.
func (r *Router) NewRouter(prefix string) *Router {
	routerMux := http.NewServeMux()
	newRouter := &Router{
		routerMux:   routerMux,
		middlewares: make([]func(http.Handler) http.Handler, 0),
		prefix:      r.prefix + prefix,
	}

	r.routerMux.Handle(prefix+"/", http.StripPrefix(prefix, newRouter))
	return newRouter
}

// AddMiddleware adds middleware specifically for this router.
func (r *Router) AddMiddleware(middleware func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middleware)
}

// ServeHTTP applies router middleware and delegates request handling to the router mux.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var handler http.Handler = r.routerMux
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	handler.ServeHTTP(w, req)
}

// Get registers a handler for HTTP GET requests on this router.
func (r *Router) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodGet, handler))
}

// Post registers a handler for HTTP POST requests on this router.
func (r *Router) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodPost, handler))
}

// Put registers a handler for HTTP PUT requests on this router.
func (r *Router) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodPut, handler))
}

// Delete registers a handler for HTTP DELETE requests on this router.
func (r *Router) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodDelete, handler))
}
