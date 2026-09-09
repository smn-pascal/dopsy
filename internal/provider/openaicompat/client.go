package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/smn-pascal/dopsy/internal/agent"
)

const (
	maxProviderResponseBytes = 2 * 1024 * 1024
	maxAssistantContentBytes = 64 * 1024
	maxToolArgumentsBytes    = 64 * 1024
	truncationNotice         = "\n\n[Response truncated by Dopsy.]"
)

type Client struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func New(baseURL, apiKey, model string, timeout time.Duration) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	model = strings.TrimSpace(model)
	if baseURL == "" || model == "" {
		return nil, fmt.Errorf("LLM base URL and model are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("invalid LLM base URL")
	}
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		endpoint: endpoint,
		apiKey:   strings.TrimSpace(apiKey),
		model:    model,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) Complete(ctx context.Context, request agent.CompletionRequest) (agent.CompletionResponse, error) {
	payload := apiRequest{
		Model:      c.model,
		Messages:   make([]apiMessage, 0, len(request.Messages)),
		Tools:      make([]apiTool, 0, len(request.Tools)),
		ToolChoice: "auto",
	}
	for _, message := range request.Messages {
		converted := apiMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
		}
		for _, call := range message.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, apiToolCall{
				ID:       call.ID,
				Type:     "function",
				Function: apiFunctionCall{Name: call.Name, Arguments: call.Arguments},
			})
		}
		payload.Messages = append(payload.Messages, converted)
	}
	for _, tool := range request.Tools {
		payload.Tools = append(payload.Tools, apiTool{
			Type: "function",
			Function: apiFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("encode completion request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("create completion request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", "Dopsy/0.1")
	if c.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	response, err := c.client.Do(httpRequest)
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("completion request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return agent.CompletionResponse{}, fmt.Errorf("completion API returned HTTP %d", response.StatusCode)
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, maxProviderResponseBytes+1))
	if err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("read completion response: %w", err)
	}
	if len(encoded) > maxProviderResponseBytes {
		return agent.CompletionResponse{}, fmt.Errorf("completion response exceeds size limit")
	}
	var decoded apiResponse
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return agent.CompletionResponse{}, fmt.Errorf("decode completion response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return agent.CompletionResponse{}, fmt.Errorf("completion response contains no choices")
	}
	choice := decoded.Choices[0].Message
	result := agent.CompletionResponse{Content: boundedAssistantContent(choice.Content)}
	totalArgumentBytes := 0
	for _, call := range choice.ToolCalls {
		if call.Type != "" && call.Type != "function" {
			continue
		}
		if len(call.Function.Arguments) > maxToolArgumentsBytes-totalArgumentBytes {
			return agent.CompletionResponse{}, fmt.Errorf("completion tool arguments exceed size limit")
		}
		totalArgumentBytes += len(call.Function.Arguments)
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	if result.Content == "" && len(result.ToolCalls) == 0 {
		return agent.CompletionResponse{}, fmt.Errorf("completion response contains no assistant content or tool calls")
	}
	return result, nil
}

func boundedAssistantContent(content string) string {
	if len(content) <= maxAssistantContentBytes {
		return content
	}
	limit := maxAssistantContentBytes - len(truncationNotice)
	content = content[:limit]
	for len(content) > 0 && !utf8.ValidString(content) {
		content = content[:len(content)-1]
	}
	return content + truncationNotice
}

type apiRequest struct {
	Model      string       `json:"model"`
	Messages   []apiMessage `json:"messages"`
	Tools      []apiTool    `json:"tools,omitempty"`
	ToolChoice string       `json:"tool_choice,omitempty"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
}

type apiTool struct {
	Type     string                `json:"type"`
	Function apiFunctionDefinition `json:"function"`
}

type apiFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type apiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function apiFunctionCall `json:"function"`
}

type apiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiResponse struct {
	Choices []struct {
		Message struct {
			Content   string        `json:"content"`
			ToolCalls []apiToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}
