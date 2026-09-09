package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/smn-pascal/dopsy/internal/dockergateway"
)

func main() {
	var err error
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		err = healthcheck()
	} else if len(os.Args) == 1 {
		err = serve()
	} else {
		err = fmt.Errorf("usage: dopsy-proxy [healthcheck]")
	}
	if err != nil {
		slog.Error("Dopsy proxy stopped", "error", err)
		os.Exit(1)
	}
}

func serve() error {
	address := envOrDefault("DOPSY_PROXY_ADDR", ":2375")
	socket := envOrDefault("DOPSY_DOCKER_SOCKET", "/var/run/docker.sock")
	server := &http.Server{
		Addr:              address,
		Handler:           dockergateway.NewReadOnlyProxy(socket),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("read-only Docker proxy listening", "address", address)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownSignal.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}

func healthcheck() error {
	endpoint := envOrDefault("DOPSY_PROXY_HEALTH_URL", "http://127.0.0.1:2375/_ping")
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || strings.TrimSpace(string(body)) != "OK" {
		return fmt.Errorf("proxy healthcheck failed with HTTP %d", response.StatusCode)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
