package action_test

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/action"

	"testing"
)

var a string

// Mock tasks for demonstration
func taskA() error {
	a = "a"
	return nil // Simulate success
}

func taskB() error {
	a = "b"

	return nil // Simulate success
}

func taskC() error {
	a = "c"
	return fmt.Errorf("taskC")
}

func taskD() error {
	a = "d"
	return fmt.Errorf("taskC")
}

func TestActionExecute(t *testing.T) {
	Convey("TestActionExecute", t, func() {
		executor := action.NewExecutor()

		// Define the list of tasks
		tasks := []action.Action{
			taskA,
			taskB,
			taskC,
			taskD,
		}
		err := executor.Execute(tasks)
		So(err, ShouldNotBeNil)
		So(a, ShouldEqual, "c")
	})
}

func TestExecute(t *testing.T) {
	Convey("TestActionExecute", t, func() {
		var err error
		err = action.Execute(err, taskA)
		err = action.Execute(err, taskB)
		err = action.Execute(err, taskC)
		err = action.Execute(err, taskD)
		So(err, ShouldNotBeNil)
		So(a, ShouldEqual, "c")

	})
}
