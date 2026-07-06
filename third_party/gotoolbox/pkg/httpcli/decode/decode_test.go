package decode_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/httpcli/decode"

	. "github.com/smartystreets/goconvey/convey"
)

func TestUnmarshalManifest(t *testing.T) {
	mm := decode.NewMarshalMapping()

	mm.Register(decode.ApplicationJson, json.Unmarshal)
	mm.Register(decode.ApplicationXml, xml.Unmarshal)

	Convey("", t, func() {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/xml"},
			},
			Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`<bodyA><message>Hello, XML!</message></bodyA>`))),
		}

		type BodyA struct {
			Message string `xml:"message" json:"message"`
		}
		ba := BodyA{}

		b, err := ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()

		err = mm.UnmarshalManifest(resp.Header.Get(decode.ContentType), b, &ba)
		So(err, ShouldBeNil)
		So(ba.Message, ShouldEqual, "Hello, XML!")
	})
}
