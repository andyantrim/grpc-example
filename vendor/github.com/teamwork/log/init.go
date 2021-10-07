package log

import (
	"time"

	"github.com/evalphobia/logrus_sentry"
	raven "github.com/getsentry/raven-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/teamwork/log/grayloghook"
	"github.com/teamwork/utils/errorutil"
	graylog "gopkg.in/gemnasium/logrus-graylog-hook.v2"
)

// SentryTimeout is the timeout we wait for Sentry responses
const SentryTimeout = 20 * time.Second

// Options for the logger configuration. Passed to Init.
type Options struct {
	// SentryEnabled controls if the output of Error*() calls are sent to
	// Sentry. If disabled, it will output just to stderr.
	SentryEnabled bool

	// SentryDSN is the DSN string in Sentry.
	SentryDSN string

	// SentryEnvironment is the environment in Sentry; this is just to make it
	// easier to see where an error originated.
	SentryEnvironment string

	// SentryInAppPrefixes the prefixes that will be matched against the stack
	// frame. If the stack frame's package matches one of these prefixes sentry
	// will identify the stack frame as "in_app"
	SentryInAppPrefixes []string

	// GraylogEnabled controls if the output of Print*() and Debug*() calls are
	// sent to Graylog. If disabled, it will output just to stdout.
	GraylogEnabled bool

	// GraylogAddress is the Graylog address to connect to. This is usually as
	// "[ip]:port" without schema.
	GraylogAddress string

	// GraylogCompressType compression type the writer should use when sending
	// messages to the graylog2 server. Possible options are CompressGzip,
	// CompressZlib and NoCompress. By default CompressGzip will be used.
	GraylogCompressType graylog.CompressType

	// GraylogFieldPrefix will be prepended to any log fields.
	// This is to avoid problems with indexining conflicting field names,
	// e.g Desk might have a different shaped object for the session field
	// than CRM so whichever gets indexed first will determine the dynamic
	// mapping causing any non matching objects to fail later.
	//
	// It is recommended you to set this to avoid any indexing issues.
	GraylogFieldPrefix string

	// KeepStdout allow logger to output data also to stderr.
	KeepStdout bool

	// Debug enables the logging of Debug*() calls.
	Debug bool

	// AWSRegion is added to the Sentry context to see on which region the error
	// occurred. This is optional.
	AWSRegion string

	// Version is a version string to add (usually the git commit sha). This is
	// optional.
	Version string

	// Filter stack traces; see errorutil.StackFilter.
	StackFilter *errorutil.Patterns
}

// Use a global as passing it down to the Formatter is a bit tricky otherwise.
var stackFilter *errorutil.Patterns

// Init the logger. This will also initialize Graylog and/or Sentry connections
// if enabled.
func Init(opts Options) error {
	l, err := NewWithOptions(opts)
	if err != nil {
		return err
	}

	SetStandardLogger(l)

	return nil
}

// initSentry connects to the requested DSN, and sets the global sentryClient.
func initSentry(opts Options) (*raven.Client, error) {
	if opts.SentryDSN == "" {
		return nil, errors.New("No Sentry DSN provided")
	}

	client, err := raven.New(opts.SentryDSN)
	if err != nil {
		return nil, err
	}

	client.SetRelease(opts.Version)
	client.SetEnvironment(opts.SentryEnvironment)
	client.SetTagsContext(map[string]string{
		"region": opts.AWSRegion,
	})

	return client, nil
}

// enableSentry configures the logger to work with the provided Sentry DSN
func (l *Logger) enableSentry(client *raven.Client, inAppPrefixes []string) error {
	hook, err := logrus_sentry.NewAsyncWithClientSentryHook(client, []logrus.Level{logrus.ErrorLevel})
	if err != nil {
		return err
	}
	hook.Timeout = SentryTimeout
	hook.StacktraceConfiguration = logrus_sentry.StackTraceConfiguration{
		Enable:        true,
		Level:         logrus.ErrorLevel,
		Skip:          0,
		InAppPrefixes: inAppPrefixes,
	}
	l.AddHook(hook)
	l.addFlusher(hook)
	return nil
}

// EnableDebug turns on debug output.
func (l *Logger) EnableDebug() {
	l.logrus.Level = logrus.DebugLevel
}

// enableGraylog configures the logger to log to Graylog additionally.
func (l *Logger) enableGraylog(
	address string, compressType graylog.CompressType, fieldPrefix string,
) error {
	if address == "" {
		return errors.New("No graylog address string provided")
	}

	hook := grayloghook.New(address, fieldPrefix)
	hook.Writer().CompressionType = compressType

	l.AddHook(hook)
	l.addFlusher(hook)
	return nil
}

func (l *Logger) addFlusher(f flusher) {
	l.flushers = append(l.flushers, f)
}
