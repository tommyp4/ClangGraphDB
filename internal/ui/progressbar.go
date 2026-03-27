package ui

import (
	"fmt"
	"graphdb/internal/progress"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressBar is a simple console progress bar.
type ProgressBar struct {
        total       int64
        current     int64
        description string
        width       int
        start       time.Time
        mu          sync.Mutex
        writer      io.Writer
        isSpinner   bool
        stopSpinner chan struct{}
        Format      func(int64, int64) string // Custom format function (current, total) -> string
        lastRender  time.Time
        isTTY       bool
        lastPercent int // Track last printed percentage for non-TTY
}

func isTerminal(w io.Writer) bool {
        if f, ok := w.(*os.File); ok {
                stat, err := f.Stat()
                if err == nil {
                        return (stat.Mode() & os.ModeCharDevice) != 0
                }
        }
        return false
}

// IsAnyActive returns true if any progress bar or spinner is currently active.
func IsAnyActive() bool {
	return progress.IsAnyActive()
}

func registerActive(pb *ProgressBar) {
	progress.RegisterActive(pb)
}

func unregisterActive(pb *ProgressBar) {
	progress.UnregisterActive(pb)
}

// NewProgressBar creates a new progress bar with a known total.
func NewProgressBar(total int64, description string) *ProgressBar {
	pb := &ProgressBar{
		total:       total,
		description: description,
		width:       20,
		start:       time.Now(),
		writer:      os.Stderr,
		Format:      FormatCountFn,
		isTTY:       isTerminal(os.Stderr),
		lastPercent: -1,
		lastRender:  time.Now(),
	}
	registerActive(pb)
	if !pb.isTTY {
		fmt.Fprintf(pb.writer, "%s (Total: %d)\n", description, total)
	}
	return pb
}

// FormatCountFn formats integers with commas for readability.
func FormatCountFn(current, total int64) string {
	format := func(n int64) string {
		s := fmt.Sprintf("%d", n)
		if len(s) <= 3 {
			return s
		}
		var res []string
		for len(s) > 3 {
			res = append([]string{s[len(s)-3:]}, res...)
			s = s[:len(s)-3]
		}
		res = append([]string{s}, res...)
		return strings.Join(res, ",")
	}
	return fmt.Sprintf("%s/%s", format(current), format(total))
}

// NewSpinner creates a new spinner for indeterminate progress.
func NewSpinner(description string) *ProgressBar {
	pb := &ProgressBar{
		description: description,
		start:       time.Now(),
		writer:      os.Stderr,
		isSpinner:   true,
		stopSpinner: make(chan struct{}),
		Format: func(current, total int64) string {
			return fmt.Sprintf("%d", current)
		},
		isTTY: isTerminal(os.Stderr),
	}
	registerActive(pb)
	if pb.isTTY {
		go pb.spin()
	} else {
		fmt.Fprintf(pb.writer, "%s...\n", description)
	}
	return pb
}
func (pb *ProgressBar) spin() {
	chars := []rune{'|', '/', '-', '\\'}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-pb.stopSpinner:
			return
		case <-ticker.C:
			pb.mu.Lock()
			// Use \r to overwrite the line
			desc := pb.description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			fmt.Fprintf(pb.writer, "\r%-30s %c %d items processed (%s)   ", desc, chars[i%len(chars)], pb.current, time.Since(pb.start).Round(time.Second))
			pb.mu.Unlock()
			i++
		}
	}
}

// UpdateDescription updates the description text of the progress bar.
func (pb *ProgressBar) UpdateDescription(text string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.description = text
	if !pb.isSpinner {
		pb.render()
	}
}

// Add increments the progress.
func (pb *ProgressBar) Add(n int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.current += n
	if !pb.isSpinner {
		pb.render()
	}
}

// SetTotal sets the total value (useful if total changes or is discovered later).
func (pb *ProgressBar) SetTotal(total int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.total = total
}

func (pb *ProgressBar) render() {
        percent := float64(pb.current) / float64(pb.total)
        if percent > 1.0 {
                percent = 1.0
        }
        currentPercent := int(percent * 100)

        if !pb.isTTY {
                now := time.Now()
                // In non-TTY mode, print at 1% increments (was 10%)
                // OR if at least 30 seconds have passed since last update.
                shouldUpdate := currentPercent >= pb.lastPercent+1 || pb.current == pb.total
                if !shouldUpdate && !pb.lastRender.IsZero() && now.Sub(pb.lastRender) >= 30*time.Second {
                        shouldUpdate = true
                }

                if shouldUpdate {
                        progressStr := pb.Format(pb.current, pb.total)
                        fmt.Fprintf(pb.writer, "  ... %d%% (%s)\n", currentPercent, progressStr)
                        pb.lastPercent = currentPercent
                        pb.lastRender = now
                }
                return
        }

        now := time.Now()
        // Rate limit: at most once every 250ms OR when percentage changes.
        // This prevents flooding when \r isn't interpreted correctly,
        // but still allows seeing progress for slow tasks.
        if pb.current < pb.total {
                hasTimePassed := !pb.lastRender.IsZero() && now.Sub(pb.lastRender) >= 250*time.Millisecond
                hasPercentChanged := currentPercent > pb.lastPercent
                if !hasPercentChanged && !hasTimePassed {
                        return
                }
        }
        pb.lastRender = now
        pb.lastPercent = currentPercent

        filled := int(float64(pb.width) * percent)
        bar := strings.Repeat("=", filled) + strings.Repeat(" ", pb.width-filled)
        if filled > 0 {
                // Add arrow head
                if filled < pb.width {
                        bar = strings.Repeat("=", filled-1) + ">" + strings.Repeat(" ", pb.width-filled)
                } else {
                        bar = strings.Repeat("=", filled)
                }
        }

        // Use \r to overwrite the line
        progressStr := pb.Format(pb.current, pb.total)

        desc := pb.description
        if len(desc) > 30 {
                desc = desc[:27] + "..."
        }

        fmt.Fprintf(pb.writer, "\r%-30s [%s] %.1f%% (%s)   ", desc, bar, percent*100, progressStr)
}
// Finish completes the progress bar.
func (pb *ProgressBar) Finish() {
	unregisterActive(pb)
	if pb.isSpinner {
		close(pb.stopSpinner)
		// Give the spinner goroutine a moment to stop writing
		time.Sleep(10 * time.Millisecond)
	}
	pb.mu.Lock()
	defer pb.mu.Unlock()
	fmt.Fprintln(pb.writer) // New line
}

// SetFormat allows changing the format function.
func (pb *ProgressBar) SetFormat(f func(current, total int64) string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.Format = f
}

// FormatBytesFn formats bytes into a human-readable string.
func FormatBytesFn(current, total int64) string {
	const unit = 1024
	if total < unit {
		return fmt.Sprintf("%d B / %d B", current, total)
	}
	div := int64(unit)
	exp := 0
	for n := total / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	valCurrent := float64(current) / float64(div)
	valTotal := float64(total) / float64(div)
	return fmt.Sprintf("%.2f / %.2f %cB", valCurrent, valTotal, "KMGTPE"[exp])
}

// ByteReader wraps an io.Reader to update a progress bar.
type ByteReader struct {
	Reader io.Reader
	Pb     *ProgressBar
}

func (r *ByteReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		r.Pb.Add(int64(n))
	}
	return n, err
}
