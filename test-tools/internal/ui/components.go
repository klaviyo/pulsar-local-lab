package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
	"github.com/rivo/tview"
)

// Color scheme constants for consistent theming
var (
	ColorHeader    = tcell.NewRGBColor(0, 255, 255)    // Cyan
	ColorLabel     = tcell.NewRGBColor(255, 255, 255)  // White
	ColorGood      = tcell.NewRGBColor(0, 255, 0)      // Green
	ColorWarning   = tcell.NewRGBColor(255, 255, 0)    // Yellow
	ColorError     = tcell.NewRGBColor(255, 0, 0)      // Red
	ColorGraph     = tcell.NewRGBColor(0, 128, 255)    // Blue
	ColorBorder    = tcell.NewRGBColor(0, 128, 128)    // Dark Cyan
	ColorHighlight = tcell.NewRGBColor(0, 255, 255)    // Cyan
)

// MetricsPanel displays key performance metrics in a formatted table
type MetricsPanel struct {
	*tview.TextView
	lastSnapshot metrics.Snapshot
	targetRate   float64
}

// NewMetricsPanel creates a new metrics panel
func NewMetricsPanel(title string, targetRate float64) *MetricsPanel {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetBorderColor(ColorBorder).
		SetTitleColor(ColorHeader)

	return &MetricsPanel{
		TextView:   tv,
		targetRate: targetRate,
	}
}

