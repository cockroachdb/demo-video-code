package llm

import (
	"context"
)

type Client interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type noop struct {
	endpoint string
	key      string
}

func NewClient(endpoint, key string) Client {
	return &noop{endpoint: endpoint, key: key}
}

func (n *noop) Complete(ctx context.Context, prompt string) (string, error) {
	// TODO: replace with actual provider (OpenAI/Claude/local inference)
	return "[LLM placeholder response]", nil
}
