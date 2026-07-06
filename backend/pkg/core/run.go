package core

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func Run[T any](c *gin.Context, req T, action func(r T) (any, error)) {
	if err := DecodeParameter(c, req); err != nil {
		WriteResponse(c, err, nil)
		return
	}
	ret, err := action(req)
	if err != nil {
		WriteResponse(c, err, nil)
		return
	}

	if postHook, ok := ret.(imachinery.PostRun); ok {
		transObj := postHook.Transform()
		WriteResponse(c, nil, transObj)
		return
	}

	WriteResponse(c, err, ret)
}
