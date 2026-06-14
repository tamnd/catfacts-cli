// Package catfacts is the library behind the catfacts command line:
// the HTTP client, request shaping, and the typed data models for catfact.ninja.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
// Build your endpoint calls and JSON decoding on top of it.
package catfacts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultUserAgent identifies the client to catfact.ninja. A real, honest
// User-Agent is both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "catfacts/dev (+https://github.com/tamnd/catfacts-cli)"

// Host is the site this client talks to, and the host the URI driver in
// domain.go claims.
const Host = "catfact.ninja"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to catfact.ninja over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 200ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// Fact is a single cat fact returned by the /fact and /facts endpoints.
type Fact struct {
	Fact   string `kit:"id" json:"fact"`
	Length int    `json:"length"`
}

// Breed is a cat breed returned by the /breeds endpoint.
type Breed struct {
	Breed   string `kit:"id" json:"breed"`
	Country string `json:"country"`
	Origin  string `json:"origin"`
	Coat    string `json:"coat"`
	Pattern string `json:"pattern"`
}

// paginatedFacts is the wire shape of /facts.
type paginatedFacts struct {
	Data []Fact `json:"data"`
}

// paginatedBreeds is the wire shape of /breeds.
type paginatedBreeds struct {
	Data []Breed `json:"data"`
}

// GetFact fetches a single random cat fact.
func (c *Client) GetFact(ctx context.Context) (*Fact, error) {
	body, err := c.Get(ctx, BaseURL+"/fact")
	if err != nil {
		return nil, err
	}
	var f Fact
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("decode fact: %w", err)
	}
	return &f, nil
}

// GetFacts fetches up to limit cat facts from the /facts endpoint.
func (c *Client) GetFacts(ctx context.Context, limit int) ([]Fact, error) {
	url := fmt.Sprintf("%s/facts?limit=%d", BaseURL, limit)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var p paginatedFacts
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decode facts: %w", err)
	}
	return p.Data, nil
}

// GetBreeds fetches up to limit cat breeds from the /breeds endpoint.
func (c *Client) GetBreeds(ctx context.Context, limit int) ([]Breed, error) {
	url := fmt.Sprintf("%s/breeds?limit=%d", BaseURL, limit)
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var p paginatedBreeds
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decode breeds: %w", err)
	}
	return p.Data, nil
}
