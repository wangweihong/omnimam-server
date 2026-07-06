package syncx

import (
	"sync/atomic"
)

const (
	True  int32 = 1
	False int32 = 0
)

func IsTrue(v *int32) bool {
	return atomic.LoadInt32(v) == True
}
func SetTrue(v *int32) {
	atomic.StoreInt32(v, True)
}
func SetFalse(v *int32) {
	atomic.StoreInt32(v, False)
}
func TrySetTrue(v *int32) bool {
	return atomic.CompareAndSwapInt32(v, False, True)
}
