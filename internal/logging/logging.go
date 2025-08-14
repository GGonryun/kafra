package logging

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func SetupLogger(verbose bool, logPath string) *logrus.Logger {
	logger := logrus.New()
	
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	if logPath != "" {
		if logFile, err := openLogFile(logPath); err == nil {
			logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
			logger.WithField("log_file", logPath).Debug("Logging to file and stdout")
		} else {
			logger.WithError(err).WithField("log_path", logPath).Warn("Failed to open log file, using stdout only")
		}
	}

	return logger
}

func openLogFile(logPath string) (*os.File, error) {
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return logFile, nil
}

func SetupLoggerFromConfig(verbose bool, config interface{ GetLogPath() string }) *logrus.Logger {
	logPath := ""
	if config != nil {
		logPath = config.GetLogPath()
	}
	return SetupLogger(verbose, logPath)
}