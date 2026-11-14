package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/httpx"
	"sam/services/chat/internal/service"
)

type Deps struct {
	Log            *slog.Logger
	Verifier       *auth.Verifier
	Health         *health.Checker
	Service        *service.Service
	RequestTimeout time.Duration
}

type handler struct {
	svc *service.Service
	log *slog.Logger
}

func NewRouter(d Deps) http.Handler {
	h := &handler{svc: d.Service, log: d.Log}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(d.Log))
	r.Use(middleware.Recoverer)
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Get("/healthz", d.Health.LiveHandler())
	r.Get("/readyz", d.Health.ReadyHandler())

	r.With(auth.Middleware(d.Verifier)).Get("/api/v1/chat/suggestions", h.suggestions)

	r.Route("/api/v1/chat/conversations", func(r chi.Router) {
		r.Use(auth.Middleware(d.Verifier))
		r.With(middleware.Timeout(d.RequestTimeout)).Group(func(r chi.Router) {
			r.Post("/", h.create)
			r.Get("/", h.list)
			r.Get("/{id}", h.get)
			r.Delete("/{id}", h.delete)
			r.Get("/{id}/messages", h.messages)
		})
		r.Post("/{id}/messages", h.send)
	})
	return r
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if r.ContentLength != 0 {
		if err := httpx.Decode(w, r, &req); err != nil {
			httpx.Error(w, r, h.log, err)
			return
		}
	}
	c, err := h.svc.CreateConversation(r.Context(), req.Title)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	items, total, err := h.svc.ListConversations(r.Context(), page)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, page, total))
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	c, err := h.svc.Conversation(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	if err := h.svc.DeleteConversation(r.Context(), id); err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) messages(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	items, total, err := h.svc.Messages(r.Context(), id, page)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, page, total))
}

func (h *handler) send(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, r, h.log, apperr.New(apperr.KindInternal, "streaming is not supported"))
		return
	}

	stream := &sseStream{w: w, flusher: flusher}
	reply, err := h.svc.Send(r.Context(), id, req.Content, stream)
	if err != nil {
		if !stream.started {
			httpx.Error(w, r, h.log, err)
			return
		}
		if r.Context().Err() == nil {
			h.log.WarnContext(r.Context(), "chat stream failed", "conversation_id", id, "err", err)
			stream.event("error", map[string]string{"code": apperr.KindOf(err).String(), "message": apperr.Message(err)})
		}
		return
	}
	stream.event("message_end", map[string]any{"message_id": reply.ID, "status": reply.Status})
}

type sseStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	started bool
}

func (s *sseStream) Start(messageID uuid.UUID) error {
	h := s.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(http.StatusOK)
	s.started = true
	return s.event("message_start", map[string]any{"message_id": messageID})
}

func (s *sseStream) Delta(text string) error {
	return s.event("delta", map[string]string{"content": text})
}

func (s *sseStream) event(name string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", name, payload); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, apperr.Invalid("conversation id must be a UUID")
	}
	return id, nil
}
