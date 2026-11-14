package http

import (
	"errors"
	"net/http"
	"time"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/services/api-gateway/internal/identity"
)

type userResponse struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Role       auth.Role `json:"role"`
	EmployeeID *string   `json:"employee_id"`
}

func toUserResponse(u auth.User) userResponse {
	r := userResponse{ID: u.ID, Username: u.Username, Role: u.Role}
	if u.EmployeeID != "" {
		r.EmployeeID = &u.EmployeeID
	}
	return r
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        userResponse `json:"user"`
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		httpx.Error(w, r, h.log, apperr.Invalid("username and password are required"))
		return
	}
	user, err := h.identity.Authenticate(r.Context(), req.Username, req.Password)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		httpx.Error(w, r, h.log, apperr.Unauthenticated("invalid username or password"))
		return
	}
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	token, expiresAt, err := h.signer.Sign(user)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, loginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.signer.TTL().Seconds()),
		ExpiresAt:   expiresAt,
		User:        toUserResponse(user),
	})
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFrom(r.Context())
	user, err := h.identity.Lookup(r.Context(), claims.Subject)
	if err != nil {
		httpx.Error(w, r, h.log, apperr.Unauthenticated("user no longer exists"))
		return
	}
	httpx.JSON(w, http.StatusOK, toUserResponse(user))
}
