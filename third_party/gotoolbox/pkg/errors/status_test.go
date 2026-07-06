package errors_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/json"
)

func init() {
	errors.UpdateModuleInfo(errors.NewModuleGetter("github.com/wangweihong/gotoolbox", "127.0.0.1", 123))
}

func example() error {
	e := errors.NewStatus(ErrEOF, "error example")
	return e
}

func TestErrorStack(t *testing.T) {
	Convey("check errors function line number", t, func() {
		// status
		Convey("ErrorStack error", func() {
			e := example()
			e = errors.WrapStatus(e, ErrEOF)
			e = errors.WrapStatus(e, ErrCall)

			s2 := errors.ToStatus(e)
			//模拟跨服务错误转换
			e2 := s2.Error()
			s3 := errors.ToStatus(e2)
			So(s3.HTTPStatus, ShouldEqual, 500)
			So(s3.Code, ShouldEqual, ErrCall)
			So(len(s3.Cause), ShouldEqual, 2)
			json.PrintObject(s3)

			/*
					{
					"HTTPStatus": 500,
					"Code": 1001,
					"Message": {
						"CN": "请求失败",
						"EN": "call error"
					},
					"Desc": "call error:call error:End of input:End of input:error example",
					"Cause": [
						{
							"Service": {
								"Host": "127.0.0.1",
								"Pid": 123,
								"Name": "github.com/wangweihong/gotoolbox"
							},
							"Stacks": [
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:31 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1.1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:23 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:21 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack'"
							]
						},
						{
							"Service": {
								"Host": "127.0.0.1",
								"Pid": 123,
								"Name": "github.com/wangweihong/gotoolbox"
							},
							"Stacks": [
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:27 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1.1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:26 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1.1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:16 github.com/wangweihong/gotoolbox/pkg/errors_test.example'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:25 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1.1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:23 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack.func1'",
								"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:21 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack'"
							]
						}
					]
				}
			*/
			fmt.Println(e2.Error())
			fmt.Println(errors.Cause(e))
			fmt.Println(e2.Error())

		})
	})
}

func efirst() error {
	e := errors.NewStatus(ErrEOF, "error example")
	return e
}

func esecond() error {
	e := efirst()
	if e != nil {
		return e
	}
	return nil
}

func ethird() error {
	e := esecond()
	if e != nil {
		return errors.NewStatus(ErrCall, e.Error())
	}
	return nil
}

func TestErrorStack_ToStatus(t *testing.T) {
	Convey("check errors function line number", t, func() {
		// status
		Convey("ErrorStack.ToStatus error", func() {

			//模拟跨服务错误转换
			e2 := ethird()
			s3 := errors.ToStatus(e2)
			_ = s3
			json.PrintObject(s3)
			//	{
			//		"HTTPStatus": 500,
			//		"Code": 1001,
			//		"Message": {
			//		"CN": "请求失败",
			//			"EN": "call error"
			//	},
			//		"Desc": "call error:End of input:error example",
			//		"Cause": [
			//	{
			//		"Service": {
			//			"Host": "127.0.0.1",
			//			"Pid": 123,
			//			"Name": "github.com/wangweihong/gotoolbox"
			//		},
			//		"Stacks": [
			//			"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:103 github.com/wangweihong/gotoolbox/pkg/errors_test.ethird'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:114 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2.func1.1'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:111 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2.func1'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:109 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2'"
			//	]
			//	}
			//]
			//}
		})

		Convey("ErrorStack.ToStatus with code", func() {

			//模拟跨服务错误转换
			e2 := errors.WithCode(123, "34")
			s3 := errors.ToStatus(e2)
			_ = s3
			json.PrintObject(s3)
			//	{
			//		"HTTPStatus": 500,
			//		"Code": 1001,
			//		"Message": {
			//		"CN": "请求失败",
			//			"EN": "call error"
			//	},
			//		"Desc": "call error:End of input:error example",
			//		"Cause": [
			//	{
			//		"Service": {
			//			"Host": "127.0.0.1",
			//			"Pid": 123,
			//			"Name": "github.com/wangweihong/gotoolbox"
			//		},
			//		"Stacks": [
			//			"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:103 github.com/wangweihong/gotoolbox/pkg/errors_test.ethird'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:114 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2.func1.1'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:111 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2.func1'",
			//		"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:109 github.com/wangweihong/gotoolbox/pkg/errors_test.TestErrorStack2'"
			//	]
			//	}
			//]
			//}
		})

		Convey("ErrorStack.ToStatus wrap code", func() {

			//模拟跨服务错误转换
			e2 := errors.WrapCode(efirst(), 123)
			s3 := errors.ToStatus(e2)
			_ = s3
			json.PrintObject(s3)

		})
	})
}