// UpdateProducerMetrics updates the panel with producer metrics
func (m *MetricsPanel) UpdateProducerMetrics(snapshot metrics.Snapshot) {
	m.lastSnapshot = snapshot
	m.Clear()

	// Calculate current throughput rate and bandwidth
	currentRate := snapshot.Throughput.SendRate
	rateColor := m.getRateColor(currentRate, m.targetRate)
	bandwidth := snapshot.Throughput.SendBandwidth

	// Messages section
	fmt.Fprintf(m, "[%s]┌─ MESSAGES ─────────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(m, " [%s]Sent:    [-][%s]%s[-] msgs\n", colorName(ColorLabel), colorName(ColorGood), formatNumber(snapshot.MessagesSent))
	fmt.Fprintf(m, " [%s]Failed:  [-][%s]%s[-] msgs\n", colorName(ColorLabel), m.getFailureColor(snapshot.MessagesFailed), formatNumber(snapshot.MessagesFailed))
	fmt.Fprintf(m, " [%s]Rate:    [-][%s]%s[-]\n", colorName(ColorLabel), rateColor, formatRate(currentRate))
	fmt.Fprintf(m, " [%s]Target:  [-]%s\n", colorName(ColorLabel), formatRate(m.targetRate))
	fmt.Fprintf(m, " [%s]Bytes:   [-][%s]%s[-]\n", colorName(ColorLabel), colorName(ColorGood), formatBytes(snapshot.BytesSent))
	fmt.Fprintf(m, " [%s]Bandwidth:[-][%s]%s[-]\n", colorName(ColorLabel), colorName(ColorGood), formatBandwidth(bandwidth))

	// Latency section
	fmt.Fprintf(m, "\n[%s]┌─ LATENCY ──────────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(m, " [%s]P50:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P50))
	fmt.Fprintf(m, " [%s]P95:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P95))
	fmt.Fprintf(m, " [%s]P99:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P99))
	fmt.Fprintf(m, " [%s]P999:    [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P999))
	fmt.Fprintf(m, " [%s]Min/Max: [-]%.2f / %.2f ms\n", colorName(ColorLabel), snapshot.LatencyStats.Min, snapshot.LatencyStats.Max)
	fmt.Fprintf(m, " [%s]Mean:    [-]%.2f ms\n", colorName(ColorLabel), snapshot.LatencyStats.Mean)
}

// UpdateConsumerMetrics updates the panel with consumer metrics
func (m *MetricsPanel) UpdateConsumerMetrics(snapshot metrics.Snapshot) {
	m.lastSnapshot = snapshot
	m.Clear()

	// Calculate current throughput rate
	currentRate := snapshot.Throughput.ReceiveRate
	rateColor := m.getRateColor(currentRate, m.targetRate)
	bandwidth := snapshot.Throughput.ReceiveBandwidth

	// Messages section
	fmt.Fprintf(m, "[%s]┌─ MESSAGES ─────────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(m, " [%s]Received:[-][%s]%s[-] msgs\n", colorName(ColorLabel), colorName(ColorGood), formatNumber(snapshot.MessagesReceived))
	fmt.Fprintf(m, " [%s]Acked:   [-][%s]%s[-] msgs\n", colorName(ColorLabel), colorName(ColorGood), formatNumber(snapshot.MessagesAcked))
	fmt.Fprintf(m, " [%s]Failed:  [-][%s]%s[-] msgs\n", colorName(ColorLabel), m.getFailureColor(snapshot.MessagesFailed), formatNumber(snapshot.MessagesFailed))
	fmt.Fprintf(m, " [%s]Rate:    [-][%s]%s[-]\n", colorName(ColorLabel), rateColor, formatRate(currentRate))
	fmt.Fprintf(m, " [%s]Bytes:   [-][%s]%s[-]\n", colorName(ColorLabel), colorName(ColorGood), formatBytes(snapshot.BytesReceived))
	fmt.Fprintf(m, " [%s]Bandwidth:[-][%s]%s[-]\n", colorName(ColorLabel), colorName(ColorGood), formatBandwidth(bandwidth))

	// Acknowledgment rate
	ackRate := float64(0)
	if snapshot.MessagesReceived > 0 {
		ackRate = float64(snapshot.MessagesAcked) / float64(snapshot.MessagesReceived) * 100
	}
	ackColor := ColorGood
	if ackRate < 99.0 {
		ackColor = ColorWarning
	}
	if ackRate < 95.0 {
		ackColor = ColorError
	}
	fmt.Fprintf(m, " [%s]Ack Rate:[-][%s]%.2f%%[-]\n", colorName(ColorLabel), colorName(ackColor), ackRate)

	// End-to-end latency section
	fmt.Fprintf(m, "\n[%s]┌─ E2E LATENCY ──────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(m, " [%s]P50:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P50))
	fmt.Fprintf(m, " [%s]P95:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P95))
	fmt.Fprintf(m, " [%s]P99:     [-]%s\n", colorName(ColorLabel), m.formatLatency(snapshot.LatencyStats.P99))
}

// getRateColor returns the appropriate color based on current rate vs target
func (m *MetricsPanel) getRateColor(current, target float64) string {
	if target == 0 {
		return colorName(ColorGood)
	}

	ratio := current / target
	if ratio >= 0.95 && ratio <= 1.05 {
		return colorName(ColorGood)
	} else if ratio >= 0.80 && ratio < 0.95 {
		return colorName(ColorWarning)
	}
	return colorName(ColorError)
}

// getFailureColor returns color based on failure count
func (m *MetricsPanel) getFailureColor(failures uint64) string {
	if failures == 0 {
		return colorName(ColorGood)
	} else if failures < 100 {
		return colorName(ColorWarning)
	}
	return colorName(ColorError)
}

// formatLatency formats latency with color coding
func (m *MetricsPanel) formatLatency(latency float64) string {
	color := ColorGood
	if latency > 100 {
		color = ColorError
	} else if latency > 50 {
		color = ColorWarning
	}
	return fmt.Sprintf("[%s]%.2f ms[-]", colorName(color), latency)
}

// GraphWidget displays an ASCII art time-series graph
type GraphWidget struct {
	*tview.TextView
	dataPoints []float64
	maxPoints  int
	targetRate float64
}

// NewGraphWidget creates a new graph widget
func NewGraphWidget(title string, maxPoints int, targetRate float64) *GraphWidget {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetBorderColor(ColorBorder).
		SetTitleColor(ColorHeader)

	return &GraphWidget{
		TextView:   tv,
		dataPoints: make([]float64, 0, maxPoints),
		maxPoints:  maxPoints,
		targetRate: targetRate,
	}
}

// AddDataPoint adds a new data point to the graph
func (g *GraphWidget) AddDataPoint(value float64) {
	g.dataPoints = append(g.dataPoints, value)
	if len(g.dataPoints) > g.maxPoints {
		g.dataPoints = g.dataPoints[1:]
	}
	g.Render()
}

// Render renders the graph
func (g *GraphWidget) Render() {
	g.Clear()

	if len(g.dataPoints) == 0 {
		fmt.Fprintf(g, "\n  [%s]Collecting data...[-]", colorName(ColorLabel))
		return
	}

	// Graph dimensions
	_, _, width, height := g.GetInnerRect()
	if width <= 4 || height <= 4 {
		return // Not enough space
	}

	graphHeight := height - 3 // Leave space for labels
	graphWidth := width - 4   // Leave space for borders and axis

	// Calculate scale
	maxValue := g.targetRate
	for _, v := range g.dataPoints {
		if v > maxValue {
			maxValue = v
		}
	}
	if maxValue == 0 {
		maxValue = 1
	}

	// Build sparkline graph
	lines := make([]string, graphHeight)
	for i := range lines {
		lines[i] = strings.Repeat(" ", graphWidth)
	}

	// Plot data points
	step := 1
	if len(g.dataPoints) > graphWidth {
		step = len(g.dataPoints) / graphWidth
	}

	for i := 0; i < len(g.dataPoints); i += step {
		value := g.dataPoints[i]
		normalized := value / maxValue
		barHeight := int(normalized * float64(graphHeight))

		x := i / step
		if x >= graphWidth {
			break
		}

		// Draw vertical bar
		for y := 0; y < barHeight && y < graphHeight; y++ {
			lineIdx := graphHeight - 1 - y
			line := []rune(lines[lineIdx])
			if x < len(line) {
				line[x] = '█'
				lines[lineIdx] = string(line)
			}
		}
	}

	// Draw target line if set
	if g.targetRate > 0 {
		targetHeight := int((g.targetRate / maxValue) * float64(graphHeight))
		if targetHeight >= 0 && targetHeight < graphHeight {
			lineIdx := graphHeight - 1 - targetHeight
			line := []rune(lines[lineIdx])
			for x := 0; x < len(line); x++ {
				if line[x] == ' ' {
					line[x] = '─'
				}
			}
			lines[lineIdx] = string(line)
		}
	}

	// Output graph with colors
	fmt.Fprintf(g, "[%s]", colorName(ColorGraph))
	for _, line := range lines {
		fmt.Fprintf(g, "  %s\n", line)
	}
	fmt.Fprintf(g, "[-]")

	// Add scale labels
	fmt.Fprintf(g, "\n  [%s]Max: %s[-]", colorName(ColorLabel), formatRate(maxValue))
	if g.targetRate > 0 {
		fmt.Fprintf(g, " [%s]Target: ─[-]", colorName(ColorWarning))
	}
}

// ConfigPanel displays current configuration
type ConfigPanel struct {
	*tview.TextView
	config *config.Config
}

// NewConfigPanel creates a new configuration panel
func NewConfigPanel(cfg *config.Config, title string) *ConfigPanel {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetBorderColor(ColorBorder).
		SetTitleColor(ColorHeader)

	panel := &ConfigPanel{
		TextView: tv,
		config:   cfg,
	}

	panel.Render()
	return panel
}

// Render renders the configuration
func (c *ConfigPanel) Render() {
	c.Clear()

	fmt.Fprintf(c, "[%s]┌─ CONNECTION ───────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(c, " [%s]URL:     [-]%s\n", colorName(ColorLabel), truncateString(c.config.Pulsar.ServiceURL, 30))
	fmt.Fprintf(c, " [%s]Topic:   [-]%s\n", colorName(ColorLabel), truncateString(c.config.Pulsar.Topic, 30))

	if c.config.Producer.NumProducers > 0 {
		fmt.Fprintf(c, "\n[%s]┌─ PRODUCER ─────────────────────────┐[-]\n", colorName(ColorHeader))
		fmt.Fprintf(c, " [%s]Workers: [-]%d\n", colorName(ColorLabel), c.config.Producer.NumProducers)
		fmt.Fprintf(c, " [%s]Batch:   [-]%d\n", colorName(ColorLabel), c.config.Producer.BatchingMaxSize)
		fmt.Fprintf(c, " [%s]MsgSize: [-]%s\n", colorName(ColorLabel), formatBytes(uint64(c.config.Producer.MessageSize)))
		fmt.Fprintf(c, " [%s]Compress:[-]%s\n", colorName(ColorLabel), c.config.Producer.CompressionType)
		fmt.Fprintf(c, " [%s]Target:  [-]%s\n", colorName(ColorLabel), formatRate(float64(c.config.Performance.TargetThroughput)))
	}

	if c.config.Consumer.NumConsumers > 0 {
		fmt.Fprintf(c, "\n[%s]┌─ CONSUMER ─────────────────────────┐[-]\n", colorName(ColorHeader))
		fmt.Fprintf(c, " [%s]Workers: [-]%d\n", colorName(ColorLabel), c.config.Consumer.NumConsumers)
		fmt.Fprintf(c, " [%s]Sub:     [-]%s\n", colorName(ColorLabel), truncateString(c.config.Consumer.SubscriptionName, 20))
		fmt.Fprintf(c, " [%s]Type:    [-]%s\n", colorName(ColorLabel), c.config.Consumer.SubscriptionType)
		fmt.Fprintf(c, " [%s]Queue:   [-]%d\n", colorName(ColorLabel), c.config.Consumer.ReceiverQueueSize)
	}
}

// StatusBar displays status information at the bottom
type StatusBar struct {
	*tview.TextView
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	return &StatusBar{
		TextView: tv,
	}
}

// Update updates the status bar
func (s *StatusBar) Update(running bool, workers int, elapsed time.Duration, shortcuts string) {
	s.Clear()

	statusText := "[green]RUNNING[-]"
	if !running {
		statusText = "[red]STOPPED[-]"
	}

	fmt.Fprintf(s, " %s | [cyan]Workers:[-] %d | [cyan]Elapsed:[-] %s | [yellow]%s[-]",
		statusText,
		workers,
		formatDuration(elapsed),
		shortcuts,
	)
}

// HelpModal displays keyboard shortcuts
type HelpModal struct {
	*tview.Modal
}

// NewHelpModal creates a new help modal
func NewHelpModal(shortcuts map[string]string) *HelpModal {
	text := "[::b]Keyboard Shortcuts[::-]\n\n"
	for key, desc := range shortcuts {
		text += fmt.Sprintf("[yellow]%s[-]  %s\n", key, desc)
	}

	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Will be handled by caller
		})

	return &HelpModal{
		Modal: modal,
	}
}

// Utility Functions

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

// formatBytes formats bytes for human-readable display
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatRate formats a rate for display
func formatRate(rate float64) string {
	if rate < 1000 {
		return fmt.Sprintf("%.0f msg/s", rate)
	}
	if rate < 1000000 {
		return fmt.Sprintf("%.2f K/s", rate/1000)
	}
	return fmt.Sprintf("%.2f M/s", rate/1000000)
}

// formatBandwidth formats bandwidth (bytes per second) for human-readable display
func formatBandwidth(bytesPerSecond float64) string {
	const unit = 1024.0
	if bytesPerSecond < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
	if bytesPerSecond < unit*unit {
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/unit)
	}
	if bytesPerSecond < unit*unit*unit {
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/(unit*unit))
	}
	return fmt.Sprintf("%.2f GB/s", bytesPerSecond/(unit*unit*unit))
}

// formatNumber formats large numbers with commas
func formatNumber(n uint64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	// Add commas
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// colorName returns the tview color name for a tcell color
func colorName(color tcell.Color) string {
	colorMap := map[tcell.Color]string{
		ColorHeader:    "cyan",
		ColorLabel:     "white",
		ColorGood:      "green",
		ColorWarning:   "yellow",
		ColorError:     "red",
		ColorGraph:     "blue",
		ColorBorder:    "darkcyan",
		ColorHighlight: "cyan",
	}
	if name, ok := colorMap[color]; ok {
		return name
	}
	return "white"
}

// createProgressBar creates a simple ASCII progress bar
func createProgressBar(current, total int64, width int) string {
	if total == 0 {
		return ""
	}

	percent := float64(current) / float64(total)
	filled := int(percent * float64(width))

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "="
		} else {
			bar += " "
		}
	}
	bar += fmt.Sprintf("] %.1f%%", percent*100)

	return bar
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minFloat64 returns the minimum of two float64s
func minFloat64(a, b float64) float64 {
	return math.Min(a, b)
}

// maxFloat64 returns the maximum of two float64s
func maxFloat64(a, b float64) float64 {
	return math.Max(a, b)
}

// LogBuffer is a thread-safe ring buffer for capturing log output
type LogBuffer struct {
	mu       sync.RWMutex
	lines    []string
	maxLines int
}

// NewLogBuffer creates a new log buffer with specified max lines
func NewLogBuffer(maxLines int) *LogBuffer {
	return &LogBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

// Write implements io.Writer interface for capturing log output
func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Split by newlines and add each line
	text := string(p)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Add timestamp
		timestamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)
		lb.lines = append(lb.lines, timestamped)

		// Keep only last maxLines
		if len(lb.lines) > lb.maxLines {
			lb.lines = lb.lines[len(lb.lines)-lb.maxLines:]
		}
	}

	return len(p), nil
}

