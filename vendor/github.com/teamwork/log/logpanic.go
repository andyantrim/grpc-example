package log

import (
	"github.com/pkg/errors"
)

// RecoverPanic recovers a panicking function and logs the panic.
//
// You need to call this as a deferred function:
//
//     defer log.RecoverPanic()
// Only the first set of log.Fields passed in will be used
// any more will be discarded
func RecoverPanic(f ...Fields) {
	logPanic(recover(), f...)
}

// RecoverPanicCallback recovers a panicking function, logs the panic, and runs
// the specified callback.
func RecoverPanicCallback(callback func(), f ...Fields) {
	if r := recover(); r != nil {
		logPanic(r, f...)
		callback()
	}
}

func logPanic(r interface{}, f ...Fields) {
	if r == nil {
		return
	}

	err, ok := r.(error)
	if !ok {
		err = errors.Errorf("%v", r)
	}

	l := Module("logpanic")
	if len(f) > 0 {
		l = l.WithFields(f[0])
	}
	l.Error(err)
}
