package core

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

type headerRT struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	for k, v := range h.headers {
		if r.Header.Get(k) == "" {
			r.Header.Set(k, v)
		}
	}
	return h.base.RoundTrip(r)
}

func NewChatModel(ctx context.Context, cnf EnvConfig) (*openai.ChatModel, error) {
	url := normalizeBaseURL(cnf.BaseURL)
	rt := headerRT{
		base: http.DefaultTransport,
		headers: map[string]string{
			"HTTP-Referer":       "http://localhost",
			"X-OpenRouter-Title": "eino-agent-practice",
		},
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: url,
		APIKey:  cnf.OpenRouterKey,
		Model:   cnf.Model,
		ByAzure: false,
		HTTPClient: &http.Client{
			Transport: rt,
			Timeout:   120 * time.Second,
		},
	})
}

func normalizeBaseURL(raw string) string {
	url := strings.TrimSpace(raw)
	if url == "" {
		return "https://openrouter.ai/api/v1"
	}
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, "/chat/completions")
	return url
}
