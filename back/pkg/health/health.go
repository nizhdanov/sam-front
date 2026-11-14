package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type CheckFunc func(ctx context.Context) error

type Checker struct {
	mu      sync.RWMutex
	checks  map[string]CheckFunc
	timeout time.Duration
}

func New(timeout time.Duration) *Checker {
	return &Checker{checks: make(map[string]CheckFunc), timeout: timeout}
}

func (c *Checker) Add(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

func (c *Checker) Check(ctx context.Context) (map[string]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		res = make(map[string]string, len(c.checks))
		ok  = true
	)
	for name, fn := range c.checks {
		wg.Go(func() {
			status := "ok"
			if err := fn(ctx); err != nil {
				status = err.Error()
			}
			mu.Lock()
			defer mu.Unlock()
			res[name] = status
			if status != "ok" {
				ok = false
			}
		})
	}
	wg.Wait()
	return res, ok
}

func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, ok := c.Check(r.Context())
		status, code := "ready", http.StatusOK
		if !ok {
			status, code = "not_ready", http.StatusServiceUnavailable
		}
		writeJSON(w, code, map[string]any{"status": status, "checks": res})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
