package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const okBody = `{
  "model": "jev-1.13.0",
  "answers": {"move": {"type": "choice", "choice": "right", "probabilities": {"down": 0.3, "right": 0.7}, "confidence": 0.4}},
  "usage": {"input_tokens": 296, "output_tokens": 20}
}`

var question = map[string]Question{"move": {
	Type:         "choice",
	Instructions: "pick a move",
	Criteria:     map[string]string{"down": "slide down", "right": "slide right"},
}}

func newTestClient(t *testing.T, maxCalls int, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Options{Endpoint: srv.URL, APIKey: "test-key", MaxCalls: maxCalls, Backoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAskSendsRequestAndParsesResponse(t *testing.T) {
	var got struct {
		Model     string              `json:"model"`
		State     map[string]any      `json:"state"`
		Questions map[string]Question `json:"questions"`
	}
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if h := r.Header.Get("Authorization"); h != "Bearer test-key" {
			t.Errorf("Authorization = %q", h)
		}
		if h := r.Header.Get("Content-Type"); h != "application/json" {
			t.Errorf("Content-Type = %q", h)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("body: %v", err)
		}
		io.WriteString(w, okBody)
	})

	resp, err := c.Ask(context.Background(), map[string]any{"board": 1}, question)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "jev-latest" || got.State["board"] != float64(1) {
		t.Errorf("request = %+v", got)
	}
	if q := got.Questions["move"]; q.Type != "choice" || q.Criteria["right"] != "slide right" {
		t.Errorf("question = %+v", q)
	}
	ans := resp.Answers["move"]
	if ans.Choice != "right" || ans.Probabilities["right"] != 0.7 || ans.Confidence != 0.4 {
		t.Errorf("answer = %+v", ans)
	}
	if s := c.Stats(); s.Calls != 1 || s.InputTokens != 296 || s.OutputTokens != 20 {
		t.Errorf("stats = %+v", s)
	}
}

func TestAskRetriesOn429And529(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		switch hits.Add(1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(529)
		default:
			io.WriteString(w, okBody)
		}
	})
	if _, err := c.Ask(context.Background(), "s", question); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits = %d, want 3", hits.Load())
	}
	if s := c.Stats(); s.Calls != 1 {
		t.Fatalf("retries counted as calls: %+v", s)
	}
}

func TestAskGivesUpAfterFiveRetries(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(529)
	})
	_, err := c.Ask(context.Background(), "s", question)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 529 {
		t.Fatalf("err = %v, want APIError 529", err)
	}
	if hits.Load() != 6 {
		t.Fatalf("hits = %d, want 6 (1 try + 5 retries)", hits.Load())
	}
}

func TestAskDoesNotRetry401(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"invalid key"}`)
	})
	_, err := c.Ask(context.Background(), "s", question)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 401 || hits.Load() != 1 {
		t.Fatalf("err = %v, hits = %d", err, hits.Load())
	}
}

func TestAskDoesNotRetry422(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"error":"validation failed"}`)
	})
	_, err := c.Ask(context.Background(), "s", question)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 422 || hits.Load() != 1 {
		t.Fatalf("err = %v, hits = %d", err, hits.Load())
	}
}

func TestAskRetriesOnTransientStatuses(t *testing.T) {
	for _, status := range []int{500, 502, 503, 504} {
		t.Run(fmt.Sprintf("%d", status), func(t *testing.T) {
			var hits atomic.Int32
			c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
				if hits.Add(1) == 1 {
					w.WriteHeader(status)
					return
				}
				io.WriteString(w, okBody)
			})
			if _, err := c.Ask(context.Background(), "s", question); err != nil {
				t.Fatal(err)
			}
			if hits.Load() != 2 {
				t.Fatalf("hits = %d, want 2", hits.Load())
			}
			if s := c.Stats(); s.Calls != 1 {
				t.Fatalf("retries counted as calls: %+v", s)
			}
		})
	}
}

func TestAskRetriesOnNetworkError(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("ResponseWriter does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			conn.Close()
			return
		}
		io.WriteString(w, okBody)
	})
	if _, err := c.Ask(context.Background(), "s", question); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", hits.Load())
	}
	if s := c.Stats(); s.Calls != 1 {
		t.Fatalf("retries counted as calls: %+v", s)
	}
}

func TestAskDoesNotRetryWhenContextAlreadyCancelled(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, okBody)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Ask(ctx, "s", question)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if hits.Load() > 1 {
		t.Fatalf("hits = %d, want at most 1", hits.Load())
	}
}

func TestAskEnforcesMaxCalls(t *testing.T) {
	var hits atomic.Int32
	c := newTestClient(t, 2, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, okBody)
	})
	for i := 0; i < 2; i++ {
		if _, err := c.Ask(context.Background(), "s", question); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Ask(context.Background(), "s", question); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", hits.Load())
	}
}

func TestAskStopsWaitingWhenCancelled(t *testing.T) {
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c.backoff = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Ask(ctx, "s", question); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Options{}); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("err = %v, want ErrNoAPIKey", err)
	}
}

func TestErrorsNeverContainTheKey(t *testing.T) {
	c := newTestClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":"unauthorized","received_header":%q}`, r.Header.Get("Authorization"))
	})
	_, err := c.Ask(context.Background(), "s", question)
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Fatalf("err leaks the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("err does not show a redaction marker: %v", err)
	}
}
