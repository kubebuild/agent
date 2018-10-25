package utils

import (
	"github.com/sirupsen/logrus"
)

// LogFormatter Ensures all entries are logged with standard fields for the application
type LogFormatter struct {
	name      string
	version   string
	formatter logrus.Formatter
}

// NewLogFormatter returns a LogFormatter for the specified config.
func NewLogFormatter(name string, version string, formatter logrus.Formatter) *LogFormatter {
	return &LogFormatter{
		name:      name,
		version:   version,
		formatter: formatter,
	}
}

// Format adds standard fields to all log output.
func (f *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	wrappedEntry := entry.WithFields(logrus.Fields{
		"version": f.version,
	})
	wrappedEntry.Time = entry.Time
	wrappedEntry.Message = entry.Message
	wrappedEntry.Level = entry.Level
	return f.formatter.Format(wrappedEntry)
}
