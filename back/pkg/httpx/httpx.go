package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"sam/pkg/apperr"
)

const MaxBodyBytes = 1 << 20

type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func StatusFor(k apperr.Kind) int {
	switch k {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnauthenticated:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	case apperr.KindRateLimited:
		return http.StatusTooManyRequests
	case apperr.KindTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperr.KindTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func Error(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	kind := apperr.KindOf(err)
	status := StatusFor(kind)
	reqID := middleware.GetReqID(r.Context())
	if status >= http.StatusInternalServerError && log != nil {
		log.ErrorContext(r.Context(), "request failed",
			"err", err, "method", r.Method, "path", r.URL.Path, "request_id", reqID)
	}
	JSON(w, status, ErrorBody{Error: ErrorDetail{
		Code:      kind.String(),
		Message:   apperr.Message(err),
		RequestID: reqID,
	}})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	Error(w, r, nil, apperr.NotFound("route not found"))
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusMethodNotAllowed, ErrorBody{Error: ErrorDetail{
		Code:      "method_not_allowed",
		Message:   "method not allowed",
		RequestID: middleware.GetReqID(r.Context()),
	}})
}

func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperr.Wrap(err, apperr.KindTooLarge, "request body too large")
		}
		return apperr.Wrap(err, apperr.KindInvalid, "invalid JSON body")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Invalid("request body must contain a single JSON object")
	}
	return nil
}

func QueryInt(r *http.Request, name string, def, minV, maxV int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minV || n > maxV {
		return 0, apperr.Invalid(name + " must be an integer between " + strconv.Itoa(minV) + " and " + strconv.Itoa(maxV))
	}
	return n, nil
}

func QueryBool(r *http.Request, name string) (*bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, apperr.Invalid(name + " must be a boolean")
	}
	return &v, nil
}

func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				return
			}
			log.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
