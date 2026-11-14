package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sam/pkg/auth"
)

var users = []auth.User{
	{ID: "00000000-0000-4000-8000-000000000001", Username: "employee", Role: auth.RoleEmployee, EmployeeID: "10000000-0000-4000-8000-000000000001"},
	{ID: "00000000-0000-4000-8000-000000000002", Username: "admin", Role: auth.RoleAdmin, EmployeeID: "10000000-0000-4000-8000-000000000002"},
}

func main() {
	dir := flag.String("dir", "secrets", "directory with jwt_private.pem and jwt_public.pem")
	generate := flag.Bool("generate", false, "generate a key pair if it does not exist")
	ttl := flag.Duration("ttl", 30*24*time.Hour, "token lifetime")
	flag.Parse()

	if err := run(*dir, *generate, *ttl); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dir string, generate bool, ttl time.Duration) error {
	privatePath := filepath.Join(dir, "jwt_private.pem")
	if generate {
		return generateKeys(dir, privatePath)
	}
	key, err := os.ReadFile(privatePath)
	if err != nil {
		return fmt.Errorf("read private key (run with -generate first): %w", err)
	}
	signer, err := auth.NewSigner(key, ttl)
	if err != nil {
		return err
	}
	for _, u := range users {
		token, expiresAt, err := signer.Sign(u)
		if err != nil {
			return err
		}
		fmt.Printf("%s (%s), expires %s\n%s\n\n", u.Username, u.Role, expiresAt.Format(time.DateOnly), token)
	}
	return nil
}

func generateKeys(dir, privatePath string) error {
	if _, err := os.Stat(privatePath); err == nil {
		fmt.Println("keys already exist in", dir)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	if err := writePEM(privatePath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), 0o600); err != nil {
		return err
	}
	if err := writePEM(filepath.Join(dir, "jwt_public.pem"), "PUBLIC KEY", public, 0o644); err != nil {
		return err
	}
	fmt.Println("generated RSA key pair in", dir)
	return nil
}

func writePEM(path, kind string, der []byte, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), mode)
}
