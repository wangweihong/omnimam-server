package tracectx

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

const (
	// XRequestIDKey defines X-Request-ID key string.
	XRequestIDKey = "X-Request-ID"
)

var (
	incrNum uint64
	pid     = os.Getpid()
)

// NewTraceID generate trace id.
func NewTraceID() string {
	return fmt.Sprintf("trace-id-%d-%s-%d",
		pid,
		time.Now().Format("2006.01.02.15.04.05.999"),
		atomic.AddUint64(&incrNum, 1))
}
