package inbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPRegistryClient_parsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": "2026-04-01",
			"verified_seller_eligible": [
				"dsp.example.com",
				"agency.example.com"
			]
		}`))
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL}
	ids, err := c.VerifiedSellerEligible(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dsp.example.com", "agency.example.com"}, ids)
}

func TestHTTPRegistryClient_ignoresUnknownFields(t *testing.T) {
	// Forward-compat: IAB can extend the schema without breaking us.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"verified_seller_eligible": ["a.com"],
			"future_field": {"unrecognized": true}
		}`))
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL}
	ids, err := c.VerifiedSellerEligible(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a.com"}, ids)
}

func TestHTTPRegistryClient_rejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL}
	_, err := c.VerifiedSellerEligible(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 503")
}

func TestHTTPRegistryClient_rejectsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL}
	_, err := c.VerifiedSellerEligible(context.Background())
	require.Error(t, err)
}

func TestHTTPRegistryClient_emptyURL(t *testing.T) {
	c := &HTTPRegistryClient{}
	_, err := c.VerifiedSellerEligible(context.Background())
	require.Error(t, err)
}

func TestHTTPRegistryClient_capsBodySize(t *testing.T) {
	huge := strings.Repeat("x", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Pretend valid JSON but pad past MaxBytes.
		_, _ = w.Write([]byte(`{"verified_seller_eligible":["` + huge + `"]}`))
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL, MaxBytes: 64}
	_, err := c.VerifiedSellerEligible(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestHTTPRegistryClient_respectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	c := &HTTPRegistryClient{URL: srv.URL}
	_, err := c.VerifiedSellerEligible(ctx)
	require.Error(t, err)
}

func TestHTTPRegistryClient_setsHeaders(t *testing.T) {
	var gotAccept, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"verified_seller_eligible":[]}`))
	}))
	defer srv.Close()

	c := &HTTPRegistryClient{URL: srv.URL}
	_, err := c.VerifiedSellerEligible(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "application/json", gotAccept)
	assert.Contains(t, gotUA, "tne-springwire")
}

func TestStaticRegistryClient_returnsCopy(t *testing.T) {
	c := &StaticRegistryClient{IDs: []string{"a", "b"}}
	got, err := c.VerifiedSellerEligible(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got)

	// Mutating the returned slice must not affect the source.
	got[0] = "MUTATED"
	again, _ := c.VerifiedSellerEligible(context.Background())
	assert.Equal(t, []string{"a", "b"}, again)
}
