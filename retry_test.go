package buttrbase

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_503ThenSuccess(t *testing.T) {
	var calls int32
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Body must be re-readable across retries: a retried POST must still
		// carry its payload.
		b, _ := io.ReadAll(r.Body)
		if len(b) == 0 {
			t.Errorf("attempt %d received empty body", atomic.LoadInt32(&calls))
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"detail": "cold start"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"valid": true})
	})
	c.RetryBaseDelay = time.Millisecond

	res, err := c.ValidateCoupon(context.Background(), "CODE", nil)
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid=true")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", got)
	}
}

func TestRetry_400NoRetry(t *testing.T) {
	var calls int32
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "bad"})
	})
	c.RetryBaseDelay = time.Millisecond

	_, err := c.ValidateCoupon(context.Background(), "CODE", nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	be, ok := err.(*ButtrbaseError)
	if !ok || be.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 ButtrbaseError, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call (no retry on 400), got %d", got)
	}
}

func TestRetry_ExhaustsMaxRetries(t *testing.T) {
	var calls int32
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, http.StatusBadGateway, map[string]any{"detail": "cold start"})
	})
	c.RetryBaseDelay = time.Millisecond
	c.MaxRetries = 2

	_, err := c.ValidateCoupon(context.Background(), "CODE", nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls (1 + 2 retries), got %d", got)
	}
}

func TestRetry_Disabled(t *testing.T) {
	var calls int32
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"detail": "down"})
	})
	c.MaxRetries = 0

	_, err := c.ValidateCoupon(context.Background(), "CODE", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call when retries disabled, got %d", got)
	}
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	d, ok := parseRetryAfter("2")
	if !ok || d != 2*time.Second {
		t.Fatalf("expected 2s, got %v ok=%v", d, ok)
	}
	if _, ok := parseRetryAfter(""); ok {
		t.Fatal("expected empty header to be unparseable")
	}
}
