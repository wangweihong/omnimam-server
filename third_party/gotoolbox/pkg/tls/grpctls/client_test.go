package grpctls_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/tls/grpctls"

	. "github.com/smartystreets/goconvey/convey"
)

func Test_NewTlsClientSkipVerifiedCredentials(t *testing.T) {
	Convey("NewTlsClientSkipVerifiedCredentials", t, func() {
		creds, err := grpctls.NewTlsClientSkipVerifiedCredentials()
		So(err, ShouldBeNil)
		So(creds, ShouldNotBeNil)
	})
}

func Test_NewTlsClientCredentials(t *testing.T) {
	Convey("NewTlsClientCredentials", t, func() {
		Convey("正确证书密钥", func() {
			_, err := grpctls.NewTlsClientCredentials([]byte(serverCA))
			So(err, ShouldBeNil)
		})

		Convey("错误证书密钥", func() {
			var err error

			// 证书为空
			_, err = grpctls.NewTlsClientCredentials([]byte(nil))
			So(err, ShouldNotBeNil)

			// 错误证书格式
			_, err = grpctls.NewTlsClientCredentials([]byte("xxxx"))
			So(err, ShouldNotBeNil)

		})
	})
}

func Test_NewMutualTlsClientCredentials(t *testing.T) {
	Convey("NewMutualTlsClientCredentials", t, func() {
		Convey("正确证书密钥", func() {
			_, err := grpctls.NewMutualTlsClientCredentials([]byte(serverCA), []byte(clientCrt), []byte(clientKey))
			So(err, ShouldBeNil)
		})

		Convey("错误证书密钥", func() {
			var err error
			// 客户端CA证书为nil
			_, err = grpctls.NewMutualTlsClientCredentials([]byte(nil), []byte(clientCrt), []byte(clientKey))
			So(err, ShouldNotBeNil)

			// 服务端证书密钥为空
			_, err = grpctls.NewMutualTlsClientCredentials([]byte(serverCA), []byte(nil), []byte(nil))
			So(err, ShouldNotBeNil)

			// 错误ca格式
			_, err = grpctls.NewMutualTlsClientCredentials([]byte("serverCA"), []byte(clientCrt), []byte(clientKey))
			So(err, ShouldNotBeNil)

			// 错误证书格式
			_, err = grpctls.NewMutualTlsClientCredentials([]byte(serverCA), []byte("xxxx"), []byte(clientKey))
			So(err, ShouldNotBeNil)

			_, err = grpctls.NewMutualTlsClientCredentials([]byte(serverCA), []byte(clientCrt), []byte("clientKey"))
			So(err, ShouldNotBeNil)

		})
	})
}
