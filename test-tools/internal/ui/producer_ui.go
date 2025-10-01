package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/worker"
	"github.com/rivo/tview"
)

// ProducerUI manages the producer terminal UI
type ProducerUI struct {
	app          *tview.Application
	pool         *worker.Pool
	ctx          context.Context
	cancelFunc   context.CancelFunc
	metricsPanel *MetricsPanel
	graphWidget  *GraphWidget
	configPanel  *ConfigPanel
	controlMenu  *ControlMenu
	statusBar    *StatusBar
	helpModal    *HelpModal
	logWindow    *LogWindow
	logBuffer    *LogBuffer
	mainLayout   *tview.Flex
	showingHelp  bool
	showingLogs  bool
	config       *config.Config
}

// NewProducerUI creates a new producer UI
func NewProducerUI(ctx context.Context, pool *worker.Pool) *ProducerUI {
	cfg := getConfigFromPool(pool)
	targetRate := float64(0)
	if cfg != nil {
		targetRate = float64(cfg.Performance.TargetThroughput)
	}

	app := tview.NewApplication()
	ctxWithCancel, cancel := context.WithCancel(ctx)

	// Create UI components
	metricsPanel := NewMetricsPanel("METRICS", targetRate)
	graphWidget := NewGraphWidget("THROUGHPUT", 60, targetRate)
	configPanel := NewConfigPanel(cfg, "CONFIGURATION")
	statusBar := NewStatusBar()
	controlMenu := NewControlMenu("CONTROLS")

	// Create help modal
	shortcuts := map[string]string{
		"Q / Ctrl+C":    "Quit application",
		"↑/↓ Arrows":    "Navigate controls",
		"←/→ Arrows":    "Adjust values",
		"Enter/Space":   "Activate button",
		"P":             "Pause/Resume",
		"R":             "Reset metrics",
		"L":             "Show/hide logs",
		"C":             "Clear logs (when visible)",
		"H / ?":         "Show/hide help",
		"* (asterisk)":  "Use 'Restart Workers' to apply",
	}
	helpModal := NewHelpModal(shortcuts)

	ui := &ProducerUI{
		app:          app,
		pool:         pool,
		ctx:          ctxWithCancel,
		cancelFunc:   cancel,
		metricsPanel: metricsPanel,
		graphWidget:  graphWidget,
		configPanel:  configPanel,
		controlMenu:  controlMenu,
		statusBar:    statusBar,
		helpModal:    helpModal,
		showingHelp:  false,
		config:       cfg,
	}

	ui.setupControlMenu()
	ui.buildLayout()
	return ui
}

// setupControlMenu configures the control menu items
func (ui *ProducerUI) setupControlMenu() {
	cfg := ui.pool.GetConfig()

	// Workers control
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Workers",
		Value:      fmt.Sprintf("%d", ui.pool.WorkerCount()),
		Adjustable: true,
		Action: func(delta int) {
			if delta > 0 {
				ui.addWorker()
			} else {
				ui.removeWorker()
			}
		},
	})

	// Target Rate control (msg/s, 0 = unlimited)
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Target Rate",
		Value:      formatTargetRate(cfg.Performance.TargetThroughput),
		Adjustable: true,
		Action: func(delta int) {
			ui.adjustTargetRate(delta)
		},
	})

	// Message Size control (requires restart to take effect)
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Message Size*",
		Value:      formatMessageSize(cfg.Producer.MessageSize),
		Adjustable: true,
		Action: func(delta int) {
			ui.adjustMessageSize(delta)
		},
	})

	// Batch Size control (requires restart to take effect)
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Batch Size*",
		Value:      fmt.Sprintf("%d", cfg.Producer.BatchingMaxSize),
		Adjustable: true,
		Action: func(delta int) {
			ui.adjustBatchSize(delta)
		},
	})

	// Compression Type control (requires restart to take effect)
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Compression*",
		Value:      cfg.Producer.CompressionType,
		Adjustable: true,
		Action: func(delta int) {
			ui.adjustCompression(delta)
		},
	})

	// Restart Workers button (applies settings marked with *)
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Restart Workers",
		Value:      "",
		Adjustable: false,
		ToggleFunc: func() {
			ui.restartWorkers()
		},
	})

	// Pause/Resume button
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Pause/Resume",
		Value:      "",
		Adjustable: false,
		ToggleFunc: func() {
			ui.togglePause()
		},
	})

	// Reset metrics button
	ui.controlMenu.AddItem(&ControlMenuItem{
		Label:      "Reset Metrics",
		Value:      "",
		Adjustable: false,
		ToggleFunc: func() {
			ui.resetMetrics()
		},
	})
}

