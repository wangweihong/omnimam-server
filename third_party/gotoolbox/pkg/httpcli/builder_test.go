package httpcli_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/maputil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestHttpRequestBuilder_AddQueryParamByObject(t *testing.T) {
	Convey("AddQueryParamByObject", t, func() {
		type objType struct {
			String  string
			Num     int
			Flag    bool
			Pointer *string
			WithTag string `form:"with_tag"`
			OmitTag string `form:"omit_tag,omitempty"`
			exx     string
		}
		ot := objType{
			String:  "name",
			Num:     123,
			Flag:    false,
			Pointer: nil,
			WithTag: "tag1",
			OmitTag: "",
		}
		//expect := make(map[string]any)
		expect := make(map[string]any)
		expect = map[string]any{
			"with_tag": "tag1",
		}
		params := httpcli.NewHttpRequestBuilder().AddQueryParamByObject(ot).Build().GetQueryParams()

		So(params, ShouldResemble, expect)
	})
}

func TestHttpRequestBuilder_AddQueryParamByObjectEmbedded(t *testing.T) {
	Convey("AddQueryParamByObject", t, func() {
		type PagingParam struct {
			PageNum  int `form:"page_num" json:"page_num"`
			PageSize int `form:"page_size" json:"page_size"`
		}

		type objType struct {
			PagingParam
			String string
		}

		Convey("匿名结构字段不设置", func() {
			ot := objType{
				String: "name",
			}
			//expect := make(map[string]any)
			expect := make(map[string]any)
			expect = map[string]any{
				"page_num":  0,
				"page_size": 0,
			}
			So(maputil.StringAny(httpcli.NewHttpRequestBuilder().AddQueryParamByObject(ot).Build().GetQueryParams()).Equal(expect), ShouldBeTrue)
		})

		Convey("匿名结构字段设置", func() {
			ot := objType{
				String: "name",
				PagingParam: PagingParam{
					PageNum:  1,
					PageSize: 3,
				},
			}
			//expect := make(map[string]any)
			expect := make(map[string]any)
			expect = map[string]any{
				"page_num":  1,
				"page_size": 3,
			}
			So(maputil.StringAny(httpcli.NewHttpRequestBuilder().AddQueryParamByObject(ot).Build().GetQueryParams()).Equal(expect), ShouldBeTrue)
		})
	})
}

func TestHttpRequestBuilder_AddPathParamByObjectEmbedded(t *testing.T) {
	Convey("AddPathParamByObject", t, func() {
		type Param struct {
			PageNum  int `form:"page_num" json:"page_num" path:"page_num"`
			PageSize int `form:"page_size" json:"page_size"`
		}

		type objType struct {
			Param
			Name string `json:"name" path:"name"`
		}

		Convey("匿名结构字段设置", func() {
			ot := objType{
				Name: "test",
			}
			//expect := make(map[string]any)
			expect := make(map[string]string)
			expect = map[string]string{
				"name":     "test",
				"page_num": "0",
			}

			So(httpcli.NewHttpRequestBuilder().AddPathParamByObject(ot).Build().GetPathPrams(), ShouldResemble, expect)
		})
	})
}
