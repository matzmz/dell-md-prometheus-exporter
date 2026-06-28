package exporter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/matzmz/dell_md_exporter/internal/smcli"
)

type fakeStatsCollector struct {
	collectFn func(context.Context) ([]smcli.DeviceStats, error)
}

func (f fakeStatsCollector) Collect(ctx context.Context) ([]smcli.DeviceStats, error) {
	return f.collectFn(ctx)
}

func TestPollerRefreshStoresSuccessfulSnapshot(t *testing.T) {
	t.Parallel()

	poller := NewPoller(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeStatsCollector{
			collectFn: func(context.Context) ([]smcli.DeviceStats, error) {
				return []smcli.DeviceStats{{Name: "Virtual Disk 0", CurrentIOsPerSecond: 1}}, nil
			},
		},
		time.Minute,
		5*time.Second,
	)

	if err := poller.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned unexpected error: %v", err)
	}

	snapshot := poller.Snapshot()
	if !snapshot.LastRefreshSuccessful {
		t.Fatal("expected snapshot to be marked successful")
	}
	if len(snapshot.Devices) != 1 {
		t.Fatalf("unexpected device count: got %d want %d", len(snapshot.Devices), 1)
	}
	if snapshot.LastSuccess.IsZero() {
		t.Fatal("expected LastSuccess to be populated")
	}
}

func TestPollerRefreshKeepsLastGoodSnapshotOnFailure(t *testing.T) {
	t.Parallel()

	collector := &fakeStatsCollector{
		collectFn: func(context.Context) ([]smcli.DeviceStats, error) {
			return []smcli.DeviceStats{{Name: "Virtual Disk 0", CurrentIOsPerSecond: 1}}, nil
		},
	}
	poller := NewPoller(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		collector,
		time.Minute,
		5*time.Second,
	)

	if err := poller.Refresh(context.Background()); err != nil {
		t.Fatalf("first Refresh returned unexpected error: %v", err)
	}

	first := poller.Snapshot()
	collector.collectFn = func(context.Context) ([]smcli.DeviceStats, error) {
		return nil, errors.New("boom")
	}

	if err := poller.Refresh(context.Background()); err == nil {
		t.Fatal("expected second Refresh to fail")
	}

	second := poller.Snapshot()
	if second.LastRefreshSuccessful {
		t.Fatal("expected last refresh to be marked failed")
	}
	if second.LastError == "" {
		t.Fatal("expected snapshot to include last error")
	}
	if len(second.Devices) != 1 {
		t.Fatalf("expected last good snapshot to be retained, got %d devices", len(second.Devices))
	}
	if !second.LastSuccess.Equal(first.LastSuccess) {
		t.Fatalf("expected last success timestamp to be retained, got %v want %v", second.LastSuccess, first.LastSuccess)
	}
}
