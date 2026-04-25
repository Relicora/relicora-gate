package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestLogger(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := log.New(buffer, "", 0)

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("ok"))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
	if !strings.Contains(buffer.String(), "GET /hello") {
		t.Fatalf("expected log to contain request path, got %q", buffer.String())
	}
	if !strings.Contains(buffer.String(), "-> 418") {
		t.Fatalf("expected log to contain status code, got %q", buffer.String())
	}
	if !strings.Contains(buffer.String(), "duration=") {
		t.Fatalf("expected log to contain duration, got %q", buffer.String())
	}
}

func TestRecoverer(t *testing.T) {
	buffer := &bytes.Buffer{}
	logger := log.New(buffer, "", 0)

	handler := Recoverer(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if !strings.Contains(buffer.String(), "panic recovered") {
		t.Fatalf("expected recovery log, got %q", buffer.String())
	}
}

func TestTimeout(t *testing.T) {
	handler := Timeout(20 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("done"))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/slow", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "request timed out") {
		t.Fatalf("expected timeout body, got %q", rr.Body.String())
	}
}
