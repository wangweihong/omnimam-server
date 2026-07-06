package wait

import (
	"net/http"
	"runtime"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

// ReallyCrash controls the behavior of HandleCrash and now defaults
// true. It's still exposed so components can optionally set to false
// to restore prior behavior.
var ReallyCrash = true

var PanicHandlers = []func(interface{}){logPanic}

func HandleCrash(additionalHandlers ...func(interface{})) {
	if r := recover(); r != nil {
		for _, fn := range PanicHandlers {
			fn(r)
		}
		for _, fn := range additionalHandlers {
			fn(r)
		}
		if ReallyCrash {
			// Actually proceed to panic.
			panic(r)
		}
	}
}

func logPanic(r interface{}) {
	if r == http.ErrAbortHandler {
		// honor the http.ErrAbortHandler sentinel panic value:
		//   ErrAbortHandler is a sentinel panic value to abort a handler.
		//   While any panic from ServeHTTP aborts the response to the client,
		//   panicking with ErrAbortHandler also suppresses logging of a stack trace to the server's error log.
		return
	}

	// Same as stdlib http server code. Manually allocate stack trace buffer size
	// to prevent excessively large logs
	const size = 64 << 10
	stacktrace := make([]byte, size)
	stacktrace = stacktrace[:runtime.Stack(stacktrace, false)]
	if _, ok := r.(string); ok {
		log.Errorf("Observed a panic: %s\n%s", r, stacktrace)
	} else {
		log.Errorf("Observed a panic: %#v (%v)\n%s", r, r, stacktrace)
	}
}
