package gate

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testServerWait = 2 * time.Second

func TestNewAppDefaultAndOptions(t *testing.T) {
	defaultApp := New()
	if defaultApp.server.Addr != ":8080" {
		t.Fatalf("expected default address :8080, got %q", defaultApp.server.Addr)
	}

	app := New(WithAddr("127.0.0.1:8081"))
	if app.server.Addr != "127.0.0.1:8081" {
		t.Fatalf("expected address 127.0.0.1:8081, got %q", app.server.Addr)
	}

	app = New(WithPort(9090))
	if app.server.Addr != ":9090" {
		t.Fatalf("expected address :9090, got %q", app.server.Addr)
	}

	app = New(WithAddr("127.0.0.1"), WithPort(8080))
	if app.server.Addr != "127.0.0.1:8080" {
		t.Fatalf("expected address 127.0.0.1:8080, got %q", app.server.Addr)
	}

	app = New(WithAddr("127.0.0.1:9090"), WithPort(8080))
	if app.server.Addr != "127.0.0.1:8080" {
		t.Fatalf("expected address 127.0.0.1:8080 when port overrides addr, got %q", app.server.Addr)
	}

	buffer := &bytes.Buffer{}
	logger := log.New(buffer, "", 0)
	app = New(WithLogger(logger))
	if app.logger != logger {
		t.Fatal("WithLogger did not set the custom logger")
	}

	app = New(WithLogger(nil))
	if app.logger == nil {
		t.Fatal("expected default logger when nil is passed to WithLogger")
	}
}

func TestMethodHandler(t *testing.T) {
	called := false
	handler := methodHandler(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/test", nil))

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for wrong method, got %d", rr.Code)
	}
	if called {
		t.Fatal("handler should not have been called for wrong method")
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/test", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct method, got %d", rr.Code)
	}
	if !called {
		t.Fatal("handler was not called for correct method")
	}
}

func TestAppRoutingWithMiddleware(t *testing.T) {
	app := New()
	order := make([]string, 0)

	app.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	})
	app.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	})

	app.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.Write([]byte("world"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Body.String(); got != "world" {
		t.Fatalf("expected body world, got %q", got)
	}
	if strings.Join(order, ",") != "m1,m2,handler" {
		t.Fatalf("expected middleware order m1,m2,handler, got %q", strings.Join(order, ","))
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/hello", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for wrong method, got %d", rr.Code)
	}
}

func TestRouterNestedAndMiddleware(t *testing.T) {
	app := New()

	api := app.NewRouter("/api")
	api.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-API", "true")
			next.ServeHTTP(w, r)
		})
	})

	api.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api"))
	})

	v1 := api.NewRouter("/v1")
	v1.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nested"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/hello", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/hello, got %d", rr.Code)
	}
	if got := rr.Body.String(); got != "api" {
		t.Fatalf("expected body api, got %q", got)
	}
	if got := rr.Header().Get("X-API"); got != "true" {
		t.Fatalf("expected X-API header true, got %q", got)
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/ping, got %d", rr.Code)
	}
	if got := rr.Body.String(); got != "nested" {
		t.Fatalf("expected body nested, got %q", got)
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/hello", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST to /api/hello, got %d", rr.Code)
	}
}

