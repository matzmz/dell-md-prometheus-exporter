package smcli

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePerformanceStatsFiltersExpansionEnclosure(t *testing.T) {
	t.Parallel()

	file := openFixture(t, "performance_stats.csv")

	devices, err := ParsePerformanceStats(file)
	if err != nil {
		t.Fatalf("ParsePerformanceStats returned unexpected error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("unexpected device count: got %d want %d", len(devices), 2)
	}
	for _, device := range devices {
		if device.Name == "Expansion Enclosure 0" {
			t.Fatalf("expected expansion enclosures to be filtered out")
		}
	}
}

func TestParsePerformanceStatsExtractsDeviceMetrics(t *testing.T) {
	t.Parallel()

	file := openFixture(t, "performance_stats.csv")

	devices, err := ParsePerformanceStats(file)
	if err != nil {
		t.Fatalf("ParsePerformanceStats returned unexpected error: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("expected at least one device")
	}

	first := devices[0]
	if first.Name != "Virtual Disk 0" {
		t.Fatalf("unexpected first device name: got %q want %q", first.Name, "Virtual Disk 0")
	}
	if first.CurrentIOsPerSecond != 12.5 {
		t.Fatalf("unexpected IOs/sec: got %v want %v", first.CurrentIOsPerSecond, 12.5)
	}
	if first.CurrentIOLatencyMS != 4 {
		t.Fatalf("unexpected latency: got %v want %v", first.CurrentIOLatencyMS, 4.0)
	}
	if first.TotalIOs != 1000 {
		t.Fatalf("unexpected total IOs: got %v want %v", first.TotalIOs, 1000.0)
	}
}

func TestParsePerformanceStatsRejectsMissingColumns(t *testing.T) {
	t.Parallel()

	malformed := "header 1\nheader 2\nheader 3\nObjects,Current IOs/sec\nsubheader 1,\nsubheader 2,\nVirtual Disk 0,12.5\n"

	if _, err := ParsePerformanceStats(strings.NewReader(malformed)); err == nil {
		t.Fatal("expected ParsePerformanceStats to reject missing columns")
	}
}

func TestParsePerformanceStatsHandlesRealSMcliLayout(t *testing.T) {
	t.Parallel()

	file := openFixture(t, "performance_stats_real_layout.csv")

	devices, err := ParsePerformanceStats(file)
	if err != nil {
		t.Fatalf("ParsePerformanceStats returned unexpected error: %v", err)
	}
	if len(devices) != 9 {
		t.Fatalf("unexpected device count: got %d want %d", len(devices), 9)
	}

	if devices[0].Name != "Storage Array Compunet-Cluster_storage" {
		t.Fatalf("unexpected first device name: got %q", devices[0].Name)
	}
	if !math.IsNaN(devices[0].CurrentIOLatencyMS) {
		t.Fatalf("expected storage array latency to be NaN when source value is '-', got %v", devices[0].CurrentIOLatencyMS)
	}

	last := devices[len(devices)-1]
	if last.Name != "Virtual Disk Data_Disk" {
		t.Fatalf("unexpected last device name: got %q want %q", last.Name, "Virtual Disk Data_Disk")
	}
	if last.CurrentIOLatencyMS != 4.4 {
		t.Fatalf("unexpected virtual disk latency: got %v want %v", last.CurrentIOLatencyMS, 4.4)
	}
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()

	path := filepath.Join("testdata", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", path, err)
	}

	t.Cleanup(func() {
		_ = file.Close()
	})

	return file
}
