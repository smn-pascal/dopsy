package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/smn-pascal/dopsy/internal/dockergateway"
	"github.com/smn-pascal/dopsy/internal/domain"
	"github.com/smn-pascal/dopsy/internal/session"
)

const (
	maxChatRequestBytes        = 16 * 1024
	maxConversationIDBytes     = 64
	hardMaxConcurrentDiagnoses = 16
)

var (
	containerIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	conversationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`)
)

type Diagnoser interface {
	Diagnose(context.Context, string, string) (domain.Diagnosis, error)
}

type contextualDiagnoser interface {
	DiagnoseWithHistory(context.Context, string, string, []session.Turn) (domain.Diagnosis, error)
}

type Options struct {
	AIConfigured           bool
	AIModel                string
	WebDir                 string
	RequestTimeout         time.Duration
	ConversationTTL        time.Duration
	MaxConversations       int
	MaxConversationTurns   int
	MaxConversationBytes   int
	MaxConcurrentDiagnoses int
	SessionStore           *session.Store
}

type API struct {
	docker         dockergateway.Gateway
	diagnoser      Diagnoser
	aiConfigured   bool
	aiModel        string
	webDir         string
	requestTimeout time.Duration
	sessions       *session.Store
	diagnosisSlots chan struct{}
}

func New(docker dockergateway.Gateway, diagnoser Diagnoser, options Options) http.Handler {
	timeout := options.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	store := options.SessionStore
	if store == nil {
		store = session.New(session.Options{
			TTL:         options.ConversationTTL,
			MaxSessions: options.MaxConversations,
			MaxTurns:    options.MaxConversationTurns,
			MaxBytes:    options.MaxConversationBytes,
		})
	}
	maxConcurrent := options.MaxConcurrentDiagnoses
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	} else if maxConcurrent > hardMaxConcurrentDiagnoses {
		maxConcurrent = hardMaxConcurrentDiagnoses
	}
	api := &API{
		docker:         docker,
		diagnoser:      diagnoser,
		aiConfigured:   options.AIConfigured,
		aiModel:        options.AIModel,
		webDir:         options.WebDir,
		requestTimeout: timeout,
		sessions:       store,
		diagnosisSlots: make(chan struct{}, maxConcurrent),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.health)
	mux.HandleFunc("/api/containers", api.containers)
	mux.HandleFunc("/api/chat", api.chat)
	mux.HandleFunc("/api/", api.notFound)
	mux.HandleFunc("/", api.static)
	return api.middleware(mux)
}

func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(request.URL.Path, "/api/") {
			writer.Header().Set("Cache-Control", "no-store")
		}
		if strings.HasPrefix(request.URL.Path, "/api/") && !sameOrigin(request) {
			writeError(writer, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), a.requestTimeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (a *API) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	dockerStatus := dockerHealth{Connected: true, Mode: a.docker.Mode()}
	if err := a.docker.Check(ctx); err != nil {
		dockerStatus.Connected = false
		dockerStatus.Message = "Docker is not reachable"
	}
	aiStatus := aiHealth{Configured: a.aiConfigured}
	if a.aiConfigured {
		aiStatus.Model = a.aiModel
	}
	writeJSON(writer, http.StatusOK, healthResponse{Status: "ok", Docker: dockerStatus, AI: aiStatus})
}

func (a *API) containers(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	containers, err := a.docker.ListContainers(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "Docker is not reachable")
		return
	}
	if containers == nil {
		containers = make([]domain.Container, 0)
	}
	writeJSON(writer, http.StatusOK, containers)
}

func (a *API) chat(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(writer, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxChatRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input chatRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	input.ContainerID = strings.TrimSpace(input.ContainerID)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	if input.Message == "" || len(input.Message) > 8000 || !utf8.ValidString(input.Message) {
		writeError(writer, http.StatusBadRequest, "message must contain between 1 and 8000 UTF-8 bytes")
		return
	}
	if input.ContainerID != "" && (!utf8.ValidString(input.ContainerID) || !containerIDPattern.MatchString(input.ContainerID)) {
		writeError(writer, http.StatusBadRequest, "invalid containerId")
		return
	}
	if input.ConversationID != "" && (len(input.ConversationID) > maxConversationIDBytes || !utf8.ValidString(input.ConversationID) || !conversationIDPattern.MatchString(input.ConversationID)) {
		writeError(writer, http.StatusBadRequest, "invalid conversationId")
		return
	}

	select {
	case a.diagnosisSlots <- struct{}{}:
		defer func() { <-a.diagnosisSlots }()
	default:
		writer.Header().Set("Retry-After", "2")
		writeError(writer, http.StatusTooManyRequests, "too many diagnoses are already running")
		return
	}

	conversation, err := a.sessions.Begin(input.ConversationID, input.ContainerID)
	if err != nil {
		a.writeSessionError(writer, err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			a.sessions.Abort(conversation.ID, conversation.Created)
		}
	}()

	var diagnosis domain.Diagnosis
	if contextual, ok := a.diagnoser.(contextualDiagnoser); ok {
		diagnosis, err = contextual.DiagnoseWithHistory(request.Context(), input.Message, conversation.ContainerID, conversation.Turns)
	} else {
		diagnosis, err = a.diagnoser.Diagnose(request.Context(), input.Message, conversation.ContainerID)
	}
	if err != nil {
		writeError(writer, http.StatusBadGateway, "diagnosis could not be completed")
		return
	}
	if diagnosis.Evidence == nil {
		diagnosis.Evidence = make([]domain.Evidence, 0)
	}
	if diagnosis.Steps == nil {
		diagnosis.Steps = make([]domain.Step, 0)
	}
	if err := a.sessions.Commit(conversation.ID, input.Message, diagnosis.Answer); err != nil {
		writeError(writer, http.StatusInternalServerError, "conversation could not be saved")
		return
	}
	committed = true
	writeJSON(writer, http.StatusOK, chatResponse{ConversationID: conversation.ID, Diagnosis: diagnosis})
}

func (a *API) writeSessionError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrNotFound):
		writeError(writer, http.StatusNotFound, "conversation not found or expired")
	case errors.Is(err, session.ErrScopeMismatch):
		writeError(writer, http.StatusConflict, "containerId does not match the conversation scope")
	case errors.Is(err, session.ErrBusy):
		writeError(writer, http.StatusConflict, "a diagnosis is already running for this conversation")
	case errors.Is(err, session.ErrCapacity):
		writer.Header().Set("Retry-After", "2")
		writeError(writer, http.StatusTooManyRequests, "conversation capacity is temporarily exhausted")
	default:
		writeError(writer, http.StatusInternalServerError, "conversation could not be created")
	}
}

func (a *API) notFound(writer http.ResponseWriter, _ *http.Request) {
	writeError(writer, http.StatusNotFound, "API endpoint not found")
}

func (a *API) static(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		methodNotAllowed(writer, http.MethodGet, http.MethodHead)
		return
	}
	if a.webDir == "" {
		http.NotFound(writer, request)
		return
	}
	relative := strings.TrimPrefix(request.URL.Path, "/")
	if relative == "" {
		relative = "index.html"
	}
	if !fs.ValidPath(relative) {
		http.NotFound(writer, request)
		return
	}
	candidate := filepath.Join(a.webDir, filepath.FromSlash(relative))
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		candidate = filepath.Join(a.webDir, "index.html")
		if _, indexErr := os.Stat(candidate); indexErr != nil {
			http.NotFound(writer, request)
			return
		}
	}
	if filepath.Base(candidate) == "index.html" {
		writer.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(writer, request, candidate)
}

func sameOrigin(request *http.Request) bool {
	if strings.EqualFold(request.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}

func methodNotAllowed(writer http.ResponseWriter, methods ...string) {
	writer.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

type chatRequest struct {
	Message        string `json:"message"`
	ContainerID    string `json:"containerId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

type chatResponse struct {
	ConversationID string `json:"conversationId"`
	domain.Diagnosis
}

type healthResponse struct {
	Status string       `json:"status"`
	Docker dockerHealth `json:"docker"`
	AI     aiHealth     `json:"ai"`
}

type dockerHealth struct {
	Connected bool   `json:"connected"`
	Mode      string `json:"mode"`
	Message   string `json:"message,omitempty"`
}

type aiHealth struct {
	Configured bool   `json:"configured"`
	Model      string `json:"model,omitempty"`
}
