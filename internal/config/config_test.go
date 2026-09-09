package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValidatesTimezoneAndUsesSafeListenDefault(t *testing.T) {
	t.Setenv("DOPSY_ADDR", "")
	t.Setenv("DOPSY_TIMEZONE", "Europe/Berlin")

	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ListenAddress != "127.0.0.1:8080" {
		t.Fatalf("listen address = %q, want loopback default", configuration.ListenAddress)
	}
	if configuration.TimezoneName != "Europe/Berlin" || configuration.Timezone == nil {
		t.Fatalf("timezone was not loaded: name=%q location=%v", configuration.TimezoneName, configuration.Timezone)
	}
	_, offset := time.Date(2026, 9, 9, 12, 0, 0, 0, configuration.Timezone).Zone()
	if offset != 2*60*60 {
		t.Fatalf("Berlin offset = %d, want 7200", offset)
	}
}

func TestLoadRejectsInvalidTimezone(t *testing.T) {
	t.Setenv("DOPSY_TIMEZONE", "Not/A-Timezone")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DOPSY_TIMEZONE") {
		t.Fatalf("Load() error = %v, want invalid timezone error", err)
	}
}

func TestLoadBoundsConversationAndAgentBudgets(t *testing.T) {
	t.Setenv("DOPSY_TIMEZONE", "UTC")
	t.Setenv("DOPSY_MAX_TOOL_CALLS", "999")
	t.Setenv("DOPSY_MAX_TOOL_OUTPUT_BYTES", "99999999")
	t.Setenv("DOPSY_CONVERSATION_TTL", "72h")
	t.Setenv("DOPSY_MAX_CONVERSATIONS", "99999")
	t.Setenv("DOPSY_MAX_CONVERSATION_TURNS", "999")
	t.Setenv("DOPSY_MAX_CONVERSATION_BYTES", "99999999")
	t.Setenv("DOPSY_MAX_CONCURRENT_DIAGNOSES", "999")

	configuration, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.MaxToolCalls != 12 || configuration.MaxToolOutputBytes != 1024*1024 {
		t.Fatalf("tool limits not clamped: calls=%d bytes=%d", configuration.MaxToolCalls, configuration.MaxToolOutputBytes)
	}
	if configuration.ConversationTTL != 24*time.Hour || configuration.MaxConversations != 1024 || configuration.MaxConversationTurns != 32 || configuration.MaxConversationBytes != 512*1024 {
		t.Fatalf("conversation limits not clamped: %+v", configuration)
	}
	if configuration.MaxConcurrentDiagnoses != 16 {
		t.Fatalf("concurrency = %d, want 16", configuration.MaxConcurrentDiagnoses)
	}
}
