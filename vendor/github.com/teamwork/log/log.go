package log

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var stdLogger = New()

// SetStandardLogger sets the global, standard logger
func SetStandardLogger(l *Logger) {
	stdLogger = l
}

// StandardLogger returns the standard logger
func StandardLogger() *Logger {
	return stdLogger
}

// New creates a new logger.
func New() *Logger {
	l := logrus.New()
	l.Formatter = &twFormatter{}
	return &Logger{
		logrus: l,
	}
}

// NewWithOptions creates a new logger with provided options.
func NewWithOptions(opts Options) (*Logger, error) {
	l := New()

	if opts.SentryEnabled {
		client, err := initSentry(opts)
		if err != nil {
			return nil, errors.Wrap(err, "cannot initialize Sentry")
		}
		if err := l.enableSentry(client, opts.SentryInAppPrefixes); err != nil {
			return nil, errors.Wrap(err, "cannot enable sentry logging")
		}
	} else {
		Module("logger").Print("Sentry disabled; not logging to Sentry")
	}

	if opts.GraylogEnabled {
		err := l.enableGraylog(opts.GraylogAddress, opts.GraylogCompressType, opts.GraylogFieldPrefix)
		if err != nil {
			return nil, errors.Wrap(err, "failure logging to graylog")
		}
	} else {
		Module("logger").Print("Graylog disabled; NOT logging to stdout; Enable KeepStdout to log to stdout")
	}

	if opts.StackFilter != nil {
		stackFilter = opts.StackFilter
	}

	if !opts.KeepStdout {
		l.SetOutput(ioutil.Discard)
	}

	if opts.Debug {
		l.EnableDebug()
	}

	return l, nil
}

// NewWithLogrus creates a new logger with the given logrus Logger.
func NewWithLogrus(l *logrus.Logger) *Logger {
	return &Logger{
		logrus: l,
	}
}

// Logger is a wrapper around a logrus.Logger
type Logger struct {
	tracemu  sync.Mutex
	mu       sync.Mutex
	logrus   *logrus.Logger
	flushers []flusher
}

type flusher interface {
	Flush()
}

// Flush ensures that the underlying hooks' queues are flushed.
func (l *Logger) Flush() {
	if l.flushers == nil {
		return
	}
	wg := &sync.WaitGroup{}
	for _, f := range l.flushers {
		wg.Add(1)
		go func(f flusher) {
			defer wg.Done()
			f.Flush()
		}(f)
	}
	wg.Wait()
}

// Flush ensures that the standard logger's hooks' queues are flushed.
func Flush() {
	stdLogger.Flush()
}

// SetFormatter sets the Logger's formatter
func (l *Logger) SetFormatter(f logrus.Formatter) {
	l.logrus.Formatter = f
}

// SetOutput sets the Logger's output writer
func (l *Logger) SetOutput(out io.Writer) {
	l.logrus.Out = out
}

// AddHook adds a hook to the logger
func (l *Logger) AddHook(hook logrus.Hook) {
	l.logrus.Hooks.Add(hook)
}

// Debug logs a message at level Debug.
func (l *Logger) Debug(args ...interface{}) {
	NewEntry(l).Debug(args...)
}

// Debug logs a message at Debug Info on the standard logger.
func Debug(args ...interface{}) {
	NewEntry(stdLogger).Debug(args...)
}

