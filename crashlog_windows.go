//go:build windows

package main

// Crash logging: Go runtime fatal errors and any panic inside a window
// procedure are written to TelemetrySim-crash.log next to the .exe.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

var crashPath string

func initCrashLog() {
	dir := os.TempDir()
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	crashPath = filepath.Join(dir, "TelemetrySim-crash.log")
	f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		crashPath = filepath.Join(os.TempDir(), "TelemetrySim-crash.log")
		if f, err = os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err != nil {
			return
		}
	}
	debug.SetCrashOutput(f, debug.CrashOptions{})
	f.Close() // SetCrashOutput keeps its own duplicate
}

// cleanupCrashLog removes the log on a normal exit if nothing was written.
func cleanupCrashLog() {
	if st, err := os.Stat(crashPath); err == nil && st.Size() == 0 {
		os.Remove(crashPath)
	}
}

func logPanic(where string, e any) {
	f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  panic in %s: %v\n%s\n\n", time.Now().Format(time.RFC3339), where, e, debug.Stack())
}
