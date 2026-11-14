package auth

import (
	"net/http"
	"strings"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
)

func BearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func Middleware(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := BearerToken(r.Header.Get("Authorization"))
			if !ok {
				httpx.Error(w, r, nil, apperr.Unauthenticated("missing bearer token"))
				return
			}
			claims, err := v.Verify(raw)
			if err != nil {
				httpx.Error(w, r, nil, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithToken(WithClaims(r.Context(), claims), raw)))
		})
	}
}

func RequireRoles(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := Require(r.Context(), roles...); err != nil {
				httpx.Error(w, r, nil, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
