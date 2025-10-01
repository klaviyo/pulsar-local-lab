package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/ui"
	"github.com/pulsar-local-lab/perf-test/internal/worker"
)

const (
	appName    = "Pulsar Producer Performance Test"
	appVersion = "1.0.0"
)

// Command-line flags
var (
	configFile = flag.String("config", "", "Path to configuration file (JSON)")
	profile    = flag.String("profile", "default", "Performance test profile (default, low-latency, high-throughput, burst, sustained)")
	serviceURL = flag.String("service-url", "", "Pulsar broker service URL (overrides config)")
	topic      = flag.String("topic", "", "Pulsar topic name (overrides config)")
	partitions = flag.Int("partitions", -1, "Number of topic partitions (overrides config, -1=use config, 0=non-partitioned)")
	numWorkers = flag.Int("workers", 0, "Number of producer workers (overrides config, 0=use config)")
	showHelp   = flag.Bool("help", false, "Show help message")
	listProfs  = flag.Bool("list-profiles", false, "List available performance profiles")
	version    = flag.Bool("version", false, "Show version information")
)

func main() {
	// Parse command-line flags
	flag.Usage = printUsage
	flag.Parse()

	// Handle special flags
	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *listProfs {
		listProfiles()
		os.Exit(0)
	}

	// Create log buffer to capture all output
	logBuffer := ui.NewLogBuffer(500)

	// Redirect stdout and stderr to log buffer (captures ALL output including Pulsar client)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdout pipe: %v\n", err)
		os.Exit(1)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stderr pipe: %v\n", err)
		os.Exit(1)
	}

	// Save original stdout/stderr for emergency error messages
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Redirect OS-level stdout/stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	log.SetOutput(logBuffer)

	// Start goroutines to copy pipe output to log buffer
	go func() {
		_, _ = io.Copy(logBuffer, stdoutReader)
	}()
	go func() {
		_, _ = io.Copy(logBuffer, stderrReader)
	}()

	// Restore stderr for configuration errors (before UI starts)
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Apply CLI overrides
	applyOverrides(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Initialize producer worker pool
	pool, err := worker.NewProducerPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create producer pool: %v", err)
	}

	// Start the interactive UI with log buffer (blocks until quit)
	_ = ui.RunProducerUI(ctx, pool, logBuffer)

	// Graceful shutdown (silent - TUI has been stopped)
	_ = pool.Stop()

	// Export metrics if enabled
	if cfg.Metrics.ExportEnabled {
		_ = exportMetrics(pool, cfg)
	}
}

// loadConfiguration loads configuration from file or uses profile
func loadConfiguration() (*config.Config, error) {
	if *configFile != "" {
		log.Printf("Loading configuration from file: %s", *configFile)
		return config.LoadConfig(*configFile, "")
	}

	log.Printf("Using profile: %s", *profile)
	return config.LoadConfig("", *profile)
}

// applyOverrides applies command-line overrides to configuration
func applyOverrides(cfg *config.Config) {
	if *serviceURL != "" {
		log.Printf("Overriding service URL: %s", *serviceURL)
		cfg.Pulsar.ServiceURL = *serviceURL
	}

	if *topic != "" {
		log.Printf("Overriding topic: %s", *topic)
		cfg.Pulsar.Topic = *topic
	}

	if *partitions >= 0 {
		log.Printf("Overriding topic partitions: %d", *partitions)
		cfg.Pulsar.TopicPartitions = *partitions
	}

	if *numWorkers > 0 {
		log.Printf("Overriding worker count: %d", *numWorkers)
		cfg.Producer.NumProducers = *numWorkers
	}
}