func TestAppRouteMethodCollision(t *testing.T) {
	app := New()
	app.Get("/item", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("get item"))
	})
	app.Post("/item", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create item"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "get item" {
		t.Fatalf("expected GET /item to return get item, got %d %q", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "create item" {
		t.Fatalf("expected POST /item to return create item, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestRouterRouteMethodCollision(t *testing.T) {
	app := New()
	r := app.NewRouter("/api")
	r.Get("/item", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("get item"))
	})
	r.Post("/item", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create item"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "get item" {
		t.Fatalf("expected GET /api/item to return get item, got %d %q", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "create item" {
		t.Fatalf("expected POST /api/item to return create item, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAppHandleFuncAndNormalizeRoute(t *testing.T) {
	app := New()
	app.HandleFunc("item/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("normalized"))
	}, http.MethodGet)

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "normalized" {
		t.Fatalf("expected GET /item to return normalized, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestRouterHandleAndNormalizeRoute(t *testing.T) {
	app := New()
	r := app.NewRouter("api")
	r.Handle("item/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("normalized route"))
	}), http.MethodPost)

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/item", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "normalized route" {
		t.Fatalf("expected POST /api/item to return normalized route, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestRouterHandleFunc(t *testing.T) {
	app := New()
	r := app.NewRouter("api")
	r.HandleFunc("hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello func"))
	}, http.MethodGet)

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/hello", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "hello func" {
		t.Fatalf("expected GET /api/hello to return hello func, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestHandleFuncRegistersAllMethodsWhenMissing(t *testing.T) {
	app := New()
	app.HandleFunc("/multi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Method))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/multi", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != http.MethodGet {
		t.Fatalf("expected GET /multi to be registered, got %d %q", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/multi", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != http.MethodPost {
		t.Fatalf("expected POST /multi to be registered, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestDuplicateMethodRegistrationIsIgnored(t *testing.T) {
	app := New()
	app.HandleFunc("/dup", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("first"))
	}, http.MethodGet)
	app.HandleFunc("/dup", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("second"))
	}, http.MethodGet)

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/dup", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "first" {
		t.Fatalf("expected duplicate GET /dup to keep first handler, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestRouterCustomNotFoundHandler(t *testing.T) {
	app := New()
	r := app.NewRouter("api")
	r.NotFoundHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("router not found"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if rr.Code != http.StatusNotFound || rr.Body.String() != "router not found" {
		t.Fatalf("expected router-level 404 response, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestRouterCustomMethodNotAllowedHandler(t *testing.T) {
	app := New()
	r := app.NewRouter("api")
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	r.MethodNotAllowedHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("router custom 405"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/login", nil))
	if rr.Code != http.StatusMethodNotAllowed || rr.Body.String() != "router custom 405" {
		t.Fatalf("expected router-level 405 response, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAppParameterizedRoute(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(w http.ResponseWriter, r *http.Request) {
		params := RouteParams(r)
		w.Write([]byte(params["id"]))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "42" {
		t.Fatalf("expected GET /users/42 to return 42, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAppWildcardRoute(t *testing.T) {
	app := New()
	app.Get("/files/*path", func(w http.ResponseWriter, r *http.Request) {
		params := RouteParams(r)
		w.Write([]byte(params["path"]))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/files/css/main.css", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "css/main.css" {
		t.Fatalf("expected /files/css/main.css to return css/main.css, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestCustomNotFoundHandler(t *testing.T) {
	app := New()
	app.NotFoundHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("custom not found"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rr.Code != http.StatusNotFound || rr.Body.String() != "custom not found" {
		t.Fatalf("expected custom 404 response, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestCustomMethodNotAllowedHandler(t *testing.T) {
	app := New()
	app.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	app.MethodNotAllowedHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("custom 405"))
	})

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rr.Code != http.StatusMethodNotAllowed || rr.Body.String() != "custom 405" {
		t.Fatalf("expected custom 405 response, got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAppPostPutDeleteRoutes(t *testing.T) {
	app := New()
	app.Post("/post", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("posted"))
	})
	app.Put("/put", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("put"))
	})
	app.Delete("/delete", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("deleted"))
	})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/post", "posted"},
		{http.MethodPut, "/put", "put"},
		{http.MethodDelete, "/delete", "deleted"},
	}

	for _, tt := range tests {
		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s %s, got %d", tt.method, tt.path, rr.Code)
		}
		if got := rr.Body.String(); got != tt.body {
			t.Fatalf("expected body %q for %s %s, got %q", tt.body, tt.method, tt.path, got)
		}
	}
}

func TestRouterPostPutDeleteRoutes(t *testing.T) {
	app := New()
	r := app.NewRouter("/api")
	r.Post("/post", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("posted"))
	})
	r.Put("/put", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("put"))
	})
	r.Delete("/delete", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("deleted"))
	})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/post", "posted"},
		{http.MethodPut, "/api/put", "put"},
		{http.MethodDelete, "/api/delete", "deleted"},
	}

	for _, tt := range tests {
		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s %s, got %d", tt.method, tt.path, rr.Code)
		}
		if got := rr.Body.String(); got != tt.body {
			t.Fatalf("expected body %q for %s %s, got %q", tt.body, tt.method, tt.path, got)
		}
	}
}

func TestAppListenAndServe(t *testing.T) {
	addr := freeAddr(t)
	buffer := &bytes.Buffer{}
	logger := log.New(buffer, "", 0)
	app := New(WithAddr(addr), WithLogger(logger))

	app.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Started", "true")
			next.ServeHTTP(w, r)
		})
	})

	app.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	done := make(chan struct{})
	go func() {
		app.ListenAndServe()
		close(done)
	}()
	defer func() {
		_ = app.server.Close()
		select {
		case <-done:
		case <-time.After(testServerWait):
			t.Fatal("server did not stop")
		}
	}()

	if err := waitForServer(addr, testServerWait); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get("http://" + addr + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /ping, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Started"); got != "true" {
		t.Fatalf("expected X-Started header true, got %q", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Fatalf("expected pong, got %q", string(body))
	}
	if !strings.Contains(buffer.String(), "Server starting") {
		t.Fatalf("expected logger output to contain Server starting, got %q", buffer.String())
	}
}

func freeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("server did not become reachable at %s", addr)
}
