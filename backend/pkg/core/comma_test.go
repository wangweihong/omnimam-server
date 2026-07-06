package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/httpcli/httpresponse"

	"github.com/wangweihong/omnimam/backend/pkg/core"
)

type QueryParams struct {
	IDs core.CommaSeparatedList `form:"ids"` // 对应 ?ids=1,2,3
}

func TestCommonDecode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	router.GET("/list", func(c *gin.Context) {
		var query QueryParams
		if err := c.ShouldBindQuery(&query); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"parsed_ids": query.IDs.Items,
		})
	})

	Convey("Test Query CommaSeparatedList Decode", t, func() {
		Convey("Test Query CommaSeparatedList Decode", func() {
			req := httptest.NewRequest(http.MethodGet, "/list?ids=1,2,3", nil)
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			So(resp.Code, ShouldEqual, http.StatusOK)
			So(httpresponse.NewHttpResponse(resp.Result()).GetBody(), ShouldEqual, `{"parsed_ids":["1","2","3"]}`)
		})
	})
}
