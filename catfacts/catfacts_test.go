package catfacts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/catfacts-cli/catfacts"
)

// newTestClient returns a Client with no pacing and one retry for fast tests.
func newTestClient(baseURL string) *catfacts.Client {
	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 1
	return c
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetFact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fact" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"fact":   "Cats have five toes on their front paws.",
			"length": 40,
		})
	}))
	defer srv.Close()

	// Point client at test server by patching the URL directly in the request.
	// We call Get directly with the test server URL.
	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 0
	body, err := c.Get(context.Background(), srv.URL+"/fact")
	if err != nil {
		t.Fatal(err)
	}
	var f catfacts.Fact
	if err := json.Unmarshal(body, &f); err != nil {
		t.Fatal(err)
	}
	if f.Fact == "" {
		t.Error("Fact is empty")
	}
	if f.Length != 40 {
		t.Errorf("Length = %d, want 40", f.Length)
	}
}

func TestGetFacts(t *testing.T) {
	facts := []catfacts.Fact{
		{Fact: "Fact one.", Length: 9},
		{Fact: "Fact two.", Length: 9},
		{Fact: "Fact three.", Length: 11},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/facts" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		limit := r.URL.Query().Get("limit")
		if limit != "3" {
			t.Errorf("limit = %q, want 3", limit)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"current_page": 1,
			"data":         facts,
			"total":        332,
		})
	}))
	defer srv.Close()

	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 0
	body, err := c.Get(context.Background(), srv.URL+"/facts?limit=3")
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Data []catfacts.Fact `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 3 {
		t.Errorf("got %d facts, want 3", len(resp.Data))
	}
	if resp.Data[0].Fact != "Fact one." {
		t.Errorf("first fact = %q, want Fact one.", resp.Data[0].Fact)
	}
}

func TestGetBreeds(t *testing.T) {
	breeds := []catfacts.Breed{
		{Breed: "Abyssinian", Country: "Ethiopia", Origin: "Natural/Standard", Coat: "Short", Pattern: "Ticked"},
		{Breed: "Bengal", Country: "United States", Origin: "Crossbreed", Coat: "Short", Pattern: "Spotted"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/breeds" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		limit := r.URL.Query().Get("limit")
		if limit != "2" {
			t.Errorf("limit = %q, want 2", limit)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"current_page": 1,
			"data":         breeds,
			"total":        98,
		})
	}))
	defer srv.Close()

	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 0
	body, err := c.Get(context.Background(), srv.URL+"/breeds?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Data []catfacts.Breed `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("got %d breeds, want 2", len(resp.Data))
	}
	if resp.Data[0].Breed != "Abyssinian" {
		t.Errorf("first breed = %q, want Abyssinian", resp.Data[0].Breed)
	}
	if resp.Data[0].Country != "Ethiopia" {
		t.Errorf("country = %q, want Ethiopia", resp.Data[0].Country)
	}
}

func TestGet404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := catfacts.NewClient()
	c.Rate = 0
	c.Retries = 0
	_, err := c.Get(context.Background(), srv.URL+"/notfound")
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := catfacts.NewClient()
	if c.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
	if c.Rate != 200*time.Millisecond {
		t.Errorf("Rate = %v, want 200ms", c.Rate)
	}
	if c.Retries != 5 {
		t.Errorf("Retries = %d, want 5", c.Retries)
	}
}
