package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExporter(t *testing.T) {
	exporter := NewExporter("/tmp/test", true)

	if exporter == nil {
		t.Fatal("NewExporter returned nil")
	}

	if exporter.exportPath != "/tmp/test" {
		t.Errorf("Expected exportPath /tmp/test, got %s", exporter.exportPath)
	}

	if !exporter.enabled {
		t.Error("Expected exporter to be enabled")
	}
}

func TestExporterDisabled(t *testing.T) {
	exporter := NewExporter("/tmp/test", false)

	snapshot := Snapshot{
		MessagesSent: 100,
		BytesSent:    1024,
	}

	// Should return nil without error when disabled
	if err := exporter.Export(snapshot); err != nil {
		t.Errorf("Export should return nil when disabled, got %v", err)
	}
}

func TestExporterExport(t *testing.T) {
	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), "metrics_test")
	defer os.RemoveAll(tmpDir)

	exporter := NewExporter(tmpDir, true)

	snapshot := Snapshot{
		MessagesSent:     1000,
		MessagesReceived: 900,
		MessagesAcked:    850,
		MessagesFailed:   50,
		BytesSent:        1024000,
		BytesReceived:    921600,
		LatencyStats: LatencyStats{
			Min:   1.0,
			Max:   100.0,
			Mean:  25.5,
			P50:   20.0,
			P95:   80.0,
			P99:   95.0,
			P999:  99.0,
			Count: 1000,
		},
		Throughput: ThroughputStats{
			SendRate:    100.0,
			ReceiveRate: 90.0,
			Window:      10 * time.Second,
		},
		Elapsed:    time.Minute,
		SinceReset: 30 * time.Second,
	}

	// Export
	if err := exporter.Export(snapshot); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify file exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read export directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	// Verify file is JSON
	filename := filepath.Join(tmpDir, files[0].Name())
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}

	// Unmarshal and verify content
	var exported Snapshot
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("Failed to unmarshal exported JSON: %v", err)
	}

	if exported.MessagesSent != snapshot.MessagesSent {
		t.Errorf("Expected MessagesSent %d, got %d", snapshot.MessagesSent, exported.MessagesSent)
	}

	if exported.BytesSent != snapshot.BytesSent {
		t.Errorf("Expected BytesSent %d, got %d", snapshot.BytesSent, exported.BytesSent)
	}
}

func TestExporterExportCSV(t *testing.T) {
	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), "metrics_csv_test")
	defer os.RemoveAll(tmpDir)

	exporter := NewExporter(tmpDir, true)

	snapshots := []Snapshot{
		{
			MessagesSent:     100,
			MessagesReceived: 90,
			MessagesAcked:    85,
			MessagesFailed:   5,
			BytesSent:        102400,
			BytesReceived:    92160,
			LatencyStats: LatencyStats{
				Min:  1.0,
				Max:  50.0,
				Mean: 10.5,
				P50:  8.0,
				P95:  40.0,
				P99:  48.0,
				P999: 50.0,
			},
			Throughput: ThroughputStats{
				SendRate:    10.0,
				ReceiveRate: 9.0,
			},
		},
		{
			MessagesSent:     200,
			MessagesReceived: 190,
			MessagesAcked:    180,
			MessagesFailed:   10,
			BytesSent:        204800,
			BytesReceived:    194560,
			LatencyStats: LatencyStats{
				Min:  1.5,
				Max:  60.0,
				Mean: 12.0,
				P50:  10.0,
				P95:  50.0,
				P99:  58.0,
				P999: 60.0,
			},
			Throughput: ThroughputStats{
				SendRate:    20.0,
				ReceiveRate: 19.0,
			},
		},
	}

	// Export
	if err := exporter.ExportCSV(snapshots); err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	// Verify file exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read export directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	// Verify file is CSV
	filename := filepath.Join(tmpDir, files[0].Name())
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}

	csvContent := string(data)

	// Verify header
	if !strings.Contains(csvContent, "timestamp,messages_sent,messages_received") {
		t.Error("CSV should contain proper header")
	}

	// Verify data rows (should have header + 2 data rows)
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines (1 header + 2 data rows), got %d", len(lines))
	}

	// Verify first data row contains expected values
	if !strings.Contains(lines[1], "100") {
		t.Error("First data row should contain messages_sent=100")
	}

	// Verify second data row contains expected values
	if !strings.Contains(lines[2], "200") {
		t.Error("Second data row should contain messages_sent=200")
	}
}

func TestExporterExportCSVDisabled(t *testing.T) {
	exporter := NewExporter("/tmp/test", false)

	snapshots := []Snapshot{{MessagesSent: 100}}

	// Should return nil without error when disabled
	if err := exporter.ExportCSV(snapshots); err != nil {
		t.Errorf("ExportCSV should return nil when disabled, got %v", err)
	}
}

func TestExporterExportCreateDirectory(t *testing.T) {
	// Use a deeply nested path that doesn't exist
	tmpDir := filepath.Join(os.TempDir(), "metrics_test_nested", "subdir1", "subdir2")
	defer os.RemoveAll(filepath.Join(os.TempDir(), "metrics_test_nested"))

	exporter := NewExporter(tmpDir, true)

	snapshot := Snapshot{
		MessagesSent: 100,
	}

	// Should create directory and export successfully
	if err := exporter.Export(snapshot); err != nil {
		t.Fatalf("Export should create directory and succeed, got %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Export directory should be created")
	}
}

func TestExporterExportCSVCreateDirectory(t *testing.T) {
	// Use a deeply nested path that doesn't exist
	tmpDir := filepath.Join(os.TempDir(), "metrics_csv_test_nested", "subdir1", "subdir2")
	defer os.RemoveAll(filepath.Join(os.TempDir(), "metrics_csv_test_nested"))

	exporter := NewExporter(tmpDir, true)

	snapshots := []Snapshot{{MessagesSent: 100}}

	// Should create directory and export successfully
	if err := exporter.ExportCSV(snapshots); err != nil {
		t.Fatalf("ExportCSV should create directory and succeed, got %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Export directory should be created")
	}
}