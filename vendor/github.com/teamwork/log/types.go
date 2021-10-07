package log

import "github.com/sirupsen/logrus"

// Level represents a supported logging level (copied from logrus.Level)
type Level uint8

// Log levels.
const (
	ErrorLevel = Level(logrus.ErrorLevel)
	WarnLevel  = Level(logrus.WarnLevel)
	InfoLevel  = Level(logrus.InfoLevel)
	DebugLevel = Level(logrus.DebugLevel)
)

const (
	fieldError        = "error"
	fieldErrorMessage = "error_message"
	fieldErrorTrace   = "error_trace"
	fieldHTTPRequest  = "http_request"
	fieldModule       = "module"
	fieldTags         = "tags"
)

// Fields type, used to pass to `WithFields`. An exact copy of logrus.Fields
type Fields map[string]interface{}

// Convert the Level to a string. E.g. InfoLevel becomes "info".
func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case ErrorLevel:
		return "error"
	case WarnLevel:
		return "warning"
	}

	return "unknown"
}
