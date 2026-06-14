package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

// logger is the global structured logger instance.
var logger = initLogger()

func initLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(os.Stdout)

	// Use JSON format for structured logging in production
	if os.Getenv("LOG_FORMAT") == "json" {
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z",
		})
	} else {
		// Text format for development
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05",
		})
	}

	// Set log level from env, default to info
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}
	l.SetLevel(lvl)

	return l
}

// Convenience functions for structured logging with optional fields.
// These replace log.Printf calls throughout the codebase.

func logInfo(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		logger.WithFields(fields[0]).Info(msg)
	} else {
		logger.Info(msg)
	}
}

func logWarn(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		logger.WithFields(fields[0]).Warn(msg)
	} else {
		logger.Warn(msg)
	}
}

func logError(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		logger.WithFields(fields[0]).Error(msg)
	} else {
		logger.Error(msg)
	}
}

func logFatal(msg string, fields ...logrus.Fields) {
	if len(fields) > 0 {
		logger.WithFields(fields[0]).Fatal(msg)
	} else {
		logger.Fatal(msg)
	}
}