// GetLines returns a copy of all buffered log lines
func (lb *LogBuffer) GetLines() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]string, len(lb.lines))
	copy(result, lb.lines)
	return result
}

// Clear clears all buffered lines
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.lines = lb.lines[:0]
}

// LogWindow displays captured log output
type LogWindow struct {
	*tview.Modal
	logBuffer *LogBuffer
	textView  *tview.TextView
}

// NewLogWindow creates a new log window
func NewLogWindow(logBuffer *LogBuffer) *LogWindow {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			// Auto-scroll to bottom
		})

	textView.SetBorder(true).
		SetTitle(" System Logs (Press L to close, C to clear) ").
		SetBorderColor(ColorBorder).
		SetTitleColor(ColorHeader)

	// Create modal wrapper
	modal := tview.NewModal().
		SetText("").
		AddButtons([]string{"Close"})

	lw := &LogWindow{
		Modal:     modal,
		logBuffer: logBuffer,
		textView:  textView,
	}

	return lw
}

// Update refreshes the log display
func (lw *LogWindow) Update() {
	lines := lw.logBuffer.GetLines()
	lw.textView.Clear()

	if len(lines) == 0 {
		fmt.Fprintf(lw.textView, "\n  [%s]No logs captured yet[-]", colorName(ColorLabel))
	} else {
		for _, line := range lines {
			fmt.Fprintf(lw.textView, "%s\n", line)
		}
	}

	// Scroll to bottom
	lw.textView.ScrollToEnd()
}

