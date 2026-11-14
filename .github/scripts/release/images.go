package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type imageInspection struct {
	ID           string `json:"Id"`
	Size         uint64
	OS           string `json:"Os"`
	Architecture string
	Variant      string
	Config       map[string]any
	RootFS       struct {
		Type   string
		Layers []string
	}
}

func inspectImage(ref string) (imageInspection, error) {
	var images []imageInspection
	data, err := command(true, "docker", "image", "inspect", ref)
	if err != nil {
		return imageInspection{}, fmt.Errorf("inspect image %s: %w", ref, err)
	}
	if json.Unmarshal([]byte(data), &images) != nil || len(images) != 1 || images[0].ID == "" {
		return imageInspection{}, fmt.Errorf("invalid image inspection for %s", ref)
	}
	return images[0], nil
}

func imageFingerprint(image imageInspection) (string, error) {
	if image.OS != "linux" || image.Architecture != "amd64" || image.RootFS.Type != "layers" || image.Config == nil {
		return "", errors.New("expected linux/amd64 image configuration and layer metadata")
	}
	identity := object{
		"os": image.OS, "architecture": image.Architecture, "variant": image.Variant,
		"layers": image.RootFS.Layers, "config": normalizeConfig(image.Config),
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return "", errors.New("cannot encode image content fingerprint")
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func normalizeConfig(value map[string]any) map[string]any {
	result := map[string]any{}
	for key, item := range value {
		if key == "Healthcheck" {
			if healthcheck, ok := item.(map[string]any); ok {
				item = normalizeConfig(healthcheck)
			}
		}
		if !emptyConfigValue(item) {
			result[key] = item
		}
	}
	return result
}

func emptyConfigValue(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case string:
		return value == ""
	case bool:
		return !value
	case float64:
		return value == 0
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	default:
		return false
	}
}

func verifyLoadedImages(m *manifest) error {
	if len(m.Images) == 0 || len(m.ImageFingerprints) != len(m.Images) {
		return errors.New("release lacks portable image fingerprints; build a new release with the updated workflow")
	}
	localIDs := make(map[string]string, len(m.Images))
	for _, ref := range m.Images {
		expected := m.ImageFingerprints[ref]
		if expected == "" {
			return fmt.Errorf("release has no content fingerprint for %s", ref)
		}
		image, err := inspectImage(ref)
		if err != nil {
			return err
		}
		actual, err := imageFingerprint(image)
		if err != nil {
			return fmt.Errorf("fingerprint loaded image %s: %w", ref, err)
		}
		if actual != expected {
			// Tags and digests are safe diagnostics; never log Config or Env.
			return fmt.Errorf("loaded image content mismatch for %s: expected %s, got %s", ref, expected, actual)
		}
		localIDs[ref] = image.ID
	}
	m.ImageIDs = localIDs
	return nil
}
