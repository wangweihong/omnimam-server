//go:build !windows
// +build !windows

package netutil

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPing(t *testing.T) {
	Convey("ping", t, func() {
		So(Ping("127.0.0.1", 5), ShouldBeTrue)
	})
}