// buildLayout constructs the UI layout
func (ui *ProducerUI) buildLayout() {
	// Title header
	title := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[cyan::b]█▓▒░ PULSAR PRODUCER PERFORMANCE TEST ░▒▓█[-:-:-]")

	// Connection info
	cfg := ui.config
	connInfo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	fmt.Fprintf(connInfo, "[darkcyan]Connection:[-] %s  [darkcyan]│[-]  [darkcyan]Topic:[-] %s",
		truncateString(cfg.Pulsar.ServiceURL, 40),
		truncateString(cfg.Pulsar.Topic, 50))

	// Top section: metrics and graph side by side
	topSection := tview.NewFlex().
		AddItem(ui.metricsPanel, 0, 1, false).
		AddItem(ui.graphWidget, 0, 2, false)

	// Right content area (metrics, graph, config)
	rightContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(title, 1, 0, false).
		AddItem(connInfo, 1, 0, false).
		AddItem(tview.NewBox().SetBorder(false), 1, 0, false). // Spacer
		AddItem(topSection, 0, 4, false).
		AddItem(ui.configPanel, 12, 0, false)

	// Main content with control menu on left
	mainContent := tview.NewFlex().
		AddItem(ui.controlMenu, 40, 0, false).
		AddItem(rightContent, 0, 1, false)

	// Main layout with status bar at bottom
	ui.mainLayout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainContent, 0, 1, false).
		AddItem(ui.statusBar, 1, 0, false)

	// Set up input handling
	ui.mainLayout.SetInputCapture(ui.handleInput)
}

// handleInput handles keyboard input
func (ui *ProducerUI) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// If help is showing, let modal handle input
	if ui.showingHelp {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'h' || event.Rune() == 'H' || event.Rune() == '?' {
			ui.hideHelp()
			return nil
		}
		return event
	}

	// Handle key events
	switch event.Key() {
	case tcell.KeyCtrlC, tcell.KeyEscape:
		ui.shutdown()
		return nil
	case tcell.KeyUp:
		ui.controlMenu.MoveSelection(-1)
		ui.updateControlMenu()
		return nil
	case tcell.KeyDown:
		ui.controlMenu.MoveSelection(1)
		ui.updateControlMenu()
		return nil
	case tcell.KeyLeft:
		ui.controlMenu.AdjustValue(-1)
		ui.updateControlMenu()
		return nil
	case tcell.KeyRight:
		ui.controlMenu.AdjustValue(1)
		ui.updateControlMenu()
		return nil
	case tcell.KeyEnter:
		ui.controlMenu.ActivateSelected()
		ui.updateControlMenu()
		return nil
	}

	// Handle rune events
	switch event.Rune() {
	case 'q', 'Q':
		ui.shutdown()
		return nil
	case 'p', 'P':
		ui.togglePause()
		return nil
	case 'r', 'R':
		ui.resetMetrics()
		return nil
	case 'l', 'L':
		ui.toggleLogs()
		return nil
	case 'c', 'C':
		if ui.showingLogs {
			ui.logBuffer.Clear()
		}
		return nil
	case ' ': // Space bar
		ui.controlMenu.ActivateSelected()
		ui.updateControlMenu()
		return nil
	case 'h', 'H', '?':
		ui.showHelp()
		return nil
	}

	return event
}

// togglePause toggles worker pool pause/resume
func (ui *ProducerUI) togglePause() {
	if ui.pool.IsRunning() {
		ui.pool.Stop()
	} else {
		ui.pool.Start(ui.ctx)
	}
}

// resetMetrics resets all metrics
func (ui *ProducerUI) resetMetrics() {
	ui.pool.GetMetrics().Reset()
	ui.graphWidget.dataPoints = ui.graphWidget.dataPoints[:0]
}

