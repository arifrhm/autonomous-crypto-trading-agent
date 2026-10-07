package llm

import (
	"context"
	"time"
)

// Message represents a prompt message in a conversation.
type Message struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// ChatRequest defines options for LLM chat completion.
type ChatRequest struct {
	Messages        []Message `json:"messages"`
	Temperature     float64   `json:"temperature"`
	MaxTokens       int       `json:"max_tokens"`
	ResponseJSON    bool      `json:"response_json"`
	ModelOverride   string    `json:"model_override,omitempty"`
}

// ChatResponse contains LLM completion text and token accounting.
type ChatResponse struct {
	Content      string    `json:"content"`
	PromptTokens int       `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	Latency      time.Duration `json:"latency"`
	Cached       bool      `json:"cached"`
}

// Client defines the interface for calling LLMs and Generating Embeddings.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	CreateEmbedding(ctx context.Context, text string) ([]float32, error)
}
