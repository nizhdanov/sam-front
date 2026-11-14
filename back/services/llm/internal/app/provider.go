package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"sam/services/llm/internal/config"
	"sam/services/llm/internal/provider"
)

func newProvider(cfg config.Config) (provider.ModelProvider, error) {
	switch cfg.LLM.Provider {
	case "mock":
		return provider.NewMock(), nil
	case "openai_compatible":
		return provider.NewOpenAI(provider.OpenAIConfig{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey}, &http.Client{Timeout: cfg.LLM.Timeout}), nil
	case "gigachat":
		client, err := gigaChatClient(cfg)
		if err != nil {
			return nil, err
		}
		return provider.NewGigaChat(provider.GigaChatConfig{
			AuthURL:      cfg.GigaChat.AuthURL,
			BaseURL:      cfg.GigaChat.BaseURL,
			AuthKey:      cfg.GigaChat.AuthKey,
			ClientID:     cfg.GigaChat.ClientID,
			ClientSecret: cfg.GigaChat.ClientSecret,
			Scope:        cfg.GigaChat.Scope,
		}, client), nil
	}
	return nil, fmt.Errorf("unknown LLM_PROVIDER %q", cfg.LLM.Provider)
}

func gigaChatClient(cfg config.Config) (*http.Client, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if path := cfg.GigaChat.CACertPath; path != "" {
		pem, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read gigachat CA certificate: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("gigachat CA certificate %s contains no PEM certificates", path)
		}
		tlsCfg.RootCAs = pool
	}
	tlsCfg.InsecureSkipVerify = cfg.GigaChat.InsecureSkipVerify
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg
	return &http.Client{Transport: transport, Timeout: cfg.LLM.Timeout + 15*time.Second}, nil
}
