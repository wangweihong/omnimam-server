package tls_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGeneratableKeyCert_Validate(t *testing.T) {
	Convey("校验证书", t, func() {
		c := tls.GeneratableKeyCert{
			CertData:      tls.CertData{},
			CertKey:       tls.CertKey{},
			CertDirectory: "",
			PairName:      "",
		}

		So(c.Validate(), ShouldNotBeNil)
		c.CertDirectory = "./"
		So(c.Validate(), ShouldNotBeNil)
		c.PairName = "test"
		So(c.Validate(), ShouldBeNil)
		c.CertKey.CertFile = "./test.crt"
		So(c.Validate(), ShouldNotBeNil)
		c.CertKey.KeyFile = "./test.key"
		So(c.Validate(), ShouldBeNil)
		c.CertData.Cert = "xxxx"
		So(c.Validate(), ShouldNotBeNil)
		c.CertData.Key = "xxx"
		So(c.Validate(), ShouldBeNil)
	})
}

var (
	tl = tls.GeneratableKeyCert{
		CertData: tls.CertData{
			Cert: "ssssss",
			Key:  "ssssss",
		},
		CertKey: tls.CertKey{
			CertFile: "xxxxx",
			KeyFile:  "xxxxxx",
		},
		CertDirectory: "",
		PairName:      "",
	}
)

func Test_TLS_CopyAndHide(t *testing.T) {
	Convey("TLS_CopyAndHide", t, func() {
		o := tl.CopyAndHide()

		So(o.CertData.Cert, ShouldNotEqual, "-")
		So(o.CertData.Key, ShouldNotEqual, "-")
		So(tl.CertData.Cert, ShouldEqual, "-")
		So(tl.CertData.Key, ShouldEqual, "-")
	})
}