// GetTextView returns the internal text view for layout purposes
func (lw *LogWindow) GetTextView() *tview.TextView {
	return lw.textView
}

// ControlMenuItem represents a controllable parameter
type ControlMenuItem struct {
	Label       string
	Value       string
	Adjustable  bool
	Action      func(delta int) // Called when left/right arrow pressed
	ToggleFunc  func()           // Called when Enter/Space pressed (for buttons)
	MinValue    int
	MaxValue    int
	CurrentValue int
}

// ControlMenu displays an interactive control menu
type ControlMenu struct {
	*tview.TextView
	items         []*ControlMenuItem
	selectedIndex int
}

// NewControlMenu creates a new control menu
func NewControlMenu(title string) *ControlMenu {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetBorderColor(ColorBorder).
		SetTitleColor(ColorHeader)

	return &ControlMenu{
		TextView:      tv,
		items:         make([]*ControlMenuItem, 0),
		selectedIndex: 0,
	}
}

// AddItem adds a menu item
func (cm *ControlMenu) AddItem(item *ControlMenuItem) {
	cm.items = append(cm.items, item)
}

// MoveSelection moves the selection up or down
func (cm *ControlMenu) MoveSelection(delta int) {
	if len(cm.items) == 0 {
		return
	}
	cm.selectedIndex += delta
	if cm.selectedIndex < 0 {
		cm.selectedIndex = len(cm.items) - 1
	}
	if cm.selectedIndex >= len(cm.items) {
		cm.selectedIndex = 0
	}
}

