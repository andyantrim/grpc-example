package log

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

const maxStackFrames = 25

var myPath string

func init() {
	_, file, _, _ := runtime.Caller(0)
	myPath = filepath.Dir(file)
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type causer interface {
	Cause() error
}

type withStack struct {
	err   error
	stack errors.StackTrace
}

func (w *withStack) Cause() error                  { return w.err }
func (w *withStack) StackTrace() errors.StackTrace { return w.stack }

func (w *withStack) Error() string {
	if w.err == nil {
		return ""
	}
	return w.err.Error()
}

// Add a stack trace to an error.
func addStackTrace(err error) error {
	if _, ok := err.(stackTracer); ok {
		return err
	}

	pc := make([]uintptr, maxStackFrames)
	count := runtime.Callers(1, pc)

	var i int
	for ; i < count; i++ {
		fn := runtime.FuncForPC(pc[i])
		file, _ := fn.FileLine(pc[i])
		if !strings.HasPrefix(file, myPath) || strings.HasSuffix(file, "_test.go") {
			break
		}
	}

	stack := make([]errors.Frame, count-i)
	for j, ptr := range pc[i:count] {
		stack[j] = errors.Frame(ptr)
	}

	return &withStack{
		err:   err,
		stack: stack,
	}
}

// Get the first error in the error list which has a stack trace.
func earliestStackTracer(err error) error {
	var tracer error
	for err != nil {
		if _, ok := err.(stackTracer); ok {
			tracer = err
		}
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}

	return tracer
}
