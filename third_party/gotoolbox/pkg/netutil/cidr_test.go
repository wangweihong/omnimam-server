package netutil_test

import (
	"fmt"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/netutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCIDR(t *testing.T) {
	Convey("cidr", t, func() {
		Convey("", func() {
			ipNet, err := netutil.ValidateCIDR("127.0.0.1/32")
			So(err, ShouldBeNil)
			So(len(netutil.GenerateIPs(ipNet)), ShouldEqual, 1)

		})
		Convey("v2", func() {
			ipNet, err := netutil.ValidateCIDR("10.30.100.200/29")
			So(err, ShouldBeNil)
			for _, ip := range netutil.GenerateIPs(ipNet) {
				fmt.Println(ip)
			}
		})

	})
}
