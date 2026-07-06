package tokenutil_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/url"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/tokenutil"
)

func TestDefaultJWTTrackedRequestCodec(t *testing.T) {
	Convey("TestDefaultJWTTrackedRequestCodec", t, func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		So(err, ShouldBeNil)

		testURL, _ := url.Parse("https://test.example.com")
		opts := tokenutil.Options{URL: *testURL, Key: key, MaxIssueDelay: 3 * time.Second}
		codec := tokenutil.DefaultJWTTrackedRequestCodec(opts)

		tokenStr, err := codec.Encode(tokenutil.TrackedRequest{
			Value: map[string]any{
				"UserID":   "test",
				"ClientIP": "127.0.0.1",
			},
		})
		So(err, ShouldBeNil)

		fmt.Println(tokenStr)

		tr, err := codec.Decode(tokenStr)
		So(err, ShouldBeNil)
		So(tr.Value, ShouldResemble, map[string]any{
			"UserID":   "test",
			"ClientIP": "127.0.0.1",
		})

		time.Sleep(4 * time.Second)
		tr, err = codec.Decode(tokenStr)
		So(err, ShouldNotBeNil)
		fmt.Println(err)
	})
}

func TestJWTCodec(t *testing.T) {
	d := map[string]any{
		"UserID":   "test",
		"ClientIP": "127.0.0.1",
	}
	Convey("TestJWTCodec", t, func() {
		Convey("NewHMACJWTCodec", func() {
			codec := tokenutil.NewHMACJWTCodec(nil, 2*time.Second)
			tokenStr, err := codec.Encode(tokenutil.TrackedRequest{Value: d})
			So(err, ShouldBeNil)
			tr, err := codec.Decode(tokenStr)
			So(err, ShouldBeNil)
			So(tr.Value, ShouldResemble, d)
			time.Sleep(3 * time.Second)
			tr, err = codec.Decode(tokenStr)
			So(err, ShouldNotBeNil)
			fmt.Println(err)
		})
		Convey("NewRSAJWTCodec", func() {
			codec := tokenutil.NewRSAJWTCodec(nil, 2*time.Second)
			tokenStr, err := codec.Encode(tokenutil.TrackedRequest{Value: d})
			So(err, ShouldBeNil)
			tr, err := codec.Decode(tokenStr)
			So(err, ShouldBeNil)
			So(tr.Value, ShouldResemble, d)
			time.Sleep(3 * time.Second)
			tr, err = codec.Decode(tokenStr)
			So(err, ShouldNotBeNil)
			fmt.Println(err)
		})
		Convey("NewEDRSAJWTCodec", func() {
			codec := tokenutil.NewECDSAJWTCodec(nil, 2*time.Second)
			tokenStr, err := codec.Encode(tokenutil.TrackedRequest{Value: d})
			So(err, ShouldBeNil)
			tr, err := codec.Decode(tokenStr)
			So(err, ShouldBeNil)
			So(tr.Value, ShouldResemble, d)
			time.Sleep(3 * time.Second)
			tr, err = codec.Decode(tokenStr)
			So(err, ShouldNotBeNil)
			fmt.Println(err)
		})
	})
}
