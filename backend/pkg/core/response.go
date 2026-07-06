package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
)

type ErrResponse struct {
	// Data contain
	Data any `json:"data,omitempty"`

	// Code defines the business error code.
	Code int `json:"code"`

	// Message contains the detail of this code.
	// This message is suitable to be exposed to external
	Message string `json:"message"`

	Messages map[string]string `json:"messages"`

	// Message contains the detail of why cause this error.
	Detail string `json:"detail"`

	Causes []errors.ServiceStack `json:"causes"`
}

// WriteResponse write an error or the response data into http response body.
// It use errors.ParseCoder to parse any error into errors.Coder
// errors.Coder contains error code, user-safe error message and http status code.
func WriteResponse(c *gin.Context, err error, data any) {
	if err != nil {
		log.Errorf("%#+v", err)
		st := errors.ToStatus(err)

		// st := errors.ToStatus(err)

		c.JSON(st.HTTPStatus, ErrResponse{
			Data:     data,
			Code:     st.Code,
			Message:  maputil.Get(st.Message, errors.MessageLangENKey),
			Messages: st.Message,
			Detail:   st.Desc,
			Causes:   st.Cause,
		})

		return
	}

	c.JSON(http.StatusOK, data)
}
