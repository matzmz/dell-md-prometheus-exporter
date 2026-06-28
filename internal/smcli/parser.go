package smcli

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type DeviceStats struct {
	Name                 string
	CurrentIOsPerSecond  float64
	CurrentIOLatencyMS   float64
	CurrentMBPerSecond   float64
	PrimaryWriteCacheHit float64
	ReadPercent          float64
	PrimaryReadCacheHit  float64
	TotalIOs             float64
}

var requiredColumns = []string{
	"Objects",
	"Current IOs/sec",
	"Current IO Latency",
	"Current MBs/sec",
	"Primary Write Cache Hit %",
	"Read %",
	"Primary Read Cache Hit %",
	"Total IOs",
}

func ParsePerformanceStats(r io.Reader) ([]DeviceStats, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	indexes, err := readHeaderIndexes(reader)
	if err != nil {
		return nil, err
	}

	var devices []DeviceStats
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv record: %w", err)
		}

		objectName := field(record, indexes["Objects"])
		if shouldSkipObject(objectName) || recordHasNoMetricValues(record, indexes) {
			continue
		}

		device := DeviceStats{Name: objectName}

		if device.CurrentIOsPerSecond, err = parseFloatField(record, indexes["Current IOs/sec"], "Current IOs/sec"); err != nil {
			return nil, err
		}
		if device.CurrentIOLatencyMS, err = parseFloatField(record, indexes["Current IO Latency"], "Current IO Latency"); err != nil {
			return nil, err
		}
		if device.CurrentMBPerSecond, err = parseFloatField(record, indexes["Current MBs/sec"], "Current MBs/sec"); err != nil {
			return nil, err
		}
		if device.PrimaryWriteCacheHit, err = parseFloatField(record, indexes["Primary Write Cache Hit %"], "Primary Write Cache Hit %"); err != nil {
			return nil, err
		}
		if device.ReadPercent, err = parseFloatField(record, indexes["Read %"], "Read %"); err != nil {
			return nil, err
		}
		if device.PrimaryReadCacheHit, err = parseFloatField(record, indexes["Primary Read Cache Hit %"], "Primary Read Cache Hit %"); err != nil {
			return nil, err
		}
		if device.TotalIOs, err = parseFloatField(record, indexes["Total IOs"], "Total IOs"); err != nil {
			return nil, err
		}

		devices = append(devices, device)
	}

	return devices, nil
}

func field(record []string, index int) string {
	if index >= len(record) {
		return ""
	}

	return strings.TrimSpace(record[index])
}

func parseFloatField(record []string, index int, name string) (float64, error) {
	value := strings.TrimSuffix(field(record, index), "%")
	if value == "" || value == "-" {
		return math.NaN(), nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return parsed, nil
}

func readHeaderIndexes(reader *csv.Reader) (map[string]int, error) {
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("read csv header: missing required column %q", requiredColumns[0])
			}
			return nil, fmt.Errorf("read csv header: %w", err)
		}

		indexes := make(map[string]int, len(record))
		for idx, column := range record {
			indexes[strings.TrimSpace(column)] = idx
		}

		missingColumn := ""
		for _, column := range requiredColumns {
			if _, ok := indexes[column]; !ok {
				missingColumn = column
				break
			}
		}
		if missingColumn == "" {
			return indexes, nil
		}
	}
}

func shouldSkipObject(objectName string) bool {
	switch {
	case objectName == "":
		return true
	case strings.Contains(objectName, "Expansion Enclosure"):
		return true
	case strings.HasPrefix(objectName, "Capture Iteration:"):
		return true
	case strings.HasPrefix(objectName, "Date/Time:"):
		return true
	default:
		return false
	}
}

func recordHasNoMetricValues(record []string, indexes map[string]int) bool {
	for _, column := range requiredColumns {
		if column == "Objects" {
			continue
		}
		if field(record, indexes[column]) != "" {
			return false
		}
	}

	return true
}
