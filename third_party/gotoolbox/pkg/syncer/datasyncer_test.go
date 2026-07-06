package syncer_test

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/syncer"
)

func TestNewDataSyncer(t *testing.T) {
	Convey("data", t, func() {

		Convey("worker syncer", func() {
			s := syncer.NewOneWorkerDataSyncer(func(arg any) (any, error) {
				if arg != nil {
					return true, nil
				}
				return false, nil
			}, 500*time.Millisecond, 3)
			s.Trigger(true, false)
			time.Sleep(500 * time.Millisecond)
			So(s.Get(), ShouldResemble, true)
			s.Trigger(nil, false)
			time.Sleep(500 * time.Millisecond)
			So(s.Get(), ShouldResemble, false)

		})
	})
}
