package urlutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	"github.com/wangweihong/gotoolbox/pkg/urlutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSplitURL(t *testing.T) {
	Convey("SplitURL", t, func() {
		Convey("http://127.0.0.1:1999", func() {
			schema, ip, port, err := urlutil.SplitURL("http://127.0.0.1:1999")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "http")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 1999)
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "http://127.0.0.1:1999")
		})
		Convey("127.0.0.1", func() {
			schema, ip, port, err := urlutil.SplitURL("127.0.0.1")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "http")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 80)
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "http://127.0.0.1")

		})
		Convey("https://127.0.0.1", func() {
			schema, ip, port, err := urlutil.SplitURL("https://127.0.0.1")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "https")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 443)
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "https://127.0.0.1")
		})

	})

	Convey("SplitURLV2", t, func() {
		Convey("https://127.0.0.1/rest/v2", func() {
			schema, ip, port, path, err := urlutil.SplitURLV2("https://127.0.0.1/rest/v2")
			So(err, ShouldBeNil)

			So(schema, ShouldEqual, "https")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 443)
			So(path, ShouldEqual, "/rest/v2")
		})

		Convey("http://127.0.0.1:1999", func() {
			schema, ip, port, path, err := urlutil.SplitURLV2("http://127.0.0.1:1999")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "http")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 1999)
			So(path, ShouldEqual, "")
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "http://127.0.0.1:1999")
		})
		Convey("127.0.0.1", func() {
			schema, ip, port, path, err := urlutil.SplitURLV2("127.0.0.1")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "http")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 80)
			So(path, ShouldEqual, "")
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "http://127.0.0.1")

		})
		Convey("https://127.0.0.1", func() {
			schema, ip, port, path, err := urlutil.SplitURLV2("https://127.0.0.1")
			So(err, ShouldBeNil)
			So(schema, ShouldEqual, "https")
			So(ip, ShouldEqual, "127.0.0.1")
			So(port, ShouldEqual, 443)
			So(path, ShouldEqual, "")
			So(urlutil.BuildURL(schema, ip, port), ShouldEqual, "https://127.0.0.1")
		})
	})
}

func TestTrimScheme(t *testing.T) {
	Convey("SplitURL", t, func() {
		So(urlutil.TrimScheme("http://10.30.100.122"), ShouldEqual, "10.30.100.122")
		So(urlutil.TrimScheme("https://10.30.100.122"), ShouldEqual, "10.30.100.122")
		So(urlutil.TrimScheme("10.30.100.122"), ShouldEqual, "10.30.100.122")
		So(urlutil.TrimScheme("http://10.30.100.122:188"), ShouldEqual, "10.30.100.122:188")
		So(urlutil.TrimScheme("http://10.30.100.122/r2/test"), ShouldEqual, "10.30.100.122/r2/test")
	})
}

func TestBuildURL(t *testing.T) {
	Convey("BuildURL", t, func() {
		So(urlutil.BuildURL("http", "127.0.0.1", 8443, ""), ShouldEqual, "http://127.0.0.1:8443")
		So(urlutil.BuildURL("http", "127.0.0.1", 8443, "abc", "123"), ShouldEqual, "http://127.0.0.1:8443/abc/123")
	})
}

func TestReplaceURL(t *testing.T) {
	Convey("ReplaceURL", t, func() {
		d, err := urlutil.ReplaceURL("http://127.0.0.1/v1/path",
			nil, typeutil.String("localhost"), typeutil.Int(8443), typeutil.String("/v2/path"))
		So(err, ShouldBeNil)

		So(d, ShouldEqual, "http://localhost:8443/v2/path")
	})
}
