package async

import (
	"context"
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

func PanicRecover(ctx context.Context, fns ...func()) {
	if x := recover(); x != nil {
		log.Errorf("run time panic: %v %v", x, string(debug.Stack()))
		for _, fn := range fns {
			fn()
		}
	}
}

func Run(ctx context.Context, f func(ctx context.Context)) {
	go func() {
		defer PanicRecover(ctx)
		f(ctx)
	}()

}

func GoRoutine(ctx context.Context, fs ...func(ctx context.Context)) {
	for i := range fs {
		f := fs[i]
		go func() {
			defer PanicRecover(ctx)
			f(ctx)
		}()
	}
}

func GoRoutineCustomPanicCover(ctx context.Context, cover func(), fs ...func(ctx context.Context)) {
	for i := range fs {
		f := fs[i]
		go func() {
			defer PanicRecover(ctx, cover)
			f(ctx)
		}()
	}
}

// ReallyCrash controls the behavior of HandleCrash and now defaults
// true. It's still exposed so components can optionally set to false
// to restore prior behavior.
var ReallyCrash = true

// PanicHandlers is a list of functions which will be invoked when a panic happens.
var PanicHandlers = []func(any){logPanic}

// HandleCrash simply catches a crash and logs an error. Meant to be called via
// defer.  Additional context-specific handlers can be provided, and will be
// called in case of panic.  HandleCrash actually crashes, after calling the
// handlers and logging the panic message.
//
// E.g., you can provide one or more additional handlers for something like shutting down go routines gracefully.
func HandleCrash(additionalHandlers ...func(any)) {
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

func logPanic(r any) {
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
