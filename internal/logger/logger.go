package logger

import (
	"io"
	"log"
	"os"
)

var (
	// Query is a logger for verbose queries.
	Query *log.Logger
)

func init() {
	// Default to silent
	Query = log.New(io.Discard, "", 0)
}

// Init initializes the logging system.
func Init(logFile string) {
	if logFile == "" {
		// If no log file is specified, Query logger remains silent (io.Discard)
		// and the global logger continues to write to os.Stderr (default).
		Query = log.New(io.Discard, "", 0)
		return
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %v", logFile, err)
	}

	// For Queries, we ONLY want them in the file, never in Stderr.
	Query = log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	
	// Also configure the global logger to write ONLY to the file, not to Stderr.
	// This ensures that when logging is enabled, the terminal remains clean.
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}
