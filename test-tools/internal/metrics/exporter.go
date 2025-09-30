package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Exporter handles exporting metrics to files
type Exporter struct {
	exportPath string
	enabled    bool
}

// NewExporter creates a new metrics exporter
func NewExporter(exportPath string, enabled bool) *Exporter {
	return &Exporter{
		exportPath: exportPath,
		enabled:    enabled,
	}
}

// Export exports a metrics snapshot to a file
func (e *Exporter) Export(snapshot Snapshot) error {
	if !e.enabled {
		return nil
	}

	// Create export directory if it doesn't exist
	if err := os.MkdirAll(e.exportPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(e.exportPath, fmt.Sprintf("metrics-%s.json", timestamp))

	// Marshal snapshot to JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write metrics file: %w", err)
	}

	return nil
}

// ExportCSV exports metrics to CSV format for analysis
func (e *Exporter) ExportCSV(snapshots []Snapshot) error {
	if !e.enabled {
		return nil
	}

	// Create export directory if it doesn't exist
	if err := os.MkdirAll(e.exportPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(e.exportPath, fmt.Sprintf("metrics-%s.csv", timestamp))

	// Create CSV file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Write CSV header
	header := "timestamp,messages_sent,messages_received,messages_acked,messages_failed," +
		"bytes_sent,bytes_received,latency_min,latency_max,latency_mean,latency_p50," +
		"latency_p95,latency_p99,latency_p999,send_rate,receive_rate\n"
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, snapshot := range snapshots {
		row := fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f\n",
			time.Now().Unix(),
			snapshot.MessagesSent,
			snapshot.MessagesReceived,
			snapshot.MessagesAcked,
			snapshot.MessagesFailed,
			snapshot.BytesSent,
			snapshot.BytesReceived,
			snapshot.LatencyStats.Min,
			snapshot.LatencyStats.Max,
			snapshot.LatencyStats.Mean,
			snapshot.LatencyStats.P50,
			snapshot.LatencyStats.P95,
			snapshot.LatencyStats.P99,
			snapshot.LatencyStats.P999,
			snapshot.Throughput.SendRate,
			snapshot.Throughput.ReceiveRate,
		)
		if _, err := file.WriteString(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}