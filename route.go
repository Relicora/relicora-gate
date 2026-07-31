package gate

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type routeEntry struct {
	segment       string
	pattern       string
	methods       map[string]http.Handler
	children      map[string]*routeEntry
	paramChild    *routeEntry
	wildcardChild *routeEntry
	paramName     string
	isWildcard    bool
	router        *Router
}

type routeManager struct {
	mu sync.RWMutex

	root *routeEntry

	routerMap      map[string]*Router
	routerPrefixes []string

	notFoundHandler         http.HandlerFunc
	methodNotAllowedHandler http.HandlerFunc
}

func newRouteEntry(segment string) *routeEntry {
	return &routeEntry{
		segment:  segment,
		methods:  make(map[string]http.Handler),
		children: make(map[string]*routeEntry),
	}
}

func (rm *routeManager) addRoute(route, method string, handler http.Handler, router *Router) {
	route = normalizeRoute(route)
	method = normalizeMethod(method)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.root == nil {
		rm.root = newRouteEntry("")
		rm.root.pattern = "/"
	}

	node := rm.root
	segments := routeSegments(route)
	for i, segment := range segments {
		if strings.HasPrefix(segment, "*") {
			if node.wildcardChild == nil {
				node.wildcardChild = newRouteEntry(segment)
				node.wildcardChild.isWildcard = true
				node.wildcardChild.paramName = strings.TrimPrefix(segment, "*")
				node.wildcardChild.pattern = route
				node.wildcardChild.router = router
			}
			node = node.wildcardChild
			break
		}

		if strings.HasPrefix(segment, ":") {
			if node.paramChild == nil {
				node.paramChild = newRouteEntry(segment)
				node.paramChild.paramName = strings.TrimPrefix(segment, ":")
				node.paramChild.pattern = route
				node.paramChild.router = router
			}
			node = node.paramChild
			continue
		}

		child, ok := node.children[segment]
		if !ok {
			child = newRouteEntry(segment)
			child.pattern = route
			child.router = router
			node.children[segment] = child
		}
		node = child
		if i == len(segments)-1 {
			node.pattern = route
			node.router = router
		}
	}

	if len(segments) == 0 {
		node.pattern = route
		node.router = router
	}

	if node.methods == nil {
		node.methods = make(map[string]http.Handler)
	}
	if _, exists := node.methods[method]; exists {
		return
	}
	node.methods[method] = handler
	if node.router == nil {
		node.router = router
	}
}

func (rm *routeManager) matchRoute(path string) (*routeEntry, map[string]string, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.root == nil {
		return nil, nil, false
	}

	path = normalizeRoute(path)
	segments := routeSegments(path)
	params := make(map[string]string)

	node, ok := rm.root.matchSegments(segments, params)
	if !ok || node == nil {
		return nil, nil, false
	}

	return node, params, true
}

func (rm *routeManager) notFound(w http.ResponseWriter, r *http.Request) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if router := rm.routerForPath(r.URL.Path); router != nil && router.notFoundHandler != nil {
		router.notFoundHandler.ServeHTTP(w, r)
		return
	}
	if rm.notFoundHandler != nil {
		rm.notFoundHandler.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (rm *routeManager) methodNotAllowed(w http.ResponseWriter, r *http.Request, entry *routeEntry) {
	w.Header().Set("Allow", entry.allowHeader())

	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if router := rm.routerForEntry(entry); router != nil && router.methodNotAllowedHandler != nil {
		router.methodNotAllowedHandler.ServeHTTP(w, r)
		return
	}
	if rm.methodNotAllowedHandler != nil {
		rm.methodNotAllowedHandler.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (rm *routeManager) setNotFoundHandler(handler http.HandlerFunc) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.notFoundHandler = handler
}

func (rm *routeManager) setMethodNotAllowedHandler(handler http.HandlerFunc) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.methodNotAllowedHandler = handler
}

func (rm *routeManager) registerRouter(prefix string, router *Router) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.routerMap == nil {
		rm.routerMap = make(map[string]*Router)
	}
	if _, ok := rm.routerMap[prefix]; ok {
		return
	}
	if rm.routerPrefixes == nil {
		rm.routerPrefixes = make([]string, 0)
	}

	rm.routerMap[prefix] = router
	rm.routerPrefixes = append(rm.routerPrefixes, prefix)
	sort.SliceStable(rm.routerPrefixes, func(i, j int) bool {
		if len(rm.routerPrefixes[i]) == len(rm.routerPrefixes[j]) {
			return rm.routerPrefixes[i] > rm.routerPrefixes[j]
		}
		return len(rm.routerPrefixes[i]) > len(rm.routerPrefixes[j])
	})
}

