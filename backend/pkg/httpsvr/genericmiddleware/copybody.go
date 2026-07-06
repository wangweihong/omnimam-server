package genericmiddleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

const (
	DisableCopy                = false
	RequestBodyKey             = "req_body"  // post请求将请求数据写到gin.Context中
	ResponseBodyKey            = "resp_body" // 当回应结束后插入回应数据到gin.Context中
	HTTPMaxContentLength int64 = 0
)

// Copy body to context bytes array.
func CopyBodyMiddleware(skippers ...skipper.SkipperFunc) gin.HandlerFunc {
	var maxMemory int64 = 4 << 20 // 4 MB
	//if v := HTTPMaxContentLength; v > 0 {
	//	maxMemory = v
	//}

	return func(c *gin.Context) {
		if skipper.Skip(c.Request.URL.Path, skippers...) || c.Request.Body == nil {
			c.Next()
			return
		}

		if !DisableCopy {
			var requestBody []byte
			isGzip := false
			safe := &io.LimitedReader{R: c.Request.Body, N: maxMemory}

			if c.GetHeader("Content-Encoding") == "gzip" {
				reader, err := gzip.NewReader(safe)
				if err == nil {
					isGzip = true
					requestBody, _ = ioutil.ReadAll(reader)
				}
			}

			if !isGzip {
				requestBody, _ = ioutil.ReadAll(safe)
			}

			c.Request.Body.Close()
			bf := bytes.NewBuffer(requestBody)
			c.Request.Body = http.MaxBytesReader(c.Writer, ioutil.NopCloser(bf), maxMemory)
			c.Set(RequestBodyKey, requestBody)
		}
		c.Next()
	}
}

func SetResponseBody(c *gin.Context, data any) {
	if !DisableCopy {
		b, err := json.Marshal(data)
		if err != nil {
			log.L(c).Errorf("json marshal data error:%v", err)
		}
		c.Set(ResponseBodyKey, b)
	}
}
