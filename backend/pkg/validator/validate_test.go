package validator_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/pkg/validator"
)

func TestCustomValidator_ValidateName(t *testing.T) {
	cval := validator.NewCustomValidator("en")
	cval.Engine()
	Convey("Test Validate Names", t, func() {
		validateNames := []string{
			"Meta",
			"MetaList",
			"Harbor1",
			"Harbor2",
			"harbor-1323",
		}
		invalidNames := []string{
			"",
			"--_MetaList",
			"**",
			"_MetaList))",
			"harbor__",
			"example.com",
		}
		for _, name := range validateNames {
			Convey("It should return success for "+name, func() {
				d := imachinery.ObjectMeta{Name: name}
				So(cval.Validate(d), ShouldBeNil)
			})
		}
		for _, name := range invalidNames {
			Convey("It should return fail for "+name, func() {
				d := imachinery.ObjectMeta{Name: name}
				err := cval.Validate(d)
				So(err, ShouldNotBeNil)
				//				t.Log(err)
			})
		}
	})
}

func TestGin_BindingDecode_ValidateEN(t *testing.T) {
	// 创建 Gin 引擎

	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// 注册路由和绑定处理
	router.POST("/test", func(c *gin.Context) {
		var req struct {
			Name        string `json:"name" binding:"name" comment:"姓名"`
			Description string `json:"description" binding:"description"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Validation passed"})
	})

	Convey("Test Validate Names EN", t, func() {
		validator.Init("en")
		// 模拟有效请求
		validPayload := `{"name": "ValidName", "description": "Valid description"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(validPayload))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		// 检查成功响应
		So(resp.Code, ShouldEqual, http.StatusOK)
		So(resp.Body.String(), ShouldEqual, `{"message":"Validation passed"}`)

		invalidNamePayload := `{"name": "", "description": "Valid description"}`
		req = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(invalidNamePayload))
		req.Header.Set("Content-Type", "application/json")
		resp = httptest.NewRecorder()

		router.ServeHTTP(resp, req)

		// 检查无效 Name 的响应
		So(resp.Code, ShouldEqual, http.StatusBadRequest)
		So(resp.Body.String(), ShouldContainSubstring, `name must match pattern:^[a-zA-Z0-9]+(?:[_-][a-zA-Z0-9]+)*$`)
	})

	Convey("Test Validate Names ZH", t, func() {
		validator.Init("zh")
		Convey("Test Validate Names ZH ok", func() {
			// 模拟有效请求
			validPayload := `{"name": "ValidName", "description": "Valid description"}`
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(validPayload))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			// 检查成功响应
			So(resp.Code, ShouldEqual, http.StatusOK)
			So(resp.Body.String(), ShouldEqual, `{"message":"Validation passed"}`)
		})
		Convey("Test Validate Names ZH fail", func() {
			invalidNamePayload := `{"name": "", "description": "Valid description"}`
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(invalidNamePayload))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			// 检查无效 Name 的响应
			So(resp.Code, ShouldEqual, http.StatusBadRequest)
			So(resp.Body.String(), ShouldContainSubstring, `必须匹配正则表达式`)
			So(resp.Body.String(), ShouldContainSubstring, `姓名: 无效值:`)
		})
	})
}
