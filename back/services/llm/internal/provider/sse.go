package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func streamSSE(ctx context.Context, resp *http.Response) <-chan StreamEvent {
	events := make(chan StreamEvent)
	go func() {
		defer close(events)
		defer func() { _ = resp.Body.Close() }()
		send := func(e StreamEvent) bool {
			select {
			case events <- e:
				return true
			case <-ctx.Done():
				return false
			}
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			data, ok := bytes.CutPrefix(line, []byte("data:"))
			if !ok {
				continue
			}
			data = bytes.TrimSpace(data)
			if bytes.Equal(data, []byte("[DONE]")) {
				return
			}
			var chunk streamChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				send(StreamEvent{Err: fmt.Errorf("decode stream chunk: %w", err)})
				return
			}
			var ev StreamEvent
			if len(chunk.Choices) > 0 {
				ev.Delta = chunk.Choices[0].Delta.Content
			}
			if chunk.Usage != nil {
				ev.Usage = Usage{PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens}
			}
			if ev.Delta == "" && ev.Usage.Total() == 0 {
				continue
			}
			if !send(ev) {
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			send(StreamEvent{Err: fmt.Errorf("read stream: %w", err)})
		}
	}()
	return events
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	_ = resp.Body.Close()
	return &HTTPError{Status: resp.StatusCode, Body: string(body)}
}

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("model provider returned %d: %s", e.Status, e.Body)
}
