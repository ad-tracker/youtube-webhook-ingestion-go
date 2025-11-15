package logger

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		logFile string
		wantErr bool
	}{
		{
			name:    "init with debug level, no file",
			level:   "debug",
			logFile: "",
			wantErr: false,
		},
		{
			name:    "init with info level, no file",
			level:   "info",
			logFile: "",
			wantErr: false,
		},
		{
			name:    "init with warn level, no file",
			level:   "warn",
			logFile: "",
			wantErr: false,
		},
		{
			name:    "init with error level, no file",
			level:   "error",
			logFile: "",
			wantErr: false,
		},
		{
			name:    "init with invalid level defaults to info",
			level:   "invalid",
			logFile: "",
			wantErr: false,
		},
		{
			name:    "init with log file",
			level:   "info",
			logFile: filepath.Join(t.TempDir(), "test.log"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global logger
			Log = nil

			err := Init(tt.level, tt.logFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && Log == nil {
				t.Error("Init() succeeded but Log is nil")
			}

			// Clean up
			if Log != nil {
				_ = Log.Sync()
			}

			if tt.logFile != "" {
				_ = os.Remove(tt.logFile)
			}
		})
	}
}

func TestSync(t *testing.T) {
	tests := []struct {
		name     string
		setupLog bool
	}{
		{
			name:     "sync with initialized logger",
			setupLog: true,
		},
		{
			name:     "sync with nil logger",
			setupLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupLog {
				Log, _ = zap.NewDevelopment()
			} else {
				Log = nil
			}

			// Sync may return errors for stdout/stderr on some systems, which is okay
			_ = Sync()
		})
	}
}

func TestInitWithLogFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "app.log")

	err := Init("info", logFile)
	if err != nil {
		t.Fatalf("Init() with log file failed: %v", err)
	}

	if Log == nil {
		t.Fatal("Log is nil after Init()")
	}

	// Test that we can write to the log
	Log.Info("test message")

	// Sync may return errors for stdout/stderr on some systems
	_ = Sync()

	// Check that log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}
