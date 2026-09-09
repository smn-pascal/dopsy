package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/smn-pascal/dopsy/internal/agent"
)

func TestCompleteSerializesConversationToolsAndAuth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s, want POST /v1/chat/completions", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "Dopsy/0.1" {
			t.Errorf("User-Agent = %q", got)
		}

		var raw map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if len(raw) != 4 {
			t.Errorf("top-level request fields = %v, want model/messages/tools/tool_choice only", keys(raw))
		}
		var payload apiRequest
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "test-model" || payload.ToolChoice != "auto" {
			t.Errorf("unexpected model/tool choice: %+v", payload)
		}
		if len(payload.Messages) != 4 {
			t.Fatalf("messages = %d, want 4", len(payload.Messages))
		}
		if payload.Messages[0].Role != "system" || payload.Messages[0].Content != "system prompt" {
			t.Errorf("system message = %+v", payload.Messages[0])
		}
		if payload.Messages[1].Role != "user" || payload.Messages[1].Content != "why?" {
			t.Errorf("user message = %+v", payload.Messages[1])
		}
		assistantMessage := payload.Messages[2]
		if assistantMessage.Role != "assistant" || len(assistantMessage.ToolCalls) != 1 || assistantMessage.ToolCalls[0].ID != "call-1" || assistantMessage.ToolCalls[0].Type != "function" || assistantMessage.ToolCalls[0].Function.Name != "inspect_container" || assistantMessage.ToolCalls[0].Function.Arguments != `{"containerId":"abc"}` {
			t.Errorf("assistant tool call = %+v", assistantMessage)
		}
		toolMessage := payload.Messages[3]
		if toolMessage.Role != "tool" || toolMessage.ToolCallID != "call-1" || toolMessage.Content != `{"exitCode":137}` {
			t.Errorf("tool result = %+v", toolMessage)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Type != "function" || payload.Tools[0].Function.Name != "inspect_container" || payload.Tools[0].Function.Description != "Inspect safely" || string(payload.Tools[0].Function.Parameters) != `{"type":"object"}` {
			t.Errorf("tool definition = %+v", payload.Tools)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"Diagnosis complete.","tool_calls":[{"id":"next-1","type":"function","function":{"name":"get_container_logs","arguments":"{\"containerId\":\"abc\",\"tail\":20}"}}]}}]}`)
	}))
	defer server.Close()

	client, err := New(server.URL+"/v1/", "test-secret", "test-model", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Complete(context.Background(), agent.CompletionRequest{
		Messages: []agent.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "why?"},
			{Role: "assistant", ToolCalls: []agent.ToolCall{{ID: "call-1", Name: "inspect_container", Arguments: `{"containerId":"abc"}`}}},
			{Role: "tool", ToolCallID: "call-1", Content: `{"exitCode":137}`},
		},
		Tools: []agent.ToolDefinition{{
			Name: "inspect_container", Description: "Inspect safely", Parameters: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Diagnosis complete." || len(result.ToolCalls) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	call := result.ToolCalls[0]
	if call.ID != "next-1" || call.Name != "get_container_logs" || call.Arguments != `{"containerId":"abc","tail":20}` {
		t.Fatalf("unexpected parsed tool call: %+v", call)
	}
}

func TestNewValidatesBaseURLAndModel(t *testing.T) {
	t.Parallel()
	invalid := []struct {
		name  string
		base  string
		model string
	}{
		{name: "empty base", base: "", model: "model"},
		{name: "empty model", base: "https://example.com/v1", model: ""},
		{name: "relative", base: "example.com/v1", model: "model"},
		{name: "unsupported scheme", base: "ftp://example.com/v1", model: "model"},
		{name: "missing host", base: "https:///v1", model: "model"},
		{name: "userinfo", base: "https://user:password@example.com/v1", model: "model"},
		{name: "query", base: "https://example.com/v1?tenant=secret", model: "model"},
		{name: "fragment", base: "https://example.com/v1#fragment", model: "model"},
	}
	for _, test := range invalid {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(test.base, "", test.model, time.Second); err == nil {
				t.Fatalf("New(%q, model=%q) unexpectedly succeeded", test.base, test.model)
			}
		})
	}

	client, err := New(" https://example.com/v1/ ", "", " model ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "https://example.com/v1/chat/completions" || client.model != "model" || client.client.Timeout != 60*time.Second {
		t.Fatalf("unexpected normalized client: %+v", client)
	}
	client, err = New("https://example.com/v1/chat/completions", "", "model", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "https://example.com/v1/chat/completions" {
		t.Fatalf("completion endpoint was appended twice: %s", client.endpoint)
	}
}

func TestCompleteDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()
	redirectFollowed := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirected" {
			redirectFollowed = true
			_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"unsafe"}}]}`)
			return
		}
		http.Redirect(writer, request, "/redirected", http.StatusFound)
	}))
	defer server.Close()
	client := mustClient(t, server.URL)
	_, err := client.Complete(context.Background(), minimalRequest())
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("error = %v, want HTTP 302", err)
	}
	if redirectFollowed {
		t.Fatal("provider client followed a redirect")
	}
}

