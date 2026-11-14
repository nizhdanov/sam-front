// embedconfig writes a service-specific configuration for go:embed.
// The input is mounted as a BuildKit secret and is never printed.
package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type buildConfig struct {
	Common   map[string]string            `json:"common"`
	Services map[string]map[string]string `json:"services"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "embed configuration:", err)
		os.Exit(1)
	}
}

func run() error {
	service := flag.String("service", "", "Compose service name")
	input := flag.String("input", "/run/secrets/backend_config", "BuildKit secret path")
	output := flag.String("output", "pkg/buildconfig/defaults.json", "go:embed input")
	revision := flag.String("revision", "development", "SHA256 of the build secret")
	flag.Parse()
	data, err := os.ReadFile(*input)
	if errors.Is(err, os.ErrNotExist) && *revision == "development" {
		return os.WriteFile(*output, []byte("{}\n"), 0o600)
	}
	if err != nil {
		return errors.New("BACKEND_CONFIG build secret is required")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != *revision {
		return errors.New("BACKEND_CONFIG revision mismatch")
	}
	var config buildConfig
	if json.Unmarshal(data, &config) != nil {
		return errors.New("BACKEND_CONFIG must be a valid JSON object")
	}
	specific, ok := config.Services[*service]
	if !ok {
		return fmt.Errorf("BACKEND_CONFIG.services is missing service %s", *service)
	}
	values := make(map[string]string)
	for key, value := range config.Common {
		values[key] = value
	}
	for key, value := range specific {
		values[key] = value
	}
	// A signing key must never be distributed among the services that verify JWTs.
	if _, exists := values["JWT_PRIVATE_PEM"]; exists {
		return errors.New("JWT_PRIVATE_PEM must not be embedded in backend services")
	}
	for _, key := range []string{"JWT_PUBLIC_PEM", "GIGACHAT_CA_PEM"} {
		if value := values[key]; value != "" {
			values[key] = strings.ReplaceAll(value, `\n`, "\n")
		}
	}
	if value := values["JWT_PUBLIC_PEM"]; value != "" {
		block, _ := pem.Decode([]byte(value))
		if block == nil {
			return errors.New("JWT_PUBLIC_PEM is not a PEM public key")
		}
		if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
			return errors.New("JWT_PUBLIC_PEM is not a valid PKIX public key")
		}
	}
	if value := values["GIGACHAT_CA_PEM"]; value != "" {
		if !x509.NewCertPool().AppendCertsFromPEM([]byte(value)) {
			return errors.New("GIGACHAT_CA_PEM contains no valid certificates")
		}
	}
	data, err = json.Marshal(values)
	if err != nil {
		return errors.New("cannot encode embedded configuration")
	}
	return os.WriteFile(*output, data, 0o600)
}
