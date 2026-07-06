package callstatus_test

//import (
//	"testing"
//
//	. "github.com/smartystreets/goconvey/convey"
//
//	"github.com/wangweihong/gotoolbox/pkg/errors"
//	"github.com/wangweihong/gotoolbox/pkg/log"
//
//	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
//	"github.com/wangweihong/omnimam/backend/pkg/grpcproto/apis/callstatus"
//)
//
//func TestCallStatus_FromError(t *testing.T) {
//	Convey("FromError", t, func() {
//		Convey("nil", func() {
//			cs := callstatus.FromError(nil)
//			So(cs, ShouldNotBeNil)
//			So(cs.Code, ShouldEqual, int64(code.ErrSuccess))
//		})
//
//		Convey("not nil", func() {
//			err := errors.Wrap(code.ErrDecodingJSON, "something")
//			cs := callstatus.FromError(err)
//			So(cs, ShouldNotBeNil)
//			So(cs.Code, ShouldEqual, int64(code.ErrDecodingJSON))
//			So(cs.Description, ShouldEqual, "something")
//			So(len(cs.Stack), ShouldNotEqual, 0)
//		})
//	})
//}
//
//func TestCallStatus_ToError(t *testing.T) {
//	Convey("ToError", t, func() {
//		Convey("nil", func() {
//			err := callstatus.ToError(nil)
//			So(err, ShouldBeNil)
//		})
//
//		Convey("not nil", func() {
//			st := errors.Wrap(code.ErrDecodingJSON, "something")
//			err := errors.WithStack(st)
//			cs := callstatus.FromError(err)
//			e := callstatus.ToError(cs)
//			So(e, ShouldNotBeNil)
//			So(len(errors.FromError(err).Stack()), ShouldNotEqual, 0)
//			log.Infof("%v", errors.FromError(err).Stack())
//		})
//	})
//}
