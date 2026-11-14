package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGigaChatTokenCacheAndRefresh(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Basic Y2xpZW50OnNlY3JldA==" || r.Header.Get("RqUID") == "" {
			t.Error("invalid OAuth request")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("scope") != "GIGACHAT_API_PERS" {
			t.Error("missing OAuth scope")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_at": time.Now().Add(30 * time.Minute).UnixMilli()})
	}))
	defer server.Close()
	p := NewGigaChat(GigaChatConfig{AuthURL: server.URL, ClientID: "client", ClientSecret: "secret", Scope: "GIGACHAT_API_PERS"}, server.Client())
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			token, err := p.accessToken(context.Background())
			if err != nil || token != "test-token" {
				t.Errorf("token=%q, err=%v", token, err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent requests fetched %d tokens, want 1", calls.Load())
	}
	p.expiresAt = time.Now().Add(30 * time.Second)
	if _, err := p.accessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("token was not refreshed before expiry")
	}
	p.invalidate("old-token")
	if p.token == "" {
		t.Fatal("a stale 401 invalidated a newer token")
	}
}

func TestGigaChatRejectsInvalidOAuthResponse(t *testing.T) {
	for _, body := range []map[string]any{
		{"expires_at": time.Now().Add(time.Hour).UnixMilli()},
		{"access_token": "expired", "expires_at": time.Now().Add(-time.Minute).UnixMilli()},
		{"access_token": "missing-expiry"},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(body)
		}))
		p := NewGigaChat(GigaChatConfig{AuthURL: server.URL, AuthKey: "test"}, server.Client())
		if _, err := p.accessToken(context.Background()); err == nil || p.token != "" {
			t.Error("invalid OAuth response accepted or cached")
		}
		server.Close()
	}
}

func TestGigaChatRefreshesAfterUnauthorized(t *testing.T) {
	var tokens, completions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth" {
			n := tokens.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + strconv.Itoa(int(n)), "expires_at": time.Now().Add(time.Hour).UnixMilli()})
			return
		}
		if completions.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-2" {
			t.Error("retry did not use refreshed token")
		}
		_, _ = w.Write([]byte(`{"model":"GigaChat","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()
	p := NewGigaChat(GigaChatConfig{AuthURL: server.URL + "/oauth", BaseURL: server.URL, AuthKey: "test"}, server.Client())
	res, err := p.Complete(context.Background(), Request{Model: "GigaChat"})
	if err != nil || res.Content != "ok" || tokens.Load() != 2 || completions.Load() != 2 {
		t.Fatalf("response=%+v err=%v tokens=%d completions=%d", res, err, tokens.Load(), completions.Load())
	}
}