// exportMetrics exports final metrics to file
func exportMetrics(pool *worker.Pool, cfg *config.Config) error {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(cfg.Metrics.ExportPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(cfg.Metrics.ExportPath, fmt.Sprintf("producer-metrics-%s.json", timestamp))

	// Export metrics (this would call a method on the collector)
	snapshot := pool.GetMetrics().GetSnapshot()

	// Write snapshot to file (simplified - real implementation would serialize to JSON)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create metrics file: %w", err)
	}
	defer file.Close()

	// Calculate throughput in Mbps
	throughputMbps := float64(snapshot.BytesSent) / snapshot.Elapsed.Seconds() / 1024 / 1024 * 8

	// Write summary
	fmt.Fprintf(file, "{\n")
	fmt.Fprintf(file, "  \"timestamp\": \"%s\",\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "  \"duration\": \"%v\",\n", snapshot.Elapsed)
	fmt.Fprintf(file, "  \"total_messages\": %d,\n", snapshot.MessagesSent)
	fmt.Fprintf(file, "  \"total_bytes\": %d,\n", snapshot.BytesSent)
	fmt.Fprintf(file, "  \"send_rate\": %.2f,\n", snapshot.Throughput.SendRate)
	fmt.Fprintf(file, "  \"throughput_mbps\": %.2f,\n", throughputMbps)
	fmt.Fprintf(file, "  \"errors\": %d\n", snapshot.MessagesFailed)
	fmt.Fprintf(file, "}\n")

	return nil
}

// printFinalStats prints final statistics to log
func printFinalStats(pool *worker.Pool) {
	snapshot := pool.GetMetrics().GetSnapshot()

	// Calculate throughput in Mbps
	throughputMbps := float64(snapshot.BytesSent) / snapshot.Elapsed.Seconds() / 1024 / 1024 * 8

	log.Printf("=== Final Statistics ===")
	log.Printf("  Duration: %v", snapshot.Elapsed)
	log.Printf("  Messages Sent: %d", snapshot.MessagesSent)
	log.Printf("  Bytes Sent: %d (%.2f MB)", snapshot.BytesSent, float64(snapshot.BytesSent)/(1024*1024))
	log.Printf("  Average Send Rate: %.2f msg/s", float64(snapshot.MessagesSent)/snapshot.Elapsed.Seconds())
	log.Printf("  Average Throughput: %.2f Mbps", throughputMbps)
	if snapshot.MessagesFailed > 0 {
		log.Printf("  Errors: %d (%.2f%%)", snapshot.MessagesFailed,
			float64(snapshot.MessagesFailed)/float64(snapshot.MessagesSent+snapshot.MessagesFailed)*100)
	}
	log.Printf("========================")
}

// printUsage prints usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "%s v%s\n\n", appName, appVersion)
	fmt.Fprintf(os.Stderr, "USAGE:\n")
	fmt.Fprintf(os.Stderr, "  %s [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "OPTIONS:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nEXAMPLES:\n")
	fmt.Fprintf(os.Stderr, "  # Start with default profile\n")
	fmt.Fprintf(os.Stderr, "  %s\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  # Use high-throughput profile\n")
	fmt.Fprintf(os.Stderr, "  %s --profile high-throughput\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  # Load from config file\n")
	fmt.Fprintf(os.Stderr, "  %s --config ./configs/custom.json\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  # Override service URL and topic\n")
	fmt.Fprintf(os.Stderr, "  %s --service-url pulsar://localhost:6650 --topic my-test-topic\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  # Use 10 workers with custom topic\n")
	fmt.Fprintf(os.Stderr, "  %s --workers 10 --topic perf-test-topic\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  # Test with 4 partitions\n")
	fmt.Fprintf(os.Stderr, "  %s --partitions 4 --workers 4\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "PROFILES:\n")
	for _, p := range config.GetAvailableProfiles() {
		fmt.Fprintf(os.Stderr, "  %-18s %s\n", p, config.GetProfileDescription(p))
	}
	fmt.Fprintf(os.Stderr, "\nKEYBOARD SHORTCUTS:\n")
	fmt.Fprintf(os.Stderr, "  Q / Ctrl+C  - Quit application\n")
	fmt.Fprintf(os.Stderr, "  P           - Pause/Resume workers\n")
	fmt.Fprintf(os.Stderr, "  R           - Reset metrics\n")
	fmt.Fprintf(os.Stderr, "  +/-         - Increase/Decrease workers\n")
	fmt.Fprintf(os.Stderr, "  H / ?       - Show help\n")
}

// listProfiles lists available performance profiles
func listProfiles() {
	fmt.Printf("Available performance profiles:\n\n")
	for _, p := range config.GetAvailableProfiles() {
		fmt.Printf("  %-18s %s\n", p, config.GetProfileDescription(p))
	}
	fmt.Printf("\nUse --profile <name> to select a profile\n")
}