func (rm *routeManager) routerForPath(path string) *Router {
	path = normalizeRoute(path)
	for _, prefix := range rm.routerPrefixes {
		if prefix == "" {
			return rm.routerMap[prefix]
		}
		if strings.HasPrefix(path, prefix) {
			if len(path) == len(prefix) || strings.HasPrefix(path[len(prefix):], "/") {
				return rm.routerMap[prefix]
			}
		}
	}
	return nil
}

func (rm *routeManager) routerForEntry(entry *routeEntry) *Router {
	if entry == nil {
		return nil
	}
	if entry.router != nil {
		return entry.router
	}
	return rm.routerForPath(entry.pattern)
}

func (rm *routeManager) serve(w http.ResponseWriter, r *http.Request) bool {
	entry, params, ok := rm.matchRoute(r.URL.Path)
	if !ok {
		return false
	}

	handler, methodOk := entry.methods[normalizeMethod(r.Method)]
	if methodOk {
		if params != nil {
			r = r.WithContext(contextWithRouteParams(r.Context(), params))
		}
		handler.ServeHTTP(w, r)
		return true
	}

	rm.methodNotAllowed(w, r, entry)
	return true
}

func (rm *routeManager) serveFallback(w http.ResponseWriter, r *http.Request, fallback http.Handler) {
	if rm.serve(w, r) {
		return
	}

	cap := newCaptureResponseWriter()
	fallback.ServeHTTP(cap, r)
	if cap.status != http.StatusNotFound {
		cap.flushTo(w)
		return
	}

	rm.notFound(w, r)
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{
		head:   make(http.Header),
		status: http.StatusOK,
	}
}

type captureResponseWriter struct {
	head    http.Header
	body    bytes.Buffer
	status  int
	written bool
}

func (c *captureResponseWriter) Header() http.Header {
	return c.head
}

func (c *captureResponseWriter) WriteHeader(status int) {
	if c.written {
		return
	}
	c.status = status
	c.written = true
}

func (c *captureResponseWriter) Write(p []byte) (int, error) {
	if !c.written {
		c.status = http.StatusOK
		c.written = true
	}
	return c.body.Write(p)
}

func (c *captureResponseWriter) flushTo(w http.ResponseWriter) {
	for k, values := range c.head {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	if c.written {
		w.WriteHeader(c.status)
	}
	_, _ = io.Copy(w, &c.body)
}

func routeSegments(route string) []string {
	route = strings.Trim(route, "/")
	if route == "" {
		return []string{}
	}
	return strings.Split(route, "/")
}

func (r *routeEntry) allowHeader() string {
	allowed := make([]string, 0, len(r.methods))
	for method := range r.methods {
		allowed = append(allowed, method)
	}
	sort.Strings(allowed)
	return strings.Join(allowed, ", ")
}

func (r *routeEntry) matchSegments(segments []string, params map[string]string) (*routeEntry, bool) {
	if len(segments) == 0 {
		if len(r.methods) > 0 {
			return r, true
		}
		return nil, false
	}

	segment := segments[0]
	if child, ok := r.children[segment]; ok {
		if node, ok2 := child.matchSegments(segments[1:], params); ok2 {
			return node, true
		}
	}

	if r.paramChild != nil {
		params[r.paramChild.paramName] = segment
		if node, ok2 := r.paramChild.matchSegments(segments[1:], params); ok2 {
			return node, true
		}
		delete(params, r.paramChild.paramName)
	}

	if r.wildcardChild != nil {
		params[r.wildcardChild.paramName] = strings.Join(segments, "/")
		return r.wildcardChild, true
	}

	return nil, false
}

type routeParamsKey struct{}

func contextWithRouteParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, routeParamsKey{}, params)
}

func RouteParams(r *http.Request) map[string]string {
	if params, ok := r.Context().Value(routeParamsKey{}).(map[string]string); ok {
		return params
	}
	return make(map[string]string)
}
