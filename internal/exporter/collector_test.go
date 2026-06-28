package exporter

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/matzmz/dell_md_exporter/internal/smcli"
)

func TestCollectorCollectExportsDeviceAndHealthMetrics(t *testing.T) {
	t.Parallel()

	poller := NewPoller(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		fakeStatsCollector{
			collectFn: func(context.Context) ([]smcli.DeviceStats, error) {
				return []smcli.DeviceStats{{
					Name:                 "Virtual Disk 0",
					CurrentIOsPerSecond:  12.5,
					CurrentIOLatencyMS:   4,
					CurrentMBPerSecond:   8.2,
					PrimaryWriteCacheHit: 95,
					ReadPercent:          70,
					PrimaryReadCacheHit:  92,
					TotalIOs:             1000,
				}}, nil
			},
		},
		time.Minute,
		5*time.Second,
	)

	if err := poller.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned unexpected error: %v", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(NewCollector(poller))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned unexpected error: %v", err)
	}

	assertMetricFamilyExists(t, metricFamilies, "dell_md_current_ios_per_second")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_current_io_latency_seconds")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_current_throughput_megabytes_per_second")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_primary_write_cache_hit_ratio")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_read_ratio")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_primary_read_cache_hit_ratio")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_total_ios")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_last_refresh_success")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_last_refresh_timestamp_seconds")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_last_refresh_duration_seconds")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_last_refresh_error")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_snapshot_age_seconds")
	assertMetricFamilyExists(t, metricFamilies, "dell_md_exporter_devices")

	assertGaugeValue(t, metricFamilies, "dell_md_current_io_latency_seconds", 0.004)
	assertGaugeValue(t, metricFamilies, "dell_md_primary_write_cache_hit_ratio", 0.95)
	assertGaugeValue(t, metricFamilies, "dell_md_exporter_last_refresh_success", 1)
	assertGaugeValue(t, metricFamilies, "dell_md_exporter_devices", 1)
}

func assertMetricFamilyExists(t *testing.T, families []*dto.MetricFamily, name string) {
	t.Helper()

	for _, family := range families {
		if family.GetName() == name {
			return
		}
	}

	t.Fatalf("expected metric family %q to exist", name)
}

func assertGaugeValue(t *testing.T, families []*dto.MetricFamily, name string, want float64) {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		if len(family.Metric) == 0 {
			t.Fatalf("metric family %q has no metrics", name)
		}
		got := family.Metric[0].GetGauge().GetValue()
		if got != want {
			t.Fatalf("unexpected gauge value for %s: got %v want %v", name, got, want)
		}
		return
	}

	t.Fatalf("expected metric family %q to exist", name)
}
