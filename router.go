package gate

import (
	"net/http"
)

type Router struct {
	routerMux   *http.ServeMux
	middlewares []func(http.Handler) http.Handler
	prefix      string
}

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

// AddMiddleware добавляет middleware для конкретного роутера
func (r *Router) AddMiddleware(middleware func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middleware)
}

// ServeHTTP реализует интерфейс http.Handler для роутера
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Применяем все middleware роутера к его routerMux
	var handler http.Handler = r.routerMux
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	handler.ServeHTTP(w, req)
}

func (r *Router) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodGet, handler))
}

func (r *Router) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodPost, handler))
}

func (r *Router) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodPut, handler))
}

func (r *Router) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	r.routerMux.HandleFunc(route, methodHandler(http.MethodDelete, handler))
}
