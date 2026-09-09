package dockergateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

func NewReadOnlyProxy(socketPath string) http.Handler {
	return NewReadOnlyProxyWithTransport(unixTransport(socketPath))
}

// NewReadOnlyProxyWithTransport is exported primarily so the exact security
// boundary can be integration-tested without a real Docker daemon.
func NewReadOnlyProxyWithTransport(upstream http.RoundTripper) http.Handler {
	target := &url.URL{Scheme: "http", Host: "docker"}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = upstream
	proxy.ModifyResponse = sanitizeProxyResponse
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, _ error) {
		writeProxyError(writer, http.StatusBadGateway, "docker upstream unavailable")
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !AllowedDockerRequest(request) {
			writeProxyError(writer, http.StatusForbidden, "docker endpoint blocked")
			return
		}
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		proxy.ServeHTTP(writer, request)
	})
}

// sanitizeProxyResponse provides a second privacy boundary. Even if the main
// application process is compromised, raw inspect secrets cannot be fetched
// through the sidecar.
func sanitizeProxyResponse(response *http.Response) error {
	if response.Request == nil || response.Request.URL == nil || response.Request.Method != http.MethodGet {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}
	switch classifyDockerPath(normalizedDockerPath(response.Request.URL.Path)) {
	case routeContainerList:
		return sanitizeContainerListResponse(response)
	case routeContainerInspect:
		return sanitizeInspectResponse(response)
	default:
		return nil
	}
}

func sanitizeContainerListResponse(response *http.Response) error {
	body, err := readProxyResponse(response)
	if err != nil {
		return err
	}
	var raw []rawContainer
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("decode container list response: %w", err)
	}
	type safeContainer struct {
		ID      string   `json:"Id"`
		Names   []string `json:"Names"`
		Image   string   `json:"Image"`
		State   string   `json:"State"`
		Status  string   `json:"Status"`
		Created int64    `json:"Created"`
	}
	safe := make([]safeContainer, 0, len(raw))
	for _, container := range raw {
		names := make([]string, 0, 1)
		if len(container.Names) > 0 {
			names = append(names, container.Names[0])
		}
		safe = append(safe, safeContainer{
			ID: container.ID, Names: names, Image: container.Image,
			State: container.State, Status: container.Status, Created: container.Created,
		})
	}
	return replaceProxyResponse(response, safe)
}

func sanitizeInspectResponse(response *http.Response) error {
	body, err := readProxyResponse(response)
	if err != nil {
		return err
	}
	var raw rawInspection
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("decode inspect response: %w", err)
	}

	type safeHealth struct {
		Status string `json:"Status"`
	}
	type safeState struct {
		Status     string      `json:"Status"`
		Running    bool        `json:"Running"`
		OOMKilled  bool        `json:"OOMKilled"`
		ExitCode   int         `json:"ExitCode"`
		StartedAt  string      `json:"StartedAt"`
		FinishedAt string      `json:"FinishedAt"`
		Health     *safeHealth `json:"Health,omitempty"`
	}
	type safeConfig struct {
		Image string `json:"Image"`
	}
	type safeHostConfig struct {
		Memory int64 `json:"Memory"`
	}
	type safeInspection struct {
		ID           string         `json:"Id"`
		Name         string         `json:"Name"`
		RestartCount int            `json:"RestartCount"`
		Config       safeConfig     `json:"Config"`
		State        safeState      `json:"State"`
		HostConfig   safeHostConfig `json:"HostConfig"`
	}
	var health *safeHealth
	if raw.State.Health != nil {
		health = &safeHealth{Status: raw.State.Health.Status}
	}
	safe := safeInspection{
		ID:           raw.ID,
		Name:         raw.Name,
		RestartCount: raw.RestartCount,
		Config:       safeConfig{Image: raw.Config.Image},
		State: safeState{
			Status:     raw.State.Status,
			Running:    raw.State.Running,
			OOMKilled:  raw.State.OOMKilled,
			ExitCode:   raw.State.ExitCode,
			StartedAt:  raw.State.StartedAt,
			FinishedAt: raw.State.FinishedAt,
			Health:     health,
		},
		HostConfig: safeHostConfig{Memory: raw.HostConfig.Memory},
	}
	return replaceProxyResponse(response, safe)
}

func readProxyResponse(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDockerJSONBytes+1))
	_ = response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read Docker response: %w", err)
	}
	if len(body) > maxDockerJSONBytes {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func replaceProxyResponse(response *http.Response, value any) error {
	sanitized, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode sanitized Docker response: %w", err)
	}
	response.Body = io.NopCloser(bytes.NewReader(sanitized))
	response.ContentLength = int64(len(sanitized))
	if response.Header == nil {
		response.Header = make(http.Header)
	}
	response.Header.Set("Content-Length", strconv.Itoa(len(sanitized)))
	response.Header.Set("Content-Type", "application/json")
	return nil
}

func writeProxyError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": message})
}
