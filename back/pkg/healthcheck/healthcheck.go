package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sam/pkg/buildconfig"
)

func Probe(defaultAddr string) error {
	values, err := buildconfig.Environment()
	if err != nil {
		return err
	}
	addr := values["HTTP_ADDR"]
	if addr == "" {
		addr = defaultAddr
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz returned %d", resp.StatusCode)
	}
	return nil
}
