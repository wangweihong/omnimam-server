package errors_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/callerutil"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/json"
)

func first(depth int) []string {
	data := callerutil.Stacks(depth)
	//json.PrintStructObject(data)
	return data
}

func second(depth int) []string {
	return first(depth)
}

func third(depth int) []string {
	return second(depth)
}

func TestStacks(t *testing.T) {
	Convey("print stack depth", t, func() {
		So(len(third(0)), ShouldEqual, 0)
		// ["C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:12 github.com/wangweihong/gotoolbox/pkg/errors_test.first"]
		So(len(third(1)), ShouldEqual, 1)
		//	[
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:12 github.com/wangweihong/gotoolbox/pkg/errors_test.first",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:18 github.com/wangweihong/gotoolbox/pkg/errors_test.second",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:22 github.com/wangweihong/gotoolbox/pkg/errors_test.third",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:30 github.com/wangweihong/gotoolbox/pkg/errors_test.TestStacks.func1",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/smartystreets/goconvey@v1.8.1/convey/discovery.go:89 github.com/smartystreets/goconvey/convey.parseAction.func1",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/smartystreets/goconvey@v1.8.1/convey/context.go:279 github.com/smartystreets/goconvey/convey.(*context).conveyInner",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/smartystreets/goconvey@v1.8.1/convey/context.go:112 github.com/smartystreets/goconvey/convey.rootConvey.func1",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/jtolds/gls@v4.20.0+incompatible/context.go:97 github.com/jtolds/gls.(*ContextManager).SetValues.func1",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/jtolds/gls@v4.20.0+incompatible/gid.go:24 github.com/jtolds/gls.EnsureGoroutineId.func1",
		//		"C:/Users/Administrator/go/pkg/mod/github.com/jtolds/gls@v4.20.0+incompatible/stack_tags.go:108 github.com/jtolds/gls._m"
		//]
		So(len(third(10)), ShouldEqual, 10)
	})
}

func pfirst() []string {
	data := errors.ProjectStacks()
	json.PrintStructObject(data)
	return data
}

func psecond() []string {
	return pfirst()
}

func pthird() []string {
	return psecond()
}

func TestProjectStacks(t *testing.T) {
	Convey("print stack in project", t, func() {
		errors.UpdateModuleInfo(errors.NewModuleGetter("github.com/wangweihong/gotoolbox", "127.0.0.1", 123))
		pthird()
	})
	// 文件路径为相对路径， 隐藏前缀防止信息泄露
	//[
	//"module:'github.com/wangweihong/gotoolbox',host:'127.0.0.1', pid:'7704', caller:'github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:47 github.com/wangweihong/gotoolbox/pkg/errors_test.pfirst'",
	//"module:'github.com/wangweihong/gotoolbox',host:'127.0.0.1', pid:'7704', caller:'github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:53 github.com/wangweihong/gotoolbox/pkg/errors_test.psecond'",
	//"module:'github.com/wangweihong/gotoolbox',host:'127.0.0.1', pid:'7704', caller:'github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:57 github.com/wangweihong/gotoolbox/pkg/errors_test.pthird'",
	//"module:'github.com/wangweihong/gotoolbox',host:'127.0.0.1', pid:'7704', caller:'github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:63 github.com/wangweihong/gotoolbox/pkg/errors_test.TestProjectStacks.func1'",
	//"module:'github.com/wangweihong/gotoolbox',host:'127.0.0.1', pid:'7704', caller:'github.com/wangweihong/gotoolbox/pkg/errors/stack_test.go:61 github.com/wangweihong/gotoolbox/pkg/errors_test.TestProjectStacks'"
	//]
}

func TestSdsf(t *testing.T) {
	a := errors.CallersDepth(10, 3).StackTrace()
	json.PrintStructObject(a)
	b := errors.CallersDepth(10, 3).StackTrace()
	json.PrintStructObject(b)

	merged := errors.MergeFrame(b, a)
	json.PrintStructObject(merged)
}
