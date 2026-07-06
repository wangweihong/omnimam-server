package syncer_test

import (
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/syncer"
	"github.com/wangweihong/gotoolbox/pkg/wait"
	"github.com/wangweihong/gotoolbox/pkg/workqueue"

	. "github.com/smartystreets/goconvey/convey"
)

type Object struct {
	ID   string
	Name string
	Data string
}

func getObject(id string) *Object {
	return &Object{
		ID:   "123",
		Name: "123",
		Data: "213",
	}
}

func TestNewWorkequeueSyncer(t *testing.T) {
	Convey("onework", t, func() {
		stop := make(chan struct{}, 0)
		s := syncer.NewWorkequeueSyncer(func(key interface{}) error {
			id := key.(string)
			getObject(id)
			return nil
		}, workqueue.New(), 3*time.Second, 1, 3)
		s.Run(stop)
		s.Trigger("123", false)
		s.Trigger("123", false)

		go wait.Until(func() {
			s.Trigger("123", false)
		}, 300*time.Second, stop)

		go func() {
			select {
			case <-time.After(1 * time.Second):
				close(stop)
			}
		}()
		<-stop
	})
}
