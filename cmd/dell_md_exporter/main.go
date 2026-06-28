package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	toolkitweb "github.com/prometheus/exporter-toolkit/web"

	"github.com/matzmz/dell_md_exporter/internal/exporter"
	"github.com/matzmz/dell_md_exporter/internal/smcli"
)

const (
	defaultListenAddress = ":9904"
	defaultMetricsPath   = "/metrics"
	defaultSMcliPath     = "/opt/dell/mdstoragesoftware/mdstoragemanager/client/SMcli"
	defaultOutputFile    = "/tmp/dell_md_exporter_stats.csv"
	serverShutdownTimeout = 5 * time.Second
)

type Config struct {
	ListenAddress string
	MetricsPath   string
	WebConfigFile string
	SMcliPath     string
	OutputFile    string
	Interval      time.Duration
	Timeout       time.Duration
	LogLevel      string
}

func (c Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("web.listen-address must not be empty")
	}
	if c.MetricsPath == "" || !strings.HasPrefix(c.MetricsPath, "/") {
		return errors.New("web.telemetry-path must start with /")
	}
	if c.Interval <= 0 {
		return errors.New("collector.interval must be greater than zero")
	}
	if c.Timeout <= 0 {
		return errors.New("collector.timeout must be greater than zero")
	}
	if c.Timeout >= c.Interval {
		return errors.New("collector timeout must be lower than interval")
	}
	if c.SMcliPath == "" {
		return errors.New("smcli.path must not be empty")
	}
	if c.OutputFile == "" {
		return errors.New("smcli.output-file must not be empty")
	}

	return nil
}

func parseFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("dell-md-exporter", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := Config{}
	fs.StringVar(&cfg.ListenAddress, "web.listen-address", defaultListenAddress, "Address to listen on for web interface and telemetry.")
	fs.StringVar(&cfg.MetricsPath, "web.telemetry-path", defaultMetricsPath, "Path under which to expose metrics.")
	fs.StringVar(&cfg.WebConfigFile, "web.config.file", "", "Path to optional exporter-toolkit web config file.")
	fs.StringVar(&cfg.SMcliPath, "smcli.path", defaultSMcliPath, "Path to the Dell SMcli binary.")
	fs.StringVar(&cfg.OutputFile, "smcli.output-file", defaultOutputFile, "Path to the temporary SMcli CSV output file.")
	fs.DurationVar(&cfg.Interval, "collector.interval", time.Minute, "How often to refresh SMcli statistics.")
	fs.DurationVar(&cfg.Timeout, "collector.timeout", 30*time.Second, "Timeout for a single SMcli collection.")
	fs.StringVar(&cfg.LogLevel, "log.level", "info", "Log level: debug, info, warn, error.")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	return cfg, cfg.Validate()
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(2)
	}

	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(2)
	}

	if err := run(cfg, logger); err != nil {
		logger.Error("exporter exited with error", "err", err)
		os.Exit(1)
	}
}

func run(cfg Config, logger *slog.Logger) error {
	if err := validateExecutable(cfg.SMcliPath); err != nil {
		return err
	}

	client := smcli.NewClient(cfg.SMcliPath, cfg.OutputFile)
	poller := exporter.NewPoller(logger, client, cfg.Interval, cfg.Timeout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := poller.Refresh(ctx); err != nil {
		logger.Warn("initial refresh failed", "err", err)
	}
	poller.Start(ctx)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		exporter.NewCollector(poller),
	)

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           newHandler(registry, cfg.MetricsPath),
		ReadHeaderTimeout: 5 * time.Second,
	}

	listenAddresses := []string{cfg.ListenAddress}
	webSystemdSocket := false
	webConfig := cfg.WebConfigFile
	flags := toolkitweb.FlagConfig{
		WebListenAddresses: &listenAddresses,
		WebSystemdSocket:   &webSystemdSocket,
		WebConfigFile:      &webConfig,
	}

	return serveUntilContextDone(
		ctx,
		func() error {
			return toolkitweb.ListenAndServe(server, &flags, logger)
		},
		func(shutdownCtx context.Context) error {
			return server.Shutdown(shutdownCtx)
		},
	)
}

func newHandler(registry *prometheus.Registry, metricsPath string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat smcli binary %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("smcli path %q is a directory", path)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("smcli path %q is not executable", path)
	}

	return nil
}

func newLogger(level string) (*slog.Logger, error) {
	var slogLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", level)
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slogLevel})), nil
}

func serveUntilContextDone(ctx context.Context, serve func() error, shutdown func(context.Context) error) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- serve()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()

		shutdownErr := shutdown(shutdownCtx)
		err := <-errCh
		if shutdownErr != nil {
			return shutdownErr
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
