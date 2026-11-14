package http

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
)

type limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newLimiter(perMinute, burst int) *limiter {
	return &limiter{visitors: make(map[string]*visitor), rate: rate.Limit(float64(perMinute) / 60), burst: burst}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, v := range l.visitors {
		if now.Sub(v.lastSeen) > 10*time.Minute {
			delete(l.visitors, k)
		}
	}
	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[key] = v
	}
	v.lastSeen = now
	return v.limiter.Allow()
}

func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.allow(ip) {
			httpx.Error(w, r, nil, apperr.RateLimited("too many requests"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
