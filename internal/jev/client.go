// Package jev is a minimal client for TypeSafe AI's System One API.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const (
	DefaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	Model           = "jev-latest"

	maxRetries     = 5
	defaultBackoff = 500 * time.Millisecond
	httpTimeout    = 10 * time.Second
	statusOverload = 529
)

var (
	ErrBudgetExceeded = errors.New("jev: call budget exceeded")
	ErrNoAPIKey       = errors.New("jev: TYPESAFE_API_KEY is not set")
)

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("jev: HTTP %d: %s", e.Status, e.Body)
}

// netError marks a failure from the underlying transport (dialing, writing
// the request, or reading the response) as opposed to request construction
// or JSON decoding. retryable uses this to decide whether it is safe to
// retry the call.
type netError struct{ err error }

func (e *netError) Error() string { return e.err.Error() }
func (e *netError) Unwrap() error { return e.err }

type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

type Options struct {
	Endpoint string        // defaults to DefaultEndpoint
	APIKey   string        // required
	MaxCalls int           // 0 means unlimited
	Backoff  time.Duration // first retry delay; defaults to 500ms
}

type Stats struct {
	Calls        int64
	InputTokens  int64
	OutputTokens int64
}

// Client is safe for concurrent use.
type Client struct {
	endpoint string
	apiKey   string
	maxCalls int64
	backoff  time.Duration
	http     *http.Client

	calls        atomic.Int64
	inputTokens  atomic.Int64
	outputTokens atomic.Int64
}

func New(opts Options) (*Client, error) {
	if opts.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	if opts.Endpoint == "" {
		opts.Endpoint = DefaultEndpoint
	}
	if opts.Backoff <= 0 {
		opts.Backoff = defaultBackoff
	}
	return &Client{
		endpoint: opts.Endpoint,
		apiKey:   opts.APIKey,
		maxCalls: int64(opts.MaxCalls),
		backoff:  opts.Backoff,
		http:     &http.Client{Timeout: httpTimeout},
	}, nil
}

// NewFromEnv reads TYPESAFE_API_KEY and the optional JEV_ENDPOINT.
func NewFromEnv(maxCalls int) (*Client, error) {
	return New(Options{
		Endpoint: os.Getenv("JEV_ENDPOINT"),
		APIKey:   os.Getenv("TYPESAFE_API_KEY"),
		MaxCalls: maxCalls,
	})
}

func (c *Client) Stats() Stats {
	return Stats{Calls: c.calls.Load(), InputTokens: c.inputTokens.Load(), OutputTokens: c.outputTokens.Load()}
}

// Ask counts as one call against the budget no matter how often it retries.
func (c *Client) Ask(ctx context.Context, state any, questions map[string]Question) (Response, error) {
	if n := c.calls.Add(1); c.maxCalls > 0 && n > c.maxCalls {
		c.calls.Add(-1)
		return Response{}, ErrBudgetExceeded
	}
	body, err := json.Marshal(map[string]any{"model": Model, "state": state, "questions": questions})
	if err != nil {
		return Response{}, fmt.Errorf("jev: encode request: %w", err)
	}

	delay := c.backoff
	for attempt := 0; ; attempt++ {
		resp, err := c.post(ctx, body)
		if err == nil {
			c.inputTokens.Add(int64(resp.Usage.InputTokens))
			c.outputTokens.Add(int64(resp.Usage.OutputTokens))
			return resp, nil
		}
		if !retryable(ctx, err) || attempt == maxRetries {
			return Response{}, err
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
}

// retryable reports whether err is worth retrying. It retries transient API
// statuses (429, 5xx, and the vendor-specific 529 "overloaded") and
// transport-level errors, but never when the caller's own ctx is what ended
// the attempt: a 10s http.Client.Timeout is not the caller's context and is
// still retried, but a cancelled or expired ctx is not.
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, statusOverload:
			return true
		default:
			return false
		}
	}
	var ne *netError
	return errors.As(err, &ne)
}

func (c *Client) post(ctx context.Context, body []byte) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("jev: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("jev: send request: %w", &netError{err})
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Response{}, fmt.Errorf("jev: read response: %w", &netError{err})
	}
	if res.StatusCode != http.StatusOK {
		// Redact the key before truncating so a key straddling the 500-byte
		// cut can never survive partially in the reported error.
		body := strings.ReplaceAll(string(data), c.apiKey, "[REDACTED]")
		if len(body) > 500 {
			body = body[:500]
		}
		return Response{}, &APIError{Status: res.StatusCode, Body: body}
	}
	var out Response
	if err := json.Unmarshal(data, &out); err != nil {
		return Response{}, fmt.Errorf("jev: decode response: %w", err)
	}
	return out, nil
}
