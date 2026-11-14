package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"

	"sam/pkg/auth"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Provider interface {
	Authenticate(ctx context.Context, username, password string) (auth.User, error)
	Lookup(ctx context.Context, id string) (auth.User, error)
}

type mockUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         auth.Role `json:"role"`
	EmployeeID   string    `json:"employee_id,omitempty"`
}

func (u mockUser) user() auth.User {
	return auth.User{ID: u.ID, Username: u.Username, Role: u.Role, EmployeeID: u.EmployeeID}
}

type Mock struct {
	byName map[string]mockUser
	byID   map[string]mockUser
	dummy  []byte
}

func LoadMock(path string) (*Mock, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read users file: %w", err)
	}
	var users []mockUser
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, fmt.Errorf("parse users file: %w", err)
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	m := &Mock{byName: make(map[string]mockUser, len(users)), byID: make(map[string]mockUser, len(users)), dummy: dummy}
	for _, u := range users {
		if u.ID == "" || u.Username == "" || u.PasswordHash == "" {
			return nil, fmt.Errorf("user %q is missing required fields", u.Username)
		}
		if !u.Role.IsUser() {
			return nil, fmt.Errorf("user %q has unsupported role %q", u.Username, u.Role)
		}
		m.byName[u.Username] = u
		m.byID[u.ID] = u
	}
	return m, nil
}

func (m *Mock) Authenticate(_ context.Context, username, password string) (auth.User, error) {
	u, ok := m.byName[username]
	if !ok {
		_ = bcrypt.CompareHashAndPassword(m.dummy, []byte(password))
		return auth.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return auth.User{}, ErrInvalidCredentials
	}
	return u.user(), nil
}

func (m *Mock) Lookup(_ context.Context, id string) (auth.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return auth.User{}, ErrInvalidCredentials
	}
	return u.user(), nil
}