// addWorker adds a new worker to the pool
func (ui *ProducerUI) addWorker() {
	err := ui.pool.AddWorker(ui.ctx, func(id int) (worker.Worker, error) {
		return worker.NewProducerWorker(id, ui.config, ui.pool.GetMetrics())
	})
	if err != nil {
		// Silently handle error - can't log during TUI
		_ = err
	}
}

// removeWorker removes a worker from the pool
func (ui *ProducerUI) removeWorker() {
	err := ui.pool.RemoveWorker()
	if err != nil {
		// Silently handle error - can't log during TUI
		_ = err
	}
}

// updateControlMenu updates the control menu display
func (ui *ProducerUI) updateControlMenu() {
	cfg := ui.pool.GetConfig()

	// Update values in menu items
	// Order: Workers, Target Rate, Message Size, Batch Size, Compression, Restart, Pause, Reset
	if len(ui.controlMenu.items) >= 5 {
		ui.controlMenu.items[0].Value = fmt.Sprintf("%d", ui.pool.WorkerCount())
		ui.controlMenu.items[1].Value = formatTargetRate(cfg.Performance.TargetThroughput)
		ui.controlMenu.items[2].Value = formatMessageSize(cfg.Producer.MessageSize)
		ui.controlMenu.items[3].Value = fmt.Sprintf("%d", cfg.Producer.BatchingMaxSize)
		ui.controlMenu.items[4].Value = cfg.Producer.CompressionType
	}
	ui.controlMenu.Render()
}

// adjustTargetRate adjusts the target message rate
func (ui *ProducerUI) adjustTargetRate(delta int) {
	cfg := ui.pool.GetConfig()
	current := cfg.Performance.TargetThroughput

	// Adjust in increments based on current value
	var increment int
	if current == 0 {
		increment = 100 // Start at 100 if currently unlimited
	} else if current < 1000 {
		increment = 100
	} else if current < 10000 {
		increment = 1000
	} else {
		increment = 5000
	}

	newRate := current + (delta * increment)
	if newRate < 0 {
		newRate = 0 // 0 = unlimited
	}

	ui.pool.UpdateTargetRate(newRate)
}

// adjustBatchSize adjusts the batching max size
func (ui *ProducerUI) adjustBatchSize(delta int) {
	cfg := ui.pool.GetConfig()
	current := cfg.Producer.BatchingMaxSize

	// Adjust in increments of 100
	increment := 100
	if current > 1000 {
		increment = 500
	}

	newSize := current + (delta * increment)
	if newSize < 1 {
		newSize = 1
	}
	if newSize > 10000 {
		newSize = 10000
	}

	ui.pool.UpdateBatchSize(newSize)
}

// adjustCompression cycles through compression types
func (ui *ProducerUI) adjustCompression(delta int) {
	compressionTypes := []string{"NONE", "LZ4", "ZLIB", "ZSTD"}
	cfg := ui.pool.GetConfig()
	current := cfg.Producer.CompressionType

	// Find current index
	currentIdx := 0
	for i, ct := range compressionTypes {
		if ct == current {
			currentIdx = i
			break
		}
	}

	// Calculate new index
	newIdx := currentIdx + delta
	if newIdx < 0 {
		newIdx = len(compressionTypes) - 1
	} else if newIdx >= len(compressionTypes) {
		newIdx = 0
	}

	ui.pool.UpdateCompression(compressionTypes[newIdx])
}

// adjustMessageSize adjusts the message size
func (ui *ProducerUI) adjustMessageSize(delta int) {
	cfg := ui.pool.GetConfig()
	current := cfg.Producer.MessageSize

	// Adjust in increments based on current value
	var increment int
	if current < 1024 {
		increment = 256 // 256 bytes
	} else if current < 10240 {
		increment = 1024 // 1 KB
	} else if current < 102400 {
		increment = 10240 // 10 KB
	} else {
		increment = 102400 // 100 KB
	}

	newSize := current + (delta * increment)
	if newSize < 256 {
		newSize = 256 // Minimum 256 bytes
	}
	if newSize > 1048576 {
		newSize = 1048576 // Maximum 1 MB
	}

	ui.pool.UpdateMessageSize(newSize)
}

