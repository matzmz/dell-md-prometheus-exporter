package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestValidateConfigRejectsTimeoutGTEInterval(t *testing.T) {
	t.Parallel()

	cfg := Config{
		ListenAddress: defaultListenAddress,
		MetricsPath:   defaultMetricsPath,
		SMcliPath:     "/bin/true",
		OutputFile:    "/tmp/stats.csv",
		Interval:      time.Minute,
		Timeout:       time.Minute,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected config validation to fail when timeout >= interval")
	}
}

func TestNewHandlerRegistersMetricsAndHealthz(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_metric",
		Help: "metric exposed by the handler test",
	})
	gauge.Set(1)
	registry.MustRegister(gauge)

	handler := newHandler(registry, "/metrics")

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthResp := httptest.NewRecorder()
	handler.ServeHTTP(healthResp, healthReq)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("unexpected healthz status code: got %d want %d", healthResp.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(healthResp.Body.String()); body != "ok" {
		t.Fatalf("unexpected healthz body: got %q want %q", body, "ok")
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResp := httptest.NewRecorder()
	handler.ServeHTTP(metricsResp, metricsReq)
	if metricsResp.Code != http.StatusOK {
		t.Fatalf("unexpected metrics status code: got %d want %d", metricsResp.Code, http.StatusOK)
	}
	if body := metricsResp.Body.String(); !strings.Contains(body, "test_metric") {
		t.Fatalf("expected metrics response to contain test metric, got %q", body)
	}
}

func TestServeUntilContextDoneShutsServerDown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownCalled := make(chan struct{}, 1)
	serveReturned := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveUntilContextDone(
			ctx,
			func() error {
				<-shutdownCalled
				close(serveReturned)
				return http.ErrServerClosed
			},
			func(context.Context) error {
				close(shutdownCalled)
				return nil
			},
		)
	}()

	cancel()

	select {
	case <-serveReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("expected serve loop to return after context cancellation")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("serveUntilContextDone returned unexpected error: %v", err)
	}
}

func TestServeUntilContextDoneReturnsShutdownError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownErr := errors.New("shutdown failed")
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveUntilContextDone(
			ctx,
			func() error {
				<-ctx.Done()
				return http.ErrServerClosed
			},
			func(context.Context) error {
				return shutdownErr
			},
		)
	}()

	cancel()

	if err := <-errCh; !errors.Is(err, shutdownErr) {
		t.Fatalf("expected shutdown error %v, got %v", shutdownErr, err)
	}
}
