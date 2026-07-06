package ginx

import (
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/gin-gonic/gin"
)

// ErrResponse defines the return messages when an error occurred.
// Reference will be omitted if it does not exist.
// swagger:model
type Response struct {
	// Status contains the detail of this request.
	// Caller should check code to determine this request is success or not.
	Status *errors.Status `json:"status"`

	// Data contains dta
	Data any `json:"data,omitempty" `
}

// WriteResponse write an error or the response data into http response body.
// If err is nil, return a success code to tell request is ok.
func WriteResponse(c *gin.Context, err error, data any) {
	e := errors.ToStatus(err)

	if err != nil {
		log.F(c).Errorf("%#+v", e)
		c.JSON(e.HTTPStatus, Response{
			Status: e,
			Data:   data,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status: e,
		Data:   data,
	})
}
