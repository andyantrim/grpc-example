package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// Entry wraps a logrus.Entry
type Entry struct {
	e          *logrus.Entry
	logger     *Logger
	traceLines *[][]byte
}

// NewEntry returns a new entry for the specified Logger.
func NewEntry(l *Logger) *Entry {
	lines := make([][]byte, 0)
	return &Entry{
		e:          logrus.NewEntry(l.logrus),
		logger:     l,
		traceLines: &lines,
	}
}

func (e *Entry) dumpTrace(writers ...io.Writer) {
	if e.traceLines == nil {
		lines := make([][]byte, 0)
		e.traceLines = &lines
	}

	for _, line := range *e.traceLines {
		for _, writer := range writers {
			if _, err := writer.Write(line); err != nil {
				e.logger.mu.Lock()
				fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err) // nolint: errcheck
				e.logger.mu.Unlock()
			}
		}
	}

	e.ClearTraceLog()
}

// ClearTraceLog removes all the pending trace logging lines
func (e *Entry) ClearTraceLog() {
	lines := make([][]byte, 0)

	e.logger.tracemu.Lock()
	e.traceLines = &lines
	e.logger.tracemu.Unlock()
}

func (e Entry) log(msg string) []byte {
	e.e.Time = time.Now()
	e.e.Level = logrus.DebugLevel
	e.e.Message = msg

	serialized, err := e.logger.logrus.Formatter.Format(e.e)
	if err != nil {
		e.logger.mu.Lock()
		fmt.Fprintf(os.Stderr, "Failed to obtain reader, %v\n", err) // nolint: errcheck
		e.logger.mu.Unlock()
	}
	return serialized
}

// Tracef adds a message that will be logged when an Err* call happens
// The message will be written immediately if the Debug output is enabled.
func (e *Entry) Tracef(format string, args ...interface{}) {
	if e.e.Logger.Level == logrus.DebugLevel {
		e.Debugf(format, args...)
		return
	}
	e.logger.tracemu.Lock()
	*e.traceLines = append(*e.traceLines, e.log(fmt.Sprintf(format, args...)))
	e.logger.tracemu.Unlock()
}

// Trace adds a message that will be logged when an Err* call happens
// The message will be written immediately if the Debug output is enabled.
func (e *Entry) Trace(args ...interface{}) {
	if e.e.Logger.Level == logrus.DebugLevel {
		e.Debug(args...)
		return
	}
	e.logger.tracemu.Lock()
	*e.traceLines = append(*e.traceLines, e.log(fmt.Sprint(args...)))
	e.logger.tracemu.Unlock()
}

// Debug logs a message at level Debug.
func (e *Entry) Debug(args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Debug(args...)
	e.logger.mu.Unlock()
}

// Debugf logs a message at level Debug.
func (e *Entry) Debugf(format string, args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Debugf(format, args...)
	e.logger.mu.Unlock()
}

// Error logs a message at level Error.
// Trace logs will be dumped into the Output
func (e *Entry) Error(err error, args ...interface{}) {
	e.logger.mu.Lock()
	defer e.logger.mu.Unlock()

	out := new(bytes.Buffer)
	e.dumpTrace(out, e.e.Logger.Out)

	e = e.WithError(err)
	if out.String() != "" {
		e = e.WithField(fieldErrorTrace, out.String())
	}

	if len(args) == 0 {
		args = []interface{}{err}
	}
	e.e.Error(args...)
}

// Errorf logs a message at level Error.
// Trace logs will be dumped into the Output
func (e *Entry) Errorf(err error, format string, args ...interface{}) {
	e.logger.mu.Lock()
	defer e.logger.mu.Unlock()

	out := new(bytes.Buffer)
	e.dumpTrace(out, e.e.Logger.Out)

	e = e.WithError(err)
	if out.String() != "" {
		e = e.WithField(fieldErrorTrace, out.String())
	}

	e.e.Errorf(format, args...)
}

// Print logs a message at level Info.
func (e *Entry) Print(args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Print(args...)
	e.logger.mu.Unlock()
}