// AdjustValue adjusts the currently selected item's value
func (cm *ControlMenu) AdjustValue(delta int) {
	if len(cm.items) == 0 || cm.selectedIndex >= len(cm.items) {
		return
	}
	item := cm.items[cm.selectedIndex]
	if item.Adjustable && item.Action != nil {
		item.Action(delta)
	}
}

// ActivateSelected activates the currently selected item (for buttons)
func (cm *ControlMenu) ActivateSelected() {
	if len(cm.items) == 0 || cm.selectedIndex >= len(cm.items) {
		return
	}
	item := cm.items[cm.selectedIndex]
	if item.ToggleFunc != nil {
		item.ToggleFunc()
	}
}

// Render renders the control menu
func (cm *ControlMenu) Render() {
	cm.Clear()

	if len(cm.items) == 0 {
		fmt.Fprintf(cm, "\n  [%s]No controls available[-]", colorName(ColorLabel))
		return
	}

	fmt.Fprintf(cm, "[%s]┌─ CONTROLS ─────────────────────────┐[-]\n", colorName(ColorHeader))
	fmt.Fprintf(cm, " [%s]Use ↑↓ to navigate, ←→ to adjust[-]\n\n", colorName(ColorLabel))

	for i, item := range cm.items {
		prefix := "  "
		suffix := ""
		labelColor := colorName(ColorLabel)

		if i == cm.selectedIndex {
			prefix = "[black:cyan] ▶ "
			suffix = " [-:-]"
			labelColor = "black"
		}

		if item.Adjustable {
			fmt.Fprintf(cm, "%s[%s]%s:[-] [%s]◀ %s ▶[-]%s\n",
				prefix, labelColor, item.Label, colorName(ColorGood), item.Value, suffix)
		} else if item.ToggleFunc != nil {
			// Button style
			fmt.Fprintf(cm, "%s[%s][%s %s][-]%s\n",
				prefix, colorName(ColorHighlight), item.Label, item.Value, suffix)
		} else {
			// Display only
			fmt.Fprintf(cm, "%s[%s]%s:[-] [%s]%s[-]%s\n",
				prefix, labelColor, item.Label, colorName(ColorGood), item.Value, suffix)
		}
	}
}