package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/smn-pascal/dopsy/internal/dockergateway"
	"github.com/smn-pascal/dopsy/internal/domain"
	"github.com/smn-pascal/dopsy/internal/session"
)

type diagnoserFunc func(context.Context, string, string) (domain.Diagnosis, error)

func (function diagnoserFunc) Diagnose(ctx context.Context, message, containerID string) (domain.Diagnosis, error) {
	return function(ctx, message, containerID)
}

type contextualDiagnoserFunc func(context.Context, string, string, []session.Turn) (domain.Diagnosis, error)

func (function contextualDiagnoserFunc) Diagnose(ctx context.Context, message, containerID string) (domain.Diagnosis, error) {
	return function(ctx, message, containerID, nil)
}

func (function contextualDiagnoserFunc) DiagnoseWithHistory(ctx context.Context, message, containerID string, history []session.Turn) (domain.Diagnosis, error) {
	return function(ctx, message, containerID, history)
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	diagnoser := diagnoserFunc(func(_ context.Context, message, containerID string) (domain.Diagnosis, error) {
		return domain.Diagnosis{
			Answer:   "diagnosed: " + message + " (" + containerID + ")",
			Evidence: []domain.Evidence{{Label: "Exit code", Value: "137", Severity: "critical"}},
			Steps:    []domain.Step{{Tool: "inspect_container", Summary: "Inspected container state"}},
		}, nil
	})
	return New(dockergateway.NewDemo(), diagnoser, Options{})
}

func TestHealthContract(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body struct {
		Status string `json:"status"`
		Docker struct {
			Connected bool   `json:"connected"`
			Mode      string `json:"mode"`
		} `json:"docker"`
		AI struct {
			Configured bool `json:"configured"`
		} `json:"ai"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || !body.Docker.Connected || body.Docker.Mode != "demo" || body.AI.Configured {
		t.Fatalf("unexpected health response: %+v", body)
	}
}

func TestContainersContract(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "/api/containers", nil)
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var containers []domain.Container
	if err := json.Unmarshal(response.Body.Bytes(), &containers); err != nil {
		t.Fatal(err)
	}
	if len(containers) != 2 || containers[0].ID == "" || containers[0].Created == 0 {
		t.Fatalf("unexpected containers: %+v", containers)
	}
}

func TestChatContract(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"Why?","containerId":"demo-api"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var diagnosis domain.Diagnosis
	if err := json.Unmarshal(response.Body.Bytes(), &diagnosis); err != nil {
		t.Fatal(err)
	}
	if diagnosis.Answer == "" || len(diagnosis.Evidence) != 1 || len(diagnosis.Steps) != 1 {
		t.Fatalf("unexpected diagnosis: %+v", diagnosis)
	}
}

func TestChatRejectsUnknownJSONAndWrongContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        string
		want        int
	}{
		{name: "content type", contentType: "text/plain", body: `{}`, want: http.StatusUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: `{"message":"hi","admin":true}`, want: http.StatusBadRequest},
		{name: "empty message", contentType: "application/json", body: `{"message":"  "}`, want: http.StatusBadRequest},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			testHandler(t).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestAPIRejectsCrossOrigin(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "http://dopsy.local/api/containers", nil)
	request.Host = "dopsy.local"
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	testHandler(t).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestStaticSPAFallback(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	index := directory + "/index.html"
	if err := os.WriteFile(index, []byte("<main>Dopsy</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnoser := diagnoserFunc(func(context.Context, string, string) (domain.Diagnosis, error) {
		return domain.Diagnosis{}, nil
	})
	handler := New(dockergateway.NewDemo(), diagnoser, Options{WebDir: directory})
	request := httptest.NewRequest(http.MethodGet, "/containers/demo-api", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Dopsy") {
		t.Fatalf("SPA fallback failed: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatReturnsConversationIDAndSuppliesServerHistory(t *testing.T) {
	t.Parallel()
	var histories [][]session.Turn
	diagnoser := contextualDiagnoserFunc(func(_ context.Context, message, containerID string, history []session.Turn) (domain.Diagnosis, error) {
		copied := append([]session.Turn(nil), history...)
		histories = append(histories, copied)
		return domain.Diagnosis{Answer: "answer to " + message + " for " + containerID}, nil
	})
	handler := New(dockergateway.NewDemo(), diagnoser, Options{})

	firstRequest := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"First question","containerId":"demo-api"}`))
	firstRequest.Header.Set("Content-Type", "application/json")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d; body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	var first struct {
		ConversationID string `json:"conversationId"`
		Answer         string `json:"answer"`
	}
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.ConversationID == "" {
		t.Fatal("first response did not contain conversationId")
	}

	secondBody := `{"message":"Follow-up","conversationId":"` + first.ConversationID + `"}`
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(secondBody))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("follow-up status = %d; body=%s", secondResponse.Code, secondResponse.Body.String())
	}
	if len(histories) != 2 || len(histories[0]) != 0 || len(histories[1]) != 1 {
		t.Fatalf("unexpected histories: %+v", histories)
	}
	if histories[1][0].User != "First question" || histories[1][0].Assistant != first.Answer {
		t.Fatalf("server-owned history was not supplied: %+v", histories[1])
	}
}

func TestChatRejectsConversationScopeChange(t *testing.T) {
	t.Parallel()
	diagnoser := diagnoserFunc(func(_ context.Context, _, _ string) (domain.Diagnosis, error) {
		return domain.Diagnosis{Answer: "done"}, nil
	})
	handler := New(dockergateway.NewDemo(), diagnoser, Options{})
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"First","containerId":"demo-api"}`))
	firstRequest.Header.Set("Content-Type", "application/json")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	var first struct {
		ConversationID string `json:"conversationId"`
	}
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"Switch","containerId":"demo-database","conversationId":"`+first.ConversationID+`"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", response.Code, response.Body.String())
	}
}

func TestChatReturns429WhenDiagnosisCapacityIsFull(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	diagnoser := diagnoserFunc(func(_ context.Context, _, _ string) (domain.Diagnosis, error) {
		close(started)
		<-release
		return domain.Diagnosis{Answer: "done"}, nil
	})
	handler := New(dockergateway.NewDemo(), diagnoser, Options{MaxConcurrentDiagnoses: 1})

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		request := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"First"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		firstDone <- response
	}()
	<-started

	secondRequest := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"Second"}`))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusTooManyRequests || secondResponse.Header().Get("Retry-After") == "" {
		t.Fatalf("status = %d retry-after=%q, want 429 with Retry-After", secondResponse.Code, secondResponse.Header().Get("Retry-After"))
	}

	close(release)
	if response := <-firstDone; response.Code != http.StatusOK {
		t.Fatalf("first status = %d; body=%s", response.Code, response.Body.String())
	}
}
