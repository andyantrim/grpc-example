package log

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/teamwork/utils/errorutil"
)

// TimeFormat to use for log messages.
const TimeFormat = "Jan 02 15:04:05"

var (
	rxEmail    = regexp.MustCompile(`(?i)("|')?(email|e-mail)("|')?(\s*[:=]\s*)("|')?([0-9A-Za-z\-\.]+@[0-9A-Za-z\-\.]+)("|')?`) // nolint: lll
	rxPassword = regexp.MustCompile(`(?i)("|')?(password|secret|tw-?auth)("|')?(\s*[:=]\s*)("|')?([\w-]+)("|')?`)

	defaultFilter = func(b []byte) []byte {
		b = rxEmail.ReplaceAll(b, []byte(`${1}${2}${3}${4}${5}[FILTERED]${7}`))
		b = rxPassword.ReplaceAll(b, []byte(`${1}${2}${3}${4}${5}[FILTERED]${7}`))
		return b
	}
)

// borrowed from https://github.com/sirupsen/logrus/blob/master/text_formatter.go#L72-L79
func checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

type twFormatter struct {
	ForceColors   bool
	DisableColors bool
}

// Format the log entry to be human-readable. This is used only when outputting
// to std{out,err} or a file; not when sending to Graylog or Sentry.
func (f *twFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	useColors := !f.DisableColors && (f.ForceColors || checkIfTerminal(os.Stdout))

	err := printMessage(b, entry, useColors)
	if err != nil {
		return nil, err
	}

	errBytes := extractError(entry.Data, useColors)
	if _, ok := entry.Data[fieldHTTPRequest].(*http.Request); ok {
		delete(entry.Data, fieldHTTPRequest)
	}

	err = printData(b, entry.Data, useColors)
	if err != nil {
		return nil, err
	}

	_, err = b.Write(errBytes)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

const (
	fgRed   = 31
	fgBlue  = 34
	fgWhite = 37
	bgWhite = 57
)

var colors = map[logrus.Level]uint8{
	logrus.DebugLevel: fgWhite,
	logrus.ErrorLevel: fgRed,  // red
	logrus.InfoLevel:  fgBlue, // blue
}

func colorize(text string, color uint8, colored bool) string {
	if !colored {
		return text
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, text)
}

func printMessage(b *bytes.Buffer, entry *logrus.Entry, color bool) error {
	pid := colorize(fmt.Sprintf("%d", os.Getpid()), bgWhite, color)

	levelText := colorize(strings.ToUpper(entry.Level.String())[0:4],
		colors[entry.Level], color)

	module, ok := entry.Data[fieldModule].(string)
	if ok {
		delete(entry.Data, fieldModule) // So it isn't repeated
		module = " (" + module + ")"
	}

	_, err := fmt.Fprintf(b, "%s [%s] %s:%s %s\n", // nolint: errcheck
		entry.Time.Format(TimeFormat), pid, levelText, module,
		strings.TrimSpace(entry.Message))
	return err
}

func printData(b *bytes.Buffer, data map[string]interface{}, color bool) error {
	if len(data) == 0 {
		return nil
	}
	if err := b.WriteByte('\t'); err != nil {
		return err
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	//    unhandled := make(map[string]interface{})

	pairs := make([][]byte, 0, len(data))
	for _, k := range keys {
		v := data[k]
		var formatted string
		switch v.(type) {
		case string:
			formatted = fmt.Sprintf("%s=%q", k, v)
		case url.URL, *url.URL:
			formatted = fmt.Sprintf("%s=%s", k, v)
		default:
			formatted = fmt.Sprintf("%s=%#v", k, v)
		}
		pairs = append(pairs, []byte(formatted))
	}
	if _, err := b.Write(bytes.Join(pairs, []byte(" "))); err != nil {
		return err
	}

	return b.WriteByte('\n')
}

func extractError(data map[string]interface{}, color bool) []byte {
	err, ok := data[fieldError].(error)
	if !ok {
		return nil
	}

	delete(data, fieldError)
	if err == nil {
		return nil
	}

	// The error message field is just for sentry; it's a duplicate
	// of err.Error(), which we handle separately here.
	delete(data, fieldErrorMessage)
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "\tError: %s\n", err.Error()) // nolint: errcheck
	if _, ok := err.(causer); ok {
		cause := errors.Cause(err)
		// No need to repeat an identical message
		if cause != nil && cause.Error() != err.Error() {
			fmt.Fprintf(b, "\tCause: %s\n", cause.Error()) // nolint: errcheck
		}
	}

	err = earliestStackTracer(err)
	if err != nil {
		if stackFilter != nil {
			err = errorutil.FilterTrace(err, stackFilter)
		}

		st := err.(stackTracer)

		fmt.Fprint(b, "\tStacktrace:\n") // nolint: errcheck
		for _, line := range strings.Split(fmt.Sprintf("%+v", st.StackTrace()), "\n") {
			if line != "" {
				fmt.Fprintf(b, "\t\t%s\n", line) // nolint: errcheck
			}
		}
	}

	return b.Bytes()
}

// SensitiveJSONFormatter formats the entry using the logrus.JSONFormatter,
// filtering the response. Useful to remove sensitive data from the logs.
type SensitiveJSONFormatter struct {
	logrus.JSONFormatter

	// Allows a custom filtering of the output.
	Filter func([]byte) []byte
}

// Format renders a single log entry.
func (s *SensitiveJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b, err := s.JSONFormatter.Format(entry)
	if s.Filter == nil {
		s.Filter = defaultFilter
	}
	return s.Filter(b), err
}