func TestCompleteHandlesHTTPAndInvalidResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "HTTP error", status: http.StatusUnauthorized, body: "provider-secret-details", wantError: "HTTP 401"},
		{name: "invalid JSON", status: http.StatusOK, body: `{`, wantError: "decode completion response"},
		{name: "no choices", status: http.StatusOK, body: `{"choices":[]}`, wantError: "contains no choices"},
		{name: "empty message", status: http.StatusOK, body: `{"choices":[{"message":{}}]}`, wantError: "contains no assistant content"},
		{name: "wrong content type", status: http.StatusOK, body: `{"choices":[{"message":{"content":42}}]}`, wantError: "decode completion response"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client := mustClient(t, server.URL)
			_, err := client.Complete(context.Background(), minimalRequest())
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want containing %q", err, test.wantError)
			}
			if strings.Contains(err.Error(), "provider-secret-details") {
				t.Fatal("provider error body leaked into returned error")
			}
		})
	}
}

func TestCompleteRejectsOversizedHTTPResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, strings.Repeat("x", maxProviderResponseBytes+1))
	}))
	defer server.Close()
	client := mustClient(t, server.URL)
	_, err := client.Complete(context.Background(), minimalRequest())
	if err == nil || !strings.Contains(err.Error(), "response exceeds size limit") {
		t.Fatalf("error = %v, want response size error", err)
	}
}

func TestCompleteTruncatesAssistantContentAtUTF8Boundary(t *testing.T) {
	t.Parallel()
	content := strings.Repeat("🙂", maxAssistantContentBytes/4+100)
	server := completionServer(t, map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	defer server.Close()
	client := mustClient(t, server.URL)
	result, err := client.Complete(context.Background(), minimalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) > maxAssistantContentBytes || !utf8.ValidString(result.Content) || !strings.HasSuffix(result.Content, truncationNotice) {
		t.Fatalf("content was not safely truncated: bytes=%d valid=%v suffix=%v", len(result.Content), utf8.ValidString(result.Content), strings.HasSuffix(result.Content, truncationNotice))
	}
}

func TestCompleteLimitsTotalToolArguments(t *testing.T) {
	t.Parallel()
	arguments := strings.Repeat("a", maxToolArgumentsBytes/2+1)
	server := completionServer(t, map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{
			"tool_calls": []any{
				map[string]any{"id": "one", "type": "function", "function": map[string]any{"name": "first", "arguments": arguments}},
				map[string]any{"id": "two", "type": "function", "function": map[string]any{"name": "second", "arguments": arguments}},
			},
		}}},
	})
	defer server.Close()
	client := mustClient(t, server.URL)
	_, err := client.Complete(context.Background(), minimalRequest())
	if err == nil || !strings.Contains(err.Error(), "tool arguments exceed size limit") {
		t.Fatalf("error = %v, want tool argument size error", err)
	}
}

func minimalRequest() agent.CompletionRequest {
	return agent.CompletionRequest{Messages: []agent.Message{{Role: "user", Content: "hello"}}}
}

func mustClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := New(baseURL, "", "test-model", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func completionServer(t *testing.T, response any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Error(err)
		}
	}))
}

func keys(value map[string]json.RawMessage) []string {
	result := make([]string, 0, len(value))
	for key := range value {
		result = append(result, key)
	}
	return result
}
