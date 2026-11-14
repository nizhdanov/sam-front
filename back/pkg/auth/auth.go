package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"sam/pkg/apperr"
)

type Role string

const (
	RoleEmployee        Role = "EMPLOYEE"
	RoleAdmin           Role = "ADMIN"
	RoleInventoryAgent  Role = "INVENTORY_AGENT"
	RoleInventoryWorker Role = "INVENTORY_WORKER"
	RoleLicenseService  Role = "LICENSE_SERVICE"
	RoleLLMService      Role = "LLM_SERVICE"
	RoleChatService     Role = "CHAT_SERVICE"
)

var (
	UserRoles    = []Role{RoleEmployee, RoleAdmin}
	ServiceRoles = []Role{RoleInventoryAgent, RoleInventoryWorker, RoleLicenseService, RoleLLMService, RoleChatService}
)

func (r Role) IsUser() bool { return slices.Contains(UserRoles, r) }

func (r Role) IsService() bool { return slices.Contains(ServiceRoles, r) }

const (
	UserIssuer    = "sam-auth"
	ServiceIssuer = "sam-internal"
	Audience      = "sam-backend"
)

type Claims struct {
	jwt.RegisteredClaims
	Role       Role   `json:"role"`
	Username   string `json:"username,omitempty"`
	EmployeeID string `json:"employee_id,omitempty"`
}

func (c *Claims) HasRole(roles ...Role) bool {
	return c != nil && slices.Contains(roles, c.Role)
}

func (c *Claims) IsAdmin() bool { return c.HasRole(RoleAdmin) }

func (c *Claims) Actor() string {
	if c == nil {
		return "system"
	}
	if c.Username != "" {
		return c.Username
	}
	return c.Subject
}

type Verifier struct {
	publicKey     *rsa.PublicKey
	serviceSecret []byte
	parser        *jwt.Parser
}

func NewVerifier(publicKeyPEM []byte, serviceSecret string, leeway time.Duration) (*Verifier, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse jwt public key: %w", err)
	}
	return &Verifier{
		publicKey:     key,
		serviceSecret: []byte(serviceSecret),
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodHS256.Alg()}),
			jwt.WithAudience(Audience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(leeway),
		),
	}, nil
}

func (v *Verifier) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := v.parser.ParseWithClaims(raw, claims, v.key)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.KindUnauthenticated, "invalid or expired token")
	}
	var valid bool
	switch token.Method.Alg() {
	case jwt.SigningMethodRS256.Alg():
		valid = claims.Issuer == UserIssuer && claims.Role.IsUser()
	case jwt.SigningMethodHS256.Alg():
		valid = claims.Issuer == ServiceIssuer && claims.Role.IsService()
	}
	if !valid || claims.Subject == "" {
		return nil, apperr.Unauthenticated("invalid or expired token")
	}
	return claims, nil
}

func (v *Verifier) key(t *jwt.Token) (any, error) {
	switch t.Method.(type) {
	case *jwt.SigningMethodRSA:
		return v.publicKey, nil
	case *jwt.SigningMethodHMAC:
		if len(v.serviceSecret) == 0 {
			return nil, errors.New("service tokens are not accepted")
		}
		return v.serviceSecret, nil
	}
	return nil, errors.New("unexpected signing method")
}

type User struct {
	ID         string
	Username   string
	Role       Role
	EmployeeID string
}

type Signer struct {
	key *rsa.PrivateKey
	ttl time.Duration
	now func() time.Time
}

func NewSigner(privateKeyPEM []byte, ttl time.Duration) (*Signer, error) {
	if ttl <= 0 {
		return nil, errors.New("auth: TTL must be positive")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse jwt private key: %w", err)
	}
	return &Signer{key: key, ttl: ttl, now: time.Now}, nil
}

func (s *Signer) TTL() time.Duration { return s.ttl }

func (s *Signer) Sign(u User) (string, time.Time, error) {
	now := s.now()
	expiresAt := now.Add(s.ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    UserIssuer,
			Subject:   u.ID,
			Audience:  jwt.ClaimStrings{Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role:       u.Role,
		Username:   u.Username,
		EmployeeID: u.EmployeeID,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(s.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return token, expiresAt, nil
}

type ServiceSigner struct {
	secret []byte
	name   string
	role   Role
	ttl    time.Duration
}

func NewServiceSigner(secret, name string, role Role) *ServiceSigner {
	return &ServiceSigner{secret: []byte(secret), name: name, role: role, ttl: 5 * time.Minute}
}

func (s *ServiceSigner) Token() (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ServiceIssuer,
			Subject:   s.name,
			Audience:  jwt.ClaimStrings{Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
		Role: s.role,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

type claimsKey struct{}

type tokenKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

func ClaimsFrom(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey{}).(*Claims)
	return c
}

func WithToken(ctx context.Context, raw string) context.Context {
	return context.WithValue(ctx, tokenKey{}, raw)
}

func TokenFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenKey{}).(string)
	return t, ok && t != ""
}

func Require(ctx context.Context, roles ...Role) (*Claims, error) {
	c := ClaimsFrom(ctx)
	if c == nil {
		return nil, apperr.Unauthenticated("authentication required")
	}
	if !c.HasRole(roles...) {
		return nil, apperr.Forbidden("insufficient permissions")
	}
	return c, nil
}