func TestUpdateService(t *testing.T) {
	Convey("update service stacks", t, func() {
		e := example()
		st := errors.ToStatus(e)
		st = st.UpdateStatus()
		_ = st
		json.PrintObject(st)
	})
}

func serviceC_HttpHandler(w http.ResponseWriter, r *http.Request) {
	errors.UpdateModuleInfo(errors.NewModuleGetter("github.com/wangweihong/gotoolbox", "127.0.0.1", 123))
	e := example()
	st := errors.FromError(e).ToStatus()

	w.WriteHeader(st.HTTPStatus)
	w.Write([]byte(json.ShouldEncode(st)))
}

func serviceB_HttpHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		st := errors.ToStatus(errors.WrapStatus(err, ErrCall))
		http.Error(w, json.ShouldEncode(st), st.HTTPStatus)
		return
	}
	param := json.ShouldDecode(reqBody)
	resp, err := http.Get(param["url"].(string))
	if err != nil {
		st := errors.ToStatus(errors.WrapStatus(err, ErrCall))
		http.Error(w, json.ShouldEncode(st), st.HTTPStatus)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		st := errors.ToStatus(errors.WrapStatus(err, ErrCall))
		http.Error(w, json.ShouldEncode(st), st.HTTPStatus)
		return
	}

	st := &errors.Status{}
	if err := json.Decode(body, st); err != nil {
		st := errors.ToStatus(errors.WrapStatus(err, ErrCall))
		http.Error(w, json.ShouldEncode(st), st.HTTPStatus)
		return
	}

	// add current module stack
	st = st.UpdateStatus()
	if !errors.IsSuccessCode(st.Code) {
		http.Error(w, json.ShouldEncode(st), st.HTTPStatus)
		return
	}

	w.Write([]byte(json.ShouldEncode(st)))
}

func TestServicesStatus(t *testing.T) {
	serverC := httptest.NewServer(http.HandlerFunc(serviceC_HttpHandler))
	defer serverC.Close()

	serverB := httptest.NewServer(http.HandlerFunc(serviceB_HttpHandler))
	defer serverB.Close()

	Convey("模拟服务间调用", t, func() {
		//os.Setenv("ERROR_STACK_ALL", "1")
		r := make(map[string]any)
		r["url"] = serverC.URL
		s := json.ShouldEncode(r)

		req, err := http.NewRequest("POST", serverB.URL, bytes.NewBuffer([]byte(s)))
		So(err, ShouldBeNil)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)
		So(err, ShouldBeNil)

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)

		st := &errors.Status{}
		err = json.Decode(body, st)
		So(err, ShouldBeNil)

		json.PrintObject(st)
		/*
		   {
		   	"HTTPStatus": 200,
		   	"Code": 1000,
		   	"Message": {
		   		"CN": "输入终止",
		   		"EN": "End of input"
		   	},
		   	"Desc": "End of input:error example",
		   	"Cause": [{
		   			"Service": {
		   				"Host": "127.0.0.1",
		   				"Pid": 123,
		   				"Name": "github.com/wangweihong/gotoolbox"
		   			},
		   			"Stacks": [
		   				"github.com/wangweihong/gotoolbox/pkg/errors/status.go:43 github.com/wangweihong/gotoolbox/pkg/errors.(*Status).UpdateStatus'",
		   				"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:199 github.com/wangweihong/gotoolbox/pkg/errors_test.serviceB_HttpHandler'"
		   			]
		   		},
		   		{
		   			"Service": {
		   				"Host": "127.0.0.1",
		   				"Pid": 123,
		   				"Name": "github.com/wangweihong/gotoolbox"
		   			},
		   			"Stacks": [
		   				"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:21 github.com/wangweihong/gotoolbox/pkg/errors_test.example'",
		   				"github.com/wangweihong/gotoolbox/pkg/errors/status_test.go:161 github.com/wangweihong/gotoolbox/pkg/errors_test.serviceC_HttpHandler'"
		   			]
		   		}
		   	]
		   }
		*/
	})
}

func TestFormatError(t *testing.T) {
	Convey("update service stacks", t, func() {
		e := example()
		fmt.Printf("%v\n", e)
		fmt.Printf("%+v\n", e)
	})
}
