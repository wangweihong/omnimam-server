package recovery

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"runtime/debug"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

func DefaultPanicHandler(ctx context.Context, p any) (any, error) {
	log.F(ctx).Errorf("[panic] %s:", string(debug.Stack()))
	return nil, status.Errorf(codes.Unknown, "panic triggered: %v", p)
}

func CustomPanicHandler(ctx context.Context, p any) (any, error) {
	var stackMessage string
	panicStacks := make([]string, 0, 10)
	for i := 3; ; i++ {
		pc, file, line, ok := runtime.Caller(i)

		if !ok {
			break
		}
		log.F(ctx).Errorf("%s:%d %s", file, line, function(pc))
		stackMessage = stackMessage + fmt.Sprintf("%s:%d %s\n", file, line, function(pc))
		panicStacks = append(panicStacks, stackMessage)
	}
	return panicStacks, status.Errorf(codes.Unknown, "panic:%v", p)
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contain dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)
