package exporter

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	poller *Poller

	currentIOsDesc             *prometheus.Desc
	currentLatencyDesc         *prometheus.Desc
	currentThroughputDesc      *prometheus.Desc
	writeCacheHitDesc          *prometheus.Desc
	readRatioDesc              *prometheus.Desc
	readCacheHitDesc           *prometheus.Desc
	totalIOsDesc               *prometheus.Desc
	lastRefreshSuccessDesc     *prometheus.Desc
	lastRefreshTimestampDesc   *prometheus.Desc
	lastRefreshDurationDesc    *prometheus.Desc
	lastRefreshErrorDesc       *prometheus.Desc
	snapshotAgeDesc            *prometheus.Desc
	devicesDesc                *prometheus.Desc
}

func NewCollector(poller *Poller) *Collector {
	return &Collector{
		poller: poller,
		currentIOsDesc: prometheus.NewDesc(
			"dell_md_current_ios_per_second",
			"Current IO operations per second reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		currentLatencyDesc: prometheus.NewDesc(
			"dell_md_current_io_latency_seconds",
			"Current IO latency reported by Dell MD, converted from milliseconds to seconds.",
			[]string{"device"},
			nil,
		),
		currentThroughputDesc: prometheus.NewDesc(
			"dell_md_current_throughput_megabytes_per_second",
			"Current throughput in megabytes per second reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		writeCacheHitDesc: prometheus.NewDesc(
			"dell_md_primary_write_cache_hit_ratio",
			"Primary write cache hit ratio reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		readRatioDesc: prometheus.NewDesc(
			"dell_md_read_ratio",
			"Read ratio reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		readCacheHitDesc: prometheus.NewDesc(
			"dell_md_primary_read_cache_hit_ratio",
			"Primary read cache hit ratio reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		totalIOsDesc: prometheus.NewDesc(
			"dell_md_total_ios",
			"Total IO operations reported by Dell MD.",
			[]string{"device"},
			nil,
		),
		lastRefreshSuccessDesc: prometheus.NewDesc(
			"dell_md_exporter_last_refresh_success",
			"Whether the most recent refresh completed successfully.",
			nil,
			nil,
		),
		lastRefreshTimestampDesc: prometheus.NewDesc(
			"dell_md_exporter_last_refresh_timestamp_seconds",
			"Unix timestamp of the last successful refresh.",
			nil,
			nil,
		),
		lastRefreshDurationDesc: prometheus.NewDesc(
			"dell_md_exporter_last_refresh_duration_seconds",
			"Duration of the most recent refresh attempt.",
			nil,
			nil,
		),
		lastRefreshErrorDesc: prometheus.NewDesc(
			"dell_md_exporter_last_refresh_error",
			"Whether the most recent refresh attempt ended with an error.",
			nil,
			nil,
		),
		snapshotAgeDesc: prometheus.NewDesc(
			"dell_md_exporter_snapshot_age_seconds",
			"Age of the most recent successful snapshot.",
			nil,
			nil,
		),
		devicesDesc: prometheus.NewDesc(
			"dell_md_exporter_devices",
			"Number of devices exposed by the current snapshot.",
			nil,
			nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.currentIOsDesc
	ch <- c.currentLatencyDesc
	ch <- c.currentThroughputDesc
	ch <- c.writeCacheHitDesc
	ch <- c.readRatioDesc
	ch <- c.readCacheHitDesc
	ch <- c.totalIOsDesc
	ch <- c.lastRefreshSuccessDesc
	ch <- c.lastRefreshTimestampDesc
	ch <- c.lastRefreshDurationDesc
	ch <- c.lastRefreshErrorDesc
	ch <- c.snapshotAgeDesc
	ch <- c.devicesDesc
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	snapshot := c.poller.Snapshot()

	for _, device := range snapshot.Devices {
		mustEmitGauge(ch, c.currentIOsDesc, device.CurrentIOsPerSecond, device.Name)
		mustEmitGauge(ch, c.currentLatencyDesc, device.CurrentIOLatencyMS/1000, device.Name)
		mustEmitGauge(ch, c.currentThroughputDesc, device.CurrentMBPerSecond, device.Name)
		mustEmitGauge(ch, c.writeCacheHitDesc, percentageToRatio(device.PrimaryWriteCacheHit), device.Name)
		mustEmitGauge(ch, c.readRatioDesc, percentageToRatio(device.ReadPercent), device.Name)
		mustEmitGauge(ch, c.readCacheHitDesc, percentageToRatio(device.PrimaryReadCacheHit), device.Name)
		mustEmitGauge(ch, c.totalIOsDesc, device.TotalIOs, device.Name)
	}

	success := 0.0
	if snapshot.LastRefreshSuccessful {
		success = 1
	}

	lastError := 0.0
	if snapshot.LastAttempt != (time.Time{}) && !snapshot.LastRefreshSuccessful {
		lastError = 1
	}

	timestamp := math.NaN()
	age := math.NaN()
	if !snapshot.LastSuccess.IsZero() {
		timestamp = float64(snapshot.LastSuccess.Unix())
		age = time.Since(snapshot.LastSuccess).Seconds()
	}

	ch <- prometheus.MustNewConstMetric(c.lastRefreshSuccessDesc, prometheus.GaugeValue, success)
	ch <- prometheus.MustNewConstMetric(c.lastRefreshTimestampDesc, prometheus.GaugeValue, timestamp)
	ch <- prometheus.MustNewConstMetric(c.lastRefreshDurationDesc, prometheus.GaugeValue, snapshot.LastDuration.Seconds())
	ch <- prometheus.MustNewConstMetric(c.lastRefreshErrorDesc, prometheus.GaugeValue, lastError)
	ch <- prometheus.MustNewConstMetric(c.snapshotAgeDesc, prometheus.GaugeValue, age)
	ch <- prometheus.MustNewConstMetric(c.devicesDesc, prometheus.GaugeValue, float64(len(snapshot.Devices)))
}

func percentageToRatio(value float64) float64 {
	if math.IsNaN(value) {
		return math.NaN()
	}

	return value / 100
}

func mustEmitGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labelValues ...string) {
	if math.IsNaN(value) {
		return
	}

	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
}
