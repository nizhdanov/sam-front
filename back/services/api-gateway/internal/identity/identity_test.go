package identity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"sam/pkg/auth"
)

func writeUsers(t *testing.T, users []mockUser) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.json")
	raw, err := json.Marshal(users)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMockAuthenticate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	path := writeUsers(t, []mockUser{{ID: "u1", Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdmin}})
	m, err := LoadMock(path)
	if err != nil {
		t.Fatal(err)
	}

	u, err := m.Authenticate(context.Background(), "admin", "secret")
	if err != nil || u.Role != auth.RoleAdmin {
		t.Fatalf("expected admin, got %+v, %v", u, err)
	}
	if _, err := m.Authenticate(context.Background(), "admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if _, err := m.Authenticate(context.Background(), "ghost", "secret"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoadMockRejectsServiceRole(t *testing.T) {
	path := writeUsers(t, []mockUser{{ID: "u1", Username: "bot", PasswordHash: "x", Role: auth.RoleLLMService}})
	if _, err := LoadMock(path); err == nil {
		t.Fatal("expected error for service role")
	}
}