// Debugf logs a message at level Debug.
func (l *Logger) Debugf(fmt string, args ...interface{}) {
	NewEntry(l).Debugf(fmt, args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(fmt string, args ...interface{}) {
	NewEntry(stdLogger).Debugf(fmt, args...)
}

// Print logs a message at level Info.
func (l *Logger) Print(args ...interface{}) {
	NewEntry(l).Print(args...)
}

// Print logs a message at level Info on the standard logger.
func Print(args ...interface{}) {
	NewEntry(stdLogger).Print(args...)
}

// Printf logs a message at level Info.
func (l *Logger) Printf(fmt string, args ...interface{}) {
	NewEntry(l).Printf(fmt, args...)
}

// Printf logs a message at level Info on the standard logger.
func Printf(fmt string, args ...interface{}) {
	NewEntry(stdLogger).Printf(fmt, args...)
}

// Info logs a message at level Info.
func (l *Logger) Info(args ...interface{}) {
	NewEntry(l).Print(args...)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	NewEntry(stdLogger).Print(args...)
}

// Infof logs a message at level Info.
func (l *Logger) Infof(fmt string, args ...interface{}) {
	NewEntry(l).Printf(fmt, args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(fmt string, args ...interface{}) {
	NewEntry(stdLogger).Printf(fmt, args...)
}

// Warn logs a message at level Warn.
func (l *Logger) Warn(args ...interface{}) {
	NewEntry(l).Warn(args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	NewEntry(stdLogger).Warn(args...)
}

// Warnf logs a message at level Warn.
func (l *Logger) Warnf(fmt string, args ...interface{}) {
	NewEntry(l).Warnf(fmt, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(fmt string, args ...interface{}) {
	NewEntry(stdLogger).Warnf(fmt, args...)
}

// Error logs a message at level Error.
func (l *Logger) Error(err error, args ...interface{}) {
	NewEntry(l).Error(err, args...)
}

// Error logs a message at level Error on the standard logger.
func Error(err error, args ...interface{}) {
	NewEntry(stdLogger).Error(err, args...)
}

// Errorf logs a message at level Error.
func (l *Logger) Errorf(err error, fmt string, args ...interface{}) {
	NewEntry(l).Errorf(err, fmt, args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(err error, fmt string, args ...interface{}) {
	NewEntry(stdLogger).Errorf(err, fmt, args...)
}

// WithError adds an error as single field to a the standard logger.
func WithError(err error) *Entry {
	return NewEntry(stdLogger).WithError(err)
}

// WithError adds an error as single field to a new log Entry.
func (l *Logger) WithError(err error) *Entry {
	return NewEntry(l).WithError(err)
}

// WithHTTPRequest adds an *http.Request field to the standard logger.
func WithHTTPRequest(req *http.Request) *Entry {
	return NewEntry(stdLogger).WithHTTPRequest(req)
}

// WithHTTPRequest adds an *http.Request field to the standard logger.
func (l *Logger) WithHTTPRequest(req *http.Request) *Entry {
	return NewEntry(l).WithHTTPRequest(req)
}

// WithField adds a field to the standard logger and returns a new Entry.
func WithField(key string, value interface{}) *Entry {
	return NewEntry(stdLogger).WithField(key, value)
}

// WithField adds a field to the log entry, note that it doesn't log until you
// call Debug, Print, or Error. It only creates a log entry. If you want
// multiple fields, use `WithFields`.
func (l *Logger) WithField(key string, value interface{}) *Entry {
	return NewEntry(l).WithField(key, value)
}

// WithDDog adds DataDog APM tracer identification to the standard logger. It is
// used to connect the log entry with the request.
//
// https://docs.datadoghq.com/tracing/advanced/connect_logs_and_traces/?tab=go#manual-trace-id-injection
func WithDDog(ctx context.Context) *Entry {
	var tracerID, spanID uint64
	if span, ok := tracer.SpanFromContext(ctx); ok {
		spanID = span.Context().SpanID()
		tracerID = span.Context().TraceID()
	}

	return NewEntry(stdLogger).WithFields(Fields{
		"dd.trace_id": tracerID,
		"dd.span_id":  spanID,
	})
}

// WithDDog adds DataDog APM tracer identification to the log entry. It is used
// to connect the log entry with the request.
//
// https://docs.datadoghq.com/tracing/advanced/connect_logs_and_traces/?tab=go#manual-trace-id-injection
func (l *Logger) WithDDog(ctx context.Context) *Entry {
	var tracerID, spanID uint64
	if span, ok := tracer.SpanFromContext(ctx); ok {
		spanID = span.Context().SpanID()
		tracerID = span.Context().TraceID()
	}

	return NewEntry(l).WithFields(Fields{
		"dd.trace_id": tracerID,
		"dd.span_id":  spanID,
	})
}

// Module adds a module field to the standard logger and returns a new Entry.
func Module(module string) *Entry {
	return NewEntry(stdLogger).Module(module)
}

// Module adds a module field and returns a new Entry.
func (l *Logger) Module(module string) *Entry {
	return NewEntry(l).Module(module)
}

// WithFields adds a map of fields to the standard logger and returns a new Entry.
func WithFields(fields Fields) *Entry {
	return stdLogger.WithFields(fields)
}

// WithSentryTags converts the Fields into Sentry Tags and duplicates these Fields into Graylog Fields.
// Allowing these to be searchable in each system.
func WithSentryTags(fields Fields) *Entry {
	tags := raven.Tags{}
	for k, v := range fields {
		tags = append(tags, raven.Tag{Key: k, Value: fmt.Sprintf("%v", v)})
	}
	fields[fieldTags] = tags

	return stdLogger.WithFields(fields)
}

// WithFields adds a map of fields to the log entry, note that it doesn't log
// until you call Debug, Print, or Error. It only creates a log entry. If you
// want multiple fields, use `WithFields`.
func (l *Logger) WithFields(fields Fields) *Entry {
	e := l.logrus.WithFields(logrus.Fields(fields))
	lines := make([][]byte, 0)
	return &Entry{e: e, logger: l, traceLines: &lines}
}
