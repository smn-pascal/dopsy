package dockergateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

const maxDockerJSONBytes = 2 * 1024 * 1024

type Engine struct {
	baseURL string
	client  *http.Client
}

func NewEngine(host string) (*Engine, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("docker host is empty")
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse docker host: %w", err)
	}

	var transport http.RoundTripper
	var baseURL string
	switch parsed.Scheme {
	case "unix":
		if parsed.Path == "" || !filepath.IsAbs(parsed.Path) {
			return nil, fmt.Errorf("docker unix socket must be an absolute path")
		}
		transport = unixTransport(parsed.Path)
		baseURL = "http://docker"
	case "tcp", "http", "https":
		if parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.User != nil {
			return nil, fmt.Errorf("invalid docker TCP URL")
		}
		if parsed.Scheme == "tcp" {
			parsed.Scheme = "http"
		}
		parsed.Path = ""
		parsed.RawPath = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		baseURL = strings.TrimRight(parsed.String(), "/")
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          16,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
		}
	default:
		return nil, fmt.Errorf("unsupported docker host scheme %q", parsed.Scheme)
	}

	return &Engine{
		baseURL: baseURL,
		client: &http.Client{
			Transport: AllowlistTransport{Next: transport},
			Timeout:   12 * time.Second,
		},
	}, nil
}

func (e *Engine) Mode() string { return "live" }

func (e *Engine) Check(ctx context.Context) error {
	body, err := e.get(ctx, "/_ping", 4096)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(body)) != "OK" {
		return fmt.Errorf("unexpected docker ping response")
	}
	return nil
}

func (e *Engine) ListContainers(ctx context.Context) ([]domain.Container, error) {
	body, err := e.get(ctx, "/containers/json?all=1", maxDockerJSONBytes)
	if err != nil {
		return nil, err
	}
	var raw []rawContainer
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode docker container list: %w", err)
	}
	containers := make([]domain.Container, 0, len(raw))
	for _, item := range raw {
		name := shortID(item.ID)
		if len(item.Names) > 0 && strings.TrimPrefix(item.Names[0], "/") != "" {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		containers = append(containers, domain.Container{
			ID:      item.ID,
			Name:    name,
			Image:   item.Image,
			State:   item.State,
			Status:  item.Status,
			Health:  healthFromStatus(item.Status),
			Created: item.Created,
		})
	}
	return containers, nil
}

func (e *Engine) InspectContainer(ctx context.Context, id string) (domain.Inspection, error) {
	if !validContainerID(id) {
		return domain.Inspection{}, ErrInvalidContainer
	}
	body, err := e.get(ctx, "/containers/"+url.PathEscape(id)+"/json", maxDockerJSONBytes)
	if err != nil {
		return domain.Inspection{}, err
	}
	var raw rawInspection
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.Inspection{}, fmt.Errorf("decode docker inspection: %w", err)
	}
	return sanitizeInspect(raw), nil
}

func (e *Engine) ContainerLogs(ctx context.Context, id string, options domain.LogOptions) (domain.Logs, error) {
	if !validContainerID(id) {
		return domain.Logs{}, ErrInvalidContainer
	}
	tail := clampTail(options.Tail)
	query := url.Values{
		"stdout":     {"1"},
		"stderr":     {"1"},
		"timestamps": {"1"},
		"tail":       {strconv.Itoa(tail)},
	}
	if options.Since > 0 {
		query.Set("since", strconv.FormatInt(options.Since, 10))
	}
	if options.Until > 0 {
		query.Set("until", strconv.FormatInt(options.Until, 10))
	}
	response, err := e.openGET(ctx, "/containers/"+url.PathEscape(id)+"/logs?"+query.Encode())
	if err != nil {
		return domain.Logs{}, err
	}
	defer response.Body.Close()
	body, truncated, err := readDockerLogs(response.Body, MaxLogBytes)
	if err != nil {
		return domain.Logs{}, fmt.Errorf("read Docker logs: %w", err)
	}
	return domain.Logs{
		Text:      string(body),
		Tail:      tail,
		Truncated: truncated,
	}, nil
}

func (e *Engine) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	if !validContainerID(id) {
		return domain.Stats{}, ErrInvalidContainer
	}
	body, err := e.get(ctx, "/containers/"+url.PathEscape(id)+"/stats?stream=0", maxDockerJSONBytes)
	if err != nil {
		return domain.Stats{}, err
	}
	var raw rawStats
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.Stats{}, fmt.Errorf("decode docker stats: %w", err)
	}
	return sanitizeStats(raw), nil
}

func (e *Engine) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	body, truncated, err := e.getTruncated(ctx, endpoint, limit)
	if err != nil {
		return nil, err
	}
	if truncated {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func (e *Engine) getTruncated(ctx context.Context, endpoint string, limit int64) ([]byte, bool, error) {
	response, err := e.openGET(ctx, endpoint)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, false, fmt.Errorf("read docker response: %w", err)
	}
	if int64(len(body)) > limit {
		return body[:limit], true, nil
	}
	return body, false, nil
}

