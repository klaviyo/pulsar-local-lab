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

// ConsumerUI manages the consumer terminal UI
type ConsumerUI struct {
	app          *tview.Application
	pool         *worker.Pool
	ctx          context.Context
	cancelFunc   context.CancelFunc
	metricsPanel *MetricsPanel
	graphWidget  *GraphWidget
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

// NewConsumerUI creates a new consumer UI
func NewConsumerUI(ctx context.Context, pool *worker.Pool) *ConsumerUI {
	cfg := getConsumerConfigFromPool(pool)
	targetRate := float64(0) // No target for consumer by default
	if cfg != nil {
		targetRate = float64(cfg.Performance.TargetThroughput)
	}

	app := tview.NewApplication()
	ctxWithCancel, cancel := context.WithCancel(ctx)

	// Create UI components
	metricsPanel := NewMetricsPanel("METRICS", targetRate)
	graphWidget := NewGraphWidget("CONSUMPTION RATE", 60, targetRate)
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
	}
	helpModal := NewHelpModal(shortcuts)

	ui := &ConsumerUI{
		app:          app,
		pool:         pool,
		ctx:          ctxWithCancel,
		cancelFunc:   cancel,
		metricsPanel: metricsPanel,
		graphWidget:  graphWidget,
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
func (ui *ConsumerUI) setupControlMenu() {
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
func (ui *ConsumerUI) buildLayout() {
	// Title header
	title := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[cyan::b]█▓▒░ PULSAR CONSUMER PERFORMANCE TEST ░▒▓█[-:-:-]")

	// Connection info
	cfg := ui.config
	connInfo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	fmt.Fprintf(connInfo, "[darkcyan]Connection:[-] %s  [darkcyan]│[-]  [darkcyan]Subscription:[-] %s ([darkcyan]%s[-])",
		truncateString(cfg.Pulsar.ServiceURL, 35),
		truncateString(cfg.Consumer.SubscriptionName, 25),
		cfg.Consumer.SubscriptionType)

	// Top section: metrics and graph side by side
	topSection := tview.NewFlex().
		AddItem(ui.metricsPanel, 0, 1, false).
		AddItem(ui.graphWidget, 0, 2, false)

	// Right content area (metrics and graph)
	rightContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(title, 1, 0, false).
		AddItem(connInfo, 1, 0, false).
		AddItem(tview.NewBox().SetBorder(false), 1, 0, false). // Spacer
		AddItem(topSection, 0, 1, false)

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
func (ui *ConsumerUI) handleInput(event *tcell.EventKey) *tcell.EventKey {
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
func (ui *ConsumerUI) togglePause() {
	if ui.pool.IsPaused() {
		ui.pool.Resume()
	} else {
		ui.pool.Pause()
	}
}

// resetMetrics resets all metrics
func (ui *ConsumerUI) resetMetrics() {
	ui.pool.GetMetrics().Reset()
	ui.graphWidget.dataPoints = ui.graphWidget.dataPoints[:0]
}

// addWorker adds a new worker to the pool
func (ui *ConsumerUI) addWorker() {
	err := ui.pool.AddWorker(ui.ctx, func(id int) (worker.Worker, error) {
		return worker.NewConsumerWorker(id, ui.config, ui.pool.GetMetrics())
	})
	if err != nil {
		// Silently handle error - can't log during TUI
		_ = err
	}
}

// removeWorker removes a worker from the pool
func (ui *ConsumerUI) removeWorker() {
	err := ui.pool.RemoveWorker()
	if err != nil {
		// Silently handle error - can't log during TUI
		_ = err
	}
}

// updateControlMenu updates the control menu display
func (ui *ConsumerUI) updateControlMenu() {
	// Update values in menu items
	if len(ui.controlMenu.items) > 0 {
		ui.controlMenu.items[0].Value = fmt.Sprintf("%d", ui.pool.WorkerCount())
	}
	ui.controlMenu.Render()
}

// showHelp displays the help modal
func (ui *ConsumerUI) showHelp() {
	ui.showingHelp = true
	ui.helpModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ui.hideHelp()
	})
	ui.app.SetRoot(ui.helpModal, true)
}

// hideHelp hides the help modal
func (ui *ConsumerUI) hideHelp() {
	ui.showingHelp = false
	ui.app.SetRoot(ui.mainLayout, true)
}

// toggleLogs shows/hides the log window
func (ui *ConsumerUI) toggleLogs() {
	if ui.showingLogs {
		ui.hideLogs()
	} else {
		ui.showLogs()
	}
}

// showLogs displays the log window
func (ui *ConsumerUI) showLogs() {
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
func (ui *ConsumerUI) hideLogs() {
	ui.showingLogs = false
	ui.app.SetRoot(ui.mainLayout, true)
}

// shutdown stops the UI and worker pool
func (ui *ConsumerUI) shutdown() {
	ui.app.Stop()       // Stop TUI first to restore terminal
	ui.cancelFunc()     // Cancel context to signal workers
	ui.pool.Stop()      // Stop workers (may take time, but terminal is restored)
}

// updateLoop runs the UI update loop
func (ui *ConsumerUI) updateLoop() {
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

				// Update metrics panel
				ui.metricsPanel.UpdateConsumerMetrics(snapshot)

				// Update graph with current rate
				ui.graphWidget.AddDataPoint(snapshot.Throughput.ReceiveRate)

				// Update status bar
				shortcuts := "↑↓←→ Navigate  [Q]uit  [P]ause  [R]eset  [L]ogs  [H]elp"
				ui.statusBar.Update(
					ui.pool.IsRunning(),
					ui.pool.IsPaused(),
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

// Run starts the consumer UI
func (ui *ConsumerUI) Run() error {
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

// RunConsumerUI is the main entry point for the consumer UI
func RunConsumerUI(ctx context.Context, pool *worker.Pool, logBuffer *LogBuffer) error {
	ui := NewConsumerUI(ctx, pool)
	ui.logBuffer = logBuffer
	ui.logWindow = NewLogWindow(logBuffer)
	return ui.Run()
}

// getConsumerConfigFromPool extracts config from pool (helper function)
func getConsumerConfigFromPool(pool *worker.Pool) *config.Config {
	return pool.GetConfig()
}