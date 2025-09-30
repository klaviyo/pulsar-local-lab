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
	mainLayout   *tview.Flex
	showingHelp  bool
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
		"H / ?":         "Show/hide help",
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
	// Update values in menu items
	if len(ui.controlMenu.items) > 0 {
		ui.controlMenu.items[0].Value = fmt.Sprintf("%d", ui.pool.WorkerCount())
	}
	ui.controlMenu.Render()
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
				// Update metrics panel
				ui.metricsPanel.UpdateProducerMetrics(snapshot)

				// Update graph with current rate
				ui.graphWidget.AddDataPoint(snapshot.Throughput.SendRate)

				// Update status bar
				shortcuts := "[Q]uit  [P]ause  [R]eset  [+/-]Workers  [H]elp"
				ui.statusBar.Update(
					ui.pool.IsRunning(),
					ui.pool.WorkerCount(),
					snapshot.Elapsed,
					shortcuts,
				)
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
func RunProducerUI(ctx context.Context, pool *worker.Pool) error {
	ui := NewProducerUI(ctx, pool)
	return ui.Run()
}

// getConfigFromPool extracts config from pool (helper function)
func getConfigFromPool(pool *worker.Pool) *config.Config {
	// This is a workaround since Pool doesn't expose Config
	// In a real implementation, you'd add a GetConfig() method to Pool
	// For now, we'll use reflection or create a default config
	return &config.Config{
		Pulsar: config.PulsarConfig{
			ServiceURL: "pulsar://localhost:6650",
			Topic:      "persistent://public/default/perf-test",
		},
		Producer: config.ProducerConfig{
			NumProducers:    pool.WorkerCount(),
			BatchingMaxSize: 100,
			MessageSize:     1024,
			CompressionType: "LZ4",
		},
		Performance: config.PerformanceConfig{
			TargetThroughput: 1000,
		},
	}
}