// Printf logs a message at level Info.
func (e *Entry) Printf(format string, args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Printf(format, args...)
	e.logger.mu.Unlock()
}

// Warn logs a message at level Warning
func (e *Entry) Warn(args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Warn(args...)
	e.logger.mu.Unlock()
}

// Warnf logs a message at level Warning
func (e *Entry) Warnf(format string, args ...interface{}) {
	e.logger.mu.Lock()
	e.e.Warnf(format, args...)
	e.logger.mu.Unlock()
}

// Info logs a message at level Info.
//
// This is just a wrapper for Print, as they are the same
// in sirupsen/logrus
func (e *Entry) Info(args ...interface{}) {
	e.Print(args...)
}

// Infof logs a message at level Info.
//
// This is just a wrapper for Printf, as they are the same
// in sirupsen/logrus
func (e *Entry) Infof(format string, args ...interface{}) {
	e.Printf(format, args...)
}

// WithError returns a new entry with an error added as field.
func (e *Entry) WithError(err error) *Entry {
	if err == nil {
		err = errors.New("(nil)")
	}

	return e.WithFields(Fields{
		fieldError:        addStackTrace(err),
		fieldErrorMessage: err.Error(),
	})
}

// WithHTTPRequest returns a new entry with an *http.Request added as field.
func (e *Entry) WithHTTPRequest(req *http.Request) *Entry {
	encoded := raven.NewHttp(req)

	// remove sensitive data
	for k, v := range encoded.Headers {
		switch strings.ToLower(k) {
		case "cookie":
			encoded.Headers[k] = rxPassword.ReplaceAllString(v, `${1}${2}${3}${4}${5}[FILTERED]${7}`)
		case "authorization":
			encoded.Headers[k] = "[FILTERED]"
		}
	}
	encoded.Cookies = rxPassword.ReplaceAllString(encoded.Cookies, `${1}${2}${3}${4}${5}[FILTERED]${7}`)

	return e.WithField(fieldHTTPRequest, encoded)
}

// WithField returns a new entry with a single field added.
func (e *Entry) WithField(key string, value interface{}) *Entry {
	return &Entry{
		e:          e.e.WithField(key, value),
		logger:     e.logger,
		traceLines: e.traceLines,
	}
}

// Module adds a module field and returns a new Entry.
func (e *Entry) Module(module string) *Entry {
	return e.WithField(fieldModule, module)
}

// GetModule gets the module field.
func (e *Entry) GetModule() string {
	m, _ := e.e.Data[fieldModule].(string)
	return m
}

// WithFields returns a new entry with a map of fields added.
func (e *Entry) WithFields(fields Fields) *Entry {
	return &Entry{
		e:          e.e.WithFields(logrus.Fields(fields)),
		logger:     e.logger,
		traceLines: e.traceLines,
	}
}

// WithSentryTags converts the Fields into Sentry Tags and duplicates these
// Fields into Graylog Fields, allowing these to be searchable in each system.
func (e *Entry) WithSentryTags(fields Fields) *Entry {
	var tags raven.Tags
	if e.e.Data[fieldTags] != nil {
		if t, ok := e.e.Data[fieldTags].(raven.Tags); ok {
			tags = t
		}
	}

	if tags == nil {
		// prepare a slice of the exact size we need
		tags = make(raven.Tags, 0, len(fields))
	}

	for k, v := range fields {
		tags = append(tags, raven.Tag{Key: k, Value: fmt.Sprintf("%v", v)})
	}

	return e.WithField(fieldTags, tags)
}

// WithDDog adds DataDog APM tracer identification to the log entry. It is used
// to connect the log entry with the request.
//
// https://docs.datadoghq.com/tracing/advanced/connect_logs_and_traces/?tab=go#manual-trace-id-injection
func (e *Entry) WithDDog(ctx context.Context) *Entry {
	var tracerID, spanID uint64
	if span, ok := tracer.SpanFromContext(ctx); ok {
		spanID = span.Context().SpanID()
		tracerID = span.Context().TraceID()
	}

	return e.WithFields(Fields{
		"dd.trace_id": tracerID,
		"dd.span_id":  spanID,
	})
}
