package statemachine_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/statemachine"
)

func TestStateMachine(t *testing.T) {
	Convey("TestStateMachine", t, func() {
		const (
			NotStarted statemachine.State = "未开始"
			Deploying                     = "部署中"
			Success                       = "部署成功"
			Failed                        = "部署失败"
		)
		// 创建状态机
		fsm := statemachine.New(NotStarted)

		// 添加状态转移规则
		fsm.AddRule(NotStarted, Deploying)
		fsm.AddRule(Deploying, Success)
		fsm.AddRule(Deploying, Failed)
		fsm.AddRule(Success, NotStarted)
		fsm.AddRule(Failed, NotStarted)

		So(fsm.Transition(Deploying), ShouldBeNil)
		So(fsm.Transition(NotStarted), ShouldNotBeNil)
		So(fsm.Transition(Deploying), ShouldNotBeNil)
		So(fsm.Transition(Success), ShouldBeNil)

	})
}
