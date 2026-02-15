package ui

import (
	"fmt"
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
}

// NewProgressBar creates a new progress bar with a known total.
func NewProgressBar(total int64, description string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		description: description,
		width:       40,
		start:       time.Now(),
		writer:      os.Stderr,
		Format: func(current, total int64) string {
			return fmt.Sprintf("%d/%d", current, total)
		},
	}
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
	}
	go pb.spin()
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
			fmt.Fprintf(pb.writer, "\r%s %c %d items processed (%s)   ", pb.description, chars[i%len(chars)], pb.current, time.Since(pb.start).Round(time.Second))
			pb.mu.Unlock()
			i++
		}
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
	fmt.Fprintf(pb.writer, "\r%s [%s] %.1f%% (%s)   ", pb.description, bar, percent*100, progressStr)
}

// Finish completes the progress bar.
func (pb *ProgressBar) Finish() {
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
