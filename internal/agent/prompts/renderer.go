package prompts

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/arifrhm/autonomous-crypto-trading-agent/internal/domain"
)

// TradingPromptData holds the context variables injected into trading prompts.
type TradingPromptData struct {
	Observation       *domain.Observation
	SimilarMemories   []domain.MarketMemory
	MaxPositionUSD    float64
	MinConfidence     float64
}

// SentimentPromptData holds headlines to be analyzed.
type SentimentPromptData struct {
	Symbol    string
	Headlines []string
}

// ReflectionPromptData holds trade execution result and historical context.
type ReflectionPromptData struct {
	ActionResult *domain.ActionResult
	RecentTrades []domain.ExecutedTrade
	WinRate      float64
	TotalPnL     float64
}

// PromptRenderer defines the contract for rendering parameterized LLM prompts.
type PromptRenderer interface {
	RenderTradingSystemPrompt() (string, error)
	RenderTradingUserPrompt(data TradingPromptData) (string, error)
	RenderSentimentSystemPrompt() (string, error)
	RenderSentimentUserPrompt(data SentimentPromptData) (string, error)
	RenderReflectionSystemPrompt() (string, error)
	RenderReflectionUserPrompt(data ReflectionPromptData) (string, error)
}

type templateRenderer struct {
	tradingSystemTmpl    *template.Template
	tradingUserTmpl      *template.Template
	sentimentSystemTmpl  *template.Template
	sentimentUserTmpl    *template.Template
	reflectionSystemTmpl *template.Template
	reflectionUserTmpl   *template.Template
}

// NewPromptRenderer parses and caches all prompt templates at initialization.
func NewPromptRenderer() (PromptRenderer, error) {
	r := &templateRenderer{}

	var err error
	if r.tradingSystemTmpl, err = template.New("trading_system").Parse(TradingSystemPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse trading system prompt: %w", err)
	}
	if r.tradingUserTmpl, err = template.New("trading_user").Parse(TradingUserPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse trading user prompt: %w", err)
	}
	if r.sentimentSystemTmpl, err = template.New("sentiment_system").Parse(SentimentSystemPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse sentiment system prompt: %w", err)
	}
	if r.sentimentUserTmpl, err = template.New("sentiment_user").Parse(SentimentUserPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse sentiment user prompt: %w", err)
	}
	if r.reflectionSystemTmpl, err = template.New("reflection_system").Parse(ReflectionSystemPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse reflection system prompt: %w", err)
	}
	if r.reflectionUserTmpl, err = template.New("reflection_user").Parse(ReflectionPromptTemplate); err != nil {
		return nil, fmt.Errorf("parse reflection user prompt: %w", err)
	}

	return r, nil
}

func (r *templateRenderer) RenderTradingSystemPrompt() (string, error) {
	var buf bytes.Buffer
	if err := r.tradingSystemTmpl.Execute(&buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *templateRenderer) RenderTradingUserPrompt(data TradingPromptData) (string, error) {
	var buf bytes.Buffer
	if err := r.tradingUserTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *templateRenderer) RenderSentimentSystemPrompt() (string, error) {
	var buf bytes.Buffer
	if err := r.sentimentSystemTmpl.Execute(&buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *templateRenderer) RenderSentimentUserPrompt(data SentimentPromptData) (string, error) {
	var buf bytes.Buffer
	if err := r.sentimentUserTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *templateRenderer) RenderReflectionSystemPrompt() (string, error) {
	var buf bytes.Buffer
	if err := r.reflectionSystemTmpl.Execute(&buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *templateRenderer) RenderReflectionUserPrompt(data ReflectionPromptData) (string, error) {
	var buf bytes.Buffer
	if err := r.reflectionUserTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
