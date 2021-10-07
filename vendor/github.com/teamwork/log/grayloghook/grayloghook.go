package grayloghook

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/teamwork/utils/httputilx"
	graylog "gopkg.in/gemnasium/logrus-graylog-hook.v2"
)

const (
	prefixIgnore = "g-"
)

// GraylogHook adds a new layer over gemnasium Graylog hook to handle specific
// cases when trigerred.
type GraylogHook struct {
	*graylog.GraylogHook
	fieldPrefix string
}

// Fire is called when a log event is fired.
func (g *GraylogHook) Fire(entry *logrus.Entry) error {
	// don't modify entry as it could be used by other hooks.
	// we need to make some changes to avoid indexing issues on Elasticsearch
	// change http.Request to a simple type and prefix any field names if the
	// option is given to avoid issues with dynamic types between different
	// library users.
	newData := make(map[string]interface{})
	for k, v := range entry.Data {
		newKey := k
		if g.fieldPrefix != "" && k != logrus.ErrorKey && !strings.HasPrefix(k, prefixIgnore) {
			newKey = fmt.Sprintf("%s-%s", g.fieldPrefix, k)
		}
		switch d := v.(type) {
		case *http.Request:
			newData[newKey] = newHTTPRequest(d)
		default:
			newData[newKey] = v
		}
	}

	newEntry := &logrus.Entry{
		Logger:  entry.Logger,
		Data:    newData,
		Time:    entry.Time,
		Level:   entry.Level,
		Message: entry.Message,
	}

	return g.GraylogHook.Fire(newEntry)
}

// New creates a hook to be added to an instance of logger.
func New(addr, fieldPrefix string) *GraylogHook {
	return &GraylogHook{
		GraylogHook: graylog.NewAsyncGraylogHook(addr, nil),
		fieldPrefix: fieldPrefix,
	}
}

type httpRequest struct {
	Method        string      `json:"method"`
	Path          string      `json:"path"`
	QueryString   url.Values  `json:"queryString"`
	Proto         string      `json:"proto"`
	Header        http.Header `json:"header"`
	Body          string      `json:"body"`
	ContentLength int64       `json:"contentLength"`
	Host          string      `json:"host"`
	RemoteAddr    string      `json:"remoteAddr"`
}

func newHTTPRequest(r *http.Request) httpRequest {
	h := httpRequest{
		Method:        r.Method,
		Path:          r.URL.Path,
		QueryString:   r.URL.Query(),
		Proto:         r.Proto,
		Header:        r.Header,
		ContentLength: r.ContentLength,
		Host:          r.Host,
		RemoteAddr:    r.RemoteAddr,
	}

	body, err := httputilx.DumpBody(r, 1000)
	if err != nil {
		h.Body = fmt.Sprintf("error reading body: %s", err)
		return h
	}

	if body != nil {
		// detect binary content
		if bytes.Contains(body, []byte{0}) {
			h.Body = "<binary>"
		} else {
			h.Body = string(body)
		}
	}

	return h
}
