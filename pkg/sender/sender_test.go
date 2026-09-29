package sender

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() { baseBackoff = time.Millisecond }

func TestValidateURL(t *testing.T) {
	ok := []string{
		"https://api.example.com/v1/reports",
		"http://localhost:8080/x",
		"http://127.0.0.1:9000/",
		"http://[::1]:9000/",
	}
	for _, u := range ok {
		if _, err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q): %v", u, err)
		}
	}
	bad := []string{"", "api.example.com", "ftp://example.com", "http://api.example.com/v1", "https://"}
	for _, u := range bad {
		if _, err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) should fail", u)
		}
	}
}

func TestSendSuccessSetsHeaders(t *testing.T) {
	var got http.Header
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"received"}`))
	}))
	defer srv.Close()

	res, err := Send(context.Background(), Request{
		URL: srv.URL, Token: "secret", Body: []byte(`{"a":1}`), Attempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 || res.Attempts != 1 || res.Body != `{"status":"received"}` {
		t.Fatalf("unexpected response: %+v", res)
	}
	if got.Get("Authorization") != "Bearer secret" ||
		got.Get("Content-Type") != "application/json" ||
		len(got.Get("Idempotency-Key")) != 64 || body != `{"a":1}` {
		t.Fatalf("unexpected request: headers=%v body=%q", got, body)
	}
}

func TestSendRetriesWithSameIdempotencyKey(t *testing.T) {
	var calls atomic.Int32
	keys := make(chan string, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys <- r.Header.Get("Idempotency-Key")
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res, err := Send(context.Background(), Request{URL: srv.URL, Body: []byte(`{}`), Attempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts != 3 || calls.Load() != 3 {
		t.Fatalf("attempts=%d calls=%d, want 3", res.Attempts, calls.Load())
	}
	close(keys)
	first := <-keys
	for k := range keys {
		if k != first {
			t.Fatal("idempotency key must be identical across retries")
		}
	}
}

func TestSendDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()

	_, err := Send(context.Background(), Request{URL: srv.URL, Body: []byte(`{}`), Attempts: 5})
	if err == nil || !strings.Contains(err.Error(), "check --token") ||
		!strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestSendGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	res, err := Send(context.Background(), Request{URL: srv.URL, Body: []byte(`{}`), Attempts: 2})
	if err == nil || !strings.Contains(err.Error(), "giving up after 2 attempt(s)") {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Attempts != 2 {
		t.Fatalf("attempts = %d", res.Attempts)
	}
}

func TestSendStopsOnCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	old := baseBackoff
	baseBackoff = time.Hour
	defer func() { baseBackoff = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := Send(ctx, Request{URL: srv.URL, Body: []byte(`{}`), Attempts: 3})
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestSendRejectsLargeBody(t *testing.T) {
	_, err := Send(context.Background(), Request{
		URL: "https://example.com", Body: make([]byte, MaxBodySize+1),
	})
	if err == nil {
		t.Fatal("want size error")
	}
}
