package async_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/async"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	stateConf = async.StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForSomethingHappen(time.Now()),
		Timeout:    3 * time.Minute,
		Delay:      3 * time.Second,
		MinTimeout: 5 * time.Second,
	}

	stateErrorConf = async.StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForSomethingError(),
		Timeout:    3 * time.Minute,
		Delay:      3 * time.Second,
		MinTimeout: 5 * time.Second,
	}

	stateNoPendingStateConf = async.StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForSomethingNotInPendingState(),
		Timeout:    3 * time.Minute,
		Delay:      3 * time.Second,
		MinTimeout: 5 * time.Second,
	}

	stateExceedLimitStateConf = async.StateChangeConf{
		Pending:                    []string{"ACTIVE"},
		Target:                     []string{"DELETED"},
		Refresh:                    waitForAlwaysWrongState(),
		Timeout:                    3 * time.Minute,
		Delay:                      3 * time.Second,
		MinTimeout:                 5 * time.Second,
		ContinuousTargetOccurrence: 5,
	}
)

func waitForSomethingHappen(moment time.Time) async.StateRefreshFunc {
	return func() (any, string, error) {
		if time.Now().Sub(moment).Seconds() < 2 {
			return "", "ACTIVE", nil
		}
		return "", "DELETED", nil
	}
}

func waitForSomethingError() async.StateRefreshFunc {
	return func() (any, string, error) {
		return "", "", errors.New("error")
	}
}

func waitForSomethingNotInPendingState() async.StateRefreshFunc {
	return func() (any, string, error) {
		return "", "WAIT", nil
	}
}

func waitForAlwaysWrongState() async.StateRefreshFunc {
	return func() (any, string, error) {
		fmt.Println("call")
		return nil, "", nil
	}
}

func TestStateChangeConf_WaitForState(t *testing.T) {
	Convey("tst", t, func() {
		SkipConvey("成功", func() {
			// Wait for the subnet be DELETED
			_, err := stateConf.WaitForState()
			So(err, ShouldBeNil)
		})

		SkipConvey("返回状态不在pending表中", func() {
			// Wait for the subnet be DELETED
			_, err := stateNoPendingStateConf.WaitForState()
			So(err, ShouldNotBeNil)
		})

		SkipConvey("请求出错", func() {
			// Wait for the subnet be DELETED
			_, err := stateErrorConf.WaitForState()
			So(err, ShouldNotBeNil)
		})

		//Convey("超过指定次数", func() {
		//	// Wait for the subnet be DELETED
		//	_, err := stateExceedLimitStateConf.WaitForState()
		//	So(err, ShouldBeNil)
		//})
	})
}
