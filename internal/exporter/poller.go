package exporter

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/matzmz/dell_md_exporter/internal/smcli"
)

type StatsCollector interface {
	Collect(ctx context.Context) ([]smcli.DeviceStats, error)
}

type Snapshot struct {
	Devices               []smcli.DeviceStats
	LastAttempt           time.Time
	LastSuccess           time.Time
	LastDuration          time.Duration
	LastError             string
	LastRefreshSuccessful bool
}

type Poller struct {
	logger    *slog.Logger
	collector StatsCollector
	interval time.Duration
	timeout  time.Duration

	mu       sync.RWMutex
	snapshot Snapshot
}

func NewPoller(logger *slog.Logger, collector StatsCollector, interval, timeout time.Duration) *Poller {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	return &Poller{
		logger:    logger,
		collector: collector,
		interval:  interval,
		timeout:   timeout,
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.Refresh(ctx); err != nil {
					p.logger.Warn("periodic refresh failed", "err", err)
				}
			}
		}
	}()
}

func (p *Poller) Refresh(ctx context.Context) error {
	refreshCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	startedAt := time.Now()
	devices, err := p.collector.Collect(refreshCtx)
	finishedAt := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.snapshot.LastAttempt = startedAt
	p.snapshot.LastDuration = finishedAt.Sub(startedAt)
	if err != nil {
		p.snapshot.LastError = err.Error()
		p.snapshot.LastRefreshSuccessful = false
		return err
	}

	p.snapshot.Devices = append([]smcli.DeviceStats(nil), devices...)
	p.snapshot.LastSuccess = finishedAt
	p.snapshot.LastError = ""
	p.snapshot.LastRefreshSuccessful = true
	return nil
}

func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshot := p.snapshot
	snapshot.Devices = append([]smcli.DeviceStats(nil), p.snapshot.Devices...)

	return snapshot
}
