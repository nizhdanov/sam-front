// Package buildconfig provides configuration embedded by the production build.
// An empty embedded configuration preserves the local environment-based setup.
package buildconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed defaults.json
var defaults []byte

func environment(data []byte, environ []string) (map[string]string, error) {
	values := make(map[string]string)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("invalid embedded configuration")
	}
	if len(values) == 0 {
		for _, item := range environ {
			key, value, ok := strings.Cut(item, "=")
			if ok {
				values[key] = value
			}
		}
	}
	return values, nil
}

// Environment returns the sealed production configuration, or the process
// environment for development binaries built without a configuration secret.
func Environment() (map[string]string, error) {
	return environment(defaults, os.Environ())
}

// ReadFile loads public PEM material from the binary in production. Development
// builds and other files continue to use the filesystem.
func ReadFile(path string) ([]byte, error) {
	values, err := Environment()
	if err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{
		{"JWT_PUBLIC_KEY_FILE", "JWT_PUBLIC_PEM"},
		{"GIGACHAT_CA_CERT_PATH", "GIGACHAT_CA_PEM"},
	} {
		if values[pair[0]] == path && values[pair[1]] != "" {
			return []byte(values[pair[1]]), nil
		}
	}
	return os.ReadFile(path)
}
