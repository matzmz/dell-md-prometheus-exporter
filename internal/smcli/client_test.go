package smcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	t             *testing.T
	outputPath    string
	args          []string
	returnErr     error
	fixtureSource string
}

func (f *fakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.args = append([]string(nil), args...)
	if f.returnErr != nil {
		return nil, f.returnErr
	}

	content, err := os.ReadFile(f.fixtureSource)
	if err != nil {
		f.t.Fatalf("read fixture source: %v", err)
	}
	if err := os.WriteFile(f.outputPath, content, 0o600); err != nil {
		f.t.Fatalf("write fake output: %v", err)
	}

	return []byte("ok"), nil
}

func TestClientCollectRunsSMcliAndParsesOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "stats.csv")
	runner := &fakeRunner{
		t:             t,
		outputPath:    outputPath,
		fixtureSource: filepath.Join("testdata", "performance_stats.csv"),
	}

	client := &Client{
		Path:       "/opt/dell/SMcli",
		OutputFile: outputPath,
		Runner:     runner,
	}

	stats, err := client.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned unexpected error: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("unexpected device count: got %d want %d", len(stats), 2)
	}

	args := strings.Join(runner.args, " ")
	if !strings.Contains(args, "localhost") {
		t.Fatalf("expected SMcli args to contain localhost, got %q", args)
	}
	if !strings.Contains(args, outputPath) {
		t.Fatalf("expected SMcli script to contain output path %q, got %q", outputPath, args)
	}
}

func TestClientCollectReturnsRunnerError(t *testing.T) {
	t.Parallel()

	client := &Client{
		Path:       "/opt/dell/SMcli",
		OutputFile: filepath.Join(t.TempDir(), "stats.csv"),
		Runner: &fakeRunner{
			t:         t,
			returnErr: errors.New("boom"),
		},
	}

	if _, err := client.Collect(context.Background()); err == nil {
		t.Fatal("expected Collect to return runner error")
	}
}
