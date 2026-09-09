package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smn-pascal/dopsy/internal/agent"
	"github.com/smn-pascal/dopsy/internal/config"
	"github.com/smn-pascal/dopsy/internal/dockergateway"
	"github.com/smn-pascal/dopsy/internal/httpapi"
	"github.com/smn-pascal/dopsy/internal/provider/openaicompat"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Dopsy stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var docker dockergateway.Gateway
	if cfg.DockerMode == "demo" {
		docker = dockergateway.NewDemo()
		slog.Info("using built-in Docker demo data")
	} else {
		engine, err := dockergateway.NewEngine(cfg.DockerHost)
		if err != nil {
			return err
		}
		docker = engine
	}

	var provider agent.Provider
	if cfg.AIConfigured() {
		client, err := openaicompat.New(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.RequestTimeout)
		if err != nil {
			return err
		}
		provider = client
	}
	diagnoser := agent.New(docker, provider, agent.Options{
		MaxRounds:          cfg.MaxAgentSteps,
		MaxToolCalls:       cfg.MaxToolCalls,
		MaxToolOutputBytes: cfg.MaxToolOutputBytes,
		MaxLogLines:        cfg.MaxLogLines,
		MaxLogBytes:        cfg.MaxLogBytes,
		TimezoneName:       cfg.TimezoneName,
		Location:           cfg.Timezone,
	})
	handler := httpapi.New(docker, diagnoser, httpapi.Options{
		AIConfigured:           cfg.AIConfigured(),
		AIModel:                cfg.LLMModel,
		WebDir:                 cfg.WebDir,
		RequestTimeout:         cfg.RequestTimeout,
		ConversationTTL:        cfg.ConversationTTL,
		MaxConversations:       cfg.MaxConversations,
		MaxConversationTurns:   cfg.MaxConversationTurns,
		MaxConversationBytes:   cfg.MaxConversationBytes,
		MaxConcurrentDiagnoses: cfg.MaxConcurrentDiagnoses,
	})
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("Dopsy listening", "address", cfg.ListenAddress, "docker_mode", docker.Mode(), "ai_configured", cfg.AIConfigured())
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownSignal.Done():
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