// restartWorkers restarts all workers to apply immutable settings
func (ui *ProducerUI) restartWorkers() {
	err := ui.pool.RestartWorkers(ui.ctx)
	if err != nil {
		// Silently handle error - can't log during TUI
		_ = err
	}
}

// formatTargetRate formats the target rate for display
func formatTargetRate(rate int) string {
	if rate == 0 {
		return "unlimited"
	}
	if rate >= 1000 {
		return fmt.Sprintf("%dk/s", rate/1000)
	}
	return fmt.Sprintf("%d/s", rate)
}

// formatMessageSize formats message size for display
func formatMessageSize(size int) string {
	if size >= 1048576 {
		return fmt.Sprintf("%.1fMB", float64(size)/1048576)
	}
	if size >= 1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%dB", size)
}

// showHelp displays the help modal
func (ui *ProducerUI) showHelp() {
	ui.showingHelp = true
	ui.helpModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ui.hideHelp()
	})
	ui.app.SetRoot(ui.helpModal, true)
}

// hideHelp hides the help modal
func (ui *ProducerUI) hideHelp() {
	ui.showingHelp = false
	ui.app.SetRoot(ui.mainLayout, true)
}

// toggleLogs shows/hides the log window
func (ui *ProducerUI) toggleLogs() {
	if ui.showingLogs {
		ui.hideLogs()
	} else {
		ui.showLogs()
	}
}

// showLogs displays the log window
func (ui *ProducerUI) showLogs() {
	ui.showingLogs = true
	ui.logWindow.Update()

	// Set input capture for log window
	ui.logWindow.GetTextView().SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'l' || event.Rune() == 'L' || event.Key() == tcell.KeyEscape {
			ui.hideLogs()
			return nil
		}
		if event.Rune() == 'c' || event.Rune() == 'C' {
			ui.logBuffer.Clear()
			ui.logWindow.Update()
			return nil
		}
		if event.Rune() == 'q' || event.Rune() == 'Q' || event.Key() == tcell.KeyCtrlC {
			ui.shutdown()
			return nil
		}
		return event
	})

	ui.app.SetRoot(ui.logWindow.GetTextView(), true)
}

// hideLogs hides the log window
func (ui *ProducerUI) hideLogs() {
	ui.showingLogs = false
	ui.app.SetRoot(ui.mainLayout, true)
}

// shutdown stops the UI and worker pool
func (ui *ProducerUI) shutdown() {
	ui.pool.Stop()
	ui.cancelFunc()
	ui.app.Stop()
}

// updateLoop runs the UI update loop
func (ui *ProducerUI) updateLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ticker.C:
			snapshot := ui.pool.GetMetrics().GetSnapshot()

			ui.app.QueueUpdateDraw(func() {
				// Update control menu
				ui.updateControlMenu()

				// Update configuration panel
				ui.configPanel.Render()

				// Update metrics panel
				ui.metricsPanel.UpdateProducerMetrics(snapshot)

				// Update graph with current rate
				ui.graphWidget.AddDataPoint(snapshot.Throughput.SendRate)

				// Update status bar
				shortcuts := "↑↓←→ Navigate  [Q]uit  [P]ause  [R]eset  [L]ogs  [H]elp"
				ui.statusBar.Update(
					ui.pool.IsRunning(),
					ui.pool.WorkerCount(),
					snapshot.Elapsed,
					shortcuts,
				)

				// Update log window if visible
				if ui.showingLogs && ui.logWindow != nil {
					ui.logWindow.Update()
				}
			})
		}
	}
}

// Run starts the producer UI
func (ui *ProducerUI) Run() error {
	// Start the worker pool
	if err := ui.pool.Start(ui.ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start update loop in background
	go ui.updateLoop()

	// Run the application
	if err := ui.app.SetRoot(ui.mainLayout, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("failed to run UI: %w", err)
	}

	return nil
}

// RunProducerUI is the main entry point for the producer UI
func RunProducerUI(ctx context.Context, pool *worker.Pool, logBuffer *LogBuffer) error {
	ui := NewProducerUI(ctx, pool)
	ui.logBuffer = logBuffer
	ui.logWindow = NewLogWindow(logBuffer)
	return ui.Run()
}

// getConfigFromPool extracts config from pool (helper function)
func getConfigFromPool(pool *worker.Pool) *config.Config {
	return pool.GetConfig()
}