package smcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

const (
	defaultTargetHost                = "localhost"
	defaultPerformanceMonitorSeconds = 3
	defaultPerformanceIterations     = 1
)

type Runner interface {
	Run(ctx context.Context, bin string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	return cmd.CombinedOutput()
}

type Client struct {
	Path       string
	OutputFile string
	Runner     Runner
}

func NewClient(path, outputFile string) *Client {
	return &Client{
		Path:       path,
		OutputFile: outputFile,
		Runner:     execRunner{},
	}
}

func (c *Client) Collect(ctx context.Context) ([]DeviceStats, error) {
	if c.Runner == nil {
		c.Runner = execRunner{}
	}

	script := buildScript(c.OutputFile, defaultPerformanceIterations, defaultPerformanceMonitorSeconds)
	if _, err := c.Runner.Run(ctx, c.Path, defaultTargetHost, "-S", "-quick", "-c", script); err != nil {
		return nil, fmt.Errorf("execute smcli: %w", err)
	}

	file, err := os.Open(c.OutputFile)
	if err != nil {
		return nil, fmt.Errorf("open output file: %w", err)
	}
	defer file.Close()

	stats, err := ParsePerformanceStats(file)
	if err != nil {
		return nil, fmt.Errorf("parse output file: %w", err)
	}

	return stats, nil
}

func buildScript(outputFile string, iterations, intervalSeconds int) string {
	if iterations < 1 {
		iterations = 1
	}
	if intervalSeconds < 3 {
		intervalSeconds = 3
	}

	return fmt.Sprintf(
		"set session performanceMonitorInterval=%d; set session performanceMonitorIterations=%d; save storageArray performanceStats file=%q;",
		intervalSeconds,
		iterations,
		outputFile,
	)
}
