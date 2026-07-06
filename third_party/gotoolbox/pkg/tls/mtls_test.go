package tls_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	mtl = &tls.MTLSCert{
		GeneratableKeyCert: tls.GeneratableKeyCert{
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
		},
		ClientCAData: "xxxxxxx",
		ClientCAPath: "xxxxxxx",
	}
)

func Test_MTLS_CopyAndHide(t *testing.T) {
	Convey("MTLS_CopyAndHide", t, func() {
		o := mtl.CopyAndHide()
		So(o.ClientCAData, ShouldNotEqual, "-")
		So(mtl.ClientCAData, ShouldEqual, "-")

		So(o.CertData.Cert, ShouldNotEqual, "-")
		So(o.CertData.Key, ShouldNotEqual, "-")
		So(mtl.CertData.Cert, ShouldEqual, "-")
		So(mtl.CertData.Key, ShouldEqual, "-")
	})
}

func Test_MTLS_DeepCopy(t *testing.T) {
	Convey("MTLS_CopyAndHide", t, func() {
		o := mtl.DeepCopy()
		So(o, ShouldResemble, mtl)

		var nil *tls.MTLSCert
		nilO := nil.DeepCopy()
		So(nilO, ShouldBeNil)
	})
}
