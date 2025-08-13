package logging

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// SetupLogger configures a logger with the given verbosity and log path
// If logPath is empty or the file cannot be opened, it falls back to stdout/stderr
func SetupLogger(verbose bool, logPath string) *logrus.Logger {
	logger := logrus.New()
	
	// Set log level based on verbose flag
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Configure output destination
	if logPath != "" {
		if logFile, err := openLogFile(logPath); err == nil {
			// Successfully opened log file - use both file and stdout
			logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
			logger.WithField("log_file", logPath).Debug("Logging to file and stdout")
		} else {
			// Failed to open log file - log error and fall back to stdout
			logger.WithError(err).WithField("log_path", logPath).Warn("Failed to open log file, using stdout only")
		}
	}
	// If logPath is empty, logger defaults to stdout

	return logger
}

// openLogFile opens a log file for writing, creating parent directories if needed
func openLogFile(logPath string) (*os.File, error) {
	// Create parent directory if it doesn't exist
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Open log file for append, create if doesn't exist
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return logFile, nil
}

// SetupLoggerFromConfig creates a logger using configuration from the config struct
func SetupLoggerFromConfig(verbose bool, config interface{ GetLogPath() string }) *logrus.Logger {
	logPath := ""
	if config != nil {
		logPath = config.GetLogPath()
	}
	return SetupLogger(verbose, logPath)
}