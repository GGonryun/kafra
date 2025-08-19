package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

func SetupLogger(verbose bool) *logrus.Logger {
	logger := logrus.New()

	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.SetFormatter(&logrus.TextFormatter{})
	
	// Always log to stdout - systemd/journalctl will handle log management
	logger.SetOutput(os.Stdout)

	return logger
}