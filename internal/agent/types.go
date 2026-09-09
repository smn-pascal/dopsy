package agent

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Complete(context.Context, CompletionRequest) (CompletionResponse, error)
}

type CompletionRequest struct {
	Messages []Message
	Tools    []ToolDefinition
}

type CompletionResponse struct {
	Content   string
	ToolCalls []ToolCall
}

type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}
