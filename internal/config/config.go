package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
	ListenAddress          string
	DockerHost             string
	DockerMode             string
	WebDir                 string
	LLMBaseURL             string
	LLMAPIKey              string
	LLMModel               string
	MaxAgentSteps          int
	MaxToolCalls           int
	MaxToolOutputBytes     int
	MaxLogLines            int
	MaxLogBytes            int
	TimezoneName           string
	Timezone               *time.Location
	ConversationTTL        time.Duration
	MaxConversations       int
	MaxConversationTurns   int
	MaxConversationBytes   int
	MaxConcurrentDiagnoses int
	RequestTimeout         time.Duration
	ShutdownTimeout        time.Duration
}

func Load() (Config, error) {
	dockerHost := firstNonEmpty(os.Getenv("DOPSY_DOCKER_HOST"), os.Getenv("DOCKER_HOST"), "unix:///var/run/docker.sock")
	mode := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("DOPSY_DOCKER_MODE"), "live")))
	if truthy(os.Getenv("DOPSY_DEMO_MODE")) || truthy(os.Getenv("DOPSY_DEMO")) {
		mode = "demo"
	}
	if mode != "demo" {
		mode = "live"
	}
	timezoneName := firstNonEmpty(os.Getenv("DOPSY_TIMEZONE"), "UTC")
	timezone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return Config{}, fmt.Errorf("invalid DOPSY_TIMEZONE %q: %w", timezoneName, err)
	}

	return Config{
		ListenAddress:          firstNonEmpty(os.Getenv("DOPSY_ADDR"), "127.0.0.1:8080"),
		DockerHost:             dockerHost,
		DockerMode:             mode,
		WebDir:                 firstNonEmpty(os.Getenv("DOPSY_WEB_DIR"), "./apps/web/dist"),
		LLMBaseURL:             firstNonEmpty(os.Getenv("DOPSY_LLM_BASE_URL"), "https://api.openai.com/v1"),
		LLMAPIKey:              strings.TrimSpace(os.Getenv("DOPSY_LLM_API_KEY")),
		LLMModel:               strings.TrimSpace(os.Getenv("DOPSY_LLM_MODEL")),
		MaxAgentSteps:          boundedIntEnv("DOPSY_MAX_AGENT_STEPS", 6, 1, 6),
		MaxToolCalls:           boundedIntEnv("DOPSY_MAX_TOOL_CALLS", 10, 1, 12),
		MaxToolOutputBytes:     boundedIntEnv("DOPSY_MAX_TOOL_OUTPUT_BYTES", 512*1024, 16*1024, 1024*1024),
		MaxLogLines:            boundedIntEnv("DOPSY_MAX_LOG_LINES", 2000, 1, 2000),
		MaxLogBytes:            boundedIntEnv("DOPSY_MAX_LOG_BYTES", 256*1024, 1024, 256*1024),
		TimezoneName:           timezoneName,
		Timezone:               timezone,
		ConversationTTL:        boundedDurationEnv("DOPSY_CONVERSATION_TTL", 30*time.Minute, 5*time.Minute, 24*time.Hour),
		MaxConversations:       boundedIntEnv("DOPSY_MAX_CONVERSATIONS", 128, 1, 1024),
		MaxConversationTurns:   boundedIntEnv("DOPSY_MAX_CONVERSATION_TURNS", 12, 1, 32),
		MaxConversationBytes:   boundedIntEnv("DOPSY_MAX_CONVERSATION_BYTES", 128*1024, 16*1024, 512*1024),
		MaxConcurrentDiagnoses: boundedIntEnv("DOPSY_MAX_CONCURRENT_DIAGNOSES", 2, 1, 16),
		RequestTimeout:         durationEnv("DOPSY_REQUEST_TIMEOUT", 60*time.Second),
		ShutdownTimeout:        durationEnv("DOPSY_SHUTDOWN_TIMEOUT", 10*time.Second),
	}, nil
}

func boundedIntEnv(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value < minimum {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func (c Config) AIConfigured() bool {
	return c.LLMBaseURL != "" && c.LLMModel != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func truthy(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func boundedDurationEnv(name string, fallback, minimum, maximum time.Duration) time.Duration {
	parsed := durationEnv(name, fallback)
	if parsed < minimum {
		return fallback
	}
	if parsed > maximum {
		return maximum
	}
	return parsed
}
