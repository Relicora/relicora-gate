package gate

import (
	"fmt"
	"log"
	"net/http"
)

type AppOption func(*App)

func WithAddr(addr string) AppOption {
	return func(a *App) {
		a.server.Addr = addr
	}
}

func WithPort(port int) AppOption {
	return func(a *App) {
		a.server.Addr = fmt.Sprintf(":%d", port)
	}
}

func WithLogger(logger *log.Logger) AppOption {
	return func(a *App) {
		if logger != nil {
			a.logger = logger
		}
	}
}

type App struct {
	server      *http.Server
	rootMux     *http.ServeMux
	middlewares []func(http.Handler) http.Handler
	logger      *log.Logger
}

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

func (a *App) Get(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodGet, handler))
}

func (a *App) Post(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodPost, handler))
}

func (a *App) Put(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodPut, handler))
}

func (a *App) Delete(route string, handler func(w http.ResponseWriter, r *http.Request)) {
	a.rootMux.HandleFunc(route, methodHandler(http.MethodDelete, handler))
}

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
