package callerutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/callerutil"

	. "github.com/smartystreets/goconvey/convey"
)

func first(depth int) []string {
	data := callerutil.Stacks(depth)
	//	json.PrintStructObject(data)
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
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:12 github.com/wangweihong/gotoolbox/pkg/callerutil.first",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:18 github.com/wangweihong/gotoolbox/pkg/callerutil.second",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:22 github.com/wangweihong/gotoolbox/pkg/callerutil.third",
		//		"C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:42 github.com/wangweihong/gotoolbox/pkg/callerutil.TestStacks.func1",
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

func TestCaller(t *testing.T) {
	//json.PrintStructObject(callerutil.CallersDepth(-1, 0).StackTrace().List())
	//[
	//	"runtime.Callers C:/Users/Administrator/go/go1.20.12/src/runtime/extern.go:282",
	//	"github.com/wangweihong/gotoolbox/pkg/callerutil.CallerDepth C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack.go:201",
	//
	//	"github.com/wangweihong/gotoolbox/pkg/callerutil_test.TestCallerC:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/callerutil/stack_test.go:48",
	//	"testing.tRunner C:/Users/Administrator/go/go1.20.12/src/testing/testing.go:1576",
	//	"runtime.goexit C:/Users/Administrator/go/go1.20.12/src/runtime/asm_amd64.s:1598"
	//]
	//json.PrintStructObject(callerutil.Callers().StackTrace().List())
	//	[
	//		"testing.tRunner C:/Users/Administrator/go/go1.20.12/src/testing/testing.go:1576",
	//		"runtime.goexit C:/Users/Administrator/go/go1.20.12/src/runtime/asm_amd64.s:1598"
	//]
}