func (e *Engine) openGET(ctx context.Context, endpoint string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, e.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create docker request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("docker request failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		return nil, fmt.Errorf("docker API returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

type rawContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Created int64    `json:"Created"`
}

type rawInspection struct {
	ID           string   `json:"Id"`
	Name         string   `json:"Name"`
	RestartCount int      `json:"RestartCount"`
	Path         string   `json:"Path"`
	Args         []string `json:"Args"`
	Config       struct {
		Image      string            `json:"Image"`
		Env        []string          `json:"Env"`
		Cmd        []string          `json:"Cmd"`
		Entrypoint []string          `json:"Entrypoint"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		OOMKilled  bool   `json:"OOMKilled"`
		ExitCode   int    `json:"ExitCode"`
		Error      string `json:"Error"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     *struct {
			Status string `json:"Status"`
			Log    []struct {
				Output string `json:"Output"`
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	HostConfig struct {
		Memory int64 `json:"Memory"`
	} `json:"HostConfig"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

func sanitizeInspect(raw rawInspection) domain.Inspection {
	var health *string
	if raw.State.Health != nil && raw.State.Health.Status != "" {
		value := raw.State.Health.Status
		health = &value
	}
	return domain.Inspection{
		ID:           raw.ID,
		Name:         strings.TrimPrefix(raw.Name, "/"),
		Image:        raw.Config.Image,
		Status:       raw.State.Status,
		Running:      raw.State.Running,
		OOMKilled:    raw.State.OOMKilled,
		ExitCode:     raw.State.ExitCode,
		StartedAt:    raw.State.StartedAt,
		FinishedAt:   raw.State.FinishedAt,
		Health:       health,
		RestartCount: raw.RestartCount,
		MemoryLimit:  raw.HostConfig.Memory,
	}
}

type rawStats struct {
	Read     string `json:"read"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
		Stats struct {
			InactiveFile uint64 `json:"inactive_file"`
		} `json:"stats"`
	} `json:"memory_stats"`
}

func sanitizeStats(raw rawStats) domain.Stats {
	var cpuPercent float64
	var cpuDelta uint64
	if raw.CPUStats.CPUUsage.TotalUsage > raw.PreCPUStats.CPUUsage.TotalUsage {
		cpuDelta = raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage
	}
	var systemDelta uint64
	if raw.CPUStats.SystemCPUUsage > raw.PreCPUStats.SystemCPUUsage {
		systemDelta = raw.CPUStats.SystemCPUUsage - raw.PreCPUStats.SystemCPUUsage
	}
	cores := raw.CPUStats.OnlineCPUs
	if cores == 0 {
		cores = uint32(len(raw.CPUStats.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0 && cpuDelta > 0 && cores > 0 {
		cpuPercent = (float64(cpuDelta) / float64(systemDelta)) * float64(cores) * 100
		cpuPercent = clampPercent(cpuPercent, float64(cores)*100)
	}
	memoryUsage := raw.MemoryStats.Usage
	if raw.MemoryStats.Stats.InactiveFile < memoryUsage {
		memoryUsage -= raw.MemoryStats.Stats.InactiveFile
	} else if raw.MemoryStats.Stats.InactiveFile > 0 {
		memoryUsage = 0
	}
	var memoryPercent float64
	if raw.MemoryStats.Limit > 0 {
		memoryPercent = clampPercent(float64(memoryUsage)/float64(raw.MemoryStats.Limit)*100, 100)
	}
	readAt := time.Now().Unix()
	if parsed, err := time.Parse(time.RFC3339Nano, raw.Read); err == nil {
		readAt = parsed.Unix()
	}
	return domain.Stats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memoryUsage,
		MemoryLimit:   raw.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		ReadAt:        readAt,
	}
}

func healthFromStatus(status string) *string {
	lower := strings.ToLower(status)
	for _, candidate := range []string{"unhealthy", "healthy", "starting"} {
		if strings.Contains(lower, "("+candidate+")") {
			value := candidate
			return &value
		}
	}
	return nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func readDockerLogs(reader io.Reader, limit int64) ([]byte, bool, error) {
	if limit < 1 {
		return nil, false, fmt.Errorf("log limit must be positive")
	}
	buffered := bufio.NewReader(reader)
	header, err := buffered.Peek(8)
	if err != nil && err != io.EOF {
		return nil, false, err
	}
	if len(header) < 8 || !validDockerLogHeader(header) {
		body, err := io.ReadAll(io.LimitReader(buffered, limit+1))
		if err != nil {
			return nil, false, err
		}
		if int64(len(body)) > limit {
			return body[:limit], true, nil
		}
		return body, false, nil
	}

	var output bytes.Buffer
	output.Grow(int(min(limit, 32*1024)))
	for {
		var frameHeader [8]byte
		_, err := io.ReadFull(buffered, frameHeader[:])
		if err == io.EOF {
			return output.Bytes(), false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read Docker log frame header: %w", err)
		}
		if !validDockerLogHeader(frameHeader[:]) {
			return nil, false, fmt.Errorf("invalid Docker log frame header")
		}
		frameLength := int64(binary.BigEndian.Uint32(frameHeader[4:8]))
		remaining := limit - int64(output.Len())
		if frameLength > remaining {
			if remaining > 0 {
				if _, err := io.CopyN(&output, buffered, remaining); err != nil {
					return nil, false, fmt.Errorf("read Docker log frame: %w", err)
				}
			}
			return output.Bytes(), true, nil
		}
		if frameLength > 0 {
			if _, err := io.CopyN(&output, buffered, frameLength); err != nil {
				return nil, false, fmt.Errorf("read Docker log frame: %w", err)
			}
		}
	}
}

func validDockerLogHeader(header []byte) bool {
	return len(header) >= 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0
}

func clampPercent(value, maximum float64) float64 {
	if value < 0 {
		return 0
	}
	if value > maximum {
		return maximum
	}
	return value
}
