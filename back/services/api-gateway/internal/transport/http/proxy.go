package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
)

func newProxy(target string, log *slog.Logger) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)
			pr.Out.Host = u.Host
			pr.SetXForwarded()
			pr.Out.Header.Set(middleware.RequestIDHeader, middleware.GetReqID(pr.In.Context()))
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 16,
			IdleConnTimeout:     90 * time.Second,
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(r.Context().Err(), context.Canceled) {
				return
			}
			kind := apperr.KindUnavailable
			if errors.Is(r.Context().Err(), context.DeadlineExceeded) {
				kind = apperr.KindTimeout
			}
			log.WarnContext(r.Context(), "upstream request failed", "upstream", u.Host, "err", err)
			httpx.Error(w, r, nil, apperr.New(kind, "upstream service unavailable"))
		},
	}, nil
}

func withTimeout(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func limitBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBytes {
			httpx.Error(w, r, nil, apperr.TooLarge("request body is too large"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}
