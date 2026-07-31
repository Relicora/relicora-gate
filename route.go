package gate

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
)

type routeEntry struct {
	pattern    string
	methods    map[string]http.Handler
	segments   []string
	paramNames []string
	hasParams  bool
	isWildcard bool
}

type routeManager struct {
	routeTable              map[string]*routeEntry
	routeEntries            []*routeEntry
	notFoundHandler         http.HandlerFunc
	methodNotAllowedHandler http.HandlerFunc
}

func newRouteEntry(pattern string) *routeEntry {
	entry := &routeEntry{
		pattern:  pattern,
		methods:  make(map[string]http.Handler),
		segments: routeSegments(pattern),
	}

	for _, segment := range entry.segments {
		if strings.HasPrefix(segment, ":") {
			entry.hasParams = true
			entry.paramNames = append(entry.paramNames, strings.TrimPrefix(segment, ":"))
		}
		if strings.HasPrefix(segment, "*") {
			entry.hasParams = true
			entry.isWildcard = true
			entry.paramNames = append(entry.paramNames, strings.TrimPrefix(segment, "*"))
		}
	}

	return entry
}

func (rm *routeManager) addRoute(route, method string, handler http.Handler) {
	route = normalizeRoute(route)
	method = normalizeMethod(method)

	if rm.routeTable == nil {
		rm.routeTable = make(map[string]*routeEntry)
	}

	entry, ok := rm.routeTable[route]
	if !ok {
		entry = newRouteEntry(route)
		rm.routeTable[route] = entry
		if entry.hasParams {
			rm.routeEntries = append(rm.routeEntries, entry)
			sortRouteEntries(rm.routeEntries)
		}
	}

	entry.methods[method] = handler
}

func (rm *routeManager) matchRoute(path string) (*routeEntry, map[string]string, bool) {
	path = normalizeRoute(path)

	if exact, ok := rm.routeTable[path]; ok {
		return exact, nil, true
	}

	for _, entry := range rm.routeEntries {
		params, ok := entry.match(path)
		if ok {
			return entry, params, true
		}
	}

	return nil, nil, false
}

func (rm *routeManager) notFound(w http.ResponseWriter, r *http.Request) {
	if rm.notFoundHandler != nil {
		rm.notFoundHandler.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (rm *routeManager) methodNotAllowed(w http.ResponseWriter, r *http.Request, entry *routeEntry) {
	w.Header().Set("Allow", entry.allowHeader())
	if rm.methodNotAllowedHandler != nil {
		rm.methodNotAllowedHandler.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (rm *routeManager) setNotFoundHandler(handler http.HandlerFunc) {
	rm.notFoundHandler = handler
}

func (rm *routeManager) setMethodNotAllowedHandler(handler http.HandlerFunc) {
	rm.methodNotAllowedHandler = handler
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

func (r *routeEntry) match(path string) (map[string]string, bool) {
	segments := routeSegments(path)
	if !r.hasParams {
		return nil, false
	}

	if r.isWildcard {
		if len(segments) < len(r.segments)-1 {
			return nil, false
		}
	} else if len(segments) != len(r.segments) {
		return nil, false
	}

	params := make(map[string]string)
	for i, segment := range r.segments {
		if strings.HasPrefix(segment, ":") {
			params[strings.TrimPrefix(segment, ":")] = segments[i]
			continue
		}
		if strings.HasPrefix(segment, "*") {
			params[strings.TrimPrefix(segment, "*")] = strings.Join(segments[i:], "/")
			return params, true
		}
		if segments[i] != segment {
			return nil, false
		}
	}

	return params, true
}

func routeEntryScore(r *routeEntry) int {
	score := len(r.segments)
	for _, segment := range r.segments {
		if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			score -= 1
		}
	}
	return score
}

func sortRouteEntries(entries []*routeEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		iScore := routeEntryScore(entries[i])
		jScore := routeEntryScore(entries[j])
		if iScore == jScore {
			return len(entries[i].segments) > len(entries[j].segments)
		}
		return iScore > jScore
	})
}

type routeParamsKey struct{}

func contextWithRouteParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, routeParamsKey{}, params)
}

func RouteParams(r *http.Request) map[string]string {
	if params, ok := r.Context().Value(routeParamsKey{}).(map[string]string); ok {
		return params
	}
	return nil
}
